// Package model 定义了 allinker CLI 工具的所有数据类型。
//
// 包括：用户账户、文件锁信息、监控项、消息、
// 审计日志条目以及应用配置。
package model

import (
	"time"
)

// =============================================================================
// 数据库模型（GORM）
// =============================================================================

// LockRecord 是 GORM 对应的锁记录表结构。
type LockRecord struct {
	Filename  string    `gorm:"primaryKey"`     // 完整文件路径（不同目录同名文件不冲突）
	Holder    string    `gorm:"not null;index"` // 锁持有者用户名
	Timestamp time.Time `gorm:"not null"`       // 锁定时间
	PID       int       `gorm:"not null"`       // 持有者进程 ID
	ExpiresAt time.Time `gorm:"not null;index"` // 过期时间（索引用于快速清理）
}

// TableName 自定义表名。
func (LockRecord) TableName() string { return "locks" }

// IsExpired 检查锁是否已过期。
func (l *LockRecord) IsExpired() bool {
	return time.Now().After(l.ExpiresAt)
}

// RemainingSeconds 返回锁剩余秒数，已过期返回 0。
func (l *LockRecord) RemainingSeconds() int {
	remaining := time.Until(l.ExpiresAt)
	if remaining < 0 {
		return 0
	}
	return int(remaining.Seconds())
}

// WatchRecord 是 GORM 对应的监听位表结构。
type WatchRecord struct {
	ID           string     `gorm:"primaryKey"`
	Name         string     `gorm:"not null;uniqueIndex"`
	Creator      string     `gorm:"not null;index"`
	Dir          string     `gorm:"not null"`
	Pattern      string     `gorm:"not null"`
	CreatedAt    time.Time  `gorm:"not null"`
	LastCheck    time.Time  `gorm:"not null"`
	LastChange   *time.Time `gorm:"default:null"`
	Status       string     `gorm:"not null;default:active"`
	SnapshotData string     `gorm:"type:text"` // 文件快照哈希（JSON: {"path":"hash"}）
}

// TableName 自定义表名。
func (WatchRecord) TableName() string { return "watches" }

// =============================================================================
// 用户账户（users.json）
// =============================================================================

// UserRole 表示用户的权限级别。
type UserRole string

const (
	RoleAdmin UserRole = "admin" // 管理员，完全访问权限
	RoleAgent UserRole = "agent" // AI 代理，协作访问权限
	RoleGuest UserRole = "guest" // 只读访客权限
)

// UserStatus 表示用户账户是激活还是禁用状态。
type UserStatus string

const (
	UserStatusActive   UserStatus = "active"
	UserStatusDisabled UserStatus = "disabled"
)

// User 表示一个注册用户（AI 代理或人类）。
type User struct {
	ID             int64             `json:"id"`                       // 唯一数字 ID，位图下标
	Name           string            `json:"name"`                     // 唯一用户名
	Role           UserRole          `json:"role"`                     // 角色：admin / agent / guest
	Created        string            `json:"created"`                  // 注册时间（RFC3339）
	Status         UserStatus        `json:"status"`                   // 账号状态
	DisabledReason string            `json:"disabledReason,omitempty"` // 禁用原因（仅 disabled 时有效）
	Meta           map[string]string `json:"meta,omitempty"`           // 附加元信息（如 model、agentType）
}

// UsersFile 是 users.json 的根结构。
type UsersFile struct {
	Users map[string]*User `json:"users"`
}

// =============================================================================
// 监控点
// =============================================================================

// WatchStatus 表示监控点的状态。
type WatchStatus string

const (
	WatchStatusActive WatchStatus = "active"
)

// WatchItem 表示一个已注册的监控点。
type WatchItem struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Creator    string      `json:"creator"`
	Dir        string      `json:"dir"`
	Pattern    string      `json:"pattern"`
	Created    string      `json:"created"`
	LastCheck  string      `json:"lastCheck"`
	LastChange string      `json:"lastChange,omitempty"`
	Status     WatchStatus `json:"status"`
}

// =============================================================================
// 消息（messages/_index.json）
// =============================================================================

// Message 表示代理之间的聊天消息。
type Message struct {
	ID        int64  `json:"id"`
	From      string `json:"from"`
	To        string `json:"to"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
	Read      bool   `json:"read"`
}

// =============================================================================
// 审计日志条目（audit.log，JSONL 格式）
// =============================================================================

// AuditEntry 表示单条审计日志条目。
type AuditEntry struct {
	Time   string `json:"time"`
	User   string `json:"user"`
	Action string `json:"action"`
	Target string `json:"target,omitempty"`
	Result string `json:"result"`
	Detail string `json:"detail,omitempty"`
}

// =============================================================================
// 应用配置（config.json）
// =============================================================================

// AppConfig 保存应用级别的配置。
type AppConfig struct {
	LockTimeout          int    `json:"lockTimeout"`
	DefaultWaitTimeout   int    `json:"defaultWaitTimeout"`
	DefaultCheckInterval int    `json:"defaultCheckInterval"`
	MessageRetentionDays int    `json:"messageRetentionDays"`
	MaxMessagesPerUser   int    `json:"maxMessagesPerUser"`
	LogLevel             string `json:"logLevel"`
	AuditEnabled         bool   `json:"auditEnabled"`
}

// =============================================================================
// 全局 ID 计数器（counter.json）
// =============================================================================

// Counter 为各种实体提供自增 ID。
type Counter struct {
	NextID int64 `json:"nextId"`
}
