// Package message 提供 Agent 之间的消息通信功能。
//
// 消息使用 GORM + SQLite 持久化存储。
// 每条消息有且仅有一个收件人（To 字段），群发时创建多条独立记录。
// 已读状态使用位图（BLOB）实现：每个用户有唯一 ID，位图中第 N bit 表示 user=N 已读。
//
// 架构（任务 B 升级）：
//   - MessageService 封装所有业务方法（Send/Receive/History）
//   - AppMessage 是全局单例，CLI handler 和 Server handler 共用
//   - 旧的 SendMessage/ReceiveMessages/GetHistory 保留为包级函数，
//     内部转调 AppMessage 对应方法，保证向后兼容
package message

import (
	"fmt"
	"strings"
	"time"

	"allinker/config"
	"allinker/core"
	"allinker/model"

	"gorm.io/gorm"
)

// =============================================================================
// MessageService — 业务封装（方案B：Service struct + New() + 全局单例 AppMessage）
// =============================================================================

// MessageService 封装所有消息相关业务方法。
// 设计参考 GOblog 的 service.UserService 模式。
// 字段为 db *gorm.DB，构造时传入（便于未来 mock 测试）。
type MessageService struct {
	db *gorm.DB
}

// NewDBService 构造 MessageService。
// 参数: db — GORM 数据库实例（通常传 core.DB）
// 返回: *MessageService — 新实例，nil 仅当 db 为 nil
func NewDBService(db *gorm.DB) *MessageService {
	if db == nil {
		return nil
	}
	return &MessageService{db: db}
}

// AppMessage 是 MessageService 的全局单例。
// CLI handler 和 Server handler 都通过它调用业务方法，避免散落的包级函数。
// 初始化见 init 包或 main.go（在 core.DB 初始化后调用 message.InitService(core.DB)）。
//
// 注意：不能直接在 var 声明中初始化为 NewMessageService(core.DB)，
// 因为包初始化时 core.DB 还未赋值，会导致 AppMessage = nil，
// 运行时调用方法会 panic（nil pointer dereference）。
var AppMessage *MessageService

// InitService 初始化全局 AppMessage 单例。
// 参数: db — GORM 数据库实例（通常传 core.DB）
// 说明: 必须在 core.DB 初始化完成后调用，建议在 main.go 的 init.InitDataDir + InitDB 之后。
// 返回: error — 当前保留为 nil，未来可加重复初始化检测
func InitService(db *gorm.DB) error {
	AppMessage = NewDBService(db)
	return nil
}

// =============================================================================
// 共享 Handler — 方案C：CLI 和 Server 共用同一套参数结构体 + 处理函数
// =============================================================================

// SendParams send 命令的共享参数结构体。
type SendParams struct {
	From    string
	To      []string
	Content string
}

// RecvParams recv 命令的共享参数结构体。
type RecvParams struct {
	From string
	User string
	All  bool
	ID   int64
}

// HistoryParams history 命令的共享参数结构体。
type HistoryParams struct {
	WithUser string
	Limit    int
}

// HandleRecv 是 recv 命令的共享处理函数。
func HandleRecv(p RecvParams) ([]*model.Message, error) {
	if p.All || p.ID > 0 {
		return AppMessage.ReceiveMessages(p.From, p.User, true, 0)
	}
	return AppMessage.ReceiveMessages(p.From, p.User, false, 0)
}

// HandleSend 是 send 命令的共享处理函数。
func HandleSend(p SendParams) (*model.Message, error) {
	return AppMessage.SendMessage(p.From, p.To, p.Content)
}

// HandleHistory 是 history 命令的共享处理函数。
func HandleHistory(p HistoryParams) ([]*model.Message, error) {
	return AppMessage.GetHistory(p.WithUser, p.Limit)
}

// GetMessagesSince 获取 ID 大于 sinceID 的消息（全局视图，不限用户）。
// 用于 chat 实时轮询。limit<=0 时不限制。
func GetMessagesSince(sinceID int64, limit int) ([]*model.Message, error) {
	return AppMessage.GetMessagesSince(sinceID, limit)
}

// MessageORM 是 GORM 对应的消息表结构。
type MessageORM struct {
	ID         int64     `gorm:"primaryKey;autoIncrement"`
	SenderName string    `gorm:"not null;index"`
	To         *string   `gorm:"index"` // 收件人用户名；nil 表示广播给所有人
	Content    string    `gorm:"not null;type:text"`
	ReadBitmap []byte    `gorm:"type:blob;not null;default:x'00'"`
	CreatedAt  time.Time `gorm:"index"`
}

// TableName 自定义表名。
// 返回: string — 固定为 "messages"（与 cli_server.go / wait.go 硬编码保持一致）。
// 说明: GORM 默认会用结构体名 + "s" 作为表名，这里显式声明避免重命名时漏改。
func (MessageORM) TableName() string { return "messages" }

