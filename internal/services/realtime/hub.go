package realtime

import (
	"log"
	"sync"

	"github.com/gofiber/websocket/v2"
	"github.com/google/uuid"
)

type Hub struct {
	mu    sync.RWMutex
	conns map[uuid.UUID]map[*websocket.Conn]struct{}
}

func NewHub() *Hub {
	return &Hub{conns: make(map[uuid.UUID]map[*websocket.Conn]struct{})}
}

func (h *Hub) Register(userID uuid.UUID, conn *websocket.Conn) {
	if h == nil || conn == nil || userID == uuid.Nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	set, ok := h.conns[userID]
	if !ok {
		set = make(map[*websocket.Conn]struct{})
		h.conns[userID] = set
	}
	set[conn] = struct{}{}
}

func (h *Hub) Unregister(userID uuid.UUID, conn *websocket.Conn) {
	if h == nil || conn == nil || userID == uuid.Nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	set, ok := h.conns[userID]
	if !ok {
		return
	}
	delete(set, conn)
	if len(set) == 0 {
		delete(h.conns, userID)
	}
}

func (h *Hub) SendToUser(userID uuid.UUID, event string, payload interface{}) {
	if h == nil || userID == uuid.Nil {
		return
	}
	h.mu.RLock()
	set := h.conns[userID]
	conns := make([]*websocket.Conn, 0, len(set))
	for c := range set {
		conns = append(conns, c)
	}
	h.mu.RUnlock()

	if len(conns) == 0 {
		return
	}
	msg := map[string]interface{}{"event": event, "data": payload}
	for _, c := range conns {
		if err := c.WriteJSON(msg); err != nil {
			log.Printf("[realtime] write failed for user=%s: %v", userID, err)
		}
	}
}
