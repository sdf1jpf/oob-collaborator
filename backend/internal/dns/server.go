package dnsengine

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/miekg/dns"
	"github.com/oob-collaborator/backend/internal/config"
	"github.com/oob-collaborator/backend/internal/ratelimit"
	"github.com/oob-collaborator/backend/internal/recon"
	"github.com/oob-collaborator/backend/internal/store"
	"github.com/oob-collaborator/backend/internal/token"
	"github.com/oob-collaborator/backend/internal/ws"
)

const (
	dnsLogQueueSize = 256
	dnsLogWorkers   = 4
)

type dnsLogJob struct {
	qname      string
	remoteAddr string
	msg        *dns.Msg
}

type Server struct {
	cfg      *config.Config
	store    *store.Store
	hub      *ws.Hub
	enricher *recon.Enricher
	acme     *ACMEProvider
	limiter  *ratelimit.IPLimiter
	logQueue chan dnsLogJob
	udpSrv   *dns.Server
	tcpSrv   *dns.Server
}

func NewServer(cfg *config.Config, st *store.Store, hub *ws.Hub, enricher *recon.Enricher, acme *ACMEProvider, limiter *ratelimit.IPLimiter) *Server {
	s := &Server{
		cfg:      cfg,
		store:    st,
		hub:      hub,
		enricher: enricher,
		acme:     acme,
		limiter:  limiter,
		logQueue: make(chan dnsLogJob, dnsLogQueueSize),
	}
	handler := dns.HandlerFunc(s.handleDNS)
	s.udpSrv = &dns.Server{Addr: fmt.Sprintf(":%d", cfg.DNSPort), Net: "udp", Handler: handler}
	s.tcpSrv = &dns.Server{Addr: fmt.Sprintf(":%d", cfg.DNSPort), Net: "tcp", Handler: handler}
	return s
}

func (s *Server) Start() error {
	for i := 0; i < dnsLogWorkers; i++ {
		go s.logWorker()
	}
	go func() {
		log.Printf("DNS listening UDP :%d", s.cfg.DNSPort)
		if err := s.udpSrv.ListenAndServe(); err != nil {
			log.Printf("DNS UDP error: %v", err)
		}
	}()
	go func() {
		log.Printf("DNS listening TCP :%d", s.cfg.DNSPort)
		if err := s.tcpSrv.ListenAndServe(); err != nil {
			log.Printf("DNS TCP error: %v", err)
		}
	}()
	return nil
}

func (s *Server) Shutdown() {
	_ = s.udpSrv.Shutdown()
	_ = s.tcpSrv.Shutdown()
}

func (s *Server) logWorker() {
	for job := range s.logQueue {
		s.logDNSInteraction(job.qname, job.remoteAddr, job.msg)
	}
}

func (s *Server) handleDNS(w dns.ResponseWriter, r *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative = true

	if len(r.Question) == 0 {
		_ = w.WriteMsg(m)
		return
	}

	q := r.Question[0]
	qname := strings.ToLower(strings.TrimSuffix(q.Name, "."))

	switch q.Qtype {
	case dns.TypeA:
		s.answerA(m, qname, w, r)
	case dns.TypeNS:
		s.answerNS(m, qname)
	case dns.TypeSOA:
		s.answerSOA(m, qname)
	case dns.TypeTXT:
		s.answerTXT(m, qname)
	default:
	}

	_ = w.WriteMsg(m)
}

func (s *Server) answerA(m *dns.Msg, qname string, w dns.ResponseWriter, r *dns.Msg) {
	if !s.isOurZone(qname) {
		return
	}

	rr := &dns.A{
		Hdr: dns.RR_Header{
			Name:   dns.Fqdn(qname),
			Rrtype: dns.TypeA,
			Class:  dns.ClassINET,
			Ttl:    60,
		},
		A: net.ParseIP(s.cfg.PublicIP),
	}
	m.Answer = append(m.Answer, rr)

	s.enqueueDNSLog(qname, w.RemoteAddr().String(), r)
}

