package tlsmanager

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/caddyserver/certmagic"
	"github.com/mholt/acmez/v3"
	"github.com/mholt/acmez/v3/acme"
	"github.com/oob-collaborator/backend/internal/config"
	dnsengine "github.com/oob-collaborator/backend/internal/dns"
)

type Manager struct {
	cfg   *config.Config
	magic *certmagic.Config
}

func New(cfg *config.Config, acmeProvider *dnsengine.ACMEProvider) (*Manager, error) {
	magic := certmagic.NewDefault()

	ca := certmagic.LetsEncryptProductionCA
	if cfg.ACMEStaging {
		ca = certmagic.LetsEncryptStagingCA
	}

	issuer := certmagic.NewACMEIssuer(magic, certmagic.ACMEIssuer{
		Agreed:                  true,
		Email:                   cfg.ACMEmail,
		CA:                      ca,
		DisableHTTPChallenge:    true,
		DisableTLSALPNChallenge: true,
		DNS01Solver:             &dns01Solver{provider: acmeProvider},
	})
	magic.Issuers = []certmagic.Issuer{issuer}

	return &Manager{cfg: cfg, magic: magic}, nil
}

type dns01Solver struct {
	provider *dnsengine.ACMEProvider
}

func (s *dns01Solver) Present(ctx context.Context, challenge acme.Challenge) error {
	dnsName := challenge.DNS01TXTRecordName()
	keyAuth := challenge.DNS01KeyAuthorization()
	return s.provider.SetTXTRecord(dnsName, keyAuth)
}

func (s *dns01Solver) CleanUp(ctx context.Context, challenge acme.Challenge) error {
	dnsName := challenge.DNS01TXTRecordName()
	return s.provider.DeleteTXTRecord(dnsName)
}

var _ acmez.Solver = (*dns01Solver)(nil)

// dnsChallengeSettleDelay is how long to wait between separate ACME DNS-01
// challenges that reuse the same _acme-challenge.<domain> TXT name. Certmagic
// issues apex and wildcard as separate certificates; without a pause, Let's
// Encrypt secondary validators can still see the previous challenge token.
const dnsChallengeSettleDelay = 90 * time.Second

func (m *Manager) ObtainCertificate(ctx context.Context) error {
	domain := m.cfg.Domain
	wildcard := "*." + domain

	// Wildcard first — required for HTTPS payload subdomains (token.x.domain).
	log.Printf("Obtaining TLS certificate for %s", wildcard)
	if err := m.magic.ManageSync(ctx, []string{wildcard}); err != nil {
		return fmt.Errorf("wildcard cert: %w", err)
	}

	log.Printf("Waiting %s for ACME DNS challenge TTL before obtaining apex cert", dnsChallengeSettleDelay)
	if err := wait(ctx, dnsChallengeSettleDelay); err != nil {
		return err
	}

	log.Printf("Obtaining TLS certificate for %s", domain)
	if err := m.magic.ManageSync(ctx, []string{domain}); err != nil {
		return fmt.Errorf("apex cert: %w", err)
	}
	return nil
}

func wait(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func (m *Manager) TLSConfig() *tls.Config {
	return m.magic.TLSConfig()
}

func (m *Manager) ListenAndServe(addr string, handler http.Handler) error {
	ln, err := tls.Listen("tcp", addr, m.TLSConfig())
	if err != nil {
		return err
	}
	return http.Serve(ln, handler)
}

func HTTPSAddr(port int) string {
	return fmt.Sprintf(":%d", port)
}

// DevListenAndServe runs plain HTTP for local development.
func DevListenAndServe(addr string, handler http.Handler) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return http.Serve(ln, handler)
}
