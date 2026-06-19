// Copyright (c) 2026 Opensense Ltd. (Hong Kong). All rights reserved.
// Proprietary software. No use, copy, modification, distribution, disclosure,
// or reverse engineering is permitted without prior written authorization
// from Opensense Ltd.

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const (
	agentMotiveHeader   = "X-AIWP-Agent-Motive"
	humanApprovalHeader = "X-AIWP-Human-Approval"

	maxAgentNameLen     = 256
	maxAgentLocationLen = 512
	maxAgentMotiveLen   = 4096
	maxHumanApprovalLen = 4096
)

type agentContext struct {
	Name     string
	Location string
	Motive   string
}

func newRequestID() string {
	token, err := RandomToken("req_", 12)
	if err != nil {
		return "req_unavailable"
	}
	return token
}

func agentContextForAgentRequest(r *http.Request, agent *AgentAccess, requireMotive bool) (agentContext, error) {
	ctx := agentContext{
		Motive: strings.TrimSpace(r.Header.Get(agentMotiveHeader)),
	}
	if agent != nil {
		ctx.Name = strings.TrimSpace(agent.FriendlyName)
		ctx.Location = strings.TrimSpace(agent.DefaultLocation)
	}
	if err := validateAgentContextField("agent name", ctx.Name, maxAgentNameLen, true); err != nil {
		return ctx, err
	}
	if err := validateAgentContextField("agent location", ctx.Location, maxAgentLocationLen, true); err != nil {
		return ctx, err
	}
	if err := validateAgentContextField("agent motive", ctx.Motive, maxAgentMotiveLen, requireMotive); err != nil {
		return ctx, err
	}
	return ctx, nil
}

func validateAgentContextField(label, value string, maxLen int, required bool) error {
	if required && value == "" {
		if label == "agent motive" {
			return fmt.Errorf("%s is required; provide %s header", label, agentMotiveHeader)
		}
		if label == "human approval" {
			return fmt.Errorf("%s is required; provide %s header with a short explanation of how approval was obtained", label, humanApprovalHeader)
		}
		return fmt.Errorf("%s is required in the agent profile", label)
	}
	if len([]rune(value)) > maxLen {
		return fmt.Errorf("%s exceeds maximum length of %d characters", label, maxLen)
	}
	return nil
}

func validateHumanApprovalHeader(r *http.Request) (string, error) {
	value := strings.TrimSpace(r.Header.Get(humanApprovalHeader))
	if err := validateAgentContextField("human approval", value, maxHumanApprovalLen, true); err != nil {
		return value, err
	}
	return value, nil
}

func (a *App) baseRequestLogEntry(r *http.Request, requestID string, user *User, agentID string, ctx agentContext) RequestLogEntry {
	return RequestLogEntry{
		RequestID:      requestID,
		UserID:         user.ID,
		UserEmail:      user.Email,
		CreatedAt:      nowUTC(),
		AgentID:        strings.TrimSpace(agentID),
		Method:         r.Method,
		Path:           r.URL.Path,
		Query:          sanitizeQueryForLog(r.URL.Query()),
		UserAgent:      strings.TrimSpace(r.UserAgent()),
		RemoteAddr:     r.RemoteAddr,
		XForwardedFor:  strings.TrimSpace(r.Header.Get("X-Forwarded-For")),
		XRealIP:        strings.TrimSpace(r.Header.Get("X-Real-IP")),
		Forwarded:      strings.TrimSpace(r.Header.Get("Forwarded")),
		CFConnectingIP: strings.TrimSpace(r.Header.Get("CF-Connecting-IP")),
		AgentName:      ctx.Name,
		AgentLocation:  ctx.Location,
		AgentMotive:    ctx.Motive,
	}
}

func sanitizeQueryForLog(query url.Values) string {
	copy := url.Values{}
	for key, values := range query {
		if strings.EqualFold(key, "token") || strings.Contains(strings.ToLower(key), "access_token") {
			copy[key] = []string{"[redacted]"}
			continue
		}
		copy[key] = values
	}
	return copy.Encode()
}

