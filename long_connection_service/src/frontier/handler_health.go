package frontier

import (
	"encoding/json"
	"net/http"
	"time"
)

// HealthHandler 健康检查处理器
type HealthHandler struct {
	hub *Hub
}

// NewHealthHandler 创建健康检查处理器
func NewHealthHandler(hub *Hub) *HealthHandler {
	return &HealthHandler{hub: hub}
}

// ServeHTTP 健康检查
func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status":             "ok",
		"active_connections": h.hub.GetActiveConnectionCount(),
		"timestamp":          time.Now().Unix(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

