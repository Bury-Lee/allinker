// Package watch 提供文件监听位（Watch Point）的注册和管理功能。
//
// 所有数据（包括文件快照哈希）均存储在 SQLite 数据库中。
//
// 设计决策：
// - 使用哈希快照对比而非 inotify/fsnotify：跨平台兼容，无需系统级权限
// - 大文件采样哈希（头+中+尾各 2MB）：避免全量读取大文件
// - 快照持久化到数据库：进程重启后无需重新扫描历史
package watch

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"allinker/core"
	"allinker/model"

	"gorm.io/gorm"
)

// =============================================================================
// 共享 Handler — 方案C：CLI 和 Server 共用
// =============================================================================

// WatchAddParams watch add 命令的共享参数结构体。
type WatchAddParams struct {
	Name    string
	Dir     string
	Pattern string
	Creator string
}

// WatchCheckParams watch check 的共享参数。
type WatchCheckParams struct {
	Name string
}

// WatchRemoveParams watch remove 的共享参数。
type WatchRemoveParams struct {
	Name     string
	Username string
}

// HandleWatchAdd 是 watch add 的共享处理函数。
func HandleWatchAdd(p WatchAddParams) (*model.WatchItem, error) {
	return AddWatch(p.Name, p.Dir, p.Pattern, p.Creator)
}

// HandleWatchCheck 是 watch check 的共享处理函数。
func HandleWatchCheck(p WatchCheckParams) ([]string, error) {
	return CheckWatch(p.Name)
}

// HandleWatchRemove 是 watch remove 的共享处理函数。
func HandleWatchRemove(p WatchRemoveParams) error {
	return RemoveWatch(p.Name, p.Username)
}

// InitModels 注册监听相关的 GORM 模型到给定数据库实例。
func InitModels(db *gorm.DB) error {
	return db.AutoMigrate(&model.WatchRecord{})
}

// AddWatch 注册一个新的监听位。
func AddWatch(name, dir, pattern, creator string) (*model.WatchItem, error) {
	if name == "" {
		return nil, fmt.Errorf("监听位名称不能为空")
	}
	if dir == "" {
		return nil, fmt.Errorf("监听目录不能为空")
	}
	if pattern == "" {
		return nil, fmt.Errorf("文件匹配模式不能为空")
	}

	// 检查名称是否已存在
	var existing model.WatchRecord
	if core.DB.Where("name = ?", name).First(&existing).Error == nil {
		return nil, fmt.Errorf("监听位 %q 已存在", name)
	}

	now := time.Now().UTC()
	rec := model.WatchRecord{
		ID:        fmt.Sprintf("watch_%d", now.UnixNano()%100000),
		Name:      name,
		Creator:   creator,
		Dir:       dir,
		Pattern:   pattern,
		CreatedAt: now,
		LastCheck: now,
		Status:    "active",
	}
	if err := core.DB.Create(&rec).Error; err != nil {
		return nil, fmt.Errorf("保存监听位失败: %w", err)
	}

	return watchRecordToItem(&rec), nil
}

// ListWatches 返回所有监听位列表。如果 name 不为空，只返回匹配的监听位。
func ListWatches(name string) ([]*model.WatchItem, error) {
	var records []model.WatchRecord
	query := core.DB.Model(&model.WatchRecord{})
	if name != "" {
		query = query.Where("name = ?", name)
	}
	query.Find(&records)

	items := make([]*model.WatchItem, 0, len(records))
	for i := range records {
		items = append(items, watchRecordToItem(&records[i]))
	}
	return items, nil
}

