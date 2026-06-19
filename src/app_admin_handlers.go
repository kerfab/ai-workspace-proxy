// Copyright (c) 2026 Opensense Ltd. (Hong Kong). All rights reserved.
// Proprietary software. No use, copy, modification, distribution, disclosure,
// or reverse engineering is permitted without prior written authorization
// from Opensense Ltd.

package main

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

func adminUserRoleLabel(user User) string {
	if user.IsAdmin {
		return "Administrator"
	}
	return "User"
}

func adminUserStatusView(user User) (label, key, className string) {
	if user.IsSuspended {
		return "Suspended", "suspended", "admin-user-status admin-user-status-suspended"
	}
	return "Active", "active", "admin-user-status admin-user-status-active"
}

func adminSuspendedByDisplay(actor *User) string {
	if actor == nil {
		return "by a former admin account"
	}
	name := strings.TrimSpace(actor.Name)
	email := strings.TrimSpace(actor.Email)
	switch {
	case name != "" && email != "":
		return "by " + name + " (" + email + ")"
	case email != "":
		return "by " + email
	default:
		return "by a former admin account"
	}
}

func organizationPolicyRiskOptions(selected int) []map[string]any {
	options := []struct {
		value int
		label string
	}{
		{1, "Low risk permissions only"},
		{2, "Low & Medium risk permissions only"},
		{3, "Any permission"},
	}
	out := make([]map[string]any, 0, len(options))
	for _, option := range options {
		out = append(out, map[string]any{
			"Value":    option.value,
			"Label":    option.label,
			"Selected": selected == option.value,
		})
	}
	return out
}

func firewallRulesText(rules []OrganizationFirewallRule) string {
	values := make([]string, 0, len(rules))
	for _, rule := range rules {
		values = append(values, rule.Value)
	}
	return strings.Join(values, "\n")
}

func (a *App) organizationAccountsPolicyView(admin *User) (map[string]any, error) {
	policy, err := a.store.GetOrganizationAccountsPolicy(admin.OrganizationID)
	if err != nil {
		return nil, err
	}
	rules, err := a.store.ListOrganizationFirewallRules(admin.OrganizationID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"Policy":                     policy,
		"RiskOptions":                organizationPolicyRiskOptions(policy.CustomPolicyMaxRisk),
		"ApprovedDomainsText":        strings.Join(policy.ApprovedWorkspaceDomains, "\n"),
		"ExternalConnectorsText":     strings.Join(policy.AllowedExternalWorkspaceConnectors, "\n"),
		"BlockedWorkspaceEmailsText": strings.Join(policy.BlockedWorkspaceEmails, "\n"),
		"AllowedWorkspaceEmailsText": strings.Join(policy.AllowedWorkspaceEmails, "\n"),
		"FirewallRules":              rules,
		"FirewallRulesText":          firewallRulesText(rules),
		"FirewallRuleCount":          len(rules),
		"Error":                      "",
		"Saved":                      false,
	}, nil
}

func (a *App) adminUserRows(admin *User) ([]map[string]any, error) {
	if admin == nil {
		return nil, nil
	}
	summaries, err := a.store.ListOrganizationUserSummaries(admin.OrganizationID)
	if err != nil {
		return nil, err
	}
	timezone := a.userTimezoneOrDefault(admin.ID)
	rows := make([]map[string]any, 0, len(summaries))
	for _, summary := range summaries {
		statusLabel, statusKey, statusClass := adminUserStatusView(summary.User)
		lastActivityDisplay := "No activity yet"
		lastActivityUnix := int64(0)
		if !summary.LastActivityAt.IsZero() {
			lastActivityDisplay = formatUserTime(summary.LastActivityAt, timezone)
			lastActivityUnix = summary.LastActivityAt.Unix()
		}
		rows = append(rows, map[string]any{
			"ID":                  summary.User.ID,
			"Name":                summary.User.Name,
			"Email":               summary.User.Email,
			"Role":                adminUserRoleLabel(summary.User),
			"RoleKey":             strings.ToLower(adminUserRoleLabel(summary.User)),
			"Status":              statusLabel,
			"StatusKey":           statusKey,
			"StatusClass":         statusClass,
			"LastActivityDisplay": lastActivityDisplay,
			"LastActivityUnix":    lastActivityUnix,
			"EditURL":             "/admin/users/" + summary.User.ID,
			"LogsReady":           false,
		})
	}
	return rows, nil
}

