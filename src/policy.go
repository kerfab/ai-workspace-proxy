package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sort"
)

const (
	systemPolicyID   = "system"
	systemPolicyName = "Default system policy"
)

type PolicyCapability struct {
	Key               string
	Group             string
	Title             string
	Summary           string
	SystemDefault     bool
	Rules             []PolicyRuleSpec
	RequiresDriveRefs bool
}

type PolicyRuleSpec struct {
	Name       string
	Method     string
	PathRegex  string
	BodyPolicy string
}

type BodyPolicyConfig struct {
	AllowedTopLevelKeys []string
	DenyAddLabelIDs     []string
	DenyRemoveLabelIDs  []string
	AllowRemoveLabelIDs []string
}

type compiledRule struct {
	spec PolicyRuleSpec
	rx   *regexp.Regexp
}

type PolicyEngine struct {
	rules        []compiledRule
	bodyPolicies map[string]BodyPolicyConfig
}

type labelModifyBody struct {
	AddLabelIDs    []string `json:"addLabelIds"`
	RemoveLabelIDs []string `json:"removeLabelIds"`
}

func PolicyCatalog() []PolicyCapability {
	return []PolicyCapability{
		{
			Key:           "gmail_profile_read",
			Group:         "Gmail",
			Title:         "Read Gmail profile",
			Summary:       "Allow agents to read the connected Gmail account profile and mailbox history metadata.",
			SystemDefault: true,
			Rules: []PolicyRuleSpec{
				rule("gmail_profile_read", http.MethodGet, `^/gmail/v1/users/me/profile$`),
				rule("gmail_history_read", http.MethodGet, `^/gmail/v1/users/me/history$`),
			},
		},
		{
			Key:           "gmail_messages_read",
			Group:         "Gmail",
			Title:         "Read Gmail messages",
			Summary:       "Allow agents to search, list, read, and download attachments from Gmail messages and threads.",
			SystemDefault: true,
			Rules: []PolicyRuleSpec{
				rule("gmail_messages_list", http.MethodGet, `^/gmail/v1/users/me/messages$`),
				rule("gmail_messages_get", http.MethodGet, `^/gmail/v1/users/me/messages/[^/]+$`),
				rule("gmail_attachments_get", http.MethodGet, `^/gmail/v1/users/me/messages/[^/]+/attachments/[^/]+$`),
				rule("gmail_threads_list", http.MethodGet, `^/gmail/v1/users/me/threads$`),
				rule("gmail_threads_get", http.MethodGet, `^/gmail/v1/users/me/threads/[^/]+$`),
			},
		},
		{
			Key:           "gmail_drafts_read",
			Group:         "Gmail",
			Title:         "Read Gmail drafts",
			Summary:       "Allow agents to list and read existing Gmail drafts without changing them.",
			SystemDefault: true,
			Rules: []PolicyRuleSpec{
				rule("gmail_drafts_list", http.MethodGet, `^/gmail/v1/users/me/drafts$`),
				rule("gmail_drafts_get", http.MethodGet, `^/gmail/v1/users/me/drafts/[^/]+$`),
			},
		},
		{
			Key:           "gmail_labels_read",
			Group:         "Gmail",
			Title:         "Read Gmail labels",
			Summary:       "Allow agents to list and read Gmail label definitions.",
			SystemDefault: true,
			Rules: []PolicyRuleSpec{
				rule("gmail_labels_list", http.MethodGet, `^/gmail/v1/users/me/labels$`),
				rule("gmail_labels_get", http.MethodGet, `^/gmail/v1/users/me/labels/[^/]+$`),
			},
		},
		{
			Key:     "gmail_drafts_write",
			Group:   "Gmail",
			Title:   "Create and edit Gmail drafts",
			Summary: "Allow agents to create, update, and delete Gmail drafts, but not send them.",
			Rules: []PolicyRuleSpec{
				rule("gmail_drafts_create", http.MethodPost, `^/gmail/v1/users/me/drafts$`),
				rule("gmail_drafts_update", http.MethodPut, `^/gmail/v1/users/me/drafts/[^/]+$`),
				rule("gmail_drafts_delete", http.MethodDelete, `^/gmail/v1/users/me/drafts/[^/]+$`),
			},
		},
		{
			Key:     "gmail_send",
			Group:   "Gmail",
			Title:   "Send Gmail messages",
			Summary: "Allow agents to send Gmail messages and drafts from the connected mailbox.",
			Rules: []PolicyRuleSpec{
				rule("gmail_messages_send", http.MethodPost, `^/gmail/v1/users/me/messages/send$`),
				rule("gmail_drafts_send", http.MethodPost, `^/gmail/v1/users/me/drafts/send$`),
			},
		},
		{
			Key:     "gmail_labels_manage",
			Group:   "Gmail",
			Title:   "Manage Gmail labels",
			Summary: "Allow agents to create, rename, and delete Gmail labels.",
			Rules: []PolicyRuleSpec{
				rule("gmail_labels_create", http.MethodPost, `^/gmail/v1/users/me/labels$`),
				rule("gmail_labels_patch", http.MethodPatch, `^/gmail/v1/users/me/labels/[^/]+$`),
				rule("gmail_labels_update", http.MethodPut, `^/gmail/v1/users/me/labels/[^/]+$`),
				rule("gmail_labels_delete", http.MethodDelete, `^/gmail/v1/users/me/labels/[^/]+$`),
			},
		},
		{
			Key:     "gmail_labels_apply_safe",
			Group:   "Gmail",
			Title:   "Apply safe Gmail label changes",
			Summary: "Allow agents to add or remove ordinary Gmail labels while blocking destructive system-label changes.",
			Rules: []PolicyRuleSpec{
				ruleWithBody("gmail_messages_modify_safe", http.MethodPost, `^/gmail/v1/users/me/messages/[^/]+/modify$`, "gmail_label_modify"),
				ruleWithBody("gmail_messages_batch_modify_safe", http.MethodPost, `^/gmail/v1/users/me/messages/batchModify$`, "gmail_label_modify"),
				ruleWithBody("gmail_threads_modify_safe", http.MethodPost, `^/gmail/v1/users/me/threads/[^/]+/modify$`, "gmail_label_modify"),
			},
		},
		{
			Key:     "gmail_messages_delete",
			Group:   "Gmail",
			Title:   "Delete or trash Gmail messages",
			Summary: "Allow agents to trash, untrash, or permanently delete Gmail messages.",
			Rules: []PolicyRuleSpec{
				rule("gmail_messages_trash", http.MethodPost, `^/gmail/v1/users/me/messages/[^/]+/trash$`),
				rule("gmail_messages_untrash", http.MethodPost, `^/gmail/v1/users/me/messages/[^/]+/untrash$`),
				rule("gmail_messages_delete", http.MethodDelete, `^/gmail/v1/users/me/messages/[^/]+$`),
				rule("gmail_messages_batch_delete", http.MethodPost, `^/gmail/v1/users/me/messages/batchDelete$`),
			},
		},
		{
			Key:     "gmail_messages_import",
			Group:   "Gmail",
			Title:   "Import Gmail messages",
			Summary: "Allow agents to import or insert messages into the connected Gmail mailbox.",
			Rules: []PolicyRuleSpec{
				rule("gmail_messages_import", http.MethodPost, `^/gmail/v1/users/me/messages/import$`),
				rule("gmail_messages_insert", http.MethodPost, `^/gmail/v1/users/me/messages$`),
			},
		},
		{
			Key:     "gmail_watch_manage",
			Group:   "Gmail",
			Title:   "Manage Gmail change notifications",
			Summary: "Allow agents to start or stop Gmail push notification watches.",
			Rules: []PolicyRuleSpec{
				rule("gmail_watch_start", http.MethodPost, `^/gmail/v1/users/me/watch$`),
				rule("gmail_watch_stop", http.MethodPost, `^/gmail/v1/users/me/stop$`),
			},
		},
		{
			Key:           "calendar_read",
			Group:         "Calendar",
			Title:         "Read calendars and events",
			Summary:       "Allow agents to list calendars and read calendar events without changing them.",
			SystemDefault: true,
			Rules: []PolicyRuleSpec{
				rule("calendar_list", http.MethodGet, `^/calendar/v3/users/me/calendarList$`),
				rule("calendar_list_get", http.MethodGet, `^/calendar/v3/users/me/calendarList/[^/]+$`),
				rule("calendar_calendars_get", http.MethodGet, `^/calendar/v3/calendars/[^/]+$`),
				rule("calendar_events_list", http.MethodGet, `^/calendar/v3/calendars/[^/]+/events$`),
				rule("calendar_events_get", http.MethodGet, `^/calendar/v3/calendars/[^/]+/events/[^/]+$`),
				rule("calendar_events_instances", http.MethodGet, `^/calendar/v3/calendars/[^/]+/events/[^/]+/instances$`),
				rule("calendar_freebusy", http.MethodPost, `^/calendar/v3/freeBusy$`),
			},
		},
		{
			Key:     "calendar_events_write",
			Group:   "Calendar",
			Title:   "Create and edit calendar events",
			Summary: "Allow agents to create, update, move, import, or delete calendar events.",
			Rules: []PolicyRuleSpec{
				rule("calendar_events_insert", http.MethodPost, `^/calendar/v3/calendars/[^/]+/events$`),
				rule("calendar_events_import", http.MethodPost, `^/calendar/v3/calendars/[^/]+/events/import$`),
				rule("calendar_events_patch", http.MethodPatch, `^/calendar/v3/calendars/[^/]+/events/[^/]+$`),
				rule("calendar_events_update", http.MethodPut, `^/calendar/v3/calendars/[^/]+/events/[^/]+$`),
				rule("calendar_events_delete", http.MethodDelete, `^/calendar/v3/calendars/[^/]+/events/[^/]+$`),
				rule("calendar_events_move", http.MethodPost, `^/calendar/v3/calendars/[^/]+/events/[^/]+/move$`),
			},
		},
		{
			Key:     "calendar_calendars_manage",
			Group:   "Calendar",
			Title:   "Manage calendars and calendar lists",
			Summary: "Allow agents to create, update, clear, or delete calendars and calendar list entries.",
			Rules: []PolicyRuleSpec{
				rule("calendar_calendars_insert", http.MethodPost, `^/calendar/v3/calendars$`),
				rule("calendar_calendars_patch", http.MethodPatch, `^/calendar/v3/calendars/[^/]+$`),
				rule("calendar_calendars_update", http.MethodPut, `^/calendar/v3/calendars/[^/]+$`),
				rule("calendar_calendars_clear", http.MethodPost, `^/calendar/v3/calendars/[^/]+/clear$`),
				rule("calendar_calendars_delete", http.MethodDelete, `^/calendar/v3/calendars/[^/]+$`),
				rule("calendar_list_insert", http.MethodPost, `^/calendar/v3/users/me/calendarList$`),
				rule("calendar_list_patch", http.MethodPatch, `^/calendar/v3/users/me/calendarList/[^/]+$`),
				rule("calendar_list_update", http.MethodPut, `^/calendar/v3/users/me/calendarList/[^/]+$`),
				rule("calendar_list_delete", http.MethodDelete, `^/calendar/v3/users/me/calendarList/[^/]+$`),
			},
		},
		{
			Key:           "calendar_access_read",
			Group:         "Calendar",
			Title:         "Read calendar sharing",
			Summary:       "Allow agents to read calendar sharing rules without changing them.",
			SystemDefault: true,
			Rules: []PolicyRuleSpec{
				rule("calendar_acl_list", http.MethodGet, `^/calendar/v3/calendars/[^/]+/acl$`),
				rule("calendar_acl_get", http.MethodGet, `^/calendar/v3/calendars/[^/]+/acl/[^/]+$`),
			},
		},
		{
			Key:     "calendar_access_manage",
			Group:   "Calendar",
			Title:   "Manage calendar sharing",
			Summary: "Allow agents to create, update, delete, or watch calendar sharing rules.",
			Rules: []PolicyRuleSpec{
				rule("calendar_acl_insert", http.MethodPost, `^/calendar/v3/calendars/[^/]+/acl$`),
				rule("calendar_acl_patch", http.MethodPatch, `^/calendar/v3/calendars/[^/]+/acl/[^/]+$`),
				rule("calendar_acl_update", http.MethodPut, `^/calendar/v3/calendars/[^/]+/acl/[^/]+$`),
				rule("calendar_acl_delete", http.MethodDelete, `^/calendar/v3/calendars/[^/]+/acl/[^/]+$`),
				rule("calendar_acl_watch", http.MethodPost, `^/calendar/v3/calendars/[^/]+/acl/watch$`),
			},
		},
		{
			Key:           "calendar_settings_read",
			Group:         "Calendar",
			Title:         "Read Calendar settings",
			Summary:       "Allow agents to read Google Calendar user settings.",
			SystemDefault: true,
			Rules: []PolicyRuleSpec{
				rule("calendar_settings_list", http.MethodGet, `^/calendar/v3/users/me/settings$`),
				rule("calendar_settings_get", http.MethodGet, `^/calendar/v3/users/me/settings/[^/]+$`),
			},
		},
		{
			Key:               "drive_files_read",
			Group:             "Drive files",
			Title:             "Read Drive files",
			Summary:           "Allow agents to search, read metadata, export, and download files inside allowed Drive folders.",
			SystemDefault:     true,
			RequiresDriveRefs: true,
			Rules: []PolicyRuleSpec{
				rule("drive_files_list", http.MethodGet, `^/drive/v3/files$`),
				rule("drive_files_get", http.MethodGet, `^/drive/v3/files/[^/]+$`),
				rule("drive_files_export", http.MethodGet, `^/drive/v3/files/[^/]+/export$`),
				rule("drive_about_get", http.MethodGet, `^/drive/v3/about$`),
			},
		},
		{
			Key:               "drive_files_create",
			Group:             "Drive files",
			Title:             "Create Drive files",
			Summary:           "Allow agents to create files inside allowed Drive folders.",
			RequiresDriveRefs: true,
			Rules: []PolicyRuleSpec{
				rule("drive_files_create", http.MethodPost, `^/drive/v3/files$`),
				rule("drive_files_copy", http.MethodPost, `^/drive/v3/files/[^/]+/copy$`),
			},
		},
		{
			Key:               "drive_files_update",
			Group:             "Drive files",
			Title:             "Edit Drive files",
			Summary:           "Allow agents to rename, update metadata, or upload new content for files in allowed Drive folders.",
			RequiresDriveRefs: true,
			Rules: []PolicyRuleSpec{
				rule("drive_files_patch", http.MethodPatch, `^/drive/v3/files/[^/]+$`),
				rule("drive_files_update", http.MethodPut, `^/drive/v3/files/[^/]+$`),
				rule("drive_files_update_media", http.MethodPatch, `^/upload/drive/v3/files/[^/]+$`),
			},
		},
		{
			Key:               "drive_files_delete",
			Group:             "Drive files",
			Title:             "Trash or delete Drive files",
			Summary:           "Allow agents to trash or permanently delete files in allowed Drive folders.",
			RequiresDriveRefs: true,
			Rules: []PolicyRuleSpec{
				rule("drive_files_delete", http.MethodDelete, `^/drive/v3/files/[^/]+$`),
				rule("drive_files_empty_trash", http.MethodDelete, `^/drive/v3/files/trash$`),
			},
		},
		{
			Key:           "drive_permissions_read",
			Group:         "Drive files",
			Title:         "Read Drive sharing",
			Summary:       "Allow agents to read Drive file permissions without changing them.",
			SystemDefault: true,
			Rules: []PolicyRuleSpec{
				rule("drive_permissions_list", http.MethodGet, `^/drive/v3/files/[^/]+/permissions$`),
				rule("drive_permissions_get", http.MethodGet, `^/drive/v3/files/[^/]+/permissions/[^/]+$`),
			},
		},
		{
			Key:     "drive_permissions_manage",
			Group:   "Drive files",
			Title:   "Manage Drive sharing",
			Summary: "Allow agents to create, update, or delete Drive file permissions.",
			Rules: []PolicyRuleSpec{
				rule("drive_permissions_create", http.MethodPost, `^/drive/v3/files/[^/]+/permissions$`),
				rule("drive_permissions_patch", http.MethodPatch, `^/drive/v3/files/[^/]+/permissions/[^/]+$`),
				rule("drive_permissions_update", http.MethodPut, `^/drive/v3/files/[^/]+/permissions/[^/]+$`),
				rule("drive_permissions_delete", http.MethodDelete, `^/drive/v3/files/[^/]+/permissions/[^/]+$`),
			},
		},
		{
			Key:           "drive_comments_read",
			Group:         "Drive files",
			Title:         "Read Drive comments",
			Summary:       "Allow agents to read Drive file comments and replies without changing them.",
			SystemDefault: true,
			Rules: []PolicyRuleSpec{
				rule("drive_comments_list", http.MethodGet, `^/drive/v3/files/[^/]+/comments$`),
				rule("drive_comments_get", http.MethodGet, `^/drive/v3/files/[^/]+/comments/[^/]+$`),
				rule("drive_replies_list", http.MethodGet, `^/drive/v3/files/[^/]+/comments/[^/]+/replies$`),
				rule("drive_replies_get", http.MethodGet, `^/drive/v3/files/[^/]+/comments/[^/]+/replies/[^/]+$`),
			},
		},
		{
			Key:     "drive_comments_manage",
			Group:   "Drive files",
			Title:   "Manage Drive comments",
			Summary: "Allow agents to create, update, delete, and reply to Drive file comments.",
			Rules: []PolicyRuleSpec{
				rule("drive_comments_create", http.MethodPost, `^/drive/v3/files/[^/]+/comments$`),
				rule("drive_comments_patch", http.MethodPatch, `^/drive/v3/files/[^/]+/comments/[^/]+$`),
				rule("drive_comments_update", http.MethodPut, `^/drive/v3/files/[^/]+/comments/[^/]+$`),
				rule("drive_comments_delete", http.MethodDelete, `^/drive/v3/files/[^/]+/comments/[^/]+$`),
				rule("drive_replies_create", http.MethodPost, `^/drive/v3/files/[^/]+/comments/[^/]+/replies$`),
				rule("drive_replies_patch", http.MethodPatch, `^/drive/v3/files/[^/]+/comments/[^/]+/replies/[^/]+$`),
				rule("drive_replies_update", http.MethodPut, `^/drive/v3/files/[^/]+/comments/[^/]+/replies/[^/]+$`),
				rule("drive_replies_delete", http.MethodDelete, `^/drive/v3/files/[^/]+/comments/[^/]+/replies/[^/]+$`),
			},
		},
		{
			Key:           "drive_metadata_read",
			Group:         "Drive files",
			Title:         "Read Drive metadata",
			Summary:       "Allow agents to read Drive changes, shared drives, apps, labels, and file revisions.",
			SystemDefault: true,
			Rules: []PolicyRuleSpec{
				rule("drive_changes_get_start_page_token", http.MethodGet, `^/drive/v3/changes/startPageToken$`),
				rule("drive_changes_list", http.MethodGet, `^/drive/v3/changes$`),
				rule("drive_drives_list", http.MethodGet, `^/drive/v3/drives$`),
				rule("drive_drives_get", http.MethodGet, `^/drive/v3/drives/[^/]+$`),
				rule("drive_apps_list", http.MethodGet, `^/drive/v3/apps$`),
				rule("drive_apps_get", http.MethodGet, `^/drive/v3/apps/[^/]+$`),
				rule("drive_labels_list", http.MethodGet, `^/drive/v3/labels$`),
				rule("drive_labels_get", http.MethodGet, `^/drive/v3/labels/[^/]+$`),
				rule("drive_revisions_list", http.MethodGet, `^/drive/v3/files/[^/]+/revisions$`),
				rule("drive_revisions_get", http.MethodGet, `^/drive/v3/files/[^/]+/revisions/[^/]+$`),
			},
		},
		{
			Key:     "drive_metadata_manage",
			Group:   "Drive files",
			Title:   "Manage Drive metadata",
			Summary: "Allow agents to hide shared drives, update labels, and delete Drive revisions.",
			Rules: []PolicyRuleSpec{
				rule("drive_drives_hide", http.MethodPost, `^/drive/v3/drives/[^/]+/hide$`),
				rule("drive_drives_unhide", http.MethodPost, `^/drive/v3/drives/[^/]+/unhide$`),
				rule("drive_files_modify_labels", http.MethodPost, `^/drive/v3/files/[^/]+/modifyLabels$`),
				rule("drive_revisions_update", http.MethodPatch, `^/drive/v3/files/[^/]+/revisions/[^/]+$`),
				rule("drive_revisions_delete", http.MethodDelete, `^/drive/v3/files/[^/]+/revisions/[^/]+$`),
			},
		},
		{
			Key:               "docs_read",
			Group:             "Docs",
			Title:             "Read Google Docs",
			Summary:           "Allow agents to read Google Docs documents located in allowed Drive folders.",
			SystemDefault:     true,
			RequiresDriveRefs: true,
			Rules: []PolicyRuleSpec{
				rule("docs_documents_get", http.MethodGet, `^/v1/documents/[^/:]+$`),
			},
		},
		{
			Key:               "docs_create",
			Group:             "Docs",
			Title:             "Create Google Docs",
			Summary:           "Allow agents to create Google Docs documents in allowed Drive folders.",
			RequiresDriveRefs: true,
			Rules: []PolicyRuleSpec{
				rule("docs_documents_create", http.MethodPost, `^/v1/documents$`),
			},
		},
		{
			Key:               "docs_edit",
			Group:             "Docs",
			Title:             "Edit Google Docs",
			Summary:           "Allow agents to modify Google Docs documents in allowed Drive folders.",
			RequiresDriveRefs: true,
			Rules: []PolicyRuleSpec{
				rule("docs_documents_batch_update", http.MethodPost, `^/v1/documents/[^/:]+:batchUpdate$`),
			},
		},
		{
			Key:               "sheets_read",
			Group:             "Sheets",
			Title:             "Read Google Sheets",
			Summary:           "Allow agents to read Google Sheets spreadsheets and cell values in allowed Drive folders.",
			SystemDefault:     true,
			RequiresDriveRefs: true,
			Rules: []PolicyRuleSpec{
				rule("sheets_spreadsheets_get", http.MethodGet, `^/v4/spreadsheets/[^/]+$`),
				rule("sheets_values_get", http.MethodGet, `^/v4/spreadsheets/[^/]+/values/.+$`),
				rule("sheets_values_batch_get", http.MethodGet, `^/v4/spreadsheets/[^/]+/values:batchGet$`),
				rule("sheets_spreadsheets_get_by_data_filter", http.MethodPost, `^/v4/spreadsheets/[^/]+:getByDataFilter$`),
				rule("sheets_values_batch_get_by_data_filter", http.MethodPost, `^/v4/spreadsheets/[^/]+/values:batchGetByDataFilter$`),
				rule("sheets_developer_metadata_get", http.MethodGet, `^/v4/spreadsheets/[^/]+/developerMetadata/[^/]+$`),
				rule("sheets_developer_metadata_search", http.MethodPost, `^/v4/spreadsheets/[^/]+/developerMetadata:search$`),
			},
		},
		{
			Key:               "sheets_create",
			Group:             "Sheets",
			Title:             "Create Google Sheets",
			Summary:           "Allow agents to create Google Sheets spreadsheets in allowed Drive folders.",
			RequiresDriveRefs: true,
			Rules: []PolicyRuleSpec{
				rule("sheets_spreadsheets_create", http.MethodPost, `^/v4/spreadsheets$`),
			},
		},
		{
			Key:               "sheets_edit",
			Group:             "Sheets",
			Title:             "Edit Google Sheets",
			Summary:           "Allow agents to update spreadsheet structure, metadata, and cell values in allowed Drive folders.",
			RequiresDriveRefs: true,
			Rules: []PolicyRuleSpec{
				rule("sheets_spreadsheets_batch_update", http.MethodPost, `^/v4/spreadsheets/[^/]+:batchUpdate$`),
				rule("sheets_values_update", http.MethodPut, `^/v4/spreadsheets/[^/]+/values/.+$`),
				rule("sheets_values_append", http.MethodPost, `^/v4/spreadsheets/[^/]+/values/.+:append$`),
				rule("sheets_values_batch_update", http.MethodPost, `^/v4/spreadsheets/[^/]+/values:batchUpdate$`),
				rule("sheets_values_batch_update_by_data_filter", http.MethodPost, `^/v4/spreadsheets/[^/]+/values:batchUpdateByDataFilter$`),
				rule("sheets_values_clear", http.MethodPost, `^/v4/spreadsheets/[^/]+/values/.+:clear$`),
				rule("sheets_values_batch_clear", http.MethodPost, `^/v4/spreadsheets/[^/]+/values:batchClear$`),
				rule("sheets_values_batch_clear_by_data_filter", http.MethodPost, `^/v4/spreadsheets/[^/]+/values:batchClearByDataFilter$`),
				rule("sheets_sheets_copy_to", http.MethodPost, `^/v4/spreadsheets/[^/]+/sheets/[^/]+:copyTo$`),
			},
		},
		{
			Key:               "slides_read",
			Group:             "Slides",
			Title:             "Read Google Slides",
			Summary:           "Allow agents to read Google Slides presentations and pages in allowed Drive folders.",
			SystemDefault:     true,
			RequiresDriveRefs: true,
			Rules: []PolicyRuleSpec{
				rule("slides_presentations_get", http.MethodGet, `^/v1/presentations/[^/]+$`),
				rule("slides_pages_get", http.MethodGet, `^/v1/presentations/[^/]+/pages/[^/]+$`),
				rule("slides_pages_get_thumbnail", http.MethodGet, `^/v1/presentations/[^/]+/pages/[^/]+/thumbnail$`),
			},
		},
		{
			Key:               "slides_create",
			Group:             "Slides",
			Title:             "Create Google Slides",
			Summary:           "Allow agents to create Google Slides presentations in allowed Drive folders.",
			RequiresDriveRefs: true,
			Rules: []PolicyRuleSpec{
				rule("slides_presentations_create", http.MethodPost, `^/v1/presentations$`),
			},
		},
		{
			Key:               "slides_edit",
			Group:             "Slides",
			Title:             "Edit Google Slides",
			Summary:           "Allow agents to modify Google Slides presentations in allowed Drive folders.",
			RequiresDriveRefs: true,
			Rules: []PolicyRuleSpec{
				rule("slides_presentations_batch_update", http.MethodPost, `^/v1/presentations/[^/]+:batchUpdate$`),
			},
		},
	}
}

