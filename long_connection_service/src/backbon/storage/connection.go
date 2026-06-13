package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/rhp-QE/roc-foundation-service/long_connection_service/src/util"
	"github.com/rhp-QE/roc-foundation-util-go/cache"
	"github.com/rhp-QE/roc-foundation-util-go/stringutil"
)

var ErrNoLiveConnectionMappings = errors.New("no live connection mappings")

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
		return nil, fmt.Errorf("%w for user %s", ErrNoLiveConnectionMappings, userID)
	}

	// 过滤空值，并清理 user hash 中已经失效的 connectionID。
	// connection key 是连接是否仍然存活的可信来源，user hash 只是按用户索引连接。
	result := make(map[string]string, len(mappings))
	staleConnectionIDs := make([]string, 0)
	for connectionID, machineAddr := range mappings {
		if stringutil.IsEmpty(connectionID) || stringutil.IsEmpty(machineAddr) {
			continue
		}

		connectionKey := util.GetConnectionKeyInCache(connectionID)
		liveMachineAddr, err := redisCache.Get(ctx, connectionKey)
		if err != nil {
			if errors.Is(err, cache.ErrKeyNotFound) {
				staleConnectionIDs = append(staleConnectionIDs, connectionID)
				continue
			}
			return nil, fmt.Errorf("failed to verify connection %s from Redis: %w", connectionID, err)
		}

		if stringutil.IsEmpty(liveMachineAddr) {
			staleConnectionIDs = append(staleConnectionIDs, connectionID)
			continue
		}

		result[connectionID] = liveMachineAddr
	}

	if len(staleConnectionIDs) > 0 {
		if _, err := redisCache.HDel(ctx, userKey, staleConnectionIDs...); err != nil {
			klog.CtxWarnf(ctx, "Failed to remove stale connection mappings for user %s: %v", userID, err)
		} else {
			klog.CtxInfof(ctx, "Removed %d stale connection mappings for user %s", len(staleConnectionIDs), userID)
		}
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("%w for user %s", ErrNoLiveConnectionMappings, userID)
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

func (s *backbonStorageImpl) RemoveConnectionMapping(ctx context.Context, userID string, connectionID string) error {
	redisCache := s.getRedis()
	if redisCache == nil {
		return fmt.Errorf("Redis is not available")
	}

	if stringutil.IsEmpty(userID) || stringutil.IsEmpty(connectionID) {
		return nil
	}

	userKey := util.GetUserConnectionKeyInCache(userID)
	if _, err := redisCache.HDel(ctx, userKey, connectionID); err != nil {
		return fmt.Errorf("failed to remove connection %s from user %s mappings: %w", connectionID, userID, err)
	}

	connectionKey := util.GetConnectionKeyInCache(connectionID)
	if err := redisCache.Delete(ctx, connectionKey); err != nil {
		return fmt.Errorf("failed to remove connection key %s: %w", connectionID, err)
	}

	return nil
}
