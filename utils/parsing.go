// Package utils 提供 allinker 命令行工具的核心工具函数。
//
// 本文件提供了参数解析工具函数，供 CLI handler 和 Server handler 共用。
// 所有函数都是纯函数，不依赖全局状态。
package utils

import (
	"fmt"
	"strings"
)

// ParseIntArg 从参数列表中解析一个 int 类型的命名参数。
// 使用全新切片避免底层数组别名问题。
func ParseIntArg(args []string, name string, defaultVal int) (int, []string) {
	for i, arg := range args {
		if arg == name || arg == "-"+name {
			if i+1 < len(args) {
				val := 0
				if _, err := fmt.Sscanf(args[i+1], "%d", &val); err == nil {
					remaining := make([]string, 0, len(args)-2)
					remaining = append(remaining, args[:i]...)
					remaining = append(remaining, args[i+2:]...)
					return val, remaining
				}
			}
		}
		if strings.HasPrefix(arg, name+"=") || strings.HasPrefix(arg, "-"+name+"=") {
			parts := strings.SplitN(arg, "=", 2)
			if len(parts) == 2 {
				val := 0
				if _, err := fmt.Sscanf(parts[1], "%d", &val); err == nil {
					remaining := make([]string, 0, len(args)-1)
					remaining = append(remaining, args[:i]...)
					remaining = append(remaining, args[i+1:]...)
					return val, remaining
				}
			}
		}
	}
	return defaultVal, args
}

// ParseStringArg 从参数列表中解析一个 string 类型的命名参数。
// 使用全新切片避免底层数组别名问题。
func ParseStringArg(args []string, name string, defaultVal string) (string, []string) {
	for i, arg := range args {
		if arg == name || arg == "-"+name {
			if i+1 < len(args) {
				val := args[i+1]
				remaining := make([]string, 0, len(args)-2)
				remaining = append(remaining, args[:i]...)
				remaining = append(remaining, args[i+2:]...)
				return val, remaining
			}
		}
		if strings.HasPrefix(arg, name+"=") || strings.HasPrefix(arg, "-"+name+"=") {
			parts := strings.SplitN(arg, "=", 2)
			if len(parts) == 2 {
				remaining := make([]string, 0, len(args)-1)
				remaining = append(remaining, args[:i]...)
				remaining = append(remaining, args[i+1:]...)
				return parts[1], remaining
			}
		}
	}
	return defaultVal, args
}

// ParseBoolArg 从参数列表中解析一个 bool 类型的命名参数。
func ParseBoolArg(args []string, name string) (bool, []string) {
	for i, arg := range args {
		if arg == name || arg == "-"+name || arg == "--"+name {
			remaining := make([]string, 0, len(args)-1)
			remaining = append(remaining, args[:i]...)
			remaining = append(remaining, args[i+1:]...)
			return true, remaining
		}
	}
	return false, args
}

// ParseArgsToMap 将命令行参数解析为 map（自动映射短参到长参）。
func ParseArgsToMap(args []string) map[string]any {
	shortToLong := map[string]string{
		"f": "file", "t": "timeout", "d": "dir", "p": "pattern",
		"m": "mode", "u": "user", "n": "name",
	}
	m := make(map[string]any)
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "--") {
			key := strings.TrimPrefix(arg, "--")
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
				m[key] = args[i+1]
				i++
			} else {
				m[key] = true
			}
		} else if strings.HasPrefix(arg, "-") {
			key := strings.TrimPrefix(arg, "-")
			if longName, ok := shortToLong[key]; ok {
				key = longName
			}
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				m[key] = args[i+1]
				i++
			} else {
				m[key] = true
			}
		}
	}
	return m
}

// MapToArgs 将 map 转换回 []string（与 ParseArgsToMap 互逆）。
// 用于将 HTTP JSON 参数转成 CLI 风格的 []string。
func MapToArgs(m map[string]any) []string {
	var args []string
	for k, v := range m {
		args = append(args, "--"+k)
		if s, ok := v.(string); ok && s != "" {
			args = append(args, s)
		}
	}
	return args
}
