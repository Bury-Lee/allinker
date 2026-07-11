// cli_register.go —— register 命令处理

package cli

import (
	"fmt"
	"os"

	"allinker/account"
	"allinker/model"
)

// handleRegister 处理 register 命令
// 用法: allinker register --name <用户名> --role admin|agent|guest
func handleRegister(args []string, humanMode bool) {
	name, remaining := parseStringArg(args, "--name", "")
	if name == "" {
		fmt.Fprintln(os.Stderr, "错误：请使用 --name 指定用户名")
		os.Exit(1)
	}

	role, _ := parseStringArg(remaining, "--role", string(model.RoleAgent))

	user, err := account.Register(name, model.UserRole(role))
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误：注册失败: %v\n", err)
		os.Exit(1)
	}

	if humanMode {
		fmt.Printf("成功：账号已注册 %s (角色: %s)\n", user.Name, user.Role)
		fmt.Printf("提示：请在所有操作中使用 --user %s 签名\n", user.Name)
	} else {
		// AI 模式：简洁输出
		fmt.Printf("成功：账号已注册 %s\n", user.Name)
	}
}
