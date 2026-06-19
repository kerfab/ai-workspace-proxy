// Copyright (c) 2026 Opensense Ltd. (Hong Kong). All rights reserved.
// Proprietary software. No use, copy, modification, distribution, disclosure,
// or reverse engineering is permitted without prior written authorization
// from Opensense Ltd.

package main

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

func (a *App) organizationAccountsPolicyForUser(user *User) (*OrganizationAccountsPolicy, error) {
	if user == nil || strings.TrimSpace(user.OrganizationID) == "" {
		return nil, nil
	}
	return a.store.GetOrganizationAccountsPolicy(user.OrganizationID)
}

func (a *App) organizationRequiresUserTwoFactor(user *User) (bool, error) {
	policy, err := a.organizationAccountsPolicyForUser(user)
	if err != nil || policy == nil {
		return false, err
	}
	return policy.RequireTwoFactor, nil
}

func (a *App) userMustCompleteOrganizationTwoFactor(user *User) (bool, error) {
	required, err := a.organizationRequiresUserTwoFactor(user)
	if err != nil || !required {
		return false, err
	}
	settings, err := a.store.GetUserTwoFactorSettings(user.ID)
	if err != nil {
		return false, err
	}
	return settings == nil || !settings.Enabled, nil
}

func pathAllowsOrganizationTwoFactorEnrollment(path string) bool {
	return path == "/settings/2fa/enroll" || strings.HasPrefix(path, "/settings/2fa")
}

func twoFactorEnrollmentURLForRequest(r *http.Request) string {
	back := "/"
	if r != nil && r.URL != nil {
		back = safeRelativeRedirectPath(r.URL.RequestURI())
		if back == "" || strings.HasPrefix(back, "/settings/2fa/enroll") || strings.HasPrefix(back, "/settings/2fa/") {
			back = "/"
		}
	}
	return "/settings/2fa/enroll?back=" + url.QueryEscape(back)
}

func emailDomain(email string) string {
	parts := strings.Split(normalizeEmail(email), "@")
	if len(parts) != 2 {
		return ""
	}
	return normalizeDomain(parts[1])
}

func listContains(values []string, value string) bool {
	value = strings.TrimSpace(value)
	for _, item := range values {
		if strings.TrimSpace(item) == value {
			return true
		}
	}
	return false
}

func connectorListAllowsEmail(values []string, email string) bool {
	email = normalizeEmail(email)
	domain := emailDomain(email)
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if strings.Contains(value, "@") && normalizeEmail(value) == email {
			return true
		}
		if !strings.Contains(value, "@") && normalizeDomain(value) == domain {
			return true
		}
	}
	return false
}

func (a *App) validateWorkspaceConnectionAgainstOrganizationPolicies(user *User, workspaceEmail string) error {
	if user == nil {
		return fmt.Errorf("user is required")
	}
	workspaceEmail = normalizeEmail(workspaceEmail)
	workspaceDomain := emailDomain(workspaceEmail)
	userDomain := organizationDomainForEmail(user.Email)

	if userDomain != "" && workspaceDomain != "" && workspaceDomain != userDomain {
		userOrgPolicy, err := a.store.GetOrganizationAccountsPolicy(user.OrganizationID)
		if err != nil {
			return err
		}
		if userOrgPolicy != nil {
			allowedDomain := listContains(userOrgPolicy.ApprovedWorkspaceDomains, workspaceDomain)
			if !userOrgPolicy.AllowExternalWorkspaceDomains || !allowedDomain {
				return fmt.Errorf("your organization policy does not allow connecting Google Workspaces under %s", workspaceDomain)
			}
		}
	}

	workspaceOrg, err := a.store.FindOrganizationByDomain(workspaceDomain)
	if err != nil {
		return err
	}
	if workspaceOrg == nil {
		return nil
	}
	workspacePolicy, err := a.store.GetOrganizationAccountsPolicy(workspaceOrg.ID)
	if err != nil {
		return err
	}
	if workspacePolicy == nil {
		return nil
	}
	if workspacePolicy.BlacklistWorkspaceAccess && listContains(workspacePolicy.BlockedWorkspaceEmails, workspaceEmail) {
		return fmt.Errorf("the administrator for %s blocked this Workspace account from being connected to the proxy", workspaceDomain)
	}
	if workspacePolicy.DenyAllWorkspaceConnectionsExcept && !listContains(workspacePolicy.AllowedWorkspaceEmails, workspaceEmail) {
		return fmt.Errorf("the administrator for %s only allows approved Workspace accounts to be connected to the proxy", workspaceDomain)
	}
	if strings.TrimSpace(user.OrganizationID) != strings.TrimSpace(workspaceOrg.ID) {
		if !workspacePolicy.AllowExternalUsersConnectOrgWorkspaces {
			return fmt.Errorf("the administrator for %s does not allow users from other domains to connect Workspaces under this domain", workspaceDomain)
		}
		if !connectorListAllowsEmail(workspacePolicy.AllowedExternalWorkspaceConnectors, user.Email) {
			return fmt.Errorf("the administrator for %s has not allowed your account or email domain to connect Workspaces under this domain", workspaceDomain)
		}
	}
	return nil
}
