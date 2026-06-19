// Copyright (c) 2026 Opensense Ltd. (Hong Kong). All rights reserved.
// Proprietary software. No use, copy, modification, distribution, disclosure,
// or reverse engineering is permitted without prior written authorization
// from Opensense Ltd.

package main

import (
	"net/http"
	"net/url"
	"strings"
)

type agentGrantAPIWriteRequest struct {
	Workspace          string `json:"workspace"`
	PolicyID           string `json:"policy_id"`
	Enabled            *bool  `json:"enabled"`
	RequireAgentMotive *bool  `json:"require_agent_motive"`
}

type agentDriveFolderGrantAPIWriteRequest struct {
	Workspace    string   `json:"workspace"`
	FolderRefIDs []string `json:"folder_ref_ids"`
}

func (a *App) handleUserAgentsAPI(w http.ResponseWriter, r *http.Request) {
	user := a.requireUserBackendAPIUser(w, r)
	if user == nil {
		return
	}
	if r.URL.Path != "/api/user/agents" {
		writeError(w, http.StatusNotFound, "not_found", "agent endpoint not found")
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET required")
		return
	}
	agents, err := a.store.ListAgents(user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	items := make([]map[string]any, 0, len(agents))
	for i := range agents {
		items = append(items, a.agentAPIView(&agents[i]))
	}
	writeJSON(w, http.StatusOK, map[string]any{"agents": items})
}

func (a *App) handleUserAgentAPI(w http.ResponseWriter, r *http.Request) {
	user := a.requireUserBackendAPIUser(w, r)
	if user == nil {
		return
	}
	agentID, action, subaction, ok := parseUserAgentAPIPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "agent endpoint not found")
		return
	}
	agent, err := a.store.GetAgent(user.ID, agentID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	if agent == nil {
		writeError(w, http.StatusNotFound, "agent_not_found", "agent not found")
		return
	}
	if action != "grants" {
		writeError(w, http.StatusNotFound, "not_found", "agent endpoint not found")
		return
	}
	if subaction == "drive-folders" {
		switch r.Method {
		case http.MethodGet:
			a.handleUserAgentDriveFolderGrantsAPIGet(w, r, user, agent)
		case http.MethodPut, http.MethodPatch:
			a.handleUserAgentDriveFolderGrantsAPIUpdate(w, r, user, agent)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET, PUT, or PATCH required")
		}
		return
	}
	if subaction != "" {
		writeError(w, http.StatusNotFound, "not_found", "agent endpoint not found")
		return
	}
	switch r.Method {
	case http.MethodGet:
		a.handleUserAgentGrantsAPIGet(w, r, user, agent)
	case http.MethodPut, http.MethodPatch:
		a.handleUserAgentGrantsAPIUpdate(w, r, user, agent)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET, PUT, or PATCH required")
	}
}

func parseUserAgentAPIPath(path string) (agentID, action, subaction string, ok bool) {
	tail := strings.Trim(strings.TrimPrefix(path, "/api/user/agents/"), "/")
	if tail == "" || tail == path {
		return "", "", "", false
	}
	parts := strings.Split(tail, "/")
	if len(parts) != 2 && len(parts) != 3 {
		return "", "", "", false
	}
	if parts[1] != "grants" {
		return "", "", "", false
	}
	rawID, err := url.PathUnescape(parts[0])
	if err != nil || strings.TrimSpace(rawID) == "" {
		return "", "", "", false
	}
	if len(parts) == 3 {
		subaction = strings.TrimSpace(parts[2])
	}
	return strings.TrimSpace(rawID), parts[1], subaction, true
}

func (a *App) handleUserAgentGrantsAPIGet(w http.ResponseWriter, r *http.Request, user *User, agent *AgentAccess) {
	grants, err := a.store.ListAgentWorkspaceGrants(user.ID, agent.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	workspaceSelector := strings.TrimSpace(r.URL.Query().Get("workspace"))
	if workspaceSelector != "" {
		conn, err := a.store.ResolveGmailConnection(user.ID, workspaceSelector)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "db_error", err.Error())
			return
		}
		if conn == nil {
			writeError(w, http.StatusNotFound, "workspace_not_found", "workspace not found")
			return
		}
		for i := range grants {
			if normalizeEmail(grants[i].MailboxEmail) == normalizeEmail(conn.MailboxEmail) {
				writeJSON(w, http.StatusOK, map[string]any{"grant": a.agentGrantAPIView(&grants[i])})
				return
			}
		}
		writeError(w, http.StatusNotFound, "agent_grant_not_found", "agent Workspace grant not found")
		return
	}
	items := make([]map[string]any, 0, len(grants))
	for i := range grants {
		items = append(items, a.agentGrantAPIView(&grants[i]))
	}
	writeJSON(w, http.StatusOK, map[string]any{"agent": a.agentAPIView(agent), "grants": items})
}

