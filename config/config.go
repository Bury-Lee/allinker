// Package config 管理应用配置的读取和保存。
//
// 设计决策：
// - 配置存储在 config.json（.alf 目录下），JSON 格式可人工编辑
// - 每次 GetConfig 检查文件的 ModTime，文件没变返回缓存，变了重新读
// - 优先级链：CLI 参数 > config.json > 硬编码默认值（同 Git 风格）
//
// 配置读取策略（类似 Git）：
//   CLI 模式：每次命令执行是独立进程，必然重新读取文件
//   Server 模式：常驻进程，通过 stat 检查文件 mtime 自动感知变更
//   优先级：CLI 参数 > config.json > 硬编码默认值
package config

import (
	"fmt"
	"os"
	"sync"
	"time"

	"allinker/core"
	"allinker/logutil"
	"allinker/model"
)

// DefaultConfig 返回默认的应用配置。
func DefaultConfig() *model.AppConfig {
	return &model.AppConfig{
		Lock: model.LockConfig{
			Timeout: 60,
		},
		Wait: model.WaitConfig{
			DefaultTimeout: 60,
			CheckInterval:  2,
		},
		Message: model.MessageConfig{
			RetentionDays: 30,
			MaxPerUser:    1000,
		},
		Log: model.LogConfig{
			Level: "info",
			Audit: true,
		},
		Server: model.ServerConfig{
			Host:      "127.0.0.1",
			Port:      8080,
			AuthToken: "",
			LogLevel:  "info",
		},
	}
}

// 缓存 + stat 检查，Server 模式可感知文件变更
var (
	cachedConfig *model.AppConfig
	cachedTime   time.Time
	configMu     sync.Mutex
)

// GetConfig 读取应用配置。
// CLI 模式下每次启动都重新读取（进程是新的，cache 为空）。
// Server 模式下 stat 检查 mtime，文件变了自动重读。
func GetConfig() (*model.AppConfig, error) {
	configMu.Lock()
	defer configMu.Unlock()

	if core.Global == nil {
		return nil, fmt.Errorf("数据目录未初始化")
	}

	configPath := core.Global.ConfigPath()
	fi, err := os.Stat(configPath)
	if err != nil {
		return DefaultConfig(), nil
	}

	// stat 检查：文件未变且缓存有效则复用
	if cachedConfig != nil && fi.ModTime().Equal(cachedTime) {
		return cachedConfig, nil
	}

	// 重新读取文件
	cfg := &model.AppConfig{}
	if err := core.Global.ReadJSON(configPath, cfg); err != nil {
		if os.IsNotExist(err) {
			cfg = DefaultConfig()
		} else {
			return nil, fmt.Errorf("读取 config.json: %w", err)
		}
	}
	cachedConfig = cfg
	cachedTime = fi.ModTime()
	return cfg, nil
}

// SaveConfig 持久化配置并更新缓存。
func SaveConfig(cfg *model.AppConfig) error {
	configMu.Lock()
	defer configMu.Unlock()

	if core.Global == nil {
		return fmt.Errorf("数据目录未初始化")
	}
	if err := core.Global.WriteJSON(core.Global.ConfigPath(), cfg); err != nil {
		return fmt.Errorf("保存 config.json: %w", err)
	}
	cachedConfig = cfg
	cachedTime = time.Now()
	return nil
}

// Reload 清除配置缓存，下次 GetConfig 时重新读取文件。
func Reload() {
	configMu.Lock()
	defer configMu.Unlock()
	cachedConfig = nil
	cachedTime = time.Time{}
}

// NextID 递增并返回全局计数器的 nextId。
// 使用文件级互斥锁（O_CREATE|O_EXCL）保证并发安全。
func NextID() (int64, error) {
	if core.Global == nil {
		return 0, fmt.Errorf("数据目录未初始化")
	}

	// 使用文件级互斥锁保证原子性，加循环重试避免并发冲突
	lockPath := core.Global.CounterPath() + ".lock"
	var lockFile *os.File
	var err error
	for i := 0; i < 100; i++ { // 最多重试 100 次（约 5 秒）
		lockFile, err = os.OpenFile(lockPath, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0644)
		if err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil {
		return 0, fmt.Errorf("获取 NextID 锁失败（重试 100 次后放弃）,请重试指令: %w", err)
	}
	// defer 确保释放锁
	defer func() {
		_ = lockFile.Close()
		_ = os.Remove(lockPath)
	}()

	counter := &model.Counter{}
	if err := core.Global.ReadJSON(core.Global.CounterPath(), counter); err != nil {
		return 0, fmt.Errorf("读取 counter.json: %w", err)
	}
	id := counter.NextID
	counter.NextID++
	if err := core.Global.WriteJSON(core.Global.CounterPath(), counter); err != nil {
		return 0, fmt.Errorf("写入 counter.json: %w", err)
	}
	return id, nil
}

// ReadJSON 是 core.Global.ReadJSON 的便捷封装。
func ReadJSON(path string, out any) error {
	if core.Global == nil {
		return fmt.Errorf("数据目录未初始化")
	}
	return core.Global.ReadJSON(path, out)
}

// WriteJSON 是 core.Global.WriteJSON 的便捷封装。
func WriteJSON(path string, data any) error {
	if core.Global == nil {
		return fmt.Errorf("数据目录未初始化")
	}
	return core.Global.WriteJSON(path, data)
}

// AppendAuditLog 记录一条审计日志到 Logs 目录的每日轮转文件。
func AppendAuditLog(entry model.AuditEntry) error {
	cfg, err := GetConfig()
	if err == nil && !cfg.Log.Audit {
		return nil
	}
	logutil.Audit(entry.User, entry.Action, entry.Target, entry.Result, entry.Detail)
	return nil
}
