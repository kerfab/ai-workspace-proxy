// Copyright (c) 2026 Opensense Ltd. (Hong Kong). All rights reserved.
// Proprietary software. No use, copy, modification, distribution, disclosure,
// or reverse engineering is permitted without prior written authorization
// from Opensense Ltd.

package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type organizationDNSAttempt struct {
	QueryType string `json:"query_type"`
	Hostname  string `json:"hostname"`
	DNSServer string `json:"dns_server"`
	Result    string `json:"result"`
	Notes     string `json:"notes,omitempty"`
}

func organizationAdminStatusClass(verified bool) string {
	if verified {
		return "logging-status-enabled"
	}
	return "logging-status-disabled"
}

func organizationAdminStatusText(verified bool) string {
	if verified {
		return "Verified"
	}
	return "Unverified"
}

func (a *App) canAccessOrganizationAdmin(user *User) (bool, error) {
	if !isOrganizationCustomer(user) {
		return false, nil
	}
	domain := organizationDomainForEmail(user.Email)
	if domain == "" {
		return false, nil
	}
	hasVerifiedAdmin, err := a.store.HasVerifiedOrganizationAdmin(domain)
	if err != nil {
		return false, err
	}
	if !hasVerifiedAdmin {
		return true, nil
	}
	record, err := a.store.GetOrganizationAdminVerification(user.ID)
	if err != nil {
		return false, err
	}
	return record != nil && !record.VerifiedAt.IsZero() && normalizeDomain(record.DomainName) == domain, nil
}

func (a *App) organizationAdminView(user *User, timezone string) (map[string]any, error) {
	record, err := a.store.EnsureOrganizationAdminVerification(user)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, fmt.Errorf("organization domain could not be resolved for this account")
	}
	hasVerifiedAdmin, err := a.store.HasVerifiedOrganizationAdmin(record.DomainName)
	if err != nil {
		return nil, err
	}
	verified := !record.VerifiedAt.IsZero()
	verificationHost := organizationAdminVerificationHost(record.DomainName)
	view := map[string]any{
		"Domain":               record.DomainName,
		"TXTRecordName":        organizationAdminVerificationRecordName(),
		"TXTRecordZone":        record.DomainName,
		"TXTRecordHost":        verificationHost,
		"TXTRecordValue":       record.VerificationValue,
		"Verified":             verified,
		"HasVerifiedAdmin":     hasVerifiedAdmin,
		"ShowClaimIntro":       !verified && !hasVerifiedAdmin,
		"StatusText":           organizationAdminStatusText(verified),
		"StatusClass":          organizationAdminStatusClass(verified),
		"VerifiedAtDisplay":    "",
		"LastCheckedAt":        "",
		"ValidationTargetFQDN": verificationHost,
	}
	if !record.VerifiedAt.IsZero() {
		view["VerifiedAtDisplay"] = formatUserTime(record.VerifiedAt, timezone)
	}
	if !record.LastCheckedAt.IsZero() {
		view["LastCheckedAt"] = formatUserTime(record.LastCheckedAt, timezone)
	}
	return view, nil
}