func (a *App) handleUserAgentGrantsAPIUpdate(w http.ResponseWriter, r *http.Request, user *User, agent *AgentAccess) {
	var req agentGrantAPIWriteRequest
	if !a.decodePolicyAPIJSON(w, r, &req, true) {
		return
	}
	conn, err := a.store.ResolveGmailConnection(user.ID, req.Workspace)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	if conn == nil {
		writeError(w, http.StatusNotFound, "workspace_not_found", "workspace not found")
		return
	}
	policyID := strings.TrimSpace(req.PolicyID)
	if policyID == "" {
		policyID = systemPolicyID
	}
	if err := a.ensurePolicyCanBeApplied(user.ID, policyID); err != nil {
		writePolicyAPIStoreError(w, err)
		return
	}
	grants, err := a.store.ListAgentWorkspaceGrants(user.ID, agent.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	next := make([]AgentWorkspaceGrant, 0, len(grants)+1)
	var oldPolicyID string
	found := false
	for _, grant := range grants {
		if normalizeEmail(grant.MailboxEmail) == normalizeEmail(conn.MailboxEmail) {
			oldPolicyID = grant.PolicyID
			found = true
			if enabled {
				grant.PolicyID = policyID
				if req.RequireAgentMotive != nil {
					grant.RequireAgentMotive = *req.RequireAgentMotive
				}
				next = append(next, grant)
			}
			continue
		}
		next = append(next, grant)
	}
	if !found && enabled {
		next = append(next, AgentWorkspaceGrant{
			AgentID:            agent.ID,
			UserID:             user.ID,
			MailboxEmail:       conn.MailboxEmail,
			PolicyID:           policyID,
			RequireAgentMotive: req.RequireAgentMotive != nil && *req.RequireAgentMotive,
		})
	}
	if err := a.store.SaveAgentWorkspaceGrants(user.ID, agent.ID, next); err != nil {
		writePolicyAPIStoreError(w, err)
		return
	}
	_ = a.store.MarkAgentSkillStale(user.ID, agent.ID, "Workspace grants changed.")
	action := "agent_grant_updated"
	if !enabled {
		action = "agent_grant_removed"
	}
	a.logUserAudit(r, user, action, agent.ID, agent.FriendlyName, map[string]any{
		"agent_id":             agent.ID,
		"workspace_email":      conn.MailboxEmail,
		"old_policy_id":        oldPolicyID,
		"new_policy_id":        policyID,
		"require_agent_motive": req.RequireAgentMotive != nil && *req.RequireAgentMotive,
		"enabled":              enabled,
		"source":               "api",
	})
	if !enabled {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":    "ok",
			"removed":   true,
			"agent":     a.agentAPIView(agent),
			"workspace": a.workspaceAPIView(user.ID, conn),
		})
		return
	}
	grant, _ := a.store.GetAgentWorkspaceGrant(user.ID, agent.ID, conn.MailboxEmail)
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"agent":  a.agentAPIView(agent),
		"grant":  a.agentGrantAPIView(grant),
	})
}

func (a *App) handleUserAgentDriveFolderGrantsAPIGet(w http.ResponseWriter, r *http.Request, user *User, agent *AgentAccess) {
	conn, grant := a.resolveAgentDriveFolderGrantWorkspace(w, user, agent, strings.TrimSpace(r.URL.Query().Get("workspace")))
	if conn == nil {
		return
	}
	registeredFolders, allowedFolders, err := a.agentDriveFolderGrantViews(user.ID, agent.ID, conn.MailboxEmail)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"agent":                 a.agentAPIView(agent),
		"workspace":             a.workspaceAPIView(user.ID, conn),
		"grant":                 a.agentGrantAPIView(grant),
		"allowed_drive_folders": allowedFolders,
		"drive_folders":         registeredFolders,
	})
}

func (a *App) handleUserAgentDriveFolderGrantsAPIUpdate(w http.ResponseWriter, r *http.Request, user *User, agent *AgentAccess) {
	var req agentDriveFolderGrantAPIWriteRequest
	if !a.decodePolicyAPIJSON(w, r, &req, true) {
		return
	}
	conn, grant := a.resolveAgentDriveFolderGrantWorkspace(w, user, agent, req.Workspace)
	if conn == nil {
		return
	}
	if err := a.store.SaveAgentDriveFolderGrants(user.ID, agent.ID, conn.MailboxEmail, req.FolderRefIDs); err != nil {
		writePolicyAPIStoreError(w, err)
		return
	}
	_ = a.store.MarkAgentSkillStale(user.ID, agent.ID, "Allowed Drive folder access changed.")
	registeredFolders, allowedFolders, err := a.agentDriveFolderGrantViews(user.ID, agent.ID, conn.MailboxEmail)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	a.logUserAudit(r, user, "agent_drive_folder_grants_updated", agent.ID, agent.FriendlyName, map[string]any{
		"agent_id":              agent.ID,
		"workspace":             conn.MailboxEmail,
		"allowed_drive_folders": len(allowedFolders),
		"source":                "api",
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"status":                "ok",
		"agent":                 a.agentAPIView(agent),
		"workspace":             a.workspaceAPIView(user.ID, conn),
		"grant":                 a.agentGrantAPIView(grant),
		"allowed_drive_folders": allowedFolders,
		"drive_folders":         registeredFolders,
	})
}

