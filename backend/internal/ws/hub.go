package ws

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/oob-collaborator/backend/internal/config"
	"github.com/oob-collaborator/backend/internal/security"
	"github.com/oob-collaborator/backend/internal/store"
)

type Hub struct {
	cfg     *config.Config
	validate func(token string) error
	mu      sync.RWMutex
	clients map[*client]struct{}
}

type client struct {
	conn          *websocket.Conn
	engagementID  uuid.UUID
}

func NewHub(cfg *config.Config, validate func(token string) error) *Hub {
	return &Hub{
		cfg:      cfg,
		validate: validate,
		clients:  make(map[*client]struct{}),
	}
}

func (h *Hub) HandleWS(w http.ResponseWriter, r *http.Request) {
	if err := h.authenticateRequest(r); err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	engagementID, err := uuid.Parse(r.URL.Query().Get("engagement"))
	if err != nil {
		http.Error(w, "engagement query parameter required", http.StatusBadRequest)
		return
	}

	upgrader := websocket.Upgrader{
		CheckOrigin: h.checkOrigin,
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade: %v", err)
		return
	}

	c := &client{conn: conn, engagementID: engagementID}
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		delete(h.clients, c)
		h.mu.Unlock()
		conn.Close()
	}()

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
}

func (h *Hub) authenticateRequest(r *http.Request) error {
	if cookie, err := r.Cookie(security.SessionCookieName); err == nil && cookie.Value != "" {
		return h.validate(cookie.Value)
	}
	if token := r.URL.Query().Get("token"); token != "" {
		return h.validate(token)
	}
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return h.validate(strings.TrimSpace(auth[7:]))
	}
	return errUnauthorized
}

var errUnauthorized = &authError{msg: "missing or invalid token"}

type authError struct{ msg string }

func (e *authError) Error() string { return e.msg }

func (h *Hub) checkOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}

	host := strings.ToLower(r.Host)
	if i := strings.Index(host, ":"); i >= 0 {
		host = host[:i]
	}

	domain := strings.ToLower(h.cfg.Domain)
	allowed := []string{
		"https://" + domain,
		"http://" + domain,
		"https://localhost",
		"http://localhost",
		"https://127.0.0.1",
		"http://127.0.0.1",
	}
	for _, a := range allowed {
		if strings.EqualFold(origin, a) {
			return true
		}
	}
	return false
}

func (h *Hub) broadcast(payload []byte, match func(*client) bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		if match != nil && !match(c) {
			continue
		}
		if err := c.conn.WriteMessage(websocket.TextMessage, payload); err != nil {
			log.Printf("websocket write: %v", err)
		}
	}
}

func (h *Hub) BroadcastInteraction(i *store.Interaction) {
	if i == nil || i.EngagementID == nil {
		return
	}
	engagementID := *i.EngagementID
	payload, err := json.Marshal(map[string]any{
		"type":        "interaction",
		"interaction": i,
	})
	if err != nil {
		return
	}
	h.broadcast(payload, func(c *client) bool {
		return c.engagementID == engagementID
	})
}

func (h *Hub) BroadcastIPRecon(r *store.IPRecon) {
	if h == nil || r == nil {
		return
	}
	payload, err := json.Marshal(map[string]any{
		"type":     "ip_recon",
		"ip_recon": r,
	})
	if err != nil {
		return
	}
	h.broadcast(payload, nil)
}
