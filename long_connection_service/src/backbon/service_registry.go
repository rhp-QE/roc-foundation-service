// Package backbon 实现 Backbon RPC 服务
//
// Author: Ruan Huipeng
// Date: 2025-12-01

package backbon

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/rhp-QE/roc-foundation-service/long_connection_service/kitex_gen"
	"github.com/roc/roc-foundation-util-go/stringutil"
)

// RegisterService implements the BackbonServiceImpl interface.
// 向 redis 内写入 service 和 method 的映射关系
// 使用 Set 结构: key = "fronter:service:{serviceName}", members = {method1, method2, ...}
func (s *BackbonServiceImpl) RegisterService(ctx context.Context, req *kitex_gen.RegisterServiceReq) (resp *kitex_gen.RegisterServiceResp, err error) {
	resp = &kitex_gen.RegisterServiceResp{
		Success: false,
	}

	// 获取 Redis 客户端
	redisCache := s.serviceCtx.GetRedis()
	if redisCache == nil {
		resp.Error = "Redis is not available"
		return resp, fmt.Errorf("redis is not available")
	}

	// 检查服务名称
	serviceName := req.GetService()
	if stringutil.IsEmpty(serviceName) {
		resp.Error = "service name cannot be empty"
		return resp, fmt.Errorf("service name cannot be empty")
	}

	// 构建 Redis key: fronter:service:{serviceName}
	key := s.getServiceKey(serviceName)
	methods := req.GetMethods()

	// 如果没有指定 methods，则使用特殊标记 "*" 表示所有方法都支持
	if len(methods) == 0 {
		methods = []string{"*"}
	}

	// 过滤空字符串的方法
	validMethods := stringutil.FilterEmpty(methods)

	if len(validMethods) == 0 {
		resp.Error = "no valid methods to register"
		return resp, fmt.Errorf("no valid methods to register")
	}

	// 将 methods 转换为 []interface{} 用于 SAdd
	members := make([]interface{}, len(validMethods))
	for i, method := range validMethods {
		members[i] = method
	}

	// 使用 SAdd 向 Set 添加方法
	_, err = redisCache.SAdd(ctx, key, members...)
	if err != nil {
		resp.Error = fmt.Sprintf("failed to store service methods in Redis: %v", err)
		return resp, err
	}

	// 设置过期时间（7天），可以通过心跳续期
	err = redisCache.Expire(ctx, key, 7*24*time.Hour)
	if err != nil {
		log.Printf("Warning: failed to set expire for key %s: %v", key, err)
	}

	resp.Success = true
	log.Printf("Successfully registered service: %s with methods: %v", serviceName, validMethods)
	return resp, nil
}

// UnregisterService implements the BackbonServiceImpl interface.
// 销毁 redis 内 service 和 method 的映射关系
func (s *BackbonServiceImpl) UnregisterService(ctx context.Context, req *kitex_gen.UnregisterServiceReq) (resp *kitex_gen.UnregisterServiceResp, err error) {
	resp = &kitex_gen.UnregisterServiceResp{
		Success: false,
	}

	// 获取 Redis 客户端
	redisCache := s.serviceCtx.GetRedis()
	if redisCache == nil {
		resp.Error = "Redis is not available"
		return resp, fmt.Errorf("redis is not available")
	}

	// 检查服务名称
	serviceName := req.GetService()
	if stringutil.IsEmpty(serviceName) {
		resp.Error = "service name cannot be empty"
		return resp, fmt.Errorf("service name cannot be empty")
	}

	// 构建 Redis key: fronter:service:{serviceName}
	key := s.getServiceKey(serviceName)
	methods := req.GetMethods()

	// 如果没有指定 methods，则删除整个服务（删除整个 key）
	if len(methods) == 0 {
		err = redisCache.Delete(ctx, key)
		if err != nil {
			resp.Error = fmt.Sprintf("failed to delete service from Redis: %v", err)
			return resp, err
		}
		resp.Success = true
		log.Printf("Successfully unregistered service: %s (all methods)", serviceName)
		return resp, nil
	}

	// 过滤空字符串的 methods
	validMethods := stringutil.FilterEmpty(methods)

	if len(validMethods) == 0 {
		resp.Error = "no valid methods to unregister"
		return resp, fmt.Errorf("no valid methods to unregister")
	}

	// 将 methods 转换为 []interface{} 用于 SRem
	members := make([]interface{}, len(validMethods))
	for i, method := range validMethods {
		members[i] = method
	}

	// 使用 SRem 从 Set 中移除方法
	_, err = redisCache.SRem(ctx, key, members...)
	if err != nil {
		resp.Error = fmt.Sprintf("failed to delete methods from Redis: %v", err)
		return resp, err
	}

	resp.Success = true
	log.Printf("Successfully unregistered service: %s with methods: %v", serviceName, validMethods)
	return resp, nil
}

// GetService implements the BackbonServiceImpl interface.
func (s *BackbonServiceImpl) GetService(ctx context.Context, req *kitex_gen.GetServiceReq) (resp *kitex_gen.GetServiceResp, err error) {
	// TODO: Your code here...
	return
}
