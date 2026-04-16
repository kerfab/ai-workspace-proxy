package main

import (
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
			user_id TEXT PRIMARY KEY,
			mailbox_email TEXT NOT NULL,
			scopes TEXT NOT NULL,
			access_token_enc TEXT,
			refresh_token_enc TEXT NOT NULL,
			token_expiry TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS drive_folder_refs (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
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
			UNIQUE(user_id, reference_key)
		);`,
		`CREATE TABLE IF NOT EXISTS user_daily_stats (
			user_id TEXT NOT NULL,
			day TEXT NOT NULL,
			allowed_count INTEGER NOT NULL DEFAULT 0,
			denied_count INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (user_id, day),
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
		`CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_token_hash ON sessions(token_hash);`,
		`CREATE INDEX IF NOT EXISTS idx_drive_folder_refs_user_id ON drive_folder_refs(user_id);`,
	}
	for _, stmt := range stmts {
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
		{`DELETE FROM gmail_connections WHERE user_id = ?;`, []any{userID}},
		{`DELETE FROM drive_folder_refs WHERE user_id = ?;`, []any{userID}},
		{`DELETE FROM user_daily_stats WHERE user_id = ?;`, []any{userID}},
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
	return s.db.Exec(`INSERT INTO gmail_connections (user_id, mailbox_email, scopes, access_token_enc, refresh_token_enc, token_expiry, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			mailbox_email = excluded.mailbox_email,
			scopes = excluded.scopes,
			access_token_enc = excluded.access_token_enc,
			refresh_token_enc = excluded.refresh_token_enc,
			token_expiry = excluded.token_expiry,
			updated_at = excluded.updated_at;`,
		conn.UserID, conn.MailboxEmail, conn.Scopes, conn.AccessTokenEnc, conn.RefreshTokenEnc, conn.TokenExpiry, now, now)
}

func (s *Store) GetGmailConnection(userID string) (*GmailConnection, error) {
	row, err := s.db.QueryOne(`SELECT * FROM gmail_connections WHERE user_id = ?;`, userID)
	if err != nil {
		return nil, err
	}
	return gmailConnectionFromRow(row), nil
}

func (s *Store) DeleteGmailConnection(userID string) error {
	return s.db.Exec(`DELETE FROM gmail_connections WHERE user_id = ?;`, userID)
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

func (s *Store) ListDriveFolderRefs(userID string) ([]AllowedDriveFolder, error) {
	rows, err := s.db.Query(`SELECT * FROM drive_folder_refs WHERE user_id = ? ORDER BY reference_name ASC;`, userID)
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

func (s *Store) CountDriveFolderRefs(userID string) (int, error) {
	row, err := s.db.QueryOne(`SELECT COUNT(*) AS cnt FROM drive_folder_refs WHERE user_id = ?;`, userID)
	if err != nil || row == nil {
		return 0, err
	}
	return atoiSafe(row["cnt"]), nil
}

func (s *Store) FindDriveFolderRefByID(userID, id string) (*AllowedDriveFolder, error) {
	row, err := s.db.QueryOne(`SELECT * FROM drive_folder_refs WHERE user_id = ? AND id = ?;`, userID, id)
	if err != nil {
		return nil, err
	}
	return driveFolderFromRow(row), nil
}

func (s *Store) FindDriveFolderRefByKey(userID, key string) (*AllowedDriveFolder, error) {
	row, err := s.db.QueryOne(`SELECT * FROM drive_folder_refs WHERE user_id = ? AND reference_key = ?;`, userID, strings.ToLower(strings.TrimSpace(key)))
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
	return s.db.Exec(`INSERT INTO drive_folder_refs (id, user_id, reference_name, reference_key, folder_url, folder_id, folder_name, resource_key, allow_docs, allow_sheets, allow_slides, allow_drive_files, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`,
		f.ID, f.UserID, f.ReferenceName, f.ReferenceKey, f.FolderURL, f.FolderID, f.FolderName, f.ResourceKey, f.AllowDocs, f.AllowSheets, f.AllowSlides, f.AllowDriveFiles, now, now)
}

func (s *Store) UpdateDriveFolderRef(f *AllowedDriveFolder) error {
	now := nowUTC()
	return s.db.Exec(`UPDATE drive_folder_refs SET reference_name = ?, reference_key = ?, folder_url = ?, folder_id = ?, folder_name = ?, resource_key = ?, allow_docs = ?, allow_sheets = ?, allow_slides = ?, allow_drive_files = ?, updated_at = ? WHERE user_id = ? AND id = ?;`,
		f.ReferenceName, f.ReferenceKey, f.FolderURL, f.FolderID, f.FolderName, f.ResourceKey, f.AllowDocs, f.AllowSheets, f.AllowSlides, f.AllowDriveFiles, now, f.UserID, f.ID)
}

func (s *Store) DeleteDriveFolderRef(userID, id string) error {
	return s.db.Exec(`DELETE FROM drive_folder_refs WHERE user_id = ? AND id = ?;`, userID, id)
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

func gmailConnectionFromRow(row map[string]string) *GmailConnection {
	if row == nil {
		return nil
	}
	return &GmailConnection{
		UserID:          row["user_id"],
		MailboxEmail:    row["mailbox_email"],
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
