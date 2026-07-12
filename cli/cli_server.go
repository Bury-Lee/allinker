// cli_server.go — 中心服务模式处理

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"allinker/config"
	"allinker/core"
	initt "allinker/init"
	"allinker/logutil"
	"allinker/utils"
)

// runServerMode 启动中心服务模式
func runServerMode(port int, stop, status, restart bool, humanMode bool) {
	// 禁止嵌套 -r：如果通过远程连接发送 -server 指令，拒绝执行
	if os.Getenv("allinker_REMOTE_ACTIVE") != "" {
		fmt.Fprintln(os.Stderr, "错误: 不允许在远程连接中启动服务模式")
		os.Exit(1)
	}
	if stop {
		stopServer(humanMode)
		return
	}
	if status {
		if err := showServerStatus(humanMode); err != nil {
			fmt.Fprintf(os.Stderr, "错误: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if restart {
		stopServer(humanMode)
		time.Sleep(1 * time.Second)
	}

	if err := startServer(port, humanMode); err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}
}

// startServer 启动 HTTP 服务（使用 lock 文件替代 PID）
// 返回 error 表示启动失败；启动成功后阻塞直到服务退出。
func startServer(port int, humanMode bool) error {
	cfg, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("读取配置失败: %w", err)
	}

	host := cfg.Server.Host
	if host == "" {
		host = "127.0.0.1"
	}
	serverPort := cfg.Server.Port
	if serverPort == 0 {
		serverPort = 8080
	}
	if port > 0 {
		serverPort = port
	}

	addr := fmt.Sprintf("%s:%d", host, serverPort)

	lockPath := filepath.Join(core.Global.Root(), "server.lock")
	lockFile, err := tryAcquireServerLock(lockPath)
	if err != nil {
		return fmt.Errorf("%w\n   使用 allinker -server --stop 停止", err)
	}
	defer func() {
		lockFile.Close()
		os.Remove(lockPath)
	}()
	fmt.Fprintf(lockFile, "%d", os.Getpid())
	lockFile.Sync()

	pid := os.Getpid()

	if humanMode {
		fmt.Print("allinker 中心服务启动\n")
		fmt.Printf("   地址: http://%s\n", addr)
		fmt.Printf("   数据目录: %s\n", core.Global.Root())
		fmt.Printf("   服务运行中 (PID: %d)\n", pid)
		if cfg.Server.AuthToken != "" {
			fmt.Printf("   密码已启用\n")
		}
		fmt.Printf("    按 Ctrl+C 停止\n\n")
	} else {
		fmt.Printf("服务已启动 http://%s\n", addr)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/command", withAuth(withLog(handleAPICommand)))
	mux.HandleFunc("/api/v1/health", withLog(handleAPIHealth))
	mux.HandleFunc("/api/v1/status", withLog(handleAPIStatus))
	mux.HandleFunc("/api/v1/reload", withAuth(withLog(handleAPIReload)))
	mux.HandleFunc("/api/v1/info", withLog(handleAPIInfo))
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

	// 正常退出时清理 lock 文件
	// 注意：os.Exit 或 SIGKILL 不会执行到这里，lock 文件会残留
	// 方案：下次启动时检测 lock 文件中的 pid 是否存活，若已死则覆盖

	if err := server.ListenAndServe(); err != nil {
		return fmt.Errorf("服务启动失败: %w", err)
	}

	// 退出时清理 lock 文件
	os.Remove(lockPath)
	lockFile.Close()

	return nil
}

// stopServer 停止服务（通过 lock 文件）
func stopServer(humanMode bool) {
	lockPath := filepath.Join(core.Global.Root(), "server.lock")
	data, err := os.ReadFile(lockPath)
	if err != nil {
		if humanMode {
			fmt.Println("服务未在运行")
		} else {
			fmt.Println("服务未运行")
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
	os.Remove(lockPath)

	if humanMode {
		fmt.Println("正在停止服务...")
		fmt.Println("服务已停止")
	} else {
		fmt.Println("服务已停止")
	}
}

// showServerStatus 查看服务状态（通过 lock 文件）
// 返回 error 表示服务未运行或读取失败；成功时输出状态信息到 stdout。
func showServerStatus(humanMode bool) error {
	lockPath := filepath.Join(core.Global.Root(), "server.lock")
	data, err := os.ReadFile(lockPath)
	if err != nil {
		if humanMode {
			fmt.Println("服务未在运行")
		} else {
			fmt.Println("服务未运行")
		}
		return fmt.Errorf("服务未运行")
	}

	pid := 0
	fmt.Sscanf(string(data), "%d", &pid)
	if humanMode {
		fmt.Printf("服务运行中 (PID: %d)\n", pid)
		fmt.Printf("   数据目录: %s\n", core.Global.Root())
	} else {
		fmt.Printf("服务运行中 (PID: %d)\n", pid)
	}
	return nil
}

// =============================================================================
// HTTP API 处理函数
// =============================================================================

// apiCommandResponse API 命令响应体
type apiCommandResponse struct {
	ID       string `json:"id,omitempty"`
	Success  bool   `json:"success"`
	Data     string `json:"data,omitempty"`
	Error    string `json:"error,omitempty"`
	ExitCode int    `json:"exitCode,omitempty"`
}

// cmdResult executeCommand 的返回结果
type cmdResult struct {
	Success  bool
	Data     string
	Error    string
	ExitCode int
}

// handleAPICommand 处理 POST /api/v1/command 请求
// 使用 CommandArg 统一结构体，不再需要 mapToArgs/parseArgsToMap 转换。
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

	var req struct {
		ID      string      `json:"id"`
		Command string      `json:"command"`
		Args    *CommandArg `json:"args"`
		Async   bool        `json:"async"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiCommandResponse{Error: "解析 JSON 失败"})
		return
	}

	cmd := req.Args

	// 指令过滤：禁止远程客户端执行全局服务指令
	if cmd == nil {
		cmd = &CommandArg{}
	}
	if req.Command != "" {
		cmd.Command = req.Command
	}

	// 执行命令（同步模式）
	result := ExecuteParsedResult(cmd)

	writeJSON(w, http.StatusOK, apiCommandResponse{
		ID:       req.ID,
		Success:  result.Success,
		Data:     result.Data,
		Error:    result.Error,
		ExitCode: result.ExitCode,
	})
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
// 三步走：[]string → ParseArgs → CommandArg → JSON → POST
func runClientMode(protocol, address string, args []string) {
	baseURL := address
	if protocol == "http" {
		baseURL = strings.TrimRight(address, "/")
	}

	// 使用 ParseArgs 统一解析参数（三步走的第一步）
	cmd := ParseArgs(args, false)
	cmd.HumanMode = false

	// 直接序列化 CommandArg 发送到服务端
	req := struct {
		ID      string      `json:"id,omitempty"`
		Command string      `json:"command"`
		Args    *CommandArg `json:"args"`
		Async   bool        `json:"async,omitempty"`
	}{
		Command: cmd.Command,
		Args:    cmd,
	}

	body, _ := json.Marshal(req)
	httpReq, err := http.NewRequest("POST", baseURL+"/api/v1/command", strings.NewReader(string(body)))
	if err != nil {
		fmt.Fprintf(os.Stderr, "创建请求失败: %v\n", err)
		os.Exit(1)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	if token := os.Getenv("allinker_REMOTE_TOKEN"); token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(httpReq)
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

// tryConnectAndRun 尝试连接服务并执行命令，成功返回 true
func tryConnectAndRun(args []string) bool {
	baseURL := "http://127.0.0.1:8080"

	resp, err := http.Get(baseURL + "/api/v1/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	runClientMode("http", baseURL, args)
	return true
}

// tryAcquireServerLock 尝试获取服务 lock 文件。
func tryAcquireServerLock(lockPath string) (*os.File, error) {
	f, err := os.OpenFile(lockPath, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0644)
	if err == nil {
		return f, nil
	}

	data, err := os.ReadFile(lockPath)
	if err != nil {
		return nil, fmt.Errorf("读取 lock 文件失败: %w", err)
	}
	var pid int
	if _, err := fmt.Sscanf(string(data), "%d", &pid); err != nil || pid <= 0 {
		os.Remove(lockPath)
		return os.OpenFile(lockPath, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0644)
	}

	proc, err := os.FindProcess(pid)
	if err == nil {
		if err := proc.Signal(os.Signal(syscall.Signal(0))); err == nil {
			return nil, fmt.Errorf("服务已在运行 (PID: %d, lock: %s)", pid, lockPath)
		}
	}

	os.Remove(lockPath)
	return os.OpenFile(lockPath, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0644)
}

// =============================================================================
// 中间件
// =============================================================================

// withAuth 鉴权中间件：从 config.json 读取 authToken 的 SHA-256 哈希并校验。
func withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg, err := config.GetConfig()
		if err == nil && cfg.Server.AuthToken != "" {
			token := r.Header.Get("Authorization")
			if !strings.HasPrefix(token, "Bearer ") || len(token) <= 7 {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}
			hashedToken := utils.HashTokenSHA256(token[7:])
			if hashedToken != cfg.Server.AuthToken {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}
		}
		next(w, r)
	}
}

// withLog 日志中间件
func withLog(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next(w, r)
		logutil.Info("%s %s (%s)", r.Method, r.URL.Path, time.Since(start))
	}
}

// handleAPIReload 处理 POST /api/v1/reload
func handleAPIReload(w http.ResponseWriter, r *http.Request) {
	config.Reload()
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "message": "配置已重新加载"})
}

// handleAPIInfo 处理 GET /api/v1/info
func handleAPIInfo(w http.ResponseWriter, r *http.Request) {
	cfg, _ := config.GetConfig()
	info := map[string]any{
		"service":   "allinker",
		"version":   Version,
		"status":    "running",
		"dataDir":   core.Global.Root(),
		"bind":      fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		"auth":      cfg.Server.AuthToken != "",
		"auditSize": getAuditCount(),
	}
	writeJSON(w, http.StatusOK, info)
}
