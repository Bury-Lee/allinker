// cli_wait.go - wait 命令处理
// 支持等待文件出现或等待其他用户发来的消息。

package cli

import (
	"fmt"
	"time"

	msgpkg "allinker/message"
	waitpkg "allinker/wait"
)

// handleWait 处理 wait 命令
func handleWait(cmd *CommandArg) error {
	mode := cmd.Mode
	if mode == "" {
		mode = "file"
	}

	switch mode {
	case "message":
		return handleWaitMessage(cmd)
	default:
		return handleWaitFile(cmd)
	}
}

// handleWaitFile 处理文件等待模式。
func handleWaitFile(cmd *CommandArg) error {
	dir := cmd.Dir
	pattern := cmd.Pattern
	if pattern == "" {
		pattern = cmd.File
	}
	timeout := cmd.Timeout
	if timeout == 0 {
		timeout = 60
	}
	watchMode := cmd.WatchMode
	if watchMode == "" {
		watchMode = "appear"
	}
	quiet := cmd.Quiet
	printContent := cmd.PrintContent

	if dir == "" || pattern == "" {
		return &CLIError{Code: 1, Msg: "错误：请使用 -d <目录> 和 -f <模式> 指定等待的文件\n示例: allinker wait -d ./inbox -f RESP_*.md -t 120"}
	}

	if !quiet {
		modeDesc := "等待新文件出现"
		if watchMode == "modify" {
			modeDesc = "监听文件变更（增删改）"
		}
		fmt.Printf("正在%s: %s/%s (超时: %d秒)\n", modeDesc, dir, pattern, timeout)
	}

	matchedFile, elapsed, err := waitpkg.WaitForFile(dir, pattern, timeout, printContent, watchMode)
	if err != nil {
		if !quiet {
			return &CLIError{Code: 2, Msg: fmt.Sprintf("错误: %v\n   建议: 检查目标 Agent 是否已输出文件", err)}
		}
		return &CLIError{Code: 2}
	}

	if !quiet {
		fmt.Printf("已检测到文件: %s (等待耗时: %d秒)\n", matchedFile, elapsed)
	} else {
		fmt.Println(matchedFile)
	}
	return nil
}

// handleWaitMessage 处理消息等待模式。
//
// 默认行为（推荐）：需要 --user 指明等待者，查位图判断未读消息。
//   有未读 → 立即返回；无未读 → 轮询直到新消息到达或超时。
// --newOnly（不推荐）：跳过位图，只看等待期间 ID 增量范围内的新消息。
func handleWaitMessage(cmd *CommandArg) error {
	from := cmd.From
	username := cmd.User
	timeout := cmd.Timeout
	if timeout <= 0 {
		timeout = 60
	}

	// === --newOnly 模式：跳过位图，使用旧版 ID 增量轮询 ===
	if cmd.NewOnly {
		if from != "" {
			fmt.Printf("正在等待来自 %s 的指令（超时: %d秒）...\n", from, timeout)
		} else {
			fmt.Printf("正在等待其他用户发来的指令（超时: %d秒）...\n", timeout)
		}
		content, elapsed, err := waitpkg.WaitForMessage(from, timeout)
		if err != nil {
			return &CLIError{Code: 2, Msg: fmt.Sprintf("错误: %v", err)}
		}
		fmt.Printf("%s\n", content)
		fmt.Printf("  (等待耗时: %d秒)\n", elapsed)
		return nil
	}

	// === 默认模式：基于位图未读消息 ===
	if username == "" {
		return &CLIError{Code: 4, Msg: "错误: 请使用 --user 指定等待者\n   wait -m message --from <发送者> --user <接收者>"}
	}

	// 预检查：有没有未读消息
	msgs, err := msgpkg.HandleRecv(msgpkg.RecvParams{
		From: from,
		User: username,
		All:  false, // 只查未读
	})
	if err == nil && len(msgs) > 0 {
		for _, m := range msgs {
			fmt.Printf("%s: %s\n", m.From, m.Content)
		}
		fmt.Println("  (已有未读消息，立即返回)")
		return nil
	}

	// 没有未读消息，进入轮询等待
	if from != "" {
		fmt.Printf("正在等待来自 %s 的指令（超时: %d秒）...\n", from, timeout)
	} else {
		fmt.Printf("正在等待其他用户发来的指令（超时: %d秒）...\n", timeout)
	}

	deadline := time.Now().Add(time.Duration(timeout) * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(1 * time.Second)

		msgs, err := msgpkg.HandleRecv(msgpkg.RecvParams{
			From: from,
			User: username,
			All:  false, // 只查未读
		})
		if err != nil {
			continue
		}
		if len(msgs) > 0 {
			for _, m := range msgs {
				fmt.Printf("%s: %s\n", m.From, m.Content)
			}
			return nil
		}
	}

	return &CLIError{Code: 2, Msg: fmt.Sprintf("超时！已等待 %d秒，未收到新消息", timeout)}
}
