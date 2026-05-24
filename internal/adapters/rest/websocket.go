package rest

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type StatusUpdate struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	Channel   string `json:"channel"`
	UpdatedAt string `json:"updated_at"`
}

type WSHub struct {
	mu          sync.RWMutex
	subscribers map[string]map[*websocket.Conn]struct{}
	logger      *slog.Logger
}

func NewWSHub(logger *slog.Logger) *WSHub {
	return &WSHub{
		subscribers: make(map[string]map[*websocket.Conn]struct{}),
		logger:      logger,
	}
}

func (h *WSHub) Subscribe(notificationID string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.subscribers[notificationID] == nil {
		h.subscribers[notificationID] = make(map[*websocket.Conn]struct{})
	}
	h.subscribers[notificationID][conn] = struct{}{}
}

func (h *WSHub) Unsubscribe(notificationID string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if conns, ok := h.subscribers[notificationID]; ok {
		delete(conns, conn)
		if len(conns) == 0 {
			delete(h.subscribers, notificationID)
		}
	}
}

func (h *WSHub) Broadcast(update StatusUpdate) {
	h.mu.RLock()
	conns, ok := h.subscribers[update.ID]
	h.mu.RUnlock()

	if !ok {
		return
	}

	data, _ := json.Marshal(update)

	h.mu.RLock()
	defer h.mu.RUnlock()
	for conn := range conns {
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			h.logger.Warn("ws write failed", slog.Any("error", err))
			conn.Close()
			delete(conns, conn)
		}
	}
}

func (h *WSHub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	notificationID := chi.URLParam(r, "id")
	if notificationID == "" {
		http.Error(w, "missing notification id", http.StatusBadRequest)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("ws upgrade failed", slog.Any("error", err))
		return
	}

	h.Subscribe(notificationID, conn)
	h.logger.Info("ws client connected", slog.String("notification_id", notificationID))

	go func() {
		defer func() {
			h.Unsubscribe(notificationID, conn)
			conn.Close()
		}()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				break
			}
		}
	}()
}

func (h *WSHub) ListenPostgres(ctx context.Context, pool *pgxpool.Pool) {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		h.logger.Error("acquire pg conn for listen", slog.Any("error", err))
		return
	}
	defer conn.Release()

	_, err = conn.Exec(ctx, "LISTEN notification_status")
	if err != nil {
		h.logger.Error("pg listen", slog.Any("error", err))
		return
	}

	h.logger.Info("listening for PostgreSQL notification_status events")

	for {
		notification, err := conn.Conn().WaitForNotification(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			h.logger.Error("wait for notification", slog.Any("error", err))
			return
		}

		var update StatusUpdate
		if err := json.Unmarshal([]byte(notification.Payload), &update); err != nil {
			h.logger.Error("unmarshal pg notification", slog.Any("error", err))
			continue
		}

		h.Broadcast(update)
	}
}
