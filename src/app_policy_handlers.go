package main

import (
	"net/http"
	"net/url"
	"strings"
)

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
			selectedIsDefault = settings.DefaultPolicyID == policy.ID
		}
	} else {
		selectedName = systemPolicyName
	}
	selectedSet := sliceToSet(selectedCapabilities)
	systemSet := sliceToSet(SystemDefaultCapabilityKeys())
	selectedIsApplied := selectedIsDefault
	if !isNew && selectedPolicyID != systemPolicyID {
		usedByWorkspace, _ := a.store.PolicyUsedByWorkspace(userID, selectedPolicyID)
		selectedIsApplied = selectedIsApplied || usedByWorkspace
	}
	return map[string]any{
		"Options":           a.policyOptionsForUser(userID, selectedPolicyID),
		"Capabilities":      PolicyCapabilitiesForSelection(selectedSet),
		"CapabilityGroups":  PolicyCapabilityGroupsForSelection(selectedSet),
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
	options := []map[string]any{{
		"ID":        systemPolicyID,
		"Name":      systemPolicyName,
		"Selected":  selectedPolicyID == systemPolicyID,
		"IsDefault": settings.DefaultPolicyID == systemPolicyID,
		"System":    true,
	}}
	policies, _ := a.store.ListUserPolicies(userID)
	for _, policy := range policies {
		options = append(options, map[string]any{
			"ID":        policy.ID,
			"Name":      policy.Name,
			"Selected":  selectedPolicyID == policy.ID,
			"IsDefault": settings.DefaultPolicyID == policy.ID,
			"System":    false,
		})
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
		http.Redirect(w, r, "/policies?policy_error="+url.QueryEscape("system default policy cannot be modified"), http.StatusFound)
		return
	}
	policy := &UserPolicy{
		ID:                  policyID,
		UserID:              user.ID,
		Name:                strings.TrimSpace(r.FormValue("name")),
		EnabledCapabilities: r.Form["capability"],
	}
	if err := a.store.SaveUserPolicy(policy); err != nil {
		http.Redirect(w, r, "/policies?policy="+url.QueryEscape(policyID)+"&policy_error="+url.QueryEscape(err.Error()), http.StatusFound)
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
		http.Redirect(w, r, "/policies?policy_error="+url.QueryEscape(err.Error()), http.StatusFound)
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
	wasDefault, err := a.store.DeleteUserPolicy(user.ID, policyID)
	if err != nil {
		http.Redirect(w, r, "/policies?policy_error="+url.QueryEscape(err.Error()), http.StatusFound)
		return
	}
	deleted := "custom"
	if wasDefault {
		deleted = "default"
	}
	http.Redirect(w, r, "/policies?policy="+systemPolicyID+"&policy_deleted="+deleted, http.StatusFound)
}
