// Package wait 提供等待文件出现和消息到达的功能。
// wait 命令会阻塞当前进程，直到条件满足或超时。
package wait

import (
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"allinker/config"
	"allinker/core"
)

// WaitForFile 等待指定目录下匹配模式的文件出现或变更。
// 参数：
//   - dir: 监听的目录
//   - pattern: 文件匹配模式（如 "RESP_*.md"）
//   - timeout: 超时秒数
//   - printContent: 检测到文件后是否打印其内容
//   - watchMode: "appear"（默认，等待新文件出现）或 "modify"（监听已有文件内容变更）
//
// 返回值：
//   - matchedFile: 匹配到的文件路径
//   - elapsed: 等待耗时（秒）
//   - err: 超时或其他错误
func WaitForFile(dir, pattern string, timeout int, printContent bool, watchMode string) (matchedFile string, elapsed int, err error) {
	if dir == "" {
		return "", 0, fmt.Errorf("目录不能为空")
	}
	if pattern == "" {
		return "", 0, fmt.Errorf("文件匹配模式不能为空")
	}
	if timeout <= 0 {
		timeout = 60 // 默认 60 秒
	}
	if watchMode == "" {
		watchMode = "modify"
	}

	startTime := time.Now()
	deadline := startTime.Add(time.Duration(timeout) * time.Second)

	if watchMode == "modify" {
		return waitForFileModify(dir, pattern, startTime, deadline, printContent)
	}

	// 默认模式：等待文件出现
	return waitForFileAppear(dir, pattern, startTime, deadline, timeout, printContent)
}

// waitForFileAppear 等待新文件出现（轮询 Glob 模式）。
func waitForFileAppear(dir, pattern string, startTime, deadline time.Time, timeout int, printContent bool) (matchedFile string, elapsed int, err error) {
	// 先检查一次
	matches, _ := filepath.Glob(filepath.Join(dir, pattern))
	if len(matches) > 0 {
		elapsed = int(time.Since(startTime).Seconds())
		matchedFile = matches[0]
		if printContent {
			printFileContent(matchedFile)
		}
		return matchedFile, elapsed, nil
	}

	for {
		if time.Now().After(deadline) {
			return "", timeout, fmt.Errorf("超时！已等待 %d秒，未检测到匹配文件", timeout)
		}

		time.Sleep(1 * time.Second)

		matches, err := filepath.Glob(filepath.Join(dir, pattern))
		if err != nil {
			return "", 0, fmt.Errorf("扫描文件失败: %w", err)
		}
		if len(matches) > 0 {
			elapsed = int(time.Since(startTime).Seconds())
			matchedFile = matches[0]
			if printContent {
				printFileContent(matchedFile)
			}
			return matchedFile, elapsed, nil
		}
	}
}

// waitForFileModify 监听现有文件的内容变更（通过 MD5 哈希对比）。
func waitForFileModify(dir, pattern string, startTime, deadline time.Time, printContent bool) (matchedFile string, elapsed int, err error) {
	// 计算初始文件哈希
	prevHashes := make(map[string]string)
	matches, _ := filepath.Glob(filepath.Join(dir, pattern))
	for _, f := range matches {
		prevHashes[f] = fileHash(f)
	}

	for {
		if time.Now().After(deadline) {
			return "", int(time.Since(startTime).Seconds()), fmt.Errorf("超时！未检测到文件变更")
		}

		time.Sleep(2 * time.Second)

		currentMatches, _ := filepath.Glob(filepath.Join(dir, pattern))
		currentHashes := make(map[string]string)

		for _, f := range currentMatches {
			h := fileHash(f)
			currentHashes[f] = h

			oldHash, existed := prevHashes[f]
			if !existed {
				// 新增文件
				elapsed = int(time.Since(startTime).Seconds())
				if printContent {
					printFileContent(f)
				}
				return f + " (新增)", elapsed, nil
			}
			if oldHash != h {
				// 文件内容变更
				elapsed = int(time.Since(startTime).Seconds())
				if printContent {
					printFileContent(f)
				}
				return f + " (修改)", elapsed, nil
			}
		}

		// 检查文件是否被删除
		for f := range prevHashes {
			if _, stillExists := currentHashes[f]; !stillExists {
				elapsed = int(time.Since(startTime).Seconds())
				return f + " (删除)", elapsed, nil
			}
		}

		prevHashes = currentHashes
	}
}

// fileHash 计算文件的 MD5 哈希值。
func fileHash(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// printFileContent 打印文件内容到标准输出。
func printFileContent(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠️ 读取文件内容失败: %v\n", err)
		return
	}
	fmt.Println("--- 文件内容 ---")
	fmt.Print(string(data))
	fmt.Println("--- 文件结束 ---")
}

// WaitForMessage 等待其他用户发来的新消息，返回期间所有新增消息。
// 它会轮询 SQLite 中的 messages 表，收集所有比当前最新 ID 更新的消息。
// 参数：
//   - who:     发送者用户名过滤（空字符串表示等待所有用户的消息）
//   - timeoutSec: 超时秒数（<=0 则从配置读取 DefaultWaitTimeout，默认 60 秒）
//
// 返回值：
//   - content: 所有新消息的内容拼接（每条一行）
//   - elapsed: 等待耗时（秒）
//   - err: 超时或其他错误
func WaitForMessage(who string, timeoutSec int) (content string, elapsed int, err error) {
	cfg, cfgErr := config.GetConfig()
	if timeoutSec <= 0 {
		timeoutSec = 60
		if cfgErr == nil && cfg.DefaultWaitTimeout > 0 {
			timeoutSec = cfg.DefaultWaitTimeout
		}
	}

	startTime := time.Now()
	deadline := startTime.Add(time.Duration(timeoutSec) * time.Second)

	// 先获取当前最新消息的 ID，后续只查询比它更新的消息
	var lastID int64
	core.DB.Table("messages").Select("COALESCE(MAX(id), 0)").Scan(&lastID)

	for {
		if time.Now().After(deadline) {
			return "", timeoutSec, fmt.Errorf("超时！已等待 %d秒，未收到新消息", timeoutSec)
		}

		// 查询比 lastID 更新的消息（可按发送者过滤）
		type msgRow struct {
			ID         int64
			SenderName string
			Content    string
		}
		var msgs []msgRow
		query := core.DB.Table("messages").
			Select("id, sender_name, content").
			Where("id > ?", lastID)
		if who != "" {
			query = query.Where("sender_name = ?", who)
		}
		query.Order("id ASC").Find(&msgs)

		if len(msgs) > 0 {
			elapsed = int(time.Since(startTime).Seconds())
			// 收集所有新消息并拼接返回
			var parts []string
			for _, msg := range msgs {
				parts = append(parts, fmt.Sprintf("%s: %s", msg.SenderName, msg.Content))
			}
			return strings.Join(parts, "\n"), elapsed, nil
		}

		time.Sleep(1 * time.Second)
	}
}
