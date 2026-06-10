package store

import (
	"time"

	"github.com/google/uuid"
)

type Engagement struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	ClientName string    `json:"client_name"`
	CreatedAt  time.Time `json:"created_at"`
}

type Payload struct {
	ID           uuid.UUID `json:"id"`
	EngagementID uuid.UUID `json:"engagement_id"`
	SubDomain    string    `json:"sub_domain"`
	Description  string    `json:"description"`
	CreatedAt    time.Time `json:"created_at"`
}

type IPRecon struct {
	IP          string    `json:"ip"`
	ReverseDNS  string    `json:"reverse_dns"`
	Country     string    `json:"country"`
	CountryCode string    `json:"country_code"`
	Region      string    `json:"region"`
	City        string    `json:"city"`
	Lat         *float64  `json:"lat,omitempty"`
	Lon         *float64  `json:"lon,omitempty"`
	ISP         string    `json:"isp"`
	Org         string    `json:"org"`
	ASN         string    `json:"asn"`
	Status      string    `json:"status"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Interaction struct {
	ID           uuid.UUID  `json:"id"`
	PayloadID    *uuid.UUID `json:"payload_id,omitempty"`
	EngagementID *uuid.UUID `json:"engagement_id,omitempty"`
	SubDomain    string     `json:"sub_domain,omitempty"`
	Protocol     string     `json:"protocol"`
	SourceIP     string     `json:"source_ip"`
	RawData      string     `json:"raw_data"`
	InteractedAt time.Time  `json:"interacted_at"`
	DeliveredAt  *time.Time `json:"delivered_at,omitempty"`
	IPRecon      *IPRecon   `json:"ip_recon,omitempty"`
}

type PollInteraction struct {
	Interaction
	InteractionType string `json:"interaction_type"`
	Host            string `json:"host"`
}
