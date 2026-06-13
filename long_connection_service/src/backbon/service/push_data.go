package service

import (
	"context"
	"fmt"

	backbon "github.com/rhp-QE/roc-foundation-service/long_connection_service/kitex_gen/backbon"
	"github.com/rhp-QE/roc-foundation-service/long_connection_service/src/frontier"
)

// PushData 推送数据到用户
func (s *backbonServiceImpl) PushData(ctx context.Context, req *backbon.PushDataReq) (*backbon.PushDataResp, error) {
	resp := &backbon.PushDataResp{
		Results: []*backbon.PushResult{},
	}

	if err := s.validatePushRequest(req, resp); err != nil {
		return resp, err
	}

	// API 层只做协议转换和结果汇总；路由、stale 清理和状态判断都在 PushRouter。
	frontierMsg := frontier.PushMessageToFrontierMessage(req.GetMessage())
	if req.GetBroadcast() {
		result := s.newPushRouter().Broadcast(frontierMsg)
		s.appendPushResult(resp, result)
		return resp, nil
	}

	for _, userID := range req.GetUserIDs() {
		result := s.newPushRouter().RouteUser(ctx, userID, req.GetMessage(), frontierMsg)
		s.appendPushResult(resp, result)
	}

	return resp, nil
}

func (s *backbonServiceImpl) validatePushRequest(req *backbon.PushDataReq, resp *backbon.PushDataResp) error {
	if req == nil {
		s.appendPushResult(resp, &backbon.PushResult{
			Success: false,
			Status:  backbon.PushStatus_PUSH_STATUS_FAILED,
			Error:   "request is required",
		})
		return fmt.Errorf("request is required")
	}
	if req.GetMessage() == nil {
		s.appendPushResult(resp, &backbon.PushResult{
			Success: false,
			Status:  backbon.PushStatus_PUSH_STATUS_FAILED,
			Error:   "message is required",
		})
		return fmt.Errorf("message is required")
	}
	if !req.GetBroadcast() && len(req.GetUserIDs()) == 0 {
		s.appendPushResult(resp, &backbon.PushResult{
			Success: false,
			Status:  backbon.PushStatus_PUSH_STATUS_FAILED,
			Error:   "userIDs is required when broadcast is false",
		})
		return fmt.Errorf("userIDs is required when broadcast is false")
	}
	return nil
}

func (s *backbonServiceImpl) appendPushResult(resp *backbon.PushDataResp, result *backbon.PushResult) {
	if result == nil {
		return
	}
	resp.Results = append(resp.Results, result)
	if result.GetSuccess() {
		resp.SuccessCount++
		return
	}
	resp.FailCount++
}
