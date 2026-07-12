// cli_watch.go - watch 子命令处理

package cli

import (
	"fmt"

	waitpkg "allinker/wait"
	watchpkg "allinker/watch"
)

// handleWatch 处理 watch 子命令
func handleWatch(cmd *CommandArg) error {
	switch cmd.SubCommand {
	case "add":
		return handleWatchAdd(cmd)
	case "list":
		return handleWatchList(cmd)
	case "check":
		return handleWatchCheck(cmd)
	case "remove":
		return handleWatchRemove(cmd)
	case "clear":
		return handleWatchClear(cmd)
	case "wait":
		return handleWatchWait(cmd)
	default:
		return &CLIError{Code: 1, Msg: fmt.Sprintf("错误: 未知 watch 子命令: %s\n   可用: add, list, check, remove, clear, wait", cmd.SubCommand)}
	}
}

// handleWatchAdd 注册监听位
func handleWatchAdd(cmd *CommandArg) error {
	name := cmd.Name
	dir := cmd.Dir
	pattern := cmd.Pattern
	username := cmd.User

	if username == "" {
		return &CLIError{Code: 4, Msg: "错误: 请使用 --user 指定操作者"}
	}
	if name == "" || dir == "" || pattern == "" {
		return &CLIError{Code: 1, Msg: "错误: 请指定 --name, -d <目录>, -p <模式>"}
	}

	item, err := watchpkg.HandleWatchAdd(watchpkg.WatchAddParams{
		Name:    name,
		Dir:     dir,
		Pattern: pattern,
		Creator: username,
	})
	if err != nil {
		return &CLIError{Code: 1, Msg: fmt.Sprintf("错误: 注册监听位失败: %v", err)}
	}

	count := watchpkg.CountMatchingFiles(dir, pattern)
	if cmd.HumanMode {
		fmt.Printf("监听位已注册: %q (创建者: %s)\n", item.Name, item.Creator)
		fmt.Printf("   目录: %s\n", item.Dir)
		fmt.Printf("   模式: %s\n", item.Pattern)
		fmt.Printf("   当前匹配文件: %d个\n", count)
	} else {
		fmt.Printf("监听位已注册: %s\n", item.Name)
	}
	return nil
}

// handleWatchList 查看监听位
func handleWatchList(cmd *CommandArg) error {
	name := cmd.Name

	watches, err := watchpkg.ListWatches(name)
	if err != nil {
		return &CLIError{Code: 1, Msg: fmt.Sprintf("错误: 查询监听位失败: %v", err)}
	}

	if len(watches) == 0 {
		if name != "" {
			fmt.Printf("监听位 %q 不存在\n", name)
		} else {
			fmt.Println("没有注册的监听位")
		}
		return nil
	}

	if cmd.HumanMode {
		fmt.Printf("监听位列表 (共%d个):\n\n", len(watches))
		for _, w := range watches {
			fmt.Printf("  名称: %s\n", w.Name)
			fmt.Printf("  创建者: %s\n", w.Creator)
			fmt.Printf("  目录: %s\n", w.Dir)
			fmt.Printf("  模式: %s\n", w.Pattern)
			fmt.Printf("  状态: 正常\n")
			fmt.Printf("  最后变更: %s\n", w.LastChange)
			fmt.Println()
		}
	} else {
		for _, w := range watches {
			fmt.Printf("%s (创建者: %s, 目录: %s, 模式: %s)\n",
				w.Name, w.Creator, w.Dir, w.Pattern)
		}
	}
	return nil
}

