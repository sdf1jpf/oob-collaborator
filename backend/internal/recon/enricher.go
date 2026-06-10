package recon

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/oob-collaborator/backend/internal/config"
	"github.com/oob-collaborator/backend/internal/store"
	"github.com/oob-collaborator/backend/internal/ws"
)

const (
	cacheTTL       = 14 * 24 * time.Hour
	requestTimeout = 10 * time.Second
	throttleDelay  = 1500 * time.Millisecond
	queueSize      = 256
)

type Enricher struct {
	cfg    *config.Config
	store  *store.Store
	hub    *ws.Hub
	queue  chan string
	stop   chan struct{}
	wg     sync.WaitGroup
	inflight sync.Map
	client *http.Client
}

func New(cfg *config.Config, st *store.Store, hub *ws.Hub) *Enricher {
	return &Enricher{
		cfg:   cfg,
		store: st,
		hub:   hub,
		queue: make(chan string, queueSize),
		stop:  make(chan struct{}),
		client: &http.Client{
			Timeout: requestTimeout,
		},
	}
}

func (e *Enricher) Start() {
	if !e.cfg.IPReconEnabled {
		return
	}
	e.wg.Add(1)
	go e.worker()
}

func (e *Enricher) Stop() {
	if !e.cfg.IPReconEnabled {
		return
	}
	close(e.stop)
	e.wg.Wait()
}

func (e *Enricher) Enqueue(ip string) {
	if e == nil || !e.cfg.IPReconEnabled {
		return
	}
	if isPrivateOrLoopback(ip) {
		return
	}
	select {
	case e.queue <- ip:
	default:
		log.Printf("ip recon: queue full, dropping %s", ip)
	}
}

func (e *Enricher) worker() {
	defer e.wg.Done()
	for {
		select {
		case <-e.stop:
			return
		case ip := <-e.queue:
			e.process(ip)
			time.Sleep(throttleDelay)
		}
	}
}

func (e *Enricher) process(ip string) {
	if _, loaded := e.inflight.LoadOrStore(ip, struct{}{}); loaded {
		return
	}
	defer e.inflight.Delete(ip)

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	if existing, err := e.store.GetIPRecon(ctx, ip); err == nil {
		if time.Since(existing.UpdatedAt) < cacheTTL {
			e.broadcastIPRecon(existing)
			return
		}
	} else if !store.IsNotFound(err) {
		log.Printf("ip recon: get %s: %v", ip, err)
		return
	}

	recon, err := e.lookup(ctx, ip)
	if err != nil {
		log.Printf("ip recon: lookup %s: %v", ip, err)
		return
	}

	if err := e.store.UpsertIPRecon(ctx, recon); err != nil {
		log.Printf("ip recon: upsert %s: %v", ip, err)
		return
	}

	if saved, err := e.store.GetIPRecon(ctx, ip); err == nil {
		e.broadcastIPRecon(saved)
	}
}

func (e *Enricher) broadcastIPRecon(recon *store.IPRecon) {
	if e.hub != nil {
		e.hub.BroadcastIPRecon(recon)
	}
}

type ipAPIResponse struct {
	Status      string  `json:"status"`
	Country     string  `json:"country"`
	CountryCode string  `json:"countryCode"`
	RegionName  string  `json:"regionName"`
	City        string  `json:"city"`
	Lat         float64 `json:"lat"`
	Lon         float64 `json:"lon"`
	ISP         string  `json:"isp"`
	Org         string  `json:"org"`
	AS          string  `json:"as"`
	Message     string  `json:"message"`
}

func (e *Enricher) lookup(ctx context.Context, ip string) (*store.IPRecon, error) {
	reverseDNS := ""
	if names, err := net.LookupAddr(ip); err == nil && len(names) > 0 {
		reverseDNS = names[0]
	}

	url := fmt.Sprintf(
		"http://ip-api.com/json/%s?fields=status,message,country,countryCode,regionName,city,lat,lon,isp,org,as",
		ip,
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	res, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var api ipAPIResponse
	if err := json.NewDecoder(res.Body).Decode(&api); err != nil {
		return nil, err
	}

	status := api.Status
	if status == "" {
		status = "fail"
	}

	recon := &store.IPRecon{
		IP:          ip,
		ReverseDNS:  reverseDNS,
		Country:     api.Country,
		CountryCode: api.CountryCode,
		Region:      api.RegionName,
		City:        api.City,
		ISP:         api.ISP,
		Org:         api.Org,
		ASN:         api.AS,
		Status:      status,
	}

	if api.Status == "success" {
		lat, lon := api.Lat, api.Lon
		recon.Lat = &lat
		recon.Lon = &lon
	} else if api.Message != "" {
		recon.Status = api.Message
	}

	return recon, nil
}

func isPrivateOrLoopback(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() {
		return true
	}
	return false
}
