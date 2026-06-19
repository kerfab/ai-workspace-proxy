// Copyright (c) 2026 Opensense Ltd. (Hong Kong). All rights reserved.
// Proprietary software. No use, copy, modification, distribution, disclosure,
// or reverse engineering is permitted without prior written authorization
// from Opensense Ltd.

package main

import (
	"net/http"
	"net/url"
	"sort"
	"strings"
)

const policyOptionLabelMaxChars = 75

func truncatePolicyOptionLabel(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= policyOptionLabelMaxChars {
		return text
	}
	if policyOptionLabelMaxChars <= 1 {
		return "…"
	}
	return string(runes[:policyOptionLabelMaxChars-1]) + "…"
}

func policyOptionFullLabel(name string, isSystem, isDefault bool) string {
	name = strings.TrimSpace(name)
	if isSystem {
		label := "System policy"
		if isDefault {
			label += " (default)"
		}
		return label + " (not editable & read permissions only)"
	}
	if isDefault {
		return name + " (default)"
	}
	return name
}

func (a *App) policyEditorView(userID string, r *http.Request) map[string]any {
	settings, _ := a.store.GetUserSettings(userID)
	selectedPolicyID := strings.TrimSpace(r.URL.Query().Get("policy"))
	if selectedPolicyID == "" {
		selectedPolicyID = settings.DefaultPolicyID
	}
	if selectedPolicyID == "" {
		selectedPolicyID = systemPolicyID
	}
	isNew := selectedPolicyID == "new"
	isSystem := selectedPolicyID == systemPolicyID && !isNew
	selectedName := ""
	selectedCapabilities := SystemDefaultCapabilityKeys()
	selectedReviewRequired := []string{}
	selectedIsDefault := settings.DefaultPolicyID == systemPolicyID
	if isNew {
		selectedName = ""
		selectedIsDefault = false
	} else if !isSystem {
		policy, _ := a.store.GetUserPolicy(userID, selectedPolicyID)
		if policy == nil {
			selectedPolicyID = systemPolicyID
			isSystem = true
			selectedName = systemPolicyName
			selectedCapabilities = SystemDefaultCapabilityKeys()
			selectedIsDefault = settings.DefaultPolicyID == systemPolicyID
		} else {
			selectedName = policy.Name
			selectedCapabilities = policy.EnabledCapabilities
			selectedReviewRequired = policy.ReviewRequiredCapabilities
			selectedIsDefault = settings.DefaultPolicyID == policy.ID
		}
	} else {
		selectedName = systemPolicyName
	}
	selectedSet := sliceToSet(selectedCapabilities)
	reviewRequiredSet := sliceToSet(selectedReviewRequired)
	systemSet := sliceToSet(SystemDefaultCapabilityKeys())
	selectedIsApplied := selectedIsDefault
	if !isNew && selectedPolicyID != systemPolicyID {
		usedByAgentGrant, _ := a.store.PolicyUsedByAgentGrant(userID, selectedPolicyID)
		selectedIsApplied = selectedIsApplied || usedByAgentGrant
	}
	return map[string]any{
		"Options":           a.policyOptionsForUser(userID, selectedPolicyID),
		"Capabilities":      PolicyCapabilitiesForSelection(selectedSet, reviewRequiredSet),
		"CapabilityGroups":  PolicyCapabilityGroupsForSelection(selectedSet, reviewRequiredSet),
		"SystemDefaults":    PolicyCapabilitiesForSelection(systemSet),
		"SelectedID":        selectedPolicyID,
		"SelectedName":      selectedName,
		"SelectedIsSystem":  isSystem,
		"SelectedIsNew":     isNew,
		"SelectedIsDefault": selectedIsDefault,
		"SelectedIsApplied": selectedIsApplied,
		"DefaultPolicyID":   settings.DefaultPolicyID,
		"Error":             r.URL.Query().Get("policy_error"),
		"Saved":             r.URL.Query().Get("policy_saved") == "1",
		"DefaultSaved":      r.URL.Query().Get("policy_default_saved") == "1",
		"Deleted":           r.URL.Query().Get("policy_deleted"),
	}
}
func (a *App) policyOptionsForUser(userID, selectedPolicyID string) []map[string]any {
	settings, _ := a.store.GetUserSettings(userID)
	selectedPolicyID = strings.TrimSpace(selectedPolicyID)
	if selectedPolicyID == "" {
		selectedPolicyID = systemPolicyID
	}
	defaultPolicyID := strings.TrimSpace(settings.DefaultPolicyID)
	if defaultPolicyID == "" {
		defaultPolicyID = systemPolicyID
	}
	systemOption := map[string]any{
		"ID":           systemPolicyID,
		"Name":         systemPolicyName,
		"Selected":     selectedPolicyID == systemPolicyID,
		"IsDefault":    defaultPolicyID == systemPolicyID,
		"System":       true,
		"FullLabel":    policyOptionFullLabel(systemPolicyName, true, defaultPolicyID == systemPolicyID),
		"DisplayLabel": truncatePolicyOptionLabel(policyOptionFullLabel(systemPolicyName, true, defaultPolicyID == systemPolicyID)),
	}
	customOptions := []map[string]any{}
	policies, _ := a.store.ListUserPolicies(userID)
	for _, policy := range policies {
		fullLabel := policyOptionFullLabel(policy.Name, false, defaultPolicyID == policy.ID)
		customOptions = append(customOptions, map[string]any{
			"ID":           policy.ID,
			"Name":         policy.Name,
			"Selected":     selectedPolicyID == policy.ID,
			"IsDefault":    defaultPolicyID == policy.ID,
			"System":       false,
			"FullLabel":    fullLabel,
			"DisplayLabel": truncatePolicyOptionLabel(fullLabel),
		})
	}
	sort.SliceStable(customOptions, func(i, j int) bool {
		left := strings.ToLower(strings.TrimSpace(customOptions[i]["Name"].(string)))
		right := strings.ToLower(strings.TrimSpace(customOptions[j]["Name"].(string)))
		if left == right {
			return strings.TrimSpace(customOptions[i]["ID"].(string)) < strings.TrimSpace(customOptions[j]["ID"].(string))
		}
		return left < right
	})
	options := make([]map[string]any, 0, len(customOptions)+1)
	if defaultPolicyID == systemPolicyID {
		options = append(options, systemOption)
		options = append(options, customOptions...)
		return options
	}
	for _, option := range customOptions {
		if option["ID"] == defaultPolicyID {
			options = append(options, option)
			break
		}
	}
	options = append(options, systemOption)
	for _, option := range customOptions {
		if option["ID"] == defaultPolicyID {
			continue
		}
		options = append(options, option)
	}
	return options
}
func (a *App) handlePolicySave(w http.ResponseWriter, r *http.Request) {
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
	policyID := strings.TrimSpace(r.FormValue("policy_id"))
	if policyID == systemPolicyID {
		if wantsJSON(r) {
			writeError(w, http.StatusBadRequest, "policy_error", "system default policy cannot be modified")
			return
		}
		http.Redirect(w, r, "/policies?policy_error="+url.QueryEscape("system default policy cannot be modified"), http.StatusFound)
		return
	}
	action := "policy_created"
	if policyID != "" {
		action = "policy_updated"
	}
	policy := &UserPolicy{
		ID:                         policyID,
		UserID:                     user.ID,
		Name:                       strings.TrimSpace(r.FormValue("name")),
		EnabledCapabilities:        r.Form["capability"],
		ReviewRequiredCapabilities: r.Form["review_capability"],
	}
	if err := a.store.SaveUserPolicy(policy); err != nil {
		if wantsJSON(r) {
			writeError(w, http.StatusBadRequest, "policy_error", err.Error())
			return
		}
		http.Redirect(w, r, "/policies?policy="+url.QueryEscape(policyID)+"&policy_error="+url.QueryEscape(err.Error()), http.StatusFound)
		return
	}
	staleAgentIDs := []string{}
	staleReason := ""
	if action == "policy_updated" {
		staleAgentIDs, _ = a.store.AgentIDsForPolicy(user.ID, policy.ID)
		staleReason = "Policy \"" + policy.Name + "\" changed."
		_ = a.store.MarkAgentsSkillStale(user.ID, staleAgentIDs, staleReason)
	}
	a.logPolicyAudit(r, user, "", action, policy.ID, policy.Name, map[string]any{
		"capabilities":                 policy.EnabledCapabilities,
		"review_required_capabilities": policy.ReviewRequiredCapabilities,
	})
	if wantsJSON(r) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":                 "ok",
			"message":                "Policy saved.",
			"policy_id":              policy.ID,
			"policy_name":            policy.Name,
			"skill_update_agent_ids": staleAgentIDs,
			"skill_update_reasons":   []string{staleReason},
		})
		return
	}
	http.Redirect(w, r, "/policies?policy="+url.QueryEscape(policy.ID)+"&policy_saved=1", http.StatusFound)
}
func (a *App) handlePolicyDefault(w http.ResponseWriter, r *http.Request) {
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
	policyID := strings.TrimSpace(r.FormValue("default_policy_id"))
	if err := a.store.SaveDefaultPolicyID(user.ID, policyID); err != nil {
		if wantsJSON(r) {
			writeError(w, http.StatusBadRequest, "policy_error", err.Error())
			return
		}
		http.Redirect(w, r, "/policies?policy_error="+url.QueryEscape(err.Error()), http.StatusFound)
		return
	}
	policyName := a.policyDisplayNameForUser(user.ID, policyID)
	a.logPolicyAudit(r, user, "", "default_policy_changed", policyID, policyName, map[string]any{})
	if wantsJSON(r) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":      "ok",
			"message":     "Default policy selection saved.",
			"policy_id":   policyID,
			"policy_name": policyName,
		})
		return
	}
	http.Redirect(w, r, "/policies?policy="+url.QueryEscape(policyID)+"&policy_default_saved=1", http.StatusFound)
}
func (a *App) handlePolicyDelete(w http.ResponseWriter, r *http.Request) {
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
	policyID := strings.TrimSpace(r.FormValue("policy_id"))
	agentIDs, _ := a.store.AgentIDsForPolicy(user.ID, policyID)
	wasDefault, err := a.store.DeleteUserPolicy(user.ID, policyID)
	if err != nil {
		http.Redirect(w, r, "/policies?policy_error="+url.QueryEscape(err.Error()), http.StatusFound)
		return
	}
	_ = a.store.MarkAgentsSkillStale(user.ID, agentIDs, "A policy used by this agent was deleted.")
	a.logPolicyAudit(r, user, "", "policy_deleted", policyID, policyID, map[string]any{"reverted": wasDefault})
	deleted := "custom"
	if wasDefault {
		deleted = "default"
	}
	http.Redirect(w, r, "/policies?policy="+systemPolicyID+"&policy_deleted="+deleted, http.StatusFound)
}
