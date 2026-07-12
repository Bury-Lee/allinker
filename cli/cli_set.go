// cli_set.go — set 子命令处理
// 管理本地服务配置和远程连接配置。

package cli

import (
	"fmt"
	"sort"

	"allinker/config"
	"allinker/model"
	"allinker/utils"
)

// handleSet 处理 set 子命令。
func handleSet(cmd *CommandArg) error {
	switch cmd.SubCommand {
	case "server":
		return handleSetServer(cmd)
	case "remote":
		return handleSetRemote(cmd)
	default:
		return &CLIError{Code: 1, Msg: "错误: 请指定 set 子命令: server, remote\n   示例: allinker set server --port 8080\n         allinker set remote --name 电脑1 --addr 192.168.1.100:8080"}
	}
}

// handleSetServer 配置本地服务参数。
func handleSetServer(cmd *CommandArg) error {
	cfg, err := config.GetConfig()
	if err != nil {
		return &CLIError{Code: 1, Msg: fmt.Sprintf("错误: 读取配置失败: %v", err)}
	}

	hasHost := cmd.Host != ""
	hasPort := cmd.Port > 0
	hasToken := cmd.Token != ""

	if !hasHost && !hasPort && !hasToken {
		if cmd.HumanMode {
			fmt.Printf("当前服务配置:\n")
			fmt.Printf("  host: %s\n", cfg.Server.Host)
			fmt.Printf("  port: %d\n", cfg.Server.Port)
			if cfg.Server.AuthToken != "" {
				fmt.Printf("  auth: 已启用（Token已设置）\n")
			} else {
				fmt.Printf("  auth: 未启用（无密码）\n")
			}
		} else {
			fmt.Printf("host=%s port=%d auth=%v\n", cfg.Server.Host, cfg.Server.Port, cfg.Server.AuthToken != "")
		}
		return nil
	}

	if hasHost {
		cfg.Server.Host = cmd.Host
	}
	if hasPort {
		cfg.Server.Port = cmd.Port
	}
	if hasToken {
		// 存储 Token 的 SHA-256 哈希值，而非明文
		cfg.Server.AuthToken = utils.HashTokenSHA256(cmd.Token)
	}

	if err := config.SaveConfig(cfg); err != nil {
		return &CLIError{Code: 1, Msg: fmt.Sprintf("错误: 保存配置失败: %v", err)}
	}

	if cmd.HumanMode {
		fmt.Println("服务配置已更新:")
		fmt.Printf("  host: %s\n", cfg.Server.Host)
		fmt.Printf("  port: %d\n", cfg.Server.Port)
		if cfg.Server.AuthToken != "" {
			fmt.Printf("  auth: 已启用\n")
		} else {
			fmt.Printf("  auth: 未启用（无密码）\n")
		}
	} else {
		fmt.Println("配置已保存")
	}
	return nil
}

// handleSetRemote 管理远程连接。
func handleSetRemote(cmd *CommandArg) error {
	if cmd.List {
		return handleSetRemoteList(cmd.HumanMode)
	}

	name := cmd.Name
	addr := cmd.Addr
	token := cmd.Token
	doDelete := cmd.Delete

	if name == "" {
		return &CLIError{Code: 1, Msg: "错误: 请使用 --name 指定远程连接名称\n   示例: allinker set remote --name 电脑1 --addr 192.168.1.100:8080"}
	}

	cfg, err := config.GetConfig()
	if err != nil {
		return &CLIError{Code: 1, Msg: fmt.Sprintf("错误: 读取配置失败: %v", err)}
	}

	if cfg.Remotes == nil {
		cfg.Remotes = make(map[string]model.RemoteConfig)
	}

	if doDelete {
		if _, exists := cfg.Remotes[name]; !exists {
			return &CLIError{Code: 1, Msg: fmt.Sprintf("错误: 远程连接 '%s' 不存在", name)}
		}
		delete(cfg.Remotes, name)
		if err := config.SaveConfig(cfg); err != nil {
			return &CLIError{Code: 1, Msg: fmt.Sprintf("错误: 保存配置失败: %v", err)}
		}
		fmt.Printf("远程连接已删除: %s\n", name)
		return nil
	}

	if addr == "" {
		if rc, exists := cfg.Remotes[name]; exists {
			if cmd.HumanMode {
				fmt.Printf("远程连接 '%s':\n", name)
				fmt.Printf("  addr: %s\n", rc.Addr)
				if rc.Token != "" {
					fmt.Printf("  auth: 有密码\n")
				} else {
					fmt.Printf("  auth: 无密码\n")
				}
			} else {
				fmt.Printf("name=%s addr=%s auth=%v\n", name, rc.Addr, rc.Token != "")
			}
		} else {
			return &CLIError{Code: 1, Msg: fmt.Sprintf("错误: 远程连接 '%s' 不存在", name)}
		}
		return nil
	}

	rc := model.RemoteConfig{
		Addr:  addr,
		Token: token,
	}
	cfg.Remotes[name] = rc

	if err := config.SaveConfig(cfg); err != nil {
		return &CLIError{Code: 1, Msg: fmt.Sprintf("错误: 保存配置失败: %v", err)}
	}

	if cmd.HumanMode {
		fmt.Printf("远程连接已添加: %s → %s", name, addr)
		if token != "" {
			fmt.Print(" (有密码)")
		}
		fmt.Println()
	} else {
		fmt.Printf("远程连接已保存: %s\n", name)
	}
	return nil
}

// handleSetRemoteList 列出所有远程连接。
func handleSetRemoteList(humanMode bool) error {
	cfg, err := config.GetConfig()
	if err != nil {
		return &CLIError{Code: 1, Msg: fmt.Sprintf("错误: 读取配置失败: %v", err)}
	}

	if len(cfg.Remotes) == 0 {
		fmt.Println("没有配置远程连接")
		fmt.Println("   使用 allinker set remote --name <名称> --addr <地址> 添加")
		return nil
	}

	names := make([]string, 0, len(cfg.Remotes))
	for name := range cfg.Remotes {
		names = append(names, name)
	}
	sort.Strings(names)

	if humanMode {
		fmt.Printf("远程连接列表 (%d):\n\n", len(names))
		for _, name := range names {
			rc := cfg.Remotes[name]
			auth := "无密码"
			if rc.Token != "" {
				auth = "有密码"
			}
			fmt.Printf("  %s → %s (%s)\n", name, rc.Addr, auth)
		}
	} else {
		for _, name := range names {
			rc := cfg.Remotes[name]
			fmt.Printf("%s %s %v\n", name, rc.Addr, rc.Token != "")
		}
	}
	return nil
}
