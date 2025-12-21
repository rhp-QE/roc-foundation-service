package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/rhp-QE/roc-foundation-service/long_connection_service/src/util"
	"github.com/rhp-QE/roc-foundation-util-go/stringutil"
)

// RegisterService 注册服务
// 使用 Set 结构: key = "fronter:service:{serviceName}", members = {method1, method2, ...}
func (s *backbonStorageImpl) RegisterService(ctx context.Context, serviceName string, methods []string) error {
	redisCache := s.getRedis()
	if redisCache == nil {
		return fmt.Errorf("Redis is not available")
	}

	// 构建 Redis key: fronter:service:{serviceName}
	key := util.GetServiceKeyInCache(serviceName)

	// 过滤空字符串的方法
	validMethods := stringutil.FilterEmpty(methods)
	if len(validMethods) == 0 {
		return fmt.Errorf("no valid methods to register")
	}

	// 将 methods 转换为 []interface{} 用于 SAdd
	members := make([]interface{}, len(validMethods))
	for i, method := range validMethods {
		members[i] = method
	}

	// 使用 SAdd 向 Set 添加方法
	_, err := redisCache.SAdd(ctx, key, members...)
	if err != nil {
		return fmt.Errorf("failed to store service methods in Redis: %w", err)
	}

	// 设置过期时间（7天），可以通过心跳续期
	err = redisCache.Expire(ctx, key, 7*24*time.Hour)
	if err != nil {
		klog.CtxWarnf(ctx, "Failed to set expire for key %s: %v", key, err)
	}

	return nil
}

// UnregisterService 注销服务
func (s *backbonStorageImpl) UnregisterService(ctx context.Context, serviceName string, methods []string) error {
	redisCache := s.getRedis()
	if redisCache == nil {
		return fmt.Errorf("Redis is not available")
	}

	// 构建 Redis key: fronter:service:{serviceName}
	key := util.GetServiceKeyInCache(serviceName)

	// 如果没有指定 methods，则删除整个服务（删除整个 key）
	if len(methods) == 0 {
		err := redisCache.Delete(ctx, key)
		if err != nil {
			return fmt.Errorf("failed to delete service from Redis: %w", err)
		}
		return nil
	}

	// 过滤空字符串的 methods
	validMethods := stringutil.FilterEmpty(methods)
	if len(validMethods) == 0 {
		return fmt.Errorf("no valid methods to unregister")
	}

	// 将 methods 转换为 []interface{} 用于 SRem
	members := make([]interface{}, len(validMethods))
	for i, method := range validMethods {
		members[i] = method
	}

	// 使用 SRem 从 Set 中移除方法
	_, err := redisCache.SRem(ctx, key, members...)
	if err != nil {
		return fmt.Errorf("failed to delete methods from Redis: %w", err)
	}

	return nil
}

// GetService 获取服务信息
func (s *backbonStorageImpl) GetService(ctx context.Context, serviceName string, method string) (bool, error) {
	redisCache := s.getRedis()
	if redisCache == nil {
		return false, fmt.Errorf("Redis is not available")
	}

	// 构建 Redis key: fronter:service:{serviceName}
	key := util.GetServiceKeyInCache(serviceName)

	// 检查 key 是否存在
	exists, err := redisCache.Exists(ctx, key)
	if err != nil {
		return false, fmt.Errorf("failed to check service existence: %w", err)
	}

	if exists == 0 {
		return false, nil
	}

	// 如果指定了 method，检查 method 是否在 Set 中
	if method != "" {
		isMember, err := redisCache.SIsMember(ctx, key, method)
		if err != nil {
			return false, fmt.Errorf("failed to check method membership: %w", err)
		}
		// 如果 method 不在 Set 中，检查是否有 "*"（表示所有方法）
		if !isMember {
			hasAll, err := redisCache.SIsMember(ctx, key, "*")
			if err != nil {
				return false, fmt.Errorf("failed to check '*' membership: %w", err)
			}
			return hasAll, nil
		}
		return true, nil
	}

	// 没有指定 method，只要服务存在就返回 true
	return true, nil
}

