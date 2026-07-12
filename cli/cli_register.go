// cli_register.go —— register 命令处理

package cli

import (
	"fmt"

	"allinker/account"
	"allinker/model"
)

// handleRegister 处理 register 命令
// 用法: allinker register --name <用户名> [--role admin|agent|guest] [--desc <岗位描述>]
func handleRegister(cmd *CommandArg) error {
	name := cmd.Name
	if name == "" {
		return &CLIError{Code: 1, Msg: "错误：请使用 --name 指定用户名"}
	}

	role := cmd.Role
	if role == "" {
		role = string(model.RoleAgent)
	}
	desc := cmd.Desc

	user, err := account.HandleRegister(account.RegisterParams{
		Name:        name,
		Role:        model.UserRole(role),
		Description: desc,
	})
	if err != nil {
		return &CLIError{Code: 1, Msg: fmt.Sprintf("错误：注册失败: %v", err)}
	}

	if cmd.HumanMode {
		fmt.Printf("成功：账号已注册 %s (角色: %s)\n", user.Name, user.Role)
		if user.Description != "" {
			fmt.Printf("  岗位描述: %s\n", user.Description)
		}
		fmt.Printf("提示：请在所有操作中使用 --user %s 签名\n", user.Name)
	} else {
		fmt.Printf("成功：账号已注册 %s\n", user.Name)
	}
	return nil
}
