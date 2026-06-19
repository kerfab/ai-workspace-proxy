// Copyright (c) 2026 Opensense Ltd. (Hong Kong). All rights reserved.
// Proprietary software. No use, copy, modification, distribution, disclosure,
// or reverse engineering is permitted without prior written authorization
// from Opensense Ltd.

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
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

type compiledRule struct {
	spec            PolicyRuleSpec
	rx              *regexp.Regexp
	capabilityKey   string
	capabilityTitle string
}

type PolicyEvalContext struct {
	MailboxEmail              string
	FetchCalendarEvent        func(calendarID, eventID string) (map[string]any, error)
	FetchDriveCommentAuthorMe func(fileID, commentID, replyID string) (bool, error)
	FetchDriveFileKind        func(fileID string) (string, error)
}

type BodyPolicyEvaluator func(ctx PolicyEvalContext, method, normalizedPath string, body []byte) (bool, string, error)

type PolicyEngine struct {
	rules                  []compiledRule
	bodyPolicies           map[string]BodyPolicyEvaluator
	reviewRequiredPolicies map[string]bool
}

type PolicyDecision struct {
	Allowed               bool
	Reason                string
	CapabilityKey         string
	CapabilityTitle       string
	RuleName              string
	RequiresHumanApproval bool
}

type labelModifyBody struct {
	AddLabelIDs    []string `json:"addLabelIds"`
	RemoveLabelIDs []string `json:"removeLabelIds"`
}

type gmailLabelBody struct {
	Name string `json:"name"`
}

