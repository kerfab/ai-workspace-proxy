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

func (a *App) handleAgentCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST required")
		return
	}
	user := a.requireSessionUser(w, r)
	if user == nil || !a.requireSessionCSRF(w, r) {
		return
	}
	_, enc, hint, err := a.newAgentToken()
	if err != nil {
		redirectAgentError(w, r, "", err)
		return
	}
	id, err := RandomToken("agt_", 12)
	if err != nil {
		redirectAgentError(w, r, "", err)
		return
	}
	agent := &AgentAccess{
		ID:              id,
		UserID:          user.ID,
		FriendlyName:    strings.TrimSpace(r.FormValue("friendly_name")),
		DefaultLocation: strings.TrimSpace(r.FormValue("default_location")),
		TokenEnc:        enc,
		TokenHint:       hint,
		Enabled:         true,
	}
	if err := a.store.SaveAgent(agent); err != nil {
		redirectAgentError(w, r, "", err)
		return
	}
	a.logUserAudit(r, user, "agent_created", agent.ID, agent.FriendlyName, map[string]any{
		"agent_id": agent.ID,
		"source":   requestSource(r),
	})
	http.Redirect(w, r, "/agents?agent="+url.QueryEscape(agent.ID)+"#agent-"+url.QueryEscape(agent.ID), http.StatusFound)
}

func (a *App) handleAgentUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST required")
		return
	}
	user := a.requireSessionUser(w, r)
	if user == nil || !a.requireSessionCSRF(w, r) {
		return
	}
	agentID := strings.TrimSpace(r.FormValue("agent_id"))
	field := strings.TrimSpace(r.FormValue("field"))
	value := strings.TrimSpace(r.FormValue("value"))
	agent, err := a.store.GetAgent(user.ID, agentID)
	if err != nil || agent == nil {
		redirectAgentError(w, r, agentID, firstErr(err, "agent not found"))
		return
	}
	oldName := agent.FriendlyName
	oldLocation := agent.DefaultLocation
	switch field {
	case "friendly_name":
		agent.FriendlyName = value
	case "default_location":
		agent.DefaultLocation = value
	default:
		redirectAgentError(w, r, agentID, errString("unknown agent profile field"))
		return
	}
	if err := a.store.SaveAgent(agent); err != nil {
		redirectAgentError(w, r, agentID, err)
		return
	}
	// Agent name/location are kept server-side, but they affect generated guidance and audit context.
	_ = a.store.MarkAgentSkillStale(user.ID, agent.ID, "Agent profile changed.")
	a.logUserAudit(r, user, "agent_profile_updated", agent.ID, agent.FriendlyName, map[string]any{
		"agent_id":     agent.ID,
		"field":        field,
		"old_name":     oldName,
		"new_name":     agent.FriendlyName,
		"old_location": oldLocation,
		"new_location": agent.DefaultLocation,
		"source":       requestSource(r),
	})
	message := "Agent profile updated."
	if field == "friendly_name" {
		message = "Agent renamed."
	}
	if field == "default_location" {
		message = "Agent location updated."
	}
	if wantsJSON(r) {
		payload := map[string]any{
			"status":           "ok",
			"message":          message,
			"agent_id":         agent.ID,
			"friendly_name":    agent.FriendlyName,
			"default_location": agent.DefaultLocation,
			"field":            field,
		}
		a.addAgentSkillWarningToPayload(user.ID, agent.ID, payload)
		writeJSON(w, http.StatusOK, payload)
		return
	}
	redirectAgentSaved(w, r, agent.ID)
}

