package service

import (
	"context"

	backbon "github.com/rhp-QE/roc-foundation-service/long_connection_service/kitex_gen/backbon"
	servicecontext "github.com/rhp-QE/roc-foundation-service/long_connection_service/src/servicecontext"
)

// BackbonService 定义 BackbonService 的业务接口（逻辑层）
type BackbonService interface {
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

// backbonServiceImpl 是 BackbonService 的具体实现
type backbonServiceImpl struct {
	hub        interface{} // *frontier.Hub
	serviceCtx *servicecontext.ServiceContext
}

// NewBackbonService 创建 BackbonService 实例
func NewBackbonService(hub interface{}, serviceCtx *servicecontext.ServiceContext) BackbonService {
	return &backbonServiceImpl{
		hub:        hub,
		serviceCtx: serviceCtx,
	}
}
