#!/usr/bin/env python3
# Copyright (c) 2026 Opensense Ltd. (Hong Kong). All rights reserved.
# Proprietary software. No use, copy, modification, distribution, disclosure,
# or reverse engineering is permitted without prior written authorization
# from Opensense Ltd.

"""Live positive/negative proxy-policy tests keyed by AIWP_COVERAGE_ITEM_ID."""

import argparse
import json
import os
import sys
import uuid
from pathlib import Path
from urllib.error import HTTPError, URLError
from urllib.parse import quote, urlencode
from urllib.request import Request, urlopen


REPO_ROOT = Path(__file__).resolve().parents[2]
DEFAULT_AGENT_CONFIG = REPO_ROOT / "testing" / "config" / "agents-workspace-api-access.config.json"
DEFAULT_USER_CONFIG = REPO_ROOT / "testing" / "config" / "user-backend-api-access.config.json"
DEFAULT_LEGACY_CONFIG = REPO_ROOT / "testing" / "config" / "ai-workspace-proxy.json"
TEST_PREFIX = "AIWP_TEST"
GOOGLE_DOC_MIME_TYPE = "application/vnd.google-apps.document"


class TestFailure(Exception):
    pass


class HTTPFailure(Exception):
    def __init__(self, status, payload, body):
        super().__init__(f"HTTP {status}: {body}")
        self.status = status
        self.payload = payload
        self.body = body


