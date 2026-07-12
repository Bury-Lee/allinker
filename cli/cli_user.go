// cli_user.go — user 命令处理（管理员功能）
package cli

import (
	"fmt"
	"strings"

	"allinker/account"
	initt "allinker/init"
	"allinker/model"
)

// handleUser 处理 user 子命令
func handleUser(cmd *CommandArg) error {
	switch cmd.SubCommand {
	case "list":
		return handleUserList(cmd)
	case "log":
		return handleUserLog(cmd)
	case "disable":
		return handleUserDisable(cmd)
	case "enable":
		return handleUserEnable(cmd)
	case "delete":
		return handleUserDelete(cmd)
	default:
		return &CLIError{Code: 1, Msg: fmt.Sprintf("未知 user 子命令: %s\n   可用: list, log, disable, enable, delete", cmd.SubCommand)}
	}
}

// handleUserList 查看所有用户列表
func handleUserList(cmd *CommandArg) error {
	username := cmd.User
	if username == "" {
		return &CLIError{Code: 4, Msg: "错误: 请使用 --user 指定操作者"}
	}

	users, err := account.ListUsers()
	if err != nil {
		return &CLIError{Code: 1, Msg: fmt.Sprintf("查询用户列表失败: %v", err)}
	}

	if cmd.HumanMode {
		fmt.Printf("用户列表 (%d)\n\n", len(users))
		fmt.Printf("  角色    用户名    描述                状态      注册时间\n")
		fmt.Printf("  ─────────────────────────────────────────────────────────\n")
		for _, u := range users {
			statusStr := "正常"
			extra := ""
			if u.Status == model.UserStatusDisabled {
				statusStr = "已禁用"
				if u.DisabledReason != "" {
					extra = fmt.Sprintf(" (原因: %s)", u.DisabledReason)
				}
			}
			created := u.Created
			if len(created) > 19 {
				created = created[:19]
			}
			created = strings.Replace(created, "T", " ", 1)
			desc := u.Description
			if desc == "" {
				desc = "-"
			}
			fmt.Printf("  %-7s %-8s %-20s %s%s\n", u.Role, u.Name, desc, statusStr, extra)
			fmt.Printf("                      %-20s %s%s\n", "", created, extra)
		}
	} else {
		for _, u := range users {
			statusStr := "正常"
			if u.Status == model.UserStatusDisabled {
				statusStr = "已禁用"
			}
			if u.Description != "" {
				fmt.Printf("%s (%s, %s, %s)\n", u.Name, u.Role, u.Description, statusStr)
			} else {
				fmt.Printf("%s (%s, %s)\n", u.Name, u.Role, statusStr)
			}
		}
	}
	return nil
}

// handleUserLog 查看用户操作记录
func handleUserLog(cmd *CommandArg) error {
	targetName := cmd.Name
	username := cmd.User
	if username == "" {
		return &CLIError{Code: 4, Msg: "错误: 请使用 --user 指定操作者"}
	}

	user, err := account.VerifyUser(username)
	if err != nil {
		return &CLIError{Code: 4, Msg: fmt.Sprintf("%v", err)}
	}
	if !account.CheckRole(user, model.RoleAdmin) {
		return &CLIError{Code: 5, Msg: "错误: 权限不足，仅管理员可查看操作记录"}
	}

	if targetName == "" {
		return &CLIError{Code: 1, Msg: "请使用 --name 指定目标用户"}
	}

	limit := cmd.Limit
	if limit == 0 {
		limit = 50
	}
	since := cmd.Since
	actionType := cmd.ActionType

	entries, err := initt.ReadAuditLog()
	if err != nil {
		return &CLIError{Code: 1, Msg: fmt.Sprintf("读取审计日志失败: %v", err)}
	}

	var filtered []model.AuditEntry
	for _, entry := range entries {
		if entry.User != targetName {
			continue
		}
		if since != "" && entry.Time < since {
			continue
		}
		if actionType != "" && entry.Action != actionType {
			continue
		}
		filtered = append(filtered, entry)
	}

	if len(filtered) > limit {
		filtered = filtered[len(filtered)-limit:]
	}

	if len(filtered) == 0 {
		fmt.Printf("%s 没有操作记录\n", targetName)
		return nil
	}

	if cmd.HumanMode {
		fmt.Printf("%s 的操作记录 (最近 %d 条", targetName, len(filtered))
		if actionType != "" {
			fmt.Printf(", 筛选: %s", actionType)
		}
		fmt.Println(")")
		for _, entry := range filtered {
			timeStr := entry.Time
			if len(timeStr) > 19 {
				timeStr = timeStr[11:19]
			}
			resultStr := "成功"
			if entry.Result == "failure" {
				resultStr = "失败"
			} else if entry.Result == "timeout" {
				resultStr = "超时"
			}
			fmt.Printf("  %s  %s %s (%s)\n", timeStr, entry.Action, entry.Target, resultStr)
			if entry.Detail != "" {
				fmt.Printf("       %s\n", entry.Detail)
			}
		}
	} else {
		for _, entry := range filtered {
			fmt.Printf("%s %s %s %s\n", entry.Time, entry.Action, entry.Target, entry.Result)
		}
	}
	return nil
}

