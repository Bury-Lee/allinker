// Package lock 提供分布式数据库锁机制。
//
// 锁存储在 SQLite 数据库中（与消息共用 allinker.db），
// 每条记录对应一个文件锁，使用完整文件路径作为主键。
// 启动时自动清理过期锁记录。
package lock

import (
	"fmt"
	"os"
	"time"

	"allinker/config"
	"allinker/core"
	"allinker/model"

	"gorm.io/gorm"
)

// InitModels 注册锁表到给定数据库实例。
func InitModels(db *gorm.DB) error {
	return db.AutoMigrate(&model.LockRecord{})
}

// =============================================================================
// 公开 API
// =============================================================================

// AcquireLock 尝试获取文件锁（阻塞等待模式）。
// deadline 为等待截止时间，如果为零值则使用默认超时。
func AcquireLock(filename, username string, deadline time.Time) error {
	if core.DB == nil {
		return fmt.Errorf("锁数据库未初始化")
	}

	// 默认超时 60 秒
	if deadline.IsZero() {
		deadline = time.Now().Add(60 * time.Second)
	}

	for {
		acquired, err := tryAcquire(filename, username)
		if err != nil {
			return err
		}
		if acquired {
			return nil
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("等待锁超时")
		}

		time.Sleep(1 * time.Second)
	}
}

// TryAcquireLock 尝试获取文件锁（立即返回模式）。
func TryAcquireLock(filename, username string) error {
	if core.DB == nil {
		return fmt.Errorf("锁数据库未初始化")
	}
	acquired, err := tryAcquire(filename, username)
	if err != nil {
		return err
	}
	if !acquired {
		info := GetLockInfo(filename)
		if info != nil && !info.IsExpired() {
			return fmt.Errorf("锁已被占用(持有者: %s, 剩余时间: %d秒)",
				info.Holder, info.RemainingSeconds())
		}
		return fmt.Errorf("锁已被占用")
	}
	return nil
}

// ReleaseLock 释放文件锁。
func ReleaseLock(filename, username string) error {
	if core.DB == nil {
		return fmt.Errorf("锁数据库未初始化")
	}

	var rec model.LockRecord
	err := core.DB.Where("filename = ?", filename).First(&rec).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil // 没有锁，视为成功
		}
		return fmt.Errorf("查询锁失败: %w", err)
	}

	if rec.Holder != username {
		return fmt.Errorf("锁不属于 %s（持有者: %s）", username, rec.Holder)
	}

	return core.DB.Delete(&rec).Error
}

// GetLockInfo 获取指定文件的锁信息。
// 返回 nil 表示没有锁。已过期的锁也会返回（调用方自行检查 IsExpired）。
func GetLockInfo(filename string) *model.LockRecord {
	if core.DB == nil {
		return nil
	}
	var rec model.LockRecord
	err := core.DB.Where("filename = ?", filename).First(&rec).Error
	if err != nil {
		return nil
	}
	return &rec
}

// ListLocks 返回所有锁记录（含已过期的）。
func ListLocks() []model.LockRecord {
	if core.DB == nil {
		return nil
	}
	var records []model.LockRecord
	core.DB.Find(&records)
	return records
}

// StartCleanup 发送一次过期锁清理指令（不等待 SQLite 返回）。
func StartCleanup() {
	go func() {
		if core.DB != nil {
			core.DB.Where("expires_at < ?", time.Now()).Delete(&model.LockRecord{})
		}
	}()
}

// tryAcquire 尝试获取锁，返回是否成功。
func tryAcquire(filename, username string) (bool, error) {
	// 读取已有锁记录
	var rec model.LockRecord
	err := core.DB.Where("filename = ?", filename).First(&rec).Error

	if err == nil {
		// 有锁记录，检查是否过期
		if !rec.IsExpired() {
			return false, nil // 锁还未过期
		}
		// 锁已过期，删除旧记录
		core.DB.Delete(&rec)
	} else if err != gorm.ErrRecordNotFound {
		return false, fmt.Errorf("查询锁失败: %w", err)
	}

	// 创建新锁
	now := time.Now()
	lockTimeout := 60
	if cfg, err := config.GetConfig(); err == nil && cfg.LockTimeout > 0 {
		lockTimeout = cfg.LockTimeout
	}
	newRec := model.LockRecord{
		Filename:  filename,
		Holder:    username,
		Timestamp: now,
		PID:       os.Getpid(),
		ExpiresAt: now.Add(time.Duration(lockTimeout) * time.Second),
	}

	if err := core.DB.Create(&newRec).Error; err != nil {
		return false, fmt.Errorf("创建锁失败: %w", err)
	}

	return true, nil
}