class LivePolicyPermissionTest:
    def __init__(self, policy_key, cfg):
        self.policy_key = policy_key
        self.cfg = cfg
        self.run_id = uuid.uuid4().hex[:10]
        self.created_files = []
        self.created_policies = []
        self.original_policy_id = ""
        self.drive_ref = None
        self.original_drive_folder_ref_ids = None

    def run(self):
        self.original_policy_id = self.agent_grant_policy_id()
        print(f"Workspace: {self.cfg['workspace']}")
        print(f"Original policy: {self.original_policy_id}")
        failure = None
        try:
            SCENARIOS[self.policy_key](self)
        except Exception as exc:
            failure = exc
        finally:
            self.cleanup()
        if failure is not None:
            raise failure
        print(f"PASS: {self.policy_key} live positive/negative policy test")

    def apply_policy_with(self, label, capabilities):
        policy_id = self.create_policy(label, capabilities)
        self.apply_policy(policy_id)
        return policy_id

    def apply_subject_policy(self):
        return self.apply_policy_with(self.policy_key, [self.policy_key])

    def create_policy(self, label, capabilities):
        policy_id = create_policy(self.cfg, f"{TEST_PREFIX} {label} {self.run_id}", capabilities)
        self.created_policies.append(policy_id)
        return policy_id

    def apply_policy(self, policy_id):
        apply_policy_to_agent_grant(self.cfg, policy_id)
        print(f"Applied temporary policy: {policy_id}")

    def agent_grant_policy_id(self):
        _, payload = backend_json(self.cfg, "GET", f"/api/user/agents/{quote(self.cfg['agent_id'], safe='')}/grants?{urlencode({'workspace': self.cfg['workspace']})}")
        grant = payload.get("grant")
        if not isinstance(grant, dict):
            raise TestFailure(f"Agent grant lookup response did not contain a grant object: {payload}")
        policy_id = str(grant.get("policy_id") or "system").strip()
        return policy_id or "system"

    def ensure_drive_ref(self):
        if self.drive_ref is not None:
            return self.drive_ref
        _, payload = backend_json(self.cfg, "GET", f"/api/user/drive-folders?{urlencode({'workspace': self.cfg['workspace']})}")
        folders = payload.get("drive_folders")
        if not isinstance(folders, list):
            raise TestFailure(f"Drive folder lookup response did not contain drive_folders: {payload}")
        for folder in folders:
            if not isinstance(folder, dict):
                continue
            reference_name = str(folder.get("reference_name") or "").strip()
            if "test" in reference_name.lower():
                self.drive_ref = folder
                self.ensure_agent_drive_folder_grant(folder)
                print(f"Allowed Drive folder: {reference_name}")
                return folder
        available = ", ".join(
            str(folder.get("reference_name") or "").strip()
            for folder in folders
            if isinstance(folder, dict) and str(folder.get("reference_name") or "").strip()
        ) or "<none>"
        raise TestFailure(f"No allowed Drive folder reference containing 'Test' was found. Available references: {available}")

    def require_folder_type(self, field, label):
        _ = field
        _ = label
        self.ensure_drive_ref()

    def agent_drive_folder_grant_ref_ids(self):
        _, payload = backend_json(
            self.cfg,
            "GET",
            f"/api/user/agents/{quote(self.cfg['agent_id'], safe='')}/grants/drive-folders?{urlencode({'workspace': self.cfg['workspace']})}",
        )
        folders = payload.get("drive_folders")
        if not isinstance(folders, list):
            raise TestFailure(f"Agent Drive folder grant lookup response did not contain drive_folders: {payload}")
        return [
            str(folder.get("id") or "").strip()
            for folder in folders
            if isinstance(folder, dict) and folder.get("allowed") is True and str(folder.get("id") or "").strip()
        ]

    def ensure_agent_drive_folder_grant(self, folder):
        folder_ref_id = str(folder.get("id") or "").strip()
        if not folder_ref_id:
            raise TestFailure(f"Drive folder response did not contain id: {folder}")
        if self.original_drive_folder_ref_ids is None:
            self.original_drive_folder_ref_ids = self.agent_drive_folder_grant_ref_ids()
        next_ids = list(self.original_drive_folder_ref_ids)
        if folder_ref_id not in next_ids:
            next_ids.append(folder_ref_id)
        backend_json(
            self.cfg,
            "PUT",
            f"/api/user/agents/{quote(self.cfg['agent_id'], safe='')}/grants/drive-folders",
            {"workspace": self.cfg["workspace"], "folder_ref_ids": next_ids},
        )

    def create_drive_text_file(self, label):
        self.require_folder_type("allow_drive_files", "Drive files")
        _, payload = agent_json(self.cfg, "POST", self.drive_path("files", include_ref=True, extra_query={
            "fields": "id,name,mimeType,parents",
        }), {
            "name": self.name(label),
            "mimeType": "text/plain",
        })
        file_id = str(payload.get("id") or "").strip()
        if not file_id:
            raise TestFailure(f"Drive file create response did not contain id: {payload}")
        self.created_files.append(file_id)
        print(f"Created text file: {file_id}")
        return file_id

    def create_doc(self, label):
        self.require_folder_type("allow_docs", "Docs")
        _, payload = agent_json(self.cfg, "POST", self.docs_path("documents", include_ref=True), {
            "title": self.name(label),
        })
        file_id = str(payload.get("documentId") or "").strip()
        if not file_id:
            raise TestFailure(f"Docs create response did not contain documentId: {payload}")
        self.created_files.append(file_id)
        print(f"Created Doc: {file_id}")
        return file_id

    def create_doc_via_drive(self, label):
        self.require_folder_type("allow_docs", "Docs")
        path = self.drive_path("files", include_ref=True, extra_query={
            "fields": "id,name,mimeType,parents",
        })
        body = {
            "name": self.name(label),
            "mimeType": GOOGLE_DOC_MIME_TYPE,
        }
        try:
            _, payload = agent_json(self.cfg, "POST", path, body)
        except HTTPFailure as exc:
            if exc.status == 403 and isinstance(exc.payload, dict) and exc.payload.get("error") == "request_denied":
                raise TestFailure(
                    "Proxy denied Drive-native Google Doc creation under docs_create. "
                    "If the proxy was not rebuilt and redeployed after adding docs_create support for "
                    "POST /drive/v3/files with application/vnd.google-apps.document, rebuild/redeploy it. "
                    f"Proxy response: {exc.body}"
                ) from exc
            raise
        file_id = str(payload.get("id") or "").strip()
        if not file_id:
            raise TestFailure(f"Drive-native Docs create response did not contain id: {payload}")
        self.created_files.append(file_id)
        print(f"Created Doc via Drive API: {file_id}")
        return file_id

    def create_sheet(self, label):
        self.require_folder_type("allow_sheets", "Sheets")
        _, payload = agent_json(self.cfg, "POST", self.sheets_path("spreadsheets", include_ref=True), {
            "properties": {"title": self.name(label)},
        })
        file_id = str(payload.get("spreadsheetId") or "").strip()
        if not file_id:
            raise TestFailure(f"Sheets create response did not contain spreadsheetId: {payload}")
        self.created_files.append(file_id)
        print(f"Created Sheet: {file_id}")
        return file_id

    def create_slide(self, label):
        self.require_folder_type("allow_slides", "Slides")
        _, payload = agent_json(self.cfg, "POST", self.slides_path("presentations", include_ref=True), {
            "title": self.name(label),
        })
        file_id = str(payload.get("presentationId") or "").strip()
        if not file_id:
            raise TestFailure(f"Slides create response did not contain presentationId: {payload}")
        self.created_files.append(file_id)
        print(f"Created Slide: {file_id}")
        return file_id

    def create_comment(self, file_id, label):
        _, payload = agent_json(self.cfg, "POST", self.drive_path(
            f"files/{quote(file_id, safe='')}/comments",
            extra_query={"fields": "id,content"},
        ), {"content": self.name(label)})
        comment_id = str(payload.get("id") or "").strip()
        if not comment_id:
            raise TestFailure(f"Comment create response did not contain id: {payload}")
        print(f"Created comment: {comment_id}")
        return comment_id

    def create_reply(self, file_id, comment_id, label, resolve=False):
        body = {"content": self.name(label)}
        if resolve:
            body["action"] = "resolve"
        _, payload = agent_json(self.cfg, "POST", self.drive_path(
            f"files/{quote(file_id, safe='')}/comments/{quote(comment_id, safe='')}/replies",
            extra_query={"fields": "id,content,action"},
        ), body)
        reply_id = str(payload.get("id") or "").strip()
        if not reply_id:
            raise TestFailure(f"Reply create response did not contain id: {payload}")
        print(f"Created reply: {reply_id}")
        return reply_id

    def setup_doc_comment(self):
        self.apply_policy_with("setup-doc-comment", [
            "docs_create",
            "drive_comments_create",
            "drive_comments_reply",
            "drive_comments_read",
        ])
        doc_id = self.create_doc_via_drive("comment fixture")
        comment_id = self.create_comment(doc_id, "comment fixture")
        reply_id = self.create_reply(doc_id, comment_id, "reply fixture")
        return doc_id, comment_id, reply_id

    def gmail_path(self, suffix, extra_query=None):
        return self.api_path("gmail.googleapis.com", f"gmail/v1/users/me/{suffix}", extra_query)

    def people_path(self, suffix, extra_query=None):
        return self.api_path("people.googleapis.com", f"v1/{suffix}", extra_query)

    def drive_path(self, suffix, include_ref=False, extra_query=None):
        query = dict(extra_query or {})
        if include_ref:
            query["driveRef"] = self.ensure_drive_ref()["reference_name"]
        return self.api_path("drive.googleapis.com", f"drive/v3/{suffix}", query)

    def docs_path(self, suffix, include_ref=False, extra_query=None):
        query = dict(extra_query or {})
        if include_ref:
            query["driveRef"] = self.ensure_drive_ref()["reference_name"]
        return self.api_path("docs.googleapis.com", f"v1/{suffix}", query)

    def sheets_path(self, suffix, include_ref=False, extra_query=None):
        query = dict(extra_query or {})
        if include_ref:
            query["driveRef"] = self.ensure_drive_ref()["reference_name"]
        return self.api_path("sheets.googleapis.com", f"v4/{suffix}", query)

    def slides_path(self, suffix, include_ref=False, extra_query=None):
        query = dict(extra_query or {})
        if include_ref:
            query["driveRef"] = self.ensure_drive_ref()["reference_name"]
        return self.api_path("slides.googleapis.com", f"v1/{suffix}", query)

    def api_path(self, host, suffix, extra_query=None):
        query = {"workspace": self.cfg["workspace"]}
        if extra_query:
            query.update(extra_query)
        return f"/{host}/{suffix}?{urlencode(query)}"

    def expect_denied(self, method, path, body=None, want_text=""):
        try:
            _, payload = agent_json(self.cfg, method, path, body, expect_status=403)
        except HTTPFailure as exc:
            raise TestFailure(f"Expected proxy denial with 403, got {exc.status}: {exc.body}") from exc
        if payload.get("error") != "request_denied":
            raise TestFailure(f"Expected request_denied error, got: {payload}")
        msg = str(payload.get("message") or "")
        if want_text and want_text.lower() not in msg.lower():
            raise TestFailure(f"Expected denial message to contain {want_text!r}, got: {payload}")
        print(f"Denied as expected: {method} {path}")

    def expect_forwarded(self, method, path, body=None):
        try:
            status, payload = agent_json(self.cfg, method, path, body)
            print(f"Forwarded request succeeded with HTTP {status}: {method} {path}")
            return status, payload
        except HTTPFailure as exc:
            if isinstance(exc.payload, dict) and isinstance(exc.payload.get("error"), dict):
                print(f"Forwarded request reached Google and returned HTTP {exc.status}: {method} {path}")
                return exc.status, exc.payload
            raise TestFailure(f"Expected request to be forwarded to Google, got proxy error {exc.status}: {exc.body}") from exc

    def forget_file(self, file_id):
        self.created_files = [existing for existing in self.created_files if existing != file_id]

    def name(self, label):
        return f"{TEST_PREFIX} {self.policy_key} {label} {self.run_id}"

    def cleanup(self):
        if self.created_files:
            try:
                self.apply_policy_with("cleanup", ["docs_delete", "drive_files_delete", "sheets_delete", "slides_delete"])
            except Exception as exc:
                print(f"WARN: could not apply cleanup policy: {exc}", file=sys.stderr)
        for file_id in reversed(list(self.created_files)):
            try:
                agent_json(self.cfg, "DELETE", self.drive_path(f"files/{quote(file_id, safe='')}"))
                print(f"Cleanup requested for file: {file_id}")
            except HTTPFailure as exc:
                if exc.status not in {404, 410}:
                    print(f"WARN: could not delete file {file_id}: {exc}", file=sys.stderr)
        self.created_files = []
        if self.original_policy_id:
            try:
                apply_policy_to_agent_grant(self.cfg, self.original_policy_id)
                print(f"Restored original policy: {self.original_policy_id}")
            except Exception as exc:
                print(f"WARN: could not restore original policy {self.original_policy_id}: {exc}", file=sys.stderr)
        if self.original_drive_folder_ref_ids is not None:
            try:
                backend_json(
                    self.cfg,
                    "PUT",
                    f"/api/user/agents/{quote(self.cfg['agent_id'], safe='')}/grants/drive-folders",
                    {"workspace": self.cfg["workspace"], "folder_ref_ids": self.original_drive_folder_ref_ids},
                )
                print("Restored original agent Drive folder grants")
            except Exception as exc:
                print(f"WARN: could not restore original agent Drive folder grants: {exc}", file=sys.stderr)
        for policy_id in reversed(self.created_policies):
            delete_policy(self.cfg, policy_id)
            print(f"Cleanup requested for policy: {policy_id}")
        self.created_policies = []


