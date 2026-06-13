package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/cloudwego/kitex/pkg/klog"
	backbon "github.com/rhp-QE/roc-foundation-service/long_connection_service/kitex_gen/backbon"
	"github.com/rhp-QE/roc-foundation-service/long_connection_service/src/backbon/storage"
	"github.com/rhp-QE/roc-foundation-service/long_connection_service/src/frontier"
)

// PushData 推送数据到用户
func (s *backbonServiceImpl) PushData(ctx context.Context, req *backbon.PushDataReq) (resp *backbon.PushDataResp, err error) {
	resp = &backbon.PushDataResp{
		SuccessCount: 0,
		FailCount:    0,
		Results:      []*backbon.PushResult{},
	}

	// 验证请求
	if err := s.validatePushRequest(req, resp); err != nil {
		return resp, err
	}

	// 转换消息
	pushMsg := req.GetMessage()
	frontierMsg := frontier.PushMessageToFrontierMessage(pushMsg)

	// 广播模式处理
	if req.GetBroadcast() {
		s.handleBroadcast(frontierMsg, resp)
		return resp, nil
	}

	// 单播模式：遍历用户ID推送
	s.pushToUsers(ctx, req.GetUserIDs(), pushMsg, frontierMsg, resp)

	return resp, nil
}

// validatePushRequest 验证推送请求
func (s *backbonServiceImpl) validatePushRequest(req *backbon.PushDataReq, resp *backbon.PushDataResp) error {
	pushMsg := req.GetMessage()
	if pushMsg == nil {
		resp.FailCount = 1
		resp.Results = append(resp.Results, &backbon.PushResult{
			Success: false,
			Error:   "message is required",
		})
		return fmt.Errorf("message is required")
	}

	// 非广播模式需要用户ID列表
	if !req.GetBroadcast() && len(req.GetUserIDs()) == 0 {
		resp.FailCount++
		resp.Results = append(resp.Results, &backbon.PushResult{
			Success: false,
			Error:   "userIDs is required when broadcast is false",
		})
		return fmt.Errorf("userIDs is required when broadcast is false")
	}

	return nil
}

// handleBroadcast 处理广播模式
func (s *backbonServiceImpl) handleBroadcast(frontierMsg *frontier.Message, resp *backbon.PushDataResp) {
	hub := s.getHub()
	if hub == nil {
		resp.FailCount++
		resp.Results = append(resp.Results, &backbon.PushResult{
			Success: false,
			Error:   "hub is not available",
		})
		return
	}

	hub.Broadcast(frontierMsg)
	resp.SuccessCount++
	klog.Infof("Broadcast message to all connections on local machine")
}

// pushToUsers 推送到多个用户
func (s *backbonServiceImpl) pushToUsers(
	ctx context.Context,
	userIDs []string,
	pushMsg *backbon.PushMessage,
	frontierMsg *frontier.Message,
	resp *backbon.PushDataResp,
) {
	localAddr := s.serviceCtx.GetLocalAddress()

	for _, userID := range userIDs {
		if userID == "" {
			continue
		}

		result := s.pushToUser(ctx, userID, pushMsg, frontierMsg, localAddr)
		if result.Success {
			resp.SuccessCount++
		} else {
			resp.FailCount++
		}
		resp.Results = append(resp.Results, result)
	}
}

// pushToUser 推送到单个用户（可能有多台机器的连接）
func (s *backbonServiceImpl) pushToUser(
	ctx context.Context,
	userID string,
	pushMsg *backbon.PushMessage,
	frontierMsg *frontier.Message,
	localAddr string,
) *backbon.PushResult {
	result := &backbon.PushResult{
		UserID:  userID,
		Success: false,
	}

	var localConnCount int32
	// 先尝试本地推送
	hub := s.getHub()
	if hub != nil {
		localResult := s.tryPushToLocalUser(userID, frontierMsg, hub)
		if localResult != nil {
			localConnCount = localResult.ConnectionCount
			if localResult.Success {
				result.Success = true
				result.ConnectionCount = localConnCount
			}
		}
	}

	// 获取所有机器地址（包括远程机器）并进行推送
	remoteResults := s.pushToRemoteMachines(ctx, userID, pushMsg, localAddr)

	// 合并结果：累计连接数，只要有一个成功就算成功
	totalConnCount := localConnCount
	for _, remoteResult := range remoteResults {
		totalConnCount += remoteResult.ConnectionCount
		if remoteResult.Success {
			result.Success = true
		}
	}
	result.ConnectionCount = totalConnCount
	if totalConnCount == 0 && len(remoteResults) == 0 {
		result.Success = true
		return result
	}

	// 如果有错误，记录第一个错误
	if !result.Success && len(remoteResults) > 0 {
		for _, remoteResult := range remoteResults {
			if remoteResult.Error != "" {
				result.Error = remoteResult.Error
				break
			}
		}
	}

	return result
}

