// Copyright (c) 2026 Opensense Ltd. (Hong Kong). All rights reserved.
// Proprietary software. No use, copy, modification, distribution, disclosure,
// or reverse engineering is permitted without prior written authorization
// from Opensense Ltd.

package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type AgentAuthContext struct {
	User  *User
	Agent *AgentAccess
}

func sessionTimeoutHours(settings *UserSettings) int {
	if settings == nil {
		return defaultUserSessionTimeoutHours
	}
	if settings.SessionTimeoutHours < 1 || settings.SessionTimeoutHours > maxUserSessionTimeoutHours {
		return defaultUserSessionTimeoutHours
	}
	return settings.SessionTimeoutHours
}

func sessionExpiryFromStart(start time.Time, settings *UserSettings, fallback time.Duration) time.Time {
	timeoutHours := sessionTimeoutHours(settings)
	if timeoutHours < 1 {
		if fallback <= 0 {
			fallback = time.Duration(defaultUserSessionTimeoutHours) * time.Hour
		}
		return start.Add(fallback)
	}
	return start.Add(time.Duration(timeoutHours) * time.Hour)
}

func (a *App) sessionAndUserFromRequest(r *http.Request) (*Session, *User, error) {
	cookie, err := r.Cookie(a.cfg.SessionCookieName)
	if err != nil {
		return nil, nil, nil
	}
	session, err := a.store.FindSessionByTokenHash(SHA256Hex(cookie.Value))
	if err != nil || session == nil {
		return nil, nil, err
	}
	if session.ExpiresAt.Before(nowUTC()) {
		_ = a.store.DeleteSessionByTokenHash(session.TokenHash)
		return nil, nil, nil
	}
	user, err := a.store.FindUserByID(session.UserID)
	if err != nil || user == nil || user.IsSuspended {
		return nil, nil, err
	}
	return session, user, nil
}

func (a *App) currentAgentFromBearer(r *http.Request) (*AgentAuthContext, error) {
	agentToken := parseBearerToken(r.Header.Get("Authorization"))
	if agentToken == "" {
		return nil, nil
	}
	user, agent, err := a.store.FindAgentByToken(a.crypto, agentToken)
	if err != nil || user == nil || agent == nil {
		return nil, err
	}
	return &AgentAuthContext{User: user, Agent: agent}, nil
}
func (a *App) currentUserFromUserBackendAPIBearer(r *http.Request) (*User, error) {
	backendToken := parseBearerToken(r.Header.Get("Authorization"))
	if backendToken == "" {
		return nil, nil
	}
	return a.store.FindUserByUserBackendAPIToken(a.crypto, backendToken)
}
func (a *App) requireUserBackendAPIUser(w http.ResponseWriter, r *http.Request) *User {
	user, err := a.currentUserFromUserBackendAPIBearer(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "auth_error", err.Error())
		return nil
	}
	if user == nil || user.IsSuspended {
		writeError(w, http.StatusUnauthorized, "privileged_auth_required", "valid end-user backend API bearer token required")
		return nil
	}
	_ = a.store.TouchUserBackendAPITokenUsage(user.ID)
	return user
}
func (a *App) currentUserFromSession(r *http.Request) (*User, error) {
	session, user, err := a.sessionAndUserFromRequest(r)
	if err != nil || session == nil || user == nil {
		return nil, err
	}
	if !session.SecondFactorVerified {
		return nil, nil
	}
	return user, nil
}

func (a *App) currentPendingSecondFactorUserFromSession(r *http.Request) (*Session, *User, error) {
	session, user, err := a.sessionAndUserFromRequest(r)
	if err != nil || session == nil || user == nil {
		return nil, nil, err
	}
	if session.SecondFactorVerified {
		return nil, nil, nil
	}
	return session, user, nil
}

func (a *App) requireSessionUser(w http.ResponseWriter, r *http.Request) *User {
	session, user, err := a.sessionAndUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "session_error", err.Error())
		return nil
	}
	if session == nil || user == nil {
		http.Redirect(w, r, "/", http.StatusFound)
		return nil
	}
	if !session.SecondFactorVerified {
		http.Redirect(w, r, "/auth/2fa", http.StatusFound)
		return nil
	}
	mustEnroll, err := a.userMustCompleteOrganizationTwoFactor(user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "organization_policy_error", err.Error())
		return nil
	}
	if mustEnroll && !pathAllowsOrganizationTwoFactorEnrollment(r.URL.Path) {
		http.Redirect(w, r, twoFactorEnrollmentURLForRequest(r), http.StatusFound)
		return nil
	}
	return user
}
func (a *App) csrfTokenFromRequest(r *http.Request) string {
	cookie, err := r.Cookie(a.cfg.SessionCookieName)
	if err != nil || cookie.Value == "" {
		return ""
	}
	return a.csrfTokenFromSession(cookie.Value)
}
func (a *App) csrfTokenFromSession(sessionToken string) string {
	mac := hmac.New(sha256.New, a.cfg.EncryptionKey)
	_, _ = mac.Write([]byte("csrf:" + sessionToken))
	return "csrf_" + hex.EncodeToString(mac.Sum(nil))
}
func (a *App) requireSessionCSRF(w http.ResponseWriter, r *http.Request) bool {
	expected := a.csrfTokenFromRequest(r)
	provided := strings.TrimSpace(r.FormValue("csrf_token"))
	if expected == "" || provided == "" || !subtleEqual(expected, provided) {
		writeError(w, http.StatusForbidden, "csrf_denied", "invalid CSRF token")
		return false
	}
	return true
}
func (a *App) resolveWorkspaceFromRequest(userID string, r *http.Request) (*GmailConnection, error) {
	workspace := strings.TrimSpace(r.FormValue("workspace"))
	if workspace == "" {
		workspace = strings.TrimSpace(r.URL.Query().Get("workspace"))
	}
	if workspace == "" {
		return nil, fmt.Errorf("workspace is required")
	}
	return a.store.ResolveGmailConnection(userID, workspace)
}
func (a *App) resolveWorkspaceFromQuery(userID string, r *http.Request) (*GmailConnection, error) {
	workspace := strings.TrimSpace(r.URL.Query().Get("workspace"))
	if workspace == "" {
		return nil, fmt.Errorf("workspace is required")
	}
	return a.store.ResolveGmailConnection(userID, workspace)
}
func (a *App) requireAdminUser(w http.ResponseWriter, r *http.Request) *User {
	user := a.requireSessionUser(w, r)
	if user == nil {
		return nil
	}
	if !user.IsAdmin {
		writeError(w, http.StatusForbidden, "admin_required", "admin privileges required")
		return nil
	}
	return user
}
func subtleEqual(aVal, bVal string) bool {
	if len(aVal) != len(bVal) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(aVal), []byte(bVal)) == 1
}