def gmail_profile_read(test):
    test.apply_subject_policy()
    test.expect_forwarded("GET", test.gmail_path("profile"))
    test.expect_forwarded("GET", test.gmail_path("history"))
    test.expect_denied("GET", test.gmail_path("messages"))


def contacts_read(test):
    test.apply_subject_policy()
    query = {"query": "Google", "pageSize": "1"}
    test.expect_forwarded("GET", test.people_path("people:searchContacts", query))
    test.expect_forwarded("GET", test.people_path("otherContacts:search", query))
    test.expect_forwarded("GET", test.people_path("people:searchDirectoryPeople", query))
    test.expect_denied("GET", test.people_path("people/me/connections"))
    test.expect_denied("POST", test.people_path("people:createContact"), {"names": [{"givenName": test.name("blocked")}]})


def gmail_messages_read(test):
    test.apply_subject_policy()
    test.expect_forwarded("GET", test.gmail_path("messages", {"maxResults": "1"}))
    test.expect_forwarded("GET", test.gmail_path("threads", {"maxResults": "1"}))
    test.expect_denied("POST", test.gmail_path("messages/send"), {"raw": "invalid"})


def gmail_drafts_read(test):
    test.apply_subject_policy()
    test.expect_forwarded("GET", test.gmail_path("drafts"))
    test.expect_denied("POST", test.gmail_path("drafts"), {"message": {"raw": "invalid"}})


