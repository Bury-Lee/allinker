// cli_server.go — 中心服务模式处理(未完成)

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"allinker/account"
	"allinker/core"
	initt "allinker/init"
	lockpkg "allinker/lock"
	msgpkg "allinker/message"
	"allinker/model"
	"allinker/wait"
	"allinker/watch"
)

// runServerMode 启动中心服务模式
func runServerMode(port int, daemon, stop, status, restart bool, humanMode bool) {
	if stop {
		stopServer(humanMode)
		return
	}
	if status {
		showServerStatus(humanMode)
		return
	}
	if restart {
		stopServer(humanMode)
		time.Sleep(1 * time.Second)
	}

	startServer(port, daemon, humanMode)
}

// startServer 启动 HTTP 服务
func startServer(port int, daemon bool, humanMode bool) {
	// 写入 PID 文件
	pidPath := core.Global.ServerPIDPath()
	pid := os.Getpid()
	os.WriteFile(pidPath, []byte(fmt.Sprintf("%d", pid)), 0644)

	addr := fmt.Sprintf(":%d", port)

	if humanMode {
		fmt.Print("allinker 中心服务启动\n")
		fmt.Printf("   地址: http://127.0.0.1:%d\n", port)
		fmt.Printf("   数据目录: %s\n", core.Global.Root())
		fmt.Printf("   服务运行中 (PID: %d)\n", pid)
		fmt.Printf("    按 Ctrl+C 停止\n\n")
	} else {
		fmt.Printf("服务已启动 http://127.0.0.1:%d\n", port)
	}

	// 注册路由
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/command", handleAPICommand)
	mux.HandleFunc("/api/v1/health", handleAPIHealth)
	mux.HandleFunc("/api/v1/status", handleAPIStatus)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"service": "allinker",
			"version": Version,
			"status":  "running",
		})
	})

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	if err := server.ListenAndServe(); err != nil {
		fmt.Fprintf(os.Stderr, "服务启动失败: %v\n", err)
		os.Exit(1)
	}
}

// stopServer 停止服务
func stopServer(humanMode bool) {
	pidPath := core.Global.ServerPIDPath()
	data, err := os.ReadFile(pidPath)
	if err != nil {
		if humanMode {
			fmt.Println("服务未在运行")
		}
		return
	}

	pid := 0
	fmt.Sscanf(string(data), "%d", &pid)
	if pid > 0 {
		proc, err := os.FindProcess(pid)
		if err == nil {
			proc.Kill()
		}
	}
	os.Remove(pidPath)

	if humanMode {
		fmt.Println("正在停止服务...")
		fmt.Println("服务已停止")
	} else {
		fmt.Println("服务已停止")
	}
}

// showServerStatus 查看服务状态
func showServerStatus(humanMode bool) {
	pidPath := core.Global.ServerPIDPath()
	data, err := os.ReadFile(pidPath)
	if err != nil {
		if humanMode {
			fmt.Println("服务未在运行")
		} else {
			fmt.Println("服务未运行")
		}
		os.Exit(1)
	}

	pid := 0
	fmt.Sscanf(string(data), "%d", &pid)
	if humanMode {
		fmt.Printf("服务运行中 (PID: %d)\n", pid)
		fmt.Printf("   数据目录: %s\n", core.Global.Root())
	} else {
		fmt.Printf("服务运行中 (PID: %d)\n", pid)
	}
}

// =============================================================================
// HTTP API 处理函数
// =============================================================================

// apiCommandRequest API 命令请求体
type apiCommandRequest struct {
	ID      string         `json:"id,omitempty"`
	Command string         `json:"command"`
	Args    map[string]any `json:"args"`
	Async   bool           `json:"async,omitempty"`
}

// apiCommandResponse API 命令响应体
type apiCommandResponse struct {
	ID       string `json:"id,omitempty"`
	Success  bool   `json:"success"`
	Data     string `json:"data,omitempty"`
	Error    string `json:"error,omitempty"`
	ExitCode int    `json:"exitCode,omitempty"`
}

// handleAPICommand 处理 POST /api/v1/command 请求
func handleAPICommand(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiCommandResponse{Error: "读取请求体失败"})
		return
	}

	var req apiCommandRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiCommandResponse{Error: "解析 JSON 失败"})
		return
	}

	// 执行命令（同步模式，目前不支持异步）
	result := executeCommand(req.Command, req.Args)

	writeJSON(w, http.StatusOK, apiCommandResponse{
		ID:       req.ID,
		Success:  result.Success,
		Data:     result.Data,
		Error:    result.Error,
		ExitCode: result.ExitCode,
	})
}