func PolicyCatalog() []PolicyCapability {
	return []PolicyCapability{
		{
			Key:           "gmail_profile_read",
			Group:         "Gmail",
			Title:         "Read profile",
			Summary:       "Allow agents to read the connected account profile and mailbox history metadata.",
			SystemDefault: true,
			Rules: []PolicyRuleSpec{
				rule("gmail_profile_read", http.MethodGet, `^/gmail/v1/users/me/profile$`),
				rule("gmail_history_read", http.MethodGet, `^/gmail/v1/users/me/history$`),
			},
		},
		{
			Key:           "gmail_messages_read",
			Group:         "Gmail",
			Title:         "Read emails",
			Summary:       "Allow agents to search, list, read, and download attachments from emails and threads.",
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
			Title:         "Read drafts",
			Summary:       "Allow agents to list and read existing drafts without changing them.",
			SystemDefault: true,
			Rules: []PolicyRuleSpec{
				rule("gmail_drafts_list", http.MethodGet, `^/gmail/v1/users/me/drafts$`),
				rule("gmail_drafts_get", http.MethodGet, `^/gmail/v1/users/me/drafts/[^/]+$`),
			},
		},
		{
			Key:           "gmail_labels_read",
			Group:         "Gmail",
			Title:         "Read labels",
			Summary:       "Allow agents to list and read label definitions.",
			SystemDefault: true,
			Rules: []PolicyRuleSpec{
				rule("gmail_labels_list", http.MethodGet, `^/gmail/v1/users/me/labels$`),
				rule("gmail_labels_get", http.MethodGet, `^/gmail/v1/users/me/labels/[^/]+$`),
			},
		},
		{
			Key:           "contacts_read",
			Group:         "Contacts",
			Title:         "Read contacts",
			Summary:       "Allow agents to search contacts and directory entries to resolve people from partial names or email hints.",
			SystemDefault: true,
			Rules: []PolicyRuleSpec{
				rule("people_contacts_search", http.MethodGet, `^/v1/people:searchContacts$`),
				rule("people_directory_search", http.MethodGet, `^/v1/people:searchDirectoryPeople$`),
				rule("people_other_contacts_search", http.MethodGet, `^/v1/otherContacts:search$`),
			},
		},
		{
			Key:     "gmail_drafts_write",
			Group:   "Gmail",
			Title:   "Create and edit drafts",
			Summary: "Allow agents to create and update drafts, but not delete or send them.",
			Rules: []PolicyRuleSpec{
				rule("gmail_drafts_create", http.MethodPost, `^/gmail/v1/users/me/drafts$`),
				rule("gmail_drafts_update", http.MethodPut, `^/gmail/v1/users/me/drafts/[^/]+$`),
			},
		},
		{
			Key:     "gmail_drafts_delete",
			Group:   "Gmail",
			Title:   "Delete drafts",
			Summary: "Allow agents to delete existing drafts without sending them.",
			Rules: []PolicyRuleSpec{
				rule("gmail_drafts_delete", http.MethodDelete, `^/gmail/v1/users/me/drafts/[^/]+$`),
			},
		},
		{
			Key:     "gmail_send",
			Group:   "Gmail",
			Title:   "Send emails",
			Summary: "Allow agents to send emails and drafts from the connected mailbox.",
			Rules: []PolicyRuleSpec{
				rule("gmail_messages_send", http.MethodPost, `^/gmail/v1/users/me/messages/send$`),
				rule("gmail_drafts_send", http.MethodPost, `^/gmail/v1/users/me/drafts/send$`),
			},
		},
		{
			Key:     "gmail_labels_apply_custom",
			Group:   "Gmail",
			Title:   "Apply or remove custom labels",
			Summary: "Allow agents to apply or remove user-created labels on emails and threads.",
			Rules: []PolicyRuleSpec{
				ruleWithBody("gmail_messages_modify_labels", http.MethodPost, `^/gmail/v1/users/me/messages/[^/]+/modify$`, "gmail_label_modify"),
				ruleWithBody("gmail_messages_batch_modify_labels", http.MethodPost, `^/gmail/v1/users/me/messages/batchModify$`, "gmail_label_modify"),
				ruleWithBody("gmail_threads_modify_labels", http.MethodPost, `^/gmail/v1/users/me/threads/[^/]+/modify$`, "gmail_label_modify"),
			},
		},
		{
			Key:     "gmail_messages_archive",
			Group:   "Gmail",
			Title:   "Archive or unarchive emails",
			Summary: "Allow agents to archive emails by removing Inbox, or unarchive them by restoring Inbox.",
			Rules: []PolicyRuleSpec{
				ruleWithBody("gmail_messages_modify_archive", http.MethodPost, `^/gmail/v1/users/me/messages/[^/]+/modify$`, "gmail_label_modify"),
				ruleWithBody("gmail_messages_batch_modify_archive", http.MethodPost, `^/gmail/v1/users/me/messages/batchModify$`, "gmail_label_modify"),
				ruleWithBody("gmail_threads_modify_archive", http.MethodPost, `^/gmail/v1/users/me/threads/[^/]+/modify$`, "gmail_label_modify"),
			},
		},
		{
			Key:     "gmail_messages_status",
			Group:   "Gmail",
			Title:   "Change email status",
			Summary: "Allow agents to mark emails read or unread, starred or unstarred, and important or not important.",
			Rules: []PolicyRuleSpec{
				ruleWithBody("gmail_messages_modify_status", http.MethodPost, `^/gmail/v1/users/me/messages/[^/]+/modify$`, "gmail_label_modify"),
				ruleWithBody("gmail_messages_batch_modify_status", http.MethodPost, `^/gmail/v1/users/me/messages/batchModify$`, "gmail_label_modify"),
				ruleWithBody("gmail_threads_modify_status", http.MethodPost, `^/gmail/v1/users/me/threads/[^/]+/modify$`, "gmail_label_modify"),
			},
		},
		{
			Key:     "gmail_messages_spam",
			Group:   "Gmail",
			Title:   "Move emails to or from Spam",
			Summary: "Allow agents to mark emails as spam or remove them from spam.",
			Rules: []PolicyRuleSpec{
				ruleWithBody("gmail_messages_modify_spam", http.MethodPost, `^/gmail/v1/users/me/messages/[^/]+/modify$`, "gmail_label_modify"),
				ruleWithBody("gmail_messages_batch_modify_spam", http.MethodPost, `^/gmail/v1/users/me/messages/batchModify$`, "gmail_label_modify"),
				ruleWithBody("gmail_threads_modify_spam", http.MethodPost, `^/gmail/v1/users/me/threads/[^/]+/modify$`, "gmail_label_modify"),
			},
		},
		{
			Key:     "gmail_messages_trash",
			Group:   "Gmail",
			Title:   "Move emails to or from Trash",
			Summary: "Allow agents to move emails and threads to Trash or restore them from Trash.",
			Rules: []PolicyRuleSpec{
				rule("gmail_messages_trash", http.MethodPost, `^/gmail/v1/users/me/messages/[^/]+/trash$`),
				rule("gmail_messages_untrash", http.MethodPost, `^/gmail/v1/users/me/messages/[^/]+/untrash$`),
				rule("gmail_threads_trash", http.MethodPost, `^/gmail/v1/users/me/threads/[^/]+/trash$`),
				rule("gmail_threads_untrash", http.MethodPost, `^/gmail/v1/users/me/threads/[^/]+/untrash$`),
				ruleWithBody("gmail_messages_modify_trash", http.MethodPost, `^/gmail/v1/users/me/messages/[^/]+/modify$`, "gmail_label_modify"),
				ruleWithBody("gmail_messages_batch_modify_trash", http.MethodPost, `^/gmail/v1/users/me/messages/batchModify$`, "gmail_label_modify"),
				ruleWithBody("gmail_threads_modify_trash", http.MethodPost, `^/gmail/v1/users/me/threads/[^/]+/modify$`, "gmail_label_modify"),
			},
		},
		{
			Key:     "gmail_labels_create_rename",
			Group:   "Gmail",
			Title:   "Create or rename labels",
			Summary: "Allow agents to create user labels and rename existing user labels, excluding system labels.",
			Rules: []PolicyRuleSpec{
				ruleWithBody("gmail_labels_create", http.MethodPost, `^/gmail/v1/users/me/labels$`, "gmail_label_create_rename"),
				ruleWithBody("gmail_labels_patch", http.MethodPatch, `^/gmail/v1/users/me/labels/[^/]+$`, "gmail_label_create_rename"),
				ruleWithBody("gmail_labels_update", http.MethodPut, `^/gmail/v1/users/me/labels/[^/]+$`, "gmail_label_create_rename"),
			},
		},
		{
			Key:     "gmail_labels_delete",
			Group:   "Gmail",
			Title:   "Delete labels",
			Summary: "Allow agents to permanently delete user labels, excluding system labels.",
			Rules: []PolicyRuleSpec{
				ruleWithBody("gmail_labels_delete", http.MethodDelete, `^/gmail/v1/users/me/labels/[^/]+$`, "gmail_label_delete"),
			},
		},
		{
			Key:     "gmail_messages_delete",
			Group:   "Gmail",
			Title:   "Permanently delete emails",
			Summary: "Allow agents to permanently delete emails without sending them to Trash.",
			Rules: []PolicyRuleSpec{
				rule("gmail_messages_delete", http.MethodDelete, `^/gmail/v1/users/me/messages/[^/]+$`),
				rule("gmail_messages_batch_delete", http.MethodPost, `^/gmail/v1/users/me/messages/batchDelete$`),
			},
		},
		{
			Key:     "gmail_messages_import",
			Group:   "Gmail",
			Title:   "Import emails",
			Summary: "Allow agents to import or insert emails into the connected mailbox.",
			Rules: []PolicyRuleSpec{
				rule("gmail_messages_import", http.MethodPost, `^/gmail/v1/users/me/messages/import$`),
				rule("gmail_messages_insert", http.MethodPost, `^/gmail/v1/users/me/messages$`),
			},
		},
		{
			Key:     "gmail_watch_manage",
			Group:   "Gmail",
			Title:   "Manage change notifications",
			Summary: "Allow agents to start or stop push notification watches.",
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
			Key:           "calendar_events_create_self",
			Group:         "Calendar",
			Title:         "Create or import myself-only events",
			Summary:       "Allow agents to create or import new calendar events only when the request has no third-party attendees, no room/resource attendees, no additional guests, and no organizer or creator other than the connected user.",
			Rules: []PolicyRuleSpec{
				ruleWithBody("calendar_events_insert_self", http.MethodPost, `^/calendar/v3/calendars/[^/]+/events$`, "calendar_event_create_self"),
				ruleWithBody("calendar_events_import_self", http.MethodPost, `^/calendar/v3/calendars/[^/]+/events/import$`, "calendar_event_create_self"),
			},
		},
		{
			Key:     "calendar_events_create_guests",
			Group:   "Calendar",
			Title:   "Create or import events with third parties",
			Summary: "Allow agents to create or import new calendar events that include third-party attendees, room/resource attendees, additional guests, or an organizer or creator other than the connected user.",
			Rules: []PolicyRuleSpec{
				ruleWithBody("calendar_events_insert_guests", http.MethodPost, `^/calendar/v3/calendars/[^/]+/events$`, "calendar_event_create_guests"),
				ruleWithBody("calendar_events_import_guests", http.MethodPost, `^/calendar/v3/calendars/[^/]+/events/import$`, "calendar_event_create_guests"),
			},
		},
		{
			Key:     "calendar_events_update_self",
			Group:   "Calendar",
			Title:   "Update existing myself-only events",
			Summary: "Allow agents to update an existing calendar event only when both the existing event and the requested update have no third-party attendees, no room/resource attendees, no additional guests, and no organizer or creator other than the connected user.",
			Rules: []PolicyRuleSpec{
				ruleWithBody("calendar_events_patch_self", http.MethodPatch, `^/calendar/v3/calendars/[^/]+/events/[^/]+$`, "calendar_event_update_self"),
				ruleWithBody("calendar_events_update_self", http.MethodPut, `^/calendar/v3/calendars/[^/]+/events/[^/]+$`, "calendar_event_update_self"),
			},
		},
		{
			Key:     "calendar_events_update_guests",
			Group:   "Calendar",
			Title:   "Update or move events with third parties",
			Summary: "Allow agents to update calendar events involving third parties and to move events between calendars. Move operations are treated as third-party-level changes.",
			Rules: []PolicyRuleSpec{
				ruleWithBody("calendar_events_patch_guests", http.MethodPatch, `^/calendar/v3/calendars/[^/]+/events/[^/]+$`, "calendar_event_update_guests"),
				ruleWithBody("calendar_events_update_guests", http.MethodPut, `^/calendar/v3/calendars/[^/]+/events/[^/]+$`, "calendar_event_update_guests"),
				rule("calendar_events_move_guests", http.MethodPost, `^/calendar/v3/calendars/[^/]+/events/[^/]+/move$`),
			},
		},
		{
			Key:     "calendar_events_delete_self",
			Group:   "Calendar",
			Title:   "Delete existing myself-only events",
			Summary: "Allow agents to delete an existing calendar event only when it has no third-party attendees, no room/resource attendees, no additional guests, and no organizer or creator other than the connected user.",
			Rules: []PolicyRuleSpec{
				ruleWithBody("calendar_events_delete_self", http.MethodDelete, `^/calendar/v3/calendars/[^/]+/events/[^/]+$`, "calendar_event_delete_self"),
			},
		},
		{
			Key:     "calendar_events_delete_guests",
			Group:   "Calendar",
			Title:   "Delete events with third parties",
			Summary: "Allow agents to delete existing calendar events that include third-party attendees, room/resource attendees, additional guests, or an organizer or creator other than the connected user.",
			Rules: []PolicyRuleSpec{
				ruleWithBody("calendar_events_delete_guests", http.MethodDelete, `^/calendar/v3/calendars/[^/]+/events/[^/]+$`, "calendar_event_delete_guests"),
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
			Key:     "calendar_settings_read",
			Group:   "Calendar",
			Title:   "Read Calendar settings",
			Summary: "Allow agents to read Google Calendar user settings.",
			Rules: []PolicyRuleSpec{
				rule("calendar_settings_list", http.MethodGet, `^/calendar/v3/users/me/settings$`),
				rule("calendar_settings_get", http.MethodGet, `^/calendar/v3/users/me/settings/[^/]+$`),
			},
		},
		{
			Key:               "drive_files_read",
			Group:             "Files",
			Title:             "Read other file types",
			Summary:           "Allow agents to read metadata and download other file types inside allowed folders.",
			SystemDefault:     true,
			RequiresDriveRefs: true,
			Rules: []PolicyRuleSpec{
				ruleWithBody("drive_files_get", http.MethodGet, `^/drive/v3/files/[^/]+$`, "drive_file_kind_drive"),
			},
		},
		{
			Key:               "drive_files_create",
			Group:             "Files",
			Title:             "Create other file types",
			Summary:           "Allow agents to create or copy other file types inside allowed folders.",
			RequiresDriveRefs: true,
			Rules: []PolicyRuleSpec{
				ruleWithBody("drive_files_create", http.MethodPost, `^/drive/v3/files$`, "drive_file_create_drive"),
				ruleWithBody("drive_files_copy", http.MethodPost, `^/drive/v3/files/[^/]+/copy$`, "drive_file_kind_drive"),
			},
		},
		{
			Key:               "drive_files_update",
			Group:             "Files",
			Title:             "Edit other file types",
			Summary:           "Allow agents to rename, update metadata, or upload new content for other file types in allowed folders.",
			RequiresDriveRefs: true,
			Rules: []PolicyRuleSpec{
				ruleWithBody("drive_files_patch", http.MethodPatch, `^/drive/v3/files/[^/]+$`, "drive_file_kind_drive"),
				ruleWithBody("drive_files_update", http.MethodPut, `^/drive/v3/files/[^/]+$`, "drive_file_kind_drive"),
				ruleWithBody("drive_files_update_media", http.MethodPatch, `^/upload/drive/v3/files/[^/]+$`, "drive_file_kind_drive"),
			},
		},
		{
			Key:               "drive_files_delete",
			Group:             "Files",
			Title:             "Delete other file types",
			Summary:           "Allow agents to permanently delete other file types in allowed folders.",
			RequiresDriveRefs: true,
			Rules: []PolicyRuleSpec{
				ruleWithBody("drive_files_delete", http.MethodDelete, `^/drive/v3/files/[^/]+$`, "drive_file_kind_drive"),
			},
		},
		{
			Key:           "drive_permissions_read",
			Group:         "Files",
			Title:         "Read sharing",
			Summary:       "Allow agents to read file permissions without changing them.",
			SystemDefault: true,
			Rules: []PolicyRuleSpec{
				rule("drive_permissions_list", http.MethodGet, `^/drive/v3/files/[^/]+/permissions$`),
				rule("drive_permissions_get", http.MethodGet, `^/drive/v3/files/[^/]+/permissions/[^/]+$`),
			},
		},
		{
			Key:     "drive_permissions_manage",
			Group:   "Files",
			Title:   "Manage sharing",
			Summary: "Allow agents to create, update, or delete file permissions.",
			Rules: []PolicyRuleSpec{
				rule("drive_permissions_create", http.MethodPost, `^/drive/v3/files/[^/]+/permissions$`),
				rule("drive_permissions_patch", http.MethodPatch, `^/drive/v3/files/[^/]+/permissions/[^/]+$`),
				rule("drive_permissions_update", http.MethodPut, `^/drive/v3/files/[^/]+/permissions/[^/]+$`),
				rule("drive_permissions_delete", http.MethodDelete, `^/drive/v3/files/[^/]+/permissions/[^/]+$`),
			},
		},
		{
			Key:           "drive_comments_read",
			Group:         "Comments",
			Title:         "Read comments",
			Summary:       "Allow agents to read file comments and replies without changing them.",
			SystemDefault: true,
			Rules: []PolicyRuleSpec{
				rule("drive_comments_list", http.MethodGet, `^/drive/v3/files/[^/]+/comments$`),
				rule("drive_comments_get", http.MethodGet, `^/drive/v3/files/[^/]+/comments/[^/]+$`),
				rule("drive_replies_list", http.MethodGet, `^/drive/v3/files/[^/]+/comments/[^/]+/replies$`),
				rule("drive_replies_get", http.MethodGet, `^/drive/v3/files/[^/]+/comments/[^/]+/replies/[^/]+$`),
			},
		},
		{
			Key:     "drive_comments_create",
			Group:   "Comments",
			Title:   "Create comments",
			Summary: "Allow agents to create new comments on files in allowed folders.",
			Rules: []PolicyRuleSpec{
				rule("drive_comments_create", http.MethodPost, `^/drive/v3/files/[^/]+/comments$`),
			},
		},
		{
			Key:     "drive_comments_reply",
			Group:   "Comments",
			Title:   "Reply to comments",
			Summary: "Allow agents to add normal replies to existing file comments.",
			Rules: []PolicyRuleSpec{
				ruleWithBody("drive_replies_create", http.MethodPost, `^/drive/v3/files/[^/]+/comments/[^/]+/replies$`, "drive_comment_reply"),
			},
		},
		{
			Key:     "drive_comments_resolve",
			Group:   "Comments",
			Title:   "Resolve comments",
			Summary: "Allow agents to resolve comment discussions by posting a resolving reply.",
			Rules: []PolicyRuleSpec{
				ruleWithBody("drive_replies_resolve", http.MethodPost, `^/drive/v3/files/[^/]+/comments/[^/]+/replies$`, "drive_comment_resolve"),
			},
		},
		{
			Key:     "drive_comments_update",
			Group:   "Comments",
			Title:   "Update own comments and replies",
			Summary: "Allow agents to update only comments and replies authored by the connected user.",
			Rules: []PolicyRuleSpec{
				ruleWithBody("drive_comments_patch_own", http.MethodPatch, `^/drive/v3/files/[^/]+/comments/[^/]+$`, "drive_comment_owned"),
				ruleWithBody("drive_comments_update_own", http.MethodPut, `^/drive/v3/files/[^/]+/comments/[^/]+$`, "drive_comment_owned"),
				ruleWithBody("drive_replies_patch_own", http.MethodPatch, `^/drive/v3/files/[^/]+/comments/[^/]+/replies/[^/]+$`, "drive_comment_owned"),
				ruleWithBody("drive_replies_update_own", http.MethodPut, `^/drive/v3/files/[^/]+/comments/[^/]+/replies/[^/]+$`, "drive_comment_owned"),
			},
		},
		{
			Key:     "drive_comments_delete",
			Group:   "Comments",
			Title:   "Delete own comments and replies",
			Summary: "Allow agents to delete only comments and replies authored by the connected user.",
			Rules: []PolicyRuleSpec{
				ruleWithBody("drive_comments_delete_own", http.MethodDelete, `^/drive/v3/files/[^/]+/comments/[^/]+$`, "drive_comment_owned"),
				ruleWithBody("drive_replies_delete_own", http.MethodDelete, `^/drive/v3/files/[^/]+/comments/[^/]+/replies/[^/]+$`, "drive_comment_owned"),
			},
		},
		{
			Key:           "drive_metadata_read",
			Group:         "Files",
			Title:         "Read metadata",
			Summary:       "Allow agents to read file search metadata, account info, changes, shared drives, apps, file labels, and file revisions.",
			SystemDefault: true,
			Rules: []PolicyRuleSpec{
				rule("drive_changes_get_start_page_token", http.MethodGet, `^/drive/v3/changes/startPageToken$`),
				rule("drive_changes_list", http.MethodGet, `^/drive/v3/changes$`),
				rule("drive_files_list_metadata", http.MethodGet, `^/drive/v3/files$`),
				rule("drive_about_get", http.MethodGet, `^/drive/v3/about$`),
				rule("drive_drives_list", http.MethodGet, `^/drive/v3/drives$`),
				rule("drive_drives_get", http.MethodGet, `^/drive/v3/drives/[^/]+$`),
				rule("drive_apps_list", http.MethodGet, `^/drive/v3/apps$`),
				rule("drive_apps_get", http.MethodGet, `^/drive/v3/apps/[^/]+$`),
				rule("drive_file_labels_list", http.MethodGet, `^/drive/v3/files/[^/]+/listLabels$`),
				rule("drive_revisions_list", http.MethodGet, `^/drive/v3/files/[^/]+/revisions$`),
				rule("drive_revisions_get", http.MethodGet, `^/drive/v3/files/[^/]+/revisions/[^/]+$`),
			},
		},
		{
			Key:     "drive_labels_update",
			Group:   "Files",
			Title:   "Update labels",
			Summary: "Allow agents to apply, update, or remove labels on files.",
			Rules: []PolicyRuleSpec{
				rule("drive_files_modify_labels", http.MethodPost, `^/drive/v3/files/[^/]+/modifyLabels$`),
			},
		},
		{
			Key:     "drive_revisions_delete",
			Group:   "Files",
			Title:   "Delete revisions",
			Summary: "Allow agents to permanently delete file revisions where Google permits it.",
			Rules: []PolicyRuleSpec{
				rule("drive_revisions_delete", http.MethodDelete, `^/drive/v3/files/[^/]+/revisions/[^/]+$`),
			},
		},
		{
			Key:               "docs_read",
			Group:             "Docs",
			Title:             "Read Google Docs",
			Summary:           "Allow agents to read Google Docs documents located in allowed folders.",
			SystemDefault:     true,
			RequiresDriveRefs: true,
			Rules: []PolicyRuleSpec{
				rule("docs_documents_get", http.MethodGet, `^/v1/documents/[^/:]+$`),
				ruleWithBody("docs_documents_export", http.MethodGet, `^/drive/v3/files/[^/]+/export$`, "drive_file_kind_docs"),
			},
		},
		{
			Key:               "docs_create",
			Group:             "Docs",
			Title:             "Create Google Docs",
			Summary:           "Allow agents to create Google Docs documents in allowed folders.",
			RequiresDriveRefs: true,
			Rules: []PolicyRuleSpec{
				rule("docs_documents_create", http.MethodPost, `^/v1/documents$`),
				ruleWithBody("docs_documents_create_via_drive", http.MethodPost, `^/drive/v3/files$`, "drive_file_create_docs"),
			},
		},
		{
			Key:               "docs_edit",
			Group:             "Docs",
			Title:             "Edit Google Docs",
			Summary:           "Allow agents to modify Google Docs documents in allowed folders.",
			RequiresDriveRefs: true,
			Rules: []PolicyRuleSpec{
				rule("docs_documents_batch_update", http.MethodPost, `^/v1/documents/[^/:]+:batchUpdate$`),
			},
		},
		{
			Key:               "docs_delete",
			Group:             "Docs",
			Title:             "Delete Google Docs",
			Summary:           "Allow agents to permanently delete Google Docs documents in allowed folders.",
			RequiresDriveRefs: true,
			Rules: []PolicyRuleSpec{
				ruleWithBody("docs_documents_delete", http.MethodDelete, `^/drive/v3/files/[^/]+$`, "drive_file_kind_docs"),
			},
		},
		{
			Key:               "sheets_read",
			Group:             "Sheets",
			Title:             "Read Google Sheets",
			Summary:           "Allow agents to read Google Sheets spreadsheets and cell values in allowed folders.",
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
				ruleWithBody("sheets_spreadsheets_export", http.MethodGet, `^/drive/v3/files/[^/]+/export$`, "drive_file_kind_sheets"),
			},
		},
		{
			Key:               "sheets_create",
			Group:             "Sheets",
			Title:             "Create Google Sheets",
			Summary:           "Allow agents to create Google Sheets spreadsheets in allowed folders.",
			RequiresDriveRefs: true,
			Rules: []PolicyRuleSpec{
				rule("sheets_spreadsheets_create", http.MethodPost, `^/v4/spreadsheets$`),
				ruleWithBody("sheets_spreadsheets_create_via_drive", http.MethodPost, `^/drive/v3/files$`, "drive_file_create_sheets"),
			},
		},
		{
			Key:               "sheets_edit",
			Group:             "Sheets",
			Title:             "Edit Google Sheets",
			Summary:           "Allow agents to update spreadsheet structure, metadata, and cell values in allowed folders.",
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
			Key:               "sheets_delete",
			Group:             "Sheets",
			Title:             "Delete Google Sheets",
			Summary:           "Allow agents to permanently delete Google Sheets spreadsheets in allowed folders.",
			RequiresDriveRefs: true,
			Rules: []PolicyRuleSpec{
				ruleWithBody("sheets_spreadsheets_delete", http.MethodDelete, `^/drive/v3/files/[^/]+$`, "drive_file_kind_sheets"),
			},
		},
		{
			Key:               "slides_read",
			Group:             "Slides",
			Title:             "Read Google Slides",
			Summary:           "Allow agents to read Google Slides presentations and pages in allowed folders.",
			SystemDefault:     true,
			RequiresDriveRefs: true,
			Rules: []PolicyRuleSpec{
				rule("slides_presentations_get", http.MethodGet, `^/v1/presentations/[^/]+$`),
				rule("slides_pages_get", http.MethodGet, `^/v1/presentations/[^/]+/pages/[^/]+$`),
				rule("slides_pages_get_thumbnail", http.MethodGet, `^/v1/presentations/[^/]+/pages/[^/]+/thumbnail$`),
				ruleWithBody("slides_presentations_export", http.MethodGet, `^/drive/v3/files/[^/]+/export$`, "drive_file_kind_slides"),
			},
		},
		{
			Key:               "slides_create",
			Group:             "Slides",
			Title:             "Create Google Slides",
			Summary:           "Allow agents to create Google Slides presentations in allowed folders.",
			RequiresDriveRefs: true,
			Rules: []PolicyRuleSpec{
				rule("slides_presentations_create", http.MethodPost, `^/v1/presentations$`),
				ruleWithBody("slides_presentations_create_via_drive", http.MethodPost, `^/drive/v3/files$`, "drive_file_create_slides"),
			},
		},
		{
			Key:               "slides_edit",
			Group:             "Slides",
			Title:             "Edit Google Slides",
			Summary:           "Allow agents to modify Google Slides presentations in allowed folders.",
			RequiresDriveRefs: true,
			Rules: []PolicyRuleSpec{
				rule("slides_presentations_batch_update", http.MethodPost, `^/v1/presentations/[^/]+:batchUpdate$`),
			},
		},
		{
			Key:               "slides_delete",
			Group:             "Slides",
			Title:             "Delete Google Slides",
			Summary:           "Allow agents to permanently delete Google Slides presentations in allowed folders.",
			RequiresDriveRefs: true,
			Rules: []PolicyRuleSpec{
				ruleWithBody("slides_presentations_delete", http.MethodDelete, `^/drive/v3/files/[^/]+$`, "drive_file_kind_slides"),
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
	return NewPolicyEngineWithReview(capabilityKeys, nil)
}

func NewPolicyEngineWithReview(capabilityKeys []string, reviewRequiredKeys []string) (*PolicyEngine, error) {
	allowed := sliceToSet(capabilityKeys)
	catalog := PolicyCatalog()
	engine := &PolicyEngine{
		rules:                  []compiledRule{},
		bodyPolicies:           defaultBodyPolicies(allowed),
		reviewRequiredPolicies: map[string]bool{},
	}
	for _, key := range reviewRequiredKeys {
		if allowed[key] {
			engine.reviewRequiredPolicies[key] = true
		}
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
			engine.rules = append(engine.rules, compiledRule{spec: spec, rx: rx, capabilityKey: cap.Key, capabilityTitle: cap.Title})
		}
	}
	return engine, nil
}

func defaultBodyPolicies(allowedCapabilities map[string]bool) map[string]BodyPolicyEvaluator {
	return map[string]BodyPolicyEvaluator{
		"gmail_label_modify": func(ctx PolicyEvalContext, method, normalizedPath string, body []byte) (bool, string, error) {
			if err := validateGmailLabelModifyPolicy(allowedCapabilities, normalizedPath, body); err != nil {
				return false, err.Error(), nil
			}
			return true, "", nil
		},
		"gmail_label_create_rename": func(ctx PolicyEvalContext, method, normalizedPath string, body []byte) (bool, string, error) {
			if err := validateGmailLabelCreateRenamePolicy(normalizedPath, body); err != nil {
				return false, err.Error(), nil
			}
			return true, "", nil
		},
		"gmail_label_delete": func(ctx PolicyEvalContext, method, normalizedPath string, body []byte) (bool, string, error) {
			if err := validateGmailLabelDeletePolicy(normalizedPath); err != nil {
				return false, err.Error(), nil
			}
			return true, "", nil
		},
		"calendar_event_create_self": func(ctx PolicyEvalContext, method, normalizedPath string, body []byte) (bool, string, error) {
			return evaluateCalendarEventCreatePolicy(ctx, body, false)
		},
		"calendar_event_create_guests": func(ctx PolicyEvalContext, method, normalizedPath string, body []byte) (bool, string, error) {
			return evaluateCalendarEventCreatePolicy(ctx, body, true)
		},
		"calendar_event_update_self": func(ctx PolicyEvalContext, method, normalizedPath string, body []byte) (bool, string, error) {
			return evaluateCalendarEventExistingPolicy(ctx, normalizedPath, body, false)
		},
		"calendar_event_update_guests": func(ctx PolicyEvalContext, method, normalizedPath string, body []byte) (bool, string, error) {
			return evaluateCalendarEventExistingPolicy(ctx, normalizedPath, body, true)
		},
		"calendar_event_delete_self": func(ctx PolicyEvalContext, method, normalizedPath string, body []byte) (bool, string, error) {
			return evaluateCalendarEventExistingPolicy(ctx, normalizedPath, nil, false)
		},
		"calendar_event_delete_guests": func(ctx PolicyEvalContext, method, normalizedPath string, body []byte) (bool, string, error) {
			return evaluateCalendarEventExistingPolicy(ctx, normalizedPath, nil, true)
		},
		"drive_comment_reply": func(ctx PolicyEvalContext, method, normalizedPath string, body []byte) (bool, string, error) {
			return evaluateDriveReplyActionPolicy(body, false)
		},
		"drive_comment_resolve": func(ctx PolicyEvalContext, method, normalizedPath string, body []byte) (bool, string, error) {
			return evaluateDriveReplyActionPolicy(body, true)
		},
		"drive_comment_owned": func(ctx PolicyEvalContext, method, normalizedPath string, body []byte) (bool, string, error) {
			return evaluateDriveCommentOwnershipPolicy(ctx, method, normalizedPath, body)
		},
		"drive_file_create_drive": func(ctx PolicyEvalContext, method, normalizedPath string, body []byte) (bool, string, error) {
			return evaluateDriveFileCreateKindPolicy(body, "drive")
		},
		"drive_file_create_docs": func(ctx PolicyEvalContext, method, normalizedPath string, body []byte) (bool, string, error) {
			return evaluateDriveFileCreateKindPolicy(body, "docs")
		},
		"drive_file_create_sheets": func(ctx PolicyEvalContext, method, normalizedPath string, body []byte) (bool, string, error) {
			return evaluateDriveFileCreateKindPolicy(body, "sheets")
		},
		"drive_file_create_slides": func(ctx PolicyEvalContext, method, normalizedPath string, body []byte) (bool, string, error) {
			return evaluateDriveFileCreateKindPolicy(body, "slides")
		},
		"drive_file_kind_drive": func(ctx PolicyEvalContext, method, normalizedPath string, body []byte) (bool, string, error) {
			return evaluateDriveFileKindPolicy(ctx, normalizedPath, "drive")
		},
		"drive_file_kind_docs": func(ctx PolicyEvalContext, method, normalizedPath string, body []byte) (bool, string, error) {
			return evaluateDriveFileKindPolicy(ctx, normalizedPath, "docs")
		},
		"drive_file_kind_sheets": func(ctx PolicyEvalContext, method, normalizedPath string, body []byte) (bool, string, error) {
			return evaluateDriveFileKindPolicy(ctx, normalizedPath, "sheets")
		},
		"drive_file_kind_slides": func(ctx PolicyEvalContext, method, normalizedPath string, body []byte) (bool, string, error) {
			return evaluateDriveFileKindPolicy(ctx, normalizedPath, "slides")
		},
	}
}

var errPolicyRuleNoMatch = errors.New("policy rule does not match request details")

func (p *PolicyEngine) Evaluate(method, normalizedPath string, body []byte, ctx PolicyEvalContext) (bool, string, error) {
	decision, err := p.EvaluateDecision(method, normalizedPath, body, ctx)
	if err != nil {
		return false, "", err
	}
	return decision.Allowed, decision.Reason, nil
}

func (p *PolicyEngine) EvaluateDecision(method, normalizedPath string, body []byte, ctx PolicyEvalContext) (PolicyDecision, error) {
	noMatchReason := ""
	for _, rule := range p.rules {
		if rule.spec.Method != method {
			continue
		}
		if !rule.rx.MatchString(normalizedPath) {
			continue
		}
		if rule.spec.BodyPolicy == "" {
			return p.allowedPolicyDecision(rule), nil
		}
		evaluate, ok := p.bodyPolicies[rule.spec.BodyPolicy]
		if !ok {
			return PolicyDecision{}, fmt.Errorf("missing body policy %q", rule.spec.BodyPolicy)
		}
		allowed, reason, err := evaluate(ctx, method, normalizedPath, body)
		if err != nil {
			if errors.Is(err, errPolicyRuleNoMatch) {
				if reason != "" {
					noMatchReason = reason
				}
				continue
			}
			return PolicyDecision{}, err
		}
		if !allowed {
			return PolicyDecision{Allowed: false, Reason: reason}, nil
		}
		return p.allowedPolicyDecision(rule), nil
	}
	if noMatchReason != "" {
		return PolicyDecision{Allowed: false, Reason: noMatchReason}, nil
	}
	return PolicyDecision{Allowed: false, Reason: "no policy capability allowed this operation"}, nil
}

func (p *PolicyEngine) allowedPolicyDecision(rule compiledRule) PolicyDecision {
	return PolicyDecision{
		Allowed:               true,
		Reason:                rule.spec.Name,
		CapabilityKey:         rule.capabilityKey,
		CapabilityTitle:       rule.capabilityTitle,
		RuleName:              rule.spec.Name,
		RequiresHumanApproval: p.reviewRequiredPolicies[rule.capabilityKey],
	}
}

func validateGmailLabelModifyPolicy(allowedCapabilities map[string]bool, normalizedPath string, body []byte) error {
	if len(body) == 0 {
		return fmt.Errorf("request body is required")
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return fmt.Errorf("request body must be valid JSON: %w", err)
	}
	allowedKeys := map[string]bool{"addLabelIds": true, "removeLabelIds": true}
	if normalizedPath == "/gmail/v1/users/me/messages/batchModify" {
		allowedKeys["ids"] = true
	}
	for key := range raw {
		if !allowedKeys[key] {
			return fmt.Errorf("body key %q is not allowed", key)
		}
	}
	var payload labelModifyBody
	if err := json.Unmarshal(body, &payload); err != nil {
		return fmt.Errorf("invalid label modify payload: %w", err)
	}
	required := map[string]string{}
	for _, label := range payload.AddLabelIDs {
		capability, description, err := gmailLabelEffectCapability(label)
		if err != nil {
			return err
		}
		required[capability] = description
	}
	for _, label := range payload.RemoveLabelIDs {
		capability, description, err := gmailLabelEffectCapability(label)
		if err != nil {
			return err
		}
		required[capability] = description
	}
	if len(required) == 0 {
		return fmt.Errorf("request must add or remove at least one supported label")
	}
	missing := make([]string, 0)
	for capability, description := range required {
		if !allowedCapabilities[capability] {
			missing = append(missing, fmt.Sprintf("%s (%s)", capability, description))
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		return fmt.Errorf("label change requires disabled policy permission: %s", strings.Join(missing, ", "))
	}
	return nil
}

func validateGmailLabelCreateRenamePolicy(normalizedPath string, body []byte) error {
	if normalizedPath != "/gmail/v1/users/me/labels" {
		labelID, err := gmailLabelIDFromPath(normalizedPath)
		if err != nil {
			return err
		}
		if !isGmailCustomLabelID(labelID) {
			return fmt.Errorf("system label %q cannot be renamed by policy", labelID)
		}
	}
	if len(body) == 0 {
		return fmt.Errorf("request body is required")
	}
	var payload gmailLabelBody
	if err := json.Unmarshal(body, &payload); err != nil {
		return fmt.Errorf("invalid label payload: %w", err)
	}
	if isReservedGmailLabelName(payload.Name) {
		return fmt.Errorf("label name %q is reserved by Gmail", payload.Name)
	}
	return nil
}

func validateGmailLabelDeletePolicy(normalizedPath string) error {
	labelID, err := gmailLabelIDFromPath(normalizedPath)
	if err != nil {
		return err
	}
	if !isGmailCustomLabelID(labelID) {
		return fmt.Errorf("system label %q cannot be deleted by policy", labelID)
	}
	return nil
}

func gmailLabelEffectCapability(labelID string) (string, string, error) {
	switch strings.ToUpper(strings.TrimSpace(labelID)) {
	case "INBOX":
		return "gmail_messages_archive", "archive or unarchive emails", nil
	case "SPAM":
		return "gmail_messages_spam", "move emails to or from Spam", nil
	case "TRASH":
		return "gmail_messages_trash", "move emails to or from Trash", nil
	case "UNREAD", "STARRED", "IMPORTANT":
		return "gmail_messages_status", "change email status", nil
	}
	if isGmailCustomLabelID(labelID) {
		return "gmail_labels_apply_custom", "apply or remove custom labels", nil
	}
	if isGmailSystemLabelID(labelID) {
		return "", "", fmt.Errorf("system label %q is not supported by any policy permission", labelID)
	}
	return "", "", fmt.Errorf("label %q is not a supported Gmail label ID", labelID)
}

func gmailLabelIDFromPath(normalizedPath string) (string, error) {
	prefix := "/gmail/v1/users/me/labels/"
	if !strings.HasPrefix(normalizedPath, prefix) {
		return "", fmt.Errorf("invalid Gmail label path")
	}
	labelID := strings.TrimPrefix(normalizedPath, prefix)
	if labelID == "" || strings.Contains(labelID, "/") {
		return "", fmt.Errorf("invalid Gmail label ID")
	}
	return labelID, nil
}

func isGmailCustomLabelID(labelID string) bool {
	return strings.HasPrefix(strings.TrimSpace(labelID), "Label_")
}

func isGmailSystemLabelID(labelID string) bool {
	labelID = strings.ToUpper(strings.TrimSpace(labelID))
	if strings.HasPrefix(labelID, "CATEGORY_") {
		return true
	}
	return gmailReservedSystemLabels[labelID]
}

func isReservedGmailLabelName(name string) bool {
	name = strings.ToUpper(strings.TrimSpace(name))
	if strings.HasPrefix(name, "CATEGORY_") {
		return true
	}
	return gmailReservedSystemLabels[name]
}

var gmailReservedSystemLabels = map[string]bool{
	"INBOX":     true,
	"SPAM":      true,
	"TRASH":     true,
	"UNREAD":    true,
	"STARRED":   true,
	"IMPORTANT": true,
	"SENT":      true,
	"DRAFT":     true,
	"CHAT":      true,
}

func evaluateCalendarEventCreatePolicy(ctx PolicyEvalContext, body []byte, requireThirdParty bool) (bool, string, error) {
	event, err := parseCalendarEventBody(body)
	if err != nil {
		return false, err.Error(), nil
	}
	return matchCalendarThirdPartyPolicy(calendarEventHasThirdParty(ctx.MailboxEmail, event), requireThirdParty)
}

func evaluateCalendarEventExistingPolicy(ctx PolicyEvalContext, normalizedPath string, body []byte, requireThirdParty bool) (bool, string, error) {
	if ctx.FetchCalendarEvent == nil {
		return false, "calendar event inspection is unavailable", nil
	}
	calendarID, eventID, err := extractCalendarEventPathIDs(normalizedPath)
	if err != nil {
		return false, err.Error(), nil
	}
	existing, err := ctx.FetchCalendarEvent(calendarID, eventID)
	if err != nil {
		return false, "unable to inspect existing calendar event: " + err.Error(), nil
	}
	hasThirdParty := calendarEventHasThirdParty(ctx.MailboxEmail, existing)
	if len(body) > 0 {
		requested, err := parseCalendarEventBody(body)
		if err != nil {
			return false, err.Error(), nil
		}
		hasThirdParty = hasThirdParty || calendarEventHasThirdParty(ctx.MailboxEmail, requested)
	}
	return matchCalendarThirdPartyPolicy(hasThirdParty, requireThirdParty)
}

func matchCalendarThirdPartyPolicy(hasThirdParty, requireThirdParty bool) (bool, string, error) {
	if hasThirdParty == requireThirdParty {
		return true, "", nil
	}
	if requireThirdParty {
		return false, "calendar event is myself-only; enable the myself-only calendar event permission", errPolicyRuleNoMatch
	}
	return false, "calendar event involves third parties; enable the third-party calendar event permission", errPolicyRuleNoMatch
}

func evaluateDriveReplyActionPolicy(body []byte, requireResolve bool) (bool, string, error) {
	payload, err := parseJSONBody(body, "Drive reply body")
	if err != nil {
		return false, err.Error(), nil
	}
	action := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", payload["action"])))
	if action == "<nil>" {
		action = ""
	}
	if action == "resolve" {
		if requireResolve {
			return true, "", nil
		}
		return false, "reply resolves the comment; enable the resolve comments permission", errPolicyRuleNoMatch
	}
	if action != "" {
		return false, fmt.Sprintf("reply action %q is not allowed by this policy", action), nil
	}
	if requireResolve {
		return false, "reply does not resolve the comment; enable the reply to comments permission", errPolicyRuleNoMatch
	}
	return true, "", nil
}

func evaluateDriveCommentOwnershipPolicy(ctx PolicyEvalContext, method, normalizedPath string, body []byte) (bool, string, error) {
	if ctx.FetchDriveCommentAuthorMe == nil {
		return false, "comment ownership inspection is unavailable", nil
	}
	if method == http.MethodPatch || method == http.MethodPut {
		payload, err := parseJSONBody(body, "comment update body")
		if err != nil {
			return false, err.Error(), nil
		}
		if _, ok := payload["action"]; ok {
			return false, "comment update cannot perform reply actions such as resolve", nil
		}
		if _, ok := payload["resolved"]; ok {
			return false, "comment update cannot change resolved state; enable the resolve comments permission", nil
		}
	}
	fileID, commentID, replyID, err := extractDriveCommentPathIDs(normalizedPath)
	if err != nil {
		return false, err.Error(), nil
	}
	authorMe, err := ctx.FetchDriveCommentAuthorMe(fileID, commentID, replyID)
	if err != nil {
		return false, "unable to inspect comment ownership: " + err.Error(), nil
	}
	if !authorMe {
		return false, "comment or reply was not authored by the connected user", nil
	}
	return true, "", nil
}

func evaluateDriveFileCreateKindPolicy(body []byte, wantKind string) (bool, string, error) {
	payload, err := parseJSONBody(body, "Drive file create body")
	if err != nil {
		return false, err.Error(), nil
	}
	mime := strings.TrimSpace(fmt.Sprintf("%v", payload["mimeType"]))
	if mime == "<nil>" {
		mime = ""
	}
	gotKind := drivePolicyKindFromMime(mime)
	return matchDriveFileKindPolicy(gotKind, wantKind)
}

func evaluateDriveFileKindPolicy(ctx PolicyEvalContext, normalizedPath, wantKind string) (bool, string, error) {
	if ctx.FetchDriveFileKind == nil {
		return false, "Drive file type inspection is unavailable", nil
	}
	fileID, err := extractDriveFilePathID(normalizedPath)
	if err != nil {
		return false, err.Error(), nil
	}
	if normalizedPath == "/drive/v3/files/trash" {
		return false, "emptying Drive trash is not allowed by this policy", nil
	}
	gotKind, err := ctx.FetchDriveFileKind(fileID)
	if err != nil {
		return false, "unable to inspect Drive file type: " + err.Error(), nil
	}
	return matchDriveFileKindPolicy(gotKind, wantKind)
}

func matchDriveFileKindPolicy(gotKind, wantKind string) (bool, string, error) {
	if gotKind == wantKind {
		return true, "", nil
	}
	return false, driveFileKindMismatchReason(gotKind, wantKind), errPolicyRuleNoMatch
}

func driveFileKindMismatchReason(gotKind, wantKind string) string {
	if gotKind == "google_native" {
		return fmt.Sprintf("target file is a Google-native file type not covered by %s", drivePolicyKindPermissionName(wantKind))
	}
	return fmt.Sprintf("target file is %s; enable %s", drivePolicyKindLabel(gotKind), drivePolicyKindPermissionName(gotKind))
}

func drivePolicyKindPermissionName(kind string) string {
	switch kind {
	case "docs":
		return "the Google Docs permission"
	case "sheets":
		return "the Google Sheets permission"
	case "slides":
		return "the Google Slides permission"
	case "drive":
		return "the Other file types permission"
	default:
		return "a matching Google-native file permission"
	}
}

func drivePolicyKindLabel(kind string) string {
	switch kind {
	case "docs":
		return "a Google Docs document"
	case "sheets":
		return "a Google Sheets spreadsheet"
	case "slides":
		return "a Google Slides presentation"
	case "drive":
		return "a file covered by Other file types"
	default:
		return "a Google-native file"
	}
}

func extractDriveFilePathID(normalizedPath string) (string, error) {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`^/drive/v3/files/([^/]+)(?:/(?:copy|export))?$`),
		regexp.MustCompile(`^/upload/drive/v3/files/([^/]+)$`),
	}
	for _, rx := range patterns {
		matches := rx.FindStringSubmatch(normalizedPath)
		if len(matches) != 2 {
			continue
		}
		fileID, err := url.PathUnescape(matches[1])
		if err != nil {
			return "", fmt.Errorf("invalid Drive file ID in path")
		}
		return fileID, nil
	}
	return "", fmt.Errorf("Drive file path does not include a file ID")
}

func extractDriveCommentPathIDs(normalizedPath string) (fileID, commentID, replyID string, err error) {
	rx := regexp.MustCompile(`^/drive/v3/files/([^/]+)/comments/([^/]+)(?:/replies/([^/]+))?$`)
	matches := rx.FindStringSubmatch(normalizedPath)
	if len(matches) != 4 {
		return "", "", "", fmt.Errorf("Drive comment path does not include file and comment IDs")
	}
	fileID, err = url.PathUnescape(matches[1])
	if err != nil {
		return "", "", "", fmt.Errorf("invalid Drive file ID in path")
	}
	commentID, err = url.PathUnescape(matches[2])
	if err != nil {
		return "", "", "", fmt.Errorf("invalid Drive comment ID in path")
	}
	if matches[3] != "" {
		replyID, err = url.PathUnescape(matches[3])
		if err != nil {
			return "", "", "", fmt.Errorf("invalid Drive reply ID in path")
		}
	}
	return fileID, commentID, replyID, nil
}

func parseCalendarEventBody(body []byte) (map[string]any, error) {
	return parseJSONBody(body, "calendar event body")
}

func parseJSONBody(body []byte, label string) (map[string]any, error) {
	if len(body) == 0 {
		return map[string]any{}, nil
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("%s must be valid JSON: %w", label, err)
	}
	return payload, nil
}

func extractCalendarEventPathIDs(normalizedPath string) (string, string, error) {
	rx := regexp.MustCompile(`^/calendar/v3/calendars/([^/]+)/events/([^/]+)(?:/move)?$`)
	matches := rx.FindStringSubmatch(normalizedPath)
	if len(matches) != 3 {
		return "", "", fmt.Errorf("calendar event path does not include calendar and event IDs")
	}
	calendarID, err := url.PathUnescape(matches[1])
	if err != nil {
		return "", "", fmt.Errorf("invalid calendar ID in path")
	}
	eventID, err := url.PathUnescape(matches[2])
	if err != nil {
		return "", "", fmt.Errorf("invalid event ID in path")
	}
	return calendarID, eventID, nil
}

func calendarEventHasThirdParty(mailboxEmail string, event map[string]any) bool {
	mailboxEmail = normalizeEmail(mailboxEmail)
	if calendarPersonIsThirdParty(mailboxEmail, event["organizer"]) {
		return true
	}
	if calendarPersonIsThirdParty(mailboxEmail, event["creator"]) {
		return true
	}
	attendees, ok := event["attendees"].([]any)
	if !ok {
		return false
	}
	for _, raw := range attendees {
		attendee, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if boolFromAny(attendee["resource"]) {
			return true
		}
		if numberFromAny(attendee["additionalGuests"]) > 0 {
			return true
		}
		email := normalizeEmail(fmt.Sprintf("%v", attendee["email"]))
		if email != "" && email != "<nil>" && email != mailboxEmail {
			return true
		}
	}
	return false
}

func calendarPersonIsThirdParty(mailboxEmail string, raw any) bool {
	person, ok := raw.(map[string]any)
	if !ok {
		return false
	}
	email := normalizeEmail(fmt.Sprintf("%v", person["email"]))
	return email != "" && email != "<nil>" && email != mailboxEmail
}

func boolFromAny(raw any) bool {
	value, ok := raw.(bool)
	return ok && value
}

func numberFromAny(raw any) float64 {
	switch value := raw.(type) {
	case float64:
		return value
	case float32:
		return float64(value)
	case int:
		return float64(value)
	case int64:
		return float64(value)
	default:
		return 0
	}
}

func PolicyCapabilitiesForSelection(selected map[string]bool, reviewRequiredArg ...map[string]bool) []map[string]any {
	reviewRequired := map[string]bool{}
	if len(reviewRequiredArg) > 0 && reviewRequiredArg[0] != nil {
		reviewRequired = reviewRequiredArg[0]
	}
	catalog := PolicyCatalog()
	out := make([]map[string]any, 0, len(catalog))
	for _, cap := range catalog {
		out = append(out, policyCapabilityView(cap, selected, reviewRequired))
	}
	return out
}

func PolicyCapabilityGroupsForSelection(selected map[string]bool, reviewRequiredArg ...map[string]bool) []map[string]any {
	reviewRequired := map[string]bool{}
	if len(reviewRequiredArg) > 0 && reviewRequiredArg[0] != nil {
		reviewRequired = reviewRequiredArg[0]
	}
	groupOrder := []string{"Calendar", "Contacts", "Drive", "Gmail"}
	grouped := map[string]map[string][]map[string]any{}
	for _, cap := range PolicyCatalog() {
		app := policyApplicationGroup(cap.Group)
		subgroup := policyCapabilitySubgroup(cap)
		if grouped[app] == nil {
			grouped[app] = map[string][]map[string]any{}
		}
		grouped[app][subgroup] = append(grouped[app][subgroup], policyCapabilityView(cap, selected, reviewRequired))
	}
	out := make([]map[string]any, 0, len(groupOrder))
	for _, name := range groupOrder {
		subgroups := policyCapabilitySubgroupViews(name, grouped[name])
		out = append(out, map[string]any{
			"Name":          name,
			"ShowSubgroups": len(subgroups) > 1,
			"Subgroups":     subgroups,
		})
	}
	return out
}

func policyCapabilitySubgroupViews(app string, grouped map[string][]map[string]any) []map[string]any {
	order := policyCapabilitySubgroupOrder(app, grouped)
	out := make([]map[string]any, 0, len(order))
	for _, name := range order {
		capabilities := grouped[name]
		sort.SliceStable(capabilities, func(i, j int) bool {
			leftRisk := capabilities[i]["RiskScore"].(int)
			rightRisk := capabilities[j]["RiskScore"].(int)
			if leftRisk != rightRisk {
				return leftRisk < rightRisk
			}
			leftTitle := strings.ToLower(capabilities[i]["Title"].(string))
			rightTitle := strings.ToLower(capabilities[j]["Title"].(string))
			return leftTitle < rightTitle
		})
		out = append(out, map[string]any{
			"Name":         name,
			"Capabilities": capabilities,
		})
	}
	return out
}

func policyCapabilitySubgroupOrder(app string, grouped map[string][]map[string]any) []string {
	if app == "Calendar" {
		return []string{"Events Operations", "Calendars Operations"}
	}
	if app == "Gmail" {
		return orderedPolicySubgroups(grouped, []string{"Emails", "Labels", "Mailbox"})
	}
	out := make([]string, 0, len(grouped))
	for name := range grouped {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func orderedPolicySubgroups(grouped map[string][]map[string]any, preferred []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(grouped))
	for _, name := range preferred {
		if _, ok := grouped[name]; ok {
			out = append(out, name)
			seen[name] = true
		}
	}
	remaining := make([]string, 0)
	for name := range grouped {
		if !seen[name] {
			remaining = append(remaining, name)
		}
	}
	sort.Strings(remaining)
	return append(out, remaining...)
}

func policyCapabilitySubgroup(cap PolicyCapability) string {
	switch policyApplicationGroup(cap.Group) {
	case "Calendar":
		switch {
		case cap.Key == "calendar_read", strings.HasPrefix(cap.Key, "calendar_events_"):
			return "Events Operations"
		default:
			return "Calendars Operations"
		}
	case "Gmail":
		switch cap.Key {
		case "gmail_messages_read", "gmail_drafts_read", "gmail_drafts_write", "gmail_drafts_delete", "gmail_send", "gmail_messages_archive", "gmail_messages_status", "gmail_messages_spam", "gmail_messages_trash", "gmail_messages_delete", "gmail_messages_import":
			return "Emails"
		case "gmail_labels_read", "gmail_labels_apply_custom", "gmail_labels_create_rename", "gmail_labels_delete":
			return "Labels"
		case "gmail_profile_read", "gmail_watch_manage":
			return "Mailbox"
		default:
			return "Mailbox"
		}
	}
	return cap.Group
}

func policyCapabilityView(cap PolicyCapability, selected, reviewRequired map[string]bool) map[string]any {
	riskScore := policyRiskScore(cap.Key)
	return map[string]any{
		"Key":               cap.Key,
		"Group":             cap.Group,
		"Title":             cap.Title,
		"Summary":           cap.Summary,
		"Checked":           selected[cap.Key],
		"ReviewRequired":    selected[cap.Key] && reviewRequired[cap.Key],
		"SystemDefault":     cap.SystemDefault,
		"RequiresDriveRefs": cap.RequiresDriveRefs,
		"RiskScore":         riskScore,
		"RiskLabel":         policyRiskLabel(riskScore),
		"RiskClass":         policyRiskClass(riskScore),
		"RiskDescription":   policyRiskDescription(cap.Key),
	}
}

// Risk scoring criteria are documented in docs/policy-risk-scoring.md.
// Every PolicyCatalog capability must have an explicit score and explanation.
var policyCapabilityRiskScores = map[string]int{
	"gmail_messages_read":         1,
	"gmail_drafts_read":           1,
	"gmail_profile_read":          1,
	"gmail_labels_read":           1,
	"calendar_read":               1,
	"calendar_access_read":        1,
	"calendar_events_create_self": 1,
	"drive_files_read":            1,
	"drive_permissions_read":      1,
	"drive_comments_read":         1,
	"drive_metadata_read":         1,
	"docs_read":                   1,
	"sheets_read":                 1,
	"slides_read":                 1,

	"contacts_read":               1,
	"gmail_labels_apply_custom":   2,
	"gmail_messages_archive":      2,
	"gmail_messages_status":       2,
	"gmail_drafts_write":          2,
	"gmail_labels_create_rename":  2,
	"calendar_events_update_self": 2,
	"calendar_settings_read":      2,
	"drive_files_create":          2,
	"drive_files_update":          2,
	"drive_comments_create":       2,
	"drive_comments_reply":        2,
	"drive_comments_update":       2,
	"drive_labels_update":         2,
	"docs_create":                 2,
	"docs_edit":                   2,
	"sheets_create":               2,
	"slides_create":               2,
	"slides_edit":                 2,

	"gmail_drafts_delete":           3,
	"gmail_send":                    3,
	"gmail_messages_spam":           3,
	"gmail_messages_trash":          3,
	"gmail_labels_delete":           3,
	"gmail_messages_delete":         3,
	"gmail_messages_import":         3,
	"gmail_watch_manage":            3,
	"calendar_events_create_guests": 3,
	"calendar_events_update_guests": 3,
	"calendar_events_delete_self":   3,
	"calendar_events_delete_guests": 3,
	"calendar_calendars_manage":     3,
	"calendar_access_manage":        3,
	"drive_files_delete":            3,
	"drive_permissions_manage":      3,
	"drive_comments_resolve":        3,
	"drive_comments_delete":         3,
	"drive_revisions_delete":        3,
	"docs_delete":                   3,
	"sheets_delete":                 3,
	"sheets_edit":                   3,
	"slides_delete":                 3,
}

var policyCapabilityRiskDescriptions = map[string]string{
	"gmail_profile_read":  "Low because it reads mailbox profile and history metadata only. It does not read email bodies, change mail, or contact anyone.",
	"gmail_messages_read": "Low because it can read user emails and attachments, but it does not change emails or send anything to contacts.",
	"gmail_drafts_read":   "Low because it only reads existing drafts. It cannot modify, delete, or send those drafts.",
	"gmail_labels_read":   "Low because it only reads label definitions. It does not apply, remove, create, rename, or delete labels.",

	"gmail_drafts_write":         "Medium because it can create and update drafts. Drafts are not sent without a separate send permission, and this permission cannot delete drafts.",
	"gmail_drafts_delete":        "High because it can delete user-written draft content. Deletion can lose unsent work before the user has reviewed or sent it.",
	"gmail_send":                 "High because it can send email from the user's mailbox. Misuse directly contacts third parties and can create business or personal impact.",
	"gmail_labels_apply_custom":  "Medium because it changes how existing emails are organized by applying or removing user-created labels. It does not change system labels, delete emails, or contact anyone.",
	"gmail_messages_archive":     "Medium because it can move emails in or out of Inbox without deleting them. The original emails remain retained in the mailbox.",
	"gmail_messages_status":      "Medium because it changes visible email state such as read/unread, starred, or important. It does not delete emails, send mail, or contact anyone.",
	"gmail_messages_spam":        "High because moving emails to or from Spam can hide important mail or affect how spam filtering treats similar mail in the future.",
	"gmail_messages_trash":       "High because moving emails to Trash is a deletion-like operation. Even when recoverable, it can hide or remove important records from normal mailbox views.",
	"gmail_labels_create_rename": "Medium because it changes mailbox organization by creating or renaming user labels. It does not delete labels or emails.",
	"gmail_labels_delete":        "High because deleting a label is immediate and permanent, and removes that organization marker from every email and thread using it.",
	"gmail_messages_delete":      "High because it can permanently delete or batch-delete emails without Trash recovery. Deletion can remove important records.",
	"gmail_messages_import":      "High because it can insert or import emails into the mailbox. Misuse can pollute records or create misleading mailbox history.",
	"gmail_watch_manage":         "High because it can configure Gmail change notifications. Misuse could expose future mailbox activity to an unintended notification channel.",
	"contacts_read":              "Low because it only searches contact and directory metadata needed to resolve recipients. It does not create, update, delete, invite, notify, or send anything.",

	"calendar_read":                 "Low because it only reads calendars, events, instances, and free/busy data. It does not create, edit, delete, invite, or notify anyone.",
	"calendar_events_create_self":   "Low because it only creates or imports new events with no third-party attendees, no room/resource attendees, no additional guests, and no organizer or creator other than the connected user. It does not alter existing events or contact third parties.",
	"calendar_events_create_guests": "High because it can create events involving guests or resources. Misuse can invite or affect third parties and their schedules.",
	"calendar_events_update_self":   "Medium because it can change existing personal events. The proxy blocks events with guests or resources, so impact stays limited to the user's own calendar.",
	"calendar_events_update_guests": "High because it can update events involving guests or move events between calendars. Misuse can affect third parties, resources, or event ownership context.",
	"calendar_events_delete_self":   "High because it deletes existing calendar events. Even without guests, deletion can remove important personal or business records.",
	"calendar_events_delete_guests": "High because it deletes events involving guests or resources. Misuse can remove records and affect third-party schedules.",
	"calendar_calendars_manage":     "High because it can create, update, clear, or delete calendars and calendar-list entries. Misuse can remove or disrupt broad calendar organization.",
	"calendar_access_read":          "Low because it only reads calendar sharing rules. It does not change access or expose secrets.",
	"calendar_access_manage":        "High because it can create, update, delete, or watch calendar access rules. Misuse can grant or remove calendar access.",
	"calendar_settings_read":        "Medium because it reads Calendar user settings. These are configuration details rather than ordinary content.",

	"drive_files_read":         "Low because it only reads or downloads other file types already inside allowed folders. It does not modify files or read Google Docs, Sheets, or Slides content.",
	"drive_files_create":       "Medium because it can create or copy other file types inside allowed folders. In shared folders this can create clutter or visible business artifacts.",
	"drive_files_update":       "Medium because it can rename, update metadata, or upload content for other file types in allowed folders. It does not edit Google Docs, Sheets, or Slides content.",
	"drive_files_delete":       "High because it can permanently delete other file types in allowed folders. Deletion can remove important data.",
	"drive_permissions_read":   "Low because it only reads file sharing permissions. It does not change access or expose secrets.",
	"drive_permissions_manage": "High because it can create, update, or delete file permissions. Misuse can expose files or remove legitimate access.",
	"drive_comments_read":      "Low because it only reads comments and replies on files. It does not post, edit, resolve, or delete comments.",
	"drive_comments_create":    "Medium because it can add comments to files, which may notify collaborators or add noise, but it does not alter file content, sharing, or existing comments.",
	"drive_comments_reply":     "Medium because it can add replies to existing comments, which may notify collaborators or add noise, but it does not alter file content or existing discussion history.",
	"drive_comments_resolve":   "High because it can mark comment discussions as resolved. Misuse can hide unresolved issues from collaborators.",
	"drive_comments_update":    "Medium because it can edit only comments and replies authored by the connected user. Ownership inspection prevents altering other people's comments, and no data is deleted.",
	"drive_comments_delete":    "High because it can delete comments or replies authored by the connected user. Deletion removes discussion context even when ownership is enforced.",
	"drive_metadata_read":      "Low because it reads user-facing metadata such as file search results, account info, changes, shared drives, apps, file labels, and revisions. It does not read file content, change files, or change access.",
	"drive_labels_update":      "Medium because it changes labels applied to files. Labels can affect organization or classification, but this does not edit file content, sharing, or delete data.",
	"drive_revisions_delete":   "High because it permanently deletes file revisions where Google permits it. Misuse can remove historical file content.",
	"docs_read":                "Low because it only reads Google Docs located in allowed folders. It does not edit, create, delete, or share documents.",
	"docs_create":              "Medium because it can create new Docs in allowed folders. In shared folders this can create visible business content.",
	"docs_edit":                "Medium because it can modify existing Docs in allowed folders. The changes affect content but are scoped to allowed folders.",
	"docs_delete":              "High because it can permanently delete Google Docs in allowed folders. Deletion can remove important document content.",

	"sheets_read":   "Low because it only reads spreadsheet structure and values in allowed folders. It does not change formulas, values, or sharing.",
	"sheets_create": "Medium because it can create new spreadsheets in allowed folders. In shared folders this can create visible business artifacts.",
	"sheets_edit":   "High because it can change spreadsheet values, formulas, and structure. Misuse can corrupt operational data or calculations that people rely on.",
	"sheets_delete": "High because it can permanently delete Google Sheets spreadsheets in allowed folders. Deletion can remove important operational data.",

	"slides_read":   "Low because it only reads presentations, pages, and related content in allowed folders. It does not alter slides.",
	"slides_create": "Medium because it can create new presentations in allowed folders. In shared folders this can create visible business content.",
	"slides_edit":   "Medium because it can modify existing presentations in allowed folders. The changes affect content but are scoped and typically recoverable.",
	"slides_delete": "High because it can permanently delete Google Slides presentations in allowed folders. Deletion can remove important presentation content.",
}

func policyRiskScore(key string) int {
	score, ok := policyCapabilityRiskScores[key]
	if !ok {
		panic(fmt.Sprintf("missing risk score for policy capability %q", key))
	}
	if score < 1 || score > 3 {
		panic(fmt.Sprintf("risk score for policy capability %q must be between 1 and 3, got %d", key, score))
	}
	return score
}

func ValidatePolicyCatalog() error {
	catalogKeys := map[string]bool{}
	var errs []error
	for _, cap := range PolicyCatalog() {
		if cap.Key == "" {
			errs = append(errs, fmt.Errorf("policy capability has an empty key"))
			continue
		}
		if catalogKeys[cap.Key] {
			errs = append(errs, fmt.Errorf("duplicate policy capability key %q", cap.Key))
		}
		catalogKeys[cap.Key] = true
		score, ok := policyCapabilityRiskScores[cap.Key]
		if !ok {
			errs = append(errs, fmt.Errorf("missing risk score for policy capability %q", cap.Key))
		} else if score < 1 || score > 3 {
			errs = append(errs, fmt.Errorf("risk score for policy capability %q must be between 1 and 3, got %d", cap.Key, score))
		}
		description, ok := policyCapabilityRiskDescriptions[cap.Key]
		if !ok {
			errs = append(errs, fmt.Errorf("missing risk description for policy capability %q", cap.Key))
		} else if strings.TrimSpace(description) == "" {
			errs = append(errs, fmt.Errorf("risk description for policy capability %q cannot be empty", cap.Key))
		}
	}
	for key, score := range policyCapabilityRiskScores {
		if !catalogKeys[key] {
			errs = append(errs, fmt.Errorf("risk score configured for unknown policy capability %q", key))
		}
		if score < 1 || score > 3 {
			errs = append(errs, fmt.Errorf("risk score for policy capability %q must be between 1 and 3, got %d", key, score))
		}
	}
	for key, description := range policyCapabilityRiskDescriptions {
		if !catalogKeys[key] {
			errs = append(errs, fmt.Errorf("risk description configured for unknown policy capability %q", key))
		}
		if strings.TrimSpace(description) == "" {
			errs = append(errs, fmt.Errorf("risk description for policy capability %q cannot be empty", key))
		}
	}
	return errors.Join(errs...)
}

func policyRiskLabel(score int) string {
	switch score {
	case 1:
		return "Low"
	case 2:
		return "Medium"
	default:
		return "High"
	}
}

func policyRiskClass(score int) string {
	switch score {
	case 1:
		return "risk-low"
	case 2:
		return "risk-medium"
	default:
		return "risk-high"
	}
}

func policyRiskDescription(key string) string {
	description, ok := policyCapabilityRiskDescriptions[key]
	if !ok {
		panic(fmt.Sprintf("missing risk description for policy capability %q", key))
	}
	return description
}

func policyApplicationGroup(group string) string {
	switch group {
	case "Calendar", "Gmail", "Contacts":
		return group
	default:
		return "Drive"
	}
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
