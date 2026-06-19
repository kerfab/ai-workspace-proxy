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
from datetime import datetime, timedelta, timezone
from pathlib import Path
from urllib.error import HTTPError, URLError
from urllib.parse import quote, urlencode
from urllib.request import Request, urlopen


REPO_ROOT = Path(__file__).resolve().parents[2]
DEFAULT_AGENT_CONFIG = REPO_ROOT / "testing" / "config" / "agents-workspace-api-access.config.json"
DEFAULT_USER_CONFIG = REPO_ROOT / "testing" / "config" / "user-backend-api-access.config.json"
DEFAULT_LEGACY_CONFIG = REPO_ROOT / "testing" / "config" / "ai-workspace-proxy.json"
TEST_PREFIX = "AIWP_TEST"
DEFAULT_THIRD_PARTY_EMAIL = "noreply@google.com"


class TestFailure(Exception):
    pass


class HTTPFailure(Exception):
    def __init__(self, status, payload, body):
        super().__init__(f"HTTP {status}: {body}")
        self.status = status
        self.payload = payload
        self.body = body


class LiveCalendarTest:
    def __init__(self, slug, cfg):
        self.slug = slug
        self.cfg = cfg
        self.run_id = uuid.uuid4().hex[:10]
        self.created_events = []
        self.created_acl_rules = []
        self.created_calendars = []
        self.created_policies = []
        self.original_policy_id = ""
        self.cleanup_policy_id = ""

    def run(self, scenario):
        self.original_policy_id = self.agent_grant_policy_id()
        print(f"Workspace: {self.cfg['workspace']}")
        print(f"Calendar: {self.cfg['calendar_id']}")
        print(f"Original policy: {self.original_policy_id}")
        failure = None
        try:
            self.cleanup_policy_id = self.create_policy("cleanup", [
                "calendar_events_delete_self",
                "calendar_events_delete_guests",
                "calendar_read",
                "calendar_access_manage",
                "calendar_calendars_manage",
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

    def create_event(self, body):
        _, payload = agent_json(self.cfg, "POST", self.calendar_path("events", {"sendUpdates": "none"}), body)
        event_id = require_event_id(payload, "Create event response")
        self.created_events.append(event_id)
        print(f"Created event: {event_id}")
        return event_id

    def import_event(self, body):
        _, payload = agent_json(self.cfg, "POST", self.calendar_path("events/import"), body)
        event_id = require_event_id(payload, "Import event response")
        self.created_events.append(event_id)
        print(f"Imported event: {event_id}")
        return event_id

    def get_event(self, event_id):
        _, payload = agent_json(self.cfg, "GET", self.calendar_path(f"events/{quote(event_id, safe='')}"))
        return payload

    def patch_event(self, event_id, body):
        _, payload = agent_json(self.cfg, "PATCH", self.calendar_path(f"events/{quote(event_id, safe='')}", {"sendUpdates": "none"}), body)
        print(f"Patched event: {event_id}")
        return payload

    def put_event(self, event_id, body):
        _, payload = agent_json(self.cfg, "PUT", self.calendar_path(f"events/{quote(event_id, safe='')}", {"sendUpdates": "none"}), body)
        print(f"Updated event: {event_id}")
        return payload

    def delete_event(self, event_id):
        agent_json(self.cfg, "DELETE", self.calendar_path(f"events/{quote(event_id, safe='')}", {"sendUpdates": "none"}))
        self.forget_event(event_id)
        print(f"Deleted event: {event_id}")

    def create_calendar(self, summary):
        _, payload = agent_json(self.cfg, "POST", self.calendar_api_path("calendars"), {
            "summary": summary,
            "timeZone": "UTC",
        })
        calendar_id = str(payload.get("id") or "").strip()
        if not calendar_id:
            raise TestFailure(f"Create calendar response did not contain a calendar ID: {payload}")
        self.created_calendars.append(calendar_id)
        print(f"Created calendar: {calendar_id}")
        return calendar_id

    def patch_calendar(self, calendar_id, body):
        _, payload = agent_json(self.cfg, "PATCH", self.calendar_resource_path(calendar_id), body)
        print(f"Patched calendar: {calendar_id}")
        return payload

    def put_calendar(self, calendar_id, body):
        _, payload = agent_json(self.cfg, "PUT", self.calendar_resource_path(calendar_id), body)
        print(f"Updated calendar: {calendar_id}")
        return payload

    def clear_calendar(self, calendar_id):
        agent_json(self.cfg, "POST", self.calendar_resource_path(calendar_id, "clear"))
        print(f"Cleared calendar: {calendar_id}")

    def delete_calendar(self, calendar_id):
        agent_json(self.cfg, "DELETE", self.calendar_resource_path(calendar_id))
        self.forget_calendar(calendar_id)
        print(f"Deleted calendar: {calendar_id}")

    def calendar_list_path(self, calendar_id="", extra_query=None):
        if calendar_id:
            return self.calendar_api_path(f"users/me/calendarList/{quote(calendar_id, safe='')}", extra_query)
        return self.calendar_api_path("users/me/calendarList", extra_query)

    def acl_path(self, calendar_id, rule_id="", suffix="", extra_query=None):
        path = f"calendars/{quote(calendar_id, safe='')}/acl"
        if rule_id:
            path += "/" + quote(rule_id, safe="")
        if suffix:
            path += "/" + suffix.strip("/")
        return self.calendar_api_path(path, extra_query)

    def create_acl_rule(self, calendar_id, body, extra_query=None):
        _, payload = agent_json(self.cfg, "POST", self.acl_path(calendar_id, extra_query=extra_query), body)
        rule_id = str(payload.get("id") or "").strip()
        if not rule_id:
            raise TestFailure(f"Create ACL response did not contain an ACL rule ID: {payload}")
        self.created_acl_rules.append((calendar_id, rule_id))
        print(f"Created ACL rule: {rule_id}")
        return rule_id

    def patch_acl_rule(self, calendar_id, rule_id, body):
        _, payload = agent_json(self.cfg, "PATCH", self.acl_path(calendar_id, rule_id), body)
        print(f"Patched ACL rule: {rule_id}")
        return payload

    def put_acl_rule(self, calendar_id, rule_id, body):
        _, payload = agent_json(self.cfg, "PUT", self.acl_path(calendar_id, rule_id), body)
        print(f"Updated ACL rule: {rule_id}")
        return payload

    def delete_acl_rule(self, calendar_id, rule_id):
        agent_json(self.cfg, "DELETE", self.acl_path(calendar_id, rule_id))
        self.forget_acl_rule(calendar_id, rule_id)
        print(f"Deleted ACL rule: {rule_id}")

    def expect_denied(self, method, path, body=None, want_text=""):
        try:
            _, payload = agent_json(self.cfg, method, path, body, expect_status=403)
        except HTTPFailure as exc:
            event_id = exc.payload.get("id") if isinstance(exc.payload, dict) else ""
            if event_id:
                self.created_events.append(str(event_id))
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

    def calendar_path(self, suffix, extra_query=None):
        calendar_id = quote(self.cfg["calendar_id"], safe="")
        query = {"workspace": self.cfg["workspace"]}
        if extra_query:
            query.update(extra_query)
        return f"/calendar.googleapis.com/calendar/v3/calendars/{calendar_id}/{suffix}?{urlencode(query)}"

    def calendar_resource_path(self, calendar_id, suffix="", extra_query=None):
        path = f"calendars/{quote(calendar_id, safe='')}"
        if suffix:
            path += "/" + suffix.strip("/")
        return self.calendar_api_path(path, extra_query)

    def calendar_api_path(self, suffix, extra_query=None):
        query = {"workspace": self.cfg["workspace"]}
        if extra_query:
            query.update(extra_query)
        return f"/calendar.googleapis.com/calendar/v3/{suffix}?{urlencode(query)}"

    def start_time(self, offset_hours=0):
        return datetime.now(timezone.utc).replace(microsecond=0) + timedelta(days=2, hours=offset_hours)

    def summary(self, label):
        return f"{TEST_PREFIX} {self.slug} {label} {self.run_id}"

    def forget_event(self, event_id):
        self.created_events = [existing for existing in self.created_events if existing != event_id]

    def forget_acl_rule(self, calendar_id, rule_id):
        self.created_acl_rules = [
            existing for existing in self.created_acl_rules
            if existing != (calendar_id, rule_id)
        ]

    def forget_calendar(self, calendar_id):
        self.created_calendars = [existing for existing in self.created_calendars if existing != calendar_id]

    def require_acl_email(self):
        acl_email = self.cfg.get("acl_email", "").strip()
        if not acl_email or "@" not in acl_email:
            raise TestFailure("Calendar sharing live tests require a third-party email address")
        if acl_email.lower() == self.cfg["workspace"].lower():
            raise TestFailure("Calendar sharing live tests cannot use the Workspace test account as the third-party email")
        return acl_email

    def cleanup(self):
        if self.cleanup_policy_id and (self.created_events or self.created_acl_rules or self.created_calendars):
            try:
                self.apply_policy(self.cleanup_policy_id)
            except Exception as exc:
                print(f"WARN: could not apply cleanup policy: {exc}", file=sys.stderr)
        for event_id in reversed(list(self.created_events)):
            try:
                agent_json(self.cfg, "DELETE", self.calendar_path(f"events/{quote(event_id, safe='')}", {"sendUpdates": "none"}))
                print(f"Cleanup requested for event: {event_id}")
            except HTTPFailure as exc:
                if exc.status not in {404, 410}:
                    print(f"WARN: could not delete event {event_id}: {exc}", file=sys.stderr)
        self.created_events = []
        for calendar_id, rule_id in reversed(list(self.created_acl_rules)):
            try:
                agent_json(self.cfg, "DELETE", self.acl_path(calendar_id, rule_id))
                print(f"Cleanup requested for ACL rule: {rule_id}")
            except HTTPFailure as exc:
                if exc.status not in {404, 410}:
                    print(f"WARN: could not delete ACL rule {rule_id}: {exc}", file=sys.stderr)
        self.created_acl_rules = []
        for calendar_id in reversed(list(self.created_calendars)):
            try:
                agent_json(self.cfg, "DELETE", self.calendar_resource_path(calendar_id))
                print(f"Cleanup requested for calendar: {calendar_id}")
            except HTTPFailure as exc:
                if exc.status not in {404, 410}:
                    print(f"WARN: could not delete calendar {calendar_id}: {exc}", file=sys.stderr)
        self.created_calendars = []
        if self.original_policy_id:
            try:
                apply_policy_to_agent_grant(self.cfg, self.original_policy_id)
                print(f"Restored original policy: {self.original_policy_id}")
            except Exception as exc:
                print(f"WARN: could not restore original policy {self.original_policy_id}: {exc}", file=sys.stderr)
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
    calendar_id = first_non_empty(os.environ.get("AIWP_TEST_CALENDAR_ID"), "primary")
    third_party_email = first_non_empty(
        os.environ.get("AIWP_TEST_THIRD_PARTY_EMAIL"),
        DEFAULT_THIRD_PARTY_EMAIL,
    )
    acl_email = first_non_empty(
        os.environ.get("AIWP_TEST_CALENDAR_ACL_EMAIL"),
        third_party_email,
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
        "calendar_id": calendar_id,
        "third_party_email": third_party_email,
        "acl_email": acl_email,
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
        with urlopen(req, timeout=60) as resp:
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


def require_event_id(payload, label):
    event_id = payload.get("id") if isinstance(payload, dict) else ""
    if not event_id:
        raise TestFailure(f"{label} did not contain an event ID")
    return str(event_id)


def event_body(summary, start_dt, minutes=30, attendees=None, i_cal_uid=""):
    body = {
        "summary": summary,
        "description": "Disposable event created by AI Workspace Proxy live policy test.",
        "start": {"dateTime": start_dt.isoformat().replace("+00:00", "Z")},
        "end": {"dateTime": (start_dt + timedelta(minutes=minutes)).isoformat().replace("+00:00", "Z")},
    }
    if attendees is not None:
        body["attendees"] = [{"email": email} for email in attendees]
    if i_cal_uid:
        body["iCalUID"] = i_cal_uid
    return body


def run_live_calendar_scenario(slug, scenario):
    parser = argparse.ArgumentParser(description=f"Live Calendar policy test: {slug}.")
    parser.add_argument("--agent-config", default=os.environ.get("AIWP_AGENT_CONFIG", str(DEFAULT_AGENT_CONFIG)))
    parser.add_argument("--user-config", default=os.environ.get("AIWP_USER_CONFIG", str(DEFAULT_USER_CONFIG)))
    parser.add_argument("--legacy-config", default=os.environ.get("AIWP_LEGACY_CONFIG", str(DEFAULT_LEGACY_CONFIG)))
    args = parser.parse_args()

    if os.environ.get("AIWP_LIVE_TESTS") != "1":
        print("SKIP: run testing/start-test.py --live TEST_ACCOUNT_EMAIL to include live Google Workspace tests.")
        return 0

    try:
        cfg = load_test_config(args)
        LiveCalendarTest(slug, cfg).run(scenario)
        return 0
    except (TestFailure, HTTPFailure) as exc:
        print(f"FAIL: {exc}", file=sys.stderr)
        return 1
