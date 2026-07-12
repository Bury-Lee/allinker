// cli_wait.go - wait 命令处理
// 支持等待文件出现或等待其他用户发来的消息。

package cli

import (
	"fmt"
	"os"

	waitpkg "allinker/wait"
)

// handleWait 处理 wait 命令
//
// 用法:
//
//	allinker wait -m file     -d <目录> -f <模式> [-t <超时>] [--watch-mode appear|modify] [--quiet] [--print-content]
//	allinker wait -m message  [--from <发送者>] [-t <超时>]
//
// 默认模式为 file（当提供了 -d 和 -f 时自动推断）。
func handleWait(args []string, humanMode bool) {
	// 解析模式（file / message）
	mode, remaining := parseStringArg(args, "-m", "")
	if mode == "" {
		mode, remaining = parseStringArg(remaining, "--mode", "")
	}

	// 如果未显式指定模式，尝试自动推断
	if mode == "" {
		// 检查是否有 --from 参数 → 消息模式
		fromVal, r := parseStringArg(args, "--from", "")
		if fromVal != "" || len(r) < len(args) {
			mode = "message"
			remaining = args // 重新解析，因为上面已消耗了参数
		}
	}

	switch mode {
	case "message":
		handleWaitMessage(remaining)
	default:
		handleWaitFile(remaining)
	}
}

// handleWaitFile 处理文件等待模式。
func handleWaitFile(args []string) {
	dir, remaining := parseStringArg(args, "-d", "")
	if dir == "" {
		dir, remaining = parseStringArg(remaining, "--dir", "")
	}
	pattern, remaining := parseStringArg(remaining, "-f", "")
	if pattern == "" {
		pattern, remaining = parseStringArg(remaining, "--file", "")
	}
	if pattern == "" {
		pattern, remaining = parseStringArg(remaining, "--pattern", "")
	}
	timeout, remaining := parseIntArg(remaining, "-t", 60)
	watchMode, remaining := parseStringArg(remaining, "--watch-mode", "appear")
	quiet, _ := parseBoolArg(remaining, "--quiet")
	printContent, _ := parseBoolArg(remaining, "--print-content")

	if dir == "" || pattern == "" {
		fmt.Fprintln(os.Stderr, "错误：请使用 -d <目录> 和 -f <模式> 指定等待的文件")
		fmt.Fprintln(os.Stderr, "示例: allinker wait -d ./inbox -f RESP_*.md -t 120")
		os.Exit(1)
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
			fmt.Printf("错误: %v\n", err)
			fmt.Printf("   建议: 检查目标 Agent 是否已输出文件\n")
		}
		ExitCode = 2
		return
	}

	if !quiet {
		fmt.Printf("已检测到文件: %s (等待耗时: %d秒)\n", matchedFile, elapsed)
	} else {
		fmt.Println(matchedFile)
	}
}

// handleWaitMessage 处理消息等待模式。
func handleWaitMessage(args []string) {
	from, remaining := parseStringArg(args, "--from", "")
	timeout, _ := parseIntArg(remaining, "-t", 60)

	if from != "" {
		fmt.Printf("正在等待来自 %s 的指令（超时: %d秒）...\n", from, timeout)
	} else {
		fmt.Printf("正在等待其他用户发来的指令（超时: %d秒）...\n", timeout)
	}

	content, elapsed, err := waitpkg.WaitForMessage(from, timeout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		ExitCode = 2
		return
	}

	fmt.Printf("%s\n", content)
	fmt.Printf("  (等待耗时: %d秒)\n", elapsed)
}
