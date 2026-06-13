package service

import (
	"context"
	"fmt"

	backbon "github.com/rhp-QE/roc-foundation-service/long_connection_service/kitex_gen/backbon"
	"github.com/rhp-QE/roc-foundation-service/long_connection_service/src/frontier"
)

// PushToConnection 向指定连接推送消息
func (s *backbonServiceImpl) PushToConnection(ctx context.Context, req *backbon.PushToConnectionReq) (*backbon.PushToConnectionResp, error) {
	if req == nil {
		return &backbon.PushToConnectionResp{
			Success: false,
			Status:  backbon.PushStatus_PUSH_STATUS_FAILED,
			Error:   "request is required",
		}, fmt.Errorf("request is required")
	}
	if req.GetConnectionID() == "" {
		return &backbon.PushToConnectionResp{
			Success: false,
			Status:  backbon.PushStatus_PUSH_STATUS_FAILED,
			Error:   "connectionID is required",
		}, fmt.Errorf("connectionID is required")
	}
	if req.GetMessage() == nil {
		return &backbon.PushToConnectionResp{
			Success: false,
			Status:  backbon.PushStatus_PUSH_STATUS_FAILED,
			Error:   "message is required",
		}, fmt.Errorf("message is required")
	}

	frontierMsg := frontier.PushMessageToFrontierMessage(req.GetMessage())
	return s.newPushRouter().RouteConnection(ctx, req.GetConnectionID(), req.GetMessage(), frontierMsg)
}