def gmail_labels_read(test):
    test.apply_subject_policy()
    test.expect_forwarded("GET", test.gmail_path("labels"))
    test.expect_forwarded("GET", test.gmail_path("labels/INBOX"))
    test.expect_denied("POST", test.gmail_path("labels"), {"name": test.name("blocked")})


def gmail_drafts_write(test):
    test.apply_subject_policy()
    test.expect_forwarded("POST", test.gmail_path("drafts"), {"message": {"raw": "invalid"}})
    test.expect_forwarded("PUT", test.gmail_path("drafts/aiwp-missing-draft"), {"id": "aiwp-missing-draft", "message": {"raw": "invalid"}})
    test.expect_denied("DELETE", test.gmail_path("drafts/aiwp-missing-draft"))
    test.expect_denied("POST", test.gmail_path("drafts/send"), {"id": "aiwp-missing-draft"})


def gmail_drafts_delete(test):
    test.apply_subject_policy()
    test.expect_forwarded("DELETE", test.gmail_path("drafts/aiwp-missing-draft"))
    test.expect_denied("POST", test.gmail_path("drafts"), {"message": {"raw": "invalid"}})
    test.expect_denied("POST", test.gmail_path("drafts/send"), {"id": "aiwp-missing-draft"})


def gmail_send(test):
    test.apply_subject_policy()
    test.expect_forwarded("POST", test.gmail_path("messages/send"), {"raw": "invalid"})
    test.expect_forwarded("POST", test.gmail_path("drafts/send"), {"id": "aiwp-missing-draft"})
    test.expect_denied("POST", test.gmail_path("drafts"), {"message": {"raw": "invalid"}})


def gmail_labels_apply_custom(test):
    test.apply_subject_policy()
    body = {"addLabelIds": ["Label_aiwp_missing"]}
    test.expect_forwarded("POST", test.gmail_path("messages/aiwp-missing-message/modify"), body)
    test.expect_forwarded("POST", test.gmail_path("threads/aiwp-missing-thread/modify"), body)
    test.expect_denied("POST", test.gmail_path("messages/aiwp-missing-message/modify"), {"removeLabelIds": ["INBOX"]})


def gmail_messages_archive(test):
    test.apply_subject_policy()
    test.expect_forwarded("POST", test.gmail_path("messages/aiwp-missing-message/modify"), {"removeLabelIds": ["INBOX"]})
    test.expect_denied("POST", test.gmail_path("messages/aiwp-missing-message/modify"), {"addLabelIds": ["Label_aiwp_missing"]})


def gmail_messages_status(test):
    test.apply_subject_policy()
    test.expect_forwarded("POST", test.gmail_path("messages/aiwp-missing-message/modify"), {"addLabelIds": ["STARRED"], "removeLabelIds": ["UNREAD"]})
    test.expect_denied("POST", test.gmail_path("messages/aiwp-missing-message/modify"), {"addLabelIds": ["TRASH"]})


def gmail_messages_spam(test):
    test.apply_subject_policy()
    test.expect_forwarded("POST", test.gmail_path("messages/aiwp-missing-message/modify"), {"addLabelIds": ["SPAM"]})
    test.expect_denied("POST", test.gmail_path("messages/aiwp-missing-message/modify"), {"addLabelIds": ["TRASH"]})


def gmail_messages_trash(test):
    test.apply_subject_policy()
    test.expect_forwarded("POST", test.gmail_path("messages/aiwp-missing-message/trash"))
    test.expect_forwarded("POST", test.gmail_path("threads/aiwp-missing-thread/untrash"))
    test.expect_denied("DELETE", test.gmail_path("messages/aiwp-missing-message"))


def gmail_labels_create_rename(test):
    test.apply_subject_policy()
    test.expect_forwarded("POST", test.gmail_path("labels"), {"name": ""})
    test.expect_forwarded("PATCH", test.gmail_path("labels/Label_aiwp_missing"), {"name": test.name("rename")})
    test.expect_denied("PATCH", test.gmail_path("labels/INBOX"), {"name": test.name("blocked")})
    test.expect_denied("DELETE", test.gmail_path("labels/Label_aiwp_missing"))


