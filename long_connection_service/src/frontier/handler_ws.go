package frontier

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/kitex-contrib/obs-opentelemetry/tracing"
	back "github.com/rhp-QE/roc-foundation-service/long_connection_service/kitex_gen/back"
	backservice "github.com/rhp-QE/roc-foundation-service/long_connection_service/kitex_gen/back/backservice"
	"github.com/rhp-QE/roc-foundation-service/long_connection_service/src/util"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// 在生产环境中应该检查 Origin
		return true
	},
}

// WebSocketHandler WebSocket处理器
type WebSocketHandler struct {
	hub *Hub
}

type websocketSession struct {
	connection *Connection
}

// NewWebSocketHandler 创建WebSocket处理器
func NewWebSocketHandler(hub *Hub) *WebSocketHandler {
	return &WebSocketHandler{
		hub: hub,
	}
}

// ServeHTTP 处理WebSocket连接请求
func (h *WebSocketHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 主流程只保留接入步骤，认证、注册和 pump 启动分别下沉，避免入口堆业务细节。
	rawConn, err := h.upgrade(w, r)
	if err != nil {
		return
	}

	session, err := h.authenticateAndBuildSession(r, rawConn)
	if err != nil {
		h.reject(rawConn, err)
		return
	}

	h.registerSession(session)
	h.sendWelcome(session)
	h.startConnectionPumps(session)
}

func (h *WebSocketHandler) upgrade(w http.ResponseWriter, r *http.Request) (*websocket.Conn, error) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "Failed to upgrade connection", http.StatusBadRequest)
		return nil, err
	}
	return conn, nil
}

func (h *WebSocketHandler) authenticateAndBuildSession(r *http.Request, rawConn *websocket.Conn) (*websocketSession, error) {
	// 认证成功后才创建带 userID 的连接会话，后续 register 才能写 route。
	binding, err := h.authenticateClient(r)
	if err != nil {
		klog.CtxWarnf(r.Context(), "WebSocket auth failed remoteAddr=%s error=%v", r.RemoteAddr, err)
		return nil, err
	}

	connection := NewConnection(uuid.New().String(), binding.userID, rawConn, h.hub)
	connection.SetMetadata("auth_mode", binding.authMode)
	connection.SetMetadata("track_id", binding.trackID)
	connection.SetMetadata("deviceID", binding.deviceID)
	connection.SetMetadata("platform", binding.platform)
	connection.SetMetadata("clientVersion", binding.clientVersion)
	if binding.token != "" {
		connection.SetMetadata("token", binding.token)
	}

	klog.CtxInfof(r.Context(), "WebSocket auth success userID=%s deviceID=%s connectionID=%s platform=%s authMode=%s",
		binding.userID, binding.deviceID, connection.ID, binding.platform, binding.authMode)
	return &websocketSession{connection: connection}, nil
}

type authBinding struct {
	userID        string
	token         string
	authMode      string
	trackID       string
	deviceID      string
	platform      string
	clientVersion string
}

func (h *WebSocketHandler) authenticateClient(r *http.Request) (authBinding, error) {
	query := r.URL.Query()
	token := query.Get("token")
	queryUserID := query.Get("user_id")
	boundUserID, authMode, err := bindUserIDFromToken(token)
	if err != nil {
		return authBinding{}, err
	}
	// query user_id 只做一致性校验，不作为身份来源，避免伪造用户污染 route。
	if queryUserID != "" && queryUserID != boundUserID {
		return authBinding{}, ErrUnauthorized
	}

	return authBinding{
		userID:        boundUserID,
		token:         token,
		authMode:      authMode,
		trackID:       query.Get("track_id"),
		deviceID:      firstNonEmpty(query.Get("deviceID"), query.Get("device_id")),
		platform:      firstNonEmpty(query.Get("platform"), query.Get("sdkType"), query.Get("sdk_type")),
		clientVersion: firstNonEmpty(query.Get("clientVersion"), query.Get("client_version")),
	}, nil
}

func bindUserIDFromToken(token string) (string, string, error) {
	// 当前先接入显式 mock token；正式鉴权服务接入后只替换这里的 token binder。
	token = strings.TrimSpace(token)
	if token == "" {
		return "", "", ErrUnauthorized
	}

	tokenUserID := userIDFromMockToken(token)
	if tokenUserID == "" {
		return "", "", ErrUnauthorized
	}
	return tokenUserID, "mock_token_bound", nil
}

