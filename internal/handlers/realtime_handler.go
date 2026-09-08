package handlers

import (
	"log"

	"github.com/ganiramadhan/ganipedia/backend/internal/services/realtime"
	jwtutil "github.com/ganiramadhan/ganipedia/backend/pkg/jwt"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"github.com/google/uuid"
)

// RealtimeHandler wires the /ws/notifications WebSocket route to the Hub.
type RealtimeHandler struct {
	hub *realtime.Hub
}

func NewRealtimeHandler(hub *realtime.Hub) *RealtimeHandler {
	return &RealtimeHandler{hub: hub}
}

func (h *RealtimeHandler) UpgradeAuth(c *fiber.Ctx) error {
	if !websocket.IsWebSocketUpgrade(c) {
		return fiber.ErrUpgradeRequired
	}
	token := c.Query("token")
	if token == "" {
		return fiber.NewError(fiber.StatusUnauthorized, "missing token")
	}
	claims, err := jwtutil.ValidateToken(token)
	if err != nil || claims.UserID == uuid.Nil {
		return fiber.NewError(fiber.StatusUnauthorized, "invalid token")
	}
	c.Locals("userID", claims.UserID)
	return c.Next()
}

func (h *RealtimeHandler) Handle(c *websocket.Conn) {
	userIDVal := c.Locals("userID")
	userID, ok := userIDVal.(uuid.UUID)
	if !ok || userID == uuid.Nil {
		_ = c.Close()
		return
	}

	h.hub.Register(userID, c)
	defer h.hub.Unregister(userID, c)

	for {
		if _, _, err := c.ReadMessage(); err != nil {
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				log.Printf("[realtime] connection error for user=%s: %v", userID, err)
			}
			return
		}
	}
}
