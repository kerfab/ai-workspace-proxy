// Copyright (c) 2026 Opensense Ltd. (Hong Kong). All rights reserved.
// Proprietary software. No use, copy, modification, distribution, disclosure,
// or reverse engineering is permitted without prior written authorization
// from Opensense Ltd.

package main

import "time"

type User struct {
	ID                string
	Email             string
	Name              string
	Picture           string
	OrganizationID    string
	IsAdmin           bool
	IsSuspended       bool
	SuspendedAt       time.Time
	SuspendedByUserID string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type OrganizationUserSummary struct {
	User           User
	LastActivityAt time.Time
}

type Organization struct {
	ID        string
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type OrganizationDomain struct {
	OrganizationID string
	DomainName     string
	IsPrimary      bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type OrganizationAdminVerification struct {
	UserID            string
	OrganizationID    string
	DomainName        string
	VerificationHost  string
	VerificationValue string
	VerifiedAt        time.Time
	LastCheckedAt     time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type OrganizationAccountsPolicy struct {
	OrganizationID                         string
	EnforceSessionTimeout                  bool
	SessionTimeoutHours                    int
	RequireTwoFactor                       bool
	AllowCustomPolicies                    bool
	CustomPolicyMaxRisk                    int
	AllowNonAdminInvites                   bool
	AllowExternalWorkspaceDomains          bool
	ApprovedWorkspaceDomains               []string
	AllowExternalUsersConnectOrgWorkspaces bool
	AllowedExternalWorkspaceConnectors     []string
	BlacklistWorkspaceAccess               bool
	BlockedWorkspaceEmails                 []string
	DenyAllWorkspaceConnectionsExcept      bool
	AllowedWorkspaceEmails                 []string
	EnforceFirewall                        bool
	AllowUserTwoFactorReset                bool
	CreatedAt                              time.Time
	UpdatedAt                              time.Time
}

type OrganizationFirewallRule struct {
	ID             string
	OrganizationID string
	Value          string
	IPVersion      string
	AddressKind    string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type Session struct {
	ID                   string
	UserID               string
	TokenHash            string
	SecondFactorVerified bool
	ExpiresAt            time.Time
	CreatedAt            time.Time
}

type GmailConnection struct {
	UserID          string
	MailboxEmail    string
	FriendlyName    string
	Scopes          string
	AccessTokenEnc  string
	RefreshTokenEnc string
	TokenExpiry     time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type UserDailyStat struct {
	Day          string
	AllowedCount int
	DeniedCount  int
}

type UserSettings struct {
	UserID              string
	Timezone            string
	DefaultPolicyID     string
	SessionTimeoutHours int
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type UserTwoFactorSettings struct {
	UserID             string
	Enabled            bool
	SecretEnc          string
	PendingSecretEnc   string
	PendingSecretSetAt time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type UserLoggingSettings struct {
	UserID              string
	RequireAgentContext bool
	RetentionDays       int
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type AgentAccess struct {
	ID               string
	UserID           string
	FriendlyName     string
	DefaultLocation  string
	TokenEnc         string
	TokenHint        string
	Enabled          bool
	FirewallEnabled  bool
	LastUsedAt       time.Time
	SkillStale       bool
	SkillStaleReason string
	SkillStaleAt     time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type AgentFirewallRule struct {
	ID          string
	AgentID     string
	UserID      string
	Value       string
	IPVersion   string
	AddressKind string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type AgentWorkspaceGrant struct {
	AgentID            string
	UserID             string
	MailboxEmail       string
	PolicyID           string
	RequireAgentMotive bool
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type AgentDriveFolderGrant struct {
	AgentID      string
	UserID       string
	MailboxEmail string
	FolderRefID  string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type UserLogViewSettings struct {
	UserID         string
	ViewKey        string
	VisibleColumns []string
	ColumnWidths   map[string]int
	PageSize       int
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type UserPolicy struct {
	ID                         string
	UserID                     string
	Name                       string
	EnabledCapabilities        []string
	ReviewRequiredCapabilities []string
	CreatedAt                  time.Time
	UpdatedAt                  time.Time
}

type AllowedDriveFolder struct {
	ID            string
	UserID        string
	MailboxEmail  string
	ReferenceName string
	ReferenceKey  string
	FolderURL     string
	FolderID      string
	FolderName    string
	ResourceKey   string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type DriveFolderTreeEntry struct {
	UserID         string
	RootRefID      string
	FolderID       string
	ParentFolderID string
	FolderName     string
	Path           string
	PathKey        string
	Depth          int
	ResourceKey    string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type RequestLogEntry struct {
	ID                    string
	RequestID             string
	UserID                string
	UserEmail             string
	WorkspaceEmail        string
	AgentID               string
	CreatedAt             time.Time
	Method                string
	Service               string
	Path                  string
	Query                 string
	TargetObjectType      string
	TargetObjectID        string
	UserAgent             string
	RemoteAddr            string
	XForwardedFor         string
	XRealIP               string
	Forwarded             string
	CFConnectingIP        string
	AgentName             string
	AgentLocation         string
	AgentMotive           string
	HumanApproval         string
	Outcome               string
	HTTPStatus            int
	UpstreamStatus        int
	PolicyID              string
	PolicyName            string
	PolicyCapabilityKey   string
	PolicyCapabilityTitle string
	PolicyRuleName        string
	ErrorMessage          string
}

type AuditLogEntry struct {
	ID             string
	UserID         string
	UserEmail      string
	WorkspaceEmail string
	CreatedAt      time.Time
	Action         string
	TargetID       string
	TargetName     string
	UserAgent      string
	RemoteAddr     string
	XForwardedFor  string
	XRealIP        string
	Forwarded      string
	CFConnectingIP string
	DetailsJSON    string
}
