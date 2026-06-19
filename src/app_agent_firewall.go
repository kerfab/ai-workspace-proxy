// Copyright (c) 2026 Opensense Ltd. (Hong Kong). All rights reserved.
// Proprietary software. No use, copy, modification, distribution, disclosure,
// or reverse engineering is permitted without prior written authorization
// from Opensense Ltd.

package main

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

func parseAgentFirewallValue(raw string) (value string, ipVersion string, addressKind string, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", "", fmt.Errorf("IP address is required")
	}
	if strings.Contains(raw, "/") {
		prefix, parseErr := netip.ParsePrefix(raw)
		if parseErr != nil {
			return "", "", "", fmt.Errorf("enter a valid IPv4 or IPv6 address, optionally using CIDR notation")
		}
		prefix = prefix.Masked()
		if prefix.Addr().Is4() {
			ipVersion = "IPv4"
		} else {
			ipVersion = "IPv6"
		}
		return prefix.String(), ipVersion, "Range", nil
	}
	addr, parseErr := netip.ParseAddr(raw)
	if parseErr != nil {
		return "", "", "", fmt.Errorf("enter a valid IPv4 or IPv6 address, optionally using CIDR notation")
	}
	if addr.Is4() {
		ipVersion = "IPv4"
	} else {
		ipVersion = "IPv6"
	}
	return addr.String(), ipVersion, "Address", nil
}

func directRequesterIP(r *http.Request) (netip.Addr, error) {
	host := strings.TrimSpace(r.RemoteAddr)
	if host == "" {
		return netip.Addr{}, fmt.Errorf("requester IP address is unavailable")
	}
	if splitHost, _, err := net.SplitHostPort(host); err == nil {
		host = splitHost
	}
	host = strings.Trim(host, "[]")
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("requester IP address is unavailable")
	}
	return addr, nil
}

func firewallRuleContains(rule AgentFirewallRule, addr netip.Addr) bool {
	value := strings.TrimSpace(rule.Value)
	if value == "" {
		return false
	}
	if strings.Contains(value, "/") {
		prefix, err := netip.ParsePrefix(value)
		return err == nil && prefix.Contains(addr)
	}
	ruleAddr, err := netip.ParseAddr(value)
	return err == nil && ruleAddr == addr
}

func (a *App) agentFirewallAllows(userID string, agent *AgentAccess, addr netip.Addr) (bool, error) {
	if agent == nil || !agent.FirewallEnabled {
		return true, nil
	}
	rules, err := a.store.ListAgentFirewallRules(userID, agent.ID)
	if err != nil {
		return false, err
	}
	for _, rule := range rules {
		if firewallRuleContains(rule, addr) {
			return true, nil
		}
	}
	return false, nil
}

func (a *App) denyIfAgentFirewallBlocks(w http.ResponseWriter, r *http.Request, requestID string, user *User, agent *AgentAccess, service string) bool {
	if agent == nil || !agent.FirewallEnabled {
		return false
	}
	ctx, _ := agentContextForAgentRequest(r, agent, false)
	logEntry := a.baseRequestLogEntry(r, requestID, user, agent.ID, ctx)
	logEntry.Service = strings.TrimSpace(service)
	if logEntry.Service == "" {
		logEntry.Service = inferServiceLabelFromPath(r.URL.Path)
	}
	addr, err := directRequesterIP(r)
	if err != nil {
		_ = a.store.IncrementDailyStat(user.ID, false)
		logEntry.Outcome = "Fail"
		logEntry.HTTPStatus = http.StatusForbidden
		logEntry.ErrorMessage = err.Error()
		a.saveRequestLog(logEntry)
		writeError(w, http.StatusForbidden, "agent_firewall_denied", err.Error())
		return true
	}
	allowed, err := a.agentFirewallAllows(user.ID, agent, addr)
	if err != nil {
		logEntry.Outcome = "Fail"
		logEntry.HTTPStatus = http.StatusInternalServerError
		logEntry.ErrorMessage = err.Error()
		a.saveRequestLog(logEntry)
		writeError(w, http.StatusInternalServerError, "agent_firewall_error", err.Error())
		return true
	}
	if allowed {
		return false
	}
	message := fmt.Sprintf("agent firewall denied request from %s", addr.String())
	_ = a.store.IncrementDailyStat(user.ID, false)
	logEntry.Outcome = "Fail"
	logEntry.HTTPStatus = http.StatusForbidden
	logEntry.ErrorMessage = message
	a.saveRequestLog(logEntry)
	writeError(w, http.StatusForbidden, "agent_firewall_denied", message)
	return true
}

