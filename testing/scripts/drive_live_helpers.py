#!/usr/bin/env python3
# Copyright (c) 2026 Opensense Ltd. (Hong Kong). All rights reserved.
# Proprietary software. No use, copy, modification, distribution, disclosure,
# or reverse engineering is permitted without prior written authorization
# from Opensense Ltd.

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


class LiveDriveTest:
    def __init__(self, slug, cfg):
        self.slug = slug
        self.cfg = cfg
        self.run_id = uuid.uuid4().hex[:10]
        self.created_files = []
        self.created_policies = []
        self.original_policy_id = ""
        self.cleanup_policy_id = ""
        self.drive_ref = {}
        self.original_drive_folder_ref_ids = None

    def run(self, scenario):
        self.original_policy_id = self.agent_grant_policy_id()
        self.drive_ref = self.find_test_drive_ref()
        self.ensure_agent_drive_folder_grant(self.drive_ref)
        print(f"Workspace: {self.cfg['workspace']}")
        print(f"Allowed Drive folder: {self.drive_ref['reference_name']}")
        print(f"Original policy: {self.original_policy_id}")
        failure = None
        try:
            self.cleanup_policy_id = self.create_policy("cleanup", [
                "docs_delete",
                "drive_files_delete",
                "sheets_delete",
                "slides_delete",
            ])
            scenario(self)
        except Exception as exc:
            failure = exc
        finally:
            self.cleanup()
        if failure is not None:
            raise failure
        print(f"PASS: {self.slug} live policy test")

    def create_policy(self, label, capabilities):
        policy_id = create_policy(self.cfg, f"AIWP_TEST {self.slug} {label} {self.run_id}", capabilities)
        self.created_policies.append(policy_id)
        return policy_id

    def apply_policy(self, policy_id):
        apply_policy_to_agent_grant(self.cfg, policy_id)
        print(f"Applied temporary policy: {policy_id}")

    def agent_grant_policy_id(self):
        query = urlencode({"workspace": self.cfg["workspace"]})
        _, payload = backend_json(self.cfg, "GET", f"/api/user/agents/{quote(self.cfg['agent_id'], safe='')}/grants?{query}")
        grant = payload.get("grant")
        if not isinstance(grant, dict):
            raise TestFailure("Agent grant lookup response did not contain a grant object")
        policy_id = str(grant.get("policy_id") or "system").strip()
        return policy_id or "system"

    def find_test_drive_ref(self):
        query = urlencode({"workspace": self.cfg["workspace"]})
        _, payload = backend_json(self.cfg, "GET", f"/api/user/drive-folders?{query}")
        folders = payload.get("drive_folders")
        if not isinstance(folders, list):
            raise TestFailure("Drive folder lookup response did not contain a drive_folders array")
        for folder in folders:
            if not isinstance(folder, dict):
                continue
            reference_name = str(folder.get("reference_name") or "").strip()
            if "test" in reference_name.lower():
                return folder
        available = ", ".join(
            str(folder.get("reference_name") or "").strip()
            for folder in folders
            if isinstance(folder, dict) and str(folder.get("reference_name") or "").strip()
        ) or "<none>"
        raise TestFailure(
            "No allowed Drive folder reference containing 'Test' was found for this Workspace. "
            f"Available references: {available}"
        )

    def agent_drive_folder_grant_ref_ids(self):
        query = urlencode({"workspace": self.cfg["workspace"]})
        _, payload = backend_json(
            self.cfg,
            "GET",
            f"/api/user/agents/{quote(self.cfg['agent_id'], safe='')}/grants/drive-folders?{query}",
        )
        folders = payload.get("drive_folders")
        if not isinstance(folders, list):
            raise TestFailure("Agent Drive folder grant lookup response did not contain drive_folders")
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

    def require_folder_type(self, field, label):
        _ = field
        _ = label

    def create_doc(self, label):
        self.require_folder_type("allow_docs", "Google Docs")
        _, payload = agent_json(self.cfg, "POST", self.docs_path("documents", include_ref=True), {
            "title": self.name(label),
        })
        doc_id = str(payload.get("documentId") or "").strip()
        if not doc_id:
            raise TestFailure(f"Docs create response did not contain documentId: {payload}")
        self.created_files.append(doc_id)
        print(f"Created Doc: {doc_id}")
        return doc_id

    def create_doc_via_drive(self, label):
        self.require_folder_type("allow_docs", "Google Docs")
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
        doc_id = str(payload.get("id") or "").strip()
        if not doc_id:
            raise TestFailure(f"Drive-native Docs create response did not contain id: {payload}")
        self.created_files.append(doc_id)
        print(f"Created Doc via Drive API: {doc_id}")
        return doc_id

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
        print(f"Created Drive file: {file_id}")
        return file_id

    def batch_update_doc(self, doc_id, text):
        path = self.docs_path(f"documents/{quote(doc_id, safe='')}:batchUpdate")
        _, payload = agent_json(self.cfg, "POST", path, {
            "requests": [
                {
                    "insertText": {
                        "location": {"index": 1},
                        "text": text,
                    }
                }
            ]
        })
        print(f"Updated Doc: {doc_id}")
        return payload

    def create_comment(self, file_id, label):
        _, payload = agent_json(self.cfg, "POST", self.drive_path(
            f"files/{quote(file_id, safe='')}/comments",
            extra_query={"fields": "id,content"},
        ), {
            "content": self.name(label),
        })
        comment_id = str(payload.get("id") or "").strip()
        if not comment_id:
            raise TestFailure(f"Comment create response did not contain id: {payload}")
        print(f"Created comment: {comment_id}")
        return comment_id

    def create_reply(self, file_id, comment_id, label):
        _, payload = agent_json(self.cfg, "POST", self.drive_path(
            f"files/{quote(file_id, safe='')}/comments/{quote(comment_id, safe='')}/replies",
            extra_query={"fields": "id,content"},
        ), {
            "content": self.name(label),
        })
        reply_id = str(payload.get("id") or "").strip()
        if not reply_id:
            raise TestFailure(f"Reply create response did not contain id: {payload}")
        print(f"Created reply: {reply_id}")
        return reply_id

    def list_revisions(self, file_id):
        _, payload = agent_json(self.cfg, "GET", self.drive_path(
            f"files/{quote(file_id, safe='')}/revisions",
            extra_query={"fields": "revisions(id,modifiedTime,keepForever)"},
        ))
        revisions = payload.get("revisions")
        if not isinstance(revisions, list):
            raise TestFailure(f"Revisions list response did not contain revisions array: {payload}")
        return revisions

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

    def expect_success(self, method, path, body=None):
        status, payload = agent_json(self.cfg, method, path, body)
        print(f"Request succeeded with HTTP {status}: {method} {path}")
        return status, payload

    def expect_forwarded(self, method, path, body=None):
        try:
            return self.expect_success(method, path, body)
        except HTTPFailure as exc:
            if isinstance(exc.payload, dict) and isinstance(exc.payload.get("error"), dict):
                print(f"Forwarded request reached Google and returned HTTP {exc.status}: {method} {path}")
                return exc.status, exc.payload
            raise TestFailure(f"Expected request to be forwarded to Google, got proxy error {exc.status}: {exc.body}") from exc

    def drive_path(self, suffix, include_ref=False, extra_query=None):
        query = {"workspace": self.cfg["workspace"]}
        if include_ref:
            query["driveRef"] = self.drive_ref["reference_name"]
        if extra_query:
            query.update(extra_query)
        return f"/drive.googleapis.com/drive/v3/{suffix}?{urlencode(query)}"

    def docs_path(self, suffix, include_ref=False, extra_query=None):
        query = {"workspace": self.cfg["workspace"]}
        if include_ref:
            query["driveRef"] = self.drive_ref["reference_name"]
        if extra_query:
            query.update(extra_query)
        return f"/docs.googleapis.com/v1/{suffix}?{urlencode(query)}"

    def name(self, label):
        return f"{TEST_PREFIX} {self.slug} {label} {self.run_id}"

    def forget_file(self, file_id):
        self.created_files = [existing for existing in self.created_files if existing != file_id]

    def cleanup(self):
        if self.cleanup_policy_id and self.created_files:
            try:
                self.apply_policy(self.cleanup_policy_id)
            except Exception as exc:
                print(f"WARN: could not apply cleanup policy: {exc}", file=sys.stderr)
        for file_id in reversed(list(self.created_files)):
            try:
                agent_json(self.cfg, "DELETE", self.drive_path(f"files/{quote(file_id, safe='')}"))
                self.forget_file(file_id)
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
    proxy_url = first_non_empty(
        os.environ.get("AIWP_TEST_BASE_URL"),
        value(agent_config, "proxy_url"),
        value(user_config, "proxy_url"),
        value(legacy_config, "proxy_url"),
    )
    agent_token = first_non_empty(
        os.environ.get("AIWP_TEST_AGENT_API_TOKEN"),
        value(agent_config, "agent_api_token"),
    )
    backend_token = first_non_empty(
        os.environ.get("AIWP_TEST_USER_BACKEND_API_TOKEN"),
        value(user_config, "user_backend_api_token"),
        value(legacy_config, "user_backend_api_token"),
    )
    agent_id = first_non_empty(
        os.environ.get("AIWP_TEST_AGENT_ID"),
        value(agent_config, "agent_id"),
    )
    workspace = first_non_empty(
        os.environ.get("AIWP_TEST_WORKSPACE"),
        single_workspace_selector(agent_config),
        single_workspace_selector(legacy_config),
    )
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
    return {
        "proxy_url": proxy_url.rstrip("/"),
        "agent_token": agent_token,
        "backend_token": backend_token,
        "agent_id": agent_id,
        "workspace": workspace,
    }


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


