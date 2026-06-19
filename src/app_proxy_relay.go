// Copyright (c) 2026 Opensense Ltd. (Hong Kong). All rights reserved.
// Proprietary software. No use, copy, modification, distribution, disclosure,
// or reverse engineering is permitted without prior written authorization
// from Opensense Ltd.

package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type relayTarget struct {
	serviceLabel   string
	upstreamBase   string
	normalizedPath string
}

func (a *App) policyEngineForPolicy(userID, policyID string) (*PolicyEngine, string, error) {
	policyID = strings.TrimSpace(policyID)
	if policyID == "" || policyID == systemPolicyID {
		engine, err := NewPolicyEngineWithReview(SystemDefaultCapabilityKeys(), nil)
		return engine, systemPolicyID, err
	}
	policy, err := a.store.GetUserPolicy(userID, policyID)
	if err != nil {
		return nil, "", err
	}
	if policy == nil {
		return nil, "", fmt.Errorf("policy not found")
	}
	engine, err := NewPolicyEngineWithReview(policy.EnabledCapabilities, policy.ReviewRequiredCapabilities)
	if err != nil {
		return nil, "", err
	}
	return engine, policyID, nil
}

func (a *App) handleProxyRelay(w http.ResponseWriter, r *http.Request) {
	requestID := newRequestID()
	w.Header().Set("X-AIWP-Request-ID", requestID)
	auth, err := a.currentAgentFromBearer(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "auth_error", err.Error())
		return
	}
	if auth == nil {
		writeError(w, http.StatusUnauthorized, "auth_required", "missing Bearer token")
		return
	}
	user := auth.User
	agent := auth.Agent
	if user == nil || user.IsSuspended {
		writeError(w, http.StatusUnauthorized, "auth_error", "invalid or suspended user")
		return
	}
	_ = a.store.TouchAgentUsage(agent.ID)

	agentCtx, err := agentContextForAgentRequest(r, agent, false)
	logEntry := a.baseRequestLogEntry(r, requestID, user, agent.ID, agentCtx)
	logEntry.Service = inferServiceLabelFromPath(r.URL.Path)
	if err != nil {
		_ = a.store.IncrementDailyStat(user.ID, false)
		logEntry.Outcome = "Fail"
		logEntry.HTTPStatus = http.StatusForbidden
		logEntry.ErrorMessage = err.Error()
		a.saveRequestLog(logEntry)
		writeError(w, http.StatusForbidden, "agent_context_required", err.Error())
		return
	}
	if a.denyIfAgentFirewallBlocks(w, r, requestID, user, agent, logEntry.Service) {
		return
	}

	conn, err := a.resolveWorkspaceFromQuery(user.ID, r)
	if err != nil {
		_ = a.store.IncrementDailyStat(user.ID, false)
		logEntry.Outcome = "Fail"
		logEntry.HTTPStatus = http.StatusForbidden
		logEntry.ErrorMessage = err.Error()
		a.saveRequestLog(logEntry)
		writeError(w, http.StatusForbidden, "workspace_not_found", err.Error())
		return
	}
	logEntry.WorkspaceEmail = conn.MailboxEmail
	grant, err := a.store.GetAgentWorkspaceGrant(user.ID, agent.ID, conn.MailboxEmail)
	if err != nil {
		logEntry.Outcome = "Fail"
		logEntry.HTTPStatus = http.StatusInternalServerError
		logEntry.ErrorMessage = err.Error()
		a.saveRequestLog(logEntry)
		writeError(w, http.StatusInternalServerError, "agent_grant_error", err.Error())
		return
	}
	if grant == nil {
		_ = a.store.IncrementDailyStat(user.ID, false)
		logEntry.Outcome = "Fail"
		logEntry.HTTPStatus = http.StatusForbidden
		logEntry.ErrorMessage = "agent is not allowed to access this workspace"
		a.saveRequestLog(logEntry)
		writeError(w, http.StatusForbidden, "workspace_not_allowed", "agent is not allowed to access this workspace")
		return
	}
	if grant.RequireAgentMotive {
		agentCtx, err = agentContextForAgentRequest(r, agent, true)
		logEntry.AgentMotive = agentCtx.Motive
		if err != nil {
			_ = a.store.IncrementDailyStat(user.ID, false)
			logEntry.Outcome = "Fail"
			logEntry.HTTPStatus = http.StatusForbidden
			logEntry.ErrorMessage = err.Error()
			a.saveRequestLog(logEntry)
			writeError(w, http.StatusForbidden, "agent_context_required", err.Error())
			return
		}
	}

	target, err := normalizeProxyTarget(r.URL.EscapedPath(), conn.MailboxEmail)
	if err != nil {
		_ = a.store.IncrementDailyStat(user.ID, false)
		logEntry.Outcome = "Fail"
		logEntry.HTTPStatus = http.StatusForbidden
		logEntry.ErrorMessage = err.Error()
		a.saveRequestLog(logEntry)
		writeError(w, http.StatusForbidden, "request_denied", err.Error())
		return
	}
	logEntry.Service = target.serviceLabel
	logEntry.Path = target.normalizedPath

	body, err := io.ReadAll(io.LimitReader(r.Body, a.cfg.MaxRequestBodyBytes+1))
	if err != nil {
		logEntry.Outcome = "Fail"
		logEntry.HTTPStatus = http.StatusBadRequest
		logEntry.ErrorMessage = err.Error()
		a.saveRequestLog(logEntry)
		writeError(w, http.StatusBadRequest, "body_error", err.Error())
		return
	}
	if int64(len(body)) > a.cfg.MaxRequestBodyBytes {
		_ = a.store.IncrementDailyStat(user.ID, false)
		logEntry.Outcome = "Fail"
		logEntry.HTTPStatus = http.StatusRequestEntityTooLarge
		logEntry.ErrorMessage = "request body too large"
		a.saveRequestLog(logEntry)
		writeError(w, http.StatusRequestEntityTooLarge, "body_too_large", "request body too large")
		return
	}

	accessToken, err := a.getValidWorkspaceAccessToken(conn)
	if err != nil {
		var authErr *workspaceAuthRequiredError
		if errors.As(err, &authErr) {
			logEntry.Outcome = "Fail"
			logEntry.HTTPStatus = http.StatusForbidden
			logEntry.ErrorMessage = authErr.Message
			a.saveRequestLog(logEntry)
			writeJSON(w, http.StatusForbidden, map[string]any{
				"error":           "workspace_reauth_required",
				"message":         authErr.AgentMessage(),
				"workspace":       authErr.WorkspaceEmail,
				"reauth_url":      authErr.ReauthURL,
				"request_id":      requestID,
				"reauth_required": true,
			})
			return
		}
		logEntry.Outcome = "Fail"
		logEntry.HTTPStatus = http.StatusBadGateway
		logEntry.ErrorMessage = err.Error()
		a.saveRequestLog(logEntry)
		writeError(w, http.StatusBadGateway, "workspace_token_error", err.Error())
		return
	}

	query := r.URL.Query()
	query.Del("workspace")
	body, rawQuery, postMoveFolderID, err := a.rewriteWorkspaceRequest(user.ID, agent.ID, conn.MailboxEmail, accessToken, r.Method, target, body, query)
	if err != nil {
		_ = a.store.IncrementDailyStat(user.ID, false)
		logEntry.Outcome = "Fail"
		logEntry.HTTPStatus = http.StatusForbidden
		logEntry.ErrorMessage = err.Error()
		a.saveRequestLog(logEntry)
		writeError(w, http.StatusForbidden, "request_denied", err.Error())
		return
	}

	policyEngine, policyID, err := a.policyEngineForPolicy(user.ID, grant.PolicyID)
	if err != nil {
		logEntry.Outcome = "Fail"
		logEntry.HTTPStatus = http.StatusInternalServerError
		logEntry.PolicyID = policyID
		logEntry.ErrorMessage = err.Error()
		a.saveRequestLog(logEntry)
		writeError(w, http.StatusInternalServerError, "policy_error", err.Error())
		return
	}
	logEntry.PolicyID = policyID
	logEntry.PolicyName = a.policyDisplayNameForUser(user.ID, policyID)
	evalCtx := PolicyEvalContext{
		MailboxEmail: conn.MailboxEmail,
		FetchCalendarEvent: func(calendarID, eventID string) (map[string]any, error) {
			return a.fetchCalendarEvent(accessToken, calendarID, eventID)
		},
		FetchDriveCommentAuthorMe: func(fileID, commentID, replyID string) (bool, error) {
			return a.fetchDriveCommentAuthorMe(accessToken, fileID, commentID, replyID)
		},
		FetchDriveFileKind: func(fileID string) (string, error) {
			meta, err := a.fetchDriveFileMetadata(accessToken, fileID, "")
			if err != nil {
				return "", err
			}
			return drivePolicyKindFromMime(meta.MimeType), nil
		},
	}
	decision, err := policyEngine.EvaluateDecision(r.Method, target.normalizedPath, body, evalCtx)
	if err != nil {
		logEntry.Outcome = "Fail"
		logEntry.HTTPStatus = http.StatusInternalServerError
		logEntry.ErrorMessage = err.Error()
		a.saveRequestLog(logEntry)
		writeError(w, http.StatusInternalServerError, "policy_error", err.Error())
		return
	}
	if !decision.Allowed {
		_ = a.store.IncrementDailyStat(user.ID, false)
		logEntry.Outcome = "Fail"
		logEntry.HTTPStatus = http.StatusForbidden
		logEntry.ErrorMessage = decision.Reason
		a.saveRequestLog(logEntry)
		writeError(w, http.StatusForbidden, "request_denied", decision.Reason)
		return
	}
	if decision.RequiresHumanApproval {
		humanApproval, err := validateHumanApprovalHeader(r)
		if err != nil {
			_ = a.store.IncrementDailyStat(user.ID, false)
			logEntry.Outcome = "Fail"
			logEntry.HTTPStatus = http.StatusForbidden
			logEntry.PolicyCapabilityKey = decision.CapabilityKey
			logEntry.PolicyCapabilityTitle = decision.CapabilityTitle
			logEntry.PolicyRuleName = decision.RuleName
			logEntry.ErrorMessage = err.Error()
			a.saveRequestLog(logEntry)
			writeError(w, http.StatusForbidden, "human_approval_required", err.Error())
			return
		}
		logEntry.HumanApproval = humanApproval
	}
	logEntry.PolicyCapabilityKey = decision.CapabilityKey
	logEntry.PolicyCapabilityTitle = decision.CapabilityTitle
	logEntry.PolicyRuleName = decision.RuleName

	upstreamURL := target.upstreamBase + target.normalizedPath
	if rawQuery != "" {
		upstreamURL += "?" + rawQuery
	}
	upstreamReq, err := http.NewRequest(r.Method, upstreamURL, bytes.NewReader(body))
	if err != nil {
		logEntry.Outcome = "Fail"
		logEntry.HTTPStatus = http.StatusInternalServerError
		logEntry.ErrorMessage = err.Error()
		a.saveRequestLog(logEntry)
		writeError(w, http.StatusInternalServerError, "upstream_error", err.Error())
		return
	}
	copyWhitelistedRequestHeaders(upstreamReq.Header, r.Header)
	upstreamReq.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := a.client.Do(upstreamReq)
	if err != nil {
		logEntry.Outcome = "Fail"
		logEntry.HTTPStatus = http.StatusBadGateway
		logEntry.ErrorMessage = err.Error()
		a.saveRequestLog(logEntry)
		writeError(w, http.StatusBadGateway, "upstream_error", err.Error())
		return
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if shouldRetryWorkspaceAuth(resp.StatusCode, respBody) {
		if refreshedToken, refreshErr := a.refreshWorkspaceAccessToken(conn); refreshErr == nil {
			_ = resp.Body.Close()
			retryReq, retryErr := http.NewRequest(r.Method, upstreamURL, bytes.NewReader(body))
			if retryErr != nil {
				logEntry.Outcome = "Fail"
				logEntry.HTTPStatus = http.StatusInternalServerError
				logEntry.ErrorMessage = retryErr.Error()
				a.saveRequestLog(logEntry)
				writeError(w, http.StatusInternalServerError, "upstream_error", retryErr.Error())
				return
			}
			copyWhitelistedRequestHeaders(retryReq.Header, r.Header)
			retryReq.Header.Set("Authorization", "Bearer "+refreshedToken)
			resp, err = a.client.Do(retryReq)
			if err != nil {
				logEntry.Outcome = "Fail"
				logEntry.HTTPStatus = http.StatusBadGateway
				logEntry.ErrorMessage = err.Error()
				a.saveRequestLog(logEntry)
				writeError(w, http.StatusBadGateway, "upstream_error", err.Error())
				return
			}
			defer resp.Body.Close()
			respBody, _ = io.ReadAll(resp.Body)
		} else if refreshErr != nil {
			var authErr *workspaceAuthRequiredError
			if errors.As(refreshErr, &authErr) {
				logEntry.Outcome = "Fail"
				logEntry.HTTPStatus = http.StatusForbidden
				logEntry.ErrorMessage = authErr.Message
				a.saveRequestLog(logEntry)
				writeJSON(w, http.StatusForbidden, map[string]any{
					"error":           "workspace_reauth_required",
					"message":         authErr.AgentMessage(),
					"workspace":       authErr.WorkspaceEmail,
					"reauth_url":      authErr.ReauthURL,
					"request_id":      requestID,
					"reauth_required": true,
				})
				return
			}
		}
	}

	if postMoveFolderID != "" && resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if err := a.postCreateMoveToFolder(accessToken, target.serviceLabel, respBody, postMoveFolderID); err != nil {
			logEntry.Outcome = "Fail"
			logEntry.HTTPStatus = http.StatusBadGateway
			logEntry.UpstreamStatus = resp.StatusCode
			logEntry.ErrorMessage = err.Error()
			a.saveRequestLog(logEntry)
			writeError(w, http.StatusBadGateway, "workspace_move_error", err.Error())
			return
		}
	}

	copyResponseHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(respBody)
	allowed := resp.StatusCode >= 200 && resp.StatusCode < 400
	_ = a.store.IncrementDailyStat(user.ID, allowed)
	logEntry.HTTPStatus = resp.StatusCode
	logEntry.UpstreamStatus = resp.StatusCode
	if allowed {
		logEntry.Outcome = "Success"
	} else {
		logEntry.Outcome = "Fail"
		logEntry.ErrorMessage = fmt.Sprintf("upstream Google returned %d", resp.StatusCode)
	}
	a.saveRequestLog(logEntry)
}

