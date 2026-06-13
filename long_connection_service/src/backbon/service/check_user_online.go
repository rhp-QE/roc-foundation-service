package service

import (
	"context"
	"errors"

	backbon "github.com/rhp-QE/roc-foundation-service/long_connection_service/kitex_gen/backbon"
	"github.com/rhp-QE/roc-foundation-service/long_connection_service/src/route"
)

// CheckUserOnline 检查用户在线状态
func (s *backbonServiceImpl) CheckUserOnline(ctx context.Context, req *backbon.CheckUserOnlineReq) (*backbon.CheckUserOnlineResp, error) {
	resp := &backbon.CheckUserOnlineResp{
		Statuses: []*backbon.UserOnlineStatus{},
	}
	if req == nil {
		return resp, nil
	}

	for _, userID := range req.GetUserIDs() {
		if userID == "" {
			continue
		}
		resp.Statuses = append(resp.Statuses, s.checkUserOnlineStatus(ctx, userID))
	}

	return resp, nil
}

func (s *backbonServiceImpl) checkUserOnlineStatus(ctx context.Context, userID string) *backbon.UserOnlineStatus {
	metas, err := s.routeStore().ListByUser(ctx, userID)
	if err != nil {
		if errors.Is(err, route.ErrNoLiveConnections) {
			return offlineStatus(userID)
		}
		return offlineStatus(userID)
	}

	status := &backbon.UserOnlineStatus{
		UserID:          userID,
		IsOnline:        len(metas) > 0,
		ConnectionCount: int32(len(metas)),
		PlatformIDs:     []int32{},
	}
	for _, meta := range metas {
		if meta.LastActiveAt > status.LastActiveTime {
			status.LastActiveTime = meta.LastActiveAt
		}
	}
	return status
}

func offlineStatus(userID string) *backbon.UserOnlineStatus {
	return &backbon.UserOnlineStatus{
		UserID:          userID,
		IsOnline:        false,
		ConnectionCount: 0,
		PlatformIDs:     []int32{},
		LastActiveTime:  0,
	}
}