func (s *Server) enqueueDNSLog(qname, remoteAddr string, r *dns.Msg) {
	job := dnsLogJob{qname: qname, remoteAddr: remoteAddr, msg: r.Copy()}
	select {
	case s.logQueue <- job:
	default:
		log.Printf("dns interaction log: queue full, dropping %s", qname)
	}
}

func (s *Server) answerNS(m *dns.Msg, qname string) {
	domain := strings.ToLower(s.cfg.Domain)
	if qname != domain {
		return
	}
	rr := &dns.NS{
		Hdr: dns.RR_Header{
			Name:   dns.Fqdn(domain),
			Rrtype: dns.TypeNS,
			Class:  dns.ClassINET,
			Ttl:    3600,
		},
		Ns: dns.Fqdn(s.cfg.NSFQDN()),
	}
	m.Answer = append(m.Answer, rr)
}

func (s *Server) answerSOA(m *dns.Msg, qname string) {
	domain := strings.ToLower(s.cfg.Domain)
	if qname != domain {
		return
	}
	rr := &dns.SOA{
		Hdr: dns.RR_Header{
			Name:   dns.Fqdn(domain),
			Rrtype: dns.TypeSOA,
			Class:  dns.ClassINET,
			Ttl:    3600,
		},
		Ns:      dns.Fqdn(s.cfg.NSFQDN()),
		Mbox:    dns.Fqdn("hostmaster." + domain),
		Serial:  2026010101,
		Refresh: 7200,
		Retry:   3600,
		Expire:  1209600,
		Minttl:  60,
	}
	m.Answer = append(m.Answer, rr)
}

func (s *Server) answerTXT(m *dns.Msg, qname string) {
	if txt, ok := s.acme.GetTXT(qname); ok {
		ttl := uint32(60)
		if strings.HasPrefix(qname, "_acme-challenge.") {
			ttl = 10 // short TTL so sequential apex/wildcard challenges don't collide
		}
		rr := &dns.TXT{
			Hdr: dns.RR_Header{
				Name:   dns.Fqdn(qname),
				Rrtype: dns.TypeTXT,
				Class:  dns.ClassINET,
				Ttl:    ttl,
			},
			Txt: []string{txt},
		}
		m.Answer = append(m.Answer, rr)
	}
}

func (s *Server) isOurZone(qname string) bool {
	domain := strings.ToLower(s.cfg.Domain)
	return qname == domain || strings.HasSuffix(qname, "."+domain)
}

func (s *Server) logDNSInteraction(qname, remoteAddr string, r *dns.Msg) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	subDomain := token.ExtractSubDomain(qname, s.cfg.Domain)
	if subDomain == "" {
		return
	}

	var payloadID *uuid.UUID
	if p, err := s.store.GetPayloadBySubDomain(ctx, subDomain); err == nil {
		payloadID = &p.ID
	} else if !s.cfg.LogUnknownTokens {
		return
	}

	sourceIP := remoteAddr
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		sourceIP = host
	}
	if s.limiter != nil && !s.limiter.Allow(sourceIP) {
		return
	}

	questions := make([]map[string]string, 0, len(r.Question))
	for _, q := range r.Question {
		questions = append(questions, map[string]string{
			"name":  q.Name,
			"type":  dns.TypeToString[q.Qtype],
			"class": dns.ClassToString[q.Qclass],
		})
	}

	raw, _ := json.Marshal(map[string]any{
		"qname":     qname,
		"subdomain": subDomain,
		"questions": questions,
		"id":        r.Id,
	})

	interaction, err := s.store.CreateInteraction(ctx, payloadID, "DNS", sourceIP, string(raw))
	if err != nil {
		log.Printf("dns interaction log: %v", err)
		return
	}
	interaction.SubDomain = subDomain
	if payloadID != nil {
		if p, err := s.store.GetPayloadBySubDomain(ctx, subDomain); err == nil {
			interaction.EngagementID = &p.EngagementID
		}
	}
	if s.enricher != nil {
		s.enricher.Enqueue(sourceIP)
	}
	if interaction.EngagementID != nil {
		s.hub.BroadcastInteraction(interaction)
	}
}
