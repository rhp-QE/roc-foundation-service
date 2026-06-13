package route

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/rhp-QE/roc-foundation-util-go/cache"
)

const DefaultTTL = 120 * time.Second

type RedisStore struct {
	redis       cache.Cache
	ttl         time.Duration
	gatewayAddr string
	gatewayID   string
}

func NewRedisStore(redis cache.Cache, gatewayAddr string, ttl time.Duration) *RedisStore {
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	return &RedisStore{
		redis:       redis,
		ttl:         ttl,
		gatewayAddr: gatewayAddr,
		gatewayID:   gatewayAddr,
	}
}

func (s *RedisStore) Register(ctx context.Context, meta ConnectionMeta) error {
	if err := s.validateReady(); err != nil {
		return err
	}
	if meta.UserID == "" || meta.ConnectionID == "" {
		return fmt.Errorf("userID and connectionID are required")
	}

	// 先写连接事实，再写用户索引，避免推送读到无法反查的 connectionID。
	meta = s.normalizeMeta(meta)
	if err := s.setConnection(ctx, meta); err != nil {
		return err
	}
	if err := s.indexConnection(ctx, meta); err != nil {
		_ = s.redis.Delete(ctx, ConnectionKey(meta.ConnectionID))
		return err
	}

	klog.CtxInfof(ctx, "Registered connection route userID=%s connectionID=%s gatewayID=%s ttl=%s",
		meta.UserID, meta.ConnectionID, meta.GatewayID, s.ttl)
	return nil
}

func (s *RedisStore) Refresh(ctx context.Context, userID string, connectionID string) error {
	if err := s.validateReady(); err != nil {
		return err
	}
	// refresh 必须反查连接事实，防止旧连接事件续期到新用户或新连接。
	meta, err := s.Get(ctx, connectionID)
	if err != nil {
		return err
	}
	if userID != "" && meta.UserID != "" && meta.UserID != userID {
		return fmt.Errorf("connection %s belongs to user %s, not %s", connectionID, meta.UserID, userID)
	}
	if userID != "" {
		meta.UserID = userID
	}
	meta = s.normalizeMeta(meta)
	return s.Register(ctx, meta)
}

func (s *RedisStore) Unregister(ctx context.Context, userID string, connectionID string) error {
	return s.Cleanup(ctx, userID, connectionID)
}

func (s *RedisStore) Get(ctx context.Context, connectionID string) (ConnectionMeta, error) {
	if err := s.validateReady(); err != nil {
		return ConnectionMeta{}, err
	}
	if connectionID == "" {
		return ConnectionMeta{}, ErrConnectionMissing
	}

	raw, err := s.redis.Get(ctx, ConnectionKey(connectionID))
	if err != nil {
		if errors.Is(err, cache.ErrKeyNotFound) {
			return ConnectionMeta{}, fmt.Errorf("%w: %s", ErrConnectionMissing, connectionID)
		}
		return ConnectionMeta{}, err
	}
	meta, err := parseConnectionMeta(raw)
	if err != nil {
		return ConnectionMeta{}, err
	}
	meta.ConnectionID = defaultString(meta.ConnectionID, connectionID)
	return meta, nil
}

func (s *RedisStore) ListByUser(ctx context.Context, userID string) ([]ConnectionMeta, error) {
	if err := s.validateReady(); err != nil {
		return nil, err
	}
	if userID == "" {
		return nil, fmt.Errorf("%w: empty userID", ErrNoLiveConnections)
	}

	userKey := UserConnectionsKey(userID)
	connectionIDs, err := s.redis.SMembers(ctx, userKey)
	if err != nil {
		return nil, fmt.Errorf("get user connection index: %w", err)
	}
	if len(connectionIDs) == 0 {
		return nil, fmt.Errorf("%w for user %s", ErrNoLiveConnections, userID)
	}

	// user set 只做索引，读取时必须逐个反查 conn key；缺失或串用户的索引立即清理。
	metas := make([]ConnectionMeta, 0, len(connectionIDs))
	stale := make([]string, 0)
	for _, connectionID := range connectionIDs {
		if strings.TrimSpace(connectionID) == "" {
			continue
		}
		meta, err := s.Get(ctx, connectionID)
		if err != nil {
			if errors.Is(err, ErrConnectionMissing) {
				stale = append(stale, connectionID)
				continue
			}
			return nil, fmt.Errorf("verify connection %s: %w", connectionID, err)
		}
		if meta.UserID != userID {
			stale = append(stale, connectionID)
			continue
		}
		metas = append(metas, meta)
	}

	if len(stale) > 0 {
		if _, err := s.redis.SRem(ctx, userKey, stringsToInterfaces(stale)...); err != nil {
			klog.CtxWarnf(ctx, "Failed to remove stale route indexes userID=%s stale=%v error=%v", userID, stale, err)
		} else {
			klog.CtxInfof(ctx, "Removed stale route indexes userID=%s count=%d", userID, len(stale))
		}
	}
	if len(metas) == 0 {
		return nil, fmt.Errorf("%w for user %s", ErrNoLiveConnections, userID)
	}

	return metas, nil
}