func (a *App) handleOrganizationDomainValidate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST required")
		return
	}
	user := a.requireSessionUser(w, r)
	if user == nil || !a.requireSessionCSRF(w, r) {
		return
	}
	canAccess, err := a.canAccessOrganizationAdmin(user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "organization_verification_error", err.Error())
		return
	}
	if !canAccess {
		writeError(w, http.StatusNotFound, "not_found", "page not found")
		return
	}
	record, err := a.store.EnsureOrganizationAdminVerification(user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "organization_verification_error", err.Error())
		return
	}
	if record == nil {
		writeError(w, http.StatusBadRequest, "organization_verification_error", "organization domain could not be resolved for this account")
		return
	}
	checkedAt := nowUTC()
	record.VerificationHost = organizationAdminVerificationHost(record.DomainName)
	txtValues, attempts, lookupErr := a.lookupOrganizationVerificationTXT(r.Context(), record)
	if lookupErr != nil {
		_ = a.store.MarkOrganizationAdminVerificationChecked(user.ID, checkedAt)
		message := organizationTXTLookupFailureMessage(record.VerificationHost, lookupErr)
		a.logUserAudit(r, user, "organization_domain_validation_failed", record.DomainName, record.DomainName, map[string]any{
			"domain":               record.DomainName,
			"verification_host":    record.VerificationHost,
			"failure_reason":       message,
			"dns_lookup_succeeded": false,
			"lookup_mode":          "authoritative",
			"dns_attempts":         attempts,
			"source":               requestSource(r),
		})
		a.respondOrganizationValidationFailure(w, r, message, attempts)
		return
	}
	if !txtValuesContain(txtValues, record.VerificationValue) {
		_ = a.store.MarkOrganizationAdminVerificationChecked(user.ID, checkedAt)
		message := fmt.Sprintf("No matching DNS TXT record value was found for %s on the authoritative nameservers. Confirm the record name and value, then try again.", record.VerificationHost)
		a.logUserAudit(r, user, "organization_domain_validation_failed", record.DomainName, record.DomainName, map[string]any{
			"domain":               record.DomainName,
			"verification_host":    record.VerificationHost,
			"dns_lookup_succeeded": true,
			"txt_records_found":    len(txtValues),
			"lookup_mode":          "authoritative",
			"dns_attempts":         attempts,
			"source":               requestSource(r),
		})
		a.respondOrganizationValidationFailure(w, r, message, attempts)
		return
	}
	if err := a.store.MarkOrganizationAdminVerificationVerified(user.ID, checkedAt); err != nil {
		writeError(w, http.StatusInternalServerError, "organization_verification_error", err.Error())
		return
	}
	record, err = a.store.GetOrganizationAdminVerification(user.ID)
	if err != nil || record == nil {
		writeError(w, http.StatusInternalServerError, "organization_verification_error", firstErr(err, "organization verification state not found").Error())
		return
	}
	a.logUserAudit(r, user, "organization_domain_verified", record.DomainName, record.DomainName, map[string]any{
		"domain":            record.DomainName,
		"verification_host": record.VerificationHost,
		"lookup_mode":       "authoritative",
		"dns_attempts":      attempts,
		"verified_at":       checkedAt.Format(time.RFC3339),
		"source":            requestSource(r),
	})
	if wantsJSON(r) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":              "ok",
			"verified":            true,
			"status_text":         organizationAdminStatusText(true),
			"status_class":        organizationAdminStatusClass(true),
			"domain":              record.DomainName,
			"verification_host":   record.VerificationHost,
			"verification_value":  record.VerificationValue,
			"verified_at_display": formatUserTime(record.VerifiedAt, a.userTimezoneOrDefault(user.ID)),
		})
		return
	}
	http.Redirect(w, r, "/admin/", http.StatusFound)
}

func (a *App) respondOrganizationValidationFailure(w http.ResponseWriter, r *http.Request, message string, attempts []organizationDNSAttempt) {
	if wantsJSON(r) {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"status":  "error",
			"message": message,
			"details": attempts,
		})
		return
	}
	http.Redirect(w, r, "/admin/?org_admin_error="+url.QueryEscape(message), http.StatusFound)
}

func (a *App) lookupOrganizationVerificationTXT(ctx context.Context, record *OrganizationAdminVerification) ([]string, []organizationDNSAttempt, error) {
	if record == nil {
		return nil, nil, fmt.Errorf("organization verification record is required")
	}
	if a.lookupNS == nil {
		a.lookupNS = net.LookupNS
	}
	if a.lookupIPAddr == nil {
		a.lookupIPAddr = net.DefaultResolver.LookupIPAddr
	}
	if a.lookupTXTAtServer == nil {
		a.lookupTXTAtServer = lookupTXTAtServer
	}
	return a.lookupAuthoritativeTXT(ctx, record.DomainName, record.VerificationHost, record.VerificationValue)
}