// handleUserDisable 禁用账号
func handleUserDisable(cmd *CommandArg) error {
	targetName := cmd.Name
	reason := cmd.Reason
	username := cmd.User
	if username == "" {
		return &CLIError{Code: 4, Msg: "错误: 请使用 --user 指定操作者"}
	}

	user, err := account.VerifyUser(username)
	if err != nil {
		return &CLIError{Code: 4, Msg: fmt.Sprintf("%v", err)}
	}
	if !account.CheckRole(user, model.RoleAdmin) {
		return &CLIError{Code: 5, Msg: "错误: 权限不足，仅管理员可禁用账号"}
	}
	if targetName == "" {
		return &CLIError{Code: 1, Msg: "请使用 --name 指定要禁用的用户"}
	}

	err = account.HandleDisable(account.UserManageParams{
		Target:   targetName,
		Reason:   reason,
		Operator: username,
	})
	if err != nil {
		return &CLIError{Code: 1, Msg: fmt.Sprintf("禁用失败: %v", err)}
	}

	fmt.Printf("账号已禁用: %s\n", targetName)
	return nil
}

// handleUserEnable 启用账号
func handleUserEnable(cmd *CommandArg) error {
	targetName := cmd.Name
	username := cmd.User
	if username == "" {
		return &CLIError{Code: 4, Msg: "错误: 请使用 --user 指定操作者"}
	}

	user, err := account.VerifyUser(username)
	if err != nil {
		return &CLIError{Code: 4, Msg: fmt.Sprintf("%v", err)}
	}
	if !account.CheckRole(user, model.RoleAdmin) {
		return &CLIError{Code: 5, Msg: "错误: 权限不足，仅管理员可启用账号"}
	}
	if targetName == "" {
		return &CLIError{Code: 1, Msg: "请使用 --name 指定要启用的用户"}
	}

	err = account.HandleEnable(account.UserManageParams{
		Target:   targetName,
		Operator: username,
	})
	if err != nil {
		return &CLIError{Code: 1, Msg: fmt.Sprintf("启用失败: %v", err)}
	}

	fmt.Printf("账号已启用: %s\n", targetName)
	return nil
}

// handleUserDelete 删除账号
func handleUserDelete(cmd *CommandArg) error {
	targetName := cmd.Name
	username := cmd.User
	if username == "" {
		return &CLIError{Code: 4, Msg: "错误: 请使用 --user 指定操作者"}
	}

	user, err := account.VerifyUser(username)
	if err != nil {
		return &CLIError{Code: 4, Msg: fmt.Sprintf("%v", err)}
	}
	if !account.CheckRole(user, model.RoleAdmin) {
		return &CLIError{Code: 5, Msg: "错误: 权限不足，仅管理员可删除账号"}
	}
	if targetName == "" {
		return &CLIError{Code: 1, Msg: "请使用 --name 指定要删除的用户"}
	}

	err = account.HandleDelete(account.UserManageParams{
		Target:   targetName,
		Operator: username,
	})
	if err != nil {
		return &CLIError{Code: 1, Msg: fmt.Sprintf("删除失败: %v", err)}
	}

	fmt.Printf("账号已删除: %s\n", targetName)
	return nil
}
