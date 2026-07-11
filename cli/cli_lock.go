// cli_lock.go — lock / tryLock / unlock / status 命令处理

package cli

import (
	"fmt"
	"os"
	"time"

	"allinker/config"
	lockpkg "allinker/lock"
	"allinker/model"
)

// handleLock 处理 lock 命令（阻塞等待模式）
// 用法: allinker lock -f <文件> -t <秒> --user <用户名>
func handleLock(args []string, humanMode bool) {
	filename, remaining := parseStringArg(args, "-f", "")
	if filename == "" {
		filename, remaining = parseStringArg(remaining, "--file", "")
	}
	if filename == "" {
		fmt.Fprintln(os.Stderr, "错误: 请使用 -f 或 --file 指定目标文件")
		os.Exit(1)
	}

	timeout, remaining := parseIntArg(remaining, "-t", 60)
	username, _ := requireUser(remaining)

	// 将超时秒数转换为截止时间
	deadline := time.Now().Add(time.Duration(timeout) * time.Second)

	if humanMode {
		fmt.Printf("正在获取锁: %s (操作者: %s)\n", filename, username)
	}

	err := lockpkg.AcquireLock(filename, username, deadline)
	if err != nil {
		if humanMode {
			fmt.Printf("错误: 锁获取失败: %v\n", err)
		}
		ExitCode = 3
		return
	}

	// 写入审计日志
	config.AppendAuditLog(model.AuditEntry{
		Time:   time.Now().UTC().Format(time.RFC3339),
		User:   username,
		Action: "lock",
		Target: filename,
		Result: "success",
		Detail: fmt.Sprintf("持有者: %s", username),
	})

	info := lockpkg.GetLockInfo(filename)
	if humanMode {
		if info != nil {
			fmt.Printf("锁获取成功 (持有者: %s, 有效期至: %s)\n",
				info.Holder, info.ExpiresAt.Format("15:04:05"))
		} else {
			fmt.Println("锁获取成功")
		}
	} else {
		fmt.Println("锁获取成功")
	}
}

// handleTryLock 处理 tryLock 命令（立即返回模式）
// 用法: allinker tryLock -f <文件> --user <用户名>
func handleTryLock(args []string, humanMode bool) {
	filename, remaining := parseStringArg(args, "-f", "")
	if filename == "" {
		filename, remaining = parseStringArg(remaining, "--file", "")
	}
	if filename == "" {
		fmt.Fprintln(os.Stderr, "错误: 请使用 -f 或 --file 指定目标文件")
		os.Exit(1)
	}
	username, _ := requireUser(remaining)

	err := lockpkg.TryAcquireLock(filename, username)
	if err != nil {
		info := lockpkg.GetLockInfo(filename)
		if humanMode && info != nil && !info.IsExpired() {
			fmt.Printf("错误: 锁已被占用 (持有者: %s, 剩余时间: %d秒)\n",
				info.Holder, info.RemainingSeconds())
		} else {
			fmt.Printf("错误: 锁获取失败: %v\n", err)
		}
		ExitCode = 3
		return
	}

	config.AppendAuditLog(model.AuditEntry{
		Time:   time.Now().UTC().Format(time.RFC3339),
		User:   username,
		Action: "tryLock",
		Target: filename,
		Result: "success",
		Detail: fmt.Sprintf("持有者: %s", username),
	})

	fmt.Println("锁获取成功")
}

// handleUnlock 处理 unlock 命令
// 用法: allinker unlock -f <文件> --user <用户名>
func handleUnlock(args []string, humanMode bool) {
	filename, remaining := parseStringArg(args, "-f", "")
	if filename == "" {
		filename, remaining = parseStringArg(remaining, "--file", "")
	}
	if filename == "" {
		fmt.Fprintln(os.Stderr, "错误: 请使用 -f 或 --file 指定目标文件")
		os.Exit(1)
	}
	username, _ := requireUser(remaining)

	err := lockpkg.ReleaseLock(filename, username)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: 释放锁失败: %v\n", err)
		ExitCode = 1
		return
	}

	config.AppendAuditLog(model.AuditEntry{
		Time:   time.Now().UTC().Format(time.RFC3339),
		User:   username,
		Action: "unlock",
		Target: filename,
		Result: "success",
	})

	if humanMode {
		fmt.Printf("已释放锁: %s (持有者: %s)\n", filename, username)
	} else {
		fmt.Println("已释放锁")
	}
}

// handleLockStatus 处理 status 命令
// 用法: allinker status [-f <文件> | --all]
func handleLockStatus(args []string, humanMode bool) {
	showAll, remaining := parseBoolArg(args, "--all")
	filename, _ := parseStringArg(remaining, "-f", "")
	if filename == "" {
		filename, _ = parseStringArg(remaining, "--file", "")
	}

	if showAll {
		// 查看所有锁
		locks := lockpkg.ListLocks()
		if len(locks) == 0 {
			fmt.Println("当前没有活动的锁")
			return
		}
		fmt.Printf("当前锁列表 (共计%d个):\n", len(locks))
		for _, info := range locks {
			if info.IsExpired() {
				fmt.Printf("   [过期] %s <- %s (已过期)\n", info.Filename, info.Holder)
			} else {
				fmt.Printf("   %s <- %s (剩余%d秒)\n", info.Filename, info.Holder, info.RemainingSeconds())
			}
		}
		return
	}

	if filename == "" {
		fmt.Fprintln(os.Stderr, "错误: 请使用 -f <文件> 指定文件，或使用 --all 查看所有锁")
		os.Exit(1)
	}

	info := lockpkg.GetLockInfo(filename)
	if info == nil || info.IsExpired() {
		if info != nil {
			if humanMode {
				fmt.Printf("警告: 锁已过期 (过期时间: %s, 当前时间: %s)\n",
					info.ExpiresAt.Format(time.RFC3339), time.Now().UTC().Format(time.RFC3339))
				fmt.Println("   建议: 使用 tryLock 重新获取")
			} else {
				fmt.Println("警告: 锁已过期")
			}
		} else {
			fmt.Println("文件未被锁定")
		}
		return
	}

	if humanMode {
		fmt.Printf("%s 已被锁定\n", filename)
		fmt.Printf("   持有者: %s\n", info.Holder)
		fmt.Printf("   锁定时间: %s\n", info.Timestamp.Format(time.RFC3339))
		fmt.Printf("   过期时间: %s\n", info.ExpiresAt.Format(time.RFC3339))
		fmt.Printf("   剩余时间: %d秒\n", info.RemainingSeconds())
	} else {
		fmt.Printf("已被 %s 锁定 (剩余%d秒)\n", info.Holder, info.RemainingSeconds())
	}
}
