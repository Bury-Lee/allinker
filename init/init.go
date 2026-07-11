// Package init 负责程序初始化：数据目录、JSON 存储、SQLite 数据库。
package init

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"allinker/config"
	"allinker/core"
	"allinker/lock"
	"allinker/logutil"
	"allinker/message"
	"allinker/model"
	"allinker/storage"
	"allinker/watch"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const DefaultDataDir = ".alf"

// InitDataDir 初始化数据目录及所有必需的 JSON 文件。
// 返回数据目录的绝对路径。
func InitDataDir(dataDir string) (string, error) {
	if core.Global != nil {
		return core.Global.Root(), nil // 已初始化
	}
	if dataDir == "" {
		dataDir = DefaultDataDir
	}

	absDir, err := filepath.Abs(dataDir)
	if err != nil {
		return "", fmt.Errorf("解析数据目录: %w", err)
	}

	core.Global = storage.NewStore(absDir)

	if err := core.Global.EnsureDir(absDir); err != nil {
		return "", fmt.Errorf("创建数据目录 %s: %w", absDir, err)
	}

	// 初始化 counter.json
	if !core.Global.FileExists(core.Global.CounterPath()) {
		counter := model.Counter{NextID: 1}
		if err := core.Global.WriteJSON(core.Global.CounterPath(), counter); err != nil {
			return "", fmt.Errorf("初始化 counter.json: %w", err)
		}
	}

	// 初始化 users.json
	if !core.Global.FileExists(core.Global.UsersPath()) {
		users := model.UsersFile{Users: make(map[string]*model.User)}
		if err := core.Global.WriteJSON(core.Global.UsersPath(), users); err != nil {
			return "", fmt.Errorf("初始化 users.json: %w", err)
		}
	}

	// 初始化 config.json
	if !core.Global.FileExists(core.Global.ConfigPath()) {
		cfg := config.DefaultConfig()
		if err := core.Global.WriteJSON(core.Global.ConfigPath(), cfg); err != nil {
			return "", fmt.Errorf("初始化 config.json: %w", err)
		}
	}

	return absDir, nil
}

// InitDB 初始化 SQLite 数据库（消息 + 锁共用），并运行自动迁移。
func InitDB(dbPath string) error {
	var err error
	core.DB, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return fmt.Errorf("打开数据库失败: %w", err)
	}

	//TODO:迁移就应该放在一起

	// 迁移消息表

	if err := message.InitModels(core.DB); err != nil {
		return fmt.Errorf("迁移消息表: %w", err)
	}

	// 迁移锁表
	if err := lock.InitModels(core.DB); err != nil {
		return fmt.Errorf("迁移锁表: %w", err)
	}

	// 迁移监听位表
	if err := watch.InitModels(core.DB); err != nil {
		return fmt.Errorf("迁移监听位表: %w", err)
	}

	return nil
}

// ReadAuditLog 扫描 Logs 目录，读取所有审计日志条目。
func ReadAuditLog() ([]model.AuditEntry, error) {
	logDir := logutil.LogDir()
	if logDir == "" {
		return nil, nil
	}
	entries, dirErr := os.ReadDir(logDir)
	if dirErr != nil {
		return nil, nil // Logs 目录不存在不算错误
	}
	var result []model.AuditEntry
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(logDir, entry.Name()))
		if err != nil {
			continue
		}
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || !strings.Contains(line, `"action"`) {
				continue // 只处理审计 JSON 行
			}
			var ae model.AuditEntry
			if err := json.Unmarshal([]byte(line), &ae); err == nil {
				result = append(result, ae)
			}
		}
	}
	return result, nil
}