func (a *App) lookupAuthoritativeTXT(ctx context.Context, domain, host, expectedValue string) ([]string, []organizationDNSAttempt, error) {
	attempts := make([]organizationDNSAttempt, 0, 8)
	queryHost := absoluteDNSName(host)
	nsRecords, err := a.lookupNS(strings.TrimSpace(domain))
	if err != nil {
		attempts = append(attempts, organizationDNSAttempt{
			QueryType: "NS",
			Hostname:  strings.TrimSpace(domain),
			DNSServer: "system resolver",
			Result:    "Fail",
			Notes:     err.Error(),
		})
		return nil, attempts, err
	}
	if len(nsRecords) == 0 {
		attempts = append(attempts, organizationDNSAttempt{
			QueryType: "NS",
			Hostname:  strings.TrimSpace(domain),
			DNSServer: "system resolver",
			Result:    "Fail",
			Notes:     "no authoritative nameservers found",
		})
		return nil, attempts, fmt.Errorf("no authoritative nameservers found for %s", domain)
	}
	nsHosts := make([]string, 0, len(nsRecords))
	for _, nsRecord := range nsRecords {
		nsHost := strings.TrimSuffix(strings.TrimSpace(nsRecord.Host), ".")
		if nsHost != "" {
			nsHosts = append(nsHosts, nsHost)
		}
	}
	attempts = append(attempts, organizationDNSAttempt{
		QueryType: "NS",
		Hostname:  strings.TrimSpace(domain),
		DNSServer: "system resolver",
		Result:    "Success",
		Notes:     strings.Join(nsHosts, ", "),
	})
	var values []string
	seenServers := map[string]bool{}
	var successfulTXTQuery bool
	for _, nsRecord := range nsRecords {
		nsHost := strings.TrimSuffix(strings.TrimSpace(nsRecord.Host), ".")
		if nsHost == "" {
			continue
		}
		ipAddrs, ipErr := a.lookupIPAddr(ctx, nsHost)
		if ipErr != nil {
			attempts = append(attempts, organizationDNSAttempt{
				QueryType: "A/AAAA",
				Hostname:  nsHost,
				DNSServer: "system resolver",
				Result:    "Fail",
				Notes:     ipErr.Error(),
			})
			continue
		}
		if len(ipAddrs) == 0 {
			attempts = append(attempts, organizationDNSAttempt{
				QueryType: "A/AAAA",
				Hostname:  nsHost,
				DNSServer: "system resolver",
				Result:    "Fail",
				Notes:     "no IP address found for nameserver",
			})
			continue
		}
		for _, ipAddr := range ipAddrs {
			serverAddr := net.JoinHostPort(ipAddr.IP.String(), "53")
			if seenServers[serverAddr] {
				continue
			}
			seenServers[serverAddr] = true
			txtValues, txtErr := a.lookupTXTAtServer(ctx, serverAddr, queryHost)
			if txtErr != nil {
				attempts = append(attempts, organizationDNSAttempt{
					QueryType: "TXT",
					Hostname:  queryHost,
					DNSServer: serverAddr,
					Result:    "Fail",
					Notes:     txtErr.Error(),
				})
				continue
			}
			successfulTXTQuery = true
			result := "Success"
			notes := "no TXT values returned"
			if len(txtValues) > 0 {
				notes = strings.Join(uniqueTXTValues(txtValues), " | ")
			}
			attempts = append(attempts, organizationDNSAttempt{
				QueryType: "TXT",
				Hostname:  queryHost,
				DNSServer: serverAddr,
				Result:    result,
				Notes:     notes,
			})
			txtValues = uniqueTXTValues(txtValues)
			values = append(values, txtValues...)
			if txtValuesContain(txtValues, expectedValue) {
				return txtValues, attempts, nil
			}
		}
	}
	values = uniqueTXTValues(values)
	if successfulTXTQuery {
		return values, attempts, nil
	}
	if len(attempts) == 0 {
		return nil, attempts, fmt.Errorf("no authoritative TXT response was returned for %s", host)
	}
	return nil, attempts, fmt.Errorf("authoritative TXT lookup failed for %s", host)
}

func (a *App) userTimezoneOrDefault(userID string) string {
	settings, err := a.store.GetUserSettings(userID)
	if err != nil || settings == nil || strings.TrimSpace(settings.Timezone) == "" {
		return "UTC"
	}
	return settings.Timezone
}

func organizationTXTLookupFailureMessage(host string, err error) string {
	return fmt.Sprintf("The proxy could not read the DNS TXT record for %s from the authoritative nameservers.", host)
}

func absoluteDNSName(host string) string {
	host = strings.TrimSpace(host)
	host = strings.TrimSuffix(host, ".")
	if host == "" {
		return ""
	}
	return host + "."
}

func uniqueTXTValues(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func txtValuesContain(values []string, expected string) bool {
	expected = strings.TrimSpace(expected)
	for _, value := range values {
		if strings.TrimSpace(value) == expected {
			return true
		}
	}
	return false
}