// handleWatchCheck 检查监听位变化
func handleWatchCheck(cmd *CommandArg) error {
	name := cmd.Name

	if name == "" {
		allWatches, err := watchpkg.ListWatches("")
		if err != nil {
			return &CLIError{Code: 1, Msg: fmt.Sprintf("错误: 获取监听位列表失败: %v", err)}
		}
		if len(allWatches) == 0 {
			if cmd.HumanMode {
				fmt.Println("没有注册的监听位")
			} else {
				fmt.Println("没有监听位")
			}
			return nil
		}

		hasChanges := false
		for _, w := range allWatches {
			newFiles, err := watchpkg.CheckWatch(w.Name)
			if err != nil {
				continue
			}
			if len(newFiles) > 0 {
				hasChanges = true
				if cmd.HumanMode {
					fmt.Printf("监听位 %q 检测到 %d 个变更:\n", w.Name, len(newFiles))
					for _, f := range newFiles {
						fmt.Printf("   - %s\n", f)
					}
				} else {
					fmt.Printf("%s: %d 个变更\n", w.Name, len(newFiles))
				}
			}
		}
		if !hasChanges {
			if cmd.HumanMode {
				fmt.Println("所有监听位均无变化")
			} else {
				fmt.Println("无变化")
			}
		}
		return nil
	}

	newFiles, err := watchpkg.HandleWatchCheck(watchpkg.WatchCheckParams{Name: name})
	if err != nil {
		return &CLIError{Code: 1, Msg: fmt.Sprintf("错误: 检查监听位失败: %v", err)}
	}

	if cmd.HumanMode {
		fmt.Printf("检查 %q\n", name)
		if len(newFiles) > 0 {
			fmt.Printf("检测到 %d 个新文件:\n", len(newFiles))
			for _, f := range newFiles {
				fmt.Printf("   - %s (新文件)\n", f)
			}
		} else {
			fmt.Println("没有新文件")
		}
	} else {
		if len(newFiles) > 0 {
			fmt.Printf("%d 个新文件\n", len(newFiles))
		} else {
			fmt.Println("无变化")
		}
	}
	return nil
}

// handleWatchRemove 取消监听位
func handleWatchRemove(cmd *CommandArg) error {
	name := cmd.Name
	username := cmd.User
	if username == "" {
		return &CLIError{Code: 4, Msg: "错误: 请使用 --user 指定操作者"}
	}

	if name == "" {
		return &CLIError{Code: 1, Msg: "错误: 请使用 --name 指定监听位名称"}
	}

	err := watchpkg.HandleWatchRemove(watchpkg.WatchRemoveParams{
		Name:     name,
		Username: username,
	})
	if err != nil {
		return &CLIError{Code: 1, Msg: fmt.Sprintf("错误: 取消监听位失败: %v", err)}
	}

	fmt.Printf("监听位已取消: %s\n", name)
	return nil
}

// handleWatchWait 阻塞等待监听位的文件变化。
func handleWatchWait(cmd *CommandArg) error {
	name := cmd.Name
	timeout := cmd.Timeout
	if timeout == 0 {
		timeout = 60
	}

	if name == "" {
		return &CLIError{Code: 1, Msg: "错误: 请使用 --name 指定监听位名称"}
	}

	watches, err := watchpkg.ListWatches(name)
	if err != nil {
		return &CLIError{Code: 1, Msg: fmt.Sprintf("错误: 查询监听位失败: %v", err)}
	}
	if len(watches) == 0 {
		return &CLIError{Code: 1, Msg: fmt.Sprintf("错误: 监听位 %q 不存在", name)}
	}

	watch := watches[0]
	if cmd.HumanMode {
		fmt.Printf("正在监听 %q 的文件变更 (%s/%s, 超时: %d秒)...\n", name, watch.Dir, watch.Pattern, timeout)
	} else {
		fmt.Printf("正在监听 %s/%s (超时: %d秒)\n", watch.Dir, watch.Pattern, timeout)
	}

	matchedFile, elapsed, err := waitpkg.WaitForFile(watch.Dir, watch.Pattern, timeout, false, "modify")
	if err != nil {
		return &CLIError{Code: 2, Msg: fmt.Sprintf("错误: %v", err)}
	}

	if cmd.HumanMode {
		fmt.Printf("检测到文件变更: %s (等待耗时: %d秒)\n", matchedFile, elapsed)
	} else {
		fmt.Printf("%s (等待%d秒)\n", matchedFile, elapsed)
	}
	return nil
}

func handleWatchClear(cmd *CommandArg) error {
	username := cmd.User
	if username == "" {
		return &CLIError{Code: 4, Msg: "错误: 请使用 --user 指定操作者"}
	}

	err := watchpkg.ClearWatches(username)
	if err != nil {
		return &CLIError{Code: 1, Msg: fmt.Sprintf("错误: 清空监听位失败: %v", err)}
	}

	fmt.Printf("已清除 %s 的所有监听位\n", username)
	return nil
}
