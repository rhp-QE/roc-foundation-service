// Package backbon 实现 Backbon RPC 服务
//
// Author: Ruan Huipeng
// Date: 2025-12-01

package backbon

import (
	"context"
	"fmt"
	"log"

	"github.com/rhp-QE/roc-foundation-service/long_connection_service/kitex_gen"
	backbonservice "github.com/rhp-QE/roc-foundation-service/long_connection_service/kitex_gen/backbonservice"
	"github.com/rhp-QE/roc-foundation-service/long_connection_service/src/frontier"
	"github.com/roc/roc-foundation-util-go/cache"
	"github.com/roc/roc-foundation-util-go/stringutil"
)

// PushData implements the BackbonServiceImpl interface.
// 推送数据到用户：先检查连接是否在本机，如果在本机直接推送，否则调用远程 BackbonService
func (s *BackbonServiceImpl) PushData(ctx context.Context, req *kitex_gen.PushDataReq) (resp *kitex_gen.PushDataResp, err error) {
	resp = &kitex_gen.PushDataResp{
		SuccessCount: 0,
		FailCount:    0,
		Results:      []*kitex_gen.PushResult{},
	}

	// 验证请求
	if err := s.validatePushRequest(req, resp); err != nil {
		return resp, err
	}

	// 转换消息
	pushMsg := req.GetMessage()
	frontierMsg := convertPushMessageToFrontierMessage(pushMsg)

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
func (s *BackbonServiceImpl) validatePushRequest(req *kitex_gen.PushDataReq, resp *kitex_gen.PushDataResp) error {
	pushMsg := req.GetMessage()
	if pushMsg == nil {
		resp.FailCount = 1
		resp.Results = append(resp.Results, &kitex_gen.PushResult{
			Success: false,
			Error:   "message is required",
		})
		return fmt.Errorf("message is required")
	}

	// 非广播模式需要用户ID列表
	if !req.GetBroadcast() && len(req.GetUserIDs()) == 0 {
		resp.FailCount++
		resp.Results = append(resp.Results, &kitex_gen.PushResult{
			Success: false,
			Error:   "userIDs is required when broadcast is false",
		})
		return fmt.Errorf("userIDs is required when broadcast is false")
	}

	return nil
}

// handleBroadcast 处理广播模式
func (s *BackbonServiceImpl) handleBroadcast(frontierMsg *frontier.Message, resp *kitex_gen.PushDataResp) {
	if s.hub == nil {
		resp.FailCount++
		resp.Results = append(resp.Results, &kitex_gen.PushResult{
			Success: false,
			Error:   "hub is not available",
		})
		return
	}

	s.hub.Broadcast(frontierMsg)
	resp.SuccessCount++
	log.Printf("Broadcast message to all connections on local machine")
}

// pushToUsers 推送到多个用户
func (s *BackbonServiceImpl) pushToUsers(
	ctx context.Context,
	userIDs []string,
	pushMsg *kitex_gen.PushMessage,
	frontierMsg *frontier.Message,
	resp *kitex_gen.PushDataResp,
) {
	localAddr := s.serviceCtx.GetLocalAddress()

	for _, userID := range userIDs {
		if stringutil.IsEmpty(userID) {
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
func (s *BackbonServiceImpl) pushToUser(
	ctx context.Context,
	userID string,
	pushMsg *kitex_gen.PushMessage,
	frontierMsg *frontier.Message,
	localAddr string,
) *kitex_gen.PushResult {
	result := &kitex_gen.PushResult{
		UserID:  userID,
		Success: false,
	}

	var localConnCount int32
	// 先尝试本地推送
	if s.hub != nil {
		localResult := s.tryPushToLocalUser(userID, frontierMsg)
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

	// 如果有错误，记录第一个错误
	if !result.Success && len(remoteResults) > 0 {
		for _, remoteResult := range remoteResults {
			if !stringutil.IsEmpty(remoteResult.Error) {
				result.Error = remoteResult.Error
				break
			}
		}
	}

	return result
}

// tryPushToLocalUser 尝试推送到本地用户，返回推送结果（如果本地没有连接则返回 nil）
func (s *BackbonServiceImpl) tryPushToLocalUser(
	userID string,
	frontierMsg *frontier.Message,
) *kitex_gen.PushResult {
	if s.hub == nil {
		return nil
	}

	conns := s.hub.GetUserConnections(userID)
	if len(conns) == 0 {
		return nil
	}

	// 连接在本机，直接推送
	result := &kitex_gen.PushResult{
		UserID:  userID,
		Success: false,
	}

	err := s.hub.SendToUser(userID, frontierMsg)
	if err == nil {
		result.Success = true
		result.ConnectionCount = int32(len(conns))
		log.Printf("Pushed message to user %s on local machine (%d connections)", userID, result.ConnectionCount)
	} else {
		result.Error = err.Error()
		log.Printf("Failed to push message to user %s on local machine: %v", userID, err)
	}

	return result
}

// pushToRemoteMachines 推送到所有远程机器的连接
func (s *BackbonServiceImpl) pushToRemoteMachines(
	ctx context.Context,
	userID string,
	pushMsg *kitex_gen.PushMessage,
	localAddr string,
) []*kitex_gen.PushResult {
	// 获取 Redis 客户端
	redisCache := s.serviceCtx.GetRedis()
	if redisCache == nil {
		return []*kitex_gen.PushResult{{
			UserID:  userID,
			Success: false,
			Error:   "Redis is not available",
		}}
	}

	// 从 Redis 获取用户的所有连接信息（connectionID -> machineAddr 映射）
	connectionMappings, err := s.getUserConnectionMappings(ctx, userID, redisCache)
	if err != nil {
		log.Printf("User %s connection mappings not found in Redis: %v", userID, err)
		return []*kitex_gen.PushResult{{
			UserID:  userID,
			Success: false,
			Error:   fmt.Sprintf("user %s connection mappings not found in Redis: %v", userID, err),
		}}
	}

	if len(connectionMappings) == 0 {
		return []*kitex_gen.PushResult{}
	}

	// 遍历所有连接进行推送
	results := make([]*kitex_gen.PushResult, 0, len(connectionMappings))
	for connectionID, machineAddr := range connectionMappings {
		// 跳过本机地址（本机已经在 tryPushToLocalUser 中处理）
		if machineAddr == localAddr {
			continue
		}

		result := s.callRemotePushToConnection(ctx, connectionID, pushMsg, machineAddr, &kitex_gen.PushResult{
			UserID:  userID,
			Success: false,
		})
		results = append(results, result)
	}

	return results
}

// getUserConnectionMappings 从 Redis 获取用户的所有连接映射
// Redis 存储结构：key = "fronter:user:connection:{userID}" (Hash)
//   - field: connectionID
//   - value: 机器地址 (ip:port)
func (s *BackbonServiceImpl) getUserConnectionMappings(ctx context.Context, userID string, redisCache cache.Cache) (map[string]string, error) {
	userKey := s.getUserConnectionKey(userID)

	// 使用 HGetAll 获取 Hash 的所有字段和值
	mappings, err := redisCache.HGetAll(ctx, userKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection mappings from Redis: %w", err)
	}

	if len(mappings) == 0 {
		return nil, fmt.Errorf("no connection mappings found for user %s", userID)
	}

	// 过滤空值
	result := make(map[string]string, len(mappings))
	for connectionID, machineAddr := range mappings {
		if !stringutil.IsEmpty(connectionID) && !stringutil.IsEmpty(machineAddr) {
			result[connectionID] = machineAddr
		}
	}

	return result, nil
}

// callRemotePushToConnection 调用远程推送服务（通过 connectionID）
func (s *BackbonServiceImpl) callRemotePushToConnection(
	ctx context.Context,
	connectionID string,
	pushMsg *kitex_gen.PushMessage,
	machineAddr string,
	result *kitex_gen.PushResult,
) *kitex_gen.PushResult {
	// 创建远程客户端
	remoteClient, err := s.createRemoteClient(machineAddr)
	if err != nil {
		result.Error = fmt.Sprintf("failed to create remote client: %v", err)
		log.Printf("Failed to create remote client for %s: %v", machineAddr, err)
		return result
	}

	// 构建远程推送请求（使用 PushToConnection）
	remoteReq := &kitex_gen.PushToConnectionReq{
		ConnectionID: connectionID,
		Message:      pushMsg,
	}

	// 调用远程服务
	remoteResp, err := remoteClient.PushToConnection(ctx, remoteReq)
	if err != nil {
		result.Error = fmt.Sprintf("failed to call remote PushToConnection service: %v", err)
		log.Printf("Failed to call remote PushToConnection at %s for connection %s: %v", machineAddr, connectionID, err)
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

	log.Printf("Pushed message to connection %s on remote machine %s: success=%v", connectionID, machineAddr, result.Success)
	return result
}

// createRemoteClient 获取或创建远程客户端（复用客户端实例）
func (s *BackbonServiceImpl) createRemoteClient(machineAddr string) (backbonservice.Client, error) {
	return s.serviceCtx.GetRemoteClient(machineAddr)
}

// PushToConnection implements the BackbonServiceImpl interface.
// 向指定连接推送消息：检查连接是否在本机，如果在本机直接推送，否则从 Redis 查找并调用远程服务
func (s *BackbonServiceImpl) PushToConnection(ctx context.Context, req *kitex_gen.PushToConnectionReq) (resp *kitex_gen.PushToConnectionResp, err error) {
	resp = &kitex_gen.PushToConnectionResp{
		Success: false,
	}

	// 验证请求
	connectionID := req.GetConnectionID()
	if stringutil.IsEmpty(connectionID) {
		resp.Error = "connectionID is required"
		return resp, fmt.Errorf("connectionID is required")
	}

	pushMsg := req.GetMessage()
	if pushMsg == nil {
		resp.Error = "message is required"
		return resp, fmt.Errorf("message is required")
	}

	// 转换为 frontier.Message
	frontierMsg := convertPushMessageToFrontierMessage(pushMsg)

	// 检查连接是否在本机
	if s.hub != nil {
		_, exists := s.hub.GetConnection(connectionID)
		if exists {
			// 连接在本机，直接推送
			err := s.hub.SendToConnection(connectionID, frontierMsg)
			if err == nil {
				resp.Success = true
				log.Printf("Pushed message to connection %s on local machine", connectionID)
			} else {
				resp.Error = err.Error()
				log.Printf("Failed to push message to connection %s on local machine: %v", connectionID, err)
			}
			return resp, err
		}
	}

	// 连接不在本机，从 Redis 获取连接所在机器并调用远程服务
	redisCache := s.serviceCtx.GetRedis()
	if redisCache == nil {
		resp.Error = "Redis is not available"
		return resp, fmt.Errorf("redis is not available")
	}

	// 从 Redis 获取连接所在机器地址
	connectionKey := s.getConnectionKey(connectionID)
	machineAddr, err := redisCache.Get(ctx, connectionKey)
	if err != nil || stringutil.IsEmpty(machineAddr) {
		resp.Error = fmt.Sprintf("connection %s not found in Redis", connectionID)
		log.Printf("Connection %s not found in Redis", connectionID)
		return resp, fmt.Errorf("connection %s not found", connectionID)
	}

	// 调用远程服务
	return s.callRemotePushToConnectionFromLocal(ctx, connectionID, pushMsg, machineAddr)
}

// callRemotePushToConnectionFromLocal 从本机调用远程服务的 PushToConnection
func (s *BackbonServiceImpl) callRemotePushToConnectionFromLocal(
	ctx context.Context,
	connectionID string,
	pushMsg *kitex_gen.PushMessage,
	machineAddr string,
) (*kitex_gen.PushToConnectionResp, error) {
	resp := &kitex_gen.PushToConnectionResp{
		Success: false,
	}

	// 创建远程客户端
	remoteClient, err := s.createRemoteClient(machineAddr)
	if err != nil {
		resp.Error = fmt.Sprintf("failed to create remote client: %v", err)
		log.Printf("Failed to create remote client for %s: %v", machineAddr, err)
		return resp, err
	}

	// 构建远程推送请求
	remoteReq := &kitex_gen.PushToConnectionReq{
		ConnectionID: connectionID,
		Message:      pushMsg,
	}

	// 调用远程服务
	remoteResp, err := remoteClient.PushToConnection(ctx, remoteReq)
	if err != nil {
		resp.Error = fmt.Sprintf("failed to call remote PushToConnection service: %v", err)
		log.Printf("Failed to call remote PushToConnection at %s for connection %s: %v", machineAddr, connectionID, err)
		return resp, err
	}

	if remoteResp == nil {
		resp.Error = "invalid response from remote service"
		return resp, fmt.Errorf("invalid response from remote service")
	}

	resp.Success = remoteResp.GetSuccess()
	resp.Error = remoteResp.GetError()

	log.Printf("Pushed message to connection %s on remote machine %s: success=%v", connectionID, machineAddr, resp.Success)
	return resp, nil
}
