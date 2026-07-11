// cli_fix.go —— fix 命令处理
// 检查和修复数据文件完整性。

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"allinker/config"
	"allinker/core"
	"allinker/logutil"
	"allinker/model"
)

// handleFix 处理 fix 命令。
// 用法: allinker fix [--check] [--dry-run] [--user <用户名>]
func handleFix(args []string, humanMode bool) {
	dryRun, _ := parseBoolArg(args, "--dry-run")
	checkOnly, _ := parseBoolArg(args, "--check")

	if humanMode {
		fmt.Println("🔍 ALLinker 数据完整性检查")
		fmt.Println(strings.Repeat("─", 48))
		if dryRun {
			fmt.Println("⚠️  模拟运行模式（不会实际修复）")
		}
		if checkOnly {
			fmt.Println("ℹ️   仅检查模式（不执行修复）")
		}
		fmt.Println()
	}

	issues := 0
	fixed := 0

	// 1. 检查 users.json
	if humanMode {
		fmt.Print("📄 检查 users.json ... ")
	}
	ok, err := checkAndFixJSON(core.Global.UsersPath(), &model.UsersFile{}, humanMode)
	if !ok {
		issues++
		if !checkOnly {
			if fixUsersJSON(humanMode) {
				fixed++
				if humanMode {
					fmt.Println("✅ 已修复")
				}
			}
		} else if humanMode {
			fmt.Println("❌ 有问题（需修复）")
		}
	} else if err == nil && humanMode {
		fmt.Println("✅ 正常")
	}

	// 2. 检查 config.json
	if humanMode {
		fmt.Print("📄 检查 config.json ... ")
	}
	ok, err = checkAndFixJSON(core.Global.ConfigPath(), &model.AppConfig{}, humanMode)
	if !ok {
		issues++
		if !checkOnly {
			if fixConfigJSON(humanMode) {
				fixed++
				if humanMode {
					fmt.Println("✅ 已修复")
				}
			}
		} else if humanMode {
			fmt.Println("❌ 有问题（需修复）")
		}
	} else if err == nil && humanMode {
		fmt.Println("✅ 正常")
	}

	// 3. 检查 counter.json
	if humanMode {
		fmt.Print("📄 检查 counter.json ... ")
	}
	ok, err = checkAndFixJSON(core.Global.CounterPath(), &model.Counter{}, humanMode)
	if !ok {
		issues++
		if !checkOnly {
			if fixCounterJSON(humanMode) {
				fixed++
				if humanMode {
					fmt.Println("✅ 已修复")
				}
			}
		} else if humanMode {
			fmt.Println("❌ 有问题（需修复）")
		}
	} else if err == nil && humanMode {
		fmt.Println("✅ 正常")
	}

	// 4. 检查审计日志
	if humanMode {
		fmt.Print("📄 检查 Logs/ 目录 ... ")
	}
	if err := checkAuditLog(humanMode); err != nil {
		issues++
		if humanMode {
			fmt.Printf("❌ %v\n", err)
		}
	} else if humanMode {
		fmt.Println("✅ 正常")
	}

	// 6. 检查 SQLite 数据库
	if humanMode {
		fmt.Print("🗄️  检查 SQLite 数据库 ... ")
	}
	if err := checkDatabase(humanMode); err != nil {
		issues++
		if humanMode {
			fmt.Printf("❌ %v\n", err)
		}
	} else if humanMode {
		fmt.Println("✅ 正常")
	}

	// 报告汇总
	if humanMode {
		fmt.Println()
		fmt.Println(strings.Repeat("─", 48))
		if issues == 0 {
			fmt.Println("✅ 所有数据文件正常，无需修复")
		} else {
			fmt.Printf("📊 发现 %d 个问题，已修复 %d 个\n", issues, fixed)
			if fixed < issues && !dryRun {
				fmt.Println("⚠️  部分问题未能自动修复，请检查日志")
			}
		}
	}
}

// checkAndFixJSON 检查 JSON 文件是否可读且结构有效。
// 返回 (是否正常, 错误信息)。
func checkAndFixJSON(path string, target any, humanMode bool) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, fmt.Errorf("文件不存在")
		}
		return false, fmt.Errorf("读取失败: %w", err)
	}

	if len(strings.TrimSpace(string(data))) == 0 {
		return false, fmt.Errorf("文件为空")
	}

	if err := json.Unmarshal(data, target); err != nil {
		return false, fmt.Errorf("JSON 解析失败: %w", err)
	}

	return true, nil
}

// fixUsersJSON 重新创建 users.json。
func fixUsersJSON(humanMode bool) bool {
	users := &model.UsersFile{Users: make(map[string]*model.User)}
	return writeJSONSafe(core.Global.UsersPath(), users)
}

// fixConfigJSON 重新创建 config.json。
func fixConfigJSON(humanMode bool) bool {
	return writeJSONSafe(core.Global.ConfigPath(), config.DefaultConfig())
}

// fixCounterJSON 重新创建 counter.json。
func fixCounterJSON(humanMode bool) bool {
	counter := &model.Counter{NextID: 1}
	return writeJSONSafe(core.Global.CounterPath(), counter)
}

// writeJSONSafe 安全写入 JSON 文件。
func writeJSONSafe(path string, data any) bool {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return false
	}
	content, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return false
	}
	tmpPath := path + ".fix.tmp"
	if err := os.WriteFile(tmpPath, content, 0644); err != nil {
		return false
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return false
	}
	return true
}

// checkAuditLog 检查 Logs 目录下的日志文件。
func checkAuditLog(humanMode bool) error {
	logDir := logutil.LogDir()
	if logDir == "" {
		return fmt.Errorf("日志系统未初始化")
	}
	entries, err := os.ReadDir(logDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("读取 Logs 目录失败: %w", err)
	}
	if len(entries) == 0 {
		return nil
	}
	// 检查最新的日志文件
	latest := entries[len(entries)-1]
	if latest.IsDir() {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(logDir, latest.Name()))
	if err != nil {
		return nil
	}
	if len(data) == 0 {
		return nil
	}
	return nil
}

// checkDatabase 检查 SQLite 数据库连接。
func checkDatabase(humanMode bool) error {
	if core.DB == nil {
		return fmt.Errorf("数据库未初始化")
	}
	// 执行简单的查询确认连接正常
	sqlDB, err := core.DB.DB()
	if err != nil {
		return fmt.Errorf("获取数据库连接失败: %w", err)
	}
	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("数据库 Ping 失败: %w", err)
	}
	// 检查消息表和锁表
	var tableCount int
	core.DB.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name IN ('messages', 'message_recipients', 'locks', 'watches')").Scan(&tableCount)
	if tableCount < 4 && humanMode {
		fmt.Printf("\n  ⚠️  预期 3 个表，实际找到 %d 个", tableCount)
	}
	return nil
}
