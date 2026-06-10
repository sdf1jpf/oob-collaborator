package interaction

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"time"

	"github.com/google/uuid"
	"github.com/oob-collaborator/backend/internal/config"
	"github.com/oob-collaborator/backend/internal/ratelimit"
	"github.com/oob-collaborator/backend/internal/recon"
	"github.com/oob-collaborator/backend/internal/store"
	"github.com/oob-collaborator/backend/internal/token"
	"github.com/oob-collaborator/backend/internal/ws"
)

type Logger struct {
	cfg      *config.Config
	store    *store.Store
	hub      *ws.Hub
	enricher *recon.Enricher
	limiter  *ratelimit.IPLimiter
}

func NewLogger(cfg *config.Config, st *store.Store, hub *ws.Hub, enricher *recon.Enricher, limiter *ratelimit.IPLimiter) *Logger {
	return &Logger{cfg: cfg, store: st, hub: hub, enricher: enricher, limiter: limiter}
}

func (l *Logger) AllowSourceIP(sourceIP string) bool {
	if l.limiter == nil {
		return true
	}
	return l.limiter.Allow(sourceIP)
}

func (l *Logger) LogHTTP(host, method, path, requestURI, sourceIP string, headers map[string][]string, query map[string][]string, body []byte, cookies map[string]string) {
	if !l.AllowSourceIP(sourceIP) {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	subDomain := token.ExtractSubDomain(host, l.cfg.Domain)
	var payloadID *uuid.UUID
	if subDomain != "" {
		if p, err := l.store.GetPayloadBySubDomain(ctx, subDomain); err == nil {
			payloadID = &p.ID
		} else if !l.cfg.LogUnknownTokens {
			return
		}
	} else if !l.cfg.LogUnknownTokens {
		return
	}

	raw, _ := json.Marshal(map[string]any{
		"host":        host,
		"method":      method,
		"path":        path,
		"request_uri": requestURI,
		"headers":     SanitizeHeaders(headers),
		"query":       query,
		"cookies":     SanitizeCookies(cookies),
		"body":        string(body),
	})

	interaction, err := l.store.CreateInteraction(ctx, payloadID, "HTTP", sourceIP, string(raw))
	if err != nil {
		log.Printf("http interaction log: %v", err)
		return
	}
	l.enrichAndBroadcast(interaction, subDomain, payloadID, sourceIP)
}

func (l *Logger) LogSMTP(host, sourceIP, mailFrom string, rcptTo []string, rawMessage []byte) {
	if !l.AllowSourceIP(sourceIP) {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	subDomain := token.ExtractSubDomain(host, l.cfg.Domain)
	var payloadID *uuid.UUID
	if subDomain != "" {
		if p, err := l.store.GetPayloadBySubDomain(ctx, subDomain); err == nil {
			payloadID = &p.ID
		} else if !l.cfg.LogUnknownTokens {
			return
		}
	} else if !l.cfg.LogUnknownTokens {
		return
	}

	raw, _ := json.Marshal(map[string]any{
		"host":      host,
		"mail_from": mailFrom,
		"rcpt_to":   rcptTo,
		"message":   string(rawMessage),
	})

	interaction, err := l.store.CreateInteraction(ctx, payloadID, "SMTP", sourceIP, string(raw))
	if err != nil {
		log.Printf("smtp interaction log: %v", err)
		return
	}
	l.enrichAndBroadcast(interaction, subDomain, payloadID, sourceIP)
}

func (l *Logger) enrichAndBroadcast(interaction *store.Interaction, subDomain string, payloadID *uuid.UUID, sourceIP string) {
	if subDomain != "" {
		interaction.SubDomain = subDomain
	}
	if payloadID != nil && subDomain != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if p, err := l.store.GetPayloadBySubDomain(ctx, subDomain); err == nil {
			interaction.EngagementID = &p.EngagementID
		}
	}
	if l.enricher != nil {
		l.enricher.Enqueue(sourceIP)
	}
	if interaction.EngagementID != nil {
		l.hub.BroadcastInteraction(interaction)
	}
}

func ClientIP(remoteAddr string) string {
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		return host
	}
	return remoteAddr
}
