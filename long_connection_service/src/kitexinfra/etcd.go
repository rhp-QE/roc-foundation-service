package kitexinfra

import (
	"github.com/cloudwego/kitex/pkg/discovery"
	"github.com/cloudwego/kitex/pkg/registry"
	etcd "github.com/kitex-contrib/registry-etcd"
)

// NewEtcdRegistry converts runtime etcd endpoints to Kitex's registry
// extension. Service registration itself is owned by Kitex server options.
func NewEtcdRegistry(endpoints []string) (registry.Registry, error) {
	return etcd.NewEtcdRegistry(normalizeEndpoints(endpoints))
}

// NewEtcdResolver converts runtime etcd endpoints to Kitex's resolver
// extension. Service discovery and load balancing stay inside Kitex client.
func NewEtcdResolver(endpoints []string) (discovery.Resolver, error) {
	return etcd.NewEtcdResolver(normalizeEndpoints(endpoints))
}

func normalizeEndpoints(endpoints []string) []string {
	filtered := make([]string, 0, len(endpoints))
	for _, endpoint := range endpoints {
		if endpoint != "" {
			filtered = append(filtered, endpoint)
		}
	}
	if len(filtered) == 0 {
		return []string{"localhost:2379"}
	}
	return filtered
}
