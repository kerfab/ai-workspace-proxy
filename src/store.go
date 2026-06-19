// Copyright (c) 2026 Opensense Ltd. (Hong Kong). All rights reserved.
// Proprietary software. No use, copy, modification, distribution, disclosure,
// or reverse engineering is permitted without prior written authorization
// from Opensense Ltd.

package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Store struct {
	db                               *SQLiteDB
	organizationAdminVerificationKey []byte
}

func NewStore(db *SQLiteDB) *Store {
	return &Store{db: db}
}

func (s *Store) SetOrganizationAdminVerificationKey(key []byte) {
	if len(key) == 0 {
		s.organizationAdminVerificationKey = nil
		return
	}
	s.organizationAdminVerificationKey = append([]byte(nil), key...)
}

func (s *Store) Init() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS organizations (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS organization_domains (
			organization_id TEXT NOT NULL,
			domain_name TEXT NOT NULL,
			is_primary INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			PRIMARY KEY(organization_id, domain_name),
			FOREIGN KEY(organization_id) REFERENCES organizations(id) ON DELETE CASCADE,
			UNIQUE(domain_name)
		);`,
		`CREATE TABLE IF NOT EXISTS organization_admin_verifications (
			user_id TEXT PRIMARY KEY,
			organization_id TEXT NOT NULL,
			domain_name TEXT NOT NULL,
			verification_host TEXT NOT NULL,
			verification_value TEXT NOT NULL,
			verified_at TEXT,
			last_checked_at TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY(organization_id) REFERENCES organizations(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS organization_accounts_policies (
			organization_id TEXT PRIMARY KEY,
			enforce_session_timeout INTEGER NOT NULL DEFAULT 0,
			session_timeout_hours INTEGER NOT NULL DEFAULT 24,
			require_two_factor INTEGER NOT NULL DEFAULT 0,
			allow_custom_policies INTEGER NOT NULL DEFAULT 0,
			custom_policy_max_risk INTEGER NOT NULL DEFAULT 1,
			allow_non_admin_invites INTEGER NOT NULL DEFAULT 0,
			allow_external_workspace_domains INTEGER NOT NULL DEFAULT 0,
			approved_workspace_domains TEXT NOT NULL DEFAULT '[]',
			allow_external_users_connect_org_workspaces INTEGER NOT NULL DEFAULT 0,
			allowed_external_workspace_connectors TEXT NOT NULL DEFAULT '[]',
			blacklist_workspace_access INTEGER NOT NULL DEFAULT 0,
			blocked_workspace_emails TEXT NOT NULL DEFAULT '[]',
			deny_all_workspace_connections_except INTEGER NOT NULL DEFAULT 0,
			allowed_workspace_emails TEXT NOT NULL DEFAULT '[]',
			enforce_firewall INTEGER NOT NULL DEFAULT 0,
			allow_user_two_factor_reset INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			FOREIGN KEY(organization_id) REFERENCES organizations(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS organization_firewall_rules (
			id TEXT PRIMARY KEY,
			organization_id TEXT NOT NULL,
			value TEXT NOT NULL,
			ip_version TEXT NOT NULL,
			address_kind TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			FOREIGN KEY(organization_id) REFERENCES organizations(id) ON DELETE CASCADE,
			UNIQUE(organization_id, value)
		);`,
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			email TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			picture TEXT NOT NULL,
			organization_id TEXT,
			is_admin INTEGER NOT NULL DEFAULT 0,
			is_suspended INTEGER NOT NULL DEFAULT 0,
			suspended_at TEXT,
			suspended_by_user_id TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			FOREIGN KEY(organization_id) REFERENCES organizations(id) ON DELETE SET NULL
		);`,
		`CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			token_hash TEXT NOT NULL UNIQUE,
			second_factor_verified INTEGER NOT NULL DEFAULT 1,
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
		`CREATE TABLE IF NOT EXISTS workspace_oauth_states (
			state_hash TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			mailbox_email TEXT NOT NULL DEFAULT '',
			expires_at TEXT NOT NULL,
			created_at TEXT NOT NULL,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS workspace_reauth_tokens (
			token_hash TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			mailbox_email TEXT NOT NULL,
			expires_at TEXT NOT NULL,
			created_at TEXT NOT NULL,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS agents (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			friendly_name TEXT NOT NULL,
			default_location TEXT NOT NULL,
			token_enc TEXT NOT NULL,
			token_hint TEXT NOT NULL,
			enabled INTEGER NOT NULL DEFAULT 1,
			firewall_enabled INTEGER NOT NULL DEFAULT 0,
			last_used_at TEXT,
			skill_stale INTEGER NOT NULL DEFAULT 0,
			skill_stale_reason TEXT NOT NULL DEFAULT '',
			skill_stale_at TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS agent_firewall_rules (
			id TEXT PRIMARY KEY,
			agent_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			value TEXT NOT NULL,
			ip_version TEXT NOT NULL,
			address_kind TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			FOREIGN KEY(agent_id) REFERENCES agents(id) ON DELETE CASCADE,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
			UNIQUE(agent_id, value)
		);`,
		`CREATE TABLE IF NOT EXISTS agent_workspace_grants (
			agent_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			mailbox_email TEXT NOT NULL,
			policy_id TEXT NOT NULL DEFAULT 'system',
			require_agent_motive INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			PRIMARY KEY(agent_id, mailbox_email),
			FOREIGN KEY(agent_id) REFERENCES agents(id) ON DELETE CASCADE,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY(user_id, mailbox_email) REFERENCES gmail_connections(user_id, mailbox_email) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS agent_drive_folder_grants (
			agent_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			mailbox_email TEXT NOT NULL,
			folder_ref_id TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			PRIMARY KEY(agent_id, folder_ref_id),
			FOREIGN KEY(agent_id) REFERENCES agents(id) ON DELETE CASCADE,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY(user_id, mailbox_email) REFERENCES gmail_connections(user_id, mailbox_email) ON DELETE CASCADE,
			FOREIGN KEY(folder_ref_id) REFERENCES drive_folder_refs(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS user_backend_api_tokens (
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
			session_timeout_hours INTEGER NOT NULL DEFAULT 24,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS user_two_factor_settings (
			user_id TEXT PRIMARY KEY,
			enabled INTEGER NOT NULL DEFAULT 0,
			secret_enc TEXT NOT NULL DEFAULT '',
			pending_secret_enc TEXT NOT NULL DEFAULT '',
			pending_secret_set_at TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS user_logging_settings (
			user_id TEXT PRIMARY KEY,
			require_agent_context INTEGER NOT NULL DEFAULT 0,
			retention_days INTEGER NOT NULL DEFAULT 7,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS user_log_view_settings (
			user_id TEXT NOT NULL,
			view_key TEXT NOT NULL,
			visible_columns TEXT NOT NULL,
			column_widths TEXT NOT NULL DEFAULT '{}',
			page_size INTEGER NOT NULL DEFAULT 100,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			PRIMARY KEY(user_id, view_key),
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS request_logs (
			id TEXT PRIMARY KEY,
			request_id TEXT NOT NULL UNIQUE,
			user_id TEXT NOT NULL,
			user_email TEXT NOT NULL,
			workspace_email TEXT NOT NULL,
			agent_id TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			method TEXT NOT NULL,
			service TEXT NOT NULL,
			path TEXT NOT NULL,
			query TEXT NOT NULL,
			target_object_type TEXT NOT NULL DEFAULT '',
			target_object_id TEXT NOT NULL DEFAULT '',
			user_agent TEXT NOT NULL,
			remote_addr TEXT NOT NULL,
			x_forwarded_for TEXT NOT NULL,
			x_real_ip TEXT NOT NULL,
			forwarded TEXT NOT NULL,
			cf_connecting_ip TEXT NOT NULL,
			agent_name TEXT NOT NULL,
			agent_location TEXT NOT NULL,
			agent_motive TEXT NOT NULL,
			human_approval TEXT NOT NULL DEFAULT '',
			outcome TEXT NOT NULL,
			http_status INTEGER NOT NULL,
			upstream_status INTEGER NOT NULL,
			policy_id TEXT NOT NULL,
			policy_name TEXT NOT NULL,
			policy_capability_key TEXT NOT NULL,
			policy_capability_title TEXT NOT NULL,
			policy_rule_name TEXT NOT NULL,
			error_message TEXT NOT NULL,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS user_policies (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			name TEXT NOT NULL,
			capabilities TEXT NOT NULL,
			review_required_capabilities TEXT NOT NULL DEFAULT '[]',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
			UNIQUE(user_id, name)
		);`,
		`CREATE TABLE IF NOT EXISTS agent_skill_download_tokens (
			user_id TEXT NOT NULL,
			agent_id TEXT NOT NULL,
			token_hash TEXT NOT NULL UNIQUE,
			platform TEXT NOT NULL,
			expires_at TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			PRIMARY KEY(user_id, agent_id),
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY(agent_id) REFERENCES agents(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS policy_audit_logs (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			user_email TEXT NOT NULL,
			workspace_email TEXT NOT NULL,
			created_at TEXT NOT NULL,
			action TEXT NOT NULL,
			target_id TEXT NOT NULL,
			target_name TEXT NOT NULL,
			user_agent TEXT NOT NULL DEFAULT '',
			remote_addr TEXT NOT NULL DEFAULT '',
			x_forwarded_for TEXT NOT NULL DEFAULT '',
			x_real_ip TEXT NOT NULL DEFAULT '',
			forwarded TEXT NOT NULL DEFAULT '',
			cf_connecting_ip TEXT NOT NULL DEFAULT '',
			details_json TEXT NOT NULL,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS workspace_audit_logs (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			user_email TEXT NOT NULL,
			workspace_email TEXT NOT NULL,
			created_at TEXT NOT NULL,
			action TEXT NOT NULL,
			target_id TEXT NOT NULL,
			target_name TEXT NOT NULL,
			user_agent TEXT NOT NULL DEFAULT '',
			remote_addr TEXT NOT NULL DEFAULT '',
			x_forwarded_for TEXT NOT NULL DEFAULT '',
			x_real_ip TEXT NOT NULL DEFAULT '',
			forwarded TEXT NOT NULL DEFAULT '',
			cf_connecting_ip TEXT NOT NULL DEFAULT '',
			details_json TEXT NOT NULL,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS drive_folder_audit_logs (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			user_email TEXT NOT NULL,
			workspace_email TEXT NOT NULL,
			created_at TEXT NOT NULL,
			action TEXT NOT NULL,
			target_id TEXT NOT NULL,
			target_name TEXT NOT NULL,
			user_agent TEXT NOT NULL DEFAULT '',
			remote_addr TEXT NOT NULL DEFAULT '',
			x_forwarded_for TEXT NOT NULL DEFAULT '',
			x_real_ip TEXT NOT NULL DEFAULT '',
			forwarded TEXT NOT NULL DEFAULT '',
			cf_connecting_ip TEXT NOT NULL DEFAULT '',
			details_json TEXT NOT NULL,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS user_audit_logs (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			user_email TEXT NOT NULL,
			workspace_email TEXT NOT NULL,
			created_at TEXT NOT NULL,
			action TEXT NOT NULL,
			target_id TEXT NOT NULL,
			target_name TEXT NOT NULL,
			user_agent TEXT NOT NULL DEFAULT '',
			remote_addr TEXT NOT NULL DEFAULT '',
			x_forwarded_for TEXT NOT NULL DEFAULT '',
			x_real_ip TEXT NOT NULL DEFAULT '',
			forwarded TEXT NOT NULL DEFAULT '',
			cf_connecting_ip TEXT NOT NULL DEFAULT '',
			details_json TEXT NOT NULL,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
	}
	for _, stmt := range stmts {
		if err := s.db.Exec(stmt); err != nil {
			return err
		}
	}
	if err := s.ensureColumn("user_log_view_settings", "column_widths", `ALTER TABLE user_log_view_settings ADD COLUMN column_widths TEXT NOT NULL DEFAULT '{}';`); err != nil {
		return err
	}
	if err := s.ensureColumn("sessions", "second_factor_verified", `ALTER TABLE sessions ADD COLUMN second_factor_verified INTEGER NOT NULL DEFAULT 1;`); err != nil {
		return err
	}
	if err := s.ensureColumn("users", "organization_id", `ALTER TABLE users ADD COLUMN organization_id TEXT;`); err != nil {
		return err
	}
	if err := s.ensureColumn("users", "suspended_at", `ALTER TABLE users ADD COLUMN suspended_at TEXT;`); err != nil {
		return err
	}
	if err := s.ensureColumn("users", "suspended_by_user_id", `ALTER TABLE users ADD COLUMN suspended_by_user_id TEXT NOT NULL DEFAULT '';`); err != nil {
		return err
	}
	if err := s.ensureColumn("user_settings", "session_timeout_hours", `ALTER TABLE user_settings ADD COLUMN session_timeout_hours INTEGER NOT NULL DEFAULT 24;`); err != nil {
		return err
	}
	if err := s.ensureColumn("request_logs", "target_object_type", `ALTER TABLE request_logs ADD COLUMN target_object_type TEXT NOT NULL DEFAULT '';`); err != nil {
		return err
	}
	if err := s.ensureColumn("request_logs", "target_object_id", `ALTER TABLE request_logs ADD COLUMN target_object_id TEXT NOT NULL DEFAULT '';`); err != nil {
		return err
	}
	if err := s.ensureColumn("request_logs", "agent_id", `ALTER TABLE request_logs ADD COLUMN agent_id TEXT NOT NULL DEFAULT '';`); err != nil {
		return err
	}
	if err := s.ensureColumn("request_logs", "human_approval", `ALTER TABLE request_logs ADD COLUMN human_approval TEXT NOT NULL DEFAULT '';`); err != nil {
		return err
	}
	if err := s.ensureColumn("agent_workspace_grants", "require_agent_motive", `ALTER TABLE agent_workspace_grants ADD COLUMN require_agent_motive INTEGER NOT NULL DEFAULT 0;`); err != nil {
		return err
	}
	if err := s.ensureColumn("user_policies", "review_required_capabilities", `ALTER TABLE user_policies ADD COLUMN review_required_capabilities TEXT NOT NULL DEFAULT '[]';`); err != nil {
		return err
	}
	if err := s.ensureColumn("agents", "skill_stale", `ALTER TABLE agents ADD COLUMN skill_stale INTEGER NOT NULL DEFAULT 0;`); err != nil {
		return err
	}
	if err := s.ensureColumn("agents", "skill_stale_reason", `ALTER TABLE agents ADD COLUMN skill_stale_reason TEXT NOT NULL DEFAULT '';`); err != nil {
		return err
	}
	if err := s.ensureColumn("agents", "skill_stale_at", `ALTER TABLE agents ADD COLUMN skill_stale_at TEXT;`); err != nil {
		return err
	}
	if err := s.ensureColumn("agents", "firewall_enabled", `ALTER TABLE agents ADD COLUMN firewall_enabled INTEGER NOT NULL DEFAULT 0;`); err != nil {
		return err
	}
	for _, column := range []struct {
		name string
		stmt string
	}{
		{name: "allow_external_users_connect_org_workspaces", stmt: `ALTER TABLE organization_accounts_policies ADD COLUMN allow_external_users_connect_org_workspaces INTEGER NOT NULL DEFAULT 0;`},
		{name: "allowed_external_workspace_connectors", stmt: `ALTER TABLE organization_accounts_policies ADD COLUMN allowed_external_workspace_connectors TEXT NOT NULL DEFAULT '[]';`},
		{name: "blacklist_workspace_access", stmt: `ALTER TABLE organization_accounts_policies ADD COLUMN blacklist_workspace_access INTEGER NOT NULL DEFAULT 0;`},
		{name: "blocked_workspace_emails", stmt: `ALTER TABLE organization_accounts_policies ADD COLUMN blocked_workspace_emails TEXT NOT NULL DEFAULT '[]';`},
		{name: "deny_all_workspace_connections_except", stmt: `ALTER TABLE organization_accounts_policies ADD COLUMN deny_all_workspace_connections_except INTEGER NOT NULL DEFAULT 0;`},
		{name: "allowed_workspace_emails", stmt: `ALTER TABLE organization_accounts_policies ADD COLUMN allowed_workspace_emails TEXT NOT NULL DEFAULT '[]';`},
	} {
		if err := s.ensureColumn("organization_accounts_policies", column.name, column.stmt); err != nil {
			return err
		}
	}
	for _, table := range []string{"policy_audit_logs", "workspace_audit_logs", "drive_folder_audit_logs", "user_audit_logs"} {
		for _, column := range []struct {
			name string
			stmt string
		}{
			{name: "user_agent", stmt: fmt.Sprintf(`ALTER TABLE %s ADD COLUMN user_agent TEXT NOT NULL DEFAULT '';`, table)},
			{name: "remote_addr", stmt: fmt.Sprintf(`ALTER TABLE %s ADD COLUMN remote_addr TEXT NOT NULL DEFAULT '';`, table)},
			{name: "x_forwarded_for", stmt: fmt.Sprintf(`ALTER TABLE %s ADD COLUMN x_forwarded_for TEXT NOT NULL DEFAULT '';`, table)},
			{name: "x_real_ip", stmt: fmt.Sprintf(`ALTER TABLE %s ADD COLUMN x_real_ip TEXT NOT NULL DEFAULT '';`, table)},
			{name: "forwarded", stmt: fmt.Sprintf(`ALTER TABLE %s ADD COLUMN forwarded TEXT NOT NULL DEFAULT '';`, table)},
			{name: "cf_connecting_ip", stmt: fmt.Sprintf(`ALTER TABLE %s ADD COLUMN cf_connecting_ip TEXT NOT NULL DEFAULT '';`, table)},
		} {
			if err := s.ensureColumn(table, column.name, column.stmt); err != nil {
				return err
			}
		}
	}
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_users_organization_id ON users(organization_id);`,
		`CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_organization_domains_domain_name ON organization_domains(domain_name);`,
		`CREATE INDEX IF NOT EXISTS idx_organization_domains_org_id ON organization_domains(organization_id);`,
		`CREATE INDEX IF NOT EXISTS idx_organization_admin_verifications_org_id ON organization_admin_verifications(organization_id);`,
		`CREATE INDEX IF NOT EXISTS idx_organization_firewall_rules_org ON organization_firewall_rules(organization_id);`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_token_hash ON sessions(token_hash);`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_user_verified ON sessions(user_id, second_factor_verified);`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);`,
		`CREATE INDEX IF NOT EXISTS idx_workspace_oauth_states_user ON workspace_oauth_states(user_id);`,
		`CREATE INDEX IF NOT EXISTS idx_user_two_factor_settings_user ON user_two_factor_settings(user_id);`,
		`CREATE INDEX IF NOT EXISTS idx_workspace_reauth_tokens_user_workspace ON workspace_reauth_tokens(user_id, mailbox_email);`,
		`CREATE INDEX IF NOT EXISTS idx_agents_user ON agents(user_id);`,
		`CREATE INDEX IF NOT EXISTS idx_agents_user_enabled ON agents(user_id, enabled);`,
		`CREATE INDEX IF NOT EXISTS idx_agent_firewall_rules_user_agent ON agent_firewall_rules(user_id, agent_id);`,
		`CREATE INDEX IF NOT EXISTS idx_agent_workspace_grants_user_agent ON agent_workspace_grants(user_id, agent_id);`,
		`CREATE INDEX IF NOT EXISTS idx_agent_workspace_grants_user_workspace ON agent_workspace_grants(user_id, mailbox_email);`,
		`CREATE INDEX IF NOT EXISTS idx_agent_drive_folder_grants_user_agent_workspace ON agent_drive_folder_grants(user_id, agent_id, mailbox_email);`,
		`CREATE INDEX IF NOT EXISTS idx_agent_drive_folder_grants_user_folder ON agent_drive_folder_grants(user_id, folder_ref_id);`,
		`CREATE INDEX IF NOT EXISTS idx_gmail_connections_user ON gmail_connections(user_id);`,
		`CREATE INDEX IF NOT EXISTS idx_workspace_order_user_sort ON workspace_order(user_id, sort_order);`,
		`CREATE INDEX IF NOT EXISTS idx_user_policies_user ON user_policies(user_id);`,
		`CREATE INDEX IF NOT EXISTS idx_agent_skill_download_tokens_hash ON agent_skill_download_tokens(token_hash);`,
		`CREATE INDEX IF NOT EXISTS idx_user_log_view_settings_user ON user_log_view_settings(user_id);`,
		`CREATE INDEX IF NOT EXISTS idx_drive_folder_refs_user_workspace ON drive_folder_refs(user_id, mailbox_email);`,
		`DROP INDEX IF EXISTS idx_drive_folder_refs_unique_folder;`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_drive_folder_refs_unique_folder ON drive_folder_refs(user_id, mailbox_email, folder_id);`,
		`CREATE INDEX IF NOT EXISTS idx_drive_folder_tree_root ON drive_folder_tree_cache(user_id, root_ref_id);`,
		`CREATE INDEX IF NOT EXISTS idx_drive_folder_tree_path ON drive_folder_tree_cache(user_id, root_ref_id, path_key);`,
		`CREATE INDEX IF NOT EXISTS idx_drive_folder_tree_folder ON drive_folder_tree_cache(user_id, folder_id);`,
		`CREATE INDEX IF NOT EXISTS idx_request_logs_user_created ON request_logs(user_id, created_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_request_logs_user_outcome_created ON request_logs(user_id, outcome, created_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_request_logs_user_workspace_created ON request_logs(user_id, workspace_email, created_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_request_logs_user_agent_created ON request_logs(user_id, agent_id, created_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_request_logs_request_id ON request_logs(request_id);`,
		`CREATE INDEX IF NOT EXISTS idx_policy_audit_logs_user_created ON policy_audit_logs(user_id, created_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_workspace_audit_logs_user_created ON workspace_audit_logs(user_id, created_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_drive_folder_audit_logs_user_created ON drive_folder_audit_logs(user_id, created_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_user_audit_logs_user_created ON user_audit_logs(user_id, created_at DESC);`,
	}
	for _, stmt := range indexes {
		if err := s.db.Exec(stmt); err != nil {
			return err
		}
	}
	if err := s.syncUserOrganizations(); err != nil {
		return err
	}
	return nil
}

func (s *Store) ensureColumn(table, column, alterStmt string) error {
	rows, err := s.db.Query("PRAGMA table_info(" + table + ");")
	if err != nil {
		return err
	}
	for _, row := range rows {
		if row["name"] == column {
			return nil
		}
	}
	return s.db.Exec(alterStmt)
}

func (s *Store) UserCount() (int, error) {
	row, err := s.db.QueryOne(`SELECT COUNT(*) AS cnt FROM users;`)
	if err != nil || row == nil {
		return 0, err
	}
	return atoiSafe(row["cnt"]), nil
}

func organizationDomainForEmail(email string) string {
	email = normalizeEmail(email)
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return ""
	}
	domain := normalizeDomain(parts[1])
	if domain == "" || domain == "gmail.com" {
		return ""
	}
	return domain
}

func (s *Store) FindOrganizationByID(id string) (*Organization, error) {
	row, err := s.db.QueryOne(`SELECT * FROM organizations WHERE id = ?;`, strings.TrimSpace(id))
	if err != nil {
		return nil, err
	}
	return organizationFromRow(row), nil
}

func (s *Store) FindOrganizationByDomain(domain string) (*Organization, error) {
	domain = normalizeDomain(domain)
	if domain == "" {
		return nil, nil
	}
	row, err := s.db.QueryOne(`SELECT o.* FROM organizations o
		JOIN organization_domains d ON d.organization_id = o.id
		WHERE d.domain_name = ?;`, domain)
	if err != nil {
		return nil, err
	}
	return organizationFromRow(row), nil
}

func defaultOrganizationAccountsPolicy(organizationID string) *OrganizationAccountsPolicy {
	now := nowUTC()
	return &OrganizationAccountsPolicy{
		OrganizationID:           strings.TrimSpace(organizationID),
		SessionTimeoutHours:      defaultUserSessionTimeoutHours,
		CustomPolicyMaxRisk:      1,
		ApprovedWorkspaceDomains: []string{},
		CreatedAt:                now,
		UpdatedAt:                now,
	}
}

func normalizeDomainList(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		for _, part := range strings.FieldsFunc(value, func(r rune) bool {
			return r == ',' || r == '\n' || r == '\r' || r == '\t' || r == ' '
		}) {
			domain := normalizeDomain(part)
			if domain == "" || seen[domain] {
				continue
			}
			seen[domain] = true
			out = append(out, domain)
		}
	}
	sort.Strings(out)
	return out
}

func normalizeEmailList(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		for _, part := range strings.FieldsFunc(value, func(r rune) bool {
			return r == ',' || r == '\n' || r == '\r' || r == '\t' || r == ' '
		}) {
			email := normalizeEmail(part)
			if email == "" || !strings.Contains(email, "@") || seen[email] {
				continue
			}
			seen[email] = true
			out = append(out, email)
		}
	}
	sort.Strings(out)
	return out
}

func normalizeEmailOrDomainList(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		for _, part := range strings.FieldsFunc(value, func(r rune) bool {
			return r == ',' || r == '\n' || r == '\r' || r == '\t' || r == ' '
		}) {
			item := strings.TrimSpace(part)
			if strings.Contains(item, "@") {
				item = normalizeEmail(item)
			} else {
				item = normalizeDomain(item)
			}
			if item == "" || seen[item] {
				continue
			}
			seen[item] = true
			out = append(out, item)
		}
	}
	sort.Strings(out)
	return out
}

func (s *Store) GetOrganizationAccountsPolicy(organizationID string) (*OrganizationAccountsPolicy, error) {
	organizationID = strings.TrimSpace(organizationID)
	if organizationID == "" {
		return nil, nil
	}
	row, err := s.db.QueryOne(`SELECT * FROM organization_accounts_policies WHERE organization_id = ?;`, organizationID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		policy := defaultOrganizationAccountsPolicy(organizationID)
		if err := s.SaveOrganizationAccountsPolicy(policy); err != nil {
			return nil, err
		}
		return s.GetOrganizationAccountsPolicy(organizationID)
	}
	return organizationAccountsPolicyFromRow(row), nil
}

func (s *Store) SaveOrganizationAccountsPolicy(policy *OrganizationAccountsPolicy) error {
	if policy == nil || strings.TrimSpace(policy.OrganizationID) == "" {
		return fmt.Errorf("organization is required")
	}
	timeoutHours := policy.SessionTimeoutHours
	if timeoutHours < 1 {
		timeoutHours = defaultUserSessionTimeoutHours
	}
	if timeoutHours > maxUserSessionTimeoutHours {
		timeoutHours = maxUserSessionTimeoutHours
	}
	maxRisk := policy.CustomPolicyMaxRisk
	if maxRisk < 1 {
		maxRisk = 1
	}
	if maxRisk > 3 {
		maxRisk = 3
	}
	domains := normalizeDomainList(policy.ApprovedWorkspaceDomains)
	rawDomains, err := json.Marshal(domains)
	if err != nil {
		return err
	}
	externalConnectors := normalizeEmailOrDomainList(policy.AllowedExternalWorkspaceConnectors)
	rawExternalConnectors, err := json.Marshal(externalConnectors)
	if err != nil {
		return err
	}
	blockedWorkspaceEmails := normalizeEmailList(policy.BlockedWorkspaceEmails)
	rawBlockedWorkspaceEmails, err := json.Marshal(blockedWorkspaceEmails)
	if err != nil {
		return err
	}
	allowedWorkspaceEmails := normalizeEmailList(policy.AllowedWorkspaceEmails)
	rawAllowedWorkspaceEmails, err := json.Marshal(allowedWorkspaceEmails)
	if err != nil {
		return err
	}
	if policy.BlacklistWorkspaceAccess && policy.DenyAllWorkspaceConnectionsExcept {
		return fmt.Errorf("Blacklist access and deny-all-except mode cannot be enabled together")
	}
	now := nowUTC()
	return s.db.Exec(`INSERT INTO organization_accounts_policies (
			organization_id, enforce_session_timeout, session_timeout_hours, require_two_factor,
			allow_custom_policies, custom_policy_max_risk, allow_non_admin_invites,
			allow_external_workspace_domains, approved_workspace_domains,
			allow_external_users_connect_org_workspaces, allowed_external_workspace_connectors,
			blacklist_workspace_access, blocked_workspace_emails,
			deny_all_workspace_connections_except, allowed_workspace_emails,
			enforce_firewall, allow_user_two_factor_reset, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(organization_id) DO UPDATE SET
			enforce_session_timeout = excluded.enforce_session_timeout,
			session_timeout_hours = excluded.session_timeout_hours,
			require_two_factor = excluded.require_two_factor,
			allow_custom_policies = excluded.allow_custom_policies,
			custom_policy_max_risk = excluded.custom_policy_max_risk,
			allow_non_admin_invites = excluded.allow_non_admin_invites,
			allow_external_workspace_domains = excluded.allow_external_workspace_domains,
			approved_workspace_domains = excluded.approved_workspace_domains,
			allow_external_users_connect_org_workspaces = excluded.allow_external_users_connect_org_workspaces,
			allowed_external_workspace_connectors = excluded.allowed_external_workspace_connectors,
			blacklist_workspace_access = excluded.blacklist_workspace_access,
			blocked_workspace_emails = excluded.blocked_workspace_emails,
			deny_all_workspace_connections_except = excluded.deny_all_workspace_connections_except,
			allowed_workspace_emails = excluded.allowed_workspace_emails,
			enforce_firewall = excluded.enforce_firewall,
			allow_user_two_factor_reset = excluded.allow_user_two_factor_reset,
			updated_at = excluded.updated_at;`,
		strings.TrimSpace(policy.OrganizationID), policy.EnforceSessionTimeout, timeoutHours, policy.RequireTwoFactor,
		policy.AllowCustomPolicies, maxRisk, policy.AllowNonAdminInvites,
		policy.AllowExternalWorkspaceDomains, string(rawDomains),
		policy.AllowExternalUsersConnectOrgWorkspaces, string(rawExternalConnectors),
		policy.BlacklistWorkspaceAccess, string(rawBlockedWorkspaceEmails),
		policy.DenyAllWorkspaceConnectionsExcept, string(rawAllowedWorkspaceEmails),
		policy.EnforceFirewall,
		policy.AllowUserTwoFactorReset, now, now)
}

func (s *Store) ListOrganizationFirewallRules(organizationID string) ([]OrganizationFirewallRule, error) {
	rows, err := s.db.Query(`SELECT * FROM organization_firewall_rules WHERE organization_id = ? ORDER BY created_at ASC, value ASC;`, strings.TrimSpace(organizationID))
	if err != nil {
		return nil, err
	}
	out := make([]OrganizationFirewallRule, 0, len(rows))
	for _, row := range rows {
		if rule := organizationFirewallRuleFromRow(row); rule != nil {
			out = append(out, *rule)
		}
	}
	return out, nil
}

func (s *Store) AddOrganizationFirewallRule(rule *OrganizationFirewallRule) error {
	if rule == nil || strings.TrimSpace(rule.OrganizationID) == "" {
		return fmt.Errorf("organization firewall rule is required")
	}
	if strings.TrimSpace(rule.ID) == "" {
		id, err := RandomToken("ofw_", 12)
		if err != nil {
			return err
		}
		rule.ID = id
	}
	now := nowUTC()
	return s.db.Exec(`INSERT INTO organization_firewall_rules (id, organization_id, value, ip_version, address_kind, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?);`, strings.TrimSpace(rule.ID), strings.TrimSpace(rule.OrganizationID), strings.TrimSpace(rule.Value), strings.TrimSpace(rule.IPVersion), strings.TrimSpace(rule.AddressKind), now, now)
}

func (s *Store) DeleteOrganizationFirewallRule(organizationID, ruleID string) error {
	return s.db.Exec(`DELETE FROM organization_firewall_rules WHERE organization_id = ? AND id = ?;`, strings.TrimSpace(organizationID), strings.TrimSpace(ruleID))
}

func (s *Store) ReplaceOrganizationFirewallRules(organizationID string, rules []OrganizationFirewallRule) error {
	organizationID = strings.TrimSpace(organizationID)
	if organizationID == "" {
		return fmt.Errorf("organization is required")
	}
	if err := s.db.Exec(`DELETE FROM organization_firewall_rules WHERE organization_id = ?;`, organizationID); err != nil {
		return err
	}
	for _, rule := range rules {
		rule.OrganizationID = organizationID
		if err := s.AddOrganizationFirewallRule(&rule); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) EnsureOrganizationForDomain(domain string) (*Organization, error) {
	domain = normalizeDomain(domain)
	if domain == "" {
		return nil, nil
	}
	existing, err := s.FindOrganizationByDomain(domain)
	if err != nil || existing != nil {
		return existing, err
	}
	orgID, err := RandomToken("org_", 16)
	if err != nil {
		return nil, err
	}
	now := nowUTC()
	if err := s.db.Exec(`INSERT INTO organizations (id, name, created_at, updated_at)
		VALUES (?, ?, ?, ?);`, orgID, domain, now, now); err != nil {
		return nil, err
	}
	if err := s.db.Exec(`INSERT OR IGNORE INTO organization_domains (organization_id, domain_name, is_primary, created_at, updated_at)
		VALUES (?, ?, 1, ?, ?);`, orgID, domain, now, now); err != nil {
		return nil, err
	}
	resolved, err := s.FindOrganizationByDomain(domain)
	if err != nil {
		return nil, err
	}
	if resolved == nil {
		return nil, fmt.Errorf("organization for domain %q could not be resolved", domain)
	}
	if resolved.ID != orgID {
		_ = s.db.Exec(`DELETE FROM organizations
			WHERE id = ?
			  AND NOT EXISTS (SELECT 1 FROM organization_domains WHERE organization_id = ?)
			  AND NOT EXISTS (SELECT 1 FROM users WHERE organization_id = ?);`, orgID, orgID, orgID)
	}
	return resolved, nil
}

func (s *Store) syncUserOrganizations() error {
	users, err := s.ListUsers()
	if err != nil {
		return err
	}
	for _, user := range users {
		wantOrganizationID := ""
		if domain := organizationDomainForEmail(user.Email); domain != "" {
			org, err := s.EnsureOrganizationForDomain(domain)
			if err != nil {
				return err
			}
			if org != nil {
				wantOrganizationID = org.ID
			}
		}
		if strings.TrimSpace(user.OrganizationID) == wantOrganizationID {
			continue
		}
		var value any
		if wantOrganizationID != "" {
			value = wantOrganizationID
		}
		if err := s.db.Exec(`UPDATE users SET organization_id = ?, updated_at = ? WHERE id = ?;`, value, nowUTC(), user.ID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) FindUserByEmail(email string) (*User, error) {
	row, err := s.db.QueryOne(`SELECT * FROM users WHERE email = ?;`, normalizeEmail(email))
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

func (s *Store) ListOrganizationUserSummaries(organizationID string) ([]OrganizationUserSummary, error) {
	organizationID = strings.TrimSpace(organizationID)
	if organizationID == "" {
		return []OrganizationUserSummary{}, nil
	}
	rows, err := s.db.Query(`SELECT
			u.*,
			COALESCE((
				SELECT MAX(activity_ts) FROM (
					SELECT MAX(created_at) AS activity_ts FROM request_logs WHERE user_id = u.id
					UNION ALL
					SELECT MAX(created_at) AS activity_ts FROM policy_audit_logs WHERE user_id = u.id
					UNION ALL
					SELECT MAX(created_at) AS activity_ts FROM workspace_audit_logs WHERE user_id = u.id
					UNION ALL
					SELECT MAX(created_at) AS activity_ts FROM drive_folder_audit_logs WHERE user_id = u.id
					UNION ALL
					SELECT MAX(created_at) AS activity_ts FROM user_audit_logs WHERE user_id = u.id
				)
			), '') AS last_activity_at
		FROM users u
		WHERE u.organization_id = ?
		ORDER BY LOWER(COALESCE(TRIM(u.name), '')) ASC, LOWER(TRIM(u.email)) ASC;`, organizationID)
	if err != nil {
		return nil, err
	}
	out := make([]OrganizationUserSummary, 0, len(rows))
	for _, row := range rows {
		user := userFromRow(row)
		if user == nil {
			continue
		}
		out = append(out, OrganizationUserSummary{
			User:           *user,
			LastActivityAt: parseTime(row["last_activity_at"]),
		})
	}
	return out, nil
}

func (s *Store) CreateOrUpdateUser(email, name, picture string, isAdmin bool) (*User, error) {
	email = normalizeEmail(email)
	var organizationID any
	if domain := organizationDomainForEmail(email); domain != "" {
		org, err := s.EnsureOrganizationForDomain(domain)
		if err != nil {
			return nil, err
		}
		if org != nil {
			organizationID = org.ID
		}
	}
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
		if err := s.db.Exec(`INSERT INTO users (id, email, name, picture, organization_id, is_admin, is_suspended, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, 0, ?, ?);`, id, email, name, picture, organizationID, isAdmin, now, now); err != nil {
			return nil, err
		}
		return s.FindUserByID(id)
	}
	if err := s.db.Exec(`UPDATE users
		SET name = ?, picture = ?, organization_id = ?, is_admin = ?, updated_at = ?
		WHERE id = ?;`, name, picture, organizationID, isAdmin, now, existing.ID); err != nil {
		return nil, err
	}
	return s.FindUserByID(existing.ID)
}

func (s *Store) SetUserSuspended(userID string, suspended bool) error {
	return s.SetUserSuspendedBy(userID, suspended, "")
}

func (s *Store) SetUserSuspendedBy(userID string, suspended bool, actorUserID string) error {
	now := nowUTC()
	if suspended {
		return s.db.Exec(`UPDATE users
			SET is_suspended = ?, suspended_at = ?, suspended_by_user_id = ?, updated_at = ?
			WHERE id = ?;`, true, now, strings.TrimSpace(actorUserID), now, userID)
	}
	return s.db.Exec(`UPDATE users
		SET is_suspended = ?, suspended_at = NULL, suspended_by_user_id = '', updated_at = ?
		WHERE id = ?;`, false, now, userID)
}

func (s *Store) DeleteUser(userID string) error {
	for _, stmt := range []struct {
		q    string
		args []any
	}{
		{`DELETE FROM sessions WHERE user_id = ?;`, []any{userID}},
		{`DELETE FROM oauth_states WHERE user_id = ?;`, []any{userID}},
		{`DELETE FROM agent_drive_folder_grants WHERE user_id = ?;`, []any{userID}},
		{`DELETE FROM agent_workspace_grants WHERE user_id = ?;`, []any{userID}},
		{`DELETE FROM agents WHERE user_id = ?;`, []any{userID}},
		{`DELETE FROM user_backend_api_tokens WHERE user_id = ?;`, []any{userID}},
		{`DELETE FROM agent_skill_download_tokens WHERE user_id = ?;`, []any{userID}},
		{`DELETE FROM workspace_order WHERE user_id = ?;`, []any{userID}},
		{`DELETE FROM drive_folder_refs WHERE user_id = ?;`, []any{userID}},
		{`DELETE FROM gmail_connections WHERE user_id = ?;`, []any{userID}},
		{`DELETE FROM user_daily_stats WHERE user_id = ?;`, []any{userID}},
		{`DELETE FROM user_settings WHERE user_id = ?;`, []any{userID}},
		{`DELETE FROM user_two_factor_settings WHERE user_id = ?;`, []any{userID}},
		{`DELETE FROM user_logging_settings WHERE user_id = ?;`, []any{userID}},
		{`DELETE FROM request_logs WHERE user_id = ?;`, []any{userID}},
		{`DELETE FROM user_log_view_settings WHERE user_id = ?;`, []any{userID}},
		{`DELETE FROM policy_audit_logs WHERE user_id = ?;`, []any{userID}},
		{`DELETE FROM workspace_audit_logs WHERE user_id = ?;`, []any{userID}},
		{`DELETE FROM drive_folder_audit_logs WHERE user_id = ?;`, []any{userID}},
		{`DELETE FROM user_audit_logs WHERE user_id = ?;`, []any{userID}},
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
	return s.CreateSessionWithSecondFactor(userID, tokenHash, expiresAt, true)
}

func (s *Store) CreateSessionWithSecondFactor(userID, tokenHash string, expiresAt time.Time, secondFactorVerified bool) error {
	id, err := RandomToken("ses_", 12)
	if err != nil {
		return err
	}
	return s.db.Exec(`INSERT INTO sessions (id, user_id, token_hash, second_factor_verified, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?);`, id, userID, tokenHash, secondFactorVerified, expiresAt, nowUTC())
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

func (s *Store) UpdateSessionSecondFactorVerified(tokenHash string, secondFactorVerified bool) error {
	return s.db.Exec(`UPDATE sessions SET second_factor_verified = ? WHERE token_hash = ?;`, secondFactorVerified, tokenHash)
}

func (s *Store) UpdateSessionExpiry(tokenHash string, expiresAt time.Time) error {
	return s.db.Exec(`UPDATE sessions SET expires_at = ? WHERE token_hash = ?;`, expiresAt, tokenHash)
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

func (s *Store) SaveWorkspaceOAuthState(stateHash, userID, mailboxEmail string, expiresAt time.Time) error {
	return s.db.Exec(`INSERT OR REPLACE INTO workspace_oauth_states (state_hash, user_id, mailbox_email, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?);`,
		stateHash, strings.TrimSpace(userID), normalizeEmail(mailboxEmail), expiresAt, nowUTC())
}

func (s *Store) ConsumeWorkspaceOAuthState(stateHash string) (userID string, mailboxEmail string, ok bool, err error) {
	row, err := s.db.QueryOne(`SELECT * FROM workspace_oauth_states WHERE state_hash = ?;`, stateHash)
	if err != nil {
		return "", "", false, err
	}
	if row == nil {
		return "", "", false, nil
	}
	_ = s.db.Exec(`DELETE FROM workspace_oauth_states WHERE state_hash = ?;`, stateHash)
	if parseTime(row["expires_at"]).Before(nowUTC()) {
		return "", "", false, nil
	}
	return row["user_id"], normalizeEmail(row["mailbox_email"]), true, nil
}

func (s *Store) SaveWorkspaceReauthToken(tokenHash, userID, mailboxEmail string, expiresAt time.Time) error {
	return s.db.Exec(`INSERT INTO workspace_reauth_tokens (token_hash, user_id, mailbox_email, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?);`,
		tokenHash, strings.TrimSpace(userID), normalizeEmail(mailboxEmail), expiresAt, nowUTC())
}

func (s *Store) GetWorkspaceReauthToken(tokenHash string) (userID string, mailboxEmail string, expired bool, ok bool, err error) {
	row, err := s.db.QueryOne(`SELECT * FROM workspace_reauth_tokens WHERE token_hash = ?;`, tokenHash)
	if err != nil {
		return "", "", false, false, err
	}
	if row == nil {
		return "", "", false, false, nil
	}
	if parseTime(row["expires_at"]).Before(nowUTC()) {
		return row["user_id"], normalizeEmail(row["mailbox_email"]), true, true, nil
	}
	return row["user_id"], normalizeEmail(row["mailbox_email"]), false, true, nil
}

func (s *Store) DeleteWorkspaceReauthToken(tokenHash string) error {
	return s.db.Exec(`DELETE FROM workspace_reauth_tokens WHERE token_hash = ?;`, tokenHash)
}

func (s *Store) SaveAgent(agent *AgentAccess) error {
	if agent == nil {
		return fmt.Errorf("agent is required")
	}
	agent.ID = strings.TrimSpace(agent.ID)
	agent.UserID = strings.TrimSpace(agent.UserID)
	agent.FriendlyName = strings.TrimSpace(agent.FriendlyName)
	agent.DefaultLocation = strings.TrimSpace(agent.DefaultLocation)
	if agent.ID == "" {
		return fmt.Errorf("agent id is required")
	}
	if agent.UserID == "" {
		return fmt.Errorf("user id is required")
	}
	if agent.FriendlyName == "" {
		return fmt.Errorf("agent name is required")
	}
	if agent.DefaultLocation == "" {
		return fmt.Errorf("agent location is required")
	}
	if len([]rune(agent.FriendlyName)) > maxAgentNameLen {
		return fmt.Errorf("agent name exceeds maximum length of %d characters", maxAgentNameLen)
	}
	if len([]rune(agent.DefaultLocation)) > maxAgentLocationLen {
		return fmt.Errorf("agent location exceeds maximum length of %d characters", maxAgentLocationLen)
	}
	now := nowUTC()
	enabled := agent.Enabled
	firewallEnabled := 0
	if agent.FirewallEnabled {
		firewallEnabled = 1
	}
	if agent.TokenEnc == "" {
		existing, err := s.GetAgent(agent.UserID, agent.ID)
		if err != nil {
			return err
		}
		if existing == nil {
			return fmt.Errorf("agent token is required")
		}
		agent.TokenEnc = existing.TokenEnc
		agent.TokenHint = existing.TokenHint
	}
	skillStale := 0
	if agent.SkillStale {
		skillStale = 1
	}
	var skillStaleAt any
	if !agent.SkillStaleAt.IsZero() {
		skillStaleAt = agent.SkillStaleAt
	}
	return s.db.Exec(`INSERT INTO agents (id, user_id, friendly_name, default_location, token_enc, token_hint, enabled, firewall_enabled, skill_stale, skill_stale_reason, skill_stale_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			friendly_name = excluded.friendly_name,
			default_location = excluded.default_location,
			token_enc = excluded.token_enc,
			token_hint = excluded.token_hint,
			enabled = excluded.enabled,
			firewall_enabled = excluded.firewall_enabled,
			skill_stale = excluded.skill_stale,
			skill_stale_reason = excluded.skill_stale_reason,
			skill_stale_at = excluded.skill_stale_at,
			updated_at = excluded.updated_at;`,
		agent.ID, agent.UserID, agent.FriendlyName, agent.DefaultLocation, agent.TokenEnc, agent.TokenHint, enabled,
		firewallEnabled, skillStale, strings.TrimSpace(agent.SkillStaleReason), skillStaleAt, now, now)
}

func (s *Store) GetAgent(userID, agentID string) (*AgentAccess, error) {
	row, err := s.db.QueryOne(`SELECT * FROM agents WHERE user_id = ? AND id = ?;`, userID, strings.TrimSpace(agentID))
	if err != nil {
		return nil, err
	}
	return agentFromRow(row), nil
}

func (s *Store) ListAgents(userID string) ([]AgentAccess, error) {
	rows, err := s.db.Query(`SELECT * FROM agents WHERE user_id = ? ORDER BY friendly_name ASC, created_at ASC;`, userID)
	if err != nil {
		return nil, err
	}
	out := make([]AgentAccess, 0, len(rows))
	for _, row := range rows {
		if agent := agentFromRow(row); agent != nil {
			out = append(out, *agent)
		}
	}
	return out, nil
}

func (s *Store) ListEnabledAgents() ([]AgentAccess, error) {
	rows, err := s.db.Query(`SELECT * FROM agents WHERE enabled = 1 ORDER BY created_at ASC;`)
	if err != nil {
		return nil, err
	}
	out := make([]AgentAccess, 0, len(rows))
	for _, row := range rows {
		if agent := agentFromRow(row); agent != nil {
			out = append(out, *agent)
		}
	}
	return out, nil
}

func (s *Store) TouchAgentUsage(agentID string) error {
	now := nowUTC()
	return s.db.Exec(`UPDATE agents SET last_used_at = ?, updated_at = ? WHERE id = ?;`, now, now, strings.TrimSpace(agentID))
}

func (s *Store) MarkAgentSkillStale(userID, agentID, reason string) error {
	userID = strings.TrimSpace(userID)
	agentID = strings.TrimSpace(agentID)
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "Settings used to generate this agent skill changed."
	}
	agent, err := s.GetAgent(userID, agentID)
	if err != nil {
		return err
	}
	if agent == nil {
		return fmt.Errorf("agent not found")
	}
	reasons := splitSkillStaleReasons(agent.SkillStaleReason)
	found := false
	for _, existing := range reasons {
		if existing == reason {
			found = true
			break
		}
	}
	if !found {
		reasons = append(reasons, reason)
	}
	now := nowUTC()
	return s.db.Exec(`UPDATE agents SET skill_stale = 1, skill_stale_reason = ?, skill_stale_at = ?, updated_at = ? WHERE user_id = ? AND id = ?;`,
		strings.Join(reasons, "\n"), now, now, userID, agentID)
}

func (s *Store) MarkAgentsSkillStale(userID string, agentIDs []string, reason string) error {
	seen := map[string]bool{}
	for _, agentID := range agentIDs {
		agentID = strings.TrimSpace(agentID)
		if agentID == "" || seen[agentID] {
			continue
		}
		seen[agentID] = true
		if err := s.MarkAgentSkillStale(userID, agentID, reason); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ClearAgentSkillStale(userID, agentID string) error {
	now := nowUTC()
	return s.db.Exec(`UPDATE agents SET skill_stale = 0, skill_stale_reason = '', skill_stale_at = NULL, updated_at = ? WHERE user_id = ? AND id = ?;`,
		now, strings.TrimSpace(userID), strings.TrimSpace(agentID))
}

func (s *Store) SetAgentFirewallEnabled(userID, agentID string, enabled bool) error {
	value := 0
	if enabled {
		value = 1
	}
	now := nowUTC()
	return s.db.Exec(`UPDATE agents SET firewall_enabled = ?, updated_at = ? WHERE user_id = ? AND id = ?;`,
		value, now, strings.TrimSpace(userID), strings.TrimSpace(agentID))
}

func (s *Store) ListAgentFirewallRules(userID, agentID string) ([]AgentFirewallRule, error) {
	rows, err := s.db.Query(`SELECT * FROM agent_firewall_rules WHERE user_id = ? AND agent_id = ? ORDER BY created_at ASC, value ASC;`,
		strings.TrimSpace(userID), strings.TrimSpace(agentID))
	if err != nil {
		return nil, err
	}
	out := make([]AgentFirewallRule, 0, len(rows))
	for _, row := range rows {
		if rule := agentFirewallRuleFromRow(row); rule != nil {
			out = append(out, *rule)
		}
	}
	return out, nil
}

func (s *Store) AddAgentFirewallRule(rule *AgentFirewallRule) error {
	if rule == nil {
		return fmt.Errorf("firewall rule is required")
	}
	rule.ID = strings.TrimSpace(rule.ID)
	rule.AgentID = strings.TrimSpace(rule.AgentID)
	rule.UserID = strings.TrimSpace(rule.UserID)
	rule.Value = strings.TrimSpace(rule.Value)
	rule.IPVersion = strings.TrimSpace(rule.IPVersion)
	rule.AddressKind = strings.TrimSpace(rule.AddressKind)
	if rule.ID == "" || rule.AgentID == "" || rule.UserID == "" || rule.Value == "" || rule.IPVersion == "" || rule.AddressKind == "" {
		return fmt.Errorf("firewall rule is incomplete")
	}
	if agent, err := s.GetAgent(rule.UserID, rule.AgentID); err != nil {
		return err
	} else if agent == nil {
		return fmt.Errorf("agent not found")
	}
	now := nowUTC()
	if rule.CreatedAt.IsZero() {
		rule.CreatedAt = now
	}
	rule.UpdatedAt = now
	return s.db.Exec(`INSERT INTO agent_firewall_rules (id, agent_id, user_id, value, ip_version, address_kind, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?);`,
		rule.ID, rule.AgentID, rule.UserID, rule.Value, rule.IPVersion, rule.AddressKind, rule.CreatedAt, rule.UpdatedAt)
}

func (s *Store) DeleteAgentFirewallRule(userID, agentID, ruleID string) error {
	return s.db.Exec(`DELETE FROM agent_firewall_rules WHERE user_id = ? AND agent_id = ? AND id = ?;`,
		strings.TrimSpace(userID), strings.TrimSpace(agentID), strings.TrimSpace(ruleID))
}

func (s *Store) DeleteAgent(userID, agentID string) error {
	return s.db.Exec(`DELETE FROM agents WHERE user_id = ? AND id = ?;`, userID, strings.TrimSpace(agentID))
}

func (s *Store) SaveAgentWorkspaceGrants(userID, agentID string, grants []AgentWorkspaceGrant) error {
	agent, err := s.GetAgent(userID, agentID)
	if err != nil {
		return err
	}
	if agent == nil {
		return fmt.Errorf("agent not found")
	}
	now := nowUTC()
	if err := s.db.Exec(`DELETE FROM agent_workspace_grants WHERE user_id = ? AND agent_id = ?;`, userID, agentID); err != nil {
		return err
	}
	for _, grant := range grants {
		mailboxEmail := normalizeEmail(grant.MailboxEmail)
		if mailboxEmail == "" {
			continue
		}
		if conn, err := s.GetGmailConnection(userID, mailboxEmail); err != nil {
			return err
		} else if conn == nil {
			return fmt.Errorf("workspace not found: %s", mailboxEmail)
		}
		policyID := strings.TrimSpace(grant.PolicyID)
		if policyID == "" {
			policyID = systemPolicyID
		}
		if policyID != systemPolicyID {
			policy, err := s.GetUserPolicy(userID, policyID)
			if err != nil {
				return err
			}
			if policy == nil {
				return fmt.Errorf("policy not found: %s", policyID)
			}
		}
		requireAgentMotive := 0
		if grant.RequireAgentMotive {
			requireAgentMotive = 1
		}
		if err := s.db.Exec(`INSERT INTO agent_workspace_grants (agent_id, user_id, mailbox_email, policy_id, require_agent_motive, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?);`, agentID, userID, mailboxEmail, policyID, requireAgentMotive, now, now); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ListAgentWorkspaceGrants(userID, agentID string) ([]AgentWorkspaceGrant, error) {
	rows, err := s.db.Query(`SELECT * FROM agent_workspace_grants WHERE user_id = ? AND agent_id = ? ORDER BY mailbox_email ASC;`, userID, strings.TrimSpace(agentID))
	if err != nil {
		return nil, err
	}
	out := make([]AgentWorkspaceGrant, 0, len(rows))
	for _, row := range rows {
		if grant := agentWorkspaceGrantFromRow(row); grant != nil {
			out = append(out, *grant)
		}
	}
	return out, nil
}

func (s *Store) GetAgentWorkspaceGrant(userID, agentID, mailboxEmail string) (*AgentWorkspaceGrant, error) {
	row, err := s.db.QueryOne(`SELECT * FROM agent_workspace_grants WHERE user_id = ? AND agent_id = ? AND mailbox_email = ?;`, userID, strings.TrimSpace(agentID), normalizeEmail(mailboxEmail))
	if err != nil {
		return nil, err
	}
	return agentWorkspaceGrantFromRow(row), nil
}

func (s *Store) AgentIDsForWorkspace(userID, mailboxEmail string) ([]string, error) {
	rows, err := s.db.Query(`SELECT DISTINCT agent_id FROM agent_workspace_grants WHERE user_id = ? AND mailbox_email = ? ORDER BY agent_id ASC;`, userID, normalizeEmail(mailboxEmail))
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, row["agent_id"])
	}
	return out, nil
}

func (s *Store) AgentIDsForPolicy(userID, policyID string) ([]string, error) {
	rows, err := s.db.Query(`SELECT DISTINCT agent_id FROM agent_workspace_grants WHERE user_id = ? AND policy_id = ? ORDER BY agent_id ASC;`, userID, strings.TrimSpace(policyID))
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, row["agent_id"])
	}
	return out, nil
}

func (s *Store) AgentIDsForDriveFolderRef(userID, mailboxEmail, folderRefID string) ([]string, error) {
	rows, err := s.db.Query(`SELECT DISTINCT agent_id FROM agent_drive_folder_grants WHERE user_id = ? AND mailbox_email = ? AND folder_ref_id = ? ORDER BY agent_id ASC;`, userID, normalizeEmail(mailboxEmail), strings.TrimSpace(folderRefID))
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, row["agent_id"])
	}
	return out, nil
}

func (s *Store) SaveAgentDriveFolderGrants(userID, agentID, mailboxEmail string, folderRefIDs []string) error {
	agentID = strings.TrimSpace(agentID)
	mailboxEmail = normalizeEmail(mailboxEmail)
	agent, err := s.GetAgent(userID, agentID)
	if err != nil {
		return err
	}
	if agent == nil {
		return fmt.Errorf("agent not found")
	}
	if conn, err := s.GetGmailConnection(userID, mailboxEmail); err != nil {
		return err
	} else if conn == nil {
		return fmt.Errorf("workspace not found: %s", mailboxEmail)
	}
	validFolderIDs := []string{}
	seen := map[string]bool{}
	for _, id := range folderRefIDs {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		folder, err := s.FindDriveFolderRefByID(userID, mailboxEmail, id)
		if err != nil {
			return err
		}
		if folder == nil {
			return fmt.Errorf("Drive folder access not found: %s", id)
		}
		seen[id] = true
		validFolderIDs = append(validFolderIDs, id)
	}
	if err := s.db.Exec(`DELETE FROM agent_drive_folder_grants WHERE user_id = ? AND agent_id = ? AND mailbox_email = ?;`, userID, agentID, mailboxEmail); err != nil {
		return err
	}
	now := nowUTC()
	for _, id := range validFolderIDs {
		if err := s.db.Exec(`INSERT INTO agent_drive_folder_grants (agent_id, user_id, mailbox_email, folder_ref_id, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?);`, agentID, userID, mailboxEmail, id, now, now); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ListAgentDriveFolderGrants(userID, agentID, mailboxEmail string) ([]AgentDriveFolderGrant, error) {
	rows, err := s.db.Query(`SELECT * FROM agent_drive_folder_grants WHERE user_id = ? AND agent_id = ? AND mailbox_email = ? ORDER BY folder_ref_id ASC;`, userID, strings.TrimSpace(agentID), normalizeEmail(mailboxEmail))
	if err != nil {
		return nil, err
	}
	out := make([]AgentDriveFolderGrant, 0, len(rows))
	for _, row := range rows {
		if grant := agentDriveFolderGrantFromRow(row); grant != nil {
			out = append(out, *grant)
		}
	}
	return out, nil
}

func (s *Store) ListAgentAllowedDriveFolderRefs(userID, agentID, mailboxEmail string) ([]AllowedDriveFolder, error) {
	rows, err := s.db.Query(`SELECT d.* FROM drive_folder_refs d
		INNER JOIN agent_drive_folder_grants g ON g.user_id = d.user_id AND g.folder_ref_id = d.id
		WHERE g.user_id = ? AND g.agent_id = ? AND g.mailbox_email = ? AND d.mailbox_email = ?
		ORDER BY d.reference_name ASC;`, userID, strings.TrimSpace(agentID), normalizeEmail(mailboxEmail), normalizeEmail(mailboxEmail))
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

func (s *Store) SaveUserBackendAPIToken(userID, tokenEnc, tokenHint string) error {
	now := nowUTC()
	return s.db.Exec(`INSERT INTO user_backend_api_tokens (user_id, token_enc, token_hint, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			token_enc = excluded.token_enc,
			token_hint = excluded.token_hint,
			updated_at = excluded.updated_at;`,
		userID, tokenEnc, tokenHint, now, now)
}

func (s *Store) GetUserBackendAPITokenRecord(userID string) (map[string]string, error) {
	return s.db.QueryOne(`SELECT * FROM user_backend_api_tokens WHERE user_id = ?;`, userID)
}

func (s *Store) TouchUserBackendAPITokenUsage(userID string) error {
	now := nowUTC()
	return s.db.Exec(`UPDATE user_backend_api_tokens SET last_used_at = ?, updated_at = ? WHERE user_id = ?;`, now, now, userID)
}

func (s *Store) SaveAgentSkillDownloadToken(userID, agentID, tokenHash, platform string, expiresAt time.Time) error {
	now := nowUTC()
	return s.db.Exec(`INSERT INTO agent_skill_download_tokens (user_id, agent_id, token_hash, platform, expires_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id, agent_id) DO UPDATE SET
			token_hash = excluded.token_hash,
			platform = excluded.platform,
			expires_at = excluded.expires_at,
			updated_at = excluded.updated_at;`,
		userID, agentID, tokenHash, platform, expiresAt, now, now)
}

func (s *Store) ConsumeAgentSkillDownloadToken(tokenHash string) (userID string, agentID string, platform string, ok bool, expired bool, err error) {
	row, err := s.db.QueryOne(`SELECT * FROM agent_skill_download_tokens WHERE token_hash = ?;`, tokenHash)
	if err != nil || row == nil {
		return "", "", "", false, false, err
	}
	_ = s.db.Exec(`DELETE FROM agent_skill_download_tokens WHERE token_hash = ?;`, tokenHash)
	if parseTime(row["expires_at"]).Before(nowUTC()) {
		return row["user_id"], row["agent_id"], row["platform"], false, true, nil
	}
	return row["user_id"], row["agent_id"], row["platform"], true, false, nil
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
	if err := s.db.Exec(`INSERT INTO gmail_connections (user_id, mailbox_email, friendly_name, scopes, access_token_enc, refresh_token_enc, token_expiry, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id, mailbox_email) DO UPDATE SET
			friendly_name = excluded.friendly_name,
			scopes = excluded.scopes,
			access_token_enc = excluded.access_token_enc,
			refresh_token_enc = excluded.refresh_token_enc,
			token_expiry = excluded.token_expiry,
			updated_at = excluded.updated_at;`,
		conn.UserID, conn.MailboxEmail, friendlyName, conn.Scopes, conn.AccessTokenEnc, conn.RefreshTokenEnc, conn.TokenExpiry, now, now); err != nil {
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

func (s *Store) DisconnectGmailConnectionOAuth(userID, mailboxEmail string) error {
	return s.db.Exec(`UPDATE gmail_connections
		SET access_token_enc = '', refresh_token_enc = '', token_expiry = ?, updated_at = ?
		WHERE user_id = ? AND mailbox_email = ?;`,
		time.Time{}, nowUTC(), strings.TrimSpace(userID), normalizeEmail(mailboxEmail))
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
		return &UserSettings{UserID: userID, Timezone: "UTC", DefaultPolicyID: systemPolicyID, SessionTimeoutHours: defaultUserSessionTimeoutHours, CreatedAt: now, UpdatedAt: now}, nil
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

func (s *Store) SaveUserSessionTimeoutHours(userID string, sessionTimeoutHours int) error {
	if sessionTimeoutHours < 1 {
		return fmt.Errorf("session timeout must be at least 1 hour")
	}
	if sessionTimeoutHours > maxUserSessionTimeoutHours {
		return fmt.Errorf("session timeout cannot exceed %d hours", maxUserSessionTimeoutHours)
	}
	settings, err := s.GetUserSettings(userID)
	if err != nil {
		return err
	}
	now := nowUTC()
	return s.db.Exec(`INSERT INTO user_settings (user_id, timezone, default_policy_id, session_timeout_hours, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			session_timeout_hours = excluded.session_timeout_hours,
			updated_at = excluded.updated_at;`, userID, settings.Timezone, settings.DefaultPolicyID, sessionTimeoutHours, now, now)
}

func (s *Store) GetUserTwoFactorSettings(userID string) (*UserTwoFactorSettings, error) {
	row, err := s.db.QueryOne(`SELECT * FROM user_two_factor_settings WHERE user_id = ?;`, userID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		now := nowUTC()
		return &UserTwoFactorSettings{
			UserID:    userID,
			CreatedAt: now,
			UpdatedAt: now,
		}, nil
	}
	return userTwoFactorSettingsFromRow(row), nil
}

func (s *Store) SaveUserTwoFactorPendingSecret(userID, pendingSecretEnc string) error {
	now := nowUTC()
	settings, err := s.GetUserTwoFactorSettings(userID)
	if err != nil {
		return err
	}
	createdAt := settings.CreatedAt
	if createdAt.IsZero() {
		createdAt = now
	}
	return s.db.Exec(`INSERT INTO user_two_factor_settings (user_id, enabled, secret_enc, pending_secret_enc, pending_secret_set_at, created_at, updated_at)
		VALUES (?, 0, '', ?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			enabled = 0,
			secret_enc = '',
			pending_secret_enc = excluded.pending_secret_enc,
			pending_secret_set_at = excluded.pending_secret_set_at,
			updated_at = excluded.updated_at;`, userID, pendingSecretEnc, now, createdAt, now)
}

func (s *Store) EnableUserTwoFactor(userID, secretEnc string) error {
	now := nowUTC()
	settings, err := s.GetUserTwoFactorSettings(userID)
	if err != nil {
		return err
	}
	createdAt := settings.CreatedAt
	if createdAt.IsZero() {
		createdAt = now
	}
	return s.db.Exec(`INSERT INTO user_two_factor_settings (user_id, enabled, secret_enc, pending_secret_enc, pending_secret_set_at, created_at, updated_at)
		VALUES (?, 1, ?, '', '', ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			enabled = 1,
			secret_enc = excluded.secret_enc,
			pending_secret_enc = '',
			pending_secret_set_at = '',
			updated_at = excluded.updated_at;`, userID, secretEnc, createdAt, now)
}

func (s *Store) CancelUserTwoFactorPendingSecret(userID string) error {
	now := nowUTC()
	return s.db.Exec(`UPDATE user_two_factor_settings
		SET pending_secret_enc = '',
			pending_secret_set_at = '',
			updated_at = ?
		WHERE user_id = ?;`, now, userID)
}

func (s *Store) ResetUserTwoFactor(userID string) error {
	now := nowUTC()
	settings, err := s.GetUserTwoFactorSettings(userID)
	if err != nil {
		return err
	}
	createdAt := settings.CreatedAt
	if createdAt.IsZero() {
		createdAt = now
	}
	return s.db.Exec(`INSERT INTO user_two_factor_settings (user_id, enabled, secret_enc, pending_secret_enc, pending_secret_set_at, created_at, updated_at)
		VALUES (?, 0, '', '', '', ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			enabled = 0,
			secret_enc = '',
			pending_secret_enc = '',
			pending_secret_set_at = '',
			updated_at = excluded.updated_at;`, userID, createdAt, now)
}

func (s *Store) GetUserLoggingSettings(userID string) (*UserLoggingSettings, error) {
	row, err := s.db.QueryOne(`SELECT * FROM user_logging_settings WHERE user_id = ?;`, userID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		now := nowUTC()
		return &UserLoggingSettings{UserID: userID, RetentionDays: 7, CreatedAt: now, UpdatedAt: now}, nil
	}
	return userLoggingSettingsFromRow(row), nil
}

func (s *Store) SaveUserLoggingSettings(userID string, requireAgentContext bool, retentionDays int) error {
	if retentionDays < 1 {
		return fmt.Errorf("retention days must be at least 1")
	}
	if retentionDays > 365 {
		return fmt.Errorf("retention days cannot exceed 365")
	}
	now := nowUTC()
	return s.db.Exec(`INSERT INTO user_logging_settings (user_id, require_agent_context, retention_days, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			require_agent_context = excluded.require_agent_context,
			retention_days = excluded.retention_days,
			updated_at = excluded.updated_at;`, userID, requireAgentContext, retentionDays, now, now)
}

func (s *Store) GetUserLogViewSettings(userID, viewKey string) (*UserLogViewSettings, error) {
	row, err := s.db.QueryOne(`SELECT * FROM user_log_view_settings WHERE user_id = ? AND view_key = ?;`, userID, viewKey)
	if err != nil {
		return nil, err
	}
	return userLogViewSettingsFromRow(row), nil
}

func (s *Store) SaveUserLogViewSettings(settings *UserLogViewSettings) error {
	if settings == nil {
		return fmt.Errorf("log view settings are required")
	}
	if strings.TrimSpace(settings.UserID) == "" || strings.TrimSpace(settings.ViewKey) == "" {
		return fmt.Errorf("log view user and key are required")
	}
	if settings.PageSize < 50 || settings.PageSize > 250 {
		return fmt.Errorf("page size must be between 50 and 250")
	}
	rawColumns, err := json.Marshal(settings.VisibleColumns)
	if err != nil {
		return err
	}
	rawWidths, err := json.Marshal(settings.ColumnWidths)
	if err != nil {
		return err
	}
	now := nowUTC()
	return s.db.Exec(`INSERT INTO user_log_view_settings (user_id, view_key, visible_columns, column_widths, page_size, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id, view_key) DO UPDATE SET
			visible_columns = excluded.visible_columns,
			column_widths = excluded.column_widths,
			page_size = excluded.page_size,
			updated_at = excluded.updated_at;`,
		settings.UserID, settings.ViewKey, string(rawColumns), string(rawWidths), settings.PageSize, now, now)
}

func (s *Store) SaveRequestLog(entry *RequestLogEntry) error {
	if entry.ID == "" {
		id, err := RandomToken("rlg_", 12)
		if err != nil {
			return err
		}
		entry.ID = id
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = nowUTC()
	}
	return s.db.Exec(`INSERT INTO request_logs (
		id, request_id, user_id, user_email, workspace_email, agent_id, created_at, method, service, path, query,
		target_object_type, target_object_id,
		user_agent, remote_addr, x_forwarded_for, x_real_ip, forwarded, cf_connecting_ip,
		agent_name, agent_location, agent_motive, human_approval, outcome, http_status, upstream_status,
		policy_id, policy_name, policy_capability_key, policy_capability_title, policy_rule_name, error_message
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`,
		entry.ID, entry.RequestID, entry.UserID, entry.UserEmail, entry.WorkspaceEmail, entry.AgentID, entry.CreatedAt,
		entry.Method, entry.Service, entry.Path, entry.Query, entry.TargetObjectType, entry.TargetObjectID, entry.UserAgent, entry.RemoteAddr,
		entry.XForwardedFor, entry.XRealIP, entry.Forwarded, entry.CFConnectingIP, entry.AgentName,
		entry.AgentLocation, entry.AgentMotive, entry.HumanApproval, entry.Outcome, entry.HTTPStatus, entry.UpstreamStatus,
		entry.PolicyID, entry.PolicyName, entry.PolicyCapabilityKey, entry.PolicyCapabilityTitle,
		entry.PolicyRuleName, entry.ErrorMessage)
}

func (s *Store) ListRequestLogs(userID string, limit int) ([]RequestLogEntry, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.Query(`SELECT * FROM request_logs WHERE user_id = ? ORDER BY created_at DESC LIMIT ?;`, userID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]RequestLogEntry, 0, len(rows))
	for _, row := range rows {
		if entry := requestLogEntryFromRow(row); entry != nil {
			out = append(out, *entry)
		}
	}
	return out, nil
}

func (s *Store) ListRequestLogsInRange(userID string, start, end time.Time) ([]RequestLogEntry, error) {
	rows, err := s.db.Query(`SELECT * FROM request_logs
		WHERE user_id = ? AND created_at >= ? AND created_at <= ?
		ORDER BY created_at DESC, id DESC;`, userID, start, end)
	if err != nil {
		return nil, err
	}
	out := make([]RequestLogEntry, 0, len(rows))
	for _, row := range rows {
		if entry := requestLogEntryFromRow(row); entry != nil {
			out = append(out, *entry)
		}
	}
	return out, nil
}

func (s *Store) CountRequestLogsByAgentSince(userID, agentID string, since time.Time) (int, error) {
	rows, err := s.db.Query(`SELECT COUNT(*) AS count FROM request_logs WHERE user_id = ? AND agent_id = ? AND created_at >= ?;`, userID, strings.TrimSpace(agentID), since)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	return atoiSafe(rows[0]["count"]), nil
}

func (s *Store) CountAgentRequestLogsSince(userID string, since time.Time) (int, error) {
	rows, err := s.db.Query(`SELECT COUNT(*) AS count FROM request_logs WHERE user_id = ? AND agent_id != '' AND created_at >= ?;`, userID, since)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	return atoiSafe(rows[0]["count"]), nil
}

func (s *Store) CountVisibleDriveFolders(userID string) (int, error) {
	rows, err := s.db.Query(`SELECT COUNT(*) AS count FROM drive_folder_tree_cache WHERE user_id = ?;`, userID)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	return atoiSafe(rows[0]["count"]), nil
}

func (s *Store) CleanupExpiredLogs() error {
	settingsRows, err := s.db.Query(`SELECT u.id AS user_id, u.is_admin AS is_admin, COALESCE(uls.retention_days, 7) AS retention_days
		FROM users u
		LEFT JOIN user_logging_settings uls ON u.id = uls.user_id;`)
	if err != nil {
		return err
	}
	for _, row := range settingsRows {
		retentionDays := 7
		if row["is_admin"] == "1" {
			retentionDays = atoiSafe(row["retention_days"])
		}
		if retentionDays < 1 || retentionDays > 365 {
			retentionDays = 7
		}
		cutoff := nowUTC().AddDate(0, 0, -retentionDays)
		for _, table := range []string{"request_logs", "policy_audit_logs", "workspace_audit_logs", "drive_folder_audit_logs", "user_audit_logs"} {
			if err := s.db.Exec(fmt.Sprintf(`DELETE FROM %s WHERE user_id = ? AND created_at < ?;`, table), row["user_id"], cutoff); err != nil {
				return err
			}
		}
	}
	defaultCutoff := nowUTC().AddDate(0, 0, -7)
	for _, table := range []string{"request_logs", "policy_audit_logs", "workspace_audit_logs", "drive_folder_audit_logs", "user_audit_logs"} {
		if err := s.db.Exec(fmt.Sprintf(`DELETE FROM %s
			WHERE user_id NOT IN (SELECT user_id FROM user_logging_settings)
			AND created_at < ?;`, table), defaultCutoff); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) SavePolicyAuditLog(entry *AuditLogEntry) error {
	return s.saveAuditLog("policy_audit_logs", entry)
}

func (s *Store) SaveWorkspaceAuditLog(entry *AuditLogEntry) error {
	return s.saveAuditLog("workspace_audit_logs", entry)
}

func (s *Store) SaveDriveFolderAuditLog(entry *AuditLogEntry) error {
	return s.saveAuditLog("drive_folder_audit_logs", entry)
}

func (s *Store) SaveUserAuditLog(entry *AuditLogEntry) error {
	return s.saveAuditLog("user_audit_logs", entry)
}

func (s *Store) saveAuditLog(table string, entry *AuditLogEntry) error {
	if entry.ID == "" {
		id, err := RandomToken("alg_", 12)
		if err != nil {
			return err
		}
		entry.ID = id
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = nowUTC()
	}
	query := fmt.Sprintf(`INSERT INTO %s (
		id, user_id, user_email, workspace_email, created_at, action, target_id, target_name,
		user_agent, remote_addr, x_forwarded_for, x_real_ip, forwarded, cf_connecting_ip, details_json
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`, table)
	return s.db.Exec(query,
		entry.ID, entry.UserID, entry.UserEmail, entry.WorkspaceEmail, entry.CreatedAt, entry.Action, entry.TargetID, entry.TargetName,
		entry.UserAgent, entry.RemoteAddr, entry.XForwardedFor, entry.XRealIP, entry.Forwarded, entry.CFConnectingIP, entry.DetailsJSON)
}

func (s *Store) ListAuditLogs(table, userID string, limit int) ([]AuditLogEntry, error) {
	switch table {
	case "policy_audit_logs", "workspace_audit_logs", "drive_folder_audit_logs", "user_audit_logs":
	default:
		return nil, fmt.Errorf("unknown audit log table")
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.Query(fmt.Sprintf(`SELECT * FROM %s WHERE user_id = ? ORDER BY created_at DESC LIMIT ?;`, table), userID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]AuditLogEntry, 0, len(rows))
	for _, row := range rows {
		if entry := auditLogEntryFromRow(row); entry != nil {
			out = append(out, *entry)
		}
	}
	return out, nil
}

func (s *Store) ListAuditLogsInRange(table, userID string, start, end time.Time) ([]AuditLogEntry, error) {
	switch table {
	case "policy_audit_logs", "workspace_audit_logs", "drive_folder_audit_logs", "user_audit_logs":
	default:
		return nil, fmt.Errorf("unknown audit log table")
	}
	rows, err := s.db.Query(fmt.Sprintf(`SELECT * FROM %s
		WHERE user_id = ? AND created_at >= ? AND created_at <= ?
		ORDER BY created_at DESC, id DESC;`, table), userID, start, end)
	if err != nil {
		return nil, err
	}
	out := make([]AuditLogEntry, 0, len(rows))
	for _, row := range rows {
		if entry := auditLogEntryFromRow(row); entry != nil {
			out = append(out, *entry)
		}
	}
	return out, nil
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
	if policy.ID == systemPolicyID {
		return fmt.Errorf("system default policy cannot be modified")
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
	policy.ReviewRequiredCapabilities = validCapabilityKeys(policy.ReviewRequiredCapabilities)
	enabledSet := sliceToSet(policy.EnabledCapabilities)
	filteredReviewRequired := make([]string, 0, len(policy.ReviewRequiredCapabilities))
	for _, key := range policy.ReviewRequiredCapabilities {
		if enabledSet[key] {
			filteredReviewRequired = append(filteredReviewRequired, key)
		}
	}
	policy.ReviewRequiredCapabilities = filteredReviewRequired
	rawCapabilities, err := json.Marshal(policy.EnabledCapabilities)
	if err != nil {
		return err
	}
	rawReviewRequired, err := json.Marshal(policy.ReviewRequiredCapabilities)
	if err != nil {
		return err
	}
	now := nowUTC()
	return s.db.Exec(`INSERT INTO user_policies (id, user_id, name, capabilities, review_required_capabilities, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			capabilities = excluded.capabilities,
			review_required_capabilities = excluded.review_required_capabilities,
			updated_at = excluded.updated_at;`,
		policy.ID, policy.UserID, policy.Name, string(rawCapabilities), string(rawReviewRequired), now, now)
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
	usedByAgentGrant, err := s.PolicyUsedByAgentGrant(userID, policyID)
	if err != nil {
		return false, err
	}
	if wasDefault {
		if err := s.SaveDefaultPolicyID(userID, systemPolicyID); err != nil {
			return false, err
		}
	}
	if usedByAgentGrant {
		if err := s.db.Exec(`UPDATE agent_workspace_grants SET policy_id = ?, updated_at = ? WHERE user_id = ? AND policy_id = ?;`,
			systemPolicyID, nowUTC(), userID, policyID); err != nil {
			return false, err
		}
	}
	if err := s.db.Exec(`DELETE FROM user_policies WHERE user_id = ? AND id = ?;`, userID, policyID); err != nil {
		return false, err
	}
	return wasDefault || usedByAgentGrant, nil
}

func (s *Store) PolicyUsedByAgentGrant(userID, policyID string) (bool, error) {
	row, err := s.db.QueryOne(`SELECT COUNT(*) AS cnt FROM agent_workspace_grants WHERE user_id = ? AND policy_id = ?;`, userID, policyID)
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

func (s *Store) FindDriveFolderRefByFolderID(userID, mailboxEmail, folderID string) (*AllowedDriveFolder, error) {
	row, err := s.db.QueryOne(`SELECT * FROM drive_folder_refs WHERE user_id = ? AND mailbox_email = ? AND folder_id = ?;`, userID, normalizeEmail(mailboxEmail), strings.TrimSpace(folderID))
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
	return s.db.Exec(`INSERT INTO drive_folder_refs (id, user_id, mailbox_email, reference_name, reference_key, folder_url, folder_id, folder_name, resource_key, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`,
		f.ID, f.UserID, normalizeEmail(f.MailboxEmail), f.ReferenceName, f.ReferenceKey, f.FolderURL, f.FolderID, f.FolderName, f.ResourceKey, now, now)
}

func (s *Store) UpdateDriveFolderRef(f *AllowedDriveFolder) error {
	now := nowUTC()
	return s.db.Exec(`UPDATE drive_folder_refs SET reference_name = ?, reference_key = ?, folder_url = ?, folder_id = ?, folder_name = ?, resource_key = ?, updated_at = ? WHERE user_id = ? AND mailbox_email = ? AND id = ?;`,
		f.ReferenceName, f.ReferenceKey, f.FolderURL, f.FolderID, f.FolderName, f.ResourceKey, now, f.UserID, normalizeEmail(f.MailboxEmail), f.ID)
}

func (s *Store) DeleteDriveFolderRef(userID, mailboxEmail, id string) error {
	if err := s.DeleteDriveFolderTree(userID, id); err != nil {
		return err
	}
	if err := s.db.Exec(`DELETE FROM agent_drive_folder_grants WHERE user_id = ? AND folder_ref_id = ?;`, userID, strings.TrimSpace(id)); err != nil {
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

func splitSkillStaleReasons(value string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, line := range strings.Split(value, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || seen[line] {
			continue
		}
		seen[line] = true
		out = append(out, line)
	}
	return out
}

func userFromRow(row map[string]string) *User {
	if row == nil {
		return nil
	}
	return &User{
		ID:                row["id"],
		Email:             row["email"],
		Name:              row["name"],
		Picture:           row["picture"],
		OrganizationID:    row["organization_id"],
		IsAdmin:           row["is_admin"] == "1",
		IsSuspended:       row["is_suspended"] == "1",
		SuspendedAt:       parseTime(row["suspended_at"]),
		SuspendedByUserID: row["suspended_by_user_id"],
		CreatedAt:         parseTime(row["created_at"]),
		UpdatedAt:         parseTime(row["updated_at"]),
	}
}

func (s *Store) GetUserLastActivityAt(userID string) (time.Time, error) {
	row, err := s.db.QueryOne(`SELECT COALESCE((
		SELECT MAX(activity_ts) FROM (
			SELECT MAX(created_at) AS activity_ts FROM request_logs WHERE user_id = ?
			UNION ALL
			SELECT MAX(created_at) AS activity_ts FROM policy_audit_logs WHERE user_id = ?
			UNION ALL
			SELECT MAX(created_at) AS activity_ts FROM workspace_audit_logs WHERE user_id = ?
			UNION ALL
			SELECT MAX(created_at) AS activity_ts FROM drive_folder_audit_logs WHERE user_id = ?
			UNION ALL
			SELECT MAX(created_at) AS activity_ts FROM user_audit_logs WHERE user_id = ?
		)
	), '') AS last_activity_at;`, userID, userID, userID, userID, userID)
	if err != nil {
		return time.Time{}, err
	}
	return parseTime(row["last_activity_at"]), nil
}

func organizationFromRow(row map[string]string) *Organization {
	if row == nil {
		return nil
	}
	return &Organization{
		ID:        row["id"],
		Name:      row["name"],
		CreatedAt: parseTime(row["created_at"]),
		UpdatedAt: parseTime(row["updated_at"]),
	}
}

func organizationAccountsPolicyFromRow(row map[string]string) *OrganizationAccountsPolicy {
	if row == nil {
		return nil
	}
	domains := []string{}
	_ = json.Unmarshal([]byte(row["approved_workspace_domains"]), &domains)
	externalConnectors := []string{}
	_ = json.Unmarshal([]byte(row["allowed_external_workspace_connectors"]), &externalConnectors)
	blockedWorkspaceEmails := []string{}
	_ = json.Unmarshal([]byte(row["blocked_workspace_emails"]), &blockedWorkspaceEmails)
	allowedWorkspaceEmails := []string{}
	_ = json.Unmarshal([]byte(row["allowed_workspace_emails"]), &allowedWorkspaceEmails)
	sessionTimeoutHours := atoiSafe(row["session_timeout_hours"])
	if sessionTimeoutHours < 1 || sessionTimeoutHours > maxUserSessionTimeoutHours {
		sessionTimeoutHours = defaultUserSessionTimeoutHours
	}
	customPolicyMaxRisk := atoiSafe(row["custom_policy_max_risk"])
	if customPolicyMaxRisk < 1 {
		customPolicyMaxRisk = 1
	}
	if customPolicyMaxRisk > 3 {
		customPolicyMaxRisk = 3
	}
	return &OrganizationAccountsPolicy{
		OrganizationID:                         row["organization_id"],
		EnforceSessionTimeout:                  row["enforce_session_timeout"] == "1",
		SessionTimeoutHours:                    sessionTimeoutHours,
		RequireTwoFactor:                       row["require_two_factor"] == "1",
		AllowCustomPolicies:                    row["allow_custom_policies"] == "1",
		CustomPolicyMaxRisk:                    customPolicyMaxRisk,
		AllowNonAdminInvites:                   row["allow_non_admin_invites"] == "1",
		AllowExternalWorkspaceDomains:          row["allow_external_workspace_domains"] == "1",
		ApprovedWorkspaceDomains:               normalizeDomainList(domains),
		AllowExternalUsersConnectOrgWorkspaces: row["allow_external_users_connect_org_workspaces"] == "1",
		AllowedExternalWorkspaceConnectors:     normalizeEmailOrDomainList(externalConnectors),
		BlacklistWorkspaceAccess:               row["blacklist_workspace_access"] == "1",
		BlockedWorkspaceEmails:                 normalizeEmailList(blockedWorkspaceEmails),
		DenyAllWorkspaceConnectionsExcept:      row["deny_all_workspace_connections_except"] == "1",
		AllowedWorkspaceEmails:                 normalizeEmailList(allowedWorkspaceEmails),
		EnforceFirewall:                        row["enforce_firewall"] == "1",
		AllowUserTwoFactorReset:                row["allow_user_two_factor_reset"] == "1",
		CreatedAt:                              parseTime(row["created_at"]),
		UpdatedAt:                              parseTime(row["updated_at"]),
	}
}

func organizationFirewallRuleFromRow(row map[string]string) *OrganizationFirewallRule {
	if row == nil {
		return nil
	}
	return &OrganizationFirewallRule{
		ID:             row["id"],
		OrganizationID: row["organization_id"],
		Value:          row["value"],
		IPVersion:      row["ip_version"],
		AddressKind:    row["address_kind"],
		CreatedAt:      parseTime(row["created_at"]),
		UpdatedAt:      parseTime(row["updated_at"]),
	}
}

func sessionFromRow(row map[string]string) *Session {
	if row == nil {
		return nil
	}
	return &Session{
		ID:                   row["id"],
		UserID:               row["user_id"],
		TokenHash:            row["token_hash"],
		SecondFactorVerified: row["second_factor_verified"] != "0",
		ExpiresAt:            parseTime(row["expires_at"]),
		CreatedAt:            parseTime(row["created_at"]),
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
	sessionTimeoutHours := atoiSafe(row["session_timeout_hours"])
	if sessionTimeoutHours < 1 || sessionTimeoutHours > maxUserSessionTimeoutHours {
		sessionTimeoutHours = defaultUserSessionTimeoutHours
	}
	return &UserSettings{
		UserID:              row["user_id"],
		Timezone:            row["timezone"],
		DefaultPolicyID:     defaultPolicyID,
		SessionTimeoutHours: sessionTimeoutHours,
		CreatedAt:           parseTime(row["created_at"]),
		UpdatedAt:           parseTime(row["updated_at"]),
	}
}

func userLoggingSettingsFromRow(row map[string]string) *UserLoggingSettings {
	if row == nil {
		return nil
	}
	retentionDays := atoiSafe(row["retention_days"])
	if retentionDays < 1 {
		retentionDays = 7
	}
	return &UserLoggingSettings{
		UserID:              row["user_id"],
		RequireAgentContext: row["require_agent_context"] == "1",
		RetentionDays:       retentionDays,
		CreatedAt:           parseTime(row["created_at"]),
		UpdatedAt:           parseTime(row["updated_at"]),
	}
}

func userTwoFactorSettingsFromRow(row map[string]string) *UserTwoFactorSettings {
	if row == nil {
		return nil
	}
	return &UserTwoFactorSettings{
		UserID:             row["user_id"],
		Enabled:            row["enabled"] == "1",
		SecretEnc:          row["secret_enc"],
		PendingSecretEnc:   row["pending_secret_enc"],
		PendingSecretSetAt: parseTime(row["pending_secret_set_at"]),
		CreatedAt:          parseTime(row["created_at"]),
		UpdatedAt:          parseTime(row["updated_at"]),
	}
}

func agentFromRow(row map[string]string) *AgentAccess {
	if row == nil {
		return nil
	}
	return &AgentAccess{
		ID:               row["id"],
		UserID:           row["user_id"],
		FriendlyName:     row["friendly_name"],
		DefaultLocation:  row["default_location"],
		TokenEnc:         row["token_enc"],
		TokenHint:        row["token_hint"],
		Enabled:          row["enabled"] != "0",
		FirewallEnabled:  row["firewall_enabled"] == "1",
		LastUsedAt:       parseTime(row["last_used_at"]),
		SkillStale:       row["skill_stale"] == "1",
		SkillStaleReason: row["skill_stale_reason"],
		SkillStaleAt:     parseTime(row["skill_stale_at"]),
		CreatedAt:        parseTime(row["created_at"]),
		UpdatedAt:        parseTime(row["updated_at"]),
	}
}

func agentFirewallRuleFromRow(row map[string]string) *AgentFirewallRule {
	if row == nil {
		return nil
	}
	return &AgentFirewallRule{
		ID:          row["id"],
		AgentID:     row["agent_id"],
		UserID:      row["user_id"],
		Value:       row["value"],
		IPVersion:   row["ip_version"],
		AddressKind: row["address_kind"],
		CreatedAt:   parseTime(row["created_at"]),
		UpdatedAt:   parseTime(row["updated_at"]),
	}
}

func agentWorkspaceGrantFromRow(row map[string]string) *AgentWorkspaceGrant {
	if row == nil {
		return nil
	}
	policyID := strings.TrimSpace(row["policy_id"])
	if policyID == "" {
		policyID = systemPolicyID
	}
	return &AgentWorkspaceGrant{
		AgentID:            row["agent_id"],
		UserID:             row["user_id"],
		MailboxEmail:       normalizeEmail(row["mailbox_email"]),
		PolicyID:           policyID,
		RequireAgentMotive: row["require_agent_motive"] == "1",
		CreatedAt:          parseTime(row["created_at"]),
		UpdatedAt:          parseTime(row["updated_at"]),
	}
}

func agentDriveFolderGrantFromRow(row map[string]string) *AgentDriveFolderGrant {
	if row == nil {
		return nil
	}
	return &AgentDriveFolderGrant{
		AgentID:      row["agent_id"],
		UserID:       row["user_id"],
		MailboxEmail: normalizeEmail(row["mailbox_email"]),
		FolderRefID:  row["folder_ref_id"],
		CreatedAt:    parseTime(row["created_at"]),
		UpdatedAt:    parseTime(row["updated_at"]),
	}
}

func userLogViewSettingsFromRow(row map[string]string) *UserLogViewSettings {
	if row == nil {
		return nil
	}
	var columns []string
	_ = json.Unmarshal([]byte(row["visible_columns"]), &columns)
	widths := map[string]int{}
	_ = json.Unmarshal([]byte(row["column_widths"]), &widths)
	return &UserLogViewSettings{
		UserID:         row["user_id"],
		ViewKey:        row["view_key"],
		VisibleColumns: columns,
		ColumnWidths:   widths,
		PageSize:       atoiSafe(row["page_size"]),
		CreatedAt:      parseTime(row["created_at"]),
		UpdatedAt:      parseTime(row["updated_at"]),
	}
}

func userPolicyFromRow(row map[string]string) *UserPolicy {
	if row == nil {
		return nil
	}
	var capabilities []string
	_ = json.Unmarshal([]byte(row["capabilities"]), &capabilities)
	var reviewRequired []string
	_ = json.Unmarshal([]byte(row["review_required_capabilities"]), &reviewRequired)
	return &UserPolicy{
		ID:                         row["id"],
		UserID:                     row["user_id"],
		Name:                       row["name"],
		EnabledCapabilities:        validCapabilityKeys(capabilities),
		ReviewRequiredCapabilities: validCapabilityKeys(reviewRequired),
		CreatedAt:                  parseTime(row["created_at"]),
		UpdatedAt:                  parseTime(row["updated_at"]),
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
		ID:            row["id"],
		UserID:        row["user_id"],
		MailboxEmail:  row["mailbox_email"],
		ReferenceName: row["reference_name"],
		ReferenceKey:  row["reference_key"],
		FolderURL:     row["folder_url"],
		FolderID:      row["folder_id"],
		FolderName:    row["folder_name"],
		ResourceKey:   row["resource_key"],
		CreatedAt:     parseTime(row["created_at"]),
		UpdatedAt:     parseTime(row["updated_at"]),
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

func requestLogEntryFromRow(row map[string]string) *RequestLogEntry {
	if row == nil {
		return nil
	}
	return &RequestLogEntry{
		ID:                    row["id"],
		RequestID:             row["request_id"],
		UserID:                row["user_id"],
		UserEmail:             row["user_email"],
		WorkspaceEmail:        row["workspace_email"],
		AgentID:               row["agent_id"],
		CreatedAt:             parseTime(row["created_at"]),
		Method:                row["method"],
		Service:               row["service"],
		Path:                  row["path"],
		Query:                 row["query"],
		TargetObjectType:      row["target_object_type"],
		TargetObjectID:        row["target_object_id"],
		UserAgent:             row["user_agent"],
		RemoteAddr:            row["remote_addr"],
		XForwardedFor:         row["x_forwarded_for"],
		XRealIP:               row["x_real_ip"],
		Forwarded:             row["forwarded"],
		CFConnectingIP:        row["cf_connecting_ip"],
		AgentName:             row["agent_name"],
		AgentLocation:         row["agent_location"],
		AgentMotive:           row["agent_motive"],
		HumanApproval:         row["human_approval"],
		Outcome:               row["outcome"],
		HTTPStatus:            atoiSafe(row["http_status"]),
		UpstreamStatus:        atoiSafe(row["upstream_status"]),
		PolicyID:              row["policy_id"],
		PolicyName:            row["policy_name"],
		PolicyCapabilityKey:   row["policy_capability_key"],
		PolicyCapabilityTitle: row["policy_capability_title"],
		PolicyRuleName:        row["policy_rule_name"],
		ErrorMessage:          row["error_message"],
	}
}

func auditLogEntryFromRow(row map[string]string) *AuditLogEntry {
	if row == nil {
		return nil
	}
	return &AuditLogEntry{
		ID:             row["id"],
		UserID:         row["user_id"],
		UserEmail:      row["user_email"],
		WorkspaceEmail: row["workspace_email"],
		CreatedAt:      parseTime(row["created_at"]),
		Action:         row["action"],
		TargetID:       row["target_id"],
		TargetName:     row["target_name"],
		UserAgent:      row["user_agent"],
		RemoteAddr:     row["remote_addr"],
		XForwardedFor:  row["x_forwarded_for"],
		XRealIP:        row["x_real_ip"],
		Forwarded:      row["forwarded"],
		CFConnectingIP: row["cf_connecting_ip"],
		DetailsJSON:    row["details_json"],
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

func (s *Store) FindAgentByToken(crypto *Crypto, token string) (*User, *AgentAccess, error) {
	rows, err := s.db.Query(`SELECT u.id AS user_id, u.email, u.name, u.picture, u.is_admin, u.is_suspended, u.created_at AS user_created_at, u.updated_at AS user_updated_at,
			a.id AS agent_id, a.friendly_name, a.default_location, a.token_enc AS agent_token_enc, a.token_hint, a.enabled, a.firewall_enabled, a.last_used_at, a.created_at AS agent_created_at, a.updated_at AS agent_updated_at
		FROM agents a
		JOIN users u ON u.id = a.user_id
		WHERE a.enabled = 1;`)
	if err != nil {
		return nil, nil, err
	}
	for _, row := range rows {
		enc := row["agent_token_enc"]
		if enc == "" {
			continue
		}
		dec, err := crypto.Decrypt(enc)
		if err != nil {
			continue
		}
		if subtleEqual(dec, token) {
			user := &User{
				ID:          row["user_id"],
				Email:       row["email"],
				Name:        row["name"],
				Picture:     row["picture"],
				IsAdmin:     row["is_admin"] == "1",
				IsSuspended: row["is_suspended"] == "1",
				CreatedAt:   parseTime(row["user_created_at"]),
				UpdatedAt:   parseTime(row["user_updated_at"]),
			}
			agent := &AgentAccess{
				ID:              row["agent_id"],
				UserID:          row["user_id"],
				FriendlyName:    row["friendly_name"],
				DefaultLocation: row["default_location"],
				TokenEnc:        enc,
				TokenHint:       row["token_hint"],
				Enabled:         row["enabled"] == "1",
				FirewallEnabled: row["firewall_enabled"] == "1",
				LastUsedAt:      parseTime(row["last_used_at"]),
				CreatedAt:       parseTime(row["agent_created_at"]),
				UpdatedAt:       parseTime(row["agent_updated_at"]),
			}
			return user, agent, nil
		}
	}
	return nil, nil, nil
}

func (s *Store) FindUserByUserBackendAPIToken(crypto *Crypto, token string) (*User, error) {
	rows, err := s.db.Query(`SELECT u.*, t.token_enc AS backend_token_enc
		FROM user_backend_api_tokens t
		JOIN users u ON u.id = t.user_id;`)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		enc := row["backend_token_enc"]
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