// cmdResult executeCommand 的返回结果
type cmdResult struct {
	Success  bool
	Data     string
	Error    string
	ExitCode int
}

// executeCommand 根据命令名和参数执行对应的逻辑
func executeCommand(cmd string, args map[string]any) cmdResult {
	switch cmd {
	case "register":
		name, _ := args["name"].(string)
		role, _ := args["role"].(string)
		if role == "" {
			role = "agent"
		}
		user, err := account.Register(name, model.UserRole(role))
		if err != nil {
			return cmdResult{Error: err.Error(), ExitCode: 1}
		}
		return cmdResult{Success: true, Data: fmt.Sprintf("账号已注册: %s", user.Name)}

	case "lock":
		filename, _ := args["file"].(string)
		username, _ := args["user"].(string)
		timeout := 60
		if t, ok := args["timeout"].(float64); ok {
			timeout = int(t)
		}
		deadline := time.Now().Add(time.Duration(timeout) * time.Second)
		err := lockpkg.AcquireLock(filename, username, deadline)
		if err != nil {
			return cmdResult{Error: err.Error(), ExitCode: 3}
		}
		return cmdResult{Success: true, Data: "锁获取成功"}

	case "send":
		from, _ := args["user"].(string)
		toStr, _ := args["to"].(string)
		content, _ := args["msg"].(string)
		toList := strings.Split(toStr, ",")
		msg, err := msgpkg.SendMessage(from, toList, content)
		if err != nil {
			return cmdResult{Error: err.Error(), ExitCode: 1}
		}
		return cmdResult{Success: true, Data: fmt.Sprintf("消息已发送 (ID: %d)", msg.ID)}

	case "recv":
		from, _ := args["from"].(string)
		user, _ := args["user"].(string)
		messages, err := msgpkg.ReceiveMessages(from, user, false, 0)
		if err != nil {
			return cmdResult{Error: err.Error(), ExitCode: 1}
		}
		if len(messages) == 0 {
			return cmdResult{Success: true, Data: "没有未读消息"}
		}
		var parts []string
		for _, msg := range messages {
			parts = append(parts, fmt.Sprintf("%s: %s", msg.From, msg.Content))
		}
		return cmdResult{Success: true, Data: strings.Join(parts, "\n")}

	case "status":
		return cmdResult{Success: true, Data: "服务运行中"}

	case "unlock":
		filename, _ := args["file"].(string)
		username, _ := args["user"].(string)
		if filename == "" || username == "" {
			return cmdResult{Error: "参数缺失: --file 和 --user 是必需的", ExitCode: 1}
		}
		err := lockpkg.ReleaseLock(filename, username)
		if err != nil {
			return cmdResult{Error: err.Error(), ExitCode: 1}
		}
		return cmdResult{Success: true, Data: "锁已释放"}

	case "tryLock":
		filename, _ := args["file"].(string)
		username, _ := args["user"].(string)
		if filename == "" || username == "" {
			return cmdResult{Error: "参数缺失: --file 和 --user 是必需的", ExitCode: 1}
		}
		err := lockpkg.TryAcquireLock(filename, username)
		if err != nil {
			return cmdResult{Error: err.Error(), ExitCode: 3}
		}
		return cmdResult{Success: true, Data: "锁获取成功"}

	case "lockInfo":
		filename, _ := args["file"].(string)
		if filename == "" {
			return cmdResult{Error: "参数缺失: --file 是必需的", ExitCode: 1}
		}
		info := lockpkg.GetLockInfo(filename)
		if info == nil || info.IsExpired() {
			return cmdResult{Success: true, Data: fmt.Sprintf("文件 %s 没有被锁定", filename)}
		}
		return cmdResult{Success: true, Data: fmt.Sprintf("持有者: %s, 剩余时间: %d秒", info.Holder, info.RemainingSeconds())}

	case "listLocks":
		locks := lockpkg.ListLocks()
		if len(locks) == 0 {
			return cmdResult{Success: true, Data: "没有活动的锁"}
		}
		var parts []string
		for _, l := range locks {
			status := "有效"
			if l.IsExpired() {
				status = "已过期"
			}
			parts = append(parts, fmt.Sprintf("%s | %s | %s", l.Filename, l.Holder, status))
		}
		return cmdResult{Success: true, Data: strings.Join(parts, "\n")}

	case "user":
		sub, _ := args["sub"].(string)
		username, _ := args["user"].(string)
		target, _ := args["name"].(string)
		reason, _ := args["reason"].(string)

		switch sub {
		case "list":
			users, err := account.ListUsers()
			if err != nil {
				return cmdResult{Error: err.Error(), ExitCode: 1}
			}
			var parts []string
			for _, u := range users {
				parts = append(parts, fmt.Sprintf("%s (%s)", u.Name, u.Role))
			}
			return cmdResult{Success: true, Data: strings.Join(parts, "\n")}
		case "log":
			if target == "" {
				return cmdResult{Error: "请使用 --name 指定目标用户", ExitCode: 1}
			}
			return cmdResult{Success: true, Data: fmt.Sprintf("查看 %s 的操作记录（请在服务器端执行）", target)}
		case "disable":
			if target == "" {
				return cmdResult{Error: "请使用 --name 指定要禁用的用户", ExitCode: 1}
			}
			err := account.DisableUser(target, reason, username)
			if err != nil {
				return cmdResult{Error: err.Error(), ExitCode: 1}
			}
			return cmdResult{Success: true, Data: fmt.Sprintf("账号已禁用: %s", target)}
		case "enable":
			if target == "" {
				return cmdResult{Error: "请使用 --name 指定要启用的用户", ExitCode: 1}
			}
			err := account.EnableUser(target, username)
			if err != nil {
				return cmdResult{Error: err.Error(), ExitCode: 1}
			}
			return cmdResult{Success: true, Data: fmt.Sprintf("账号已启用: %s", target)}
		case "delete":
			if target == "" {
				return cmdResult{Error: "请使用 --name 指定要删除的用户", ExitCode: 1}
			}
			err := account.DeleteUser(target, username)
			if err != nil {
				return cmdResult{Error: err.Error(), ExitCode: 1}
			}
			return cmdResult{Success: true, Data: fmt.Sprintf("账号已删除: %s", target)}
		default:
			return cmdResult{Error: fmt.Sprintf("未知 user 子命令: %s", sub), ExitCode: 1}
		}

	case "watch":
		sub, _ := args["sub"].(string)
		watchName, _ := args["name"].(string)
		dir, _ := args["dir"].(string)
		pattern, _ := args["pattern"].(string)
		creator, _ := args["user"].(string)

		switch sub {
		case "add":
			if watchName == "" || dir == "" || pattern == "" {
				return cmdResult{Error: "参数缺失: --name, --dir 和 --pattern 是必需的", ExitCode: 1}
			}
			item, err := watch.AddWatch(watchName, dir, pattern, creator)
			if err != nil {
				return cmdResult{Error: err.Error(), ExitCode: 1}
			}
			return cmdResult{Success: true, Data: fmt.Sprintf("监听位已添加: %s", item.Name)}
		case "list":
			items, err := watch.ListWatches(watchName)
			if err != nil {
				return cmdResult{Error: err.Error(), ExitCode: 1}
			}
			if len(items) == 0 {
				return cmdResult{Success: true, Data: "没有监听位"}
			}
			var parts []string
			for _, item := range items {
				parts = append(parts, fmt.Sprintf("%s | %s | %s", item.Name, item.Dir, item.Pattern))
			}
			return cmdResult{Success: true, Data: strings.Join(parts, "\n")}
		case "check":
			if watchName == "" {
				return cmdResult{Error: "参数缺失: --name 是必需的", ExitCode: 1}
			}
			files, err := watch.CheckWatch(watchName)
			if err != nil {
				return cmdResult{Error: err.Error(), ExitCode: 1}
			}
			if len(files) == 0 {
				return cmdResult{Success: true, Data: fmt.Sprintf("监听位 %s 无变更", watchName)}
			}
			return cmdResult{Success: true, Data: fmt.Sprintf("变更文件:\n%s", strings.Join(files, "\n"))}
		case "remove":
			if watchName == "" {
				return cmdResult{Error: "参数缺失: --name 是必需的", ExitCode: 1}
			}
			err := watch.RemoveWatch(watchName, creator)
			if err != nil {
				return cmdResult{Error: err.Error(), ExitCode: 1}
			}
			return cmdResult{Success: true, Data: fmt.Sprintf("监听位已移除: %s", watchName)}
		case "clear":
			err := watch.ClearWatches(creator)
			if err != nil {
				return cmdResult{Error: err.Error(), ExitCode: 1}
			}
			return cmdResult{Success: true, Data: "所有监听位已清空"}
		default:
			return cmdResult{Error: fmt.Sprintf("未知 watch 子命令: %s", sub), ExitCode: 1}
		}

	case "wait":
		mode, _ := args["mode"].(string)
		switch mode {
		case "message":
			who, _ := args["from"].(string)
			timeout := 60
			if t, ok := args["timeout"].(float64); ok {
				timeout = int(t)
			}
			content, elapsed, err := wait.WaitForMessage(who, timeout)
			if err != nil {
				return cmdResult{Error: err.Error(), ExitCode: 2}
			}
			return cmdResult{Success: true, Data: fmt.Sprintf("收到消息: %s (等待%d秒)", content, elapsed)}
		case "file":
			dir, _ := args["dir"].(string)
			pattern, _ := args["pattern"].(string)
			timeout := 60
			if t, ok := args["timeout"].(float64); ok {
				timeout = int(t)
			}
			printContent, _ := args["printContent"].(bool)
			watchMode, _ := args["watchMode"].(string)
			matchedFile, elapsed, err := wait.WaitForFile(dir, pattern, timeout, printContent, watchMode)
			if err != nil {
				return cmdResult{Error: err.Error(), ExitCode: 2}
			}
			return cmdResult{Success: true, Data: fmt.Sprintf("发现文件: %s (等待%d秒)", matchedFile, elapsed)}
		default:
			return cmdResult{Error: fmt.Sprintf("未知 wait 模式: %s（可用: message, file）", mode), ExitCode: 1}
		}

	case "history":
		withUser, _ := args["with"].(string)
		limit := 20
		if l, ok := args["limit"].(float64); ok {
			limit = int(l)
		}
		messages, err := msgpkg.GetHistory(withUser, limit)
		if err != nil {
			return cmdResult{Error: err.Error(), ExitCode: 1}
		}
		if len(messages) == 0 {
			return cmdResult{Success: true, Data: "没有历史消息"}
		}
		var parts []string
		for _, msg := range messages {
			parts = append(parts, fmt.Sprintf("[%s] %s → %s: %s", msg.Timestamp, msg.From, msg.To, msg.Content))
		}
		return cmdResult{Success: true, Data: strings.Join(parts, "\n")}

	default:
		return cmdResult{Error: fmt.Sprintf("未知命令: %s", cmd), ExitCode: 1}
	}
}

