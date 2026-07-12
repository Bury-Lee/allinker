// cli_lock.go — lock / tryLock / unlock / status 命令处理

package cli

import (
	"fmt"
	"time"

	"allinker/config"
	lockpkg "allinker/lock"
	"allinker/model"
)

// handleLock 处理 lock 命令（阻塞等待模式）
// 用法: allinker lock -f <文件> -t <秒> --user <用户名>
func handleLock(cmd *CommandArg) error {
	filename := cmd.File
	if filename == "" {
		return &CLIError{Code: 1, Msg: "错误: 请使用 -f 或 --file 指定目标文件"}
	}
	username := cmd.User
	if username == "" {
		return &CLIError{Code: 4, Msg: "错误: 请使用 --user 指定操作者"}
	}
	timeout := cmd.Timeout
	if timeout == 0 {
		timeout = 60
	}

	if cmd.HumanMode {
		fmt.Printf("正在获取锁: %s (操作者: %s)\n", filename, username)
	}

	err := lockpkg.HandleLock(lockpkg.LockParams{
		Filename: filename,
		Username: username,
		Timeout:  timeout,
	})
	if err != nil {
		if cmd.HumanMode {
			return &CLIError{Code: 3, Msg: fmt.Sprintf("错误: 锁获取失败: %v", err)}
		}
		return &CLIError{Code: 3}
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
	if cmd.HumanMode {
		if info != nil {
			fmt.Printf("锁获取成功 (持有者: %s, 有效期至: %s)\n",
				info.Holder, info.ExpiresAt.Format("15:04:05"))
		} else {
			fmt.Println("锁获取成功")
		}
	} else {
		fmt.Println("锁获取成功")
	}
	return nil
}

// handleTryLock 处理 tryLock 命令（立即返回模式）
// 用法: allinker tryLock -f <文件> --user <用户名>
func handleTryLock(cmd *CommandArg) error {
	filename := cmd.File
	if filename == "" {
		return &CLIError{Code: 1, Msg: "错误: 请使用 -f 或 --file 指定目标文件"}
	}
	username := cmd.User
	if username == "" {
		return &CLIError{Code: 4, Msg: "错误: 请使用 --user 指定操作者"}
	}

	err := lockpkg.HandleLock(lockpkg.LockParams{
		Filename: filename,
		Username: username,
		Timeout:  0, // tryLock 模式
	})
	if err != nil {
		info := lockpkg.GetLockInfo(filename)
		if cmd.HumanMode && info != nil && !info.IsExpired() {
			return &CLIError{Code: 3, Msg: fmt.Sprintf("错误: 锁已被占用 (持有者: %s, 剩余时间: %d秒)",
				info.Holder, info.RemainingSeconds())}
		}
		return &CLIError{Code: 3, Msg: fmt.Sprintf("错误: 锁获取失败: %v", err)}
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
	return nil
}

// handleUnlock 处理 unlock 命令
// 用法: allinker unlock -f <文件> --user <用户名>
func handleUnlock(cmd *CommandArg) error {
	filename := cmd.File
	if filename == "" {
		return &CLIError{Code: 1, Msg: "错误: 请使用 -f 或 --file 指定目标文件"}
	}
	username := cmd.User
	if username == "" {
		return &CLIError{Code: 4, Msg: "错误: 请使用 --user 指定操作者"}
	}

	err := lockpkg.ReleaseLock(filename, username)
	if err != nil {
		return &CLIError{Code: 1, Msg: fmt.Sprintf("错误: 释放锁失败: %v", err)}
	}

	config.AppendAuditLog(model.AuditEntry{
		Time:   time.Now().UTC().Format(time.RFC3339),
		User:   username,
		Action: "unlock",
		Target: filename,
		Result: "success",
	})

	if cmd.HumanMode {
		fmt.Printf("已释放锁: %s (持有者: %s)\n", filename, username)
	} else {
		fmt.Println("已释放锁")
	}
	return nil
}

// handleLockStatus 处理 status 命令
// 用法: allinker status [-f <文件> | --all]
func handleLockStatus(cmd *CommandArg) error {
	filename := cmd.File

	if cmd.All {
		locks := lockpkg.ListLocks()
		if len(locks) == 0 {
			fmt.Println("当前没有活动的锁")
			return nil
		}
		fmt.Printf("当前锁列表 (共计%d个):\n", len(locks))
		for _, info := range locks {
			if info.IsExpired() {
				fmt.Printf("   [过期] %s <- %s (已过期)\n", info.Filename, info.Holder)
			} else {
				fmt.Printf("   %s <- %s (剩余%d秒)\n", info.Filename, info.Holder, info.RemainingSeconds())
			}
		}
		return nil
	}

	if filename == "" {
		return &CLIError{Code: 1, Msg: "错误: 请使用 -f <文件> 指定文件，或使用 --all 查看所有锁"}
	}

	info := lockpkg.GetLockInfo(filename)
	if info == nil || info.IsExpired() {
		if info != nil {
			if cmd.HumanMode {
				fmt.Printf("警告: 锁已过期 (过期时间: %s, 当前时间: %s)\n",
					info.ExpiresAt.Format(time.RFC3339), time.Now().UTC().Format(time.RFC3339))
				fmt.Println("   建议: 使用 tryLock 重新获取")
			} else {
				fmt.Println("警告: 锁已过期")
			}
		} else {
			fmt.Println("文件未被锁定")
		}
		return nil
	}

	if cmd.HumanMode {
		fmt.Printf("%s 已被锁定\n", filename)
		fmt.Printf("   持有者: %s\n", info.Holder)
		fmt.Printf("   锁定时间: %s\n", info.Timestamp.Format(time.RFC3339))
		fmt.Printf("   过期时间: %s\n", info.ExpiresAt.Format(time.RFC3339))
		fmt.Printf("   剩余时间: %d秒\n", info.RemainingSeconds())
	} else {
		fmt.Printf("已被 %s 锁定 (剩余%d秒)\n", info.Holder, info.RemainingSeconds())
	}
	return nil
}
