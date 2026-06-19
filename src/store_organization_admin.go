// Copyright (c) 2026 Opensense Ltd. (Hong Kong). All rights reserved.
// Proprietary software. No use, copy, modification, distribution, disclosure,
// or reverse engineering is permitted without prior written authorization
// from Opensense Ltd.

package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
	"time"
)

const (
	organizationAdminTXTRecordPrefix = "_ai-workspace-proxy-verification"
)

func organizationAdminVerificationRecordName() string {
	return organizationAdminTXTRecordPrefix
}

func organizationAdminVerificationHost(domain string) string {
	domain = normalizeDomain(domain)
	if domain == "" {
		return ""
	}
	return organizationAdminVerificationRecordName() + "." + domain
}

func organizationAdminVerificationTXTValue(token string) string {
	return "ai-workspace-proxy-verification=" + strings.TrimSpace(token)
}

func organizationAdminVerificationTokenForEmail(email string, secret []byte) (string, error) {
	email = normalizeEmail(email)
	if len(secret) == 0 {
		return "", fmt.Errorf("organization admin verification key is not configured")
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(strings.TrimSpace(email)))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func organizationAdminVerificationTXTValueForEmail(email string, secret []byte) (string, error) {
	token, err := organizationAdminVerificationTokenForEmail(email, secret)
	if err != nil {
		return "", err
	}
	return organizationAdminVerificationTXTValue(token), nil
}

func (s *Store) GetOrganizationAdminVerification(userID string) (*OrganizationAdminVerification, error) {
	row, err := s.db.QueryOne(`SELECT * FROM organization_admin_verifications WHERE user_id = ?;`, strings.TrimSpace(userID))
	if err != nil {
		return nil, err
	}
	return organizationAdminVerificationFromRow(row), nil
}

func (s *Store) HasVerifiedOrganizationAdmin(domain string) (bool, error) {
	domain = normalizeDomain(domain)
	if domain == "" {
		return false, nil
	}
	row, err := s.db.QueryOne(`SELECT COUNT(*) AS cnt
		FROM organization_admin_verifications
		WHERE domain_name = ?
		  AND verified_at IS NOT NULL
		  AND TRIM(verified_at) <> '';`, domain)
	if err != nil {
		return false, err
	}
	return atoiSafe(row["cnt"]) > 0, nil
}

func (s *Store) EnsureOrganizationAdminVerification(user *User) (*OrganizationAdminVerification, error) {
	if user == nil || strings.TrimSpace(user.ID) == "" {
		return nil, fmt.Errorf("user is required")
	}
	domain := organizationDomainForEmail(user.Email)
	if domain == "" {
		return nil, nil
	}
	existing, err := s.GetOrganizationAdminVerification(user.ID)
	if err != nil {
		return nil, err
	}
	wantOrganizationID := strings.TrimSpace(user.OrganizationID)
	wantHost := organizationAdminVerificationHost(domain)
	wantValue, err := organizationAdminVerificationTXTValueForEmail(user.Email, s.organizationAdminVerificationKey)
	if err != nil {
		return nil, err
	}
	if existing != nil &&
		existing.DomainName == domain &&
		strings.TrimSpace(existing.VerificationHost) == wantHost &&
		strings.TrimSpace(existing.VerificationValue) == wantValue {
		if strings.TrimSpace(existing.OrganizationID) == wantOrganizationID {
			return existing, nil
		}
		existing.OrganizationID = wantOrganizationID
		if err := s.SaveOrganizationAdminVerification(existing); err != nil {
			return nil, err
		}
		return s.GetOrganizationAdminVerification(user.ID)
	}
	now := nowUTC()
	record := &OrganizationAdminVerification{
		UserID:            user.ID,
		OrganizationID:    wantOrganizationID,
		DomainName:        domain,
		VerificationHost:  wantHost,
		VerificationValue: wantValue,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if existing != nil {
		if !existing.CreatedAt.IsZero() {
			record.CreatedAt = existing.CreatedAt
		}
		if strings.TrimSpace(existing.VerificationValue) == wantValue &&
			strings.TrimSpace(existing.VerificationHost) == wantHost &&
			existing.DomainName == domain {
			record.VerifiedAt = existing.VerifiedAt
			record.LastCheckedAt = existing.LastCheckedAt
		}
	}
	if err := s.SaveOrganizationAdminVerification(record); err != nil {
		return nil, err
	}
	return s.GetOrganizationAdminVerification(user.ID)
}

func (s *Store) SaveOrganizationAdminVerification(record *OrganizationAdminVerification) error {
	if record == nil {
		return fmt.Errorf("organization admin verification is required")
	}
	now := nowUTC()
	if record.CreatedAt.IsZero() {
		record.CreatedAt = now
	}
	record.UpdatedAt = now
	var verifiedAt any
	if !record.VerifiedAt.IsZero() {
		verifiedAt = record.VerifiedAt
	}
	var lastCheckedAt any
	if !record.LastCheckedAt.IsZero() {
		lastCheckedAt = record.LastCheckedAt
	}
	return s.db.Exec(`INSERT INTO organization_admin_verifications (
		user_id, organization_id, domain_name, verification_host, verification_value, verified_at, last_checked_at, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(user_id) DO UPDATE SET
		organization_id = excluded.organization_id,
		domain_name = excluded.domain_name,
		verification_host = excluded.verification_host,
		verification_value = excluded.verification_value,
		verified_at = excluded.verified_at,
		last_checked_at = excluded.last_checked_at,
		updated_at = excluded.updated_at;`,
		record.UserID,
		record.OrganizationID,
		record.DomainName,
		record.VerificationHost,
		record.VerificationValue,
		verifiedAt,
		lastCheckedAt,
		record.CreatedAt,
		record.UpdatedAt,
	)
}

func (s *Store) MarkOrganizationAdminVerificationChecked(userID string, checkedAt time.Time) error {
	return s.db.Exec(`UPDATE organization_admin_verifications
		SET last_checked_at = ?, updated_at = ?
		WHERE user_id = ?;`, checkedAt, checkedAt, strings.TrimSpace(userID))
}

func (s *Store) MarkOrganizationAdminVerificationVerified(userID string, checkedAt time.Time) error {
	return s.db.Exec(`UPDATE organization_admin_verifications
		SET verified_at = ?, last_checked_at = ?, updated_at = ?
		WHERE user_id = ?;`, checkedAt, checkedAt, checkedAt, strings.TrimSpace(userID))
}

func organizationAdminVerificationFromRow(row map[string]string) *OrganizationAdminVerification {
	if row == nil {
		return nil
	}
	return &OrganizationAdminVerification{
		UserID:            row["user_id"],
		OrganizationID:    row["organization_id"],
		DomainName:        row["domain_name"],
		VerificationHost:  row["verification_host"],
		VerificationValue: row["verification_value"],
		VerifiedAt:        parseTime(row["verified_at"]),
		LastCheckedAt:     parseTime(row["last_checked_at"]),
		CreatedAt:         parseTime(row["created_at"]),
		UpdatedAt:         parseTime(row["updated_at"]),
	}
}