func (a *App) handleAgentGrantsUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST required")
		return
	}
	user := a.requireSessionUser(w, r)
	if user == nil || !a.requireSessionCSRF(w, r) {
		return
	}
	agentID := strings.TrimSpace(r.FormValue("agent_id"))
	grants := []AgentWorkspaceGrant{}
	selected := map[string]bool{}
	for _, workspace := range r.Form["workspace"] {
		selected[normalizeEmail(workspace)] = true
	}
	for workspace := range selected {
		policyID := strings.TrimSpace(r.FormValue("policy_" + formKey(workspace)))
		grants = append(grants, AgentWorkspaceGrant{
			AgentID:            agentID,
			UserID:             user.ID,
			MailboxEmail:       workspace,
			PolicyID:           policyID,
			RequireAgentMotive: r.FormValue("require_agent_motive_"+formKey(workspace)) != "",
		})
	}
	if err := a.store.SaveAgentWorkspaceGrants(user.ID, agentID, grants); err != nil {
		redirectAgentError(w, r, agentID, err)
		return
	}
	_ = a.store.MarkAgentSkillStale(user.ID, agentID, "Workspace grants changed.")
	agent, _ := a.store.GetAgent(user.ID, agentID)
	targetName := agentID
	if agent != nil {
		targetName = agent.FriendlyName
	}
	a.logUserAudit(r, user, "agent_grants_updated", agentID, targetName, map[string]any{
		"agent_id":    agentID,
		"grant_count": len(grants),
		"source":      requestSource(r),
	})
	if wantsJSON(r) {
		payload := map[string]any{
			"status":      "ok",
			"message":     "Workspace grants updated.",
			"agent_id":    agentID,
			"grant_count": len(grants),
		}
		a.addAgentSkillWarningToPayload(user.ID, agentID, payload)
		writeJSON(w, http.StatusOK, payload)
		return
	}
	redirectAgentSaved(w, r, agentID)
}

func (a *App) handleAgentDriveFolderGrantsUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST required")
		return
	}
	user := a.requireSessionUser(w, r)
	if user == nil || !a.requireSessionCSRF(w, r) {
		return
	}
	agentID := strings.TrimSpace(r.FormValue("agent_id"))
	workspace := normalizeEmail(r.FormValue("workspace"))
	folderRefIDs := r.Form["folder_ref_id"]
	if err := a.store.SaveAgentDriveFolderGrants(user.ID, agentID, workspace, folderRefIDs); err != nil {
		if wantsJSON(r) {
			writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": err.Error()})
			return
		}
		redirectAgentError(w, r, agentID, err)
		return
	}
	_ = a.store.MarkAgentSkillStale(user.ID, agentID, "Allowed Drive folder access changed.")
	savedGrants, _ := a.store.ListAgentDriveFolderGrants(user.ID, agentID, workspace)
	agent, _ := a.store.GetAgent(user.ID, agentID)
	targetName := agentID
	if agent != nil {
		targetName = agent.FriendlyName
	}
	a.logUserAudit(r, user, "agent_drive_folder_grants_updated", agentID, targetName, map[string]any{
		"agent_id":              agentID,
		"workspace":             workspace,
		"allowed_drive_folders": len(savedGrants),
		"source":                requestSource(r),
	})
	if wantsJSON(r) {
		payload := map[string]any{
			"status":        "ok",
			"message":       "Allowed Drive folders updated.",
			"agent_id":      agentID,
			"workspace":     workspace,
			"allowed_count": len(savedGrants),
		}
		a.addAgentSkillWarningToPayload(user.ID, agentID, payload)
		writeJSON(w, http.StatusOK, payload)
		return
	}
	redirectAgentSaved(w, r, agentID)
}

func (a *App) handleAgentRotate(w http.ResponseWriter, r *http.Request) {
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
		redirectAgentError(w, r, agentID, firstErr(err, "agent not found"))
		return
	}
	_, enc, hint, err := a.newAgentToken()
	if err != nil {
		redirectAgentError(w, r, agentID, err)
		return
	}
	agent.TokenEnc = enc
	agent.TokenHint = hint
	if err := a.store.SaveAgent(agent); err != nil {
		redirectAgentError(w, r, agentID, err)
		return
	}
	_ = a.store.MarkAgentSkillStale(user.ID, agent.ID, "Agent API key rotated.")
	a.logUserAudit(r, user, "agent_token_rotated", agent.ID, agent.FriendlyName, map[string]any{"agent_id": agent.ID, "source": requestSource(r)})
	if wantsJSON(r) {
		payload := map[string]any{
			"status":     "ok",
			"message":    "Agent API key rotated.",
			"agent_id":   agent.ID,
			"token_hint": agent.TokenHint,
		}
		a.addAgentSkillWarningToPayload(user.ID, agent.ID, payload)
		writeJSON(w, http.StatusOK, payload)
		return
	}
	redirectAgentSaved(w, r, agent.ID)
}

