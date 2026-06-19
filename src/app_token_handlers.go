// Copyright (c) 2026 Opensense Ltd. (Hong Kong). All rights reserved.
// Proprietary software. No use, copy, modification, distribution, disclosure,
// or reverse engineering is permitted without prior written authorization
// from Opensense Ltd.

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

const (
	agentWorkspaceAPIConfigFilename = "agents-workspace-api-access.config.json"
	userBackendAPIConfigFilename    = "user-backend-api-access.config.json"
	agentWorkspaceAPIConfigPath     = "config/" + agentWorkspaceAPIConfigFilename
)

func (a *App) newAgentToken() (rawToken, enc, hint string, err error) {
	rawToken, err = RandomToken("atk_", 24)
	if err != nil {
		return "", "", "", err
	}
	enc, err = a.crypto.Encrypt(rawToken)
	if err != nil {
		return "", "", "", err
	}
	hint = agentTokenHint(rawToken)
	return rawToken, enc, hint, nil
}

func agentTokenHint(rawTokenOrHint string) string {
	value := strings.TrimSpace(rawTokenOrHint)
	if before, after, ok := strings.Cut(value, "..."); ok {
		if len(before) >= 7 && len(after) >= 3 {
			return before[:7] + "..." + after[len(after)-3:]
		}
	}
	return tokenHint(value, 7, 3)
}

func (a *App) displayAgentTokenHint(agent AgentAccess) string {
	if rawToken, err := a.crypto.Decrypt(agent.TokenEnc); err == nil && rawToken != "" {
		return agentTokenHint(rawToken)
	}
	return agentTokenHint(agent.TokenHint)
}

func tokenHint(rawToken string, prefixLen, suffixLen int) string {
	if prefixLen < 0 {
		prefixLen = 0
	}
	if suffixLen < 0 {
		suffixLen = 0
	}
	if len(rawToken) <= prefixLen+suffixLen {
		return rawToken
	}
	return rawToken[:prefixLen] + "..." + rawToken[len(rawToken)-suffixLen:]
}

func (a *App) agentTokenForAgent(userID, agentID string) (string, error) {
	agent, err := a.store.GetAgent(userID, agentID)
	if err != nil {
		return "", err
	}
	if agent == nil {
		return "", fmt.Errorf("agent not found")
	}
	return a.crypto.Decrypt(agent.TokenEnc)
}

func (a *App) rotateUserBackendAPIToken(userID string) error {
	rawToken, err := RandomToken("ubk_", 24)
	if err != nil {
		return err
	}
	enc, err := a.crypto.Encrypt(rawToken)
	if err != nil {
		return err
	}
	hint := tokenHint(rawToken, 8, 6)
	return a.store.SaveUserBackendAPIToken(userID, enc, hint)
}

func (a *App) handleDownloadAgentWorkspaceAPIConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET required")
		return
	}
	user := a.requireSessionUser(w, r)
	if user == nil {
		return
	}
	agentID := strings.TrimSpace(r.URL.Query().Get("agent"))
	if agentID == "" {
		writeError(w, http.StatusBadRequest, "agent_required", "agent is required")
		return
	}
	raw, err := a.agentTokenForAgent(user.ID, agentID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token_error", err.Error())
		return
	}
	config, err := a.agentWorkspaceAPIConfig(user.ID, agentID, raw, "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "config_error", err.Error())
		return
	}
	writeDownloadJSON(w, agentWorkspaceAPIConfigFilename, config)
}

func (a *App) handleDownloadUserBackendAPIConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET required")
		return
	}
	user := a.requireSessionUser(w, r)
	if user == nil {
		return
	}
	rec, err := a.store.GetUserBackendAPITokenRecord(user.ID)
	if err != nil || rec == nil {
		writeError(w, http.StatusNotFound, "token_error", "user backend API token not found")
		return
	}
	raw, err := a.crypto.Decrypt(rec["token_enc"])
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token_error", err.Error())
		return
	}
	writeDownloadJSON(w, userBackendAPIConfigFilename, map[string]any{
		"config_type":            "user_backend_api_access",
		"proxy_url":              a.cfg.BaseURL,
		"user_backend_api_token": raw,
	})
}

func (a *App) agentWorkspaceAPIConfig(userID, agentID, rawToken, _ string) (map[string]any, error) {
	agent, err := a.store.GetAgent(userID, agentID)
	if err != nil {
		return nil, err
	}
	if agent == nil {
		return nil, fmt.Errorf("agent not found")
	}
	workspaces, err := a.agentSkillWorkspaces(userID, agentID)
	if err != nil {
		return nil, err
	}
	config := map[string]any{
		"proxy_url":       a.cfg.BaseURL,
		"agent_api_token": rawToken,
		"workspaces":      agentSkillConfigWorkspaces(workspaces),
	}
	return config, nil
}

func writeDownloadJSON(w http.ResponseWriter, filename string, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(data)
}

func (a *App) handleRotateUserBackendAPIKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST required")
		return
	}
	user := a.requireSessionUser(w, r)
	if user == nil {
		return
	}
	if !a.requireSessionCSRF(w, r) {
		return
	}
	if err := a.rotateUserBackendAPIToken(user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "token_error", err.Error())
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

func (a *App) ensureUserBackendAPIToken(userID string) error {
	rec, err := a.store.GetUserBackendAPITokenRecord(userID)
	if err != nil {
		return err
	}
	if rec != nil {
		return nil
	}
	return a.rotateUserBackendAPIToken(userID)
}
