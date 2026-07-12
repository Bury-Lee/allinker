package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// ParseArgs 将原始命令行参数解析为 *CommandArg。
// 三步走的第一步：原始 []string → 统一结构体。
func ParseArgs(args []string, humanMode bool) *CommandArg {
	cmd := &CommandArg{HumanMode: humanMode}
	if len(args) == 0 {
		return cmd
	}

	// 识别一级命令
	cmd.Command = args[0]
	rest := args[1:]

	// 识别二级命令（watch add, user list, set server 等）
	switch cmd.Command {
	case "watch", "user", "set":
		if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
			cmd.SubCommand = rest[0]
			rest = rest[1:]
		}
	}

	// 剩余参数全部按 --key value 或 --key=value 或 -k value 解析
	for i := 0; i < len(rest); i++ {
		arg := rest[i]

		if !strings.HasPrefix(arg, "-") {
			continue // 不是参数名，跳过
		}

		// 去掉前缀 - 或 --
		name := strings.TrimLeft(arg, "-")

		// 处理 --key=value 格式
		if idx := strings.Index(name, "="); idx >= 0 {
			val := name[idx+1:]
			name = name[:idx]
			setField(cmd, name, val)
			continue
		}

		// 短参映射到长参名
		name = expandShortArg(name)

		// 判断下一个 token 是值还是下一个参数
		if i+1 < len(rest) && !strings.HasPrefix(rest[i+1], "-") {
			// 有值参数
			setField(cmd, name, rest[i+1])
			i++
		} else {
			// bool 标志
			setField(cmd, name, "true")
		}
	}

	return cmd
}

// ExecuteParsed 根据 *CommandArg 执行对应命令。
// 三步走的第三步：统一结构体 → 业务逻辑。
// Server 模式下调用此函数前应先做指令过滤。
func ExecuteParsed(cmd *CommandArg) {
	handleErr := func(err error) {
		if err != nil {
			var cliErr *CLIError
			if errors.As(err, &cliErr) {
				if cliErr.Msg != "" {
					fmt.Fprintln(os.Stderr, cliErr.Msg)
				}
				os.Exit(cliErr.Code)
			}
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	}

	switch cmd.Command {
	case "register":
		handleErr(handleRegister(cmd))
	case "lock":
		handleErr(handleLock(cmd))
	case "tryLock":
		handleErr(handleTryLock(cmd))
	case "unlock":
		handleErr(handleUnlock(cmd))
	case "status":
		handleErr(handleLockStatus(cmd))
	case "watch":
		handleErr(handleWatch(cmd))
	case "wait":
		handleErr(handleWait(cmd))
	case "send":
		handleErr(handleSend(cmd))
	case "recv":
		handleErr(handleRecv(cmd))
	case "history":
		handleErr(handleHistory(cmd))
	case "user":
		handleErr(handleUser(cmd))
	case "set":
		handleErr(handleSet(cmd))
	case "fix":
		handleErr(handleFix(cmd))
	default:
		if cmd.Command != "" {
			handleErr(&CLIError{Code: 1, Msg: fmt.Sprintf(" 未知命令: %s\n   使用 allinker --help 查看可用命令", cmd.Command)})
		}
	}
	os.Exit(0)
}

// ExecuteParsedResult 与 ExecuteParsed 相同但返回 cmdResult，用于 Server 模式。
func ExecuteParsedResult(cmd *CommandArg) cmdResult {
	if cmd.Command == "wait" {
		return cmdResult{Error: "wait 命令不支持远程执行", ExitCode: 2}
	}

	var err error
	switch cmd.Command {
	case "register":
		err = handleRegister(cmd)
	case "lock":
		err = handleLock(cmd)
	case "tryLock":
		err = handleTryLock(cmd)
	case "unlock":
		err = handleUnlock(cmd)
	case "status":
		err = handleLockStatus(cmd)
	case "send":
		err = handleSend(cmd)
	case "recv":
		err = handleRecv(cmd)
	case "history":
		err = handleHistory(cmd)
	case "user":
		err = handleUser(cmd)
	case "watch":
		err = handleWatch(cmd)
	case "set":
		err = handleSet(cmd)
	case "fix":
		err = handleFix(cmd)
	default:
		return cmdResult{Error: fmt.Sprintf("未知命令: %s", cmd.Command), ExitCode: 1}
	}

	if err != nil {
		exitCode := 1
		if cmd.Command == "lock" || cmd.Command == "tryLock" {
			exitCode = 3
		}
		return cmdResult{Error: err.Error(), ExitCode: exitCode}
	}
	return cmdResult{Success: true, Data: "操作成功"}
}

// expandShortArg 将短参名映射为长参名。
func expandShortArg(name string) string {
	m := map[string]string{
		"f": "file",
		"t": "timeout",
		"u": "user",
		"d": "dir",
		"p": "pattern",
		"m": "mode",
		"n": "name",
		"r": "remote",
	}
	if long, ok := m[name]; ok {
		return long
	}
	return name
}

// setField 根据参数名设置 CommandArg 的对应字段。
func setField(cmd *CommandArg, name, value string) {
	switch name {
	// 字符串字段
	case "file":
		cmd.File = value
	case "user":
		cmd.User = value
	case "name":
		cmd.Name = value
	case "role":
		cmd.Role = value
	case "desc":
		cmd.Desc = value
	case "msg", "content":
		cmd.Msg = value
	case "to", "at":
		cmd.To = value
	case "from":
		cmd.From = value
	case "id":
		cmd.MsgID = value
	case "with":
		cmd.With = value
	case "dir":
		cmd.Dir = value
	case "pattern":
		cmd.Pattern = value
	case "mode":
		cmd.Mode = value
	case "watch-mode":
		cmd.WatchMode = value
	case "host":
		cmd.Host = value
	case "token":
		cmd.Token = value
	case "addr":
		cmd.Addr = value
	case "reason":
		cmd.Reason = value
	case "since":
		cmd.Since = value
	case "type":
		cmd.ActionType = value
	case "remote":
		cmd.Name = value // -r 远程连接名 → Name 字段

	// 整数字段
	case "timeout":
		cmd.Timeout = parseIntOrZero(value)
	case "port":
		cmd.Port = parseIntOrZero(value)
	case "limit":
		cmd.Limit = parseIntOrZero(value)

	// bool 字段
	case "all":
		cmd.All = true
	case "quiet":
		cmd.Quiet = true
	case "print-content":
		cmd.PrintContent = true
	case "dry-run":
		cmd.DryRun = true
	case "check":
		cmd.Check = true
	case "delete":
		cmd.Delete = true
	case "list":
		cmd.List = true
	}
}

func parseIntOrZero(s string) int {
	var v int
	if _, err := fmt.Sscanf(s, "%d", &v); err == nil {
		return v
	}
	return 0
}
