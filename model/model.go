// Package model 定义了 allinker CLI 工具的所有数据类型。
//
// 包括：用户账户、文件锁信息、监控项、消息、
// 审计日志条目以及应用配置。
//
// 设计原则：
// 1. 与 GORM 表结构对应的模型（LockRecord/WatchRecord/MessageORM）放在 DB Models 段
// 2. JSON 文件存储的模型（User/AppConfig）放在各自功能段
// 3. 公共基类 Model 提供 ID/CreatedAt/UpdatedAt 三个通用字段，其他模型通过嵌入复用
package model

import (
	"time"
)

// =============================================================================
// 公共基类
// =============================================================================

// Model 是所有数据库模型的公共基类。
// 嵌入此结构体的模型自动获得主键 ID、创建时间和更新时间三个字段，
// 避免每个模型重复定义。
// 参考 GoBlog 项目的 models.Model 设计模式。
type Model struct {
	ID        uint      `gorm:"primarykey" json:"id"`          // 主键 ID（自增，GORM 自动维护）
	CreatedAt time.Time `gorm:"not null" json:"createdAt"`    // 记录创建时间（自动写入，仅创建时设置）
	UpdatedAt time.Time `gorm:"not null" json:"updatedAt"`    // 记录更新时间（每次更新自动刷新）
}

// =============================================================================
// 数据库模型（GORM）
// 与 SQLite 表结构一一对应，通过 GORM AutoMigrate 自动建表。
// =============================================================================

// LockRecord 锁记录表结构（对应 SQLite locks 表）。
// 使用完整文件路径作为主键，天然保证同一文件只有一个锁记录。
// 每条记录包含持有者、锁定时间、进程 PID 和过期时间，
// 启动时自动清理过期锁防止死锁堆积。
// 注意：此为建议锁（advisory lock），不强制拦截文件系统写入。
// 注意：不使用 Model 嵌入——因为主键是 Filename 而非 ID，
// 嵌入会导致 GORM 复合主键 (id, filename) 而非预期的单主键 filename。
type LockRecord struct {
	Filename  string    `gorm:"primaryKey"`     // 完整文件路径（Unix 格式，跨平台兼容）
	Holder    string    `gorm:"not null;index"` // 锁持有者用户名
	Timestamp time.Time `gorm:"not null"`       // 锁定时间
	PID       int       `gorm:"not null"`       // 持有者进程 ID（仅用于日志，不用于锁判定）
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
	Description    string            `json:"description,omitempty"`    // 岗位描述（如"后端开发"）
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
// 按功能拆分子结构体，参考 GoBlog 的 conf/ 目录风格。
type AppConfig struct {
	Lock    LockConfig    `json:"lock"`    // 锁配置
	Wait    WaitConfig    `json:"wait"`    // 等待配置
	Message MessageConfig `json:"message"` // 消息配置
	Log     LogConfig     `json:"log"`     // 日志配置
	Server  ServerConfig  `json:"server"`  // 服务模式配置
	Remotes map[string]RemoteConfig `json:"remotes,omitempty"` // 命名远程连接
}

// LockConfig 锁配置。
type LockConfig struct {
	Timeout int `json:"timeout"` // 锁超时秒数，默认 60
}

// WaitConfig 等待配置。
type WaitConfig struct {
	DefaultTimeout int `json:"defaultTimeout"` // 默认等待超时秒数，默认 60
	CheckInterval  int `json:"checkInterval"`  // 轮询检查间隔秒数，默认 2
}

// MessageConfig 消息配置。
type MessageConfig struct {
	RetentionDays int `json:"retentionDays"` // 消息保留天数，默认 30
	MaxPerUser    int `json:"maxPerUser"`    // 每用户最大消息数，默认 1000
}

// LogConfig 日志配置。
type LogConfig struct {
	Level  string `json:"level"`  // 日志级别：debug/info/warn/error
	Audit  bool   `json:"audit"`  // 是否启用审计日志，默认 true
}

// ServerConfig 服务模式配置。
type ServerConfig struct {
	Host      string `json:"host"`      // 绑定地址，默认 127.0.0.1
	Port      int    `json:"port"`      // 端口，默认 8080
	AuthToken string `json:"authToken"` // 可选鉴权 Token，空表示不鉴权
	LogLevel  string `json:"logLevel"`  // 服务日志级别
}

// RemoteConfig 远程连接配置。
type RemoteConfig struct {
	Addr  string `json:"addr"`  // 远程地址，格式 "host:port"
	Token string `json:"token"` // 远程鉴权 Token，空表示无密码
}

// =============================================================================
// 全局 ID 计数器（counter.json）
// =============================================================================

// Counter 为各种实体提供自增 ID。
type Counter struct {
	NextID int64 `json:"nextId"`
}