// CheckWatch 检查监听位的文件变化，返回有变化的文件列表（新增或修改）。
// 快照哈希存储在数据库的 SnapshotData 字段中。
func CheckWatch(name string) ([]string, error) {
	// 从数据库读取监听位
	var rec model.WatchRecord
	if err := core.DB.Where("name = ?", name).First(&rec).Error; err != nil {
		return nil, fmt.Errorf("监听位 %q 不存在", name)
	}

	// 读取当前匹配的文件列表
	currentFiles, err := filepath.Glob(filepath.Join(rec.Dir, rec.Pattern))
	if err != nil {
		return nil, fmt.Errorf("扫描文件失败: %w", err)
	}

	// 从数据库读取上次快照
	prevHashes := make(map[string]string)
	if rec.SnapshotData != "" {
		json.Unmarshal([]byte(rec.SnapshotData), &prevHashes)
	}

	// 对比当前哈希与快照
	var changedFiles []string
	newHashes := make(map[string]string)
	for _, f := range currentFiles {
		hash := quickHash(f)
		newHashes[f] = hash
		oldHash, exists := prevHashes[f]
		if !exists || oldHash != hash {
			changedFiles = append(changedFiles, f)
		}
	}

	// 序列化新快照为 JSON 并更新数据库
	snapshotJSON, _ := json.Marshal(newHashes)

	now := time.Now().UTC()
	updates := map[string]interface{}{
		"snapshot_data": string(snapshotJSON),
		"last_check":    now,
	}
	if len(changedFiles) > 0 {
		updates["last_change"] = now
	}
	core.DB.Model(&rec).Updates(updates)

	return changedFiles, nil
}

// RemoveWatch 取消指定监听位。
func RemoveWatch(name, username string) error {
	var rec model.WatchRecord
	if err := core.DB.Where("name = ?", name).First(&rec).Error; err != nil {
		return fmt.Errorf("监听位 %q 不存在", name)
	}

	if rec.Creator != username {
		return fmt.Errorf("只能取消自己创建的监听位（创建者: %s）", rec.Creator)
	}

	return core.DB.Delete(&rec).Error
}

// ClearWatches 清空指定用户创建的所有监听位。
func ClearWatches(username string) error {
	return core.DB.Where("creator = ?", username).Delete(&model.WatchRecord{}).Error
}

// CountMatchingFiles 返回指定目录下匹配模式的文件数量。
func CountMatchingFiles(dir, pattern string) int {
	matches, err := filepath.Glob(filepath.Join(dir, pattern))
	if err != nil {
		return 0
	}
	return len(matches)
}

// watchRecordToItem 将数据库记录转换为模型中的 WatchItem。
func watchRecordToItem(r *model.WatchRecord) *model.WatchItem {
	created := r.CreatedAt.Format(time.RFC3339)
	lastCheck := r.LastCheck.Format(time.RFC3339)
	lastChange := ""
	if r.LastChange != nil {
		lastChange = r.LastChange.Format(time.RFC3339)
	}
	return &model.WatchItem{
		ID:         r.ID,
		Name:       r.Name,
		Creator:    r.Creator,
		Dir:        r.Dir,
		Pattern:    r.Pattern,
		Created:    created,
		LastCheck:  lastCheck,
		LastChange: lastChange,
		Status:     model.WatchStatus(r.Status),
	}
}

// quickHash 对文件做快速哈希（小文件完整读入，大文件采样头+中+尾）。
func quickHash(path string) string {
	const sampleSize = 2 * 1024 * 1024 // 2MB
	info, err := os.Stat(path)
	if err != nil {
		return ""
	}
	if info.Size() <= sampleSize*3 {
		data, err := os.ReadFile(path)
		if err != nil {
			return ""
		}
		return fmt.Sprintf("%x", data)
	}
	// 大文件采样：头 + 中 + 尾
	data := make([]byte, sampleSize*3)
	f, _ := os.Open(path)
	if f != nil {
		defer f.Close()
		f.Read(data[:sampleSize])
		f.Seek(info.Size()/2-sampleSize/2, 0)
		f.Read(data[sampleSize : sampleSize*2])
		f.Seek(info.Size()-sampleSize, 0)
		f.Read(data[sampleSize*2:])
	}
	return fmt.Sprintf("%x", data)
}
