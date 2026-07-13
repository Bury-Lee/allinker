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

// CommandArg 是所有 CLI 命令的统一参数结构体。
// ParseArgs 将 []string 解析为 CommandArg，各 handler 直接读取字段。
// JSON 标签用于 Client/Server 模式之间的序列化传输。
//
// 字段命名规则：
//   - 业务参数（file/user/name等）：对应 --xxx 命令参数
//   - 全局动作（ServerMode/RemoteName等）：对应 -server/-r 等全局动作，
//     在 CLI 模式由 flag 包处理，在 Server 模式通过过滤拒绝嵌套执行。
type CommandArg struct {
	// 命令标识
	Command    string `json:"command"`              // 一级命令：lock, send, register, watch, user, set, wait, fix, history, status, recv
	SubCommand string `json:"subCommand,omitempty"` // 二级命令
	HumanMode  bool   `json:"humanMode"`            // 输出模式

	// === 全局动作（对应 -server/-stop/-restart/-r 等全局 flag）===
	// CLI 模式下这些由 flag.Parse() 消费后走 runServerMode/runClientMode
	// Server 模式下如果在 CommandArg 中出现（来自恶意或错误客户端），ExecuteParsedResult 会直接拒绝
	ServerMode   bool   `json:"server,omitempty"`   // -server：启动服务
	ServerStop   bool   `json:"stop,omitempty"`     // -stop：停止服务
	ServerStatus bool   `json:"status,omitempty"`   // -status：查看服务状态（与 lock status 共享此标志）
	ServerRestart bool  `json:"restart,omitempty"`  // -restart：重启服务
	RemoteName   string `json:"remote,omitempty"`   // -r <名称> / --remote <名称>：远程连接

	// === 文件锁 -f/--file, -t/--timeout ===
	File    string `json:"file,omitempty"`
	Timeout int    `json:"timeout,omitempty"`

	// === 用户身份 --user/-u ===
	User string `json:"user,omitempty"`

	// === 注册 --name, --role, --desc ===
	Name string `json:"name,omitempty"` // 用户名 / watch 名 / remote 名 / user 目标
	Role string `json:"role,omitempty"`
	Desc string `json:"desc,omitempty"`

	// === 消息 --msg, --to/--at, --from, --id, --with, --limit ===
	Msg   string `json:"msg,omitempty"`
	To    string `json:"to,omitempty"`
	From  string `json:"from,omitempty"`
	MsgID string `json:"msgId,omitempty"`
	With  string `json:"with,omitempty"`
	Limit int    `json:"limit,omitempty"`

	// === 通用 bool 标志 ===
	All          bool `json:"all,omitempty"`
	Delete       bool `json:"delete,omitempty"`
	List         bool `json:"list,omitempty"`
	DryRun       bool `json:"dryRun,omitempty"`
	Check        bool `json:"check,omitempty"`
	Quiet        bool `json:"quiet,omitempty"`
	PrintContent bool `json:"printContent,omitempty"`
	NewOnly      bool `json:"newOnly,omitempty"`

	// === 等待/监听位 -d/--dir, -p/--pattern, -m/--mode, --watch-mode ===
	Dir       string `json:"dir,omitempty"`
	Pattern   string `json:"pattern,omitempty"`
	Mode      string `json:"mode,omitempty"`
	WatchMode string `json:"watchMode,omitempty"`

	// === 用户管理 --reason, --since, --type ===
	Reason     string `json:"reason,omitempty"`
	Since      string `json:"since,omitempty"`
	ActionType string `json:"actionType,omitempty"`

	// === 服务配置 --host, --port, --token, --addr ===
	Host  string `json:"host,omitempty"`
	Port  int    `json:"port,omitempty"`
	Token string `json:"token,omitempty"`
	Addr  string `json:"addr,omitempty"`
}