def gmail_labels_delete(test):
    test.apply_subject_policy()
    test.expect_forwarded("DELETE", test.gmail_path("labels/Label_aiwp_missing"))
    test.expect_denied("DELETE", test.gmail_path("labels/INBOX"))
    test.expect_denied("POST", test.gmail_path("labels"), {"name": test.name("blocked")})


def gmail_messages_delete(test):
    test.apply_subject_policy()
    test.expect_forwarded("DELETE", test.gmail_path("messages/aiwp-missing-message"))
    test.expect_forwarded("POST", test.gmail_path("messages/batchDelete"), {"ids": ["aiwp-missing-message"]})
    test.expect_denied("POST", test.gmail_path("messages/aiwp-missing-message/trash"))
    test.expect_denied("GET", test.gmail_path("messages/aiwp-missing-message"))


def gmail_messages_import(test):
    test.apply_subject_policy()
    test.expect_forwarded("POST", test.gmail_path("messages/import"), {"raw": "invalid"})
    test.expect_forwarded("POST", test.gmail_path("messages"), {"raw": "invalid"})
    test.expect_denied("POST", test.gmail_path("messages/send"), {"raw": "invalid"})


def gmail_watch_manage(test):
    test.apply_subject_policy()
    test.expect_forwarded("POST", test.gmail_path("watch"), {"topicName": "projects/aiwp-missing/topics/aiwp"})
    test.expect_denied("GET", test.gmail_path("history"))


def drive_files_create(test):
    test.apply_subject_policy()
    file_id = test.create_drive_text_file("create positive")
    test.expect_denied("GET", test.drive_path(f"files/{quote(file_id, safe='')}", extra_query={"fields": "id"}))
    test.expect_denied("PATCH", test.drive_path(f"files/{quote(file_id, safe='')}"), {"name": test.name("blocked")})


def drive_files_update(test):
    test.apply_policy_with("setup-files-update", ["drive_files_create"])
    file_id = test.create_drive_text_file("update positive")
    blocked_file_id = test.create_drive_text_file("update negative file")
    test.apply_subject_policy()
    test.expect_forwarded("PATCH", test.drive_path(f"files/{quote(file_id, safe='')}"), {"name": test.name("renamed")})
    test.expect_denied("POST", test.drive_path("files", include_ref=True), {"name": test.name("blocked create"), "mimeType": "text/plain"})
    test.expect_denied("DELETE", test.drive_path(f"files/{quote(file_id, safe='')}"))
    test.expect_denied("GET", test.drive_path(f"files/{quote(blocked_file_id, safe='')}", extra_query={"fields": "id"}))


def drive_files_delete(test):
    test.apply_policy_with("setup-files-delete", ["drive_files_create"])
    file_id = test.create_drive_text_file("delete positive")
    blocked_file_id = test.create_drive_text_file("delete negative file")
    test.apply_subject_policy()
    test.expect_forwarded("DELETE", test.drive_path(f"files/{quote(file_id, safe='')}"))
    test.forget_file(file_id)
    test.expect_denied("PATCH", test.drive_path(f"files/{quote(blocked_file_id, safe='')}"), {"name": test.name("blocked update")})
    test.expect_denied("DELETE", test.drive_path("files/trash"))


def drive_permissions_read(test):
    test.apply_policy_with("setup-permissions-read", ["drive_files_create"])
    file_id = test.create_drive_text_file("permissions read fixture")
    test.apply_subject_policy()
    test.expect_forwarded("GET", test.drive_path(f"files/{quote(file_id, safe='')}/permissions", extra_query={"fields": "permissions(id,type,role)"}))
    test.expect_denied("POST", test.drive_path(f"files/{quote(file_id, safe='')}/permissions"), {"role": "reader", "type": "user"})


def drive_permissions_manage(test):
    test.apply_policy_with("setup-permissions-manage", ["drive_files_create"])
    file_id = test.create_drive_text_file("permissions manage fixture")
    test.apply_subject_policy()
    test.expect_forwarded("POST", test.drive_path(f"files/{quote(file_id, safe='')}/permissions"), {"role": "reader", "type": "invalid"})
    test.expect_forwarded("PATCH", test.drive_path(f"files/{quote(file_id, safe='')}/permissions/aiwp-missing-permission"), {"role": "commenter"})
    test.expect_denied("GET", test.drive_path(f"files/{quote(file_id, safe='')}/permissions"))


def drive_comments_create(test):
    doc_id, comment_id, _ = test.setup_doc_comment()
    test.apply_subject_policy()
    test.expect_forwarded("POST", test.drive_path(f"files/{quote(doc_id, safe='')}/comments"), {"content": test.name("allowed comment")})
    test.expect_denied("POST", test.drive_path(f"files/{quote(doc_id, safe='')}/comments/{quote(comment_id, safe='')}/replies"), {"content": test.name("blocked reply")})


def drive_comments_reply(test):
    doc_id, comment_id, _ = test.setup_doc_comment()
    test.apply_subject_policy()
    replies = f"files/{quote(doc_id, safe='')}/comments/{quote(comment_id, safe='')}/replies"
    test.expect_forwarded("POST", test.drive_path(replies), {"content": test.name("allowed reply")})
    test.expect_denied("POST", test.drive_path(replies), {"action": "resolve", "content": test.name("blocked resolve")})
    test.expect_denied("POST", test.drive_path(f"files/{quote(doc_id, safe='')}/comments"), {"content": test.name("blocked comment")})


