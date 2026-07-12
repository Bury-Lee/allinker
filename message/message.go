// Package message 提供 Agent 之间的消息通信功能。
//
// 消息使用 GORM + SQLite 持久化存储。
// 每条消息有且仅有一个收件人（To 字段），群发时创建多条独立记录。
// 已读状态使用位图（BLOB）实现：每个用户有唯一 ID，位图中第 N bit 表示 user=N 已读。
package message

import (
	"fmt"
	"time"

	"allinker/config"
	"allinker/core"
	"allinker/model"

	"gorm.io/gorm"
)

// MessageORM 是 GORM 对应的消息表结构。
type MessageORM struct {
	ID         int64     `gorm:"primaryKey;autoIncrement"`
	SenderName string    `gorm:"not null;index"`
	To         string    `gorm:"not null;index"` // 收件人用户名（单条消息只有一个收件人）
	Content    string    `gorm:"not null;type:text"`
	ReadBitmap []byte    `gorm:"type:blob;not null;default:x'00'"`
	CreatedAt  time.Time `gorm:"index"`
}

// TableName 自定义表名。
func (MessageORM) TableName() string { return "messages" }

// InitModels 注册消息相关的 GORM 模型到给定数据库实例。
func InitModels(db *gorm.DB) error {
	return db.AutoMigrate(&MessageORM{})
}

// =============================================================================
// 位图操作
// =============================================================================

// IsReadBy 检查用户是否已读此消息。
func IsReadBy(bitmap []byte, userID int64) bool {
	byteIdx := userID / 8
	bitIdx := userID % 8
	if int(byteIdx) >= len(bitmap) {
		return false
	}
	return bitmap[byteIdx]&(1<<bitIdx) != 0
}

// MarkReadBy 标记用户已读，返回更新后的位图。
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

// SendMessage 发送一条或多条消息（每个收件人创建独立记录）。
// 返回发给第一个收件人的消息用于确认提示。
func SendMessage(from string, to []string, content string) (*model.Message, error) {
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

	for i, recipient := range to {
		msg := MessageORM{
			SenderName: from,
			To:         recipient,
			Content:    content,
			ReadBitmap: []byte{0},
			CreatedAt:  now,
		}
		if err := core.DB.Create(&msg).Error; err != nil {
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
// showAll=false：只返回当前用户未读的消息，并自动标记已读。
// showAll=true：返回全部消息（忽略已读状态标记）。
// limit<=0 时不限制返回数量。
func ReceiveMessages(from string, currentUserName string, showAll bool, limit int) ([]*model.Message, error) {
	userID := getUserID(currentUserName)

	var messages []MessageORM
	query := core.DB.Model(&MessageORM{})
	if currentUserName != "" {
		// 直接按 To 字段过滤，不再需要 JOIN
		query = query.Where("`to` = ?", currentUserName)
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

		msg := &model.Message{
			ID:        m.ID,
			From:      m.SenderName,
			To:        m.To,
			Content:   m.Content,
			Timestamp: m.CreatedAt.Format(time.RFC3339),
			Read:      IsReadBy(m.ReadBitmap, userID),
		}
		result = append(result, msg)

		if !showAll {
			newBitmap := MarkReadBy(m.ReadBitmap, userID)
			core.DB.Model(&MessageORM{}).Where("id = ?", m.ID).Update("read_bitmap", newBitmap)
		}
	}

	return result, nil
}

// GetHistory 获取通信历史记录。
// withUser 不为空时，返回该用户发送或接收的消息。
func GetHistory(withUser string, limit int) ([]*model.Message, error) {
	if limit <= 0 {
		limit = 10
	}

	var messages []MessageORM
	query := core.DB.Model(&MessageORM{}).Order("id DESC").Limit(limit)
	if withUser != "" {
		// 返回该用户发送或接收的消息
		query = query.Where("sender_name = ? OR `to` = ?", withUser, withUser)
	}
	query.Find(&messages)

	var result []*model.Message
	for i := len(messages) - 1; i >= 0; i-- {
		m := messages[i]
		result = append(result, &model.Message{
			ID:        m.ID,
			From:      m.SenderName,
			To:        m.To,
			Content:   m.Content,
			Timestamp: m.CreatedAt.Format(time.RFC3339),
		})
	}

	return result, nil
}

// getUserID 从 users.json 读取用户的 ID。
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