// InitModels 注册消息相关的 GORM 模型到给定数据库实例。
// 参数: db — GORM 数据库实例。
// 返回: error — AutoMigrate 失败的错误（包装 %w）。
// 说明: 实际已统一在 init/init.go 的 InitDB() 里调 AutoMigrate 3 个表，
// 这里保留函数定义是给外部包/测试用例做单表迁移用的，不删除以保持向后兼容。
func InitModels(db *gorm.DB) error {
	return db.AutoMigrate(&MessageORM{})
}

// =============================================================================
// 位图操作
// =============================================================================

// IsReadBy 检查用户是否已读此消息。
// 参数: bitmap — 消息的位图字节切片（来自 ReadBitmap 字段）；userID — 用户数字 ID。
// 返回: bool — true 表示已读，false 表示未读或超出位图范围。
// 说明: 位图按 userID/8 字节索引 + userID%8 位索引 寻址。
// 之所以用位图而非"每用户一行"：节省空间（O(用户数/8) 而非 O(消息数×用户数)），
// 单条 UPDATE 即可标记 N 个用户已读。约束：userID 必须永不重用（依赖 config.NextID 永远递增）。
func IsReadBy(bitmap []byte, userID int64) bool {
	byteIdx := userID / 8
	bitIdx := userID % 8
	if int(byteIdx) >= len(bitmap) {
		return false
	}
	return bitmap[byteIdx]&(1<<bitIdx) != 0
}

// MarkReadBy 标记用户已读，返回更新后的位图（必要时自动扩容）。
// 参数: bitmap — 消息的原位图字节切片；userID — 用户数字 ID。
// 返回: []byte — 新位图（如果原 bitmap 容量不够，会按需扩容到 byteIdx+1 字节）。
// 说明: 自动扩容是必要的——初始 ReadBitmap 只有 1 字节（支持 user 0-7），
// 标记 user=10 时需扩到 2 字节。注意：如果调用方拿到返回值后忘了写回数据库，
// 内存位图与持久化位图会不一致，调用方应将返回值写回。
func MarkReadBy(bitmap []byte, userID int64) []byte {
	byteIdx := userID / 8
	bitIdx := userID % 8
	need := byteIdx + 1
	if int(need) > len(bitmap) {
		newBitmap := make([]byte, need)
		copy(newBitmap, bitmap)
		bitmap = newBitmap
	}
	bitmap[byteIdx] |= 1 << bitIdx
	return bitmap
}

// =============================================================================
// 业务函数
// =============================================================================

// SendMessage 发送消息。
// to 包含 "All"（不区分大小写）时视为广播：创建单条 To=nil 的记录。
// 否则每个收件人创建独立记录。返回第一个收件人的消息用于确认提示。
func (s *MessageService) SendMessage(from string, to []string, content string) (*model.Message, error) {
	if from == "" {
		return nil, fmt.Errorf("发送方不能为空")
	}
	if len(to) == 0 {
		return nil, fmt.Errorf("接收方不能为空")
	}
	if content == "" {
		return nil, fmt.Errorf("消息内容不能为空")
	}

	var firstMsg *model.Message
	now := time.Now().UTC()

	// 广播模式：收件人为 "All"，To=nil 一条记录
	if len(to) == 1 && strings.EqualFold(to[0], "All") {
		msg := MessageORM{
			SenderName: from,
			To:         nil,
			Content:    content,
			ReadBitmap: []byte{0},
			CreatedAt:  now,
		}
		if err := s.db.Create(&msg).Error; err != nil {
			return nil, fmt.Errorf("保存广播消息失败: %w", err)
		}
		//返回发送值
		firstMsg = &model.Message{
			ID:        msg.ID,
			From:      msg.SenderName,
			To:        "All",
			Content:   msg.Content,
			Timestamp: msg.CreatedAt.Format(time.RFC3339),
			Read:      false,
		}
		return firstMsg, nil
	}

	for i, recipient := range to {
		r := recipient
		msg := MessageORM{
			SenderName: from,
			To:         &r,
			Content:    content,
			ReadBitmap: []byte{0},
			CreatedAt:  now,
		}
		if err := s.db.Create(&msg).Error; err != nil {
			return nil, fmt.Errorf("保存消息给 %s 失败: %w", recipient, err)
		}

		m := &model.Message{
			ID:        msg.ID,
			From:      msg.SenderName,
			To:        recipient,
			Content:   msg.Content,
			Timestamp: msg.CreatedAt.Format(time.RFC3339),
			Read:      false,
		}
		if i == 0 {
			firstMsg = m
		}
	}

	return firstMsg, nil
}