def drive_comments_resolve(test):
    doc_id, comment_id, _ = test.setup_doc_comment()
    test.apply_subject_policy()
    replies = f"files/{quote(doc_id, safe='')}/comments/{quote(comment_id, safe='')}/replies"
    test.expect_forwarded("POST", test.drive_path(replies), {"action": "resolve", "content": test.name("allowed resolve")})
    test.expect_denied("POST", test.drive_path(replies), {"content": test.name("blocked reply")})


def drive_comments_update(test):
    doc_id, comment_id, reply_id = test.setup_doc_comment()
    test.apply_subject_policy()
    comment = f"files/{quote(doc_id, safe='')}/comments/{quote(comment_id, safe='')}"
    reply = f"{comment}/replies/{quote(reply_id, safe='')}"
    test.expect_forwarded("PATCH", test.drive_path(comment), {"content": test.name("updated comment")})
    test.expect_forwarded("PATCH", test.drive_path(reply), {"content": test.name("updated reply")})
    test.expect_denied("POST", test.drive_path(f"{comment}/replies"), {"content": test.name("blocked reply")})


def drive_comments_delete(test):
    doc_id, comment_id, reply_id = test.setup_doc_comment()
    test.apply_subject_policy()
    comment = f"files/{quote(doc_id, safe='')}/comments/{quote(comment_id, safe='')}"
    reply = f"{comment}/replies/{quote(reply_id, safe='')}"
    test.expect_forwarded("DELETE", test.drive_path(reply))
    test.expect_forwarded("DELETE", test.drive_path(comment))
    test.expect_denied("PATCH", test.drive_path(comment), {"content": test.name("blocked update")})


def drive_labels_update(test):
    test.apply_policy_with("setup-labels-update", ["drive_files_create"])
    file_id = test.create_drive_text_file("labels update fixture")
    test.apply_subject_policy()
    test.expect_forwarded("POST", test.drive_path(f"files/{quote(file_id, safe='')}/modifyLabels"), {"labelModifications": []})
    test.expect_denied("DELETE", test.drive_path(f"files/{quote(file_id, safe='')}/revisions/aiwp-missing-revision"))


def drive_revisions_delete(test):
    test.apply_policy_with("setup-revisions-delete", ["drive_files_create"])
    file_id = test.create_drive_text_file("revision delete fixture")
    test.apply_subject_policy()
    test.expect_forwarded("DELETE", test.drive_path(f"files/{quote(file_id, safe='')}/revisions/aiwp-missing-revision"))
    test.expect_denied("POST", test.drive_path(f"files/{quote(file_id, safe='')}/modifyLabels"), {"labelModifications": []})


def docs_delete(test):
    test.apply_policy_with("setup-docs-delete", ["docs_create", "drive_files_create"])
    doc_id = test.create_doc("delete positive")
    file_id = test.create_drive_text_file("delete negative file")
    test.apply_subject_policy()
    test.expect_forwarded("DELETE", test.drive_path(f"files/{quote(doc_id, safe='')}"))
    test.forget_file(doc_id)
    test.expect_denied("DELETE", test.drive_path(f"files/{quote(file_id, safe='')}"))


def sheets_read(test):
    test.apply_policy_with("setup-sheets-read", ["sheets_create"])
    sheet_id = test.create_sheet("read fixture")
    test.apply_subject_policy()
    test.expect_forwarded("GET", test.sheets_path(f"spreadsheets/{quote(sheet_id, safe='')}", extra_query={"includeGridData": "false"}))
    test.expect_forwarded("GET", test.sheets_path(f"spreadsheets/{quote(sheet_id, safe='')}/values/Sheet1!A1"))
    test.expect_denied("POST", test.sheets_path("spreadsheets", include_ref=True), {"properties": {"title": test.name("blocked")}})
    test.expect_denied("POST", test.sheets_path(f"spreadsheets/{quote(sheet_id, safe='')}:batchUpdate"), {"requests": []})


def sheets_create(test):
    test.apply_subject_policy()
    sheet_id = test.create_sheet("create positive")
    test.expect_denied("GET", test.sheets_path(f"spreadsheets/{quote(sheet_id, safe='')}"))
    test.expect_denied("POST", test.sheets_path(f"spreadsheets/{quote(sheet_id, safe='')}:batchUpdate"), {"requests": []})


def sheets_edit(test):
    test.apply_policy_with("setup-sheets-edit", ["sheets_create"])
    sheet_id = test.create_sheet("edit fixture")
    test.apply_subject_policy()
    test.expect_forwarded("POST", test.sheets_path(f"spreadsheets/{quote(sheet_id, safe='')}:batchUpdate"), {
        "requests": [{"updateSpreadsheetProperties": {"properties": {"title": test.name("edited")}, "fields": "title"}}],
    })
    test.expect_forwarded("PUT", test.sheets_path(f"spreadsheets/{quote(sheet_id, safe='')}/values/Sheet1!A1", extra_query={"valueInputOption": "RAW"}), {"values": [["x"]]})
    test.expect_forwarded("POST", test.sheets_path(f"spreadsheets/{quote(sheet_id, safe='')}/values/Sheet1!A1:clear"), {})
    test.expect_denied("GET", test.sheets_path(f"spreadsheets/{quote(sheet_id, safe='')}"))
    test.expect_denied("POST", test.sheets_path("spreadsheets", include_ref=True), {"properties": {"title": test.name("blocked")}})