func (s *RedisStore) Cleanup(ctx context.Context, userID string, connectionID string) error {
	if err := s.validateReady(); err != nil {
		return err
	}
	if connectionID == "" {
		return nil
	}
	// cleanup 是幂等操作，只删除指定 connectionID，不能按 userID 批量清理。
	if userID != "" {
		if _, err := s.redis.SRem(ctx, UserConnectionsKey(userID), connectionID); err != nil {
			return fmt.Errorf("remove user route index: %w", err)
		}
	}
	if err := s.redis.Delete(ctx, ConnectionKey(connectionID)); err != nil {
		return fmt.Errorf("delete connection route: %w", err)
	}
	klog.CtxInfof(ctx, "Cleaned connection route userID=%s connectionID=%s", userID, connectionID)
	return nil
}

func (s *RedisStore) CleanupStale(ctx context.Context, userID string) (int, error) {
	if err := s.validateReady(); err != nil {
		return 0, err
	}
	userKey := UserConnectionsKey(userID)
	connectionIDs, err := s.redis.SMembers(ctx, userKey)
	if err != nil {
		return 0, err
	}
	// sweeper 只修复索引，不删除仍然有效的连接事实，避免误删并发新连接。
	stale := make([]string, 0)
	for _, connectionID := range connectionIDs {
		meta, err := s.Get(ctx, connectionID)
		if err != nil || meta.UserID != userID {
			stale = append(stale, connectionID)
		}
	}
	if len(stale) == 0 {
		return 0, nil
	}
	_, err = s.redis.SRem(ctx, userKey, stringsToInterfaces(stale)...)
	return len(stale), err
}

func (s *RedisStore) validateReady() error {
	if s == nil || s.redis == nil {
		return fmt.Errorf("redis route store is not available")
	}
	return nil
}

func (s *RedisStore) normalizeMeta(meta ConnectionMeta) ConnectionMeta {
	now := time.Now()
	meta.GatewayAddr = defaultString(meta.GatewayAddr, s.gatewayAddr)
	meta.GatewayID = defaultString(meta.GatewayID, s.gatewayID)
	return meta.Touch(now)
}

func (s *RedisStore) setConnection(ctx context.Context, meta ConnectionMeta) error {
	data, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("marshal connection route: %w", err)
	}
	return s.redis.Set(ctx, ConnectionKey(meta.ConnectionID), string(data), s.ttl)
}

func (s *RedisStore) indexConnection(ctx context.Context, meta ConnectionMeta) error {
	userKey := UserConnectionsKey(meta.UserID)
	if _, err := s.redis.SAdd(ctx, userKey, meta.ConnectionID); err != nil {
		return fmt.Errorf("set user route index: %w", err)
	}
	if err := s.redis.Expire(ctx, userKey, s.ttl*2); err != nil {
		klog.CtxWarnf(ctx, "Failed to refresh user route index ttl key=%s ttl=%s error=%v", userKey, s.ttl*2, err)
	}
	return nil
}

func parseConnectionMeta(raw string) (ConnectionMeta, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ConnectionMeta{}, ErrConnectionMissing
	}
	if !strings.HasPrefix(raw, "{") {
		return ConnectionMeta{}, fmt.Errorf("invalid connection route meta")
	}

	var meta ConnectionMeta
	if err := json.Unmarshal([]byte(raw), &meta); err != nil {
		return ConnectionMeta{}, fmt.Errorf("decode connection route meta: %w", err)
	}
	if meta.GatewayAddr == "" {
		return ConnectionMeta{}, ErrConnectionMissing
	}
	return meta, nil
}

func stringsToInterfaces(values []string) []interface{} {
	items := make([]interface{}, 0, len(values))
	for _, value := range values {
		items = append(items, value)
	}
	return items
}

func defaultString(value string, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}