func (a *App) saveRequestLog(entry RequestLogEntry) {
	if entry.Outcome == "" {
		entry.Outcome = "Fail"
	}
	if entry.PolicyID != "" && entry.PolicyName == "" {
		entry.PolicyName = a.policyDisplayNameForUser(entry.UserID, entry.PolicyID)
	}
	if entry.TargetObjectType == "" && entry.TargetObjectID == "" {
		entry.TargetObjectType, entry.TargetObjectID = inferRequestLogTargetObject(entry.Service, entry.Path, entry.Query)
	}
	_ = a.store.SaveRequestLog(&entry)
}

func inferServiceLabelFromPath(path string) string {
	switch {
	case strings.HasPrefix(path, "/gmail.googleapis.com/"):
		return "gmail"
	case strings.HasPrefix(path, "/people.googleapis.com/"):
		return "people"
	case strings.HasPrefix(path, "/calendar.googleapis.com/"):
		return "calendar"
	case strings.HasPrefix(path, "/drive.googleapis.com/"):
		return "drive"
	case strings.HasPrefix(path, "/docs.googleapis.com/"):
		return "docs"
	case strings.HasPrefix(path, "/sheets.googleapis.com/"):
		return "sheets"
	case strings.HasPrefix(path, "/slides.googleapis.com/"):
		return "slides"
	default:
		return ""
	}
}

func inferRequestLogTargetObject(service, path, rawQuery string) (string, string) {
	query, _ := url.ParseQuery(rawQuery)
	if service == "gmail" {
		if id := pathIDAfter(path, "/gmail/v1/users/me/messages/"); id != "" {
			return "Gmail message", id
		}
		if id := pathIDAfter(path, "/gmail/v1/users/me/drafts/"); id != "" {
			return "Gmail draft", id
		}
		if id := pathIDAfter(path, "/gmail/v1/users/me/labels/"); id != "" {
			return "Gmail label", id
		}
		if id := pathIDAfter(path, "/gmail/v1/users/me/threads/"); id != "" {
			return "Gmail thread", id
		}
		return "", ""
	}
	if service == "calendar" {
		if strings.HasPrefix(path, "/calendar/v3/users/me/calendarList/") {
			if id := pathIDAfter(path, "/calendar/v3/users/me/calendarList/"); id != "" {
				return "Calendar", id
			}
		}
		if strings.HasPrefix(path, "/calendar/v3/calendars/") {
			rest := strings.TrimPrefix(path, "/calendar/v3/calendars/")
			parts := pathParts(rest)
			if len(parts) >= 3 && parts[1] == "events" {
				return "Calendar event", parts[0] + "/" + stripGoogleMethodSuffix(parts[2])
			}
			if len(parts) >= 1 {
				return "Calendar", parts[0]
			}
		}
		return "", ""
	}
	if service == "drive" {
		if fileID := pathIDAfter(path, "/upload/drive/v3/files/"); fileID != "" {
			return "Drive file", fileID
		}
		if strings.HasPrefix(path, "/drive/v3/files/") {
			parts := pathParts(strings.TrimPrefix(path, "/drive/v3/files/"))
			if len(parts) >= 5 && parts[1] == "comments" && parts[3] == "replies" {
				return "Drive comment reply", parts[0] + "/" + parts[2] + "/" + stripGoogleMethodSuffix(parts[4])
			}
			if len(parts) >= 3 && parts[1] == "comments" {
				return "Drive comment", parts[0] + "/" + stripGoogleMethodSuffix(parts[2])
			}
			if len(parts) >= 1 {
				return "Drive file", stripGoogleMethodSuffix(parts[0])
			}
		}
		return driveFolderTargetFromQuery(query)
	}
	switch service {
	case "docs":
		if id := pathIDAfter(path, "/v1/documents/"); id != "" {
			return "Google Doc", id
		}
	case "sheets":
		if id := pathIDAfter(path, "/v4/spreadsheets/"); id != "" {
			return "Google Sheet", id
		}
	case "slides":
		if id := pathIDAfter(path, "/v1/presentations/"); id != "" {
			return "Google Slide", id
		}
	}
	if service == "docs" || service == "sheets" || service == "slides" {
		if targetType, targetID := driveFolderTargetFromQuery(query); targetType != "" || targetID != "" {
			return targetType, targetID
		}
	}
	return "", ""
}

