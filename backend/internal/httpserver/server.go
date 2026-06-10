package httpserver

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/oob-collaborator/backend/internal/config"
	"github.com/oob-collaborator/backend/internal/hostedfile"
	"github.com/oob-collaborator/backend/internal/interaction"
	"github.com/oob-collaborator/backend/internal/store"
	"github.com/oob-collaborator/backend/internal/token"
)

type Server struct {
	cfg    *config.Config
	store  *store.Store
	logger *interaction.Logger
	mux    http.Handler
}

func New(cfg *config.Config, st *store.Store, logger *interaction.Logger, mux http.Handler) *Server {
	return &Server{cfg: cfg, store: st, logger: logger, mux: mux}
}

func (s *Server) StartHTTP() error {
	addr := ":" + itoa(s.cfg.HTTPPort)
	srv := &http.Server{
		Addr:    addr,
		Handler: http.HandlerFunc(s.handleHTTP),
	}
	log.Printf("HTTP listening on %s", addr)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP error: %v", err)
		}
	}()
	return nil
}

func (s *Server) handleHTTP(w http.ResponseWriter, r *http.Request) {
	// ACME HTTP-01 fallback + redirect to HTTPS for dashboard/API
	if strings.HasPrefix(r.URL.Path, "/.well-known/acme-challenge/") {
		s.mux.ServeHTTP(w, r)
		return
	}

	s.trapInteraction(w, r)
}

func (s *Server) TrapHandler() http.HandlerFunc {
	return s.trapInteraction
}

func (s *Server) trapInteraction(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	_ = r.Body.Close()

	headers := map[string][]string{}
	for k, v := range r.Header {
		headers[k] = v
	}
	query := map[string][]string{}
	for k, v := range r.URL.Query() {
		query[k] = v
	}
	cookies := map[string]string{}
	for _, c := range r.Cookies() {
		cookies[c.Name] = c.Value
	}

	host := r.Host
	if h := r.Header.Get("Host"); h != "" {
		host = h
	}

	hostedFilePath := ""
	var hostedContent []byte
	var hostedContentType string

	if s.store != nil {
		subDomain := token.ExtractSubDomain(host, s.cfg.Domain)
		if subDomain != "" {
			if filePath, err := hostedfile.NormalizePath(r.URL.Path); err == nil {
				ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
				engagementID, err := s.store.GetEngagementIDBySubDomain(ctx, subDomain)
				cancel()
				if err == nil {
					ctx, cancel = context.WithTimeout(r.Context(), 2*time.Second)
					file, err := s.store.GetHostedFileByEngagementAndPath(ctx, engagementID, filePath)
					cancel()
					if err == nil {
						hostedFilePath = file.Path
						hostedContent = file.Content
						hostedContentType = file.ContentType
					}
				}
			}
		}
	}

	s.logger.LogHTTP(
		host,
		r.Method,
		r.URL.Path,
		r.URL.RequestURI(),
		interaction.ClientIP(r.RemoteAddr),
		headers,
		query,
		body,
		cookies,
		hostedFilePath,
	)

	if hostedFilePath != "" {
		w.Header().Set("Content-Type", hostedContentType)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(hostedContent)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}

func (s *Server) ServeDashboard(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/bi/") || strings.HasPrefix(r.URL.Path, "/ws") {
		s.mux.ServeHTTP(w, r)
		return
	}
	s.mux.ServeHTTP(w, r)
}

func FormatHTTPRaw(r *http.Request, body []byte) string {
	var buf bytes.Buffer
	buf.WriteString(r.Method + " " + r.URL.RequestURI() + " HTTP/1.1\r\n")
	for k, vals := range r.Header {
		for _, v := range vals {
			buf.WriteString(k + ": " + v + "\r\n")
		}
	}
	buf.WriteString("\r\n")
	buf.Write(body)
	return buf.String()
}

func itoa(n int) string {
	return strconv.Itoa(n)
}
