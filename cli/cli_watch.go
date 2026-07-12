// cli_watch.go - watch 子命令处理

package cli

import (
	"fmt"
	"os"

	waitpkg "allinker/wait"
	watchpkg "allinker/watch"
)

// TODO:待完成功能 — 后续可考虑实现后台守护进程实时监听

// handleWatch 处理 watch 子命令
// 用法:
//
//	allinker watch add --name <名称> -d <目录> -p <模式> --user <用户名>
//	allinker watch list [--name <名称>]
//	allinker watch check --name <名称>
//	allinker watch remove --name <名称> --user <用户名>
//	allinker watch clear --user <用户名>
//	allinker watch wait --name <名称> -t <超时秒数>
func handleWatch(args []string, humanMode bool) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "错误: 请指定 watch 子命令: add, list, check, remove, clear, wait")
		fmt.Fprintln(os.Stderr, "   使用 allinker --help 查看详细用法")
		os.Exit(1)
	}

	subCmd := args[0]
	subArgs := args[1:]

	switch subCmd {
	case "add":
		handleWatchAdd(subArgs, humanMode)
	case "list":
		handleWatchList(subArgs, humanMode)
	case "check":
		handleWatchCheck(subArgs, humanMode)
	case "remove":
		handleWatchRemove(subArgs, humanMode)
	case "clear":
		handleWatchClear(subArgs, humanMode)
	case "wait":
		handleWatchWait(subArgs, humanMode)
	default:
		fmt.Fprintf(os.Stderr, "错误: 未知 watch 子命令: %s\n", subCmd)
		fmt.Fprintln(os.Stderr, "   可用: add, list, check, remove, clear, wait")
		os.Exit(1)
	}
}

// handleWatchAdd 注册监听位
func handleWatchAdd(args []string, humanMode bool) {
	name, remaining := parseStringArg(args, "--name", "")
	dir, remaining := parseStringArg(remaining, "-d", "")
	if dir == "" {
		dir, remaining = parseStringArg(remaining, "--dir", "")
	}
	pattern, remaining := parseStringArg(remaining, "-p", "")
	if pattern == "" {
		pattern, remaining = parseStringArg(remaining, "--pattern", "")
	}
	username, _ := requireUser(remaining)

	if name == "" || dir == "" || pattern == "" {
		fmt.Fprintln(os.Stderr, "错误: 请指定 --name, -d <目录>, -p <模式>")
		os.Exit(1)
	}

	item, err := watchpkg.AddWatch(name, dir, pattern, username)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: 注册监听位失败: %v\n", err)
		os.Exit(1)
	}

	count := watchpkg.CountMatchingFiles(dir, pattern)
	if humanMode {
		fmt.Printf("监听位已注册: %q (创建者: %s)\n", item.Name, item.Creator)
		fmt.Printf("   目录: %s\n", item.Dir)
		fmt.Printf("   模式: %s\n", item.Pattern)
		fmt.Printf("   当前匹配文件: %d个\n", count)
	} else {
		fmt.Printf("监听位已注册: %s\n", item.Name)
	}
}

// handleWatchList 查看监听位
func handleWatchList(args []string, humanMode bool) {
	name, _ := parseStringArg(args, "--name", "")

	watches, err := watchpkg.ListWatches(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: 查询监听位失败: %v\n", err)
		os.Exit(1)
	}

	if len(watches) == 0 {
		if name != "" {
			fmt.Printf("监听位 %q 不存在\n", name)
		} else {
			fmt.Println("没有注册的监听位")
		}
		return
	}

	if humanMode {
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
}

// handleWatchCheck 检查监听位变化
func handleWatchCheck(args []string, humanMode bool) {
	name, remaining := parseStringArg(args, "--name", "")
	_ = remaining

	if name == "" {
		// 不指定名称时检查所有监听位
		allWatches, err := watchpkg.ListWatches("")
		if err != nil {
			fmt.Fprintf(os.Stderr, "错误: 获取监听位列表失败: %v\n", err)
			os.Exit(1)
		}
		if len(allWatches) == 0 {
			if humanMode {
				fmt.Println("没有注册的监听位")
			} else {
				fmt.Println("没有监听位")
			}
			return
		}

		hasChanges := false
		for _, w := range allWatches {
			newFiles, err := watchpkg.CheckWatch(w.Name)
			if err != nil {
				continue
			}
			if len(newFiles) > 0 {
				hasChanges = true
				if humanMode {
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
			if humanMode {
				fmt.Println("所有监听位均无变化")
			} else {
				fmt.Println("无变化")
			}
		}
		return
	}

	newFiles, err := watchpkg.CheckWatch(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: 检查监听位失败: %v\n", err)
		os.Exit(1)
	}

	if humanMode {
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
}

// handleWatchRemove 取消监听位
func handleWatchRemove(args []string, humanMode bool) {
	name, remaining := parseStringArg(args, "--name", "")
	username, _ := requireUser(remaining)

	if name == "" {
		fmt.Fprintln(os.Stderr, "错误: 请使用 --name 指定监听位名称")
		os.Exit(1)
	}

	err := watchpkg.RemoveWatch(name, username)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: 取消监听位失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("监听位已取消: %s\n", name)
}

// handleWatchWait 阻塞等待监听位的文件变化。
// 用法: allinker watch wait --name <名称> [-t <超时秒数>]
func handleWatchWait(args []string, humanMode bool) {
	name, remaining := parseStringArg(args, "--name", "")
	timeout, _ := parseIntArg(remaining, "-t", 60)

	if name == "" {
		fmt.Fprintln(os.Stderr, "错误: 请使用 --name 指定监听位名称")
		os.Exit(1)
	}

	// 获取监听位信息
	watches, err := watchpkg.ListWatches(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: 查询监听位失败: %v\n", err)
		os.Exit(1)
	}
	if len(watches) == 0 {
		fmt.Fprintf(os.Stderr, "错误: 监听位 %q 不存在\n", name)
		os.Exit(1)
	}

	watch := watches[0]
	if humanMode {
		fmt.Printf("正在监听 %q 的文件变更 (%s/%s, 超时: %d秒)...\n", name, watch.Dir, watch.Pattern, timeout)
	} else {
		fmt.Printf("正在监听 %s/%s (超时: %d秒)\n", watch.Dir, watch.Pattern, timeout)
	}

	matchedFile, elapsed, err := waitpkg.WaitForFile(watch.Dir, watch.Pattern, timeout, false, "modify")
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		ExitCode = 2
		return
	}

	if humanMode {
		fmt.Printf("检测到文件变更: %s (等待耗时: %d秒)\n", matchedFile, elapsed)
	} else {
		fmt.Printf("%s (等待%d秒)\n", matchedFile, elapsed)
	}
}
func handleWatchClear(args []string, humanMode bool) {
	username, _ := requireUser(args)

	err := watchpkg.ClearWatches(username)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: 清空监听位失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("已清除 %s 的所有监听位\n", username)
}
