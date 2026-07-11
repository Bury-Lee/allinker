package main

import (
	"fmt"
	"os"
	"path/filepath"

	"allinker/cli"
	initt "allinker/init"
	"allinker/lock"
	"allinker/logutil"
)

func main() {
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
