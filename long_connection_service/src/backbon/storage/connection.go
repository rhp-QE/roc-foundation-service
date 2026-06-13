package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/rhp-QE/roc-foundation-service/long_connection_service/src/route"
)

var ErrNoLiveConnectionMappings = route.ErrNoLiveConnections

// GetUserConnectionMappings 获取用户的所有连接映射。
// BackbonStorage 不再定义 route 语义，连接路由统一由 RouteStore 决定。
func (s *backbonStorageImpl) GetUserConnectionMappings(ctx context.Context, userID string) (map[string]string, error) {
	metas, err := s.routeStore().ListByUser(ctx, userID)
	if err != nil {
		if errors.Is(err, route.ErrNoLiveConnections) {
			return nil, fmt.Errorf("%w for user %s", ErrNoLiveConnectionMappings, userID)
		}
		return nil, err
	}

	result := make(map[string]string, len(metas))
	for _, meta := range metas {
		if meta.ConnectionID == "" || meta.GatewayAddr == "" {
			continue
		}
		result[meta.ConnectionID] = meta.GatewayAddr
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("%w for user %s", ErrNoLiveConnectionMappings, userID)
	}
	return result, nil
}

// GetConnectionMachineAddr 获取连接所在机器地址。
func (s *backbonStorageImpl) GetConnectionMachineAddr(ctx context.Context, connectionID string) (string, error) {
	meta, err := s.routeStore().Get(ctx, connectionID)
	if err != nil {
		return "", fmt.Errorf("connection %s not found in Redis: %w", connectionID, err)
	}
	if meta.GatewayAddr == "" {
		return "", fmt.Errorf("connection %s not found", connectionID)
	}
	return meta.GatewayAddr, nil
}

func (s *backbonStorageImpl) RemoveConnectionMapping(ctx context.Context, userID string, connectionID string) error {
	return s.routeStore().Cleanup(ctx, userID, connectionID)
}

func (s *backbonStorageImpl) routeStore() route.Store {
	return route.NewRedisStore(s.getRedis(), s.serviceCtx.GetLocalAddress(), route.DefaultTTL)
}