func rule(name, method, pathRegex string) PolicyRuleSpec {
	return PolicyRuleSpec{Name: name, Method: method, PathRegex: pathRegex}
}

func ruleWithBody(name, method, pathRegex, bodyPolicy string) PolicyRuleSpec {
	return PolicyRuleSpec{Name: name, Method: method, PathRegex: pathRegex, BodyPolicy: bodyPolicy}
}

func SystemDefaultCapabilityKeys() []string {
	out := []string{}
	for _, cap := range PolicyCatalog() {
		if cap.SystemDefault {
			out = append(out, cap.Key)
		}
	}
	sort.Strings(out)
	return out
}

func NewPolicyEngine(capabilityKeys []string) (*PolicyEngine, error) {
	allowed := sliceToSet(capabilityKeys)
	catalog := PolicyCatalog()
	engine := &PolicyEngine{
		rules:        []compiledRule{},
		bodyPolicies: defaultBodyPolicies(),
	}
	for _, cap := range catalog {
		if !allowed[cap.Key] {
			continue
		}
		for _, spec := range cap.Rules {
			rx, err := regexp.Compile(spec.PathRegex)
			if err != nil {
				return nil, fmt.Errorf("invalid path_regex for %s: %w", spec.Name, err)
			}
			engine.rules = append(engine.rules, compiledRule{spec: spec, rx: rx})
		}
	}
	return engine, nil
}

