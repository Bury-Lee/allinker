// Package logutil 提供基于每日轮转文件的日志记录抽象层。
//
// 日志写入 data-dir/Logs/YYYY-MM-DD.log 文件，
// 自动按天轮转，支持多级别日志与结构化审计记录。
package logutil

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Level 表示日志级别。
type Level int

const (
	LevelInfo  Level = iota // 一般信息
	LevelWarn               // 警告
	LevelError              // 错误
	LevelAudit              // 审计操作
)

func (l Level) String() string {
	switch l {
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	case LevelAudit:
		return "AUDIT"
	default:
		return "UNKNOWN"
	}
}

// Logger 是每日轮转的日志记录器。
type Logger struct {
	mu       sync.Mutex
	root     string         // 数据根目录（.alf）
	logDir   string         // Logs 子目录
	file     *os.File       // 当前打开的日志文件
	curDate  string         // 当前文件日期（YYYY-MM-DD）
	enabled  bool
}

// Global 是全局日志记录器实例。
var Global *Logger

// Init 初始化日志系统，创建 Logs 目录。
// root 为数据根目录（如 .alf）。
func Init(root string) error {
	logDir := filepath.Join(root, "Logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("创建 Logs 目录失败: %w", err)
	}

	Global = &Logger{
		root:    root,
		logDir:  logDir,
		enabled: true,
	}
	return nil
}

// Close 关闭当前日志文件。
func Close() {
	if Global != nil {
		Global.mu.Lock()
		defer Global.mu.Unlock()
		if Global.file != nil {
			Global.file.Close()
			Global.file = nil
		}
	}
}

// LogDir 返回 Logs 目录路径。
func LogDir() string {
	if Global == nil {
		return ""
	}
	return Global.logDir
}

// Info 记录一条 INFO 级别日志。
func Info(format string, args ...any) {
	write(LevelInfo, format, args...)
}

// Warn 记录一条 WARN 级别日志。
func Warn(format string, args ...any) {
	write(LevelWarn, format, args...)
}

// Error 记录一条 ERROR 级别日志。
func Error(format string, args ...any) {
	write(LevelError, format, args...)
}

// auditEntry 审计日志内部结构。
type auditEntry struct {
	Time   string `json:"time"`
	User   string `json:"user"`
	Action string `json:"action"`
	Target string `json:"target"`
	Result string `json:"result"`
	Detail string `json:"detail"`
}

// Audit 记录一条审计日志（结构化 JSON 行）。
func Audit(user, action, target, result, detail string) {
	if Global == nil || !Global.enabled {
		return
	}
	entry := auditEntry{
		Time:   time.Now().UTC().Format(time.RFC3339),
		User:   user,
		Action: action,
		Target: target,
		Result: result,
		Detail: detail,
	}
	data, _ := json.Marshal(entry)
	writeRaw(time.Now(), "AUDIT", string(data))
}

// write 写入一条格式化的日志。
func write(level Level, format string, args ...any) {
	if Global == nil || !Global.enabled {
		return
	}
	msg := fmt.Sprintf(format, args...)
	now := time.Now()
	line := fmt.Sprintf("[%s] %s: %s", level.String(), now.Format("15:04:05.000"), msg)
	writeRaw(now, level.String(), line)
}

// writeRaw 将原始行写入日志文件。
func writeRaw(now time.Time, level, line string) {
	Global.mu.Lock()
	defer Global.mu.Unlock()

	dateStr := now.Format("2006-01-02")
	// 检查是否需要轮转
	if Global.file == nil || Global.curDate != dateStr {
		Global.rotate(now)
	}

	if Global.file != nil {
		fmt.Fprintln(Global.file, line)
	}
}

// rotate 关闭旧文件，打开新日期的日志文件。
func (l *Logger) rotate(now time.Time) {
	if l.file != nil {
		l.file.Close()
		l.file = nil
	}

	dateStr := now.Format("2006-01-02")
	l.curDate = dateStr
	logPath := filepath.Join(l.logDir, dateStr+".log")

	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "打开日志文件失败: %v\n", err)
		return
	}
	l.file = file
}
