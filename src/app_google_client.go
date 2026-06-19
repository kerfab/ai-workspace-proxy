// Copyright (c) 2026 Opensense Ltd. (Hong Kong). All rights reserved.
// Proprietary software. No use, copy, modification, distribution, disclosure,
// or reverse engineering is permitted without prior written authorization
// from Opensense Ltd.

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type workspaceAuthRequiredError struct {
	WorkspaceEmail string
	Message        string
	ReauthURL      string
}

func (e *workspaceAuthRequiredError) Error() string {
	return e.Message
}

func (e *workspaceAuthRequiredError) AgentMessage() string {
	if strings.TrimSpace(e.ReauthURL) == "" {
		return e.Message
	}
	return e.Message + " This URL must be opened by the human operator in a browser, not by the AI agent itself: " + e.ReauthURL
}

type GoogleTokenResponse struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
	TokenType    string `json:"token_type"`
	IDToken      string `json:"id_token"`
}
type GoogleUserInfo struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}
type GmailProfile struct {
	EmailAddress string `json:"emailAddress"`
}

func (a *App) exchangeCodeForToken(code, clientID, clientSecret, redirectURI string) (*GoogleTokenResponse, error) {
	form := url.Values{}
	form.Set("code", code)
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("redirect_uri", redirectURI)
	form.Set("grant_type", "authorization_code")
	return a.postFormToken(form)
}
func (a *App) refreshToken(refreshToken, clientID, clientSecret string) (*GoogleTokenResponse, error) {
	form := url.Values{}
	form.Set("refresh_token", refreshToken)
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("grant_type", "refresh_token")
	return a.postFormToken(form)
}
func (a *App) postFormToken(form url.Values) (*GoogleTokenResponse, error) {
	req, err := http.NewRequest(http.MethodPost, googleTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, string(raw))
	}
	var out GoogleTokenResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
func (a *App) fetchUserInfo(accessToken string) (*GoogleUserInfo, error) {
	req, err := http.NewRequest(http.MethodGet, googleUserInfoURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("userinfo endpoint returned %d: %s", resp.StatusCode, string(raw))
	}
	var out GoogleUserInfo
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
func (a *App) fetchGmailProfile(accessToken string) (*GmailProfile, error) {
	req, err := http.NewRequest(http.MethodGet, gmailProfileURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("gmail profile endpoint returned %d: %s", resp.StatusCode, string(raw))
	}
	var out GmailProfile
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
func (a *App) fetchCalendarEvent(accessToken, calendarID, eventID string) (map[string]any, error) {
	u := calendarAPIBase + "/calendar/v3/calendars/" + url.PathEscape(calendarID) + "/events/" + url.PathEscape(eventID)
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("calendar event returned %d: %s", resp.StatusCode, string(raw))
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func workspaceHasStoredOAuth(conn *GmailConnection) bool {
	return conn != nil && strings.TrimSpace(conn.RefreshTokenEnc) != ""
}

func workspaceAuthRefreshLikelyNeedsReauth(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "invalid_grant")
}

func (a *App) newWorkspaceAuthRequiredError(userID, mailboxEmail, message string) *workspaceAuthRequiredError {
	rawToken, err := RandomToken("wra_", 24)
	if err != nil {
		return &workspaceAuthRequiredError{
			WorkspaceEmail: mailboxEmail,
			Message:        message,
		}
	}
	expiresAt := nowUTC().Add(workspaceReauthTokenTTL)
	if err := a.store.SaveWorkspaceReauthToken(SHA256Hex(rawToken), userID, mailboxEmail, expiresAt); err != nil {
		return &workspaceAuthRequiredError{
			WorkspaceEmail: mailboxEmail,
			Message:        message,
		}
	}
	return &workspaceAuthRequiredError{
		WorkspaceEmail: mailboxEmail,
		Message:        message,
		ReauthURL:      a.cfg.BaseURL + "/auth/workspace/reauth?token=" + url.QueryEscape(rawToken),
	}
}

func (a *App) getValidWorkspaceAccessToken(conn *GmailConnection) (string, error) {
	return a.getValidWorkspaceAccessTokenWithMode(conn, false)
}

func (a *App) refreshWorkspaceAccessToken(conn *GmailConnection) (string, error) {
	return a.getValidWorkspaceAccessTokenWithMode(conn, true)
}

func (a *App) getValidWorkspaceAccessTokenWithMode(conn *GmailConnection, forceRefresh bool) (string, error) {
	if conn == nil {
		return "", fmt.Errorf("workspace is required")
	}
	if !forceRefresh && conn.AccessTokenEnc != "" && conn.TokenExpiry.After(nowUTC().Add(1*time.Minute)) {
		return a.crypto.Decrypt(conn.AccessTokenEnc)
	}
	if !workspaceHasStoredOAuth(conn) {
		return "", a.newWorkspaceAuthRequiredError(conn.UserID, conn.MailboxEmail, fmt.Sprintf("Google authorization is disconnected for Workspace %s.", conn.MailboxEmail))
	}
	refreshToken, err := a.crypto.Decrypt(conn.RefreshTokenEnc)
	if err != nil {
		return "", err
	}
	refreshed, err := a.refreshToken(refreshToken, a.cfg.WorkspaceClientID, a.cfg.WorkspaceClientSecret)
	if err != nil {
		if workspaceAuthRefreshLikelyNeedsReauth(err) {
			return "", a.newWorkspaceAuthRequiredError(conn.UserID, conn.MailboxEmail, fmt.Sprintf("Google authorization for Workspace %s has expired or was revoked.", conn.MailboxEmail))
		}
		return "", err
	}
	accessEnc, err := a.crypto.Encrypt(refreshed.AccessToken)
	if err != nil {
		return "", err
	}
	conn.AccessTokenEnc = accessEnc
	conn.TokenExpiry = nowUTC().Add(time.Duration(refreshed.ExpiresIn) * time.Second)
	if refreshed.Scope != "" {
		conn.Scopes = refreshed.Scope
	}
	if err := a.store.SaveGmailConnection(conn); err != nil {
		return "", err
	}
	return refreshed.AccessToken, nil
}