func (a *App) renderAdminShell(w http.ResponseWriter, r *http.Request, admin *User, activeSection string, extra map[string]any) {
	settings, err := a.store.GetUserSettings(admin.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "settings_error", err.Error())
		return
	}
	timezone := "UTC"
	if settings != nil && strings.TrimSpace(settings.Timezone) != "" {
		timezone = settings.Timezone
	}
	organizationAdminView, err := a.organizationAdminView(admin, timezone)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "organization_admin_error", err.Error())
		return
	}
	data := map[string]any{
		"AppName":                 a.cfg.AppName,
		"User":                    admin,
		"CSRFToken":               a.csrfTokenFromRequest(r),
		"ActiveSection":           activeSection,
		"AdminMode":               true,
		"PageTitle":               pageTitleForSection(activeSection),
		"PageSubtitle":            pageSubtitleForSection(activeSection),
		"OrganizationAdmin":       organizationAdminView,
		"ShowOrgAdmin":            activeSection == "admin",
		"ShowAdminUsers":          activeSection == "admin_users",
		"ShowAdminUser":           activeSection == "admin_user",
		"ShowAdminAccountsPolicy": activeSection == "admin_accounts_policy",
	}
	for key, value := range extra {
		data[key] = value
	}
	renderDashboardResponse(w, r, data)
}

func (a *App) handleAdminUsers(w http.ResponseWriter, r *http.Request) {
	admin := a.requireAdminUser(w, r)
	if admin == nil {
		return
	}
	rows, err := a.adminUserRows(admin)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	a.renderAdminShell(w, r, admin, "admin_users", map[string]any{
		"AdminUserRows": rows,
	})
}

func (a *App) handleAdminAccountsPolicy(w http.ResponseWriter, r *http.Request) {
	admin := a.requireAdminUser(w, r)
	if admin == nil {
		return
	}
	switch r.Method {
	case http.MethodGet:
		view, err := a.organizationAccountsPolicyView(admin)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "accounts_policy_error", err.Error())
			return
		}
		view["Error"] = r.URL.Query().Get("accounts_policy_error")
		view["Saved"] = r.URL.Query().Get("accounts_policy_saved") == "1"
		a.renderAdminShell(w, r, admin, "admin_accounts_policy", map[string]any{
			"AdminAccountsPolicy": view,
		})
	case http.MethodPost:
		a.handleAdminAccountsPolicyUpdate(w, r, admin)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET or POST required")
	}
}

