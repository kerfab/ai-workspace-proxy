package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Store struct {
	db *SQLiteDB
}

func NewStore(db *SQLiteDB) *Store {
	return &Store{db: db}
}

func (s *Store) Init() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			email TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			picture TEXT NOT NULL,
			is_admin INTEGER NOT NULL DEFAULT 0,
			is_suspended INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			token_hash TEXT NOT NULL UNIQUE,
			expires_at TEXT NOT NULL,
			created_at TEXT NOT NULL,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS oauth_states (
			state_hash TEXT PRIMARY KEY,
			purpose TEXT NOT NULL,
			user_id TEXT,
			expires_at TEXT NOT NULL,
			created_at TEXT NOT NULL,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS proxy_tokens (
			user_id TEXT PRIMARY KEY,
			token_enc TEXT NOT NULL,
			token_hint TEXT NOT NULL,
			last_used_at TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS gmail_connections (
			user_id TEXT NOT NULL,
			mailbox_email TEXT NOT NULL,
			friendly_name TEXT NOT NULL,
			policy_id TEXT NOT NULL DEFAULT 'system',
			scopes TEXT NOT NULL,
			access_token_enc TEXT,
			refresh_token_enc TEXT NOT NULL,
			token_expiry TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			PRIMARY KEY(user_id, mailbox_email),
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS workspace_order (
			user_id TEXT NOT NULL,
			mailbox_email TEXT NOT NULL,
			sort_order INTEGER NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			PRIMARY KEY(user_id, mailbox_email),
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY(user_id, mailbox_email) REFERENCES gmail_connections(user_id, mailbox_email) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS drive_folder_refs (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			mailbox_email TEXT NOT NULL,
			reference_name TEXT NOT NULL,
			reference_key TEXT NOT NULL,
			folder_url TEXT NOT NULL,
			folder_id TEXT NOT NULL,
			folder_name TEXT NOT NULL,
			resource_key TEXT NOT NULL DEFAULT '',
			allow_docs INTEGER NOT NULL DEFAULT 1,
			allow_sheets INTEGER NOT NULL DEFAULT 1,
			allow_slides INTEGER NOT NULL DEFAULT 1,
			allow_drive_files INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
			UNIQUE(user_id, mailbox_email, reference_key)
		);`,
		`CREATE TABLE IF NOT EXISTS drive_folder_tree_cache (
			user_id TEXT NOT NULL,
			root_ref_id TEXT NOT NULL,
			folder_id TEXT NOT NULL,
			parent_folder_id TEXT NOT NULL DEFAULT '',
			folder_name TEXT NOT NULL,
			path TEXT NOT NULL,
			path_key TEXT NOT NULL,
			depth INTEGER NOT NULL,
			resource_key TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			PRIMARY KEY (user_id, root_ref_id, folder_id),
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY(root_ref_id) REFERENCES drive_folder_refs(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS user_daily_stats (
			user_id TEXT NOT NULL,
			day TEXT NOT NULL,
			allowed_count INTEGER NOT NULL DEFAULT 0,
			denied_count INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (user_id, day),
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS user_settings (
			user_id TEXT PRIMARY KEY,
			timezone TEXT NOT NULL,
			default_policy_id TEXT NOT NULL DEFAULT 'system',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS user_policies (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			name TEXT NOT NULL,
			capabilities TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
			UNIQUE(user_id, name)
		);`,
	}
	for _, stmt := range stmts {
		if err := s.db.Exec(stmt); err != nil {
			return err
		}
	}
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_token_hash ON sessions(token_hash);`,
		`CREATE INDEX IF NOT EXISTS idx_gmail_connections_user ON gmail_connections(user_id);`,
		`CREATE INDEX IF NOT EXISTS idx_workspace_order_user_sort ON workspace_order(user_id, sort_order);`,
		`CREATE INDEX IF NOT EXISTS idx_user_policies_user ON user_policies(user_id);`,
		`CREATE INDEX IF NOT EXISTS idx_drive_folder_refs_user_workspace ON drive_folder_refs(user_id, mailbox_email);`,
		`CREATE INDEX IF NOT EXISTS idx_drive_folder_tree_root ON drive_folder_tree_cache(user_id, root_ref_id);`,
		`CREATE INDEX IF NOT EXISTS idx_drive_folder_tree_path ON drive_folder_tree_cache(user_id, root_ref_id, path_key);`,
		`CREATE INDEX IF NOT EXISTS idx_drive_folder_tree_folder ON drive_folder_tree_cache(user_id, folder_id);`,
	}
	for _, stmt := range indexes {
		if err := s.db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) UserCount() (int, error) {
	row, err := s.db.QueryOne(`SELECT COUNT(*) AS cnt FROM users;`)
	if err != nil || row == nil {
		return 0, err
	}
	return atoiSafe(row["cnt"]), nil
}

func (s *Store) FindUserByEmail(email string) (*User, error) {
	row, err := s.db.QueryOne(`SELECT * FROM users WHERE email = ?;`, email)
	if err != nil {
		return nil, err
	}
	return userFromRow(row), nil
}

func (s *Store) FindUserByID(id string) (*User, error) {
	row, err := s.db.QueryOne(`SELECT * FROM users WHERE id = ?;`, id)
	if err != nil {
		return nil, err
	}
	return userFromRow(row), nil
}

func (s *Store) ListUsers() ([]User, error) {
	rows, err := s.db.Query(`SELECT * FROM users ORDER BY created_at ASC;`)
	if err != nil {
		return nil, err
	}
	out := make([]User, 0, len(rows))
	for _, row := range rows {
		if u := userFromRow(row); u != nil {
			out = append(out, *u)
		}
	}
	return out, nil
}

func (s *Store) CreateOrUpdateUser(email, name, picture string, isAdmin bool) (*User, error) {
	existing, err := s.FindUserByEmail(email)
	if err != nil {
		return nil, err
	}
	now := nowUTC()
	if existing == nil {
		id, err := RandomToken("usr_", 16)
		if err != nil {
			return nil, err
		}
		if err := s.db.Exec(`INSERT INTO users (id, email, name, picture, is_admin, is_suspended, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, 0, ?, ?);`, id, email, name, picture, isAdmin, now, now); err != nil {
			return nil, err
		}
		return s.FindUserByID(id)
	}
	if err := s.db.Exec(`UPDATE users
		SET name = ?, picture = ?, is_admin = ?, updated_at = ?
		WHERE id = ?;`, name, picture, isAdmin, now, existing.ID); err != nil {
		return nil, err
	}
	return s.FindUserByID(existing.ID)
}

func (s *Store) SetUserSuspended(userID string, suspended bool) error {
	return s.db.Exec(`UPDATE users SET is_suspended = ?, updated_at = ? WHERE id = ?;`, suspended, nowUTC(), userID)
}

func (s *Store) DeleteUser(userID string) error {
	for _, stmt := range []struct {
		q    string
		args []any
	}{
		{`DELETE FROM sessions WHERE user_id = ?;`, []any{userID}},
		{`DELETE FROM oauth_states WHERE user_id = ?;`, []any{userID}},
		{`DELETE FROM proxy_tokens WHERE user_id = ?;`, []any{userID}},
		{`DELETE FROM workspace_order WHERE user_id = ?;`, []any{userID}},
		{`DELETE FROM drive_folder_refs WHERE user_id = ?;`, []any{userID}},
		{`DELETE FROM gmail_connections WHERE user_id = ?;`, []any{userID}},
		{`DELETE FROM user_daily_stats WHERE user_id = ?;`, []any{userID}},
		{`DELETE FROM user_settings WHERE user_id = ?;`, []any{userID}},
		{`DELETE FROM user_policies WHERE user_id = ?;`, []any{userID}},
		{`DELETE FROM users WHERE id = ?;`, []any{userID}},
	} {
		if err := s.db.Exec(stmt.q, stmt.args...); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) CreateSession(userID, tokenHash string, expiresAt time.Time) error {
	id, err := RandomToken("ses_", 12)
	if err != nil {
		return err
	}
	return s.db.Exec(`INSERT INTO sessions (id, user_id, token_hash, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?);`, id, userID, tokenHash, expiresAt, nowUTC())
}

func (s *Store) FindSessionByTokenHash(tokenHash string) (*Session, error) {
	row, err := s.db.QueryOne(`SELECT * FROM sessions WHERE token_hash = ?;`, tokenHash)
	if err != nil {
		return nil, err
	}
	return sessionFromRow(row), nil
}

func (s *Store) DeleteSessionByTokenHash(tokenHash string) error {
	return s.db.Exec(`DELETE FROM sessions WHERE token_hash = ?;`, tokenHash)
}

func (s *Store) DeleteExpiredSessions() error {
	return s.db.Exec(`DELETE FROM sessions WHERE expires_at < ?;`, nowUTC())
}

func (s *Store) SaveOAuthState(stateHash, purpose, userID string, expiresAt time.Time) error {
	var uid any
	if strings.TrimSpace(userID) != "" {
		uid = userID
	}
	return s.db.Exec(`INSERT OR REPLACE INTO oauth_states (state_hash, purpose, user_id, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?);`, stateHash, purpose, uid, expiresAt, nowUTC())
}

func (s *Store) ConsumeOAuthState(stateHash, purpose string) (string, bool, error) {
	row, err := s.db.QueryOne(`SELECT * FROM oauth_states WHERE state_hash = ? AND purpose = ?;`, stateHash, purpose)
	if err != nil {
		return "", false, err
	}
	if row == nil {
		return "", false, nil
	}
	_ = s.db.Exec(`DELETE FROM oauth_states WHERE state_hash = ?;`, stateHash)
	if parseTime(row["expires_at"]).Before(nowUTC()) {
		return "", false, nil
	}
	return row["user_id"], true, nil
}

func (s *Store) SaveProxyToken(userID, tokenEnc, tokenHint string) error {
	now := nowUTC()
	return s.db.Exec(`INSERT INTO proxy_tokens (user_id, token_enc, token_hint, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			token_enc = excluded.token_enc,
			token_hint = excluded.token_hint,
			updated_at = excluded.updated_at;`,
		userID, tokenEnc, tokenHint, now, now)
}

func (s *Store) GetProxyTokenRecord(userID string) (map[string]string, error) {
	return s.db.QueryOne(`SELECT * FROM proxy_tokens WHERE user_id = ?;`, userID)
}

func (s *Store) TouchProxyTokenUsage(userID string) error {
	now := nowUTC()
	return s.db.Exec(`UPDATE proxy_tokens SET last_used_at = ?, updated_at = ? WHERE user_id = ?;`, now, now, userID)
}

func (s *Store) SaveGmailConnection(conn *GmailConnection) error {
	now := nowUTC()
	conn.MailboxEmail = normalizeEmail(conn.MailboxEmail)
	friendlyName := strings.TrimSpace(conn.FriendlyName)
	if friendlyName == "" {
		friendlyName = conn.MailboxEmail
	}
	existing, err := s.GetGmailConnection(conn.UserID, conn.MailboxEmail)
	if err != nil {
		return err
	}
	policyID := strings.TrimSpace(conn.PolicyID)
	if policyID == "" {
		policyID = systemPolicyID
	}
	if existing != nil && strings.TrimSpace(conn.PolicyID) == "" {
		policyID = existing.PolicyID
	}
	if policyID != systemPolicyID {
		policy, err := s.GetUserPolicy(conn.UserID, policyID)
		if err != nil {
			return err
		}
		if policy == nil {
			return fmt.Errorf("policy not found")
		}
	}
	if err := s.db.Exec(`INSERT INTO gmail_connections (user_id, mailbox_email, friendly_name, policy_id, scopes, access_token_enc, refresh_token_enc, token_expiry, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id, mailbox_email) DO UPDATE SET
			friendly_name = excluded.friendly_name,
			policy_id = excluded.policy_id,
			scopes = excluded.scopes,
			access_token_enc = excluded.access_token_enc,
			refresh_token_enc = excluded.refresh_token_enc,
			token_expiry = excluded.token_expiry,
			updated_at = excluded.updated_at;`,
		conn.UserID, conn.MailboxEmail, friendlyName, policyID, conn.Scopes, conn.AccessTokenEnc, conn.RefreshTokenEnc, conn.TokenExpiry, now, now); err != nil {
		return err
	}
	if existing == nil {
		if err := s.db.Exec(`UPDATE workspace_order SET sort_order = sort_order + 1, updated_at = ? WHERE user_id = ?;`, now, conn.UserID); err != nil {
			return err
		}
		return s.db.Exec(`INSERT INTO workspace_order (user_id, mailbox_email, sort_order, created_at, updated_at)
			VALUES (?, ?, 0, ?, ?);`, conn.UserID, conn.MailboxEmail, now, now)
	}
	return nil
}

func (s *Store) GetGmailConnection(userID, mailboxEmail string) (*GmailConnection, error) {
	row, err := s.db.QueryOne(`SELECT * FROM gmail_connections WHERE user_id = ? AND mailbox_email = ?;`, userID, normalizeEmail(mailboxEmail))
	if err != nil {
		return nil, err
	}
	return gmailConnectionFromRow(row), nil
}

func (s *Store) ListGmailConnections(userID string) ([]GmailConnection, error) {
	rows, err := s.db.Query(`SELECT g.* FROM gmail_connections g
		LEFT JOIN workspace_order wo ON wo.user_id = g.user_id AND wo.mailbox_email = g.mailbox_email
		WHERE g.user_id = ?
		ORDER BY COALESCE(wo.sort_order, 999999) ASC, g.mailbox_email ASC;`, userID)
	if err != nil {
		return nil, err
	}
	out := make([]GmailConnection, 0, len(rows))
	for _, row := range rows {
		if conn := gmailConnectionFromRow(row); conn != nil {
			out = append(out, *conn)
		}
	}
	return out, nil
}

func (s *Store) ResolveGmailConnection(userID, workspace string) (*GmailConnection, error) {
	workspace = strings.TrimSpace(workspace)
	if workspace == "" {
		return nil, fmt.Errorf("workspace is required")
	}
	rows, err := s.db.Query(`SELECT g.* FROM gmail_connections g
		LEFT JOIN workspace_order wo ON wo.user_id = g.user_id AND wo.mailbox_email = g.mailbox_email
		WHERE g.user_id = ?
		ORDER BY COALESCE(wo.sort_order, 999999) ASC, g.mailbox_email ASC;`, userID)
	if err != nil {
		return nil, err
	}
	var friendlyMatches []GmailConnection
	for _, row := range rows {
		if conn := gmailConnectionFromRow(row); conn != nil && conn.FriendlyName == workspace {
			friendlyMatches = append(friendlyMatches, *conn)
		}
	}
	if len(friendlyMatches) == 1 {
		return &friendlyMatches[0], nil
	}
	if len(friendlyMatches) > 1 {
		return nil, fmt.Errorf("workspace friendly name is ambiguous; use the full email address")
	}
	for _, row := range rows {
		if conn := gmailConnectionFromRow(row); conn != nil && normalizeEmail(conn.MailboxEmail) == normalizeEmail(workspace) {
			return conn, nil
		}
	}
	return nil, fmt.Errorf("workspace not found")
}

func (s *Store) UpdateGmailConnectionFriendlyName(userID, mailboxEmail, friendlyName string) error {
	friendlyName = strings.TrimSpace(friendlyName)
	if friendlyName == "" {
		return fmt.Errorf("friendly name cannot be empty")
	}
	return s.db.Exec(`UPDATE gmail_connections SET friendly_name = ?, updated_at = ? WHERE user_id = ? AND mailbox_email = ?;`,
		friendlyName, nowUTC(), userID, normalizeEmail(mailboxEmail))
}

func (s *Store) UpdateGmailConnectionPolicy(userID, mailboxEmail, policyID string) error {
	policyID = strings.TrimSpace(policyID)
	if policyID == "" {
		policyID = systemPolicyID
	}
	if policyID != systemPolicyID {
		policy, err := s.GetUserPolicy(userID, policyID)
		if err != nil {
			return err
		}
		if policy == nil {
			return fmt.Errorf("policy not found")
		}
	}
	conn, err := s.GetGmailConnection(userID, mailboxEmail)
	if err != nil {
		return err
	}
	if conn == nil {
		return fmt.Errorf("workspace not found")
	}
	return s.db.Exec(`UPDATE gmail_connections SET policy_id = ?, updated_at = ? WHERE user_id = ? AND mailbox_email = ?;`,
		policyID, nowUTC(), userID, normalizeEmail(mailboxEmail))
}

func (s *Store) DeleteGmailConnection(userID, mailboxEmail string) error {
	mailboxEmail = normalizeEmail(mailboxEmail)
	refs, err := s.ListDriveFolderRefs(userID, mailboxEmail)
	if err != nil {
		return err
	}
	for _, ref := range refs {
		if err := s.DeleteDriveFolderRef(userID, mailboxEmail, ref.ID); err != nil {
			return err
		}
	}
	if err := s.db.Exec(`DELETE FROM workspace_order WHERE user_id = ? AND mailbox_email = ?;`, userID, mailboxEmail); err != nil {
		return err
	}
	return s.db.Exec(`DELETE FROM gmail_connections WHERE user_id = ? AND mailbox_email = ?;`, userID, mailboxEmail)
}

func (s *Store) SaveWorkspaceOrder(userID string, mailboxEmails []string) error {
	known, err := s.ListGmailConnections(userID)
	if err != nil {
		return err
	}
	knownSet := map[string]bool{}
	for _, conn := range known {
		knownSet[conn.MailboxEmail] = true
	}
	seen := map[string]bool{}
	now := nowUTC()
	order := 0
	for _, email := range mailboxEmails {
		email = normalizeEmail(email)
		if email == "" || seen[email] || !knownSet[email] {
			continue
		}
		seen[email] = true
		if err := s.db.Exec(`INSERT INTO workspace_order (user_id, mailbox_email, sort_order, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(user_id, mailbox_email) DO UPDATE SET
				sort_order = excluded.sort_order,
				updated_at = excluded.updated_at;`, userID, email, order, now, now); err != nil {
			return err
		}
		order++
	}
	for _, conn := range known {
		if seen[conn.MailboxEmail] {
			continue
		}
		if err := s.db.Exec(`INSERT INTO workspace_order (user_id, mailbox_email, sort_order, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(user_id, mailbox_email) DO UPDATE SET
				sort_order = excluded.sort_order,
				updated_at = excluded.updated_at;`, userID, conn.MailboxEmail, order, now, now); err != nil {
			return err
		}
		order++
	}
	return nil
}

func (s *Store) IncrementDailyStat(userID string, allowed bool) error {
	day := nowUTC().Format("2006-01-02")
	if err := s.db.Exec(`INSERT OR IGNORE INTO user_daily_stats (user_id, day, allowed_count, denied_count)
		VALUES (?, ?, 0, 0);`, userID, day); err != nil {
		return err
	}
	if allowed {
		return s.db.Exec(`UPDATE user_daily_stats SET allowed_count = allowed_count + 1 WHERE user_id = ? AND day = ?;`, userID, day)
	}
	return s.db.Exec(`UPDATE user_daily_stats SET denied_count = denied_count + 1 WHERE user_id = ? AND day = ?;`, userID, day)
}

func (s *Store) GetUserDailyStats(userID string, days int) ([]UserDailyStat, error) {
	rows, err := s.db.Query(`SELECT day, allowed_count, denied_count FROM user_daily_stats
		WHERE user_id = ? ORDER BY day DESC LIMIT ?;`, userID, days)
	if err != nil {
		return nil, err
	}
	out := make([]UserDailyStat, 0, len(rows))
	for _, row := range rows {
		out = append(out, UserDailyStat{
			Day:          row["day"],
			AllowedCount: atoiSafe(row["allowed_count"]),
			DeniedCount:  atoiSafe(row["denied_count"]),
		})
	}
	return out, nil
}

func (s *Store) GetUserSettings(userID string) (*UserSettings, error) {
	row, err := s.db.QueryOne(`SELECT * FROM user_settings WHERE user_id = ?;`, userID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		now := nowUTC()
		return &UserSettings{UserID: userID, Timezone: "UTC", DefaultPolicyID: systemPolicyID, CreatedAt: now, UpdatedAt: now}, nil
	}
	return userSettingsFromRow(row), nil
}

func (s *Store) SaveUserTimezone(userID, timezone string) error {
	timezone = strings.TrimSpace(timezone)
	if timezone == "" {
		return fmt.Errorf("timezone cannot be empty")
	}
	if _, err := time.LoadLocation(timezone); err != nil {
		return fmt.Errorf("invalid timezone")
	}
	now := nowUTC()
	return s.db.Exec(`INSERT INTO user_settings (user_id, timezone, default_policy_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			timezone = excluded.timezone,
			updated_at = excluded.updated_at;`, userID, timezone, systemPolicyID, now, now)
}

func (s *Store) SaveDefaultPolicyID(userID, policyID string) error {
	policyID = strings.TrimSpace(policyID)
	if policyID == "" {
		policyID = systemPolicyID
	}
	if policyID != systemPolicyID {
		policy, err := s.GetUserPolicy(userID, policyID)
		if err != nil {
			return err
		}
		if policy == nil {
			return fmt.Errorf("policy not found")
		}
	}
	settings, err := s.GetUserSettings(userID)
	if err != nil {
		return err
	}
	now := nowUTC()
	return s.db.Exec(`INSERT INTO user_settings (user_id, timezone, default_policy_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			default_policy_id = excluded.default_policy_id,
			updated_at = excluded.updated_at;`, userID, settings.Timezone, policyID, now, now)
}

func (s *Store) ListUserPolicies(userID string) ([]UserPolicy, error) {
	rows, err := s.db.Query(`SELECT * FROM user_policies WHERE user_id = ? ORDER BY name ASC;`, userID)
	if err != nil {
		return nil, err
	}
	out := make([]UserPolicy, 0, len(rows))
	for _, row := range rows {
		if policy := userPolicyFromRow(row); policy != nil {
			out = append(out, *policy)
		}
	}
	return out, nil
}

func (s *Store) GetUserPolicy(userID, policyID string) (*UserPolicy, error) {
	row, err := s.db.QueryOne(`SELECT * FROM user_policies WHERE user_id = ? AND id = ?;`, userID, policyID)
	if err != nil {
		return nil, err
	}
	return userPolicyFromRow(row), nil
}

func (s *Store) SaveUserPolicy(policy *UserPolicy) error {
	policy.Name = strings.TrimSpace(policy.Name)
	if policy.Name == "" {
		return fmt.Errorf("policy name cannot be empty")
	}
	if policy.ID == "" {
		id, err := RandomUUID()
		if err != nil {
			return err
		}
		policy.ID = id
	} else {
		row, err := s.db.QueryOne(`SELECT user_id FROM user_policies WHERE id = ?;`, policy.ID)
		if err != nil {
			return err
		}
		if row != nil && row["user_id"] != policy.UserID {
			return fmt.Errorf("policy not found")
		}
	}
	policy.EnabledCapabilities = validCapabilityKeys(policy.EnabledCapabilities)
	rawCapabilities, err := json.Marshal(policy.EnabledCapabilities)
	if err != nil {
		return err
	}
	now := nowUTC()
	return s.db.Exec(`INSERT INTO user_policies (id, user_id, name, capabilities, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			capabilities = excluded.capabilities,
			updated_at = excluded.updated_at;`,
		policy.ID, policy.UserID, policy.Name, string(rawCapabilities), now, now)
}

func (s *Store) DeleteUserPolicy(userID, policyID string) (bool, error) {
	if policyID == systemPolicyID {
		return false, fmt.Errorf("system default policy cannot be deleted")
	}
	policy, err := s.GetUserPolicy(userID, policyID)
	if err != nil {
		return false, err
	}
	if policy == nil {
		return false, fmt.Errorf("policy not found")
	}
	settings, err := s.GetUserSettings(userID)
	if err != nil {
		return false, err
	}
	wasDefault := settings.DefaultPolicyID == policyID
	usedByWorkspace, err := s.PolicyUsedByWorkspace(userID, policyID)
	if err != nil {
		return false, err
	}
	if wasDefault {
		if err := s.SaveDefaultPolicyID(userID, systemPolicyID); err != nil {
			return false, err
		}
	}
	if usedByWorkspace {
		if err := s.db.Exec(`UPDATE gmail_connections SET policy_id = ?, updated_at = ? WHERE user_id = ? AND policy_id = ?;`,
			systemPolicyID, nowUTC(), userID, policyID); err != nil {
			return false, err
		}
	}
	if err := s.db.Exec(`DELETE FROM user_policies WHERE user_id = ? AND id = ?;`, userID, policyID); err != nil {
		return false, err
	}
	return wasDefault || usedByWorkspace, nil
}

func (s *Store) PolicyUsedByWorkspace(userID, policyID string) (bool, error) {
	row, err := s.db.QueryOne(`SELECT COUNT(*) AS cnt FROM gmail_connections WHERE user_id = ? AND policy_id = ?;`, userID, policyID)
	if err != nil || row == nil {
		return false, err
	}
	return atoiSafe(row["cnt"]) > 0, nil
}

func (s *Store) DefaultPolicyCapabilities(userID string) ([]string, string, error) {
	settings, err := s.GetUserSettings(userID)
	if err != nil {
		return nil, "", err
	}
	if settings.DefaultPolicyID == "" || settings.DefaultPolicyID == systemPolicyID {
		return SystemDefaultCapabilityKeys(), systemPolicyID, nil
	}
	policy, err := s.GetUserPolicy(userID, settings.DefaultPolicyID)
	if err != nil {
		return nil, "", err
	}
	if policy == nil {
		if err := s.SaveDefaultPolicyID(userID, systemPolicyID); err != nil {
			return nil, "", err
		}
		return SystemDefaultCapabilityKeys(), systemPolicyID, nil
	}
	return policy.EnabledCapabilities, policy.ID, nil
}

func (s *Store) WorkspacePolicyCapabilities(userID, mailboxEmail string) ([]string, string, error) {
	conn, err := s.GetGmailConnection(userID, mailboxEmail)
	if err != nil {
		return nil, "", err
	}
	if conn == nil {
		return nil, "", fmt.Errorf("workspace not found")
	}
	policyID := strings.TrimSpace(conn.PolicyID)
	if policyID == "" || policyID == systemPolicyID {
		return SystemDefaultCapabilityKeys(), systemPolicyID, nil
	}
	policy, err := s.GetUserPolicy(userID, policyID)
	if err != nil {
		return nil, "", err
	}
	if policy == nil {
		return nil, "", fmt.Errorf("policy not found")
	}
	return policy.EnabledCapabilities, policy.ID, nil
}

func (s *Store) ListDriveFolderRefs(userID, mailboxEmail string) ([]AllowedDriveFolder, error) {
	rows, err := s.db.Query(`SELECT * FROM drive_folder_refs WHERE user_id = ? AND mailbox_email = ? ORDER BY reference_name ASC;`, userID, normalizeEmail(mailboxEmail))
	if err != nil {
		return nil, err
	}
	out := make([]AllowedDriveFolder, 0, len(rows))
	for _, row := range rows {
		if f := driveFolderFromRow(row); f != nil {
			out = append(out, *f)
		}
	}
	return out, nil
}

func (s *Store) FindDriveFolderRefByID(userID, mailboxEmail, id string) (*AllowedDriveFolder, error) {
	row, err := s.db.QueryOne(`SELECT * FROM drive_folder_refs WHERE user_id = ? AND mailbox_email = ? AND id = ?;`, userID, normalizeEmail(mailboxEmail), id)
	if err != nil {
		return nil, err
	}
	return driveFolderFromRow(row), nil
}

func (s *Store) FindDriveFolderRefByKey(userID, mailboxEmail, key string) (*AllowedDriveFolder, error) {
	row, err := s.db.QueryOne(`SELECT * FROM drive_folder_refs WHERE user_id = ? AND mailbox_email = ? AND reference_key = ?;`, userID, normalizeEmail(mailboxEmail), strings.ToLower(strings.TrimSpace(key)))
	if err != nil {
		return nil, err
	}
	return driveFolderFromRow(row), nil
}

func (s *Store) CreateDriveFolderRef(f *AllowedDriveFolder) error {
	if f.ID == "" {
		id, err := RandomToken("dfr_", 12)
		if err != nil {
			return err
		}
		f.ID = id
	}
	now := nowUTC()
	return s.db.Exec(`INSERT INTO drive_folder_refs (id, user_id, mailbox_email, reference_name, reference_key, folder_url, folder_id, folder_name, resource_key, allow_docs, allow_sheets, allow_slides, allow_drive_files, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`,
		f.ID, f.UserID, normalizeEmail(f.MailboxEmail), f.ReferenceName, f.ReferenceKey, f.FolderURL, f.FolderID, f.FolderName, f.ResourceKey, f.AllowDocs, f.AllowSheets, f.AllowSlides, f.AllowDriveFiles, now, now)
}

func (s *Store) UpdateDriveFolderRef(f *AllowedDriveFolder) error {
	now := nowUTC()
	return s.db.Exec(`UPDATE drive_folder_refs SET reference_name = ?, reference_key = ?, folder_url = ?, folder_id = ?, folder_name = ?, resource_key = ?, allow_docs = ?, allow_sheets = ?, allow_slides = ?, allow_drive_files = ?, updated_at = ? WHERE user_id = ? AND mailbox_email = ? AND id = ?;`,
		f.ReferenceName, f.ReferenceKey, f.FolderURL, f.FolderID, f.FolderName, f.ResourceKey, f.AllowDocs, f.AllowSheets, f.AllowSlides, f.AllowDriveFiles, now, f.UserID, normalizeEmail(f.MailboxEmail), f.ID)
}

func (s *Store) DeleteDriveFolderRef(userID, mailboxEmail, id string) error {
	if err := s.DeleteDriveFolderTree(userID, id); err != nil {
		return err
	}
	return s.db.Exec(`DELETE FROM drive_folder_refs WHERE user_id = ? AND mailbox_email = ? AND id = ?;`, userID, normalizeEmail(mailboxEmail), id)
}

func (s *Store) ReplaceDriveFolderTree(userID, rootRefID string, entries []DriveFolderTreeEntry) error {
	if err := s.db.Exec(`DELETE FROM drive_folder_tree_cache WHERE user_id = ? AND root_ref_id = ?;`, userID, rootRefID); err != nil {
		return err
	}
	now := nowUTC()
	for _, entry := range entries {
		if entry.UserID == "" {
			entry.UserID = userID
		}
		if entry.RootRefID == "" {
			entry.RootRefID = rootRefID
		}
		if entry.PathKey == "" {
			entry.PathKey = strings.ToLower(strings.TrimSpace(entry.Path))
		}
		if err := s.db.Exec(`INSERT INTO drive_folder_tree_cache (user_id, root_ref_id, folder_id, parent_folder_id, folder_name, path, path_key, depth, resource_key, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`,
			entry.UserID, entry.RootRefID, entry.FolderID, entry.ParentFolderID, entry.FolderName, entry.Path, entry.PathKey, entry.Depth, entry.ResourceKey, now, now); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) DeleteDriveFolderTree(userID, rootRefID string) error {
	return s.db.Exec(`DELETE FROM drive_folder_tree_cache WHERE user_id = ? AND root_ref_id = ?;`, userID, rootRefID)
}

func (s *Store) ListDriveFolderTree(userID, rootRefID string) ([]DriveFolderTreeEntry, error) {
	rows, err := s.db.Query(`SELECT * FROM drive_folder_tree_cache WHERE user_id = ? AND root_ref_id = ? ORDER BY depth ASC, path_key ASC, folder_name ASC;`, userID, rootRefID)
	if err != nil {
		return nil, err
	}
	out := make([]DriveFolderTreeEntry, 0, len(rows))
	for _, row := range rows {
		if entry := driveFolderTreeEntryFromRow(row); entry != nil {
			out = append(out, *entry)
		}
	}
	return out, nil
}

func (s *Store) FindDriveFolderTreeByPathKey(userID, rootRefID, pathKey string) ([]DriveFolderTreeEntry, error) {
	rows, err := s.db.Query(`SELECT * FROM drive_folder_tree_cache WHERE user_id = ? AND root_ref_id = ? AND path_key = ? ORDER BY depth ASC, folder_name ASC;`, userID, rootRefID, pathKey)
	if err != nil {
		return nil, err
	}
	out := make([]DriveFolderTreeEntry, 0, len(rows))
	for _, row := range rows {
		if entry := driveFolderTreeEntryFromRow(row); entry != nil {
			out = append(out, *entry)
		}
	}
	return out, nil
}

func (s *Store) FindDriveFolderTreeByFolderID(userID, rootRefID, folderID string) (*DriveFolderTreeEntry, error) {
	row, err := s.db.QueryOne(`SELECT * FROM drive_folder_tree_cache WHERE user_id = ? AND root_ref_id = ? AND folder_id = ?;`, userID, rootRefID, folderID)
	if err != nil {
		return nil, err
	}
	return driveFolderTreeEntryFromRow(row), nil
}

func userFromRow(row map[string]string) *User {
	if row == nil {
		return nil
	}
	return &User{
		ID:          row["id"],
		Email:       row["email"],
		Name:        row["name"],
		Picture:     row["picture"],
		IsAdmin:     row["is_admin"] == "1",
		IsSuspended: row["is_suspended"] == "1",
		CreatedAt:   parseTime(row["created_at"]),
		UpdatedAt:   parseTime(row["updated_at"]),
	}
}

func sessionFromRow(row map[string]string) *Session {
	if row == nil {
		return nil
	}
	return &Session{
		ID:        row["id"],
		UserID:    row["user_id"],
		TokenHash: row["token_hash"],
		ExpiresAt: parseTime(row["expires_at"]),
		CreatedAt: parseTime(row["created_at"]),
	}
}

func userSettingsFromRow(row map[string]string) *UserSettings {
	if row == nil {
		return nil
	}
	defaultPolicyID := strings.TrimSpace(row["default_policy_id"])
	if defaultPolicyID == "" {
		defaultPolicyID = systemPolicyID
	}
	return &UserSettings{
		UserID:          row["user_id"],
		Timezone:        row["timezone"],
		DefaultPolicyID: defaultPolicyID,
		CreatedAt:       parseTime(row["created_at"]),
		UpdatedAt:       parseTime(row["updated_at"]),
	}
}

func userPolicyFromRow(row map[string]string) *UserPolicy {
	if row == nil {
		return nil
	}
	var capabilities []string
	_ = json.Unmarshal([]byte(row["capabilities"]), &capabilities)
	return &UserPolicy{
		ID:                  row["id"],
		UserID:              row["user_id"],
		Name:                row["name"],
		EnabledCapabilities: validCapabilityKeys(capabilities),
		CreatedAt:           parseTime(row["created_at"]),
		UpdatedAt:           parseTime(row["updated_at"]),
	}
}

func gmailConnectionFromRow(row map[string]string) *GmailConnection {
	if row == nil {
		return nil
	}
	return &GmailConnection{
		UserID:          row["user_id"],
		MailboxEmail:    row["mailbox_email"],
		FriendlyName:    row["friendly_name"],
		PolicyID:        row["policy_id"],
		Scopes:          row["scopes"],
		AccessTokenEnc:  row["access_token_enc"],
		RefreshTokenEnc: row["refresh_token_enc"],
		TokenExpiry:     parseTime(row["token_expiry"]),
		CreatedAt:       parseTime(row["created_at"]),
		UpdatedAt:       parseTime(row["updated_at"]),
	}
}

func driveFolderFromRow(row map[string]string) *AllowedDriveFolder {
	if row == nil {
		return nil
	}
	return &AllowedDriveFolder{
		ID:              row["id"],
		UserID:          row["user_id"],
		MailboxEmail:    row["mailbox_email"],
		ReferenceName:   row["reference_name"],
		ReferenceKey:    row["reference_key"],
		FolderURL:       row["folder_url"],
		FolderID:        row["folder_id"],
		FolderName:      row["folder_name"],
		ResourceKey:     row["resource_key"],
		AllowDocs:       row["allow_docs"] == "1",
		AllowSheets:     row["allow_sheets"] == "1",
		AllowSlides:     row["allow_slides"] == "1",
		AllowDriveFiles: row["allow_drive_files"] == "1",
		CreatedAt:       parseTime(row["created_at"]),
		UpdatedAt:       parseTime(row["updated_at"]),
	}
}

func driveFolderTreeEntryFromRow(row map[string]string) *DriveFolderTreeEntry {
	if row == nil {
		return nil
	}
	return &DriveFolderTreeEntry{
		UserID:         row["user_id"],
		RootRefID:      row["root_ref_id"],
		FolderID:       row["folder_id"],
		ParentFolderID: row["parent_folder_id"],
		FolderName:     row["folder_name"],
		Path:           row["path"],
		PathKey:        row["path_key"],
		Depth:          atoiSafe(row["depth"]),
		ResourceKey:    row["resource_key"],
		CreatedAt:      parseTime(row["created_at"]),
		UpdatedAt:      parseTime(row["updated_at"]),
	}
}

func parseTime(raw string) time.Time {
	if raw == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Time{}
	}
	return t.UTC()
}

func atoiSafe(raw string) int {
	n, _ := strconv.Atoi(raw)
	return n
}

func (s *Store) FindUserByProxyToken(crypto *Crypto, token string) (*User, error) {
	rows, err := s.db.Query(`SELECT u.*, p.token_enc AS proxy_token_enc
		FROM proxy_tokens p
		JOIN users u ON u.id = p.user_id;`)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		enc := row["proxy_token_enc"]
		if enc == "" {
			continue
		}
		dec, err := crypto.Decrypt(enc)
		if err != nil {
			continue
		}
		if subtleEqual(dec, token) {
			return userFromRow(row), nil
		}
	}
	return nil, nil
}

func (s *Store) DebugDump() string {
	rows, err := s.db.Query(`SELECT email FROM users ORDER BY email;`)
	if err != nil {
		return err.Error()
	}
	return fmt.Sprintf("%d users", len(rows))
}