// handleAPIHealth 处理 GET /api/v1/health 请求
func handleAPIHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleAPIStatus 处理 GET /api/v1/status 请求
func handleAPIStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"service":    "allinker",
		"version":    Version,
		"status":     "running",
		"dataDir":    core.Global.Root(),
		"auditCount": getAuditCount(),
	})
}

// getAuditCount 获取审计日志条数
func getAuditCount() int {
	entries, err := initt.ReadAuditLog()
	if err != nil {
		return 0
	}
	return len(entries)
}

// writeJSON 写入 JSON 响应
func writeJSON(w http.ResponseWriter, statusCode int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// =============================================================================
// 客户端模式（连接中心服务）
// =============================================================================

// runClientMode 以客户端模式运行命令（连接中心服务）
func runClientMode(protocol, address string, args []string) {
	baseURL := address
	if protocol == "http" {
		baseURL = strings.TrimRight(address, "/")
	}

	// 构建请求
	cmd := ""
	if len(args) > 0 {
		cmd = args[0]
	}

	reqBody := apiCommandRequest{
		Command: cmd,
		Args:    parseArgsToMap(args[1:]),
	}

	body, _ := json.Marshal(reqBody)
	resp, err := http.Post(baseURL+"/api/v1/command", "application/json", strings.NewReader(string(body)))
	if err != nil {
		fmt.Fprintf(os.Stderr, "连接服务失败: %v\n", err)
		fmt.Fprintln(os.Stderr, "   请确认服务是否已启动 (allinker -server)")
		os.Exit(1)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result apiCommandResponse
	json.Unmarshal(respBody, &result)

	if result.Error != "" {
		fmt.Fprintf(os.Stderr, "%s\n", result.Error)
		os.Exit(result.ExitCode)
	}

	if result.Data != "" {
		fmt.Println(result.Data)
	}
}

// parseArgsToMap 将命令行参数解析为 map
func parseArgsToMap(args []string) map[string]any {
	m := make(map[string]any)
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "--") {
			key := strings.TrimPrefix(arg, "--")
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
				m[key] = args[i+1]
				i++
			} else {
				m[key] = true
			}
		} else if strings.HasPrefix(arg, "-") {
			key := strings.TrimPrefix(arg, "-")
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				m[key] = args[i+1]
				i++
			} else {
				m[key] = true
			}
		}
	}
	return m
}

// tryConnectAndRun 尝试连接服务并执行命令，成功返回 true
func tryConnectAndRun(args []string) bool {
	// 尝试连接本地默认端口
	baseURL := "http://127.0.0.1:8080"

	resp, err := http.Get(baseURL + "/api/v1/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	// 服务可连接，使用客户端模式
	runClientMode("http", baseURL, args)
	return true
}
