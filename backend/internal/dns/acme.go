package dnsengine

import (
	"context"
	"strings"
	"sync"
)

// ACMEProvider stores DNS-01 challenge TXT records served by our authoritative DNS.
type ACMEProvider struct {
	mu      sync.RWMutex
	records map[string]string
}

func NewACMEProvider() *ACMEProvider {
	return &ACMEProvider{records: make(map[string]string)}
}

func (p *ACMEProvider) SetTXTRecord(name, value string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.records[strings.ToLower(strings.TrimSuffix(name, "."))] = value
	return nil
}

func (p *ACMEProvider) DeleteTXTRecord(name string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.records, strings.ToLower(strings.TrimSuffix(name, ".")))
	return nil
}

func (p *ACMEProvider) GetTXT(name string) (string, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	v, ok := p.records[strings.ToLower(strings.TrimSuffix(name, "."))]
	return v, ok
}

// CertMagicDNS implements certmagic.DNSProvider interface.
type CertMagicDNS struct {
	provider *ACMEProvider
}

func NewCertMagicDNS(p *ACMEProvider) *CertMagicDNS {
	return &CertMagicDNS{provider: p}
}

func (d *CertMagicDNS) AppendRecords(ctx context.Context, zone string, recs []interface{}) error {
	for _, r := range recs {
		if m, ok := r.(map[string]string); ok {
			if txt, ok := m["txt"]; ok {
				name := m["name"]
				if name == "" {
					name = zone
				}
				_ = d.provider.SetTXTRecord(name, txt)
			}
		}
	}
	return nil
}

func (d *CertMagicDNS) DeleteRecords(ctx context.Context, zone string, recs []interface{}) error {
	for _, r := range recs {
		if m, ok := r.(map[string]string); ok {
			name := m["name"]
			if name == "" {
				name = zone
			}
			_ = d.provider.DeleteTXTRecord(name)
		}
	}
	return nil
}

// CertMagicDNS01 implements the DNS01Solver interface used by certmagic.
type CertMagicDNS01 struct {
	provider *ACMEProvider
}

func NewCertMagicDNS01(p *ACMEProvider) *CertMagicDNS01 {
	return &CertMagicDNS01{provider: p}
}

func (s *CertMagicDNS01) Present(ctx context.Context, challengeTXT, fqdn string) error {
	return s.provider.SetTXTRecord(fqdn, challengeTXT)
}

func (s *CertMagicDNS01) CleanUp(ctx context.Context, challengeTXT, fqdn string) error {
	return s.provider.DeleteTXTRecord(fqdn)
}
