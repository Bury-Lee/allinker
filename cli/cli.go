// Package cli 解析命令行参数并将命令分发给对应的处理模块。
//
// allinker CLI 工具的命令格式：
//
//	allinker [全局选项] <命令> [参数...]
//
// 全局选项：
//
//	--data-dir   指定数据目录（默认 .alf）
//	--help       显示帮助信息
//	--version    显示版本信息
//
// 中心服务模式：
//
//	allinker -server             启动服务
//	allinker -server --port 8080 指定端口
//
// 客户端模式：
//
//	allinker --server http://127.0.0.1:8080 lock -f xxx --user XXX
package cli

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"allinker/account"
	initt "allinker/init"
)

// Version 是工具的版本号，编译时可通过 -ldflags 注入。
var Version = "1.0.0"

// ExitCode 存储命令的退出码。
var ExitCode = 0

// Run 是 CLI 的入口函数，解析参数并执行对应命令。
func Run() {
	// =========================================================================
	// 定义全局标志
	// =========================================================================
	dataDir := flag.String("data-dir", "", "数据目录路径（默认 .alf）")
	showHelp := flag.Bool("help", false, "显示帮助信息")
	showVersion := flag.Bool("version", false, "显示版本信息")

	// 服务模式标志
	serverMode := flag.Bool("server", false, "启动中心服务模式（预备字段）")
	serverPort := flag.Int("port", 8080, "服务端口（预备字段，仅服务模式有效）")
	serverDaemon := flag.Bool("daemon", false, "后台运行（预备字段，仅服务模式有效）")
	serverStop := flag.Bool("stop", false, "停止服务")
	serverStatus := flag.Bool("status", false, "查看服务状态")
	serverRestart := flag.Bool("restart", false, "重启服务")

	// 客户端模式标志
	serverURL := flag.String("connect", "", "连接中心服务地址（如 http://127.0.0.1:8080）")
	socketPath := flag.String("socket", "", "通过 Unix Socket 连接服务")
	autoMode := flag.Bool("auto", false, "自动检测服务，存在则连接，否则直接执行")

	// 输出模式
	humanMode := flag.Bool("human", false, "人类模式（详细输出）")
	aiMode := flag.Bool("ai", false, "AI 模式（结构化输出，默认启用）")
	aiHelp := flag.Bool("AIhelp", false, "显示 AI 使用建议")

	// 自定义 Usage
	flag.Usage = func() {
		fmt.Fprint(os.Stderr, `用法: allinker [全局选项] <命令> [参数...]

为不同的 AI Agent 提供统一的协作入口，实现跨 Agent 协同工作。

全局选项:
  --data-dir <路径>   数据目录（默认 .alf）
  --ai                输出 AI 可解析的结构化内容
  --human             人类可读输出（带 emoji/表格）
  --help              显示此帮助
  --version           显示版本号
  --AIhelp             显示 AI 使用建议

命令:

  账号管理
    register --name <用户名> [--role admin|agent|guest]
                                      注册新账号

  文件锁
    lock -f <文件> -t <秒> --user <用户名>
                                      获取锁（阻塞等待）
    tryLock -f <文件> --user <用户名>  获取锁（立即返回）
    unlock -f <文件> --user <用户名>   释放锁
    status [-f <文件> | --all]        查看锁状态

  消息通信
    send [--to <接收方>] --msg <内容> --user <用户名>
                                      发送消息（默认 All 群发）
    recv [--from <发送方>] [--id <ID>] [--limit <数量>] [--all] [--user <用户名>]
                                      接收消息（默认仅未读）
    history [--with <用户>] [--limit <条数>]
                                      查看通信记录

  等待
    wait -m file -d <目录> -f <模式> [-t <秒>] [--quiet] [--print-content]
                                      等待文件出现（默认模式）
    wait -m message [--from <发送者>] [-t <秒>]
                                      等待其他用户发来的消息

  文件监听
    watch add --name <名称> -d <目录> -p <模式> --user <用户名>
                                      注册监听位，监控文件变化
    watch list [--name <名称>]        查看已注册的监听位
    watch check --name <名称>         检查监听位，返回新增或修改的文件
    watch remove --name <名称> --user <用户名>
                                      取消监听位
    watch clear --user <用户名>       清空当前用户的所有监听位
    watch wait --name <名称> [-t <秒>]  阻塞等待监听位文件变更

  管理
    user list --user <用户名>                     查看所有用户
    user log --name <用户名> --user <管理员>       查看操作记录
    user disable --name <用户名> --reason <原因> --user <管理员>
    user enable --name <用户名> --user <管理员>
    user delete --name <用户名> --user <管理员>

  数据维护
    fix [--check] [--dry-run]         检查并修复数据文件完整性

退出码:
  0    操作成功
  1    一般错误
  2    超时
  3    锁获取失败
  4    账号不存在
  5    权限不足
  6    文件不存在
`)
	}

	flag.Parse()

	// 确定输出模式（AI 模式为默认，--human 切换到人类模式）
	isHuman := *humanMode
	_ = aiMode // --ai 可显式指定，效果同默认输出

	// =========================================================================
	// 处理 --help 和 --version
	// =========================================================================
	if *showHelp {
		flag.Usage()
		os.Exit(0)
	}
	if *showVersion {
		fmt.Printf("allinker v%s\n", Version)
		os.Exit(0)
	}
	if *aiHelp {
		printAIHelp()
		os.Exit(0)
	}

	// =========================================================================
	// 处理服务模式标志
	// =========================================================================
	if *serverMode {
		runServerMode(*serverPort, *serverDaemon, *serverStop, *serverStatus, *serverRestart, isHuman)
		return
	}

	// =========================================================================
	// 初始化数据目录
	// =========================================================================
	initDataDir(*dataDir)

	// =========================================================================
	// 处理客户端模式
	// =========================================================================
	if *serverURL != "" {
		runClientMode("http", *serverURL, flag.Args())
		return
	}
	if *socketPath != "" {
		runClientMode("unix", *socketPath, flag.Args())
		return
	}
	if *autoMode {
		// 尝试连接服务，失败则回退到 CLI 模式
		if tryConnectAndRun(flag.Args()) {
			return
		}
		// 回退到 CLI 模式，继续执行
	}

	// =========================================================================
	// CLI 模式 — 解析子命令
	// =========================================================================
	args := flag.Args()
	if len(args) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	cmd := args[0]
	cmdArgs := args[1:]

	switch cmd {
	case "register":
		handleRegister(cmdArgs, isHuman)
	case "lock":
		handleLock(cmdArgs, isHuman)
	case "tryLock":
		handleTryLock(cmdArgs, isHuman)
	case "unlock":
		handleUnlock(cmdArgs, isHuman)
	case "status":
		handleLockStatus(cmdArgs, isHuman)
	case "watch":
		handleWatch(cmdArgs, isHuman)
	case "wait":
		handleWait(cmdArgs, isHuman)
	case "send":
		handleSend(cmdArgs, isHuman)
	case "recv":
		handleRecv(cmdArgs, isHuman)
	case "history":
		handleHistory(cmdArgs, isHuman)
	case "user":
		handleUser(cmdArgs, isHuman)
	case "fix":
		handleFix(cmdArgs, isHuman)
	default:
		fmt.Fprintf(os.Stderr, " 未知命令: %s\n", cmd)
		fmt.Fprintf(os.Stderr, "   使用 allinker --help 查看可用命令\n")
		os.Exit(1)
	}

	os.Exit(ExitCode)
}

