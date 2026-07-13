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

	"allinker/config"
	initt "allinker/init"
)

// Version 是工具的版本号，编译时可通过 -ldflags 注入。
var Version = "1.0.0"

// ExitCode 存储命令的退出码。
var ExitCode = 0

// CLIError 携带退出码的错误类型，供顶层统一处理。
type CLIError struct {
	Code int
	Msg  string
}

func (e *CLIError) Error() string { return e.Msg }

// runCLI 已内联到 Run() 的 switch 之后。

// Run 是 CLI 的入口函数，解析参数并执行对应命令。
func Run() {
	//TODO:同样改造成三步走
	// =========================================================================
	// 定义全局标志
	// =========================================================================
	dataDir := flag.String("data-dir", "", "数据目录路径（默认 .alf）")
	showHelp := flag.Bool("help", false, "显示帮助信息")
	showVersion := flag.Bool("version", false, "显示版本信息")

	// 服务模式标志
	serverMode := flag.Bool("server", false, "启动中心服务模式")
	serverPort := flag.Int("port", 8080, "服务端口（仅服务模式有效）")
	serverStop := flag.Bool("stop", false, "停止服务")
	serverStatus := flag.Bool("status", false, "查看服务状态")
	serverRestart := flag.Bool("restart", false, "重启服务")

	// 客户端模式标志
	serverURL := flag.String("connect", "", "连接中心服务地址（如 http://127.0.0.1:8080）")
	remoteName := flag.String("remote", "", "使用已保存的远程连接（如 -r 电脑1）")
	remoteShort := flag.String("r", "", "使用已保存的远程连接（简写）")
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
  --connect <地址>    连接远程服务（如 http://192.168.1.100:8080）
  -r <名称>           使用已保存的远程连接（如 -r 电脑1）
  --remote <名称>      同上
  --auto              自动检测本地服务，存在则连接
  --ai                输出 AI 可解析的结构化内容
  --human             人类可读输出（带 emoji/表格）
  --help              显示此帮助
  --version           显示版本号
  --AIhelp             显示 AI 使用建议

命令:

  账号管理
    register --name <用户名> [--role admin|agent|guest] [--desc <岗位描述>]
                                      注册新账号（--desc 可选的自定义描述）

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
    chat [--user <用户名>] [--interval <秒>]
                                      人类聊天室：实时查看 AI 对话（--user 可选，可参与发言）

  等待
    wait -m file -d <目录> -f <模式> [-t <秒>] [--quiet] [--print-content]
                                      等待文件出现（默认模式）
    wait -m message [--from <发送者>] --user <接收者> [-t <秒>]
                                      等待消息：查位图未读，有则立即返回（默认模式）
    wait -m message [--from <发送者>] [-t <秒>] [--newOnly]
                                      等待消息：只看等待期间到达的新消息（不查位图）

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

  配置管理
    set server [--host <地址>] [--port <端口>] [--token <令牌>]
                                  配置本地服务
    set remote --name <名称> --addr <地址> [--token <令牌>]
                                  添加远程连接
    set remote --list              列出远程连接
    set remote --name <名称> --delete
                                  删除远程连接

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
		runServerMode(*serverPort, *serverStop, *serverStatus, *serverRestart, isHuman)
		return
	}

	// =========================================================================
	// 初始化数据目录
	// =========================================================================
	if err := initDataDir(*dataDir); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

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

	remote := *remoteName
	if remote == "" {
		remote = *remoteShort
	}
	if remote != "" {
		cfg, err := config.GetConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "错误: 读取配置失败: %v\n", err)
			os.Exit(1)
		}
		rc, ok := cfg.Remotes[remote]
		if !ok {
			fmt.Fprintf(os.Stderr, "错误: 远程连接 '%s' 未配置\n", remote)
			fmt.Fprintf(os.Stderr, "   使用 allinker set remote --name %s --addr <地址> 添加\n", remote)
			os.Exit(1)
		}
		url := fmt.Sprintf("http://%s", rc.Addr)
		if rc.Token != "" {
			os.Setenv("allinker_REMOTE_TOKEN", rc.Token)
		}
		os.Setenv("allinker_REMOTE_ACTIVE", "1")
		runClientMode("http", url, flag.Args())
		return
	}

	// =========================================================================
	// CLI 模式 — 使用 ParseArgs + ExecuteParsed 统一解析和执行
	// 三步走：[]string → ParseArgs → CommandArg → ExecuteParsed
	// =========================================================================
	args := flag.Args()
	if len(args) == 0 {
		flag.Usage()
		ExitCode = 1
		return
	}

	cmd := ParseArgs(args, isHuman)
	ExecuteParsed(cmd)
}

// initDataDir 初始化数据目录。
func initDataDir(dataDir string) error {
	absDir, err := initt.InitDataDir(dataDir)
	if err != nil {
		return fmt.Errorf("错误: 初始化数据目录失败: %v", err)
	}
	// 设置数据目录环境变量，供子命令使用
	os.Setenv("allinker_DATA_DIR", absDir)
	return nil
}

// printAIHelp 打印 AI 使用建议。
func printAIHelp() {
	fmt.Print(`
╔══════════════════════════════════════════════════════════════╗
║         allinker — AI 协作使用建议                            ║
╚══════════════════════════════════════════════════════════════╝

【第一次使用】

   1. 先注册自己
      ./allinker register --name <你的名字> --role agent [--desc <岗位描述>]

   2. 查看所有已注册的队友
      ./allinker user list --user <你的名字>

【消息通信】

   3. 尽量多使用群发（--to 缺省即为 All 群发），
      让所有队友都能感知项目进展，避免信息孤岛。

   4. 私发（--to 指定个人）仅用于敏感或定向沟通。

【文件锁】

   5. 每次修改共享文件前，先用 tryLock 获取锁。
   6. 获取锁后尽快完成修改并 unlock 释放。
   7. 无法 tryLock 时使用 lock -t <秒> 阻塞等待。
   8. 修改完成后记得 unlock，避免死锁。

【文件监听】

   9. 注册 watch 监听队友的工作目录，
      被动感知变更，减少主动询问。

   10. 用 watch wait 阻塞等待关键文件的出现或修改。

【消息等待】

   11. 需要队友响应时用 wait -m message --from <发送者> --user <接收者>
       阻塞等待，避免空轮询浪费资源。有未读消息会立即返回。

【账号管理】

   12. 所有操作必须带 --user（或 -u）签名。
   13. 管理员使用 user disable 禁用异常账号。

【空闲等待 — 核心行为规范】

   14. 没有任务时，使用长等待：
      ./allinker wait -m message -t 300
      ./allinker wait -d <目录> -f "RESP_*.md" -t 300
      超时后继续重新等待，直到收到明确指令。

   15. 非必要时**不要打扰用户**。任务认领、进度同步、问题讨论
       等能通过 wait 等待消息解决的，优先在 Agent 内部完成。

   16. 只有用户明确表示"你可以直接和组长沟通"时，才能主动联系组长。
       除此之外，所有内部协商都通过消息通信完成，不经过用户。

   17. 跟进任务时首选 wait -m message 等待别人的消息，
       也可以 watch wait 等待关键文件的修改。
       尽量不要主动 pull/recv 打扰别人。

【人类聊天室】

   18. 人类可通过 chat 命令实时旁观 Agent 对话：
      ./allinker chat                        # 只读
      ./allinker chat --user <用户名>        # 可参与发言

【最佳实践】

   19. 先锁后改，改完即放，消息群发，监听等待。

更多帮助: allinker --help
`)
}