func (a *App) handleAgentFirewallToggle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST required")
		return
	}
	user := a.requireSessionUser(w, r)
	if user == nil || !a.requireSessionCSRF(w, r) {
		return
	}
	agentID := strings.TrimSpace(r.FormValue("agent_id"))
	agent, err := a.store.GetAgent(user.ID, agentID)
	if err != nil || agent == nil {
		writeError(w, http.StatusNotFound, "agent_not_found", firstErr(err, "agent not found").Error())
		return
	}
	enabled := r.FormValue("enabled") == "1"
	if err := a.store.SetAgentFirewallEnabled(user.ID, agent.ID, enabled); err != nil {
		writeError(w, http.StatusInternalServerError, "firewall_error", err.Error())
		return
	}
	a.logUserAudit(r, user, "agent_firewall_status_updated", agent.ID, agent.FriendlyName, map[string]any{
		"agent_id": agent.ID,
		"enabled":  enabled,
		"source":   requestSource(r),
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"status":           "ok",
		"message":          "Firewall settings updated.",
		"agent_id":         agent.ID,
		"firewall_enabled": enabled,
	})
}

func (a *App) handleAgentFirewallAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST required")
		return
	}
	user := a.requireSessionUser(w, r)
	if user == nil || !a.requireSessionCSRF(w, r) {
		return
	}
	agentID := strings.TrimSpace(r.FormValue("agent_id"))
	agent, err := a.store.GetAgent(user.ID, agentID)
	if err != nil || agent == nil {
		writeError(w, http.StatusNotFound, "agent_not_found", firstErr(err, "agent not found").Error())
		return
	}
	value, ipVersion, addressKind, err := parseAgentFirewallValue(r.FormValue("ip_address"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_ip_address", err.Error())
		return
	}
	existing, err := a.store.ListAgentFirewallRules(user.ID, agent.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "firewall_error", err.Error())
		return
	}
	for _, rule := range existing {
		if rule.Value == value {
			writeError(w, http.StatusBadRequest, "duplicate_ip_address", "this IP address is already allowed for this agent")
			return
		}
	}
	id, err := RandomToken("afw_", 12)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "firewall_error", err.Error())
		return
	}
	rule := &AgentFirewallRule{
		ID:          id,
		AgentID:     agent.ID,
		UserID:      user.ID,
		Value:       value,
		IPVersion:   ipVersion,
		AddressKind: addressKind,
	}
	if err := a.store.AddAgentFirewallRule(rule); err != nil {
		writeError(w, http.StatusBadRequest, "firewall_error", err.Error())
		return
	}
	a.logUserAudit(r, user, "agent_firewall_address_added", agent.ID, agent.FriendlyName, map[string]any{
		"agent_id":     agent.ID,
		"value":        value,
		"ip_version":   ipVersion,
		"address_kind": addressKind,
		"source":       requestSource(r),
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"status":       "ok",
		"message":      "Firewall address added.",
		"agent_id":     agent.ID,
		"rule":         a.agentFirewallRuleAPIView(rule),
		"rule_count":   len(existing) + 1,
		"firewall_row": a.agentFirewallRuleAPIView(rule),
	})
}

func (a *App) handleAgentFirewallDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST required")
		return
	}
	user := a.requireSessionUser(w, r)
	if user == nil || !a.requireSessionCSRF(w, r) {
		return
	}
	agentID := strings.TrimSpace(r.FormValue("agent_id"))
	ruleID := strings.TrimSpace(r.FormValue("rule_id"))
	agent, err := a.store.GetAgent(user.ID, agentID)
	if err != nil || agent == nil {
		writeError(w, http.StatusNotFound, "agent_not_found", firstErr(err, "agent not found").Error())
		return
	}
	rules, err := a.store.ListAgentFirewallRules(user.ID, agent.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "firewall_error", err.Error())
		return
	}
	var deleted *AgentFirewallRule
	for i := range rules {
		if rules[i].ID == ruleID {
			deleted = &rules[i]
			break
		}
	}
	if deleted == nil {
		writeError(w, http.StatusNotFound, "firewall_rule_not_found", "firewall address not found")
		return
	}
	if err := a.store.DeleteAgentFirewallRule(user.ID, agent.ID, ruleID); err != nil {
		writeError(w, http.StatusInternalServerError, "firewall_error", err.Error())
		return
	}
	a.logUserAudit(r, user, "agent_firewall_address_deleted", agent.ID, agent.FriendlyName, map[string]any{
		"agent_id": agent.ID,
		"rule_id":  deleted.ID,
		"value":    deleted.Value,
		"source":   requestSource(r),
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"status":     "ok",
		"message":    "Firewall address removed.",
		"agent_id":   agent.ID,
		"rule_id":    ruleID,
		"rule_count": len(rules) - 1,
	})
}

func (a *App) agentFirewallRuleAPIView(rule *AgentFirewallRule) map[string]any {
	if rule == nil {
		return nil
	}
	return map[string]any{
		"id":           rule.ID,
		"type":         rule.IPVersion,
		"address_kind": rule.AddressKind,
		"value":        rule.Value,
		"date_added":   rule.CreatedAt.UTC().Format("2006-01-02 15:04:05 UTC"),
	}
}
