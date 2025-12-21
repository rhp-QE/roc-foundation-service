package api

import (
	"context"

	"github.com/cloudwego/kitex/pkg/klog"
	backbon "github.com/rhp-QE/roc-foundation-service/long_connection_service/kitex_gen/backbon"
	"github.com/rhp-QE/roc-foundation-service/long_connection_service/src/backbon/service"
	servicecontext "github.com/rhp-QE/roc-foundation-service/long_connection_service/src/servicecontext"
)

// BackbonServiceAPI BackbonService API 层接口
type BackbonServiceAPI interface {
	// CheckUserOnline 检查用户在线状态
	CheckUserOnline(ctx context.Context, req *backbon.CheckUserOnlineReq) (resp *backbon.CheckUserOnlineResp, err error)
	// PushData 推送数据到用户
	PushData(ctx context.Context, req *backbon.PushDataReq) (resp *backbon.PushDataResp, err error)
	// PushToConnection 推送到指定连接
	PushToConnection(ctx context.Context, req *backbon.PushToConnectionReq) (resp *backbon.PushToConnectionResp, err error)
	// RegisterService 注册服务
	RegisterService(ctx context.Context, req *backbon.RegisterServiceReq) (resp *backbon.RegisterServiceResp, err error)
	// UnregisterService 注销服务
	UnregisterService(ctx context.Context, req *backbon.UnregisterServiceReq) (resp *backbon.UnregisterServiceResp, err error)
	// GetService 获取服务信息
	GetService(ctx context.Context, req *backbon.GetServiceReq) (resp *backbon.GetServiceResp, err error)
}

type backbonServiceAPIImpl struct {
	service service.BackbonService
}

// NewBackbonServiceAPI 创建 BackbonServiceAPI 实例
func NewBackbonServiceAPI(service service.BackbonService) BackbonServiceAPI {
	return &backbonServiceAPIImpl{
		service: service,
	}
}

// CheckUserOnline 检查用户在线状态
func (a *backbonServiceAPIImpl) CheckUserOnline(ctx context.Context, req *backbon.CheckUserOnlineReq) (resp *backbon.CheckUserOnlineResp, err error) {
	return a.service.CheckUserOnline(ctx, req)
}

// PushData 推送数据到用户
func (a *backbonServiceAPIImpl) PushData(ctx context.Context, req *backbon.PushDataReq) (resp *backbon.PushDataResp, err error) {
	return a.service.PushData(ctx, req)
}

// PushToConnection 推送到指定连接
func (a *backbonServiceAPIImpl) PushToConnection(ctx context.Context, req *backbon.PushToConnectionReq) (resp *backbon.PushToConnectionResp, err error) {
	return a.service.PushToConnection(ctx, req)
}

// RegisterService 注册服务
func (a *backbonServiceAPIImpl) RegisterService(ctx context.Context, req *backbon.RegisterServiceReq) (resp *backbon.RegisterServiceResp, err error) {
	return a.service.RegisterService(ctx, req)
}

// UnregisterService 注销服务
func (a *backbonServiceAPIImpl) UnregisterService(ctx context.Context, req *backbon.UnregisterServiceReq) (resp *backbon.UnregisterServiceResp, err error) {
	return a.service.UnregisterService(ctx, req)
}

// GetService 获取服务信息
func (a *backbonServiceAPIImpl) GetService(ctx context.Context, req *backbon.GetServiceReq) (resp *backbon.GetServiceResp, err error) {
	return a.service.GetService(ctx, req)
}

// BackbonServiceImpl 实现 kitex 生成的 BackbonService 接口
type BackbonServiceImpl struct {
	api BackbonServiceAPI
}

// NewBackbonServiceImpl 创建 BackbonServiceImpl 实例（用于 kitex server）
func NewBackbonServiceImpl(hub interface{}, serviceCtx *servicecontext.ServiceContext) *BackbonServiceImpl {
	// 组装各层：service → api
	backbonService := service.NewBackbonService(hub, serviceCtx)
	backbonAPI := NewBackbonServiceAPI(backbonService)

	return &BackbonServiceImpl{
		api: backbonAPI,
	}
}

// CheckUserOnline 检查用户在线状态（RPC 映射层）
func (s *BackbonServiceImpl) CheckUserOnline(ctx context.Context, req *backbon.CheckUserOnlineReq) (resp *backbon.CheckUserOnlineResp, err error) {
	// 参数校验
	if req == nil {
		klog.CtxErrorf(ctx, "[BackbonServiceAPI] CheckUserOnline request is nil")
		return nil, err
	}

	// 调用 service 层处理业务逻辑
	return s.api.CheckUserOnline(ctx, req)
}

// PushData 推送数据到用户（RPC 映射层）
func (s *BackbonServiceImpl) PushData(ctx context.Context, req *backbon.PushDataReq) (resp *backbon.PushDataResp, err error) {
	// 参数校验
	if req == nil {
		klog.CtxErrorf(ctx, "[BackbonServiceAPI] PushData request is nil")
		return nil, err
	}

	// 调用 service 层处理业务逻辑
	return s.api.PushData(ctx, req)
}

// PushToConnection 推送到指定连接（RPC 映射层）
func (s *BackbonServiceImpl) PushToConnection(ctx context.Context, req *backbon.PushToConnectionReq) (resp *backbon.PushToConnectionResp, err error) {
	// 参数校验
	if req == nil {
		klog.CtxErrorf(ctx, "[BackbonServiceAPI] PushToConnection request is nil")
		return nil, err
	}

	// 调用 service 层处理业务逻辑
	return s.api.PushToConnection(ctx, req)
}

// RegisterService 注册服务（RPC 映射层）
func (s *BackbonServiceImpl) RegisterService(ctx context.Context, req *backbon.RegisterServiceReq) (resp *backbon.RegisterServiceResp, err error) {
	// 参数校验
	if req == nil {
		klog.CtxErrorf(ctx, "[BackbonServiceAPI] RegisterService request is nil")
		return nil, err
	}

	// 调用 service 层处理业务逻辑
	return s.api.RegisterService(ctx, req)
}

// UnregisterService 注销服务（RPC 映射层）
func (s *BackbonServiceImpl) UnregisterService(ctx context.Context, req *backbon.UnregisterServiceReq) (resp *backbon.UnregisterServiceResp, err error) {
	// 参数校验
	if req == nil {
		klog.CtxErrorf(ctx, "[BackbonServiceAPI] UnregisterService request is nil")
		return nil, err
	}

	// 调用 service 层处理业务逻辑
	return s.api.UnregisterService(ctx, req)
}

// GetService 获取服务信息（RPC 映射层）
func (s *BackbonServiceImpl) GetService(ctx context.Context, req *backbon.GetServiceReq) (resp *backbon.GetServiceResp, err error) {
	// 参数校验
	if req == nil {
		klog.CtxErrorf(ctx, "[BackbonServiceAPI] GetService request is nil")
		return nil, err
	}

	// 调用 service 层处理业务逻辑
	return s.api.GetService(ctx, req)
}
