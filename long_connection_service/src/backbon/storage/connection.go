package storage

import (
	"context"
	"fmt"

	"github.com/rhp-QE/roc-foundation-service/long_connection_service/src/util"
	"github.com/rhp-QE/roc-foundation-util-go/stringutil"
)

// GetUserConnectionMappings 获取用户的所有连接映射
// Redis 存储结构：key = "fronter:user:connection:{userID}" (Hash)
//   - field: connectionID
//   - value: 机器地址 (ip:port)
func (s *backbonStorageImpl) GetUserConnectionMappings(ctx context.Context, userID string) (map[string]string, error) {
	redisCache := s.getRedis()
	if redisCache == nil {
		return nil, fmt.Errorf("Redis is not available")
	}

	userKey := util.GetUserConnectionKeyInCache(userID)

	// 使用 HGetAll 获取 Hash 的所有字段和值
	mappings, err := redisCache.HGetAll(ctx, userKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection mappings from Redis: %w", err)
	}

	if len(mappings) == 0 {
		return nil, fmt.Errorf("no connection mappings found for user %s", userID)
	}

	// 过滤空值
	result := make(map[string]string, len(mappings))
	for connectionID, machineAddr := range mappings {
		if !stringutil.IsEmpty(connectionID) && !stringutil.IsEmpty(machineAddr) {
			result[connectionID] = machineAddr
		}
	}

	return result, nil
}

// GetConnectionMachineAddr 获取连接所在机器地址
func (s *backbonStorageImpl) GetConnectionMachineAddr(ctx context.Context, connectionID string) (string, error) {
	redisCache := s.getRedis()
	if redisCache == nil {
		return "", fmt.Errorf("Redis is not available")
	}

	connectionKey := util.GetConnectionKeyInCache(connectionID)
	machineAddr, err := redisCache.Get(ctx, connectionKey)
	if err != nil {
		return "", fmt.Errorf("connection %s not found in Redis: %w", connectionID, err)
	}

	if stringutil.IsEmpty(machineAddr) {
		return "", fmt.Errorf("connection %s not found", connectionID)
	}

	return machineAddr, nil
}