func defaultBodyPolicies() map[string]BodyPolicyConfig {
	return map[string]BodyPolicyConfig{
		"gmail_label_modify": {
			AllowedTopLevelKeys: []string{"addLabelIds", "removeLabelIds", "ids"},
			DenyAddLabelIDs:     []string{"TRASH", "SPAM", "UNREAD"},
			DenyRemoveLabelIDs:  []string{"TRASH", "SPAM", "SENT", "DRAFT"},
			AllowRemoveLabelIDs: []string{"INBOX", "UNREAD", "CATEGORY_PERSONAL", "CATEGORY_SOCIAL", "CATEGORY_PROMOTIONS", "CATEGORY_UPDATES", "CATEGORY_FORUMS", "*"},
		},
	}
}

func (p *PolicyEngine) Evaluate(method, normalizedPath string, body []byte) (bool, string, error) {
	for _, rule := range p.rules {
		if rule.spec.Method != method {
			continue
		}
		if !rule.rx.MatchString(normalizedPath) {
			continue
		}
		if rule.spec.BodyPolicy == "" {
			return true, rule.spec.Name, nil
		}
		cfg, ok := p.bodyPolicies[rule.spec.BodyPolicy]
		if !ok {
			return false, "", fmt.Errorf("missing body policy %q", rule.spec.BodyPolicy)
		}
		if err := validateBodyPolicy(cfg, body); err != nil {
			return false, err.Error(), nil
		}
		return true, rule.spec.Name, nil
	}
	return false, "no policy capability allowed this operation", nil
}

