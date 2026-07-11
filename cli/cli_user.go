// cli_user.go — user 命令处理（管理员功能）
package cli

import (
	"fmt"
	"os"
	"strings"

	"allinker/account"
	initt "allinker/init"
	"allinker/model"
)

// handleUser 处理 user 子命令
//
// 用法:
//
//	allinker user list --user <用户名>
//	allinker user log --name <目标用户> --user <用户名>
//	allinker user disable --name <目标用户> --reason <原因> --user <用户名>
//	allinker user enable --name <目标用户> --user <用户名>
//	allinker user delete --name <目标用户> --user <用户名>
func handleUser(args []string, humanMode bool) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "请指定 user 子命令: list, log, disable, enable, delete")
		os.Exit(1)
	}

	subCmd := args[0]
	subArgs := args[1:]

	switch subCmd {
	case "list":
		handleUserList(subArgs, humanMode)
	case "log":
		handleUserLog(subArgs, humanMode)
	case "disable":
		handleUserDisable(subArgs, humanMode)
	case "enable":
		handleUserEnable(subArgs, humanMode)
	case "delete":
		handleUserDelete(subArgs, humanMode)
	default:
		fmt.Fprintf(os.Stderr, "未知 user 子命令: %s\n", subCmd)
		fmt.Fprintln(os.Stderr, "   可用: list, log, disable, enable, delete")
		os.Exit(1)
	}
}

// handleUserList 查看所有用户列表
func handleUserList(args []string, humanMode bool) {
	username, _ := requireUser(args)

	// 检查 admin 权限
	user, err := account.VerifyUser(username)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(4)
	}
	if !account.CheckRole(user, model.RoleAdmin) {
		fmt.Fprintln(os.Stderr, "错误: 权限不足，仅管理员可查看用户列表")
		os.Exit(5)
	}

	users, err := account.ListUsers()
	if err != nil {
		fmt.Fprintf(os.Stderr, "查询用户列表失败: %v\n", err)
		os.Exit(1)
	}

	if humanMode {
		fmt.Printf("👥 用户列表 (%d)\n\n", len(users))
		fmt.Printf("  角色    用户名    状态      注册时间\n")
		fmt.Printf("  ──────────────────────────────────────\n")
		for _, u := range users {
			statusStr := "🟢 正常"
			extra := ""
			if u.Status == model.UserStatusDisabled {
				statusStr = "🔴 已禁用"
				if u.DisabledReason != "" {
					extra = fmt.Sprintf(" (原因: %s)", u.DisabledReason)
				}
			}
			created := u.Created
			if len(created) > 19 {
				created = created[:19]
			}
			created = strings.Replace(created, "T", " ", 1)
			fmt.Printf("  %-7s %-8s %s%s\n", u.Role, u.Name, statusStr, extra)
			fmt.Printf("                      %s%s\n", created, extra)
		}
	} else {
		for _, u := range users {
			statusStr := "正常"
			if u.Status == model.UserStatusDisabled {
				statusStr = "已禁用"
			}
			fmt.Printf("%s (%s, %s)\n", u.Name, u.Role, statusStr)
		}
	}
}

// handleUserLog 查看用户操作记录
func handleUserLog(args []string, humanMode bool) {
	targetName, remaining := parseStringArg(args, "--name", "")
	username, remaining := requireUser(remaining)

	// 检查 admin 权限
	user, err := account.VerifyUser(username)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(4)
	}
	if !account.CheckRole(user, model.RoleAdmin) {
		fmt.Fprintln(os.Stderr, "错误: 权限不足，仅管理员可查看操作记录")
		os.Exit(5)
	}

	if targetName == "" {
		fmt.Fprintln(os.Stderr, "请使用 --name 指定目标用户")
		os.Exit(1)
	}

	limit, _ := parseIntArg(remaining, "--limit", 50)
	since, _ := parseStringArg(remaining, "--since", "")
	actionType, _ := parseStringArg(remaining, "--type", "")

	// 读取审计日志
	entries, err := initt.ReadAuditLog()
	if err != nil {
		fmt.Fprintf(os.Stderr, "读取审计日志失败: %v\n", err)
		os.Exit(1)
	}

	// 筛选
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

	// 取最近的 limit 条
	if len(filtered) > limit {
		filtered = filtered[len(filtered)-limit:]
	}

	if len(filtered) == 0 {
		fmt.Printf("📋 %s 没有操作记录\n", targetName)
		return
	}

	if humanMode {
		fmt.Printf("📋 %s 的操作记录 (最近 %d 条", targetName, len(filtered))
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
}

// handleUserDisable 禁用账号
func handleUserDisable(args []string, humanMode bool) {
	targetName, remaining := parseStringArg(args, "--name", "")
	reason, remaining := parseStringArg(remaining, "--reason", "")
	username, _ := requireUser(remaining)

	// 检查 admin 权限
	user, err := account.VerifyUser(username)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(4)
	}
	if !account.CheckRole(user, model.RoleAdmin) {
		fmt.Fprintln(os.Stderr, "错误: 权限不足，仅管理员可禁用账号")
		os.Exit(5)
	}
	if targetName == "" {
		fmt.Fprintln(os.Stderr, "请使用 --name 指定要禁用的用户")
		os.Exit(1)
	}

	err = account.DisableUser(targetName, reason, username)
	if err != nil {
		fmt.Fprintf(os.Stderr, "禁用失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("账号已禁用: %s\n", targetName)
}

// handleUserEnable 启用账号
func handleUserEnable(args []string, humanMode bool) {
	targetName, remaining := parseStringArg(args, "--name", "")
	username, _ := requireUser(remaining)

	user, err := account.VerifyUser(username)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(4)
	}
	if !account.CheckRole(user, model.RoleAdmin) {
		fmt.Fprintln(os.Stderr, "错误: 权限不足，仅管理员可启用账号")
		os.Exit(5)
	}
	if targetName == "" {
		fmt.Fprintln(os.Stderr, "请使用 --name 指定要启用的用户")
		os.Exit(1)
	}

	err = account.EnableUser(targetName, username)
	if err != nil {
		fmt.Fprintf(os.Stderr, "启用失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("账号已启用: %s\n", targetName)
}

// handleUserDelete 删除账号
func handleUserDelete(args []string, humanMode bool) {
	targetName, remaining := parseStringArg(args, "--name", "")
	username, _ := requireUser(remaining)

	user, err := account.VerifyUser(username)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(4)
	}
	if !account.CheckRole(user, model.RoleAdmin) {
		fmt.Fprintln(os.Stderr, "错误: 权限不足，仅管理员可删除账号")
		os.Exit(5)
	}
	if targetName == "" {
		fmt.Fprintln(os.Stderr, "请使用 --name 指定要删除的用户")
		os.Exit(1)
	}

	err = account.DeleteUser(targetName, username)
	if err != nil {
		fmt.Fprintf(os.Stderr, "删除失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("账号已删除: %s\n", targetName)
}