// ReceiveMessages 获取消息。
// showAll=false：只返回未读消息，自动标记已读。
// showAll=true：返回全部消息。
// limit<=0 时不限制数量。
// 查询逻辑：返回 to=当前用户 或 to IS NULL（广播）的消息。
func (s *MessageService) ReceiveMessages(from string, currentUserName string, showAll bool, limit int) ([]*model.Message, error) {
	userID := getUserID(currentUserName)

	var messages []MessageORM
	query := s.db.Model(&MessageORM{})
	if currentUserName != "" {
		// 返回发给我的 + 广播消息（to IS NULL）
		query = query.Where("`to` = ? OR `to` IS NULL", currentUserName)
	}
	if from != "" {
		query = query.Where("sender_name = ?", from)
	}
	query.Order("id ASC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	query.Find(&messages)

	var result []*model.Message
	for _, m := range messages {
		if !showAll && IsReadBy(m.ReadBitmap, userID) {
			continue
		}

		toName := "All"
		if m.To != nil {
			toName = *m.To
		}
		msg := &model.Message{
			ID:        m.ID,
			From:      m.SenderName,
			To:        toName,
			Content:   m.Content,
			Timestamp: m.CreatedAt.Format(time.RFC3339),
			Read:      IsReadBy(m.ReadBitmap, userID),
		}
		result = append(result, msg)

		if !showAll {
			newBitmap := MarkReadBy(m.ReadBitmap, userID)
			s.db.Model(&MessageORM{}).Where("id = ?", m.ID).Update("read_bitmap", newBitmap)
		}
	}

	return result, nil
}

// GetHistory 获取通信历史记录。
// withUser 不为空时，返回该用户发送或接收的消息（含广播）。
func (s *MessageService) GetHistory(withUser string, limit int) ([]*model.Message, error) {
	if limit <= 0 {
		limit = 10
	}

	var messages []MessageORM
	query := s.db.Model(&MessageORM{}).Order("id DESC").Limit(limit)
	if withUser != "" {
		// 返回该用户发送、接收或广播（to IS NULL）的消息
		query = query.Where("sender_name = ? OR `to` = ? OR `to` IS NULL", withUser, withUser)
	}
	query.Find(&messages)

	var result []*model.Message
	for i := len(messages) - 1; i >= 0; i-- {
		m := messages[i]
		toName := "All"
		if m.To != nil {
			toName = *m.To
		}
		result = append(result, &model.Message{
			ID:        m.ID,
			From:      m.SenderName,
			To:        toName,
			Content:   m.Content,
			Timestamp: m.CreatedAt.Format(time.RFC3339),
		})
	}

	return result, nil
}

// GetMessagesSince 获取 ID 大于 sinceID 的所有消息（全局视图，不限用户）。
func (s *MessageService) GetMessagesSince(sinceID int64, limit int) ([]*model.Message, error) {
	var messages []MessageORM
	query := s.db.Model(&MessageORM{}).Where("id > ?", sinceID).Order("id ASC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	query.Find(&messages)

	result := make([]*model.Message, len(messages))
	for i, m := range messages {
		toName := "All"
		if m.To != nil {
			toName = *m.To
		}
		result[i] = &model.Message{
			ID:        m.ID,
			From:      m.SenderName,
			To:        toName,
			Content:   m.Content,
			Timestamp: m.CreatedAt.Format(time.RFC3339),
		}
	}
	return result, nil
}

// GetBroadcastsSince 获取 ID 大于 sinceID 的广播消息（To IS NULL，即群发 All 的消息）。
// 不包含私发给特定用户的消息，用于 chat 聊天室展示。
func (s *MessageService) GetBroadcastsSince(sinceID int64, limit int) ([]*model.Message, error) {
	var messages []MessageORM
	query := s.db.Model(&MessageORM{}).
		Where("id > ? AND `to` IS NULL", sinceID).
		Order("id ASC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	query.Find(&messages)

	result := make([]*model.Message, len(messages))
	for i, m := range messages {
		result[i] = &model.Message{
			ID:        m.ID,
			From:      m.SenderName,
			To:        "All",
			Content:   m.Content,
			Timestamp: m.CreatedAt.Format(time.RFC3339),
		}
	}
	return result, nil
}

// getUserID 根据用户名获取其数字 ID（用于位图寻址）。
// 参数: username — 用户名。
// 返回: int64 — 用户 ID；用户不存在时返回 0（约定：ID=0 永远未读）。
// 说明: 内部读 .alf/users.json（与 account 包共享），不依赖数据库查询，
// 这样 ReceiveMessages 在位图操作时不需要再查 users 表，省一次 IO。
// 隐含约束：users.json 必须是最新版本（getUserID 不会做缓存，可能与数据库不一致），
// 极端情况下（用户被禁用后立即 recv）可能用过期 ID 写入位图，但不影响功能正确性。
func getUserID(username string) int64 {
	users := &model.UsersFile{}
	if err := config.ReadJSON(core.Global.UsersPath(), users); err != nil {
		return 0
	}
	user, exists := users.Users[username]
	if !exists {
		return 0
	}
	return user.ID
}
