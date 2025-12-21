package service

import (
	"context"
	"fmt"

	"github.com/cloudwego/kitex/pkg/klog"
	backbon "github.com/rhp-QE/roc-foundation-service/long_connection_service/kitex_gen/backbon"
	"github.com/rhp-QE/roc-foundation-service/long_connection_service/src/backbon/storage"
)

// RegisterService 注册服务
func (s *backbonServiceImpl) RegisterService(ctx context.Context, req *backbon.RegisterServiceReq) (resp *backbon.RegisterServiceResp, err error) {
	resp = &backbon.RegisterServiceResp{
		Success: false,
	}

	// 检查服务名称
	serviceName := req.GetService()
	if serviceName == "" {
		resp.Error = "service name cannot be empty"
		return resp, fmt.Errorf("service name cannot be empty")
	}

	methods := req.GetMethods()
	// 如果没有指定 methods，则使用特殊标记 "*" 表示所有方法都支持
	if len(methods) == 0 {
		methods = []string{"*"}
	}

	// 调用 storage 层注册服务
	storage := storage.NewBackbonStorage(s.serviceCtx)
	if err := storage.RegisterService(ctx, serviceName, methods); err != nil {
		resp.Error = fmt.Sprintf("failed to register service: %v", err)
		return resp, err
	}

	resp.Success = true
	klog.CtxInfof(ctx, "Successfully registered service: %s with methods: %v", serviceName, methods)
	return resp, nil
}

// UnregisterService 注销服务
func (s *backbonServiceImpl) UnregisterService(ctx context.Context, req *backbon.UnregisterServiceReq) (resp *backbon.UnregisterServiceResp, err error) {
	resp = &backbon.UnregisterServiceResp{
		Success: false,
	}

	// 检查服务名称
	serviceName := req.GetService()
	if serviceName == "" {
		resp.Error = "service name cannot be empty"
		return resp, fmt.Errorf("service name cannot be empty")
	}

	methods := req.GetMethods()

	// 调用 storage 层注销服务
	storage := storage.NewBackbonStorage(s.serviceCtx)
	if err := storage.UnregisterService(ctx, serviceName, methods); err != nil {
		resp.Error = fmt.Sprintf("failed to unregister service: %v", err)
		return resp, err
	}

	resp.Success = true
	klog.CtxInfof(ctx, "Successfully unregistered service: %s with methods: %v", serviceName, methods)
	return resp, nil
}

// GetService 获取服务信息
func (s *backbonServiceImpl) GetService(ctx context.Context, req *backbon.GetServiceReq) (resp *backbon.GetServiceResp, err error) {
	// TODO: 实现获取服务信息逻辑
	resp = &backbon.GetServiceResp{
		Registered: false,
		Service:    req.GetService(),
	}

	// 调用 storage 层查询服务
	storage := storage.NewBackbonStorage(s.serviceCtx)
	registered, err := storage.GetService(ctx, req.GetService(), req.GetMethod())
	if err != nil {
		return resp, err
	}

	resp.Registered = registered
	return resp, nil
}
