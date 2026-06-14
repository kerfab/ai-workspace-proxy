package main

import "testing"

func TestNormalizeProxyTargetStructuredMethodSeparators(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "docs encoded batch update",
			raw:  "/docs.googleapis.com/v1/documents/doc123%3AbatchUpdate",
			want: "/v1/documents/doc123:batchUpdate",
		},
		{
			name: "sheets encoded batch update",
			raw:  "/sheets.googleapis.com/v4/spreadsheets/sheet123%3AbatchUpdate",
			want: "/v4/spreadsheets/sheet123:batchUpdate",
		},
		{
			name: "sheets encoded value method keeps range escaping",
			raw:  "/sheets.googleapis.com/v4/spreadsheets/sheet123/values/Sheet%201%2FA1%3Aclear",
			want: "/v4/spreadsheets/sheet123/values/Sheet%201%2FA1:clear",
		},
		{
			name: "slides encoded batch update",
			raw:  "/slides.googleapis.com/v1/presentations/pres123%3abatchUpdate",
			want: "/v1/presentations/pres123:batchUpdate",
		},
		{
			name: "people encoded contact search method",
			raw:  "/people.googleapis.com/v1/people%3AsearchContacts",
			want: "/v1/people:searchContacts",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeProxyTarget(tt.raw, "user@example.com")
			if err != nil {
				t.Fatalf("normalizeProxyTarget returned error: %v", err)
			}
			if got.normalizedPath != tt.want {
				t.Fatalf("normalizedPath = %q, want %q", got.normalizedPath, tt.want)
			}
		})
	}
}

func TestNormalizeProxyTargetPeopleOnlyAllowsContactSearch(t *testing.T) {
	allowed, err := normalizeProxyTarget("/people.googleapis.com/v1/otherContacts:search", "user@example.com")
	if err != nil {
		t.Fatalf("normalizeProxyTarget allowed path returned error: %v", err)
	}
	if allowed.serviceLabel != "people" {
		t.Fatalf("serviceLabel = %q, want people", allowed.serviceLabel)
	}

	if _, err := normalizeProxyTarget("/people.googleapis.com/v1/people/me/connections", "user@example.com"); err == nil {
		t.Fatal("expected non-search People API path to be denied")
	}
}
