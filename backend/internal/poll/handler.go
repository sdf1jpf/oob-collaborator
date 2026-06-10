package poll

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/oob-collaborator/backend/internal/config"
	"github.com/oob-collaborator/backend/internal/ratelimit"
	"github.com/oob-collaborator/backend/internal/security"
	"github.com/oob-collaborator/backend/internal/store"
)

type Handler struct {
	cfg      *config.Config
	store    *store.Store
	limiter  *ratelimit.IPLimiter
	lockout  *ratelimit.LockoutLimiter
}

func NewHandler(cfg *config.Config, st *store.Store) *Handler {
	return &Handler{
		cfg:     cfg,
		store:   st,
		limiter: ratelimit.NewIPLimiter(120, time.Minute),
		lockout: ratelimit.NewLockoutLimiter(10, 15*time.Minute, 15*time.Minute),
	}
}

func (h *Handler) Register(r gin.IRoutes) {
	r.GET("/bi/b", h.Poll)
	r.GET("/bi/health", h.Health)
}

func (h *Handler) Health(c *gin.Context) {
	if !h.authenticate(c) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Poll returns undelivered interactions in Collaborator-like JSON format.
// Auth: X-Collaborator-Token header or Authorization: Bearer <token>
func (h *Handler) Poll(c *gin.Context) {
	if !h.authenticate(c) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	interactions, err := h.store.FetchUndeliveredInteractions(c.Request.Context(), 100)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch interactions"})
		return
	}

	ids := make([]uuid.UUID, 0, len(interactions))
	response := make([]PollResponseItem, 0, len(interactions))

	for _, i := range interactions {
		ids = append(ids, i.ID)
		item := PollResponseItem{
			InteractionID:   i.ID.String(),
			InteractionType: protocolToType(i.Protocol),
			Protocol:        i.Protocol,
			SourceIP:        i.SourceIP,
			TimeStamp:       i.InteractedAt.UTC().Format(time.RFC3339Nano),
			Host:            buildHost(i.SubDomain, h.cfg.Domain),
			RawData:         json.RawMessage(i.RawData),
		}
		response = append(response, item)
	}

	if len(ids) > 0 {
		if err := h.store.MarkInteractionsDelivered(c.Request.Context(), ids); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to mark delivered"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"interactions": response,
	})
}

func (h *Handler) authenticate(c *gin.Context) bool {
	ip := clientIP(c)
	if h.lockout.IsLocked(ip) {
		return false
	}
	if !h.limiter.Allow(ip) {
		return false
	}

	token := c.GetHeader("X-Collaborator-Token")
	if token == "" {
		auth := c.GetHeader("Authorization")
		if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
			token = strings.TrimSpace(auth[7:])
		}
	}
	if token == "" || !security.ConstantTimeEqual(token, h.cfg.PollToken) {
		h.lockout.RecordFailure(ip)
		return false
	}
	h.lockout.RecordSuccess(ip)
	return true
}

func clientIP(c *gin.Context) string {
	if ip := c.GetHeader("X-Real-IP"); ip != "" {
		return ip
	}
	if ip := c.GetHeader("X-Forwarded-For"); ip != "" {
		if i := strings.Index(ip, ","); i >= 0 {
			return strings.TrimSpace(ip[:i])
		}
		return ip
	}
	return c.ClientIP()
}

type PollResponseItem struct {
	InteractionID   string          `json:"interactionId"`
	InteractionType string          `json:"interactionType"`
	Protocol        string          `json:"protocol"`
	SourceIP        string          `json:"sourceIp"`
	TimeStamp       string          `json:"timeStamp"`
	Host            string          `json:"host"`
	RawData         json.RawMessage `json:"rawData"`
}

func protocolToType(protocol string) string {
	switch protocol {
	case "DNS":
		return "dns"
	case "HTTP":
		return "http"
	case "SMTP":
		return "smtp"
	default:
		return strings.ToLower(protocol)
	}
}

func buildHost(subDomain, domain string) string {
	if subDomain == "" {
		return domain
	}
	return subDomain + "." + domain
}
