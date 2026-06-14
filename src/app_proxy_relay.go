package main

import (
	"bytes"
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

func (a *App) policyEngineForWorkspace(userID, mailboxEmail string) (*PolicyEngine, string, error) {
	capabilities, policyID, err := a.store.WorkspacePolicyCapabilities(userID, mailboxEmail)
	if err != nil {
		return nil, "", err
	}
	engine, err := NewPolicyEngine(capabilities)
	if err != nil {
		return nil, "", err
	}
	return engine, policyID, nil
}
func (a *App) handleProxyRelay(w http.ResponseWriter, r *http.Request) {
	proxyToken := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	if proxyToken == "" {
		writeError(w, http.StatusUnauthorized, "auth_required", "missing Bearer token")
		return
	}
	user, err := a.store.FindUserByProxyToken(a.crypto, proxyToken)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "auth_error", err.Error())
		return
	}
	if user == nil || user.IsSuspended {
		writeError(w, http.StatusUnauthorized, "auth_error", "invalid or suspended user")
		return
	}
	_ = a.store.TouchProxyTokenUsage(user.ID)

	conn, err := a.resolveWorkspaceFromQuery(user.ID, r)
	if err != nil {
		writeError(w, http.StatusForbidden, "workspace_not_found", err.Error())
		return
	}

	target, err := normalizeProxyTarget(r.URL.EscapedPath(), conn.MailboxEmail)
	if err != nil {
		_ = a.store.IncrementDailyStat(user.ID, false)
		a.logDeniedRequest(r, user, conn.MailboxEmail, nil, err.Error())
		writeError(w, http.StatusForbidden, "request_denied", err.Error())
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, a.cfg.MaxRequestBodyBytes+1))
	if err != nil {
		writeError(w, http.StatusBadRequest, "body_error", err.Error())
		return
	}
	if int64(len(body)) > a.cfg.MaxRequestBodyBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "body_too_large", "request body too large")
		return
	}

	accessToken, err := a.getValidWorkspaceAccessToken(conn)
	if err != nil {
		writeError(w, http.StatusBadGateway, "workspace_token_error", err.Error())
		return
	}

	query := r.URL.Query()
	query.Del("workspace")
	body, rawQuery, postMoveFolderID, err := a.rewriteWorkspaceRequest(user.ID, conn.MailboxEmail, accessToken, r.Method, target, body, query)
	if err != nil {
		_ = a.store.IncrementDailyStat(user.ID, false)
		a.logDeniedRequest(r, user, conn.MailboxEmail, body, err.Error())
		writeError(w, http.StatusForbidden, "request_denied", err.Error())
		return
	}

	policyEngine, policyID, err := a.policyEngineForWorkspace(user.ID, conn.MailboxEmail)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "policy_error", err.Error())
		return
	}
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
	allowed, reason, err := policyEngine.Evaluate(r.Method, target.normalizedPath, body, evalCtx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "policy_error", err.Error())
		return
	}
	if !allowed {
		_ = a.store.IncrementDailyStat(user.ID, false)
		a.logDeniedRequest(r, user, conn.MailboxEmail, body, "policy "+policyID+": "+reason)
		writeError(w, http.StatusForbidden, "request_denied", reason)
		return
	}

	upstreamURL := target.upstreamBase + target.normalizedPath
	if rawQuery != "" {
		upstreamURL += "?" + rawQuery
	}
	upstreamReq, err := http.NewRequest(r.Method, upstreamURL, bytes.NewReader(body))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "upstream_error", err.Error())
		return
	}
	copyWhitelistedRequestHeaders(upstreamReq.Header, r.Header)
	upstreamReq.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := a.client.Do(upstreamReq)
	if err != nil {
		writeError(w, http.StatusBadGateway, "upstream_error", err.Error())
		return
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if postMoveFolderID != "" && resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if err := a.postCreateMoveToFolder(accessToken, target.serviceLabel, respBody, postMoveFolderID); err != nil {
			writeError(w, http.StatusBadGateway, "workspace_move_error", err.Error())
			return
		}
	}

	copyResponseHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(respBody)
	_ = a.store.IncrementDailyStat(user.ID, true)
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
func (a *App) logDeniedRequest(r *http.Request, user *User, mailbox string, body []byte, reason string) {
	headers := map[string]string{}
	for key, values := range r.Header {
		if strings.EqualFold(key, "Authorization") || strings.EqualFold(key, "Cookie") {
			continue
		}
		headers[key] = strings.Join(values, ",")
	}
	_ = a.deniedLog.Log(DeniedLogEntry{
		UserID:     user.ID,
		Mailbox:    mailbox,
		Method:     r.Method,
		Path:       r.URL.Path,
		Query:      r.URL.RawQuery,
		Headers:    headers,
		Body:       sanitizeBody(body),
		Reason:     reason,
		RemoteAddr: r.RemoteAddr,
	})
}
func sanitizeBody(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	text := string(body)
	lower := strings.ToLower(text)
	if strings.Contains(lower, `"raw"`) {
		return `{"redacted":"gmail raw MIME content omitted"}`
	}
	if len(text) > 4096 {
		return text[:4096] + "...[truncated]"
	}
	return text
}