def sheets_delete(test):
    test.apply_policy_with("setup-sheets-delete", ["sheets_create", "docs_create"])
    sheet_id = test.create_sheet("delete positive")
    doc_id = test.create_doc("delete negative doc")
    test.apply_subject_policy()
    test.expect_forwarded("DELETE", test.drive_path(f"files/{quote(sheet_id, safe='')}"))
    test.forget_file(sheet_id)
    test.expect_denied("DELETE", test.drive_path(f"files/{quote(doc_id, safe='')}"))


def slides_read(test):
    test.apply_policy_with("setup-slides-read", ["slides_create"])
    slide_id = test.create_slide("read fixture")
    test.apply_subject_policy()
    test.expect_forwarded("GET", test.slides_path(f"presentations/{quote(slide_id, safe='')}"))
    test.expect_denied("POST", test.slides_path("presentations", include_ref=True), {"title": test.name("blocked")})
    test.expect_denied("POST", test.slides_path(f"presentations/{quote(slide_id, safe='')}:batchUpdate"), {"requests": []})


def slides_create(test):
    test.apply_subject_policy()
    slide_id = test.create_slide("create positive")
    test.expect_denied("GET", test.slides_path(f"presentations/{quote(slide_id, safe='')}"))
    test.expect_denied("POST", test.slides_path(f"presentations/{quote(slide_id, safe='')}:batchUpdate"), {"requests": []})


def slides_edit(test):
    test.apply_policy_with("setup-slides-edit", ["slides_create"])
    slide_id = test.create_slide("edit fixture")
    test.apply_subject_policy()
    test.expect_forwarded("POST", test.slides_path(f"presentations/{quote(slide_id, safe='')}:batchUpdate"), {"requests": []})
    test.expect_denied("GET", test.slides_path(f"presentations/{quote(slide_id, safe='')}"))
    test.expect_denied("POST", test.slides_path("presentations", include_ref=True), {"title": test.name("blocked")})


def slides_delete(test):
    test.apply_policy_with("setup-slides-delete", ["slides_create", "drive_files_create"])
    slide_id = test.create_slide("delete positive")
    file_id = test.create_drive_text_file("delete negative file")
    test.apply_subject_policy()
    test.expect_forwarded("DELETE", test.drive_path(f"files/{quote(slide_id, safe='')}"))
    test.forget_file(slide_id)
    test.expect_denied("DELETE", test.drive_path(f"files/{quote(file_id, safe='')}"))


SCENARIOS = {
    name: value
    for name, value in globals().items()
    if name.startswith(("gmail_", "contacts_", "drive_", "docs_", "sheets_", "slides_")) and callable(value)
}


def load_json(path):
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except FileNotFoundError:
        return None
    except json.JSONDecodeError as exc:
        raise TestFailure(f"Invalid JSON in {path}: {exc}") from exc


def load_test_config(args):
    agent_config = load_json(Path(args.agent_config))
    user_config = load_json(Path(args.user_config))
    legacy_config = load_json(Path(args.legacy_config))
    proxy_url = first_non_empty(os.environ.get("AIWP_TEST_BASE_URL"), value(agent_config, "proxy_url"), value(user_config, "proxy_url"), value(legacy_config, "proxy_url"))
    agent_token = first_non_empty(os.environ.get("AIWP_TEST_AGENT_API_TOKEN"), value(agent_config, "agent_api_token"))
    backend_token = first_non_empty(os.environ.get("AIWP_TEST_USER_BACKEND_API_TOKEN"), value(user_config, "user_backend_api_token"), value(legacy_config, "user_backend_api_token"))
    agent_id = first_non_empty(os.environ.get("AIWP_TEST_AGENT_ID"), value(agent_config, "agent_id"))
    workspace = first_non_empty(os.environ.get("AIWP_TEST_WORKSPACE"), single_workspace_selector(agent_config), single_workspace_selector(legacy_config))
    missing = []
    if not proxy_url:
        missing.append("proxy_url / AIWP_TEST_BASE_URL")
    if not agent_token:
        missing.append("agent_api_token / AIWP_TEST_AGENT_API_TOKEN")
    if not agent_id:
        missing.append("agent_id / AIWP_TEST_AGENT_ID")
    if not backend_token:
        missing.append("user_backend_api_token / AIWP_TEST_USER_BACKEND_API_TOKEN")
    if not workspace:
        missing.append("workspace / AIWP_TEST_WORKSPACE")
    if missing:
        raise TestFailure("Missing live-test configuration: " + ", ".join(missing))
    return {"proxy_url": proxy_url.rstrip("/"), "agent_token": agent_token, "backend_token": backend_token, "agent_id": agent_id, "workspace": workspace}