func driveFolderTargetFromQuery(query url.Values) (string, string) {
	if id := strings.TrimSpace(query.Get("driveFolderId")); id != "" {
		return "Drive folder", id
	}
	if ref := strings.TrimSpace(query.Get("driveRef")); ref != "" {
		return "Drive folder reference", ref
	}
	if path := strings.TrimSpace(query.Get("drivePath")); path != "" {
		return "Drive folder path", path
	}
	return "", ""
}

func pathIDAfter(path, prefix string) string {
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	parts := pathParts(strings.TrimPrefix(path, prefix))
	if len(parts) == 0 {
		return ""
	}
	return stripGoogleMethodSuffix(parts[0])
}

func pathParts(raw string) []string {
	rawParts := strings.Split(strings.Trim(raw, "/"), "/")
	parts := make([]string, 0, len(rawParts))
	for _, part := range rawParts {
		if part == "" {
			continue
		}
		if decoded, err := url.PathUnescape(part); err == nil {
			part = decoded
		}
		parts = append(parts, part)
	}
	return parts
}

func stripGoogleMethodSuffix(id string) string {
	return strings.Split(id, ":")[0]
}

func auditDetailsJSON(values map[string]any) string {
	raw, err := json.Marshal(values)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

func auditNetworkFields(r *http.Request) (userAgent, remoteAddr, xForwardedFor, xRealIP, forwarded, cfConnectingIP string) {
	if r == nil {
		return "", "", "", "", "", ""
	}
	return strings.TrimSpace(r.UserAgent()),
		strings.TrimSpace(r.RemoteAddr),
		strings.TrimSpace(r.Header.Get("X-Forwarded-For")),
		strings.TrimSpace(r.Header.Get("X-Real-IP")),
		strings.TrimSpace(r.Header.Get("Forwarded")),
		strings.TrimSpace(r.Header.Get("CF-Connecting-IP"))
}

func (a *App) logPolicyAudit(r *http.Request, user *User, workspaceEmail, action, targetID, targetName string, details map[string]any) {
	userAgent, remoteAddr, xForwardedFor, xRealIP, forwarded, cfConnectingIP := auditNetworkFields(r)
	_ = a.store.SavePolicyAuditLog(&AuditLogEntry{
		UserID:         user.ID,
		UserEmail:      user.Email,
		WorkspaceEmail: normalizeEmail(workspaceEmail),
		Action:         action,
		TargetID:       targetID,
		TargetName:     targetName,
		UserAgent:      userAgent,
		RemoteAddr:     remoteAddr,
		XForwardedFor:  xForwardedFor,
		XRealIP:        xRealIP,
		Forwarded:      forwarded,
		CFConnectingIP: cfConnectingIP,
		DetailsJSON:    auditDetailsJSON(details),
	})
}

func (a *App) logWorkspaceAudit(r *http.Request, user *User, workspaceEmail, action, targetID, targetName string, details map[string]any) {
	userAgent, remoteAddr, xForwardedFor, xRealIP, forwarded, cfConnectingIP := auditNetworkFields(r)
	_ = a.store.SaveWorkspaceAuditLog(&AuditLogEntry{
		UserID:         user.ID,
		UserEmail:      user.Email,
		WorkspaceEmail: normalizeEmail(workspaceEmail),
		Action:         action,
		TargetID:       targetID,
		TargetName:     targetName,
		UserAgent:      userAgent,
		RemoteAddr:     remoteAddr,
		XForwardedFor:  xForwardedFor,
		XRealIP:        xRealIP,
		Forwarded:      forwarded,
		CFConnectingIP: cfConnectingIP,
		DetailsJSON:    auditDetailsJSON(details),
	})
}

func (a *App) logDriveFolderAudit(r *http.Request, user *User, workspaceEmail, action, targetID, targetName string, details map[string]any) {
	userAgent, remoteAddr, xForwardedFor, xRealIP, forwarded, cfConnectingIP := auditNetworkFields(r)
	_ = a.store.SaveDriveFolderAuditLog(&AuditLogEntry{
		UserID:         user.ID,
		UserEmail:      user.Email,
		WorkspaceEmail: normalizeEmail(workspaceEmail),
		Action:         action,
		TargetID:       targetID,
		TargetName:     targetName,
		UserAgent:      userAgent,
		RemoteAddr:     remoteAddr,
		XForwardedFor:  xForwardedFor,
		XRealIP:        xRealIP,
		Forwarded:      forwarded,
		CFConnectingIP: cfConnectingIP,
		DetailsJSON:    auditDetailsJSON(details),
	})
}

func (a *App) logUserAudit(r *http.Request, user *User, action, targetID, targetName string, details map[string]any) {
	if user == nil {
		return
	}
	a.writeUserAudit(r, user, action, targetID, targetName, details)
}

func (a *App) writeUserAudit(r *http.Request, user *User, action, targetID, targetName string, details map[string]any) {
	if user == nil {
		return
	}
	userAgent, remoteAddr, xForwardedFor, xRealIP, forwarded, cfConnectingIP := auditNetworkFields(r)
	_ = a.store.SaveUserAuditLog(&AuditLogEntry{
		UserID:         user.ID,
		UserEmail:      user.Email,
		Action:         action,
		TargetID:       targetID,
		TargetName:     targetName,
		UserAgent:      userAgent,
		RemoteAddr:     remoteAddr,
		XForwardedFor:  xForwardedFor,
		XRealIP:        xRealIP,
		Forwarded:      forwarded,
		CFConnectingIP: cfConnectingIP,
		DetailsJSON:    auditDetailsJSON(details),
	})
}

func requestLogView(entry RequestLogEntry, timezone string) map[string]any {
	errorMessage := entry.ErrorMessage
	if len(errorMessage) > 240 {
		errorMessage = errorMessage[:240] + "..."
	}
	motive := entry.AgentMotive
	if len(motive) > 240 {
		motive = motive[:240] + "..."
	}
	return map[string]any{
		"RequestID":             entry.RequestID,
		"CreatedAt":             formatUserTime(entry.CreatedAt, timezone),
		"WorkspaceEmail":        entry.WorkspaceEmail,
		"Method":                entry.Method,
		"Service":               entry.Service,
		"Path":                  entry.Path,
		"Query":                 entry.Query,
		"TargetObjectType":      entry.TargetObjectType,
		"TargetObjectID":        entry.TargetObjectID,
		"UserAgent":             entry.UserAgent,
		"RemoteAddr":            entry.RemoteAddr,
		"XForwardedFor":         entry.XForwardedFor,
		"XRealIP":               entry.XRealIP,
		"Forwarded":             entry.Forwarded,
		"CFConnectingIP":        entry.CFConnectingIP,
		"AgentName":             entry.AgentName,
		"AgentLocation":         entry.AgentLocation,
		"AgentMotive":           motive,
		"Outcome":               entry.Outcome,
		"HTTPStatus":            entry.HTTPStatus,
		"UpstreamStatus":        entry.UpstreamStatus,
		"PolicyID":              entry.PolicyID,
		"PolicyName":            entry.PolicyName,
		"PolicyCapabilityKey":   entry.PolicyCapabilityKey,
		"PolicyCapabilityTitle": entry.PolicyCapabilityTitle,
		"PolicyRuleName":        entry.PolicyRuleName,
		"ErrorMessage":          errorMessage,
	}
}

func auditLogView(entry AuditLogEntry, timezone string) map[string]any {
	return map[string]any{
		"CreatedAt":      formatUserTime(entry.CreatedAt, timezone),
		"WorkspaceEmail": entry.WorkspaceEmail,
		"Action":         entry.Action,
		"TargetID":       entry.TargetID,
		"TargetName":     entry.TargetName,
		"UserAgent":      entry.UserAgent,
		"RemoteAddr":     entry.RemoteAddr,
		"XForwardedFor":  entry.XForwardedFor,
		"XRealIP":        entry.XRealIP,
		"Forwarded":      entry.Forwarded,
		"CFConnectingIP": entry.CFConnectingIP,
		"DetailsJSON":    entry.DetailsJSON,
	}
}

func parseRetentionDays(raw string, maxDays int) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("retention days must be a whole number")
	}
	if n < 1 || n > maxDays {
		return 0, fmt.Errorf("retention days must be between 1 and %d", maxDays)
	}
	return n, nil
}
