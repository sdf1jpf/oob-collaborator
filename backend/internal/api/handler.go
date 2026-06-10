package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/oob-collaborator/backend/internal/config"
	"github.com/oob-collaborator/backend/internal/store"
	"github.com/oob-collaborator/backend/internal/token"
)

type Handler struct {
	cfg   *config.Config
	store *store.Store
}

func NewHandler(cfg *config.Config, st *store.Store) *Handler {
	return &Handler{cfg: cfg, store: st}
}

func (h *Handler) Register(r *gin.RouterGroup) {
	r.GET("/engagements", h.ListEngagements)
	r.POST("/engagements", h.CreateEngagement)
	r.GET("/engagements/:id/interactions", h.ListInteractions)
	r.POST("/payloads/generate", h.GeneratePayload)
	r.GET("/engagements/:id/payloads", h.ListPayloads)
	h.RegisterHostedFiles(r)
}

type createEngagementRequest struct {
	Name       string `json:"name" binding:"required"`
	ClientName string `json:"client_name" binding:"required"`
}

type generatePayloadRequest struct {
	EngagementID uuid.UUID `json:"engagement_id" binding:"required"`
	Description  string    `json:"description"`
}

type payloadResponse struct {
	store.Payload
	FullDomain string `json:"full_domain"`
}

func (h *Handler) ListEngagements(c *gin.Context) {
	items, err := h.store.ListEngagements(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list engagements"})
		return
	}
	if items == nil {
		items = []store.Engagement{}
	}
	c.JSON(http.StatusOK, items)
}

func (h *Handler) CreateEngagement(c *gin.Context) {
	var req createEngagementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	e, err := h.store.CreateEngagement(c.Request.Context(), req.Name, req.ClientName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create engagement"})
		return
	}
	c.JSON(http.StatusCreated, e)
}

func (h *Handler) ListInteractions(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid engagement id"})
		return
	}
	if _, err := h.store.GetEngagement(c.Request.Context(), id); err != nil {
		if store.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "engagement not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get engagement"})
		return
	}
	items, err := h.store.ListInteractionsByEngagement(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list interactions"})
		return
	}
	if items == nil {
		items = []store.Interaction{}
	}

	enriched := make([]gin.H, 0, len(items))
	for _, i := range items {
		host := token.FullHost(i.SubDomain, h.cfg.Domain)
		reverseDNS := ""
		var ipRecon gin.H
		if i.IPRecon != nil {
			reverseDNS = i.IPRecon.ReverseDNS
			ipRecon = gin.H{
				"ip":           i.IPRecon.IP,
				"reverse_dns":  i.IPRecon.ReverseDNS,
				"country":      i.IPRecon.Country,
				"country_code": i.IPRecon.CountryCode,
				"region":       i.IPRecon.Region,
				"city":         i.IPRecon.City,
				"lat":          i.IPRecon.Lat,
				"lon":          i.IPRecon.Lon,
				"isp":          i.IPRecon.ISP,
				"org":          i.IPRecon.Org,
				"asn":          i.IPRecon.ASN,
				"status":       i.IPRecon.Status,
				"updated_at":   i.IPRecon.UpdatedAt,
			}
		}
		item := gin.H{
			"id":            i.ID,
			"payload_id":    i.PayloadID,
			"engagement_id": i.EngagementID,
			"sub_domain":    i.SubDomain,
			"protocol":      i.Protocol,
			"source_ip":     i.SourceIP,
			"reverse_dns":   reverseDNS,
			"host":          host,
			"raw_data":      i.RawData,
			"interacted_at": i.InteractedAt,
			"delivered_at":  i.DeliveredAt,
		}
		if ipRecon != nil {
			item["ip_recon"] = ipRecon
		}
		enriched = append(enriched, item)
	}
	c.JSON(http.StatusOK, enriched)
}

func (h *Handler) GeneratePayload(c *gin.Context) {
	var req generatePayloadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if _, err := h.store.GetEngagement(c.Request.Context(), req.EngagementID); err != nil {
		if store.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "engagement not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get engagement"})
		return
	}
	p, err := h.store.GeneratePayload(c.Request.Context(), req.EngagementID, req.Description)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate payload"})
		return
	}
	c.JSON(http.StatusCreated, payloadResponse{
		Payload:    *p,
		FullDomain: token.FullHost(p.SubDomain, h.cfg.Domain),
	})
}

func (h *Handler) ListPayloads(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid engagement id"})
		return
	}
	items, err := h.store.ListPayloadsByEngagement(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list payloads"})
		return
	}
	if items == nil {
		items = []store.Payload{}
	}
	out := make([]payloadResponse, 0, len(items))
	for _, p := range items {
		out = append(out, payloadResponse{
			Payload:    p,
			FullDomain: token.FullHost(p.SubDomain, h.cfg.Domain),
		})
	}
	c.JSON(http.StatusOK, out)
}
