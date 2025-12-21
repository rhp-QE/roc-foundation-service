package service

import (
	"context"

	backbon "github.com/rhp-QE/roc-foundation-service/long_connection_service/kitex_gen/backbon"
	"github.com/rhp-QE/roc-foundation-service/long_connection_service/src/frontier"
)

// CheckUserOnline 检查用户在线状态
func (s *backbonServiceImpl) CheckUserOnline(ctx context.Context, req *backbon.CheckUserOnlineReq) (resp *backbon.CheckUserOnlineResp, err error) {
	// TODO: 实现检查用户在线状态逻辑
	resp = &backbon.CheckUserOnlineResp{
		Statuses: []*backbon.UserOnlineStatus{},
	}

	hub := s.getHub()
	if hub == nil {
		return resp, nil
	}

	// 遍历用户ID列表，检查在线状态
	for _, userID := range req.GetUserIDs() {
		if userID == "" {
			continue
		}

		status := s.checkUserOnlineStatus(ctx, userID, hub)
		if status != nil {
			resp.Statuses = append(resp.Statuses, status)
		}
	}

	return resp, nil
}

// checkUserOnlineStatus 检查单个用户的在线状态
func (s *backbonServiceImpl) checkUserOnlineStatus(ctx context.Context, userID string, hub *frontier.Hub) *backbon.UserOnlineStatus {
	// 获取本地连接
	conns := hub.GetUserConnections(userID)
	localConnCount := int32(len(conns))
	isOnline := localConnCount > 0

	// TODO: 从 storage 获取远程机器的连接信息，合并在线状态

	status := &backbon.UserOnlineStatus{
		UserID:          userID,
		IsOnline:        isOnline,
		ConnectionCount: localConnCount,
		PlatformIDs:     []int32{}, // TODO: 从连接信息中提取平台ID
		LastActiveTime:  0,         // TODO: 从连接信息中获取最后活跃时间
	}

	return status
}
