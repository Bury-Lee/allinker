// Package lock 提供分布式数据库锁机制。
//
// 锁存储在 SQLite 数据库中（与消息共用 allinker.db），
// 每条记录对应一个文件锁，使用完整文件路径作为主键。
// 启动时自动清理过期锁记录。
//
// 设计决策：
// - 使用 SQLite 而非内存 map：进程崩溃后锁记录不丢失，重启可恢复
// - 完整文件路径作主键：不同目录下的同名文件不冲突
// - 自动过期（默认 60s）：防止 AI 崩溃后锁永远不释放
// - 清理过期锁的 WHERE 条件包含 expires_at：防止并发误删其他 AI 刚创建的新锁
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

// =============================================================================
// 共享 Handler — 方案C：CLI 和 Server 共用同一套参数结构体 + 处理函数
// =============================================================================

// LockParams lock/tryLock 命令的共享参数结构体。
// CLI 和 Server 模式都通过这个结构体传递参数，确保解析一致性。
// 在方重构后，所有调用方（CLI handler + Server executeCommand）都使用此结构体，
// 避免两套解析逻辑产生差异（如 -f 在远程不生效的问题）。
type LockParams struct {
	Filename string // 目标文件路径
	Username string // 操作者用户名
	Timeout  int    // 等待超时秒数（0 = tryLock 立即返回，>0 = 阻塞等待）
}

// HandleLock 是 lock/tryLock 的共享处理函数。
// CLI 模式的 handleLock/handleTryLock 和 Server 模式的 executeCommand case 'lock' 都调此函数。
// timeout == 0 时等同于 tryLock；timeout > 0 时阻塞等待直到超时。
func HandleLock(params LockParams) error {
	if params.Timeout > 0 {
		deadline := time.Now().Add(time.Duration(params.Timeout) * time.Second)
		return AcquireLock(params.Filename, params.Username, deadline)
	}
	return TryAcquireLock(params.Filename, params.Username)
}

// InitModels 注册锁表到给定数据库实例。
func InitModels(db *gorm.DB) error {
	return db.AutoMigrate(&model.LockRecord{})
}

// =============================================================================
// 公开 API
// =============================================================================

// AcquireLock 尝试获取文件锁（阻塞等待模式）。
// deadline 为等待截止时间，如果为零值则使用默认超时（60s）。
// 轮询间隔 1 秒：AI 场景下 1 秒延迟可接受，且避免 SQLite 频繁读写。
// 返回 error 的情况：超时未获取到锁、数据库错误。
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
// 不阻塞等待。如果锁已被其他 AI 持有，立即返回错误，附带当前持有者信息。
// 适用于 AI 在修改前先尝试加锁，失败则转用阻塞锁或跳过。
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
// 只有锁的持有者才能释放自己的锁（通过 username 匹配 Holder）。
// 锁不存在时返回 nil（视为已释放，幂等操作）。
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
// 返回 nil 表示没有锁。已过期的锁也会返回，调用方通过 IsExpired 自行判断。
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
// 调用方需要过滤过期锁，或直接使用 status --all 命令。
func ListLocks() []model.LockRecord {
	if core.DB == nil {
		return nil
	}
	var records []model.LockRecord
	core.DB.Find(&records)
	return records
}

// StartCleanup 发送一次过期锁清理指令（不等待 SQLite 返回）。
// 在 main.go 启动时调用，以 goroutine 异步执行，不阻塞启动流程。
// 清理条件：expires_at < NOW() 的锁记录会被删除。
func StartCleanup() {
	go func() {
		if core.DB != nil {
			core.DB.Where("expires_at < ?", time.Now()).Delete(&model.LockRecord{})
		}
	}()
}

// TODO:加一个锁时间的参数t second,t>10分钟时虽然设置,但是会进行警告,进行一下调用方适配
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
		// 锁已过期，按条件删除（只删过期锁，防止误删并发新锁）
		core.DB.Where("filename = ? AND expires_at < ?",
			filename, time.Now()).Delete(&model.LockRecord{})
	} else if err != gorm.ErrRecordNotFound {
		return false, fmt.Errorf("查询锁失败: %w", err)
	}

	// 创建新锁
	now := time.Now()
	lockTimeout := 60
	if cfg, err := config.GetConfig(); err == nil && cfg.Lock.Timeout > 0 {
		lockTimeout = cfg.Lock.Timeout
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