func (a *App) handleAdminAccountsPolicyUpdate(w http.ResponseWriter, r *http.Request, admin *User) {
	if !a.requireSessionCSRF(w, r) {
		return
	}
	maxRisk, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("custom_policy_max_risk")))
	timeoutHours, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("session_timeout_hours")))
	policy := &OrganizationAccountsPolicy{
		OrganizationID:                         admin.OrganizationID,
		EnforceSessionTimeout:                  r.FormValue("enforce_session_timeout") == "1",
		SessionTimeoutHours:                    timeoutHours,
		RequireTwoFactor:                       r.FormValue("require_two_factor") == "1",
		AllowCustomPolicies:                    r.FormValue("allow_custom_policies") == "1",
		CustomPolicyMaxRisk:                    maxRisk,
		AllowNonAdminInvites:                   r.FormValue("allow_non_admin_invites") == "1",
		AllowExternalWorkspaceDomains:          r.FormValue("allow_external_workspace_domains") == "1",
		ApprovedWorkspaceDomains:               []string{r.FormValue("approved_workspace_domains")},
		AllowExternalUsersConnectOrgWorkspaces: r.FormValue("allow_external_users_connect_org_workspaces") == "1",
		AllowedExternalWorkspaceConnectors:     []string{r.FormValue("allowed_external_workspace_connectors")},
		BlacklistWorkspaceAccess:               r.FormValue("blacklist_workspace_access") == "1",
		BlockedWorkspaceEmails:                 []string{r.FormValue("blocked_workspace_emails")},
		DenyAllWorkspaceConnectionsExcept:      r.FormValue("deny_all_workspace_connections_except") == "1",
		AllowedWorkspaceEmails:                 []string{r.FormValue("allowed_workspace_emails")},
		EnforceFirewall:                        r.FormValue("enforce_firewall") == "1",
		AllowUserTwoFactorReset:                r.FormValue("allow_user_two_factor_reset") == "1",
	}
	if policy.BlacklistWorkspaceAccess && policy.DenyAllWorkspaceConnectionsExcept {
		http.Redirect(w, r, "/admin/accounts-policy?accounts_policy_error="+url.QueryEscape("Blacklist access and deny-all-except mode cannot be enabled together."), http.StatusFound)
		return
	}
	parsedRules := []OrganizationFirewallRule{}
	seenRules := map[string]bool{}
	for _, line := range strings.Split(r.FormValue("firewall_addresses"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		value, ipVersion, addressKind, err := parseAgentFirewallValue(line)
		if err != nil {
			http.Redirect(w, r, "/admin/accounts-policy?accounts_policy_error="+url.QueryEscape("Firewall address "+line+": "+err.Error()), http.StatusFound)
			return
		}
		if seenRules[value] {
			continue
		}
		seenRules[value] = true
		parsedRules = append(parsedRules, OrganizationFirewallRule{
			Value:       value,
			IPVersion:   ipVersion,
			AddressKind: addressKind,
		})
	}
	if policy.EnforceFirewall && len(parsedRules) == 0 {
		http.Redirect(w, r, "/admin/accounts-policy?accounts_policy_error="+url.QueryEscape("Add at least one IPv4 or IPv6 address before enforcing the organization firewall."), http.StatusFound)
		return
	}
	if err := a.store.SaveOrganizationAccountsPolicy(policy); err != nil {
		http.Redirect(w, r, "/admin/accounts-policy?accounts_policy_error="+url.QueryEscape(err.Error()), http.StatusFound)
		return
	}
	if err := a.store.ReplaceOrganizationFirewallRules(admin.OrganizationID, parsedRules); err != nil {
		http.Redirect(w, r, "/admin/accounts-policy?accounts_policy_error="+url.QueryEscape(err.Error()), http.StatusFound)
		return
	}
	a.logUserAudit(r, admin, "organization_accounts_policy_updated", admin.OrganizationID, admin.OrganizationID, map[string]any{
		"enforce_session_timeout":                     policy.EnforceSessionTimeout,
		"session_timeout_hours":                       policy.SessionTimeoutHours,
		"require_two_factor":                          policy.RequireTwoFactor,
		"allow_custom_policies":                       policy.AllowCustomPolicies,
		"custom_policy_max_risk":                      policy.CustomPolicyMaxRisk,
		"allow_non_admin_invites":                     policy.AllowNonAdminInvites,
		"allow_external_workspace_domains":            policy.AllowExternalWorkspaceDomains,
		"approved_workspace_domains":                  normalizeDomainList(policy.ApprovedWorkspaceDomains),
		"allow_external_users_connect_org_workspaces": policy.AllowExternalUsersConnectOrgWorkspaces,
		"allowed_external_workspace_connectors":       normalizeEmailOrDomainList(policy.AllowedExternalWorkspaceConnectors),
		"blacklist_workspace_access":                  policy.BlacklistWorkspaceAccess,
		"blocked_workspace_emails":                    normalizeEmailList(policy.BlockedWorkspaceEmails),
		"deny_all_workspace_connections_except":       policy.DenyAllWorkspaceConnectionsExcept,
		"allowed_workspace_emails":                    normalizeEmailList(policy.AllowedWorkspaceEmails),
		"enforce_firewall":                            policy.EnforceFirewall,
		"firewall_rule_count":                         len(parsedRules),
		"allow_user_two_factor_reset":                 policy.AllowUserTwoFactorReset,
	})
	http.Redirect(w, r, "/admin/accounts-policy?accounts_policy_saved=1", http.StatusFound)
}