// initDataDir 初始化数据目录。
func initDataDir(dataDir string) {
	absDir, err := initt.InitDataDir(dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: 初始化数据目录失败: %v\n", err)
		os.Exit(1)
	}
	// 设置数据目录环境变量，供子命令使用
	os.Setenv("ALLINKER_DATA_DIR", absDir)
}

// parseIntArg 从参数列表中解析一个 int 类型的命名参数。
// 使用全新切片避免底层数组别名问题。
func parseIntArg(args []string, name string, defaultVal int) (int, []string) {
	for i, arg := range args {
		if arg == name || arg == "-"+name {
			if i+1 < len(args) {
				val := 0
				if _, err := fmt.Sscanf(args[i+1], "%d", &val); err == nil {
					// 构建新切片，避免共享底层数组
					remaining := make([]string, 0, len(args)-2)
					remaining = append(remaining, args[:i]...)
					remaining = append(remaining, args[i+2:]...)
					return val, remaining
				}
			}
		}
		if strings.HasPrefix(arg, name+"=") || strings.HasPrefix(arg, "-"+name+"=") {
			parts := strings.SplitN(arg, "=", 2)
			if len(parts) == 2 {
				val := 0
				if _, err := fmt.Sscanf(parts[1], "%d", &val); err == nil {
					remaining := make([]string, 0, len(args)-1)
					remaining = append(remaining, args[:i]...)
					remaining = append(remaining, args[i+1:]...)
					return val, remaining
				}
			}
		}
	}
	return defaultVal, args
}