func validateBodyPolicy(cfg BodyPolicyConfig, body []byte) error {
	if len(body) == 0 {
		return fmt.Errorf("request body is required")
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return fmt.Errorf("request body must be valid JSON: %w", err)
	}
	allowedKeys := sliceToSet(cfg.AllowedTopLevelKeys)
	for key := range raw {
		if !allowedKeys[key] {
			return fmt.Errorf("body key %q is not allowed", key)
		}
	}
	var payload labelModifyBody
	if err := json.Unmarshal(body, &payload); err != nil {
		return fmt.Errorf("invalid label modify payload: %w", err)
	}
	denyAdd := sliceToSet(cfg.DenyAddLabelIDs)
	denyRemove := sliceToSet(cfg.DenyRemoveLabelIDs)
	allowRemove := sliceToSet(cfg.AllowRemoveLabelIDs)
	wildcard := allowRemove["*"]

	for _, label := range payload.AddLabelIDs {
		if denyAdd[label] {
			return fmt.Errorf("adding label %q is forbidden", label)
		}
	}
	for _, label := range payload.RemoveLabelIDs {
		if denyRemove[label] {
			return fmt.Errorf("removing label %q is forbidden", label)
		}
		if !wildcard && !allowRemove[label] {
			return fmt.Errorf("removing label %q is not allowed by policy", label)
		}
	}
	return nil
}

func PolicyCapabilitiesForSelection(selected map[string]bool) []map[string]any {
	catalog := PolicyCatalog()
	out := make([]map[string]any, 0, len(catalog))
	for _, cap := range catalog {
		out = append(out, map[string]any{
			"Key":               cap.Key,
			"Group":             cap.Group,
			"Title":             cap.Title,
			"Summary":           cap.Summary,
			"Checked":           selected[cap.Key],
			"SystemDefault":     cap.SystemDefault,
			"RequiresDriveRefs": cap.RequiresDriveRefs,
		})
	}
	return out
}

func validCapabilityKeys(keys []string) []string {
	valid := map[string]bool{}
	for _, cap := range PolicyCatalog() {
		valid[cap.Key] = true
	}
	seen := map[string]bool{}
	out := []string{}
	for _, key := range keys {
		if valid[key] && !seen[key] {
			seen[key] = true
			out = append(out, key)
		}
	}
	sort.Strings(out)
	return out
}

func sliceToSet(values []string) map[string]bool {
	out := map[string]bool{}
	for _, v := range values {
		out[v] = true
	}
	return out
}
