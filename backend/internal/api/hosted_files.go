package api

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/oob-collaborator/backend/internal/hostedfile"
	"github.com/oob-collaborator/backend/internal/store"
	"github.com/oob-collaborator/backend/internal/token"
)

type hostedFileResponse struct {
	ID           uuid.UUID `json:"id"`
	EngagementID uuid.UUID `json:"engagement_id"`
	Path         string    `json:"path"`
	ContentType  string    `json:"content_type"`
	Size         int       `json:"size"`
	CreatedAt    string    `json:"created_at"`
	ExampleURL   string    `json:"example_url"`
}

func (h *Handler) RegisterHostedFiles(r *gin.RouterGroup) {
	r.GET("/engagements/:id/files", h.ListHostedFiles)
	r.POST("/engagements/:id/files", h.UploadHostedFile)
	r.DELETE("/files/:id", h.DeleteHostedFile)
}

func (h *Handler) ListHostedFiles(c *gin.Context) {
	engagementID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid engagement id"})
		return
	}
	if _, err := h.store.GetEngagement(c.Request.Context(), engagementID); err != nil {
		if store.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "engagement not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get engagement"})
		return
	}

	items, err := h.store.ListHostedFilesByEngagement(c.Request.Context(), engagementID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list hosted files"})
		return
	}
	if items == nil {
		items = []store.HostedFile{}
	}

	exampleSub := h.examplePayloadSubDomain(c, engagementID)
	out := make([]hostedFileResponse, 0, len(items))
	for _, f := range items {
		out = append(out, h.toHostedFileResponse(f, exampleSub))
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) UploadHostedFile(c *gin.Context) {
	engagementID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid engagement id"})
		return
	}
	if _, err := h.store.GetEngagement(c.Request.Context(), engagementID); err != nil {
		if store.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "engagement not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get engagement"})
		return
	}

	content, filePath, contentType, err := h.readHostedFileUpload(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(content) > h.cfg.HostedFileMaxBytes {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("file exceeds maximum size of %d bytes", h.cfg.HostedFileMaxBytes)})
		return
	}

	normalizedPath, err := hostedfile.NormalizePath(filePath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid path"})
		return
	}

	if contentType == "" {
		contentType = hostedfile.ContentTypeFromPath(normalizedPath)
	}

	f, err := h.store.CreateHostedFile(c.Request.Context(), engagementID, normalizedPath, contentType, content)
	if err != nil {
		if store.IsUniqueViolation(err) {
			c.JSON(http.StatusConflict, gin.H{"error": "a file already exists at this path"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create hosted file"})
		return
	}

	exampleSub := h.examplePayloadSubDomain(c, engagementID)
	c.JSON(http.StatusCreated, h.toHostedFileResponse(*f, exampleSub))
}

func (h *Handler) DeleteHostedFile(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file id"})
		return
	}
	if err := h.store.DeleteHostedFile(c.Request.Context(), id); err != nil {
		if store.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete hosted file"})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) readHostedFileUpload(c *gin.Context) (content []byte, filePath, contentType string, err error) {
	contentType = strings.TrimSpace(c.PostForm("content_type"))

	if upload, headerErr := c.FormFile("file"); headerErr == nil && upload != nil {
		f, openErr := upload.Open()
		if openErr != nil {
			return nil, "", "", fmt.Errorf("failed to open uploaded file")
		}
		defer f.Close()
		limited := io.LimitReader(f, int64(h.cfg.HostedFileMaxBytes)+1)
		content, err = io.ReadAll(limited)
		if err != nil {
			return nil, "", "", fmt.Errorf("failed to read uploaded file")
		}
		filePath = strings.TrimSpace(c.PostForm("path"))
		if filePath == "" {
			filePath = filepath.Base(upload.Filename)
		}
		return content, filePath, contentType, nil
	}

	content = []byte(c.PostForm("content"))
	if len(content) == 0 {
		return nil, "", "", fmt.Errorf("file or content is required")
	}
	filePath = strings.TrimSpace(c.PostForm("path"))
	if filePath == "" {
		return nil, "", "", fmt.Errorf("path is required when uploading content text")
	}
	return content, filePath, contentType, nil
}

func (h *Handler) examplePayloadSubDomain(c *gin.Context, engagementID uuid.UUID) string {
	payloads, err := h.store.ListPayloadsByEngagement(c.Request.Context(), engagementID)
	if err != nil || len(payloads) == 0 {
		return "{token}"
	}
	return payloads[0].SubDomain
}

func (h *Handler) toHostedFileResponse(f store.HostedFile, exampleSub string) hostedFileResponse {
	host := token.FullHost(exampleSub, h.cfg.Domain)
	scheme := "https"
	if h.cfg.DevMode && (h.cfg.Domain == "localhost" || h.cfg.Domain == "127.0.0.1") {
		scheme = "https"
	}
	return hostedFileResponse{
		ID:           f.ID,
		EngagementID: f.EngagementID,
		Path:         f.Path,
		ContentType:  f.ContentType,
		Size:         f.Size,
		CreatedAt:    f.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		ExampleURL:   fmt.Sprintf("%s://%s%s", scheme, host, f.Path),
	}
}
