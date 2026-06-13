package storage

import (
	"context"

	"github.com/rhp-QE/roc-foundation-service/long_connection_service/src/servicecontext"
	"github.com/rhp-QE/roc-foundation-util-go/cache"
)

// BackbonStorage Backbon 存储层接口
type BackbonStorage interface {
	// GetUserConnectionMappings 获取用户的所有连接映射
	GetUserConnectionMappings(ctx context.Context, userID string) (map[string]string, error)
	// GetConnectionMachineAddr 获取连接所在机器地址
	GetConnectionMachineAddr(ctx context.Context, connectionID string) (string, error)
	// RemoveConnectionMapping 删除用户连接映射及连接地址
	RemoveConnectionMapping(ctx context.Context, userID string, connectionID string) error
	// RegisterService 注册服务
	RegisterService(ctx context.Context, serviceName string, methods []string) error
	// UnregisterService 注销服务
	UnregisterService(ctx context.Context, serviceName string, methods []string) error
	// GetService 获取服务信息
	GetService(ctx context.Context, serviceName string, method string) (bool, error)
}

type backbonStorageImpl struct {
	serviceCtx *servicecontext.ServiceContext
}

// NewBackbonStorage 创建 BackbonStorage 实例
func NewBackbonStorage(serviceCtx *servicecontext.ServiceContext) BackbonStorage {
	return &backbonStorageImpl{
		serviceCtx: serviceCtx,
	}
}

// getRedis 获取 Redis 客户端
func (s *backbonStorageImpl) getRedis() cache.Cache {
	return s.serviceCtx.GetRedis()
}