func (a *App) handleAdminUserRoutes(w http.ResponseWriter, r *http.Request) {
	admin := a.requireAdminUser(w, r)
	if admin == nil {
		return
	}
	trimmed := strings.TrimPrefix(r.URL.Path, "/admin/users/")
	trimmed = strings.Trim(trimmed, "/")
	if trimmed == "" {
		writeError(w, http.StatusNotFound, "not_found", "user route not found")
		return
	}
	parts := strings.Split(trimmed, "/")
	userID := parts[0]
	target, err := a.store.FindUserByID(userID)
	if err != nil || target == nil || strings.TrimSpace(target.OrganizationID) != strings.TrimSpace(admin.OrganizationID) {
		writeError(w, http.StatusNotFound, "user_error", "user not found")
		return
	}

	if len(parts) == 1 && r.Method == http.MethodGet {
		lastActivityAt, err := a.store.GetUserLastActivityAt(userID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "db_error", err.Error())
			return
		}
		settings, err := a.store.GetUserSettings(admin.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "settings_error", err.Error())
			return
		}
		timezone := "UTC"
		if settings != nil && strings.TrimSpace(settings.Timezone) != "" {
			timezone = settings.Timezone
		}
		statusLabel, _, statusClass := adminUserStatusView(*target)
		lastActivityDisplay := "No activity yet"
		suspendedAtDisplay := ""
		if !lastActivityAt.IsZero() {
			lastActivityDisplay = formatUserTime(lastActivityAt, timezone)
		}
		suspendedByDisplay := ""
		if target.IsSuspended && !target.SuspendedAt.IsZero() {
			suspendedAtDisplay = formatUserTime(target.SuspendedAt, timezone)
			actor, err := a.store.FindUserByID(target.SuspendedByUserID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "db_error", err.Error())
				return
			}
			suspendedByDisplay = adminSuspendedByDisplay(actor)
		}
		a.renderAdminShell(w, r, admin, "admin_user", map[string]any{
			"AdminUser":                 target,
			"AdminUserCanSuspendDelete": strings.TrimSpace(admin.ID) != strings.TrimSpace(target.ID),
			"AdminUserRoleLabel":        adminUserRoleLabel(*target),
			"AdminUserStatusLabel":      statusLabel,
			"AdminUserStatusClass":      statusClass,
			"AdminUserCreatedAtLocal":   formatUserTime(target.CreatedAt, timezone),
			"AdminUserLastActivity":     lastActivityDisplay,
			"AdminUserSuspendedAtLocal": suspendedAtDisplay,
			"AdminUserSuspendedBy":      suspendedByDisplay,
		})
		return
	}

	if len(parts) == 2 && r.Method == http.MethodPost {
		if !a.requireSessionCSRF(w, r) {
			return
		}
		action := parts[1]
		switch action {
		case "suspend":
			if strings.TrimSpace(admin.ID) == strings.TrimSpace(userID) {
				writeError(w, http.StatusForbidden, "admin_action_denied", "administrators cannot suspend themselves")
				return
			}
			if err := a.store.SetUserSuspendedBy(userID, true, admin.ID); err != nil {
				writeError(w, http.StatusInternalServerError, "db_error", err.Error())
				return
			}
			http.Redirect(w, r, "/admin/users/"+userID, http.StatusFound)
			return
		case "unsuspend":
			if err := a.store.SetUserSuspendedBy(userID, false, admin.ID); err != nil {
				writeError(w, http.StatusInternalServerError, "db_error", err.Error())
				return
			}
			http.Redirect(w, r, "/admin/users/"+userID, http.StatusFound)
			return
		case "delete":
			if strings.TrimSpace(admin.ID) == strings.TrimSpace(userID) {
				writeError(w, http.StatusForbidden, "admin_action_denied", "administrators cannot delete themselves")
				return
			}
			if err := a.store.DeleteUser(userID); err != nil {
				writeError(w, http.StatusInternalServerError, "db_error", err.Error())
				return
			}
		default:
			writeError(w, http.StatusNotFound, "not_found", "admin action not found")
			return
		}
		http.Redirect(w, r, "/admin/users", http.StatusFound)
		return
	}

	writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "unsupported admin route")
}
