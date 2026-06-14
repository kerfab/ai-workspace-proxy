package main

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestValidatePolicyCatalog(t *testing.T) {
	if err := ValidatePolicyCatalog(); err != nil {
		t.Fatal(err)
	}
}

func TestSystemDefaultPolicyMatchesLowRiskCapabilities(t *testing.T) {
	for _, capability := range PolicyCatalog() {
		wantDefault := policyRiskScore(capability.Key) == 1
		if capability.SystemDefault != wantDefault {
			t.Fatalf("capability %q SystemDefault=%v, want %v for risk score %d", capability.Key, capability.SystemDefault, wantDefault, policyRiskScore(capability.Key))
		}
	}
}

func TestPolicyCoverageRegistryCoversEveryCapability(t *testing.T) {
	raw, err := os.ReadFile("../testing/test-coverage.json")
	if err != nil {
		t.Fatal(err)
	}
	var registry struct {
		Items []struct {
			ID    string `json:"id"`
			Kind  string `json:"kind"`
			Tests []struct {
				TestName   string `json:"test_name"`
				TestScript string `json:"test_script"`
				Live       bool   `json:"live"`
			} `json:"tests"`
		} `json:"items"`
	}
	if err := json.Unmarshal(raw, &registry); err != nil {
		t.Fatal(err)
	}
	covered := map[string]int{}
	liveCovered := map[string]int{}
	for _, item := range registry.Items {
		if item.Kind == "policy_permission" && strings.HasPrefix(item.ID, "policy.") {
			key := strings.TrimPrefix(item.ID, "policy.")
			covered[key] = len(item.Tests)
			for _, test := range item.Tests {
				if test.Live || strings.HasPrefix(strings.ToLower(test.TestName), "live ") {
					liveCovered[key]++
				}
			}
		}
	}
	for _, capability := range PolicyCatalog() {
		count, ok := covered[capability.Key]
		if !ok {
			t.Fatalf("policy capability %q is missing from testing/test-coverage.json", capability.Key)
		}
		if count == 0 {
			t.Fatalf("policy capability %q has no tests in testing/test-coverage.json", capability.Key)
		}
		if liveCovered[capability.Key] == 0 {
			t.Fatalf("policy capability %q has no live tests in testing/test-coverage.json", capability.Key)
		}
	}
	for key := range covered {
		found := false
		for _, capability := range PolicyCatalog() {
			if capability.Key == key {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("testing/test-coverage.json references unknown policy capability %q", key)
		}
	}
}

func TestPolicyEngineGmailReadPermissions(t *testing.T) {
	ctx := PolicyEvalContext{MailboxEmail: "me@example.com"}

	profile := newTestPolicyEngine(t, []string{"gmail_profile_read"})
	assertPolicyAllowed(t, profile, http.MethodGet, "/gmail/v1/users/me/profile", nil, ctx)
	assertPolicyAllowed(t, profile, http.MethodGet, "/gmail/v1/users/me/history", nil, ctx)
	assertPolicyDenied(t, profile, http.MethodGet, "/gmail/v1/users/me/messages", nil, ctx, "no policy capability")

	messages := newTestPolicyEngine(t, []string{"gmail_messages_read"})
	assertPolicyAllowed(t, messages, http.MethodGet, "/gmail/v1/users/me/messages", nil, ctx)
	assertPolicyAllowed(t, messages, http.MethodGet, "/gmail/v1/users/me/messages/msg123", nil, ctx)
	assertPolicyAllowed(t, messages, http.MethodGet, "/gmail/v1/users/me/messages/msg123/attachments/att123", nil, ctx)
	assertPolicyAllowed(t, messages, http.MethodGet, "/gmail/v1/users/me/threads", nil, ctx)
	assertPolicyAllowed(t, messages, http.MethodGet, "/gmail/v1/users/me/threads/thread123", nil, ctx)
	assertPolicyDenied(t, messages, http.MethodPost, "/gmail/v1/users/me/messages/send", []byte(`{"raw":"abc"}`), ctx, "no policy capability")

	drafts := newTestPolicyEngine(t, []string{"gmail_drafts_read"})
	assertPolicyAllowed(t, drafts, http.MethodGet, "/gmail/v1/users/me/drafts", nil, ctx)
	assertPolicyAllowed(t, drafts, http.MethodGet, "/gmail/v1/users/me/drafts/draft123", nil, ctx)
	assertPolicyDenied(t, drafts, http.MethodPost, "/gmail/v1/users/me/drafts", []byte(`{"message":{}}`), ctx, "no policy capability")

	labels := newTestPolicyEngine(t, []string{"gmail_labels_read"})
	assertPolicyAllowed(t, labels, http.MethodGet, "/gmail/v1/users/me/labels", nil, ctx)
	assertPolicyAllowed(t, labels, http.MethodGet, "/gmail/v1/users/me/labels/Label_123", nil, ctx)
	assertPolicyDenied(t, labels, http.MethodPost, "/gmail/v1/users/me/labels", []byte(`{"name":"Blocked"}`), ctx, "no policy capability")
}

func TestPolicyEngineContactsReadPermissions(t *testing.T) {
	ctx := PolicyEvalContext{MailboxEmail: "me@example.com"}
	contacts := newTestPolicyEngine(t, []string{"contacts_read"})

	assertPolicyAllowed(t, contacts, http.MethodGet, "/v1/people:searchContacts", nil, ctx)
	assertPolicyAllowed(t, contacts, http.MethodGet, "/v1/people:searchDirectoryPeople", nil, ctx)
	assertPolicyAllowed(t, contacts, http.MethodGet, "/v1/otherContacts:search", nil, ctx)
	assertPolicyDenied(t, contacts, http.MethodGet, "/v1/people/me/connections", nil, ctx, "no policy capability")
	assertPolicyDenied(t, contacts, http.MethodPost, "/v1/people:createContact", []byte(`{}`), ctx, "no policy capability")
}

func TestPolicyEngineGmailWritePermissions(t *testing.T) {
	ctx := PolicyEvalContext{MailboxEmail: "me@example.com"}

	drafts := newTestPolicyEngine(t, []string{"gmail_drafts_write"})
	assertPolicyAllowed(t, drafts, http.MethodPost, "/gmail/v1/users/me/drafts", []byte(`{"message":{}}`), ctx)
	assertPolicyAllowed(t, drafts, http.MethodPut, "/gmail/v1/users/me/drafts/draft123", []byte(`{"message":{}}`), ctx)
	assertPolicyDenied(t, drafts, http.MethodDelete, "/gmail/v1/users/me/drafts/draft123", nil, ctx, "no policy capability")
	assertPolicyDenied(t, drafts, http.MethodPost, "/gmail/v1/users/me/drafts/send", []byte(`{"id":"draft123"}`), ctx, "no policy capability")

	deleteDrafts := newTestPolicyEngine(t, []string{"gmail_drafts_delete"})
	assertPolicyAllowed(t, deleteDrafts, http.MethodDelete, "/gmail/v1/users/me/drafts/draft123", nil, ctx)
	assertPolicyDenied(t, deleteDrafts, http.MethodPost, "/gmail/v1/users/me/drafts", []byte(`{"message":{}}`), ctx, "no policy capability")
	assertPolicyDenied(t, deleteDrafts, http.MethodPost, "/gmail/v1/users/me/drafts/send", []byte(`{"id":"draft123"}`), ctx, "no policy capability")

	send := newTestPolicyEngine(t, []string{"gmail_send"})
	assertPolicyAllowed(t, send, http.MethodPost, "/gmail/v1/users/me/messages/send", []byte(`{"raw":"abc"}`), ctx)
	assertPolicyAllowed(t, send, http.MethodPost, "/gmail/v1/users/me/drafts/send", []byte(`{"id":"draft123"}`), ctx)
	assertPolicyDenied(t, send, http.MethodPost, "/gmail/v1/users/me/drafts", []byte(`{"message":{}}`), ctx, "no policy capability")

	customLabels := newTestPolicyEngine(t, []string{"gmail_labels_apply_custom"})
	assertPolicyAllowed(t, customLabels, http.MethodPost, "/gmail/v1/users/me/messages/msg123/modify", []byte(`{"addLabelIds":["Label_123"]}`), ctx)
	assertPolicyAllowed(t, customLabels, http.MethodPost, "/gmail/v1/users/me/messages/batchModify", []byte(`{"ids":["msg123"],"removeLabelIds":["Label_123"]}`), ctx)
	assertPolicyAllowed(t, customLabels, http.MethodPost, "/gmail/v1/users/me/threads/thread123/modify", []byte(`{"addLabelIds":["Label_123"]}`), ctx)
	assertPolicyDenied(t, customLabels, http.MethodPost, "/gmail/v1/users/me/messages/msg123/modify", []byte(`{"removeLabelIds":["INBOX"]}`), ctx, "gmail_messages_archive")

	archive := newTestPolicyEngine(t, []string{"gmail_messages_archive"})
	assertPolicyAllowed(t, archive, http.MethodPost, "/gmail/v1/users/me/messages/msg123/modify", []byte(`{"removeLabelIds":["INBOX"]}`), ctx)
	assertPolicyAllowed(t, archive, http.MethodPost, "/gmail/v1/users/me/threads/thread123/modify", []byte(`{"addLabelIds":["INBOX"]}`), ctx)
	assertPolicyDenied(t, archive, http.MethodPost, "/gmail/v1/users/me/messages/msg123/modify", []byte(`{"addLabelIds":["Label_123"]}`), ctx, "gmail_labels_apply_custom")

	status := newTestPolicyEngine(t, []string{"gmail_messages_status"})
	assertPolicyAllowed(t, status, http.MethodPost, "/gmail/v1/users/me/messages/msg123/modify", []byte(`{"removeLabelIds":["UNREAD"],"addLabelIds":["STARRED","IMPORTANT"]}`), ctx)
	assertPolicyAllowed(t, status, http.MethodPost, "/gmail/v1/users/me/threads/thread123/modify", []byte(`{"addLabelIds":["UNREAD"],"removeLabelIds":["STARRED","IMPORTANT"]}`), ctx)
	assertPolicyDenied(t, status, http.MethodPost, "/gmail/v1/users/me/messages/msg123/modify", []byte(`{"raw":"abc"}`), ctx, "not allowed")

	spam := newTestPolicyEngine(t, []string{"gmail_messages_spam"})
	assertPolicyAllowed(t, spam, http.MethodPost, "/gmail/v1/users/me/messages/msg123/modify", []byte(`{"addLabelIds":["SPAM"]}`), ctx)
	assertPolicyAllowed(t, spam, http.MethodPost, "/gmail/v1/users/me/threads/thread123/modify", []byte(`{"removeLabelIds":["SPAM"]}`), ctx)
	assertPolicyDenied(t, spam, http.MethodPost, "/gmail/v1/users/me/messages/msg123/modify", []byte(`{"addLabelIds":["TRASH"]}`), ctx, "gmail_messages_trash")

	trash := newTestPolicyEngine(t, []string{"gmail_messages_trash"})
	assertPolicyAllowed(t, trash, http.MethodPost, "/gmail/v1/users/me/messages/msg123/trash", nil, ctx)
	assertPolicyAllowed(t, trash, http.MethodPost, "/gmail/v1/users/me/messages/msg123/untrash", nil, ctx)
	assertPolicyAllowed(t, trash, http.MethodPost, "/gmail/v1/users/me/threads/thread123/trash", nil, ctx)
	assertPolicyAllowed(t, trash, http.MethodPost, "/gmail/v1/users/me/threads/thread123/untrash", nil, ctx)
	assertPolicyAllowed(t, trash, http.MethodPost, "/gmail/v1/users/me/messages/msg123/modify", []byte(`{"addLabelIds":["TRASH"]}`), ctx)
	assertPolicyDenied(t, trash, http.MethodDelete, "/gmail/v1/users/me/messages/msg123", nil, ctx, "no policy capability")

	labelDefinitions := newTestPolicyEngine(t, []string{"gmail_labels_create_rename"})
	assertPolicyAllowed(t, labelDefinitions, http.MethodPost, "/gmail/v1/users/me/labels", []byte(`{"name":"Project"}`), ctx)
	assertPolicyAllowed(t, labelDefinitions, http.MethodPatch, "/gmail/v1/users/me/labels/Label_123", []byte(`{"name":"Renamed"}`), ctx)
	assertPolicyAllowed(t, labelDefinitions, http.MethodPut, "/gmail/v1/users/me/labels/Label_123", []byte(`{"name":"Updated"}`), ctx)
	assertPolicyDenied(t, labelDefinitions, http.MethodPatch, "/gmail/v1/users/me/labels/INBOX", []byte(`{"name":"Renamed"}`), ctx, "system label")
	assertPolicyDenied(t, labelDefinitions, http.MethodPost, "/gmail/v1/users/me/labels", []byte(`{"name":"TRASH"}`), ctx, "reserved")
	assertPolicyDenied(t, labelDefinitions, http.MethodDelete, "/gmail/v1/users/me/labels/Label_123", nil, ctx, "no policy capability")

	deleteLabels := newTestPolicyEngine(t, []string{"gmail_labels_delete"})
	assertPolicyAllowed(t, deleteLabels, http.MethodDelete, "/gmail/v1/users/me/labels/Label_123", nil, ctx)
	assertPolicyDenied(t, deleteLabels, http.MethodDelete, "/gmail/v1/users/me/labels/TRASH", nil, ctx, "system label")
	assertPolicyDenied(t, deleteLabels, http.MethodPatch, "/gmail/v1/users/me/labels/Label_123", []byte(`{"name":"Renamed"}`), ctx, "no policy capability")

	mixedLabels := newTestPolicyEngine(t, []string{"gmail_labels_apply_custom", "gmail_messages_archive"})
	assertPolicyAllowed(t, mixedLabels, http.MethodPost, "/gmail/v1/users/me/messages/msg123/modify", []byte(`{"addLabelIds":["Label_123"],"removeLabelIds":["INBOX"]}`), ctx)
	assertPolicyDenied(t, mixedLabels, http.MethodPost, "/gmail/v1/users/me/messages/msg123/modify", []byte(`{"addLabelIds":["Label_123","SPAM"],"removeLabelIds":["INBOX"]}`), ctx, "gmail_messages_spam")

	deleteMessages := newTestPolicyEngine(t, []string{"gmail_messages_delete"})
	assertPolicyAllowed(t, deleteMessages, http.MethodDelete, "/gmail/v1/users/me/messages/msg123", nil, ctx)
	assertPolicyAllowed(t, deleteMessages, http.MethodPost, "/gmail/v1/users/me/messages/batchDelete", []byte(`{"ids":["msg123"]}`), ctx)
	assertPolicyDenied(t, deleteMessages, http.MethodPost, "/gmail/v1/users/me/messages/msg123/trash", nil, ctx, "no policy capability")
	assertPolicyDenied(t, deleteMessages, http.MethodGet, "/gmail/v1/users/me/messages/msg123", nil, ctx, "no policy capability")

	importMessages := newTestPolicyEngine(t, []string{"gmail_messages_import"})
	assertPolicyAllowed(t, importMessages, http.MethodPost, "/gmail/v1/users/me/messages/import", []byte(`{"raw":"abc"}`), ctx)
	assertPolicyAllowed(t, importMessages, http.MethodPost, "/gmail/v1/users/me/messages", []byte(`{"raw":"abc"}`), ctx)
	assertPolicyDenied(t, importMessages, http.MethodPost, "/gmail/v1/users/me/messages/send", []byte(`{"raw":"abc"}`), ctx, "no policy capability")

	watch := newTestPolicyEngine(t, []string{"gmail_watch_manage"})
	assertPolicyAllowed(t, watch, http.MethodPost, "/gmail/v1/users/me/watch", []byte(`{"topicName":"projects/test/topics/gmail"}`), ctx)
	assertPolicyAllowed(t, watch, http.MethodPost, "/gmail/v1/users/me/stop", nil, ctx)
	assertPolicyDenied(t, watch, http.MethodGet, "/gmail/v1/users/me/history", nil, ctx, "no policy capability")
}

func TestPolicyEngineCalendarRead(t *testing.T) {
	engine := newTestPolicyEngine(t, []string{"calendar_read"})
	ctx := PolicyEvalContext{MailboxEmail: "me@example.com"}
	assertPolicyAllowed(t, engine, http.MethodGet, "/calendar/v3/users/me/calendarList", nil, ctx)
	assertPolicyAllowed(t, engine, http.MethodGet, "/calendar/v3/users/me/calendarList/primary", nil, ctx)
	assertPolicyAllowed(t, engine, http.MethodGet, "/calendar/v3/calendars/primary", nil, ctx)
	assertPolicyAllowed(t, engine, http.MethodGet, "/calendar/v3/calendars/primary/events", nil, ctx)
	assertPolicyAllowed(t, engine, http.MethodGet, "/calendar/v3/calendars/primary/events/event123", nil, ctx)
	assertPolicyAllowed(t, engine, http.MethodGet, "/calendar/v3/calendars/primary/events/event123/instances", nil, ctx)
	assertPolicyAllowed(t, engine, http.MethodPost, "/calendar/v3/freeBusy", []byte(`{"items":[{"id":"primary"}]}`), ctx)
	assertPolicyDenied(t, engine, http.MethodPost, "/calendar/v3/calendars/primary/events", []byte(`{"summary":"Blocked"}`), ctx, "no policy capability")
}

func TestPolicyEngineCalendarEventCreateSplit(t *testing.T) {
	ctx := PolicyEvalContext{MailboxEmail: "me@example.com"}
	bodySelf := []byte(`{"summary":"Focus time"}`)
	bodyGuests := []byte(`{"summary":"Review","attendees":[{"email":"guest@example.com"}]}`)

	selfOnly := newTestPolicyEngine(t, []string{"calendar_events_create_self"})
	assertPolicyAllowed(t, selfOnly, http.MethodPost, "/calendar/v3/calendars/primary/events", bodySelf, ctx)
	assertPolicyDenied(t, selfOnly, http.MethodPost, "/calendar/v3/calendars/primary/events", bodyGuests, ctx, "third parties")

	withGuests := newTestPolicyEngine(t, []string{"calendar_events_create_guests"})
	assertPolicyAllowed(t, withGuests, http.MethodPost, "/calendar/v3/calendars/primary/events", bodyGuests, ctx)
	assertPolicyDenied(t, withGuests, http.MethodPost, "/calendar/v3/calendars/primary/events", bodySelf, ctx, "myself-only")
}

func TestPolicyEngineCalendarEventUpdateSplit(t *testing.T) {
	ctxSelf := PolicyEvalContext{
		MailboxEmail: "me@example.com",
		FetchCalendarEvent: func(calendarID, eventID string) (map[string]any, error) {
			return map[string]any{"summary": "Existing focus time"}, nil
		},
	}
	ctxGuests := PolicyEvalContext{
		MailboxEmail: "me@example.com",
		FetchCalendarEvent: func(calendarID, eventID string) (map[string]any, error) {
			return map[string]any{
				"summary":   "Existing review",
				"attendees": []any{map[string]any{"email": "guest@example.com"}},
			}, nil
		},
	}
	bodySelf := []byte(`{"summary":"Updated focus time"}`)
	bodyGuests := []byte(`{"attendees":[{"email":"guest@example.com"}]}`)
	path := "/calendar/v3/calendars/primary/events/event123"

	selfOnly := newTestPolicyEngine(t, []string{"calendar_events_update_self"})
	assertPolicyAllowed(t, selfOnly, http.MethodPatch, path, bodySelf, ctxSelf)
	assertPolicyDenied(t, selfOnly, http.MethodPatch, path, bodyGuests, ctxSelf, "third parties")
	assertPolicyDenied(t, selfOnly, http.MethodPatch, path, bodySelf, ctxGuests, "third parties")

	withGuests := newTestPolicyEngine(t, []string{"calendar_events_update_guests"})
	assertPolicyAllowed(t, withGuests, http.MethodPatch, path, bodyGuests, ctxSelf)
	assertPolicyAllowed(t, withGuests, http.MethodPatch, path, bodySelf, ctxGuests)
	assertPolicyAllowed(t, withGuests, http.MethodPost, path+"/move", nil, PolicyEvalContext{MailboxEmail: "me@example.com"})
}

func TestPolicyEngineCalendarEventDeleteSplit(t *testing.T) {
	path := "/calendar/v3/calendars/primary/events/event123"
	ctxSelf := PolicyEvalContext{
		MailboxEmail: "me@example.com",
		FetchCalendarEvent: func(calendarID, eventID string) (map[string]any, error) {
			return map[string]any{"summary": "Solo event"}, nil
		},
	}
	ctxGuests := PolicyEvalContext{
		MailboxEmail: "me@example.com",
		FetchCalendarEvent: func(calendarID, eventID string) (map[string]any, error) {
			return map[string]any{
				"summary":   "Guest event",
				"attendees": []any{map[string]any{"email": "guest@example.com"}},
			}, nil
		},
	}

	selfOnly := newTestPolicyEngine(t, []string{"calendar_events_delete_self"})
	assertPolicyAllowed(t, selfOnly, http.MethodDelete, path, nil, ctxSelf)
	assertPolicyDenied(t, selfOnly, http.MethodDelete, path, nil, ctxGuests, "third parties")

	withGuests := newTestPolicyEngine(t, []string{"calendar_events_delete_guests"})
	assertPolicyAllowed(t, withGuests, http.MethodDelete, path, nil, ctxGuests)
	assertPolicyDenied(t, withGuests, http.MethodDelete, path, nil, ctxSelf, "myself-only")
}

func TestPolicyEngineCalendarOperations(t *testing.T) {
	ctx := PolicyEvalContext{MailboxEmail: "me@example.com"}

	accessRead := newTestPolicyEngine(t, []string{"calendar_access_read"})
	assertPolicyAllowed(t, accessRead, http.MethodGet, "/calendar/v3/calendars/primary/acl", nil, ctx)
	assertPolicyAllowed(t, accessRead, http.MethodGet, "/calendar/v3/calendars/primary/acl/user:friend@example.com", nil, ctx)
	assertPolicyDenied(t, accessRead, http.MethodPost, "/calendar/v3/calendars/primary/acl", []byte(`{"role":"reader"}`), ctx, "no policy capability")

	settingsRead := newTestPolicyEngine(t, []string{"calendar_settings_read"})
	assertPolicyAllowed(t, settingsRead, http.MethodGet, "/calendar/v3/users/me/settings", nil, ctx)
	assertPolicyAllowed(t, settingsRead, http.MethodGet, "/calendar/v3/users/me/settings/timezone", nil, ctx)
	assertPolicyDenied(t, settingsRead, http.MethodPost, "/calendar/v3/calendars", []byte(`{"summary":"Blocked"}`), ctx, "no policy capability")

	accessManage := newTestPolicyEngine(t, []string{"calendar_access_manage"})
	assertPolicyAllowed(t, accessManage, http.MethodPost, "/calendar/v3/calendars/primary/acl", []byte(`{"role":"reader"}`), ctx)
	assertPolicyAllowed(t, accessManage, http.MethodPatch, "/calendar/v3/calendars/primary/acl/user:friend@example.com", []byte(`{"role":"freeBusyReader"}`), ctx)
	assertPolicyAllowed(t, accessManage, http.MethodPut, "/calendar/v3/calendars/primary/acl/user:friend@example.com", []byte(`{"role":"reader"}`), ctx)
	assertPolicyAllowed(t, accessManage, http.MethodDelete, "/calendar/v3/calendars/primary/acl/user:friend@example.com", nil, ctx)
	assertPolicyAllowed(t, accessManage, http.MethodPost, "/calendar/v3/calendars/primary/acl/watch", []byte(`{"id":"channel","type":"web_hook"}`), ctx)
	assertPolicyDenied(t, accessManage, http.MethodGet, "/calendar/v3/calendars/primary/acl", nil, ctx, "no policy capability")

	calendarsManage := newTestPolicyEngine(t, []string{"calendar_calendars_manage"})
	assertPolicyAllowed(t, calendarsManage, http.MethodPost, "/calendar/v3/calendars", []byte(`{"summary":"Test"}`), ctx)
	assertPolicyAllowed(t, calendarsManage, http.MethodPatch, "/calendar/v3/calendars/test-calendar", []byte(`{"summary":"Patch"}`), ctx)
	assertPolicyAllowed(t, calendarsManage, http.MethodPut, "/calendar/v3/calendars/test-calendar", []byte(`{"summary":"Put"}`), ctx)
	assertPolicyAllowed(t, calendarsManage, http.MethodPost, "/calendar/v3/calendars/test-calendar/clear", nil, ctx)
	assertPolicyAllowed(t, calendarsManage, http.MethodDelete, "/calendar/v3/calendars/test-calendar", nil, ctx)
	assertPolicyAllowed(t, calendarsManage, http.MethodPost, "/calendar/v3/users/me/calendarList", []byte(`{"id":"test-calendar"}`), ctx)
	assertPolicyAllowed(t, calendarsManage, http.MethodPatch, "/calendar/v3/users/me/calendarList/test-calendar", []byte(`{"summaryOverride":"Patch"}`), ctx)
	assertPolicyAllowed(t, calendarsManage, http.MethodPut, "/calendar/v3/users/me/calendarList/test-calendar", []byte(`{"id":"test-calendar"}`), ctx)
	assertPolicyAllowed(t, calendarsManage, http.MethodDelete, "/calendar/v3/users/me/calendarList/test-calendar", nil, ctx)
	assertPolicyDenied(t, calendarsManage, http.MethodGet, "/calendar/v3/users/me/calendarList", nil, ctx, "no policy capability")
}

func TestPolicyEngineDriveDocsOperations(t *testing.T) {
	ctx := driveKindCtx("docs")

	docsRead := newTestPolicyEngine(t, []string{"docs_read"})
	assertPolicyAllowed(t, docsRead, http.MethodGet, "/v1/documents/doc123", nil, ctx)
	assertPolicyAllowed(t, docsRead, http.MethodGet, "/drive/v3/files/doc123/export", nil, ctx)
	assertPolicyDenied(t, docsRead, http.MethodPost, "/v1/documents", []byte(`{"title":"Blocked"}`), ctx, "no policy capability")
	assertPolicyDenied(t, docsRead, http.MethodPost, "/v1/documents/doc123:batchUpdate", []byte(`{"requests":[]}`), ctx, "no policy capability")
	assertPolicyDenied(t, docsRead, http.MethodDelete, "/drive/v3/files/doc123", nil, ctx, "no policy capability")

	docsCreate := newTestPolicyEngine(t, []string{"docs_create"})
	assertPolicyAllowed(t, docsCreate, http.MethodPost, "/v1/documents", []byte(`{"title":"Created"}`), ctx)
	assertPolicyAllowed(t, docsCreate, http.MethodPost, "/drive/v3/files", []byte(`{"name":"Created","mimeType":"application/vnd.google-apps.document"}`), ctx)
	assertPolicyDenied(t, docsCreate, http.MethodGet, "/v1/documents/doc123", nil, ctx, "no policy capability")
	assertPolicyDenied(t, docsCreate, http.MethodPost, "/v1/documents/doc123:batchUpdate", []byte(`{"requests":[]}`), ctx, "no policy capability")
	assertPolicyDenied(t, docsCreate, http.MethodPost, "/drive/v3/files", []byte(`{"name":"Blocked","mimeType":"application/vnd.google-apps.spreadsheet"}`), ctx, "Google Sheets")

	docsEdit := newTestPolicyEngine(t, []string{"docs_edit"})
	assertPolicyAllowed(t, docsEdit, http.MethodPost, "/v1/documents/doc123:batchUpdate", []byte(`{"requests":[]}`), ctx)
	assertPolicyDenied(t, docsEdit, http.MethodGet, "/v1/documents/doc123", nil, ctx, "no policy capability")
	assertPolicyDenied(t, docsEdit, http.MethodPost, "/v1/documents", []byte(`{"title":"Blocked"}`), ctx, "no policy capability")

	docsDelete := newTestPolicyEngine(t, []string{"docs_delete"})
	assertPolicyAllowed(t, docsDelete, http.MethodDelete, "/drive/v3/files/doc123", nil, ctx)
	assertPolicyDenied(t, docsDelete, http.MethodDelete, "/drive/v3/files/sheet123", nil, driveKindCtx("sheets"), "Google Sheets")
}

func TestPolicyEngineDriveFilesRead(t *testing.T) {
	ctx := driveKindCtx("drive")
	engine := newTestPolicyEngine(t, []string{"drive_files_read"})

	assertPolicyAllowed(t, engine, http.MethodGet, "/drive/v3/files/file123", nil, ctx)

	assertPolicyDenied(t, engine, http.MethodGet, "/drive/v3/files", nil, ctx, "no policy capability")
	assertPolicyDenied(t, engine, http.MethodGet, "/drive/v3/about", nil, ctx, "no policy capability")
	assertPolicyDenied(t, engine, http.MethodGet, "/drive/v3/files/doc123", nil, driveKindCtx("docs"), "Google Docs")
	assertPolicyDenied(t, engine, http.MethodGet, "/drive/v3/files/doc123/export", nil, driveKindCtx("docs"), "no policy capability")
	assertPolicyDenied(t, engine, http.MethodPost, "/drive/v3/files", []byte(`{"name":"Blocked"}`), ctx, "no policy capability")
	assertPolicyDenied(t, engine, http.MethodPost, "/drive/v3/files/file123/copy", []byte(`{"name":"Blocked copy"}`), ctx, "no policy capability")
	assertPolicyDenied(t, engine, http.MethodPatch, "/drive/v3/files/file123", []byte(`{"name":"Blocked rename"}`), ctx, "no policy capability")
	assertPolicyDenied(t, engine, http.MethodPut, "/drive/v3/files/file123", []byte(`{"name":"Blocked update"}`), ctx, "no policy capability")
	assertPolicyDenied(t, engine, http.MethodPatch, "/upload/drive/v3/files/file123", []byte(`blocked content`), ctx, "no policy capability")
	assertPolicyDenied(t, engine, http.MethodDelete, "/drive/v3/files/file123", nil, ctx, "no policy capability")
	assertPolicyDenied(t, engine, http.MethodDelete, "/drive/v3/files/trash", nil, ctx, "no policy capability")
}

func TestPolicyEngineDriveFilesWriteUsesNonGoogleNativeFiles(t *testing.T) {
	driveCtx := driveKindCtx("drive")
	docsCtx := driveKindCtx("docs")

	create := newTestPolicyEngine(t, []string{"drive_files_create"})
	assertPolicyAllowed(t, create, http.MethodPost, "/drive/v3/files", []byte(`{"name":"file.txt","mimeType":"text/plain"}`), driveCtx)
	assertPolicyDenied(t, create, http.MethodPost, "/drive/v3/files", []byte(`{"name":"Doc","mimeType":"application/vnd.google-apps.document"}`), driveCtx, "Google Docs")
	assertPolicyAllowed(t, create, http.MethodPost, "/drive/v3/files/file123/copy", []byte(`{"name":"Copy"}`), driveCtx)
	assertPolicyDenied(t, create, http.MethodPost, "/drive/v3/files/doc123/copy", []byte(`{"name":"Copy"}`), docsCtx, "Google Docs")

	update := newTestPolicyEngine(t, []string{"drive_files_update"})
	assertPolicyAllowed(t, update, http.MethodPatch, "/drive/v3/files/file123", []byte(`{"name":"Updated"}`), driveCtx)
	assertPolicyAllowed(t, update, http.MethodPut, "/drive/v3/files/file123", []byte(`{"name":"Updated"}`), driveCtx)
	assertPolicyAllowed(t, update, http.MethodPatch, "/upload/drive/v3/files/file123", []byte(`updated content`), driveCtx)
	assertPolicyDenied(t, update, http.MethodPatch, "/drive/v3/files/doc123", []byte(`{"name":"Updated"}`), docsCtx, "Google Docs")
	assertPolicyDenied(t, update, http.MethodPatch, "/upload/drive/v3/files/doc123", []byte(`updated content`), docsCtx, "Google Docs")

	deleteFile := newTestPolicyEngine(t, []string{"drive_files_delete"})
	assertPolicyAllowed(t, deleteFile, http.MethodDelete, "/drive/v3/files/file123", nil, driveCtx)
	assertPolicyDenied(t, deleteFile, http.MethodDelete, "/drive/v3/files/doc123", nil, docsCtx, "Google Docs")
	assertPolicyDenied(t, deleteFile, http.MethodDelete, "/drive/v3/files/trash", nil, driveCtx, "emptying Drive trash")
}

func TestPolicyEngineDriveSharingPermissions(t *testing.T) {
	ctx := PolicyEvalContext{MailboxEmail: "me@example.com"}

	read := newTestPolicyEngine(t, []string{"drive_permissions_read"})
	assertPolicyAllowed(t, read, http.MethodGet, "/drive/v3/files/file123/permissions", nil, ctx)
	assertPolicyAllowed(t, read, http.MethodGet, "/drive/v3/files/file123/permissions/permission123", nil, ctx)
	assertPolicyDenied(t, read, http.MethodPost, "/drive/v3/files/file123/permissions", []byte(`{"role":"reader","type":"user"}`), ctx, "no policy capability")
	assertPolicyDenied(t, read, http.MethodPatch, "/drive/v3/files/file123/permissions/permission123", []byte(`{"role":"writer"}`), ctx, "no policy capability")

	manage := newTestPolicyEngine(t, []string{"drive_permissions_manage"})
	assertPolicyAllowed(t, manage, http.MethodPost, "/drive/v3/files/file123/permissions", []byte(`{"role":"reader","type":"user"}`), ctx)
	assertPolicyAllowed(t, manage, http.MethodPatch, "/drive/v3/files/file123/permissions/permission123", []byte(`{"role":"commenter"}`), ctx)
	assertPolicyAllowed(t, manage, http.MethodPut, "/drive/v3/files/file123/permissions/permission123", []byte(`{"role":"writer","type":"user"}`), ctx)
	assertPolicyAllowed(t, manage, http.MethodDelete, "/drive/v3/files/file123/permissions/permission123", nil, ctx)
	assertPolicyDenied(t, manage, http.MethodGet, "/drive/v3/files/file123/permissions", nil, ctx, "no policy capability")
}

func TestPolicyEngineStructuredFileNativeDriveRoutes(t *testing.T) {
	sheetsRead := newTestPolicyEngine(t, []string{"sheets_read"})
	assertPolicyAllowed(t, sheetsRead, http.MethodGet, "/drive/v3/files/sheet123/export", nil, driveKindCtx("sheets"))
	assertPolicyDenied(t, sheetsRead, http.MethodGet, "/drive/v3/files/doc123/export", nil, driveKindCtx("docs"), "Google Docs")

	sheetsDelete := newTestPolicyEngine(t, []string{"sheets_delete"})
	assertPolicyAllowed(t, sheetsDelete, http.MethodDelete, "/drive/v3/files/sheet123", nil, driveKindCtx("sheets"))
	assertPolicyDenied(t, sheetsDelete, http.MethodDelete, "/drive/v3/files/slide123", nil, driveKindCtx("slides"), "Google Slides")

	slidesRead := newTestPolicyEngine(t, []string{"slides_read"})
	assertPolicyAllowed(t, slidesRead, http.MethodGet, "/drive/v3/files/slide123/export", nil, driveKindCtx("slides"))
	assertPolicyDenied(t, slidesRead, http.MethodGet, "/drive/v3/files/sheet123/export", nil, driveKindCtx("sheets"), "Google Sheets")

	slidesDelete := newTestPolicyEngine(t, []string{"slides_delete"})
	assertPolicyAllowed(t, slidesDelete, http.MethodDelete, "/drive/v3/files/slide123", nil, driveKindCtx("slides"))
	assertPolicyDenied(t, slidesDelete, http.MethodDelete, "/drive/v3/files/file123", nil, driveKindCtx("drive"), "non-Google-native files")
}

func TestPolicyEngineSheetsCreateAndEdit(t *testing.T) {
	ctx := driveKindCtx("sheets")

	create := newTestPolicyEngine(t, []string{"sheets_create"})
	assertPolicyAllowed(t, create, http.MethodPost, "/v4/spreadsheets", []byte(`{"properties":{"title":"Created"}}`), ctx)
	assertPolicyAllowed(t, create, http.MethodPost, "/drive/v3/files", []byte(`{"name":"Created","mimeType":"application/vnd.google-apps.spreadsheet"}`), ctx)
	assertPolicyDenied(t, create, http.MethodGet, "/v4/spreadsheets/sheet123", nil, ctx, "no policy capability")
	assertPolicyDenied(t, create, http.MethodPost, "/v4/spreadsheets/sheet123:batchUpdate", []byte(`{"requests":[]}`), ctx, "no policy capability")
	assertPolicyDenied(t, create, http.MethodPost, "/drive/v3/files", []byte(`{"name":"Blocked","mimeType":"application/vnd.google-apps.presentation"}`), ctx, "Google Slides")

	edit := newTestPolicyEngine(t, []string{"sheets_edit"})
	assertPolicyAllowed(t, edit, http.MethodPost, "/v4/spreadsheets/sheet123:batchUpdate", []byte(`{"requests":[]}`), ctx)
	assertPolicyAllowed(t, edit, http.MethodPut, "/v4/spreadsheets/sheet123/values/Sheet1!A1", []byte(`{"values":[["x"]]}`), ctx)
	assertPolicyAllowed(t, edit, http.MethodPost, "/v4/spreadsheets/sheet123/values/Sheet1!A1:append", []byte(`{"values":[["x"]]}`), ctx)
	assertPolicyAllowed(t, edit, http.MethodPost, "/v4/spreadsheets/sheet123/values:batchUpdate", []byte(`{"data":[]}`), ctx)
	assertPolicyAllowed(t, edit, http.MethodPost, "/v4/spreadsheets/sheet123/values:batchUpdateByDataFilter", []byte(`{"data":[]}`), ctx)
	assertPolicyAllowed(t, edit, http.MethodPost, "/v4/spreadsheets/sheet123/values/Sheet1!A1:clear", []byte(`{}`), ctx)
	assertPolicyAllowed(t, edit, http.MethodPost, "/v4/spreadsheets/sheet123/values:batchClear", []byte(`{"ranges":["Sheet1!A1"]}`), ctx)
	assertPolicyAllowed(t, edit, http.MethodPost, "/v4/spreadsheets/sheet123/values:batchClearByDataFilter", []byte(`{"dataFilters":[]}`), ctx)
	assertPolicyAllowed(t, edit, http.MethodPost, "/v4/spreadsheets/sheet123/sheets/0:copyTo", []byte(`{"destinationSpreadsheetId":"target"}`), ctx)
	assertPolicyDenied(t, edit, http.MethodGet, "/v4/spreadsheets/sheet123", nil, ctx, "no policy capability")
	assertPolicyDenied(t, edit, http.MethodPost, "/v4/spreadsheets", []byte(`{"properties":{"title":"Blocked"}}`), ctx, "no policy capability")
}

func TestPolicyEngineSlidesCreateAndEdit(t *testing.T) {
	ctx := driveKindCtx("slides")

	create := newTestPolicyEngine(t, []string{"slides_create"})
	assertPolicyAllowed(t, create, http.MethodPost, "/v1/presentations", []byte(`{"title":"Created"}`), ctx)
	assertPolicyAllowed(t, create, http.MethodPost, "/drive/v3/files", []byte(`{"name":"Created","mimeType":"application/vnd.google-apps.presentation"}`), ctx)
	assertPolicyDenied(t, create, http.MethodGet, "/v1/presentations/pres123", nil, ctx, "no policy capability")
	assertPolicyDenied(t, create, http.MethodPost, "/v1/presentations/pres123:batchUpdate", []byte(`{"requests":[]}`), ctx, "no policy capability")
	assertPolicyDenied(t, create, http.MethodPost, "/drive/v3/files", []byte(`{"name":"Blocked","mimeType":"application/vnd.google-apps.document"}`), ctx, "Google Docs")

	edit := newTestPolicyEngine(t, []string{"slides_edit"})
	assertPolicyAllowed(t, edit, http.MethodPost, "/v1/presentations/pres123:batchUpdate", []byte(`{"requests":[]}`), ctx)
	assertPolicyDenied(t, edit, http.MethodGet, "/v1/presentations/pres123", nil, ctx, "no policy capability")
	assertPolicyDenied(t, edit, http.MethodPost, "/v1/presentations", []byte(`{"title":"Blocked"}`), ctx, "no policy capability")
}

func TestPolicyEngineDriveCommentsRead(t *testing.T) {
	ctx := PolicyEvalContext{MailboxEmail: "me@example.com"}
	engine := newTestPolicyEngine(t, []string{"drive_comments_read"})

	assertPolicyAllowed(t, engine, http.MethodGet, "/drive/v3/files/file123/comments", nil, ctx)
	assertPolicyAllowed(t, engine, http.MethodGet, "/drive/v3/files/file123/comments/comment123", nil, ctx)
	assertPolicyAllowed(t, engine, http.MethodGet, "/drive/v3/files/file123/comments/comment123/replies", nil, ctx)
	assertPolicyAllowed(t, engine, http.MethodGet, "/drive/v3/files/file123/comments/comment123/replies/reply123", nil, ctx)

	assertPolicyDenied(t, engine, http.MethodPost, "/drive/v3/files/file123/comments", []byte(`{"content":"Blocked"}`), ctx, "no policy capability")
	assertPolicyDenied(t, engine, http.MethodPatch, "/drive/v3/files/file123/comments/comment123", []byte(`{"content":"Blocked"}`), ctx, "no policy capability")
	assertPolicyDenied(t, engine, http.MethodPut, "/drive/v3/files/file123/comments/comment123", []byte(`{"content":"Blocked"}`), ctx, "no policy capability")
	assertPolicyDenied(t, engine, http.MethodDelete, "/drive/v3/files/file123/comments/comment123", nil, ctx, "no policy capability")
	assertPolicyDenied(t, engine, http.MethodPost, "/drive/v3/files/file123/comments/comment123/replies", []byte(`{"content":"Blocked"}`), ctx, "no policy capability")
	assertPolicyDenied(t, engine, http.MethodPatch, "/drive/v3/files/file123/comments/comment123/replies/reply123", []byte(`{"content":"Blocked"}`), ctx, "no policy capability")
	assertPolicyDenied(t, engine, http.MethodPut, "/drive/v3/files/file123/comments/comment123/replies/reply123", []byte(`{"content":"Blocked"}`), ctx, "no policy capability")
	assertPolicyDenied(t, engine, http.MethodDelete, "/drive/v3/files/file123/comments/comment123/replies/reply123", nil, ctx, "no policy capability")
}

func TestPolicyEngineDriveCommentsWriteSplit(t *testing.T) {
	ctxOwned := PolicyEvalContext{
		MailboxEmail: "me@example.com",
		FetchDriveCommentAuthorMe: func(fileID, commentID, replyID string) (bool, error) {
			return true, nil
		},
	}
	ctxNotOwned := PolicyEvalContext{
		MailboxEmail: "me@example.com",
		FetchDriveCommentAuthorMe: func(fileID, commentID, replyID string) (bool, error) {
			return false, nil
		},
	}
	commentPath := "/drive/v3/files/file123/comments/comment123"
	replyPath := commentPath + "/replies/reply123"
	repliesPath := commentPath + "/replies"

	create := newTestPolicyEngine(t, []string{"drive_comments_create"})
	assertPolicyAllowed(t, create, http.MethodPost, "/drive/v3/files/file123/comments", []byte(`{"content":"New comment"}`), ctxOwned)
	assertPolicyDenied(t, create, http.MethodPost, repliesPath, []byte(`{"content":"Blocked reply"}`), ctxOwned, "no policy capability")

	reply := newTestPolicyEngine(t, []string{"drive_comments_reply"})
	assertPolicyAllowed(t, reply, http.MethodPost, repliesPath, []byte(`{"content":"New reply"}`), ctxOwned)
	assertPolicyDenied(t, reply, http.MethodPost, repliesPath, []byte(`{"action":"resolve","content":"Resolving"}`), ctxOwned, "resolve")
	assertPolicyDenied(t, reply, http.MethodPost, "/drive/v3/files/file123/comments", []byte(`{"content":"Blocked comment"}`), ctxOwned, "no policy capability")

	resolve := newTestPolicyEngine(t, []string{"drive_comments_resolve"})
	assertPolicyAllowed(t, resolve, http.MethodPost, repliesPath, []byte(`{"action":"resolve","content":"Done"}`), ctxOwned)
	assertPolicyDenied(t, resolve, http.MethodPost, repliesPath, []byte(`{"content":"Blocked normal reply"}`), ctxOwned, "reply")

	update := newTestPolicyEngine(t, []string{"drive_comments_update"})
	assertPolicyAllowed(t, update, http.MethodPatch, commentPath, []byte(`{"content":"Updated comment"}`), ctxOwned)
	assertPolicyAllowed(t, update, http.MethodPut, commentPath, []byte(`{"content":"Updated comment"}`), ctxOwned)
	assertPolicyAllowed(t, update, http.MethodPatch, replyPath, []byte(`{"content":"Updated reply"}`), ctxOwned)
	assertPolicyAllowed(t, update, http.MethodPut, replyPath, []byte(`{"content":"Updated reply"}`), ctxOwned)
	assertPolicyDenied(t, update, http.MethodPatch, commentPath, []byte(`{"resolved":true}`), ctxOwned, "resolve")
	assertPolicyDenied(t, update, http.MethodPatch, replyPath, []byte(`{"action":"resolve"}`), ctxOwned, "reply actions")
	assertPolicyDenied(t, update, http.MethodPatch, commentPath, []byte(`{"content":"Blocked"}`), ctxNotOwned, "not authored")
	assertPolicyDenied(t, update, http.MethodPost, repliesPath, []byte(`{"content":"Blocked reply"}`), ctxOwned, "no policy capability")

	deleteOwn := newTestPolicyEngine(t, []string{"drive_comments_delete"})
	assertPolicyAllowed(t, deleteOwn, http.MethodDelete, commentPath, nil, ctxOwned)
	assertPolicyAllowed(t, deleteOwn, http.MethodDelete, replyPath, nil, ctxOwned)
	assertPolicyDenied(t, deleteOwn, http.MethodDelete, commentPath, nil, ctxNotOwned, "not authored")
	assertPolicyDenied(t, deleteOwn, http.MethodPatch, commentPath, []byte(`{"content":"Blocked"}`), ctxOwned, "no policy capability")
}

func TestPolicyEngineDriveMetadataRead(t *testing.T) {
	ctx := PolicyEvalContext{MailboxEmail: "me@example.com"}
	engine := newTestPolicyEngine(t, []string{"drive_metadata_read"})

	assertPolicyAllowed(t, engine, http.MethodGet, "/drive/v3/changes/startPageToken", nil, ctx)
	assertPolicyAllowed(t, engine, http.MethodGet, "/drive/v3/changes", nil, ctx)
	assertPolicyAllowed(t, engine, http.MethodGet, "/drive/v3/files", nil, ctx)
	assertPolicyAllowed(t, engine, http.MethodGet, "/drive/v3/about", nil, ctx)
	assertPolicyAllowed(t, engine, http.MethodGet, "/drive/v3/drives", nil, ctx)
	assertPolicyAllowed(t, engine, http.MethodGet, "/drive/v3/drives/drive123", nil, ctx)
	assertPolicyAllowed(t, engine, http.MethodGet, "/drive/v3/apps", nil, ctx)
	assertPolicyAllowed(t, engine, http.MethodGet, "/drive/v3/apps/app123", nil, ctx)
	assertPolicyAllowed(t, engine, http.MethodGet, "/drive/v3/files/file123/listLabels", nil, ctx)
	assertPolicyAllowed(t, engine, http.MethodGet, "/drive/v3/files/file123/revisions", nil, ctx)
	assertPolicyAllowed(t, engine, http.MethodGet, "/drive/v3/files/file123/revisions/rev123", nil, ctx)

	assertPolicyDenied(t, engine, http.MethodPost, "/drive/v3/changes/startPageToken", nil, ctx, "no policy capability")
	assertPolicyDenied(t, engine, http.MethodPost, "/drive/v3/changes", nil, ctx, "no policy capability")
	assertPolicyDenied(t, engine, http.MethodGet, "/drive/v3/files/file123", nil, ctx, "no policy capability")
	assertPolicyDenied(t, engine, http.MethodGet, "/drive/v3/files/file123/export", nil, ctx, "no policy capability")
	assertPolicyDenied(t, engine, http.MethodPost, "/drive/v3/drives/drive123/hide", nil, ctx, "no policy capability")
	assertPolicyDenied(t, engine, http.MethodPost, "/drive/v3/drives/drive123/unhide", nil, ctx, "no policy capability")
	assertPolicyDenied(t, engine, http.MethodPost, "/drive/v3/apps", nil, ctx, "no policy capability")
	assertPolicyDenied(t, engine, http.MethodPost, "/drive/v3/apps/app123", nil, ctx, "no policy capability")
	assertPolicyDenied(t, engine, http.MethodPost, "/drive/v3/files/file123/listLabels", nil, ctx, "no policy capability")
	assertPolicyDenied(t, engine, http.MethodPost, "/drive/v3/files/file123/modifyLabels", []byte(`{"labelModifications":[]}`), ctx, "no policy capability")
	assertPolicyDenied(t, engine, http.MethodPatch, "/drive/v3/files/file123/revisions/rev123", []byte(`{"keepForever":true}`), ctx, "no policy capability")
	assertPolicyDenied(t, engine, http.MethodDelete, "/drive/v3/files/file123/revisions/rev123", nil, ctx, "no policy capability")
}

func TestPolicyEngineDriveMetadataWriteSplit(t *testing.T) {
	ctx := PolicyEvalContext{MailboxEmail: "me@example.com"}

	labels := newTestPolicyEngine(t, []string{"drive_labels_update"})
	assertPolicyAllowed(t, labels, http.MethodPost, "/drive/v3/files/file123/modifyLabels", []byte(`{"labelModifications":[]}`), ctx)
	assertPolicyDenied(t, labels, http.MethodPost, "/drive/v3/drives/drive123/hide", nil, ctx, "no policy capability")
	assertPolicyDenied(t, labels, http.MethodPost, "/drive/v3/drives/drive123/unhide", nil, ctx, "no policy capability")
	assertPolicyDenied(t, labels, http.MethodPatch, "/drive/v3/files/file123/revisions/rev123", []byte(`{"keepForever":true}`), ctx, "no policy capability")
	assertPolicyDenied(t, labels, http.MethodDelete, "/drive/v3/files/file123/revisions/rev123", nil, ctx, "no policy capability")

	revisions := newTestPolicyEngine(t, []string{"drive_revisions_delete"})
	assertPolicyAllowed(t, revisions, http.MethodDelete, "/drive/v3/files/file123/revisions/rev123", nil, ctx)
	assertPolicyDenied(t, revisions, http.MethodPost, "/drive/v3/files/file123/modifyLabels", []byte(`{"labelModifications":[]}`), ctx, "no policy capability")
	assertPolicyDenied(t, revisions, http.MethodPost, "/drive/v3/drives/drive123/hide", nil, ctx, "no policy capability")
	assertPolicyDenied(t, revisions, http.MethodPost, "/drive/v3/drives/drive123/unhide", nil, ctx, "no policy capability")
	assertPolicyDenied(t, revisions, http.MethodPatch, "/drive/v3/files/file123/revisions/rev123", []byte(`{"keepForever":true}`), ctx, "no policy capability")
}

func TestPolicyCapabilityGroupsCalendarSubgroupsAndSorting(t *testing.T) {
	groups := PolicyCapabilityGroupsForSelection(map[string]bool{})
	calendar := findPolicyGroup(t, groups, "Calendar")
	subgroups := calendar["Subgroups"].([]map[string]any)
	if len(subgroups) != 2 {
		t.Fatalf("expected 2 Calendar subgroups, got %d", len(subgroups))
	}
	assertPolicySubgroup(t, subgroups[0], "Events Operations", []string{
		"calendar_events_create_self",
		"calendar_read",
		"calendar_events_update_self",
		"calendar_events_create_guests",
		"calendar_events_delete_guests",
		"calendar_events_delete_self",
		"calendar_events_update_guests",
	})
	assertPolicySubgroup(t, subgroups[1], "Calendars Operations", []string{
		"calendar_access_read",
		"calendar_settings_read",
		"calendar_access_manage",
		"calendar_calendars_manage",
	})
}

func TestPolicyCapabilityGroupsDriveCommentsSubgroupAndSorting(t *testing.T) {
	groups := PolicyCapabilityGroupsForSelection(map[string]bool{})
	drive := findPolicyGroup(t, groups, "Drive")
	subgroups := drive["Subgroups"].([]map[string]any)
	comments := findPolicySubgroup(t, subgroups, "Comments")
	assertPolicySubgroup(t, comments, "Comments", []string{
		"drive_comments_read",
		"drive_comments_create",
		"drive_comments_reply",
		"drive_comments_update",
		"drive_comments_delete",
		"drive_comments_resolve",
	})
}

func TestPolicyCapabilityGroupsDriveFilesSortingNoDrives(t *testing.T) {
	groups := PolicyCapabilityGroupsForSelection(map[string]bool{})
	drive := findPolicyGroup(t, groups, "Drive")
	subgroups := drive["Subgroups"].([]map[string]any)

	assertPolicySubgroup(t, findPolicySubgroup(t, subgroups, "Files"), "Files", []string{
		"drive_metadata_read",
		"drive_files_read",
		"drive_permissions_read",
		"drive_files_create",
		"drive_files_update",
		"drive_labels_update",
		"drive_files_delete",
		"drive_revisions_delete",
		"drive_permissions_manage",
	})
	assertPolicySubgroupMissing(t, subgroups, "Drives")
}

func TestPolicyCapabilityGroupsGmailSubgroupsAndSorting(t *testing.T) {
	groups := PolicyCapabilityGroupsForSelection(map[string]bool{})
	gmail := findPolicyGroup(t, groups, "Gmail")
	subgroups := gmail["Subgroups"].([]map[string]any)

	assertPolicySubgroup(t, findPolicySubgroup(t, subgroups, "Emails"), "Emails", []string{
		"gmail_drafts_read",
		"gmail_messages_read",
		"gmail_messages_archive",
		"gmail_messages_status",
		"gmail_drafts_write",
		"gmail_drafts_delete",
		"gmail_messages_import",
		"gmail_messages_spam",
		"gmail_messages_trash",
		"gmail_messages_delete",
		"gmail_send",
	})
	assertPolicySubgroup(t, findPolicySubgroup(t, subgroups, "Labels"), "Labels", []string{
		"gmail_labels_read",
		"gmail_labels_apply_custom",
		"gmail_labels_create_rename",
		"gmail_labels_delete",
	})
	assertPolicySubgroup(t, findPolicySubgroup(t, subgroups, "Mailbox"), "Mailbox", []string{
		"gmail_profile_read",
		"gmail_watch_manage",
	})
}

func TestPolicyCapabilityGroupsContacts(t *testing.T) {
	groups := PolicyCapabilityGroupsForSelection(map[string]bool{})
	contacts := findPolicyGroup(t, groups, "Contacts")
	subgroups := contacts["Subgroups"].([]map[string]any)

	assertPolicySubgroup(t, subgroups[0], "Contacts", []string{
		"contacts_read",
	})
}

func findPolicyGroup(t *testing.T, groups []map[string]any, name string) map[string]any {
	t.Helper()
	for _, group := range groups {
		if group["Name"] == name {
			return group
		}
	}
	t.Fatalf("%s policy group not found", name)
	return nil
}

func findPolicySubgroup(t *testing.T, subgroups []map[string]any, name string) map[string]any {
	t.Helper()
	for _, subgroup := range subgroups {
		if subgroup["Name"] == name {
			return subgroup
		}
	}
	t.Fatalf("%s policy subgroup not found", name)
	return nil
}

func assertPolicySubgroupMissing(t *testing.T, subgroups []map[string]any, name string) {
	t.Helper()
	for _, subgroup := range subgroups {
		if subgroup["Name"] == name {
			t.Fatalf("%s policy subgroup should not be present", name)
		}
	}
}

func assertPolicySubgroup(t *testing.T, subgroup map[string]any, wantName string, wantKeys []string) {
	t.Helper()
	if subgroup["Name"] != wantName {
		t.Fatalf("expected subgroup %q, got %q", wantName, subgroup["Name"])
	}
	capabilities := subgroup["Capabilities"].([]map[string]any)
	gotKeys := make([]string, 0, len(capabilities))
	for _, capability := range capabilities {
		gotKeys = append(gotKeys, capability["Key"].(string))
	}
	if strings.Join(gotKeys, ",") != strings.Join(wantKeys, ",") {
		t.Fatalf("unexpected capability order for %s:\nwant %v\n got %v", wantName, wantKeys, gotKeys)
	}
}

func newTestPolicyEngine(t *testing.T, keys []string) *PolicyEngine {
	t.Helper()
	engine, err := NewPolicyEngine(keys)
	if err != nil {
		t.Fatal(err)
	}
	return engine
}

func driveKindCtx(kind string) PolicyEvalContext {
	return PolicyEvalContext{
		MailboxEmail: "me@example.com",
		FetchDriveFileKind: func(fileID string) (string, error) {
			return kind, nil
		},
	}
}

func assertPolicyAllowed(t *testing.T, engine *PolicyEngine, method, path string, body []byte, ctx PolicyEvalContext) {
	t.Helper()
	allowed, reason, err := engine.Evaluate(method, path, body, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !allowed {
		t.Fatalf("expected %s %s to be allowed, denied with reason: %s", method, path, reason)
	}
}

func assertPolicyDenied(t *testing.T, engine *PolicyEngine, method, path string, body []byte, ctx PolicyEvalContext, wantReason string) {
	t.Helper()
	allowed, reason, err := engine.Evaluate(method, path, body, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if allowed {
		t.Fatalf("expected %s %s to be denied", method, path)
	}
	if !strings.Contains(reason, wantReason) {
		t.Fatalf("expected denial reason to contain %q, got %q", wantReason, reason)
	}
}
