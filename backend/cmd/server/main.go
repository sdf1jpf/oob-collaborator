package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/oob-collaborator/backend/internal/api"
	"github.com/oob-collaborator/backend/internal/config"
	dnsengine "github.com/oob-collaborator/backend/internal/dns"
	"github.com/oob-collaborator/backend/internal/httpserver"
	"github.com/oob-collaborator/backend/internal/interaction"
	"github.com/oob-collaborator/backend/internal/poll"
	"github.com/oob-collaborator/backend/internal/ratelimit"
	"github.com/oob-collaborator/backend/internal/recon"
	smtpserver "github.com/oob-collaborator/backend/internal/smtp"
	"github.com/oob-collaborator/backend/internal/store"
	"github.com/oob-collaborator/backend/internal/tlsmanager"
	"github.com/oob-collaborator/backend/internal/web"
	"github.com/oob-collaborator/backend/internal/ws"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx := context.Background()
	st, err := store.New(ctx, cfg.DatabaseURL, cfg.PayloadTokenLength)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer st.Close()

	if err := st.WaitForDB(ctx, 60*time.Second); err != nil {
		log.Fatalf("database wait: %v", err)
	}

	ingestionLimiter := ratelimit.NewIPLimiter(120, time.Minute)
	hub := ws.NewHub(cfg, func(token string) error {
		return api.ValidateToken(token, cfg)
	})
	enricher := recon.New(cfg, st, hub)
	enricher.Start()
	defer enricher.Stop()

	logger := interaction.NewLogger(cfg, st, hub, enricher, ingestionLimiter)
	acmeProvider := dnsengine.NewACMEProvider()

	dnsSrv := dnsengine.NewServer(cfg, st, hub, enricher, acmeProvider, ingestionLimiter)
	if err := dnsSrv.Start(); err != nil {
		log.Fatalf("dns: %v", err)
	}
	defer dnsSrv.Shutdown()

	smtpSrv := smtpserver.New(cfg, logger)
	if err := smtpSrv.Start(); err != nil {
		log.Fatalf("smtp: %v", err)
	}

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery(), gin.Logger())

	authHandler := api.NewAuthHandler(cfg)
	apiHandler := api.NewHandler(cfg, st)
	pollHandler := poll.NewHandler(cfg, st)

	router.POST("/api/login", authHandler.Login)
	router.POST("/api/logout", authHandler.Logout)

	protected := router.Group("/api")
	protected.Use(api.JWTAuth(cfg))
	protected.GET("/me", authHandler.Me)
	apiHandler.Register(protected)

	router.GET("/ws", func(c *gin.Context) {
		hub.HandleWS(c.Writer, c.Request)
	})

	pollHandler.Register(router)

	static := web.StaticHandler(cfg.WebDistPath)
	httpTrap := httpserver.New(cfg, st, logger, router)

	// HTTP :80 — trap OOB interactions
	go func() {
		addr := ":" + strconv.Itoa(cfg.HTTPPort)
		srv := &http.Server{
			Addr: addr,
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if hasPrefix(r.URL.Path, "/.well-known/") {
					router.ServeHTTP(w, r)
					return
				}
				httpTrap.TrapHandler()(w, r)
			}),
		}
		log.Printf("HTTP listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP error: %v", err)
		}
	}()

	// HTTPS / dev server — API, dashboard, poll, trap
	httpsHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hasPrefix(r.URL.Path, "/api/") || hasPrefix(r.URL.Path, "/bi/") || hasPrefix(r.URL.Path, "/ws") {
			router.ServeHTTP(w, r)
			return
		}
		if shouldServeSPA(r, cfg.Domain) {
			static.ServeHTTP(w, r)
			return
		}
		httpTrap.TrapHandler()(w, r)
	})

	skipTLS := os.Getenv("SKIP_TLS") == "true" || cfg.Domain == "localhost"
	if skipTLS {
		addr := ":" + strconv.Itoa(cfg.HTTPSPort)
		go func() {
			log.Printf("Dev mode: serving API+dashboard on %s (SKIP_TLS)", addr)
			if err := tlsmanager.DevListenAndServe(addr, httpsHandler); err != nil && err != http.ErrServerClosed {
				log.Printf("dev server error: %v", err)
			}
		}()
	} else {
		tlsMgr, err := tlsmanager.New(cfg, acmeProvider)
		if err != nil {
			log.Fatalf("tls: %v", err)
		}
		certCtx, certCancel := context.WithTimeout(ctx, 8*time.Minute)
		if err := tlsMgr.ObtainCertificate(certCtx); err != nil {
			log.Printf("TLS cert obtain warning: %v (continuing)", err)
		}
		certCancel()

		addr := tlsmanager.HTTPSAddr(cfg.HTTPSPort)
		go func() {
			log.Printf("HTTPS listening on %s", addr)
			if err := tlsMgr.ListenAndServe(addr, httpsHandler); err != nil && err != http.ErrServerClosed {
				log.Printf("HTTPS error: %v", err)
			}
		}()
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("shutting down...")
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func shouldServeSPA(r *http.Request, domain string) bool {
	host := r.Host
	if i := strings.Index(host, ":"); i >= 0 {
		host = host[:i]
	}
	host = strings.ToLower(host)
	domain = strings.ToLower(domain)

	// Payload subdomain requests are always trapped, even for paths like /r/test.js.
	if host != domain && host != "localhost" && host != "127.0.0.1" {
		return false
	}

	path := r.URL.Path
	if hasPrefix(path, "/dashboard") || hasPrefix(path, "/login") {
		return true
	}
	if isStaticAsset(path) {
		return true
	}
	return true
}

func isStaticAsset(path string) bool {
	if path == "/" || path == "/index.html" {
		return true
	}
	exts := []string{".js", ".css", ".svg", ".png", ".ico", ".woff", ".woff2", ".map"}
	for _, ext := range exts {
		if len(path) > len(ext) && path[len(path)-len(ext):] == ext {
			return true
		}
	}
	return hasPrefix(path, "/assets/")
}
