// Package storage 提供原子性的 JSON 文件读写操作。
//
// 所有写入操作都采用「先写临时文件，再重命名」的策略，以防止数据损坏。
//
// 设计决策：
// - 临时文件 + os.Rename：在 Windows/Linux/macOS 上都是原子操作
// - 读写锁 sync.RWMutex：允许并发读，写操作互斥
// - JSON 而非二进制：数据可人工编辑调试，对低频变更场景性能足够
package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Store 封装一个数据目录，并提供线程安全的 JSON 文件操作。
// root 为 .alf 目录的绝对路径。
// 所有路径方法（UsersPath/ConfigPath/CounterPath）都基于 root 拼接。
type Store struct {
	mu   sync.RWMutex
	root string
}

// NewStore 创建一个以指定目录为根目录的 Store。
func NewStore(root string) *Store {
	return &Store{root: root}
}

// Root 返回数据目录路径。
func (s *Store) Root() string {
	return s.root
}

// UsersPath 返回 users.json 的路径。
func (s *Store) UsersPath() string {
	return filepath.Join(s.root, "users.json")
}

// ConfigPath 返回 config.json 的路径。
func (s *Store) ConfigPath() string {
	return filepath.Join(s.root, "config.json")
}

// CounterPath 返回 counter.json 的路径。
func (s *Store) CounterPath() string {
	return filepath.Join(s.root, "counter.json")
}

// ServerPIDPath 返回 server.pid 的路径。
func (s *Store) ServerPIDPath() string {
	return filepath.Join(s.root, "server.pid")
}

// ReadJSON 读取 JSON 文件并将其反序列化到 out 中。
// 使用 RLock 支持并发读（多个 Server handler 同时读取配置）。
// 注意：out 必须是指针类型，否则 json.Unmarshal 会静默失败（返回 nil error 但 out 未被填充）。
func (s *Store) ReadJSON(path string, out any) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}

// WriteJSON 将数据以 JSON 格式原子性地写入指定路径。
// 原子写入策略：先写 .tmp 临时文件 → os.Rename 替换目标文件。
// 为什么用 tmp+rename 而非直接 WriteFile：
//   - 直接写入中途崩溃会留下半截文件 → 下次 ReadJSON 解析失败
//   - os.Rename 在 Win/Linux/macOS 上都是原子操作（目标路径被原子替换）
//   - .tmp 后缀不会被其他逻辑误读，崩溃残留一眼可识别
func (s *Store) WriteJSON(path string, data any) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	content, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("json marshal failed: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("mkdir %s failed: %w", dir, err)
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, content, 0644); err != nil {
		return fmt.Errorf("write tmp %s failed: %w", tmpPath, err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename %s -> %s failed: %w", tmpPath, path, err)
	}

	return nil
}
// FileExists 检查文件是否存在。
func (s *Store) FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// EnsureDir 创建目录（如果目录不存在）。
func (s *Store) EnsureDir(path string) error {
	return os.MkdirAll(path, 0755)
}