func shouldRetryWorkspaceAuth(statusCode int, body []byte) bool {
	if statusCode == http.StatusUnauthorized {
		return true
	}
	if statusCode != http.StatusForbidden {
		return false
	}
	lower := strings.ToLower(string(body))
	return strings.Contains(lower, "invalid credentials") || strings.Contains(lower, "autherror")
}
func normalizeProxyTarget(rawPath, accountEmail string) (relayTarget, error) {
	switch {
	case strings.HasPrefix(rawPath, "/gmail.googleapis.com/"):
		normalizedPath, err := normalizeGmailPath(rawPath, accountEmail)
		if err != nil {
			return relayTarget{}, err
		}
		return relayTarget{serviceLabel: "gmail", upstreamBase: gmailAPIBase, normalizedPath: normalizedPath}, nil
	case strings.HasPrefix(rawPath, "/people.googleapis.com/"):
		path := normalizeGoogleMethodSeparators(strings.TrimPrefix(rawPath, "/people.googleapis.com"))
		if !isAllowedPeoplePath(path) {
			return relayTarget{}, fmt.Errorf("only People API contact search paths are allowed")
		}
		return relayTarget{serviceLabel: "people", upstreamBase: peopleAPIBase, normalizedPath: path}, nil
	case strings.HasPrefix(rawPath, "/calendar.googleapis.com/"):
		path := strings.TrimPrefix(rawPath, "/calendar.googleapis.com")
		if !strings.HasPrefix(path, "/calendar/v3/") {
			return relayTarget{}, fmt.Errorf("only /calendar/v3/... paths are allowed")
		}
		return relayTarget{serviceLabel: "calendar", upstreamBase: calendarAPIBase, normalizedPath: path}, nil
	case strings.HasPrefix(rawPath, "/drive.googleapis.com/"):
		path := strings.TrimPrefix(rawPath, "/drive.googleapis.com")
		if !strings.HasPrefix(path, "/drive/v3/") {
			return relayTarget{}, fmt.Errorf("only /drive/v3/... paths are allowed")
		}
		return relayTarget{serviceLabel: "drive", upstreamBase: driveAPIBase, normalizedPath: path}, nil
	case strings.HasPrefix(rawPath, "/docs.googleapis.com/"):
		path := normalizeGoogleMethodSeparators(strings.TrimPrefix(rawPath, "/docs.googleapis.com"))
		if !strings.HasPrefix(path, "/v1/documents") {
			return relayTarget{}, fmt.Errorf("only /v1/documents... paths are allowed")
		}
		return relayTarget{serviceLabel: "docs", upstreamBase: docsAPIBase, normalizedPath: path}, nil
	case strings.HasPrefix(rawPath, "/sheets.googleapis.com/"):
		path := normalizeGoogleMethodSeparators(strings.TrimPrefix(rawPath, "/sheets.googleapis.com"))
		if !strings.HasPrefix(path, "/v4/spreadsheets") {
			return relayTarget{}, fmt.Errorf("only /v4/spreadsheets... paths are allowed")
		}
		return relayTarget{serviceLabel: "sheets", upstreamBase: sheetsAPIBase, normalizedPath: path}, nil
	case strings.HasPrefix(rawPath, "/slides.googleapis.com/"):
		path := normalizeGoogleMethodSeparators(strings.TrimPrefix(rawPath, "/slides.googleapis.com"))
		if !strings.HasPrefix(path, "/v1/presentations") {
			return relayTarget{}, fmt.Errorf("only /v1/presentations... paths are allowed")
		}
		return relayTarget{serviceLabel: "slides", upstreamBase: slidesAPIBase, normalizedPath: path}, nil
	default:
		return relayTarget{}, fmt.Errorf("path must start with /gmail.googleapis.com/, /people.googleapis.com/, /calendar.googleapis.com/, /drive.googleapis.com/, /docs.googleapis.com/, /sheets.googleapis.com/, or /slides.googleapis.com/")
	}
}
func isAllowedPeoplePath(path string) bool {
	switch path {
	case "/v1/people:searchContacts", "/v1/people:searchDirectoryPeople", "/v1/otherContacts:search":
		return true
	default:
		return false
	}
}
func normalizeGoogleMethodSeparators(path string) string {
	return strings.NewReplacer("%3A", ":", "%3a", ":").Replace(path)
}
func normalizeGmailPath(rawPath, mailboxEmail string) (string, error) {
	if !strings.HasPrefix(rawPath, "/gmail.googleapis.com/") {
		return "", fmt.Errorf("path must start with /gmail.googleapis.com/")
	}
	path := strings.TrimPrefix(rawPath, "/gmail.googleapis.com")
	if !strings.HasPrefix(path, "/gmail/v1/users/") {
		return "", fmt.Errorf("only /gmail/v1/users/... paths are allowed")
	}
	parts := strings.Split(path, "/")
	if len(parts) < 5 {
		return "", fmt.Errorf("invalid Gmail API path")
	}
	userID := strings.ToLower(parts[4])
	mailboxEmail = strings.ToLower(strings.TrimSpace(mailboxEmail))
	if userID != "me" && userID != mailboxEmail {
		return "", fmt.Errorf("path may only target users/me or the connected account email")
	}
	parts[4] = "me"
	return strings.Join(parts, "/"), nil
}
func copyWhitelistedRequestHeaders(dst, src http.Header) {
	for _, key := range []string{"Accept", "Content-Type"} {
		for _, value := range src.Values(key) {
			dst.Add(key, value)
		}
	}
}
func copyResponseHeaders(dst, src http.Header) {
	for key, values := range src {
		if strings.EqualFold(key, "Transfer-Encoding") {
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}