func userIDFromMockToken(token string) string {
	for _, prefix := range []string{"user:", "uid:", "mock:"} {
		if strings.HasPrefix(token, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(token, prefix))
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func (h *WebSocketHandler) registerSession(session *websocketSession) {
	h.hub.Register <- session.connection
}

func (h *WebSocketHandler) sendWelcome(session *websocketSession) {
	welcomeMsg := NewMessage(MessageTypePush)
	welcomeMsg.SetMetadata("connectionID", session.connection.ID)
	welcomeMsg.SetMetadata("userID", session.connection.UserID)
	_ = session.connection.SendMessage(welcomeMsg)
}

func (h *WebSocketHandler) startConnectionPumps(session *websocketSession) {
	go session.connection.WritePump()
	go session.connection.ReadPump()
}

func (h *WebSocketHandler) reject(conn *websocket.Conn, err error) {
	_ = conn.WriteControl(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.ClosePolicyViolation, err.Error()),
		time.Now().Add(time.Second),
	)
	_ = conn.Close()
}

// HandleAuth 处理认证请求
// 注意：网关不解析payload，认证逻辑应由业务方处理
// token、track_id等验证信息应该放在metadata中，payload是业务数据
func (h *WebSocketHandler) HandleAuth(conn *Connection, msg *Message) error {
	// 从metadata中获取验证信息（token、track_id等）
	// 网关只负责转发，不解析payload内容
	// 业务方应该从 msg.GetMetadata() 中获取验证信息，从 msg.Payload 中获取业务数据

	// 将metadata中的验证信息保存到连接的元数据中
	if token, ok := msg.GetMetadata("token"); ok {
		conn.SetMetadata("token", token)
	}
	if trackID, ok := msg.GetMetadata("track_id"); ok {
		conn.SetMetadata("track_id", trackID)
	}
	if userID, ok := msg.GetMetadata("user_id"); ok {
		if userID != "" && userID != conn.UserID {
			return ErrUnauthorized
		}
	}
	if deviceID, ok := msg.GetMetadata("deviceID"); ok {
		conn.SetMetadata("deviceID", deviceID)
	}
	if platform, ok := msg.GetMetadata("platform"); ok {
		conn.SetMetadata("platform", platform)
	}
	if clientVersion, ok := msg.GetMetadata("clientVersion"); ok {
		conn.SetMetadata("clientVersion", clientVersion)
	}

	response := NewMessage(MessageTypeAuth)
	return conn.SendMessage(response)
}

// HandleMessage 处理RPC请求消息（网关模式）
func (h *WebSocketHandler) HandleMessage(conn *Connection, msg *Message) error {
	// 验证请求
	if err := h.validateRequest(msg); err != nil {
		return err
	}

	// 获取服务上下文
	serviceCtx := h.hub.GetServiceContext()
	if serviceCtx == nil {
		response := NewErrorMessage(msg.RequestID, "service context not available")
		return conn.SendMessage(response)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 1. 检查服务是否注册
	if err := h.checkServiceRegistered(ctx, serviceCtx, msg); err != nil {
		response := NewErrorMessage(msg.RequestID, err.Error())
		return conn.SendMessage(response)
	}

	// 2. 获取服务实例并调用后端服务
	callResp, err := h.callBackendService(ctx, serviceCtx, msg, conn)
	if err != nil {
		response := NewErrorMessage(msg.RequestID, err.Error())
		return conn.SendMessage(response)
	}

	// 3. 转换响应并返回
	response := CallResponseToFontierMessage(callResp, msg.RequestID)
	return conn.SendMessage(response)
}

// validateRequest 验证请求消息
func (h *WebSocketHandler) validateRequest(msg *Message) error {
	if msg.Service == "" {
		return ErrServiceNotFound
	}
	if msg.Method == "" {
		return ErrMethodNotFound
	}
	return nil
}

// callBackendService 通过 Kitex resolver 调用后端服务。
func (h *WebSocketHandler) callBackendService(ctx context.Context, serviceCtx ServiceContext, msg *Message, conn *Connection) (*back.CallResponse, error) {
	resolver := serviceCtx.GetKitexResolver()
	if resolver == nil {
		return nil, fmt.Errorf("kitex resolver not available")
	}

	backServiceClient, err := backservice.NewClient(
		msg.Service,
		client.WithResolver(resolver),
		client.WithSuite(tracing.NewClientSuite()),
		client.WithClientBasicInfo(&rpcinfo.EndpointBasicInfo{ServiceName: "frontier"}),
		client.WithRPCTimeout(10*time.Second),
	)
	if err != nil {
		klog.Errorf("Failed to create backservice client for %s: %v", msg.Service, err)
		return nil, fmt.Errorf("failed to create client: %v", err)
	}

	// 构建 CallRequest
	callReq := MessageToCallRequest(msg, conn)

	// 调用后端服务
	callResp, err := backServiceClient.Call(ctx, callReq)
	if err != nil {
		klog.Errorf("Failed to call backservice %s.%s: %v", msg.Service, msg.Method, err)
		return nil, fmt.Errorf("service call failed: %v", err)
	}

	return callResp, nil
}

// checkServiceRegistered 检查服务和方法是否已注册
func (h *WebSocketHandler) checkServiceRegistered(ctx context.Context, serviceCtx ServiceContext, msg *Message) error {
	// 获取 Redis 客户端
	redisCache := serviceCtx.GetRedis()
	if redisCache == nil {
		return fmt.Errorf("redis is not available")
	}

	// 构建 Redis key
	key := util.GetServiceKeyInCache(msg.Service)

	// 检查服务是否存在
	exists, err := redisCache.Exists(ctx, key)
	if err != nil {
		klog.Errorf("Failed to check service existence: %v", err)
		return fmt.Errorf("failed to check service existence: %w", err)
	}

	if exists == 0 {
		return fmt.Errorf("service %s with method %s is not registered", msg.Service, msg.Method)
	}

	// 检查 method 是否在 Set 中
	isMember, err := redisCache.SIsMember(ctx, key, msg.Method)
	if err != nil {
		klog.Errorf("Failed to check method: %v", err)
		return fmt.Errorf("failed to check method: %w", err)
	}

	// 检查是否支持所有方法（"*" 标记）
	allMethods, err := redisCache.SIsMember(ctx, key, "*")
	if err != nil {
		klog.Errorf("Failed to check all methods flag: %v", err)
		return fmt.Errorf("failed to check all methods flag: %w", err)
	}

	// 如果支持所有方法，或者指定的 method 在 Set 中，则返回 nil（已注册）
	if allMethods || isMember {
		return nil
	}

	return fmt.Errorf("service %s with method %s is not registered", msg.Service, msg.Method)
}