// tryPushToLocalUser 尝试推送到本地用户
func (s *backbonServiceImpl) tryPushToLocalUser(
	userID string,
	frontierMsg *frontier.Message,
	hub *frontier.Hub,
) *backbon.PushResult {
	conns := hub.GetUserConnections(userID)
	if len(conns) == 0 {
		return nil
	}

	// 连接在本机，直接推送
	result := &backbon.PushResult{
		UserID:  userID,
		Success: false,
	}

	err := hub.SendToUser(userID, frontierMsg)
	if err == nil {
		result.Success = true
		result.ConnectionCount = int32(len(conns))
		klog.Infof("Pushed message to user %s on local machine (%d connections)", userID, result.ConnectionCount)
	} else {
		result.Error = err.Error()
		klog.Errorf("Failed to push message to user %s on local machine: %v", userID, err)
	}

	return result
}

// pushToRemoteMachines 推送到所有远程机器的连接
func (s *backbonServiceImpl) pushToRemoteMachines(
	ctx context.Context,
	userID string,
	pushMsg *backbon.PushMessage,
	localAddr string,
) []*backbon.PushResult {
	// 从 storage 获取用户连接映射
	connectionMappings, err := s.getUserConnectionMappings(ctx, userID)
	if err != nil {
		if errors.Is(err, storage.ErrNoLiveConnectionMappings) {
			klog.CtxDebugf(ctx, "User %s has no live connection mappings, skip remote push", userID)
			return []*backbon.PushResult{}
		}

		klog.CtxWarnf(ctx, "User %s connection mappings not found: %v", userID, err)
		return []*backbon.PushResult{{
			UserID:  userID,
			Success: false,
			Error:   fmt.Sprintf("user %s connection mappings not found: %v", userID, err),
		}}
	}

	if len(connectionMappings) == 0 {
		return []*backbon.PushResult{}
	}

	// 遍历所有连接进行推送
	results := make([]*backbon.PushResult, 0, len(connectionMappings))
	store := storage.NewBackbonStorage(s.serviceCtx)
	for connectionID, machineAddr := range connectionMappings {
		// 跳过本机地址（本机已经在 tryPushToLocalUser 中处理）
		if machineAddr == localAddr {
			hub := s.getHub()
			if hub != nil {
				if _, exists := hub.GetConnection(connectionID); !exists {
					if err := store.RemoveConnectionMapping(ctx, userID, connectionID); err != nil {
						klog.CtxWarnf(ctx, "Failed to remove stale local connection mapping %s for user %s: %v", connectionID, userID, err)
					} else {
						klog.CtxInfof(ctx, "Removed stale local connection mapping %s for user %s", connectionID, userID)
					}
				}
			}
			continue
		}

		result := s.callRemotePushToConnection(ctx, connectionID, pushMsg, machineAddr, &backbon.PushResult{
			UserID:  userID,
			Success: false,
		})
		results = append(results, result)
	}

	return results
}

// getUserConnectionMappings 从 storage 获取用户的所有连接映射
func (s *backbonServiceImpl) getUserConnectionMappings(ctx context.Context, userID string) (map[string]string, error) {
	storage := storage.NewBackbonStorage(s.serviceCtx)
	return storage.GetUserConnectionMappings(ctx, userID)
}

// callRemotePushToConnection 调用远程推送服务（通过 connectionID）
func (s *backbonServiceImpl) callRemotePushToConnection(
	ctx context.Context,
	connectionID string,
	pushMsg *backbon.PushMessage,
	machineAddr string,
	result *backbon.PushResult,
) *backbon.PushResult {
	// 创建远程客户端
	remoteClient, err := s.serviceCtx.GetRemoteClient(machineAddr)
	if err != nil {
		result.Error = fmt.Sprintf("failed to create remote client: %v", err)
		klog.CtxErrorf(ctx, "Failed to create remote client for %s: %v", machineAddr, err)
		return result
	}

	// 构建远程推送请求（使用 PushToConnection）
	remoteReq := &backbon.PushToConnectionReq{
		ConnectionID: connectionID,
		Message:      pushMsg,
	}

	// 调用远程服务
	remoteResp, err := remoteClient.PushToConnection(ctx, remoteReq)
	if err != nil {
		result.Error = fmt.Sprintf("failed to call remote PushToConnection service: %v", err)
		klog.CtxErrorf(ctx, "Failed to call remote PushToConnection at %s for connection %s: %v", machineAddr, connectionID, err)
		return result
	}

	// 处理远程服务响应
	if remoteResp == nil {
		result.Error = "invalid response from remote service"
		return result
	}

	result.Success = remoteResp.GetSuccess()
	result.Error = remoteResp.GetError()
	if result.Success {
		result.ConnectionCount = 1 // PushToConnection 只推送一个连接
	}

	klog.CtxInfof(ctx, "Pushed message to connection %s on remote machine %s: success=%v", connectionID, machineAddr, result.Success)
	return result
}

// getHub 获取 Hub 实例
func (s *backbonServiceImpl) getHub() *frontier.Hub {
	if s.hub == nil {
		return nil
	}
	hub, ok := s.hub.(*frontier.Hub)
	if !ok {
		return nil
	}
	return hub
}
