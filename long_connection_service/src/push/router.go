package push

import (
	"context"
	"errors"
	"fmt"

	"github.com/cloudwego/kitex/pkg/klog"
	backbon "github.com/rhp-QE/roc-foundation-service/long_connection_service/kitex_gen/backbon"
	"github.com/rhp-QE/roc-foundation-service/long_connection_service/kitex_gen/backbon/backbonservice"
	"github.com/rhp-QE/roc-foundation-service/long_connection_service/src/frontier"
	"github.com/rhp-QE/roc-foundation-service/long_connection_service/src/route"
)

type LocalHub interface {
	Broadcast(msg *frontier.Message)
	GetConnection(connID string) (*frontier.Connection, bool)
	GetUserConnections(userID string) []*frontier.Connection
	SendToConnection(connID string, msg *frontier.Message) error
}

type RemoteClientProvider interface {
	GetRemoteClient(machineAddr string) (backbonservice.Client, error)
}

type Router struct {
	store     route.Store
	hub       LocalHub
	remotes   RemoteClientProvider
	localAddr string
}

// Router 是推送路由的唯一决策点：
// RouteStore 提供候选连接，Hub 负责本机事实确认，远端 gateway 负责异地事实确认。
func NewRouter(store route.Store, hub LocalHub, remotes RemoteClientProvider, localAddr string) *Router {
	return &Router{
		store:     store,
		hub:       hub,
		remotes:   remotes,
		localAddr: localAddr,
	}
}

func (r *Router) Broadcast(msg *frontier.Message) *backbon.PushResult {
	if r.hub == nil {
		return result("", false, backbon.PushStatus_PUSH_STATUS_FAILED, 0, "hub is not available")
	}
	r.hub.Broadcast(msg)
	return result("", true, backbon.PushStatus_PUSH_STATUS_SUCCESS, 0, "")
}

func (r *Router) RouteUser(ctx context.Context, userID string, pushMsg *backbon.PushMessage, frontierMsg *frontier.Message) *backbon.PushResult {
	if userID == "" {
		return result(userID, false, backbon.PushStatus_PUSH_STATUS_FAILED, 0, "userID is empty")
	}
	if r.store == nil {
		klog.CtxErrorf(ctx, "Push route failed userID=%s error=route store is not available", userID)
		return result(userID, false, backbon.PushStatus_PUSH_STATUS_FAILED, 0, "route store is not available")
	}

	// 没有有效 route 是业务 noop，不是长连接系统错误。
	metas, err := r.store.ListByUser(ctx, userID)
	if err != nil {
		if errors.Is(err, route.ErrNoLiveConnections) {
			klog.CtxInfof(ctx, "Push route noop userID=%s status=%s", userID, backbon.PushStatus_PUSH_STATUS_NOOP_OFFLINE.String())
			return result(userID, true, backbon.PushStatus_PUSH_STATUS_NOOP_OFFLINE, 0, "")
		}
		klog.CtxErrorf(ctx, "Push route failed userID=%s error=%v", userID, err)
		return result(userID, false, backbon.PushStatus_PUSH_STATUS_FAILED, 0, err.Error())
	}

	summary := routeSummary{}
	for _, meta := range metas {
		r.routeConnectionMeta(ctx, meta, pushMsg, frontierMsg, &summary)
	}
	pushResult := summary.toPushResult(userID)
	klog.CtxInfof(ctx, "Push route completed userID=%s status=%s success=%v connectionCount=%d stale=%d failed=%d",
		userID, pushResult.GetStatus().String(), pushResult.GetSuccess(), pushResult.GetConnectionCount(), summary.staleCount, summary.failCount)
	return pushResult
}

func (r *Router) RouteConnection(ctx context.Context, connectionID string, pushMsg *backbon.PushMessage, frontierMsg *frontier.Message) (*backbon.PushToConnectionResp, error) {
	if connectionID == "" {
		return pushToConnectionResp(false, backbon.PushStatus_PUSH_STATUS_FAILED, "connectionID is required"), fmt.Errorf("connectionID is required")
	}
	// 指定 connectionID 推送先查本机 Hub，Hub 是 WebSocket 是否真实存在的事实源。
	if r.hub != nil {
		if _, ok := r.hub.GetConnection(connectionID); ok {
			return r.pushLocalConnection(connectionID, frontierMsg)
		}
	}
	if r.store == nil {
		return pushToConnectionResp(false, backbon.PushStatus_PUSH_STATUS_STALE_CLEANED, "connection not found"), fmt.Errorf("connection %s not found", connectionID)
	}

	meta, err := r.store.Get(ctx, connectionID)
	if err != nil {
		return pushToConnectionResp(false, backbon.PushStatus_PUSH_STATUS_STALE_CLEANED, "connection not found"), fmt.Errorf("connection %s not found: %w", connectionID, err)
	}
	// Redis 指向本机但 Hub 不存在，说明 route 已脏，只清理该 connectionID。
	if r.isLocal(meta.GatewayAddr) {
		_ = r.store.Cleanup(ctx, meta.UserID, connectionID)
		klog.CtxInfof(ctx, "Push connection stale connectionID=%s userID=%s gatewayAddr=%s", connectionID, meta.UserID, meta.GatewayAddr)
		return pushToConnectionResp(false, backbon.PushStatus_PUSH_STATUS_STALE_CLEANED, "local connection is stale"), nil
	}
	return r.pushRemoteConnection(ctx, meta, pushMsg)
}

func (r *Router) routeConnectionMeta(ctx context.Context, meta route.ConnectionMeta, pushMsg *backbon.PushMessage, frontierMsg *frontier.Message, summary *routeSummary) {
	if meta.ConnectionID == "" {
		return
	}
	// address 只决定本地/远端路径；最终成功必须由目标 Hub 确认。
	if r.isLocal(meta.GatewayAddr) {
		r.routeLocalConnection(ctx, meta, frontierMsg, summary)
		return
	}
	r.routeRemoteConnection(ctx, meta, pushMsg, summary)
}

