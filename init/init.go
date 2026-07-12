// Package init 负责程序初始化：数据目录、JSON 存储、SQLite 数据库。
//
// 初始化顺序（不可随意调换）：
// 1. InitDataDir — 创建 .alf 目录及 users.json/config.json/counter.json
// 2. InitDB — 打开 SQLite 并 AutoMigrate 所有表
// 3. message.InitService + wait.InitService — 初始化业务层的全局单例
//
// 设计决策：
// - CLI 模式每次调用都走完整的初始化流程（短进程自然无状态）
// - Server 模式只初始化一次（长进程复用连接）
package init

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"allinker/config"
	"allinker/core"
	"allinker/logutil"
	"allinker/message"
	"allinker/model"
	"allinker/storage"
	"allinker/wait"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const DefaultDataDir = ".alf"

// InitDataDir 初始化数据目录及所有必需的 JSON 文件。
// 为什么用 JSON 文件而非 SQLite 存这些数据：
//   - users.json：低频变更 + 人工可直接编辑（紧急情况加账号）
//   - config.json：用户希望像 Git 一样直接编辑配置文件生效
//   - counter.json：只有 nextId 一个字段，JSON 比 SQLite 查询快
// 幂等设计：多次调用安全（core.Global != nil 时直接返回）。
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
	} else {
		// 确保旧版 config.json 补充 server 字段
		ensureServerConfig()
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

	// 统一迁移所有表（消息 + 锁 + 监听位共享同一 SQLite 数据库）
	if err := core.DB.AutoMigrate(
		&message.MessageORM{},
		&model.LockRecord{},
		&model.WatchRecord{},
	); err != nil {
		return fmt.Errorf("迁移数据表: %w", err)
	}

	// 初始化 MessageService 全局单例（任务B：Service 化）
	// 必须在 AutoMigrate 之后调用，因为 MessageService 依赖已迁移的表
	if err := message.InitService(core.DB); err != nil {
		return fmt.Errorf("初始化 MessageService: %w", err)
	}

	// 初始化 WaitService 全局单例（任务B：Service 化）
	if err := wait.InitService(core.DB); err != nil {
		return fmt.Errorf("初始化 WaitService: %w", err)
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

// ensureServerConfig 确保 config.json 包含 server 配置段。
// 兼容升级：旧版 config.json 没有 server 字段，补上默认值再写回。
func ensureServerConfig() {
	cfg, err := config.GetConfig()
	if err != nil {
		return
	}
	// server 端口为零值说明旧版 config.json 没有 server 段
	if cfg.Server.Port == 0 {
		def := config.DefaultConfig()
		cfg.Server = def.Server
		config.SaveConfig(cfg)
	}
}
