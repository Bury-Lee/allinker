// Package account 提供用户注册、验证和管理功能。
//
// 所有需要 --user 参数的操作都通过 VerifyUser 进行检查：
//  1. 账户是否存在
//  2. 账户是否被禁用
//  3. 角色权限（由调用方检查）
package account

import (
	"fmt"
	"strings"
	"time"

	"allinker/config"
	"allinker/core"
	"allinker/model"
)

// Register 创建一个新的用户账户。
func Register(name string, role model.UserRole) (*model.User, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("username cannot be empty")
	}
	if role == "" {
		role = model.RoleAgent
	}
	if role != model.RoleAdmin && role != model.RoleAgent && role != model.RoleGuest {
		return nil, fmt.Errorf("invalid role %q, use: admin, agent, guest", role)
	}

	users := &model.UsersFile{}
	if err := config.ReadJSON(core.Global.UsersPath(), users); err != nil {
		return nil, fmt.Errorf("read users failed: %w", err)
	}
	if _, exists := users.Users[name]; exists {
		return nil, fmt.Errorf("user %q already exists", name)
	}

	// 分配唯一数字 ID（用于位图下标）
	id, err := config.NextID()
	if err != nil {
		return nil, fmt.Errorf("分配用户 ID 失败: %w", err)
	}

	user := &model.User{
		ID:      id,
		Name:    name,
		Role:    role,
		Created: time.Now().UTC().Format(time.RFC3339),
		Status:  model.UserStatusActive,
		Meta:    make(map[string]string),
	}
	users.Users[name] = user

	if err := config.WriteJSON(core.Global.UsersPath(), users); err != nil {
		return nil, fmt.Errorf("save users failed: %w", err)
	}

	config.AppendAuditLog(model.AuditEntry{
		Time:   time.Now().UTC().Format(time.RFC3339),
		User:   name,
		Action: "register",
		Result: "success",
		Detail: fmt.Sprintf("role: %s, id: %d", role, id),
	})

	return user, nil
}

// VerifyUser 检查用户是否存在且未被禁用。
func VerifyUser(username string) (*model.User, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, fmt.Errorf("use --user to specify the operator")
	}

	users := &model.UsersFile{}
	if err := config.ReadJSON(core.Global.UsersPath(), users); err != nil {
		return nil, fmt.Errorf("read users failed: %w", err)
	}

	user, exists := users.Users[username]
	if !exists {
		available := make([]string, 0, len(users.Users))
		for name := range users.Users {
			available = append(available, name)
		}
		return nil, fmt.Errorf("user %q not found\n  available: %s",
			username, strings.Join(available, ", "))
	}

	if user.Status == model.UserStatusDisabled {
		reason := user.DisabledReason
		if reason == "" {
			reason = "unspecified"
		}
		return nil, fmt.Errorf("user %q is disabled (reason: %s)", username, reason)
	}

	return user, nil
}

// CheckRole 验证用户是否至少具有所要求的角色等级。
// 角色层级：admin > agent > guest。
func CheckRole(user *model.User, requiredRole model.UserRole) bool {
	roleLevel := map[model.UserRole]int{
		model.RoleAdmin: 3,
		model.RoleAgent: 2,
		model.RoleGuest: 1,
	}
	userLevel, ok := roleLevel[user.Role]
	if !ok {
		return false
	}
	requiredLevel, ok := roleLevel[requiredRole]
	if !ok {
		return false
	}
	return userLevel >= requiredLevel
}

// ListUsers 返回所有已注册的用户。
func ListUsers() ([]*model.User, error) {
	users := &model.UsersFile{}
	if err := config.ReadJSON(core.Global.UsersPath(), users); err != nil {
		return nil, fmt.Errorf("read users failed: %w", err)
	}
	result := make([]*model.User, 0, len(users.Users))
	for _, u := range users.Users {
		result = append(result, u)
	}
	return result, nil
}

// DisableUser 禁用一个用户账户。
func DisableUser(targetName, reason, operator string) error {
	users := &model.UsersFile{}
	if err := config.ReadJSON(core.Global.UsersPath(), users); err != nil {
		return fmt.Errorf("read users failed: %w", err)
	}
	user, exists := users.Users[targetName]
	if !exists {
		return fmt.Errorf("user %q not found", targetName)
	}
	if user.Status == model.UserStatusDisabled {
		return fmt.Errorf("user %q is already disabled", targetName)
	}
	user.Status = model.UserStatusDisabled
	user.DisabledReason = reason
	if err := config.WriteJSON(core.Global.UsersPath(), users); err != nil {
		return fmt.Errorf("save users failed: %w", err)
	}
	config.AppendAuditLog(model.AuditEntry{
		Time:   time.Now().UTC().Format(time.RFC3339),
		User:   operator,
		Action: "user_disable",
		Target: targetName,
		Result: "success",
		Detail: fmt.Sprintf("reason: %s", reason),
	})
	return nil
}

// EnableUser 重新启用一个已被禁用的用户账户。
func EnableUser(targetName, operator string) error {
	users := &model.UsersFile{}
	if err := config.ReadJSON(core.Global.UsersPath(), users); err != nil {
		return fmt.Errorf("read users failed: %w", err)
	}
	user, exists := users.Users[targetName]
	if !exists {
		return fmt.Errorf("user %q not found", targetName)
	}
	if user.Status == model.UserStatusActive {
		return fmt.Errorf("user %q is already active", targetName)
	}
	user.Status = model.UserStatusActive
	user.DisabledReason = ""
	if err := config.WriteJSON(core.Global.UsersPath(), users); err != nil {
		return fmt.Errorf("save users failed: %w", err)
	}
	config.AppendAuditLog(model.AuditEntry{
		Time:   time.Now().UTC().Format(time.RFC3339),
		User:   operator,
		Action: "user_enable",
		Target: targetName,
		Result: "success",
	})
	return nil
}

// DeleteUser 永久删除一个用户账户。
func DeleteUser(targetName, operator string) error {
	users := &model.UsersFile{}
	if err := config.ReadJSON(core.Global.UsersPath(), users); err != nil {
		return fmt.Errorf("read users failed: %w", err)
	}
	if _, exists := users.Users[targetName]; !exists {
		return fmt.Errorf("user %q not found", targetName)
	}
	delete(users.Users, targetName)
	if err := config.WriteJSON(core.Global.UsersPath(), users); err != nil {
		return fmt.Errorf("save users failed: %w", err)
	}
	config.AppendAuditLog(model.AuditEntry{
		Time:   time.Now().UTC().Format(time.RFC3339),
		User:   operator,
		Action: "user_delete",
		Target: targetName,
		Result: "success",
	})
	return nil
}
