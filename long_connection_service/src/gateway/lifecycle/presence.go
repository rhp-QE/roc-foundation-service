package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/rhp-QE/roc-foundation-service/long_connection_service/src/route"
)

type PresenceLifecycle struct {
	store   route.Store
	timeout time.Duration
}

// PresenceLifecycle 只编排连接生命周期，不直接管理 Hub 内存连接。
// 所有 Redis 操作都带短超时，避免读写 pump 被路由缓存阻塞。
func NewPresenceLifecycle(store route.Store, timeout time.Duration) *PresenceLifecycle {
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	return &PresenceLifecycle{
		store:   store,
		timeout: timeout,
	}
}

func (p *PresenceLifecycle) Register(ctx context.Context, meta route.ConnectionMeta) error {
	// 注册是 connectionID 级别的事实写入，同一用户的其他连接不受影响。
	return p.withTimeout(ctx, func(ctx context.Context) error {
		if p == nil || p.store == nil {
			return nil
		}
		if err := p.store.Register(ctx, meta); err != nil {
			return fmt.Errorf("register presence: %w", err)
		}
		return nil
	})
}

func (p *PresenceLifecycle) Refresh(ctx context.Context, userID string, connectionID string) error {
	// 续期只允许已有连接事实刷新 TTL，缺失的 route 不在心跳路径补造。
	return p.withTimeout(ctx, func(ctx context.Context) error {
		if p == nil || p.store == nil || connectionID == "" {
			return nil
		}
		if err := p.store.Refresh(ctx, userID, connectionID); err != nil {
			if errors.Is(err, route.ErrConnectionMissing) {
				klog.CtxDebugf(ctx, "Skip refresh for missing connection route userID=%s connectionID=%s", userID, connectionID)
				return nil
			}
			return fmt.Errorf("refresh presence: %w", err)
		}
		return nil
	})
}

func (p *PresenceLifecycle) Unregister(ctx context.Context, userID string, connectionID string) error {
	// 注销必须幂等，延迟 close 只能删除自己的 connectionID。
	return p.withTimeout(ctx, func(ctx context.Context) error {
		if p == nil || p.store == nil || connectionID == "" {
			return nil
		}
		if err := p.store.Unregister(ctx, userID, connectionID); err != nil {
			return fmt.Errorf("unregister presence: %w", err)
		}
		return nil
	})
}

func (p *PresenceLifecycle) CleanupStale(ctx context.Context, userID string) error {
	return p.withTimeout(ctx, func(ctx context.Context) error {
		if p == nil || p.store == nil || userID == "" {
			return nil
		}
		_, err := p.store.CleanupStale(ctx, userID)
		return err
	})
}

func (p *PresenceLifecycle) withTimeout(ctx context.Context, fn func(context.Context) error) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if p == nil || p.timeout <= 0 {
		return fn(ctx)
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()
	return fn(timeoutCtx)
}
