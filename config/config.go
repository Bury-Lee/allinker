// Package config 管理应用配置的读取和保存。
package config

import (
	"fmt"
	"os"
	"time"

	"allinker/core"
	"allinker/logutil"
	"allinker/model"
)

// DefaultConfig 返回默认的应用配置。
func DefaultConfig() *model.AppConfig {
	return &model.AppConfig{
		LockTimeout:          60,
		DefaultWaitTimeout:   60,
		DefaultCheckInterval: 2,
		MessageRetentionDays: 30,
		MaxMessagesPerUser:   1000,
		LogLevel:             "info",
		AuditEnabled:         true,
	}
}

// cachedConfig 避免每次访问都读取 config.json。
var cachedConfig *model.AppConfig

// GetConfig 读取并缓存应用配置。
func GetConfig() (*model.AppConfig, error) {
	if cachedConfig != nil {
		return cachedConfig, nil
	}
	if core.Global == nil {
		return nil, fmt.Errorf("数据目录未初始化")
	}

	cfg := &model.AppConfig{}
	if err := core.Global.ReadJSON(core.Global.ConfigPath(), cfg); err != nil {
		if os.IsNotExist(err) {
			cfg = DefaultConfig()
		} else {
			return nil, fmt.Errorf("读取 config.json: %w", err)
		}
	}
	cachedConfig = cfg
	return cfg, nil
}

// 预备函数:SaveConfig 持久化配置并更新缓存。
func SaveConfig(cfg *model.AppConfig) error {
	if core.Global == nil {
		return fmt.Errorf("数据目录未初始化")
	}
	if err := core.Global.WriteJSON(core.Global.ConfigPath(), cfg); err != nil {
		return fmt.Errorf("保存 config.json: %w", err)
	}
	cachedConfig = cfg
	return nil
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
	if err == nil && !cfg.AuditEnabled {
		return nil
	}
	logutil.Audit(entry.User, entry.Action, entry.Target, entry.Result, entry.Detail)
	return nil
}
