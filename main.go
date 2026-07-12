// ALLinker 命令行工具入口。
//
// 启动流程：
//  1. 初始化数据目录（.alf/）+ JSON 存储
//  2. 初始化日志系统（每日 1 个日志文件）
//  3. 初始化 SQLite 数据库（消息 + 锁 + 监听位共享）
//  4. 启动过期锁清理 goroutine（不等待返回）
//  5. 将控制权交给 CLI 模块的 Run()
//
// 全局 panic recovery：捕获 CLI 模块的 panic，
// 写日志后退出码 99，避免栈打印污染 AI Agent 解析。
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"

	"allinker/cli"
	initt "allinker/init"
	"allinker/lock"
	"allinker/logutil"
)

// exitCodePanic 是 panic 时的退出码。
// 与文档中的标准退出码（0/1/2/3/4/5/6）不冲突，单独 99 标识"未捕获异常"。
const exitCodePanic = 99

func main() {
	// 全局 panic recovery（MM1 善后任务：P0 安全改进）
	// 必须在最顶部注册，否则 InitDataDir 自身 panic 时无法捕获
	defer func() {
		if r := recover(); r != nil {
			stack := string(debug.Stack())
			logutil.Error("panic: %v\n%s", r, stack)
			fmt.Fprintf(os.Stderr, "panic: %v\n", r)
			os.Exit(exitCodePanic)
		}
	}()

	// 初始化数据目录 + JSON 存储
	absDir, err := initt.InitDataDir("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "初始化数据目录失败: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志系统
	if err := logutil.Init(absDir); err != nil {
		fmt.Fprintf(os.Stderr, "初始化日志系统失败: %v\n", err)
		os.Exit(1)
	}
	defer logutil.Close()

	// 初始化 SQLite 数据库（消息 + 锁共用）
	dbPath := filepath.Join(absDir, "allinker.db")
	if err := initt.InitDB(dbPath); err != nil {
		fmt.Fprintf(os.Stderr, "初始化数据库失败: %v\n", err)
		os.Exit(1)
	}

	// 发送一次过期锁清理（不等待返回）
	lock.StartCleanup()
	// 将控制权交给 CLI 模块
	cli.Run()
}
