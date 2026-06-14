package main

import "testing"

func TestExtractStructuredFileID(t *testing.T) {
	tests := []struct {
		name    string
		service string
		path    string
		want    string
	}{
		{
			name:    "docs get",
			service: "docs",
			path:    "/v1/documents/doc123",
			want:    "doc123",
		},
		{
			name:    "docs batch update",
			service: "docs",
			path:    "/v1/documents/doc123:batchUpdate",
			want:    "doc123",
		},
		{
			name:    "docs encoded batch update",
			service: "docs",
			path:    "/v1/documents/doc123%3AbatchUpdate",
			want:    "doc123",
		},
		{
			name:    "sheets get",
			service: "sheets",
			path:    "/v4/spreadsheets/sheet123",
			want:    "sheet123",
		},
		{
			name:    "sheets batch update",
			service: "sheets",
			path:    "/v4/spreadsheets/sheet123:batchUpdate",
			want:    "sheet123",
		},
		{
			name:    "sheets encoded batch update",
			service: "sheets",
			path:    "/v4/spreadsheets/sheet123%3AbatchUpdate",
			want:    "sheet123",
		},
		{
			name:    "sheets values update",
			service: "sheets",
			path:    "/v4/spreadsheets/sheet123/values/Sheet1!A1",
			want:    "sheet123",
		},
		{
			name:    "sheets values batch update",
			service: "sheets",
			path:    "/v4/spreadsheets/sheet123/values:batchUpdate",
			want:    "sheet123",
		},
		{
			name:    "slides get",
			service: "slides",
			path:    "/v1/presentations/pres123",
			want:    "pres123",
		},
		{
			name:    "slides batch update",
			service: "slides",
			path:    "/v1/presentations/pres123:batchUpdate",
			want:    "pres123",
		},
		{
			name:    "slides encoded batch update",
			service: "slides",
			path:    "/v1/presentations/pres123%3AbatchUpdate",
			want:    "pres123",
		},
		{
			name:    "slides page get",
			service: "slides",
			path:    "/v1/presentations/pres123/pages/page123",
			want:    "pres123",
		},
		{
			name:    "wrong service path",
			service: "sheets",
			path:    "/v1/documents/doc123",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractStructuredFileID(tt.service, tt.path)
			if got != tt.want {
				t.Fatalf("extractStructuredFileID(%q, %q) = %q, want %q", tt.service, tt.path, got, tt.want)
			}
		})
	}
}