func (r *Router) routeLocalConnection(ctx context.Context, meta route.ConnectionMeta, frontierMsg *frontier.Message, summary *routeSummary) {
	if r.hub == nil {
		summary.fail("hub is not available")
		return
	}
	if _, ok := r.hub.GetConnection(meta.ConnectionID); !ok {
		summary.stale()
		r.cleanupStale(ctx, meta)
		klog.CtxInfof(ctx, "Push local stale userID=%s connectionID=%s gatewayAddr=%s", meta.UserID, meta.ConnectionID, meta.GatewayAddr)
		return
	}
	if err := r.hub.SendToConnection(meta.ConnectionID, frontierMsg); err != nil {
		summary.fail(err.Error())
		klog.CtxWarnf(ctx, "Push local failed userID=%s connectionID=%s error=%v", meta.UserID, meta.ConnectionID, err)
		return
	}
	summary.success()
	klog.CtxInfof(ctx, "Push local success userID=%s connectionID=%s", meta.UserID, meta.ConnectionID)
}

func (r *Router) routeRemoteConnection(ctx context.Context, meta route.ConnectionMeta, pushMsg *backbon.PushMessage, summary *routeSummary) {
	resp, err := r.pushRemoteConnection(ctx, meta, pushMsg)
	if err != nil {
		summary.fail(err.Error())
		return
	}
	// 远端返回 stale/noop/success 后，本机只负责汇总语义和清理本地 Redis 索引。
	switch resp.GetStatus() {
	case backbon.PushStatus_PUSH_STATUS_SUCCESS:
		summary.success()
		klog.CtxInfof(ctx, "Push remote success userID=%s connectionID=%s gatewayAddr=%s", meta.UserID, meta.ConnectionID, meta.GatewayAddr)
	case backbon.PushStatus_PUSH_STATUS_STALE_CLEANED:
		summary.stale()
		r.cleanupStale(ctx, meta)
		klog.CtxInfof(ctx, "Push remote stale userID=%s connectionID=%s gatewayAddr=%s", meta.UserID, meta.ConnectionID, meta.GatewayAddr)
	case backbon.PushStatus_PUSH_STATUS_NOOP_OFFLINE:
		summary.noop()
		klog.CtxInfof(ctx, "Push remote noop userID=%s connectionID=%s gatewayAddr=%s", meta.UserID, meta.ConnectionID, meta.GatewayAddr)
	default:
		summary.fail(resp.GetError())
		klog.CtxWarnf(ctx, "Push remote failed userID=%s connectionID=%s gatewayAddr=%s status=%s error=%s",
			meta.UserID, meta.ConnectionID, meta.GatewayAddr, resp.GetStatus().String(), resp.GetError())
	}
}

func (r *Router) pushLocalConnection(connectionID string, frontierMsg *frontier.Message) (*backbon.PushToConnectionResp, error) {
	if r.hub == nil {
		return pushToConnectionResp(false, backbon.PushStatus_PUSH_STATUS_FAILED, "hub is not available"), fmt.Errorf("hub is not available")
	}
	if err := r.hub.SendToConnection(connectionID, frontierMsg); err != nil {
		return pushToConnectionResp(false, backbon.PushStatus_PUSH_STATUS_FAILED, err.Error()), err
	}
	return pushToConnectionResp(true, backbon.PushStatus_PUSH_STATUS_SUCCESS, ""), nil
}

func (r *Router) pushRemoteConnection(ctx context.Context, meta route.ConnectionMeta, pushMsg *backbon.PushMessage) (*backbon.PushToConnectionResp, error) {
	if r.remotes == nil {
		return pushToConnectionResp(false, backbon.PushStatus_PUSH_STATUS_FAILED, "remote client provider is not available"), fmt.Errorf("remote client provider is not available")
	}
	remoteClient, err := r.remotes.GetRemoteClient(meta.GatewayAddr)
	if err != nil {
		return pushToConnectionResp(false, backbon.PushStatus_PUSH_STATUS_FAILED, err.Error()), err
	}
	resp, err := remoteClient.PushToConnection(ctx, &backbon.PushToConnectionReq{
		ConnectionID: meta.ConnectionID,
		Message:      pushMsg,
	})
	if err != nil {
		return pushToConnectionResp(false, backbon.PushStatus_PUSH_STATUS_FAILED, err.Error()), err
	}
	if resp == nil {
		return pushToConnectionResp(false, backbon.PushStatus_PUSH_STATUS_FAILED, "invalid response from remote service"), nil
	}
	return resp, nil
}

func (r *Router) cleanupStale(ctx context.Context, meta route.ConnectionMeta) {
	if r.store == nil || meta.ConnectionID == "" {
		return
	}
	if err := r.store.Cleanup(ctx, meta.UserID, meta.ConnectionID); err != nil {
		klog.CtxWarnf(ctx, "Failed to cleanup stale route userID=%s connectionID=%s error=%v", meta.UserID, meta.ConnectionID, err)
	}
}

func (r *Router) isLocal(addr string) bool {
	return addr == r.localAddr
}

func result(userID string, success bool, status backbon.PushStatus, count int32, err string) *backbon.PushResult {
	return &backbon.PushResult{
		UserID:          userID,
		Success:         success,
		Error:           err,
		ConnectionCount: count,
		Status:          status,
	}
}

func pushToConnectionResp(success bool, status backbon.PushStatus, err string) *backbon.PushToConnectionResp {
	return &backbon.PushToConnectionResp{
		Success: success,
		Error:   err,
		Status:  status,
	}
}
