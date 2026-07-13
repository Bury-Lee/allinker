// cli_chat.go — chat 命令处理
// 人类聊天室：实时查看 AI 对话，可选参与发言。

// TODO:这里一般是做校验层,到时候应该把实现代码移去业务层
package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"allinker/account"
	msgpkg "allinker/message"
	"allinker/model"
)

// 用户颜色调色板（ANSI 亮色前景，保证在深色背景上可读）
var userColors = []string{
	"\033[1;31m", // 亮红
	"\033[1;32m", // 亮绿
	"\033[1;33m", // 亮黄
	"\033[1;34m", // 亮蓝
	"\033[1;35m", // 亮紫
	"\033[1;36m", // 亮青
	"\033[1;91m", // 亮浅红
	"\033[1;92m", // 亮浅绿
	"\033[1;93m", // 亮浅黄
	"\033[1;94m", // 亮浅蓝
	"\033[1;95m", // 亮浅紫
	"\033[1;96m", // 亮浅青
}

const colorReset = "\033[0m"

// userNameColor 根据用户名哈希返回对应的 ANSI 颜色码。
// 同一个名字总是映射到同一种颜色。
func userNameColor(name string) string {
	h := uint64(0)
	for i := 0; i < len(name); i++ {
		h = h*31 + uint64(name[i])
	}
	return userColors[h%uint64(len(userColors))]
}

// handleChat 处理 chat 命令
// 用法: allinker chat [--user <用户名>] [--interval <秒>]
// --user  可选，指定后可在聊天室中发言（必须先注册）
// --interval 轮询间隔（默认 2 秒）
func handleChat(cmd *CommandArg) error {
	username := cmd.User
	interval := cmd.Timeout
	if interval <= 0 {
		interval = 2
	}

	// 如果指定了 --user，先验证用户是否存在且未被禁用
	if username != "" {
		if _, err := account.VerifyUser(username); err != nil {
			return &CLIError{Code: 4, Msg: fmt.Sprintf("错误: %v", err)}
		}
	}

	fmt.Println()
	fmt.Println("  ╔══════════════════════════════════════════════╗")
	fmt.Println("  ║        ALLinker Chat Room                    ║")
	fmt.Println("  ║        实时查看 AI 协作对话                   ║")
	fmt.Println("  ╚══════════════════════════════════════════════╝")
	fmt.Println()

	if username == "" {
		fmt.Println("  只读模式（使用 --user <用户名> 可参与发言）")
	} else {
		fmt.Printf("  参与身份: %s\n", username)
		fmt.Println("  输入消息后按 Enter 发送")
	}
	fmt.Println("  输入 /quit 或 /exit 退出")
	fmt.Println()

	lastID := int64(0)
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	// 用户输入通道
	inputCh := make(chan string)
	if username != "" {
		go func() {
			scanner := bufio.NewScanner(os.Stdin)
			for scanner.Scan() {
				inputCh <- scanner.Text()
			}
		}()
	}

	// 先拉一波历史消息（最近 20 条）
	initial, err := msgpkg.GetMessagesSince(0, 20)
	if err == nil && len(initial) > 0 {
		fmt.Println("  ─── 最近消息 ───")
		for _, m := range initial {
			displayMessage(m)
			if m.ID > lastID {
				lastID = m.ID
			}
		}
		fmt.Println()
	}

	// 首次输入提示，只有有发言权限才显示
	if username != "" {
		fmt.Print("> ")
	}

	for {
		select {
		case <-ticker.C:
			messages, err := msgpkg.GetMessagesSince(lastID, 0)
			if err != nil {
				continue
			}
			showed := false
			for _, m := range messages {
				if m.ID > lastID {
					if username != "" {
						fmt.Print("\r\033[K") // 清掉当前行的 `> `
					}
					displayMessage(m)
					lastID = m.ID
					showed = true
				}
			}
			// 只有确实显示了新消息，才重新绘输入提示
			if showed && username != "" {
				fmt.Print("> ")
			}

		case input := <-inputCh:
			input = strings.TrimSpace(input)
			if input == "/quit" || input == "/exit" {
				fmt.Println("\n  离开聊天室")
				return nil
			}
			if input == "" {
				fmt.Print("> ")
				continue
			}
			_, err := msgpkg.HandleSend(msgpkg.SendParams{
				From:    username,
				To:      []string{"All"},
				Content: input,
			})
			if err != nil {
				fmt.Printf("\r\033[K发送失败: %v\n> ", err)
			} else {
				// 自己发的消息会被轮询拉回来，不重复打印
				fmt.Print("> ")
			}
		}
	}
}

// displayMessage 格式化输出一条消息（发送者名字带颜色）
func displayMessage(m *model.Message) {
	ts := m.Timestamp
	if len(ts) > 19 {
		ts = ts[11:19] // 只显示 HH:MM:SS
	}
	color := userNameColor(m.From)
	fmt.Printf("  [%s] %s%s%s → %s: %s\n", ts, color, m.From, colorReset, m.To, m.Content)
}