def request_json(base_url, method, path, token, body=None, expect_status=None):
    data = None
    headers = {
        "Authorization": f"Bearer {token}",
        "Accept": "application/json",
    }
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
        raise TestFailure("Create policy response did not contain a policy ID")
    return str(policy["id"])


def apply_policy_to_agent_grant(cfg, policy_id):
    backend_json(cfg, "PUT", f"/api/user/agents/{quote(cfg['agent_id'], safe='')}/grants", {
        "workspace": cfg["workspace"],
        "policy_id": policy_id,
    })


def delete_policy(cfg, policy_id):
    encoded = quote(policy_id, safe="")
    try:
        backend_json(cfg, "DELETE", f"/api/user/policies/{encoded}")
    except HTTPFailure as exc:
        print(f"WARN: could not delete temporary policy {policy_id}: {exc}", file=sys.stderr)


def run_live_drive_scenario(slug, scenario):
    parser = argparse.ArgumentParser(description=f"Live Drive/Docs policy test: {slug}.")
    parser.add_argument("--agent-config", default=os.environ.get("AIWP_AGENT_CONFIG", str(DEFAULT_AGENT_CONFIG)))
    parser.add_argument("--user-config", default=os.environ.get("AIWP_USER_CONFIG", str(DEFAULT_USER_CONFIG)))
    parser.add_argument("--legacy-config", default=os.environ.get("AIWP_LEGACY_CONFIG", str(DEFAULT_LEGACY_CONFIG)))
    args = parser.parse_args()

    if os.environ.get("AIWP_LIVE_TESTS") != "1":
        print("SKIP: run testing/start-test.py --live TEST_ACCOUNT_EMAIL to include live Google Workspace tests.")
        return 0

    try:
        cfg = load_test_config(args)
        LiveDriveTest(slug, cfg).run(scenario)
        return 0
    except (TestFailure, HTTPFailure) as exc:
        print(f"FAIL: {exc}", file=sys.stderr)
        return 1
