// Package util 工具包，提供通用工具函数
//
// Author: Ruan Huipeng
// Date: 2025-12-01

package util

import (
	"github.com/rhp-QE/roc-foundation-util-go/stringutil"
)

// GetServiceKeyInCache 获取服务在缓存中的 key
func GetServiceKeyInCache(serviceName string) string {
	return stringutil.FormatKey("fronter", "service", serviceName)
}

// GetUserConnectionKeyInCache 获取用户连接在缓存中的 key
func GetUserConnectionKeyInCache(userID string) string {
	return stringutil.FormatKey("fronter", "user", "connection", userID)
}

// GetConnectionKeyInCache 获取连接在缓存中的 key
func GetConnectionKeyInCache(connectionID string) string {
	return stringutil.FormatKey("fronter", "connection", connectionID)
}

