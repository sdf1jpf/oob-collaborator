package tlsmanager

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"

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

func (m *Manager) ObtainCertificate(ctx context.Context) error {
	domains := []string{m.cfg.Domain, "*." + m.cfg.Domain}
	log.Printf("Obtaining TLS certificates for %v", domains)
	return m.magic.ManageSync(ctx, domains)
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
