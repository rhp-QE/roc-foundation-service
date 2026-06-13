package service

import (
	"context"

	backbon "github.com/rhp-QE/roc-foundation-service/long_connection_service/kitex_gen/backbon"
	"github.com/rhp-QE/roc-foundation-service/long_connection_service/src/frontier"
	"github.com/rhp-QE/roc-foundation-service/long_connection_service/src/push"
	"github.com/rhp-QE/roc-foundation-service/long_connection_service/src/route"
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

func (s *backbonServiceImpl) newPushRouter() *push.Router {
	// Backbon service 只装配依赖，推送决策全部交给 PushRouter。
	return push.NewRouter(s.routeStore(), s.getHub(), s.serviceCtx, s.serviceCtx.GetLocalAddress())
}

func (s *backbonServiceImpl) routeStore() route.Store {
	// 优先复用 gateway Hub 的 RouteStore，保证 register/refresh/push 看到同一套 key 和 TTL。
	if hub := s.getHub(); hub != nil && hub.GetRouteStore() != nil {
		return hub.GetRouteStore()
	}
	return route.NewRedisStore(s.serviceCtx.GetRedis(), s.serviceCtx.GetLocalAddress(), route.DefaultTTL)
}

func (s *backbonServiceImpl) getHub() *frontier.Hub {
	hub, ok := s.hub.(*frontier.Hub)
	if !ok {
		return nil
	}
	return hub
}