func (a *App) resolveAgentDriveFolderGrantWorkspace(w http.ResponseWriter, user *User, agent *AgentAccess, workspaceSelector string) (*GmailConnection, *AgentWorkspaceGrant) {
	if strings.TrimSpace(workspaceSelector) == "" {
		writeError(w, http.StatusBadRequest, "workspace_required", "workspace is required")
		return nil, nil
	}
	conn, err := a.store.ResolveGmailConnection(user.ID, workspaceSelector)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return nil, nil
	}
	if conn == nil {
		writeError(w, http.StatusNotFound, "workspace_not_found", "workspace not found")
		return nil, nil
	}
	grant, err := a.store.GetAgentWorkspaceGrant(user.ID, agent.ID, conn.MailboxEmail)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return nil, nil
	}
	if grant == nil {
		return conn, nil
	}
	return conn, grant
}

func (a *App) agentDriveFolderGrantViews(userID, agentID, mailboxEmail string) ([]map[string]any, []map[string]any, error) {
	folders, err := a.store.ListDriveFolderRefs(userID, mailboxEmail)
	if err != nil {
		return nil, nil, err
	}
	grants, err := a.store.ListAgentDriveFolderGrants(userID, agentID, mailboxEmail)
	if err != nil {
		return nil, nil, err
	}
	allowedIDs := map[string]bool{}
	for _, grant := range grants {
		allowedIDs[grant.FolderRefID] = true
	}
	registeredFolders := make([]map[string]any, 0, len(folders))
	allowedFolders := []map[string]any{}
	for _, folder := range folders {
		view := agentDriveFolderAPIView(folder, allowedIDs[folder.ID])
		registeredFolders = append(registeredFolders, view)
		if allowedIDs[folder.ID] {
			allowedFolders = append(allowedFolders, view)
		}
	}
	return registeredFolders, allowedFolders, nil
}

func agentDriveFolderAPIView(folder AllowedDriveFolder, allowed bool) map[string]any {
	return map[string]any{
		"id":             folder.ID,
		"reference_name": folder.ReferenceName,
		"reference_key":  folder.ReferenceKey,
		"folder_name":    folder.FolderName,
		"folder_id":      folder.FolderID,
		"folder_url":     folder.FolderURL,
		"allowed":        allowed,
	}
}

func (a *App) agentAPIView(agent *AgentAccess) map[string]any {
	if agent == nil {
		return map[string]any{}
	}
	firewallRules, _ := a.store.ListAgentFirewallRules(agent.UserID, agent.ID)
	firewallRuleViews := make([]map[string]any, 0, len(firewallRules))
	for i := range firewallRules {
		firewallRuleViews = append(firewallRuleViews, a.agentFirewallRuleAPIView(&firewallRules[i]))
	}
	return map[string]any{
		"id":                    agent.ID,
		"name":                  agent.FriendlyName,
		"default_location":      agent.DefaultLocation,
		"enabled":               agent.Enabled,
		"firewall_enabled":      agent.FirewallEnabled,
		"firewall_rules":        firewallRuleViews,
		"token_hint":            a.displayAgentTokenHint(*agent),
		"skill_update_required": agent.SkillStale,
		"skill_update_reason":   agent.SkillStaleReason,
		"skill_update_reasons":  splitSkillStaleReasons(agent.SkillStaleReason),
		"last_used_at":          agent.LastUsedAt,
		"created_at":            agent.CreatedAt,
		"updated_at":            agent.UpdatedAt,
	}
}

func (a *App) agentGrantAPIView(grant *AgentWorkspaceGrant) map[string]any {
	if grant == nil {
		return map[string]any{}
	}
	policyID := strings.TrimSpace(grant.PolicyID)
	if policyID == "" {
		policyID = systemPolicyID
	}
	return map[string]any{
		"agent_id":             grant.AgentID,
		"workspace_email":      grant.MailboxEmail,
		"policy_id":            policyID,
		"policy_name":          a.policyDisplayNameForUser(grant.UserID, policyID),
		"require_agent_motive": grant.RequireAgentMotive,
		"created_at":           grant.CreatedAt,
		"updated_at":           grant.UpdatedAt,
	}
}
