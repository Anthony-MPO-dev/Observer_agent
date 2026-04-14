package gateway

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"logstream/server/auth"
	"logstream/server/hub"
)

const (
	pingInterval  = 30 * time.Second
	writeDeadline = 10 * time.Second
)

var upgrader = websocket.Upgrader{
	// Allow all origins — the dashboard may be served from a different origin.
	CheckOrigin: func(r *http.Request) bool { return true },
	ReadBufferSize:  1024,
	WriteBufferSize: 4096,
}

// Handler returns an http.HandlerFunc that upgrades to WebSocket and streams
// live log entries matching the query-param filters.
//
// URL format:
//
//	GET /ws/logs?token=JWT&service_ids=a,b&levels=INFO,ERROR&task_id=X&documento=Y&module=Z&search=Q
func Handler(h *hub.Hub, jwtSecret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Validate JWT from query param.
		tokenStr := r.URL.Query().Get("token")
		if tokenStr == "" {
			http.Error(w, `{"error":"missing token"}`, http.StatusUnauthorized)
			return
		}
		if _, err := auth.ValidateToken(tokenStr, jwtSecret); err != nil {
			http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
			return
		}

		// 2. Parse filters from query params.
		filter := hub.Filter{
			ServiceIDs: splitParam(r.URL.Query().Get("service_ids")),
			Levels:     splitParam(r.URL.Query().Get("levels")),
			TaskID:     r.URL.Query().Get("task_id"),
			Documento:  r.URL.Query().Get("documento"),
			Module:     r.URL.Query().Get("module"),
			Search:     r.URL.Query().Get("search"),
		}

		// 3. Upgrade connection.
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("ws: upgrade: %v", err)
			return
		}
		defer conn.Close()

		// 4. Subscribe to the hub.
		sub := h.Subscribe(filter)
		defer h.Unsubscribe(sub.ID)

		log.Printf("ws: subscriber %s connected (services=%v levels=%v)", sub.ID, filter.ServiceIDs, filter.Levels)

		// Ping ticker to keep the connection alive.
		ticker := time.NewTicker(pingInterval)
		defer ticker.Stop()

		// 5. Read pump: detect client close without blocking the write pump.
		closeCh := make(chan struct{})
		go func() {
			defer close(closeCh)
			for {
				if _, _, err := conn.ReadMessage(); err != nil {
					return
				}
			}
		}()

		for {
			select {
			case <-closeCh:
				log.Printf("ws: subscriber %s disconnected", sub.ID)
				return

			case entry, ok := <-sub.Ch:
				if !ok {
					return
				}
				data, err := json.Marshal(entry)
				if err != nil {
					log.Printf("ws: marshal entry: %v", err)
					continue
				}
				_ = conn.SetWriteDeadline(time.Now().Add(writeDeadline))
				if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
					log.Printf("ws: write: %v", err)
					return
				}

			case event, ok := <-sub.EventCh:
				if !ok {
					return
				}
				eventJSON, err := json.Marshal(event)
				if err != nil {
					continue
				}
				_ = conn.SetWriteDeadline(time.Now().Add(writeDeadline))
				if err := conn.WriteMessage(websocket.TextMessage, eventJSON); err != nil {
					log.Printf("ws: write event: %v", err)
					return
				}

			case <-ticker.C:
				_ = conn.SetWriteDeadline(time.Now().Add(writeDeadline))
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					log.Printf("ws: ping: %v", err)
					return
				}
			}
		}
	}
}

// splitParam splits a comma-separated query param value into a slice,
// trimming whitespace and ignoring empty strings.
func splitParam(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var out []string
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