def request_json(base_url, method, path, token, body=None, expect_status=None):
    data = None
    headers = {"Authorization": f"Bearer {token}", "Accept": "application/json"}
    if body is not None:
        data = json.dumps(body).encode("utf-8")
        headers["Content-Type"] = "application/json"
    req = Request(base_url + path, data=data, headers=headers, method=method)
    try:
        with urlopen(req, timeout=90) as resp:
            status = resp.status
            raw = resp.read().decode("utf-8", errors="replace")
    except HTTPError as exc:
        status = exc.code
        raw = exc.read().decode("utf-8", errors="replace")
    except URLError as exc:
        raise TestFailure(f"Could not reach proxy: {exc}") from exc
    payload = {}
    if raw.strip():
        try:
            payload = json.loads(raw)
        except json.JSONDecodeError:
            payload = {"raw": raw}
    if not (200 <= status < 300):
        message = google_service_disabled_message(payload)
        if message:
            raise TestFailure(message)
    if expect_status is not None and status != expect_status:
        raise HTTPFailure(status, payload, raw)
    if expect_status is None and not (200 <= status < 300):
        raise HTTPFailure(status, payload, raw)
    return status, payload


def google_service_disabled_message(payload):
    if not isinstance(payload, dict):
        return ""
    error = payload.get("error")
    if not isinstance(error, dict):
        return ""
    details = error.get("details")
    if not isinstance(details, list):
        return ""
    for detail in details:
        if not isinstance(detail, dict) or detail.get("reason") != "SERVICE_DISABLED":
            continue
        metadata = detail.get("metadata")
        if not isinstance(metadata, dict):
            metadata = {}
        service_title = str(metadata.get("serviceTitle") or "Google API")
        service = str(metadata.get("service") or "").strip()
        activation_url = str(metadata.get("activationUrl") or "").strip()
        parts = [
            f"Google Cloud project prerequisite missing: {service_title} is disabled.",
        ]
        if service:
            parts.append(f"Service: {service}.")
        if activation_url:
            parts.append(f"Enable it here: {activation_url}")
        parts.append(f"Original Google error: {error.get('message')}")
        return " ".join(parts)
    return ""


def backend_json(cfg, method, path, body=None, expect_status=None):
    return request_json(cfg["proxy_url"], method, path, cfg["backend_token"], body, expect_status)


def agent_json(cfg, method, path, body=None, expect_status=None):
    return request_json(cfg["proxy_url"], method, path, cfg["agent_token"], body, expect_status)


def create_policy(cfg, name, capabilities):
    _, payload = backend_json(cfg, "POST", "/api/user/policies", {
        "name": name,
        "capabilities": capabilities,
    }, expect_status=201)
    policy = payload.get("policy")
    if not isinstance(policy, dict) or not policy.get("id"):
        raise TestFailure(f"Create policy response did not contain a policy ID: {payload}")
    return str(policy["id"])


def apply_policy_to_agent_grant(cfg, policy_id):
    backend_json(cfg, "PUT", f"/api/user/agents/{quote(cfg['agent_id'], safe='')}/grants", {
        "workspace": cfg["workspace"],
        "policy_id": policy_id,
    })


def delete_policy(cfg, policy_id):
    try:
        backend_json(cfg, "DELETE", f"/api/user/policies/{quote(policy_id, safe='')}")
    except HTTPFailure as exc:
        print(f"WARN: could not delete temporary policy {policy_id}: {exc}", file=sys.stderr)


def value(data, key):
    if isinstance(data, dict):
        raw = data.get(key)
        if isinstance(raw, str):
            return raw.strip()
    return ""


def first_non_empty(*values):
    for raw in values:
        if isinstance(raw, str) and raw.strip():
            return raw.strip()
    return ""


def single_workspace_selector(data):
    if not isinstance(data, dict):
        return ""
    workspaces = data.get("workspaces")
    if not isinstance(workspaces, list) or len(workspaces) != 1:
        return ""
    workspace = workspaces[0]
    if not isinstance(workspace, dict):
        return ""
    return str(workspace.get("name") or workspace.get("email") or "").strip()


def main():
    parser = argparse.ArgumentParser(description="Run the live policy permission scenario for AIWP_COVERAGE_ITEM_ID.")
    parser.add_argument("--agent-config", default=os.environ.get("AIWP_AGENT_CONFIG", str(DEFAULT_AGENT_CONFIG)))
    parser.add_argument("--user-config", default=os.environ.get("AIWP_USER_CONFIG", str(DEFAULT_USER_CONFIG)))
    parser.add_argument("--legacy-config", default=os.environ.get("AIWP_LEGACY_CONFIG", str(DEFAULT_LEGACY_CONFIG)))
    args = parser.parse_args()
    if os.environ.get("AIWP_LIVE_TESTS") != "1":
        print("SKIP: run testing/start-test.py --live TEST_ACCOUNT_EMAIL to include live Google Workspace tests.")
        return 0
    coverage_item = os.environ.get("AIWP_COVERAGE_ITEM_ID", "").strip()
    if not coverage_item.startswith("policy."):
        raise TestFailure("AIWP_COVERAGE_ITEM_ID must be set to a policy.* item by testing/start-test.py")
    policy_key = coverage_item.removeprefix("policy.")
    if policy_key not in SCENARIOS:
        raise TestFailure(f"No live dispatcher scenario exists for {policy_key}")
    cfg = load_test_config(args)
    LivePolicyPermissionTest(policy_key, cfg).run()
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (TestFailure, HTTPFailure) as exc:
        print(f"FAIL: {exc}", file=sys.stderr)
        raise SystemExit(1)
