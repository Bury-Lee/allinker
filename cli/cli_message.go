// cli_message.go —— send / recv / history 命令处理

package cli

import (
	"fmt"
	"os"
	"strings"

	"allinker/account"
	"allinker/config"
	msgpkg "allinker/message"
	"allinker/model"
)

// handleSend 处理 send 命令。
// 用法: allinker send [--to <接收方>] --msg <内容> --user <用户名>
func handleSend(cmd *CommandArg) error {
	toStr := cmd.To
	content := cmd.Msg
	username := cmd.User

	if username == "" {
		return &CLIError{Code: 4, Msg: "错误: 请使用 --user 指定操作者"}
	}
	if content == "" {
		return &CLIError{Code: 1, Msg: "错误: 请使用 --msg 指定消息内容"}
	}

	if toStr == "" {
		toStr = "All"
	}

	toList := strings.Split(toStr, ",")

	// 快捷指令: All 发送给所有已注册用户（排除自己）
	if len(toList) == 1 && strings.EqualFold(toList[0], "All") {
		allUsers, err := account.ListUsers()
		if err != nil {
			return &CLIError{Code: 1, Msg: fmt.Sprintf("错误: 获取用户列表失败: %v", err)}
		}
		toList = nil
		for _, u := range allUsers {
			if u.Name != username && u.Status == model.UserStatusActive {
				toList = append(toList, u.Name)
			}
		}
		if len(toList) == 0 {
			fmt.Fprintln(os.Stderr, "提示: 没有其他活跃用户可发送")
			return nil
		}
	}

	msg, err := msgpkg.HandleSend(msgpkg.SendParams{
		From:    username,
		To:      toList,
		Content: content,
	})
	if err != nil {
		return &CLIError{Code: 1, Msg: fmt.Sprintf("错误: 发送失败: %v", err)}
	}

	config.AppendAuditLog(model.AuditEntry{
		Time:   msg.Timestamp,
		User:   username,
		Action: "send",
		Target: strings.Join(toList, ","),
		Result: "success",
		Detail: fmt.Sprintf("msg_id:%d,content:%s", msg.ID, content),
	})

	if cmd.HumanMode {
		fmt.Printf("消息已发送 (ID: %d)\n", msg.ID)
		fmt.Printf("   发送方: %s\n", msg.From)
		fmt.Printf("   接收方: %s\n", msg.To)
		fmt.Printf("   时间: %s\n", msg.Timestamp)
	} else {
		fmt.Printf("消息已发送 (ID: %d)\n", msg.ID)
	}
	return nil
}

// handleRecv 处理 recv 命令。
// 用法: allinker recv [--from <发送方>] [--id <ID>] [--limit <数量>]
func handleRecv(cmd *CommandArg) error {
	from := cmd.From
	showAll := cmd.All
	idStr := cmd.MsgID
	username := cmd.User

	if username == "" {
		return &CLIError{Code: 4, Msg: "错误: 请使用 --user 指定操作者"}
	}

	var msgID int64
	if idStr != "" {
		fmt.Sscanf(idStr, "%d", &msgID)
	}

	// 按 ID 获取单条消息
	if msgID > 0 {
		messages, err := msgpkg.HandleRecv(msgpkg.RecvParams{
			From: "",
			User: username,
			All:  true,
			ID:   0,
		})
		if err != nil {
			return &CLIError{Code: 1, Msg: fmt.Sprintf("错误: 获取消息失败: %v", err)}
		}
		var found *model.Message
		for _, m := range messages {
			if m.ID == msgID {
				found = m
				break
			}
		}
		if found == nil {
			fmt.Println("提示: 消息不存在")
			return nil
		}
		m := found
		if cmd.HumanMode {
			fmt.Printf("[%d] %s  %s -> %s\n", m.ID, m.Timestamp, m.From, m.To)
			fmt.Printf("    %q\n", m.Content)
			fmt.Println("状态: 已标记为已读")
		} else {
			fmt.Printf("%s: %s\n", m.From, m.Content)
		}
		return nil
	}

	messages, err := msgpkg.HandleRecv(msgpkg.RecvParams{
		From: from,
		User: username,
		All:  showAll,
		ID:   0,
	})
	if err != nil {
		return &CLIError{Code: 1, Msg: fmt.Sprintf("错误: 接收消息失败: %v", err)}
	}

	if cmd.HumanMode {
		fmt.Printf("未读消息 (共计%d条):\n\n", len(messages))
		for _, msg := range messages {
			fmt.Printf("[%d] %s  %s -> %s\n", msg.ID, msg.Timestamp, msg.From, msg.To)
			fmt.Printf("    %q\n", msg.Content)
			fmt.Println()
		}
		fmt.Println("状态: 已标记为已读")
	} else {
		for _, msg := range messages {
			fmt.Printf("%s: %s\n", msg.From, msg.Content)
		}
	}
	return nil
}

// handleHistory 处理 history 命令,可以查询到私信内容。
// 用法: allinker history [--with <用户>] [--limit <条数>]
func handleHistory(cmd *CommandArg) error {
	withUser := cmd.With
	limit := cmd.Limit
	if limit == 0 {
		limit = 50
	}

	messages, err := msgpkg.HandleHistory(msgpkg.HistoryParams{
		WithUser: withUser,
		Limit:    limit,
	})
	if err != nil {
		return &CLIError{Code: 1, Msg: fmt.Sprintf("错误: 查询历史记录失败: %v", err)}
	}

	if len(messages) == 0 {
		if withUser != "" {
			fmt.Printf("提示: 与 %s 没有通信记录\n", withUser)
		} else {
			fmt.Println("提示: 没有通信记录")
		}
		return nil
	}

	if cmd.HumanMode {
		withHint := ""
		if withUser != "" {
			withHint = fmt.Sprintf("与 %s ", withUser)
		}
		fmt.Printf("对话记录 %s(共计%d条):\n\n", withHint, len(messages))
		for _, msg := range messages {
			fmt.Printf("  %s  %s -> %s: %q\n", msg.Timestamp, msg.From, msg.To, msg.Content)
		}
	} else {
		for _, msg := range messages {
			fmt.Printf("%s -> %s: %s\n", msg.From, msg.To, msg.Content)
		}
	}
	return nil
}
