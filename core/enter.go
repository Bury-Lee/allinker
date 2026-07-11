// Package core 存放全局变量和共享实例。
package core

import (
	"allinker/storage"
	"gorm.io/gorm"
)

// Global 是整个应用程序使用的单例 JSON Store 实例。
var Global *storage.Store

// DB 是全局 GORM 数据库实例（消息 + 锁共用）。
var DB *gorm.DB
