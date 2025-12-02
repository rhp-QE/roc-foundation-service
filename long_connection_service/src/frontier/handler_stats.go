package frontier

import (
	"encoding/json"
	"net/http"
	"time"
)

// StatsHandler 统计信息处理器
type StatsHandler struct {
	hub *Hub
}

// NewStatsHandler 创建统计处理器
func NewStatsHandler(hub *Hub) *StatsHandler {
	return &StatsHandler{hub: hub}
}

// ServeHTTP 返回统计信息
func (h *StatsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	stats := h.hub.GetStats()

	response := map[string]interface{}{
		"total_connections":  stats.TotalConnections,
		"active_connections": stats.ActiveConnections,
		"messages_received":  stats.MessagesReceived,
		"messages_sent":      stats.MessagesSent,
		"timestamp":          time.Now().Unix(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