// parseStringArg 从参数列表中解析一个 string 类型的命名参数。
// 使用全新切片避免底层数组别名问题。
func parseStringArg(args []string, name string, defaultVal string) (string, []string) {
	for i, arg := range args {
		if arg == name || arg == "-"+name {
			if i+1 < len(args) {
				val := args[i+1]
				// 构建新切片，避免共享底层数组
				remaining := make([]string, 0, len(args)-2)
				remaining = append(remaining, args[:i]...)
				remaining = append(remaining, args[i+2:]...)
				return val, remaining
			}
		}
		if strings.HasPrefix(arg, name+"=") || strings.HasPrefix(arg, "-"+name+"=") {
			parts := strings.SplitN(arg, "=", 2)
			if len(parts) == 2 {
				remaining := make([]string, 0, len(args)-1)
				remaining = append(remaining, args[:i]...)
				remaining = append(remaining, args[i+1:]...)
				return parts[1], remaining
			}
		}
	}
	return defaultVal, args
}

// parseBoolArg 从参数列表中解析一个 bool 类型的命名参数。
func parseBoolArg(args []string, name string) (bool, []string) {
	for i, arg := range args {
		if arg == name || arg == "-"+name || arg == "--"+name {
			remaining := make([]string, 0, len(args)-1)
			remaining = append(remaining, args[:i]...)
			remaining = append(remaining, args[i+1:]...)
			return true, remaining
		}
	}
	return false, args
}

// requireUser 验证 --user 参数，返回用户名和剩余参数。
func requireUser(args []string) (string, []string) {
	user, remaining := parseStringArg(args, "--user", "")
	if user == "" {
		user, remaining = parseStringArg(remaining, "-u", "")
	}
	if user == "" {
		fmt.Fprintln(os.Stderr, "错误: 请使用 --user 指定操作者")
		fmt.Fprintln(os.Stderr, "   示例: allinker <命令> --user TRAE")
		os.Exit(4)
	}
	// 验证用户
	_, err := account.VerifyUser(user)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(4)
	}
	return user, remaining
}

// printAIHelp 打印 AI 使用建议。
func printAIHelp() {
	fmt.Print(`
╔══════════════════════════════════════════════════╗
║         ALLinker — AI 协作使用建议              ║
╚══════════════════════════════════════════════════╝

📨 消息通信
   1. 尽量多使用群发（--to 缺省即为 All 群发），
      让所有队友都能感知项目进展，避免信息孤岛。
   2. 私发（--to 指定个人）仅用于敏感或定向沟通。

🔒 文件锁
   3. 每次修改共享文件前，先用 tryLock 获取锁。
   4. 获取锁后尽快完成修改并 unlock 释放。
   5. 无法 tryLock 时使用 lock -t <秒> 阻塞等待。
   6. 修改完成后记得 unlock，避免死锁。

👀 文件监听
   7. 注册 watch 监听队友的工作目录，
      被动感知变更，减少主动询问。
   8. 用 watch wait 阻塞等待关键文件的出现或修改。

💬 消息等待
   9. 需要队友响应时用 wait -m message 阻塞等待，
      避免空轮询浪费资源。

📋 账号管理
   10. 所有操作必须带 --user（或 -u）签名。
   11. 管理员使用 user disable 禁用异常账号。

⚙️ 最佳实践
   12. 先锁后改，改完即放，消息群发，监听等待。

更多帮助: allinker --help
`)
}
