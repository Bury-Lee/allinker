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
// 不指定 --to 时默认为 All（发送给所有活跃用户）
func handleSend(args []string, humanMode bool) {
	toStr, remaining := parseStringArg(args, "--to", "")
	if toStr == "" {
		toStr, remaining = parseStringArg(remaining, "--at", "") // 兼容旧参数
	}
	content, remaining := parseStringArg(remaining, "--msg", "")
	username, _ := requireUser(remaining)
	if content == "" {
		fmt.Fprintln(os.Stderr, "错误: 请使用 --msg 指定消息内容")
		os.Exit(1)
	}

	if toStr == "" {
		// 不指定 --to 时默认为 All
		toStr = "All"
	}

	toList := strings.Split(toStr, ",")

	// 快捷指令: All 发送给所有已注册用户（排除自己）
	if len(toList) == 1 && strings.EqualFold(toList[0], "All") {
		allUsers, err := account.ListUsers()
		if err == nil {
			toList = nil
			for _, u := range allUsers {
				if u.Name != username && u.Status == model.UserStatusActive {
					toList = append(toList, u.Name)
				}
			}
		}
		if len(toList) == 0 {
			fmt.Fprintln(os.Stderr, "提示: 没有其他活跃用户可发送")
			os.Exit(0)
		}
	}

	msg, err := msgpkg.SendMessage(username, toList, content)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: 发送失败: %v\n", err)
		os.Exit(1)
	}

	config.AppendAuditLog(model.AuditEntry{
		Time:   msg.Timestamp,
		User:   username,
		Action: "send",
		Target: strings.Join(toList, ","),
		Result: "success",
		Detail: fmt.Sprintf("msg_id:%d,content:%s", msg.ID, content),
	})

	if humanMode {
		fmt.Printf("消息已发送 (ID: %d)\n", msg.ID)
		fmt.Printf("   发送方: %s\n", msg.From)
		fmt.Printf("   接收方: %s\n", msg.To)
		fmt.Printf("   时间: %s\n", msg.Timestamp)
	} else {
		fmt.Printf("消息已发送 (ID: %d)\n", msg.ID)
	}
}

// handleRecv 处理 recv 命令。
// 用法: allinker recv [--from <发送方>] [--id <ID>] [--limit <数量>]
func handleRecv(args []string, humanMode bool) {
	from, remaining := parseStringArg(args, "--from", "")
	showAll, remaining := parseBoolArg(remaining, "--all")
	idStr, remaining := parseStringArg(remaining, "--id", "")
	limit, remaining := parseIntArg(remaining, "--limit", 0)
	// --user 最后解析，用 requireUser 做校验
	username, remaining := requireUser(remaining)
	_ = remaining // 剩余参数忽略

	var msgID int64
	if idStr != "" {
		fmt.Sscanf(idStr, "%d", &msgID)
	}

	// 按 ID 获取单条消息
	if msgID > 0 {
		messages, err := msgpkg.ReceiveMessages("", username, true, 0) // 忽略 from 过滤，全量获取后按 ID 筛选
		if err != nil {
			fmt.Fprintf(os.Stderr, "错误: 获取消息失败: %v\n", err)
			os.Exit(1)
		}
		// 找到指定 ID 的消息
		var found *model.Message
		for _, m := range messages {
			if m.ID == msgID {
				found = m
				break
			}
		}
		if found == nil {
			fmt.Println("提示: 消息不存在")
			return
		}
		m := found
		if humanMode {
			fmt.Printf("[%d] %s  %s -> %s\n", m.ID, m.Timestamp, m.From, m.To)
			fmt.Printf("    %q\n", m.Content)
			fmt.Println("状态: 已标记为已读")
		} else {
			fmt.Printf("%s: %s\n", m.From, m.Content)
		}
		return
	}

	messages, err := msgpkg.ReceiveMessages(from, username, showAll, limit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: 接收消息失败: %v\n", err)
		os.Exit(1)
	}

	if humanMode {
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
}

// handleHistory 处理 history 命令,可以查询到私信内容。
// 用法: allinker history [--with <用户>] [--limit <条数>]
func handleHistory(args []string, humanMode bool) {
	withUser, remaining := parseStringArg(args, "--with", "")
	limit, _ := parseIntArg(remaining, "--limit", 50)

	messages, err := msgpkg.GetHistory(withUser, limit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: 查询历史记录失败: %v\n", err)
		os.Exit(1)
	}

	if len(messages) == 0 {
		if withUser != "" {
			fmt.Printf("提示: 与 %s 没有通信记录\n", withUser)
		} else {
			fmt.Println("提示: 没有通信记录")
		}
		return
	}

	if humanMode {
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
}