func (a *App) handleAgentSkillWarningDismiss(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST required")
		return
	}
	user := a.requireSessionUser(w, r)
	if user == nil || !a.requireSessionCSRF(w, r) {
		return
	}
	agentID := strings.TrimSpace(r.FormValue("agent_id"))
	if err := a.store.ClearAgentSkillStale(user.ID, agentID); err != nil {
		writeError(w, http.StatusBadRequest, "agents_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":                "ok",
		"agent_id":              agentID,
		"skill_update_required": false,
	})
}

func (a *App) addAgentSkillWarningToPayload(userID, agentID string, payload map[string]any) {
	agent, err := a.store.GetAgent(userID, agentID)
	if err != nil || agent == nil {
		return
	}
	payload["skill_update_required"] = agent.SkillStale
	payload["skill_update_reason"] = agent.SkillStaleReason
	payload["skill_update_reasons"] = splitSkillStaleReasons(agent.SkillStaleReason)
}

func (a *App) handleAgentToggle(w http.ResponseWriter, r *http.Request) {
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
		redirectAgentError(w, r, agentID, firstErr(err, "agent not found"))
		return
	}
	agent.Enabled = r.FormValue("enabled") == "1"
	if err := a.store.SaveAgent(agent); err != nil {
		redirectAgentError(w, r, agentID, err)
		return
	}
	a.logUserAudit(r, user, "agent_status_updated", agent.ID, agent.FriendlyName, map[string]any{"agent_id": agent.ID, "enabled": agent.Enabled, "source": requestSource(r)})
	if wantsJSON(r) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":   "ok",
			"message":  "Agent access status updated.",
			"agent_id": agent.ID,
			"enabled":  agent.Enabled,
		})
		return
	}
	redirectAgentSaved(w, r, agent.ID)
}

func (a *App) handleAgentDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST required")
		return
	}
	user := a.requireSessionUser(w, r)
	if user == nil || !a.requireSessionCSRF(w, r) {
		return
	}
	agentID := strings.TrimSpace(r.FormValue("agent_id"))
	agent, _ := a.store.GetAgent(user.ID, agentID)
	if err := a.store.DeleteAgent(user.ID, agentID); err != nil {
		redirectAgentError(w, r, agentID, err)
		return
	}
	targetName := agentID
	if agent != nil {
		targetName = agent.FriendlyName
	}
	a.logUserAudit(r, user, "agent_deleted", agentID, targetName, map[string]any{"agent_id": agentID, "source": requestSource(r)})
	if wantsJSON(r) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":        "ok",
			"message":       "Agent profile deleted.",
			"agent_id":      agentID,
			"agent_name":    targetName,
			"agent_deleted": true,
		})
		return
	}
	http.Redirect(w, r, "/agents?agents_deleted=1", http.StatusFound)
}

func redirectAgentSaved(w http.ResponseWriter, r *http.Request, agentID string) {
	http.Redirect(w, r, agentRedirectPath(agentID, "agents_saved=1"), http.StatusFound)
}

func redirectAgentError(w http.ResponseWriter, r *http.Request, agentID string, err error) {
	if wantsJSON(r) {
		writeError(w, http.StatusBadRequest, "agents_error", err.Error())
		return
	}
	http.Redirect(w, r, agentRedirectPath(agentID, "agents_error="+url.QueryEscape(err.Error())), http.StatusFound)
}

func agentRedirectPath(agentID, query string) string {
	values := []string{}
	if agentID != "" {
		values = append(values, "agent="+url.QueryEscape(agentID))
	}
	if query != "" {
		values = append(values, query)
	}
	path := "/agents"
	if len(values) > 0 {
		path += "?" + strings.Join(values, "&")
	}
	if agentID != "" {
		path += "#agent-" + url.QueryEscape(agentID)
	}
	return path
}

func firstErr(err error, fallback string) error {
	if err != nil {
		return err
	}
	return errString(fallback)
}

type errString string

func (e errString) Error() string { return string(e) }

func formKey(value string) string {
	replacer := strings.NewReplacer("@", "_at_", ".", "_dot_", "-", "_dash_", "+", "_plus_")
	return replacer.Replace(normalizeEmail(value))
}
