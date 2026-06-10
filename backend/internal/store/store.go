package store

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DNS-safe lowercase alphanumeric alphabet for short payload labels.
const payloadTokenChars = "abcdefghijklmnopqrstuvwxyz0123456789"

type Store struct {
	pool               *pgxpool.Pool
	payloadTokenLength int
}

func New(ctx context.Context, databaseURL string, payloadTokenLength int) (*Store, error) {
	if payloadTokenLength < 4 {
		payloadTokenLength = 4
	}
	if payloadTokenLength > 32 {
		payloadTokenLength = 32
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("connect database: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return &Store{pool: pool, payloadTokenLength: payloadTokenLength}, nil
}

func (s *Store) Close() {
	s.pool.Close()
}

func (s *Store) ListEngagements(ctx context.Context) ([]Engagement, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, client_name, created_at
		FROM engagements
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Engagement
	for rows.Next() {
		var e Engagement
		if err := rows.Scan(&e.ID, &e.Name, &e.ClientName, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) CreateEngagement(ctx context.Context, name, clientName string) (*Engagement, error) {
	var e Engagement
	err := s.pool.QueryRow(ctx, `
		INSERT INTO engagements (name, client_name)
		VALUES ($1, $2)
		RETURNING id, name, client_name, created_at
	`, name, clientName).Scan(&e.ID, &e.Name, &e.ClientName, &e.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (s *Store) GetEngagement(ctx context.Context, id uuid.UUID) (*Engagement, error) {
	var e Engagement
	err := s.pool.QueryRow(ctx, `
		SELECT id, name, client_name, created_at
		FROM engagements
		WHERE id = $1
	`, id).Scan(&e.ID, &e.Name, &e.ClientName, &e.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (s *Store) GeneratePayload(ctx context.Context, engagementID uuid.UUID, description string) (*Payload, error) {
	const maxAttempts = 8
	for range maxAttempts {
		token, err := randomPayloadToken(s.payloadTokenLength)
		if err != nil {
			return nil, err
		}

		var p Payload
		err = s.pool.QueryRow(ctx, `
			INSERT INTO payloads (engagement_id, sub_domain, description)
			VALUES ($1, $2, $3)
			RETURNING id, engagement_id, sub_domain, description, created_at
		`, engagementID, token, description).Scan(&p.ID, &p.EngagementID, &p.SubDomain, &p.Description, &p.CreatedAt)
		if err == nil {
			return &p, nil
		}
		if isUniqueViolation(err) {
			continue
		}
		return nil, err
	}
	return nil, errors.New("failed to generate unique payload token")
}

func (s *Store) ListPayloadsByEngagement(ctx context.Context, engagementID uuid.UUID) ([]Payload, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, engagement_id, sub_domain, description, created_at
		FROM payloads
		WHERE engagement_id = $1
		ORDER BY created_at DESC
	`, engagementID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Payload
	for rows.Next() {
		var p Payload
		if err := rows.Scan(&p.ID, &p.EngagementID, &p.SubDomain, &p.Description, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) GetPayloadBySubDomain(ctx context.Context, subDomain string) (*Payload, error) {
	var p Payload
	err := s.pool.QueryRow(ctx, `
		SELECT id, engagement_id, sub_domain, description, created_at
		FROM payloads
		WHERE sub_domain = $1
	`, subDomain).Scan(&p.ID, &p.EngagementID, &p.SubDomain, &p.Description, &p.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *Store) CreateInteraction(ctx context.Context, payloadID *uuid.UUID, protocol, sourceIP, rawData string) (*Interaction, error) {
	var i Interaction
	err := s.pool.QueryRow(ctx, `
		INSERT INTO interactions (payload_id, protocol, source_ip, raw_data)
		VALUES ($1, $2, $3, $4)
		RETURNING id, payload_id, protocol, source_ip, raw_data, interacted_at, delivered_at
	`, payloadID, protocol, sourceIP, rawData).Scan(
		&i.ID, &i.PayloadID, &i.Protocol, &i.SourceIP, &i.RawData, &i.InteractedAt, &i.DeliveredAt,
	)
	if err != nil {
		return nil, err
	}
	return &i, nil
}

func (s *Store) GetIPRecon(ctx context.Context, ip string) (*IPRecon, error) {
	var r IPRecon
	err := s.pool.QueryRow(ctx, `
		SELECT ip, reverse_dns, country, country_code, region, city, lat, lon, isp, org, asn, status, updated_at
		FROM ip_recon
		WHERE ip = $1
	`, ip).Scan(
		&r.IP, &r.ReverseDNS, &r.Country, &r.CountryCode, &r.Region, &r.City,
		&r.Lat, &r.Lon, &r.ISP, &r.Org, &r.ASN, &r.Status, &r.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *Store) UpsertIPRecon(ctx context.Context, r *IPRecon) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO ip_recon (ip, reverse_dns, country, country_code, region, city, lat, lon, isp, org, asn, status, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NOW())
		ON CONFLICT (ip) DO UPDATE SET
			reverse_dns = EXCLUDED.reverse_dns,
			country = EXCLUDED.country,
			country_code = EXCLUDED.country_code,
			region = EXCLUDED.region,
			city = EXCLUDED.city,
			lat = EXCLUDED.lat,
			lon = EXCLUDED.lon,
			isp = EXCLUDED.isp,
			org = EXCLUDED.org,
			asn = EXCLUDED.asn,
			status = EXCLUDED.status,
			updated_at = NOW()
	`, r.IP, r.ReverseDNS, r.Country, r.CountryCode, r.Region, r.City, r.Lat, r.Lon, r.ISP, r.Org, r.ASN, r.Status)
	return err
}

func (s *Store) ListInteractionsByEngagement(ctx context.Context, engagementID uuid.UUID) ([]Interaction, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT i.id, i.payload_id, p.engagement_id, p.sub_domain, i.protocol, i.source_ip, i.raw_data, i.interacted_at, i.delivered_at,
			r.ip, r.reverse_dns, r.country, r.country_code, r.region, r.city, r.lat, r.lon, r.isp, r.org, r.asn, r.status, r.updated_at
		FROM interactions i
		LEFT JOIN payloads p ON p.id = i.payload_id
		LEFT JOIN ip_recon r ON r.ip = i.source_ip
		WHERE p.engagement_id = $1
		ORDER BY i.interacted_at DESC
	`, engagementID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Interaction
	for rows.Next() {
		var i Interaction
		var engagementIDPtr *uuid.UUID
		var subDomain *string
		var reconIP *string
		var reconReverseDNS, reconCountry, reconCountryCode, reconRegion, reconCity *string
		var reconISP, reconOrg, reconASN, reconStatus *string
		var reconLat, reconLon *float64
		var reconUpdatedAt *time.Time
		if err := rows.Scan(
			&i.ID, &i.PayloadID, &engagementIDPtr, &subDomain, &i.Protocol, &i.SourceIP, &i.RawData, &i.InteractedAt, &i.DeliveredAt,
			&reconIP, &reconReverseDNS, &reconCountry, &reconCountryCode, &reconRegion, &reconCity,
			&reconLat, &reconLon, &reconISP, &reconOrg, &reconASN, &reconStatus, &reconUpdatedAt,
		); err != nil {
			return nil, err
		}
		i.EngagementID = engagementIDPtr
		if subDomain != nil {
			i.SubDomain = *subDomain
		}
		if reconIP != nil {
			i.IPRecon = &IPRecon{
				IP:          *reconIP,
				ReverseDNS:  derefString(reconReverseDNS),
				Country:     derefString(reconCountry),
				CountryCode: derefString(reconCountryCode),
				Region:      derefString(reconRegion),
				City:        derefString(reconCity),
				Lat:         reconLat,
				Lon:         reconLon,
				ISP:         derefString(reconISP),
				Org:         derefString(reconOrg),
				ASN:         derefString(reconASN),
				Status:      derefString(reconStatus),
				UpdatedAt:   derefTime(reconUpdatedAt),
			}
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefTime(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}

func (s *Store) FetchUndeliveredInteractions(ctx context.Context, limit int) ([]Interaction, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT i.id, i.payload_id, p.engagement_id, p.sub_domain, i.protocol, i.source_ip, i.raw_data, i.interacted_at, i.delivered_at
		FROM interactions i
		LEFT JOIN payloads p ON p.id = i.payload_id
		WHERE i.delivered_at IS NULL
		ORDER BY i.interacted_at ASC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Interaction
	for rows.Next() {
		var i Interaction
		var engagementIDPtr *uuid.UUID
		var subDomain *string
		if err := rows.Scan(&i.ID, &i.PayloadID, &engagementIDPtr, &subDomain, &i.Protocol, &i.SourceIP, &i.RawData, &i.InteractedAt, &i.DeliveredAt); err != nil {
			return nil, err
		}
		i.EngagementID = engagementIDPtr
		if subDomain != nil {
			i.SubDomain = *subDomain
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

func (s *Store) MarkInteractionsDelivered(ctx context.Context, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := s.pool.Exec(ctx, `
		UPDATE interactions
		SET delivered_at = NOW()
		WHERE id = ANY($1)
	`, ids)
	return err
}

func (s *Store) GetInteraction(ctx context.Context, id uuid.UUID) (*Interaction, error) {
	var i Interaction
	var engagementIDPtr *uuid.UUID
	var subDomain *string
	err := s.pool.QueryRow(ctx, `
		SELECT i.id, i.payload_id, p.engagement_id, p.sub_domain, i.protocol, i.source_ip, i.raw_data, i.interacted_at, i.delivered_at
		FROM interactions i
		LEFT JOIN payloads p ON p.id = i.payload_id
		WHERE i.id = $1
	`, id).Scan(&i.ID, &i.PayloadID, &engagementIDPtr, &subDomain, &i.Protocol, &i.SourceIP, &i.RawData, &i.InteractedAt, &i.DeliveredAt)
	if err != nil {
		return nil, err
	}
	i.EngagementID = engagementIDPtr
	if subDomain != nil {
		i.SubDomain = *subDomain
	}
	return &i, nil
}

func randomPayloadToken(length int) (string, error) {
	if length < 4 {
		length = 4
	}
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	out := make([]byte, length)
	for i, v := range b {
		out[i] = payloadTokenChars[int(v)%len(payloadTokenChars)]
	}
	return string(out), nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func IsNotFound(err error) bool {
	return err == pgx.ErrNoRows
}

func (s *Store) WaitForDB(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if err := s.pool.Ping(ctx); err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("database not ready after %s", timeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}
