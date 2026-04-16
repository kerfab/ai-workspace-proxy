package main

import "time"

type User struct {
	ID          string
	Email       string
	Name        string
	Picture     string
	IsAdmin     bool
	IsSuspended bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Session struct {
	ID        string
	UserID    string
	TokenHash string
	ExpiresAt time.Time
	CreatedAt time.Time
}

type GmailConnection struct {
	UserID          string
	MailboxEmail    string
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

type AllowedDriveFolder struct {
	ID              string
	UserID          string
	ReferenceName   string
	ReferenceKey    string
	FolderURL       string
	FolderID        string
	FolderName      string
	ResourceKey     string
	AllowDocs       bool
	AllowSheets     bool
	AllowSlides     bool
	AllowDriveFiles bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
