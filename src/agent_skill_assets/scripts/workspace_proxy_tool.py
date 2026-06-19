#!/usr/bin/env python3
# Copyright (c) 2026 Opensense Ltd. (Hong Kong). All rights reserved.
# Proprietary software. No use, copy, modification, distribution, disclosure,
# or reverse engineering is permitted without prior written authorization
# from Opensense Ltd.

import argparse
import base64
import json
import mimetypes
import os
import sys
from email.message import EmailMessage
from pathlib import Path
from typing import Any, Dict, List, Optional
from urllib.parse import urlencode
from urllib.request import Request, urlopen
from urllib.error import HTTPError, URLError

PACKAGE_ROOT = Path(__file__).resolve().parent.parent
LOCAL_CONFIG_PATH = PACKAGE_ROOT / "config" / "agents-workspace-api-access.config.json"
ACTIVE_WORKSPACE = ""
ACTIVE_AGENT_MOTIVE = ""
ACTIVE_HUMAN_APPROVAL = ""


def config_path() -> Path:
    env_path = os.environ.get("AI_WORKSPACE_PROXY_CONFIG", "").strip()
    if env_path:
        return Path(env_path)
    return LOCAL_CONFIG_PATH


def die(msg: str, code: int = 1):
    print(msg, file=sys.stderr)
    raise SystemExit(code)


def load_config():
    path = config_path()
    if not path.exists():
        die(f"Missing config file: {path}")
    try:
        data = json.loads(path.read_text(encoding="utf-8"))
    except Exception as e:
        die(f"Invalid JSON in {path}: {e}")
    proxy_url = str(data.get("proxy_url", "")).rstrip("/")
    agent_api_token = str(data.get("agent_api_token", "")).strip()
    if not proxy_url:
        die("Config error: proxy_url is missing")
    if not agent_api_token:
        die("Config error: agent_api_token is missing")
    workspaces = data.get("workspaces", [])
    if workspaces is None:
        workspaces = []
    if not isinstance(workspaces, list):
        die("Config error: workspaces must be an array")
    default_workspace = ""
    if len(workspaces) == 1 and isinstance(workspaces[0], dict):
        default_workspace = str(workspaces[0].get("name") or workspaces[0].get("email") or "").strip()
    return proxy_url, agent_api_token, default_workspace


def read_json_file(path: str) -> Any:
    return json.loads(Path(path).read_text(encoding="utf-8"))


def read_text_file(path: str) -> str:
    return Path(path).read_text(encoding="utf-8")


def agent_context_headers() -> Dict[str, str]:
    headers: Dict[str, str] = {}
    if ACTIVE_AGENT_MOTIVE:
        headers["X-AIWP-Agent-Motive"] = ACTIVE_AGENT_MOTIVE
    if ACTIVE_HUMAN_APPROVAL:
        headers["X-AIWP-Human-Approval"] = ACTIVE_HUMAN_APPROVAL
    return headers


def request_json_result(method: str, path: str, token: str, body: Any = None, query: Optional[Dict[str, Any]] = None):
    proxy_url, _, _ = load_config()
    url = proxy_url + path
    query = dict(query or {})
    if ACTIVE_WORKSPACE:
        query.setdefault("workspace", ACTIVE_WORKSPACE)
    if query:
        qs = urlencode(query, doseq=True)
        if qs:
            url += "?" + qs

    payload = None
    headers = {"Authorization": f"Bearer {token}", "Accept": "application/json"}
    headers.update(agent_context_headers())
    if body is not None:
        payload = json.dumps(body).encode("utf-8")
        headers["Content-Type"] = "application/json"

    req = Request(url, data=payload, method=method.upper())
    for k, v in headers.items():
        req.add_header(k, v)

    try:
        with urlopen(req) as resp:
            raw = resp.read().decode("utf-8")
            if not raw.strip():
                return {}, ""
            return json.loads(raw), ""
    except HTTPError as e:
        detail = e.read().decode("utf-8", errors="replace")
        return None, f"Proxy/Workspace HTTP {e.code}: {detail}"
    except URLError as e:
        return None, f"Connection error: {e}"


def request_json(method: str, path: str, token: str, body: Any = None, query: Optional[Dict[str, Any]] = None):
    payload, error = request_json_result(method, path, token, body=body, query=query)
    if error:
        die(error, 2)
    return payload


def request_json_optional(method: str, path: str, token: str, body: Any = None, query: Optional[Dict[str, Any]] = None):
    return request_json_result(method, path, token, body=body, query=query)


def request_bytes(method: str, path: str, token: str, query: Optional[Dict[str, Any]] = None) -> bytes:
    proxy_url, _, _ = load_config()
    url = proxy_url + path
    query = dict(query or {})
    if ACTIVE_WORKSPACE:
        query.setdefault("workspace", ACTIVE_WORKSPACE)
    if query:
        qs = urlencode(query, doseq=True)
        if qs:
            url += "?" + qs
    req = Request(url, method=method.upper())
    req.add_header("Authorization", f"Bearer {token}")
    req.add_header("Accept", "*/*")
    for k, v in agent_context_headers().items():
        req.add_header(k, v)
    try:
        with urlopen(req) as resp:
            return resp.read()
    except HTTPError as e:
        detail = e.read().decode("utf-8", errors="replace")
        die(f"Proxy/Workspace HTTP {e.code}: {detail}", 2)
    except URLError as e:
        die(f"Connection error: {e}", 2)


def request_raw(method: str, path: str, token: str, body: Any = None, query: Optional[Dict[str, Any]] = None) -> bytes:
    proxy_url, _, _ = load_config()
    url = proxy_url + path
    query = dict(query or {})
    if ACTIVE_WORKSPACE:
        query.setdefault("workspace", ACTIVE_WORKSPACE)
    if query:
        qs = urlencode(query, doseq=True)
        if qs:
            url += "?" + qs
    payload = None
    headers = {"Authorization": f"Bearer {token}", "Accept": "*/*"}
    headers.update(agent_context_headers())
    if body is not None:
        payload = json.dumps(body).encode("utf-8")
        headers["Content-Type"] = "application/json"
    req = Request(url, data=payload, method=method.upper())
    for k, v in headers.items():
        req.add_header(k, v)
    try:
        with urlopen(req) as resp:
            return resp.read()
    except HTTPError as e:
        detail = e.read().decode("utf-8", errors="replace")
        die(f"Proxy/Workspace HTTP {e.code}: {detail}", 2)
    except URLError as e:
        die(f"Connection error: {e}", 2)


def parse_query_pairs(pairs: List[str]) -> Dict[str, Any]:
    out: Dict[str, Any] = {}
    for pair in pairs:
        if "=" not in pair:
            die(f"Invalid query pair {pair!r}; expected KEY=VALUE")
        key, value = pair.split("=", 1)
        if key in out:
            existing = out[key]
            if isinstance(existing, list):
                existing.append(value)
            else:
                out[key] = [existing, value]
        else:
            out[key] = value
    return out


def save_bytes(data: bytes, save_to: str) -> Dict[str, Any]:
    out = Path(save_to)
    out.parent.mkdir(parents=True, exist_ok=True)
    out.write_bytes(data)
    return {"savedTo": str(out), "size": len(data)}


def b64url_encode(data: bytes) -> str:
    return base64.urlsafe_b64encode(data).decode("utf-8").rstrip("=")


def b64url_decode(data: str) -> bytes:
    padding = "=" * ((4 - len(data) % 4) % 4)
    return base64.urlsafe_b64decode(data + padding)


def attach_files(msg: EmailMessage, attach_paths: List[str]):
    for path_str in attach_paths:
        path = Path(path_str)
        if not path.exists():
            die(f"Attachment not found: {path}")
        data = path.read_bytes()
        ctype, encoding = mimetypes.guess_type(path.name)
        if ctype is None or encoding is not None:
            ctype = "application/octet-stream"
        maintype, subtype = ctype.split("/", 1)
        msg.add_attachment(data, maintype=maintype, subtype=subtype, filename=path.name)


def make_plain_message(to_addr: str, subject: str, body: str, cc: str = "", bcc: str = "", attach_paths: Optional[List[str]] = None) -> str:
    attach_paths = attach_paths or []
    msg = EmailMessage()
    msg["To"] = to_addr
    if cc:
        msg["Cc"] = cc
    if bcc:
        msg["Bcc"] = bcc
    msg["Subject"] = subject
    msg.set_content(body)
    attach_files(msg, attach_paths)
    return b64url_encode(msg.as_bytes())


def header_map(payload) -> dict:
    headers = {}
    for h in payload.get("headers", []) or []:
        name = h.get("name", "")
        value = h.get("value", "")
        if name:
            headers[name.lower()] = value
    return headers


def get_message(token: str, message_id: str, fmt: str = "metadata"):
    query: Dict[str, Any] = {"format": fmt}
    if fmt == "metadata":
        query["metadataHeaders"] = [
            "From", "To", "Cc", "Subject", "Date", "Message-ID", "In-Reply-To", "References", "Reply-To"
        ]
    return request_json("GET", f"/gmail.googleapis.com/gmail/v1/users/me/messages/{message_id}", token, query=query)


def get_thread(token: str, thread_id: str, fmt: str = "full"):
    return request_json("GET", f"/gmail.googleapis.com/gmail/v1/users/me/threads/{thread_id}", token, query={"format": fmt})


def list_messages(token: str, query_str: str = "", max_results: int = 10):
    query: Dict[str, Any] = {"maxResults": str(max_results)}
    if query_str:
        query["q"] = query_str
    return request_json("GET", "/gmail.googleapis.com/gmail/v1/users/me/messages", token, query=query)


def list_unread(token: str, max_results: int = 10):
    data = list_messages(token, "is:unread", max_results)
    msgs = data.get("messages", []) or []
    out = {"resultSizeEstimate": data.get("resultSizeEstimate", 0), "messages": []}
    for m in msgs:
        detail = get_message(token, m["id"], "metadata")
        payload = detail.get("payload", {}) or {}
        headers = header_map(payload)
        out["messages"].append({
            "id": detail.get("id"),
            "threadId": detail.get("threadId"),
            "from": headers.get("from", ""),
            "to": headers.get("to", ""),
            "cc": headers.get("cc", ""),
            "subject": headers.get("subject", ""),
            "date": headers.get("date", ""),
            "snippet": detail.get("snippet", ""),
        })
    return out


def list_drafts(token: str):
    return request_json("GET", "/gmail.googleapis.com/gmail/v1/users/me/drafts", token)


def get_draft(token: str, draft_id: str):
    return request_json("GET", f"/gmail.googleapis.com/gmail/v1/users/me/drafts/{draft_id}", token)


def create_draft(token: str, to_addr: str, subject: str, body: str, cc: str = "", bcc: str = "", attach_paths: Optional[List[str]] = None):
    raw = make_plain_message(to_addr, subject, body, cc=cc, bcc=bcc, attach_paths=attach_paths)
    return request_json("POST", "/gmail.googleapis.com/gmail/v1/users/me/drafts", token, body={"message": {"raw": raw}})


def update_draft(token: str, draft_id: str, to_addr: str, subject: str, body: str, cc: str = "", bcc: str = "", attach_paths: Optional[List[str]] = None):
    raw = make_plain_message(to_addr, subject, body, cc=cc, bcc=bcc, attach_paths=attach_paths)
    return request_json("PUT", f"/gmail.googleapis.com/gmail/v1/users/me/drafts/{draft_id}", token, body={"id": draft_id, "message": {"raw": raw}})


def delete_draft(token: str, draft_id: str):
    return request_json("DELETE", f"/gmail.googleapis.com/gmail/v1/users/me/drafts/{draft_id}", token)


def build_reply_raw(orig_message: dict, body: str, attach_paths: Optional[List[str]] = None) -> str:
    payload = orig_message.get("payload", {}) or {}
    headers = header_map(payload)
    subject = headers.get("subject", "")
    if subject and not subject.lower().startswith("re:"):
        subject = "Re: " + subject
    to_addr = headers.get("reply-to") or headers.get("from", "")
    msg = EmailMessage()
    if to_addr:
        msg["To"] = to_addr
    if subject:
        msg["Subject"] = subject
    message_id = headers.get("message-id", "")
    in_reply_to = headers.get("in-reply-to", "")
    references = headers.get("references", "")
    if message_id:
        msg["In-Reply-To"] = message_id
        msg["References"] = ((references + " ") if references else "") + message_id
    elif in_reply_to:
        msg["In-Reply-To"] = in_reply_to
        if references:
            msg["References"] = references
    msg.set_content(body)
    attach_files(msg, attach_paths or [])
    return b64url_encode(msg.as_bytes())


def create_reply_draft(token: str, message_id: str, body: str, attach_paths: Optional[List[str]] = None):
    original = get_message(token, message_id, "metadata")
    raw = build_reply_raw(original, body, attach_paths=attach_paths)
    payload = {"message": {"threadId": original.get("threadId"), "raw": raw}}
    return request_json("POST", "/gmail.googleapis.com/gmail/v1/users/me/drafts", token, body=payload)


def modify_message_labels(token: str, message_id: str, add_labels: Optional[List[str]] = None, remove_labels: Optional[List[str]] = None):
    return request_json("POST", f"/gmail.googleapis.com/gmail/v1/users/me/messages/{message_id}/modify", token, body={
        "addLabelIds": add_labels or [],
        "removeLabelIds": remove_labels or [],
    })


def modify_thread_labels(token: str, thread_id: str, add_labels: Optional[List[str]] = None, remove_labels: Optional[List[str]] = None):
    return request_json("POST", f"/gmail.googleapis.com/gmail/v1/users/me/threads/{thread_id}/modify", token, body={
        "addLabelIds": add_labels or [],
        "removeLabelIds": remove_labels or [],
    })


def list_labels(token: str):
    return request_json("GET", "/gmail.googleapis.com/gmail/v1/users/me/labels", token)


def get_label(token: str, label_id: str):
    return request_json("GET", f"/gmail.googleapis.com/gmail/v1/users/me/labels/{label_id}", token)


def create_label(token: str, body: Dict[str, Any]):
    return request_json("POST", "/gmail.googleapis.com/gmail/v1/users/me/labels", token, body=body)


def patch_label(token: str, label_id: str, body: Dict[str, Any]):
    return request_json("PATCH", f"/gmail.googleapis.com/gmail/v1/users/me/labels/{label_id}", token, body=body)


def delete_label(token: str, label_id: str):
    return request_json("DELETE", f"/gmail.googleapis.com/gmail/v1/users/me/labels/{label_id}", token)


def trash_message(token: str, message_id: str):
    return request_json("POST", f"/gmail.googleapis.com/gmail/v1/users/me/messages/{message_id}/trash", token)


def untrash_message(token: str, message_id: str):
    return request_json("POST", f"/gmail.googleapis.com/gmail/v1/users/me/messages/{message_id}/untrash", token)


def trash_thread(token: str, thread_id: str):
    return request_json("POST", f"/gmail.googleapis.com/gmail/v1/users/me/threads/{thread_id}/trash", token)


def untrash_thread(token: str, thread_id: str):
    return request_json("POST", f"/gmail.googleapis.com/gmail/v1/users/me/threads/{thread_id}/untrash", token)


def delete_message(token: str, message_id: str):
    return request_json("DELETE", f"/gmail.googleapis.com/gmail/v1/users/me/messages/{message_id}", token)


def list_calendars(token: str):
    return request_json("GET", "/calendar.googleapis.com/calendar/v3/users/me/calendarList", token)


def get_calendar(token: str, calendar_id: str):
    return request_json("GET", f"/calendar.googleapis.com/calendar/v3/users/me/calendarList/{calendar_id}", token)


def list_events(token: str, calendar_id: str, time_min: str = "", time_max: str = "", query_str: str = "", max_results: int = 20):
    query: Dict[str, Any] = {"maxResults": str(max_results)}
    if time_min:
        query["timeMin"] = time_min
    if time_max:
        query["timeMax"] = time_max
    if query_str:
        query["q"] = query_str
    return request_json("GET", f"/calendar.googleapis.com/calendar/v3/calendars/{calendar_id}/events", token, query=query)


def get_event(token: str, calendar_id: str, event_id: str):
    return request_json("GET", f"/calendar.googleapis.com/calendar/v3/calendars/{calendar_id}/events/{event_id}", token)


def add_drive_location_query(query: Dict[str, Any], ref: str = "", drive_path: str = "", drive_folder_id: str = ""):
    if ref:
        query["driveRef"] = ref
    if drive_path:
        query["drivePath"] = drive_path
    if drive_folder_id:
        query["driveFolderId"] = drive_folder_id
    return query


def drive_folder_tree(token: str, ref: str = ""):
    query: Dict[str, Any] = {}
    if ref:
        query["driveRef"] = ref
    return request_json("GET", "/api/drive-folders/tree", token, query=query)


def drive_refresh_folders(token: str, ref: str = ""):
    query: Dict[str, Any] = {}
    if ref:
        query["driveRef"] = ref
    return request_json("POST", "/api/drive-folders/tree/refresh", token, query=query)


def drive_search(token: str, query_str: str = "", ref: str = "", drive_path: str = "", drive_folder_id: str = "", page_size: int = 20):
    query: Dict[str, Any] = {"pageSize": str(page_size)}
    if query_str:
        query["q"] = query_str
    add_drive_location_query(query, ref, drive_path, drive_folder_id)
    return request_json("GET", "/drive.googleapis.com/drive/v3/files", token, query=query)


def drive_get_file(token: str, file_id: str, fields: str = ""):
    query = {"fields": fields} if fields else None
    return request_json("GET", f"/drive.googleapis.com/drive/v3/files/{file_id}", token, query=query)


def drive_download(token: str, file_id: str, save_to: str):
    data = request_bytes("GET", f"/drive.googleapis.com/drive/v3/files/{file_id}", token, query={"alt": "media"})
    out = Path(save_to)
    out.parent.mkdir(parents=True, exist_ok=True)
    out.write_bytes(data)
    return {"fileId": file_id, "savedTo": str(out), "size": len(data)}


def drive_export(token: str, file_id: str, mime_type: str, save_to: str):
    data = request_bytes("GET", f"/drive.googleapis.com/drive/v3/files/{file_id}/export", token, query={"mimeType": mime_type})
    out = Path(save_to)
    out.parent.mkdir(parents=True, exist_ok=True)
    out.write_bytes(data)
    return {"fileId": file_id, "savedTo": str(out), "size": len(data), "mimeType": mime_type}


def drive_create_file(token: str, ref: str, name: str, mime_type: str = "", body_file: str = "", drive_path: str = "", drive_folder_id: str = ""):
    query = add_drive_location_query({}, ref, drive_path, drive_folder_id)
    body: Dict[str, Any] = {"name": name}
    if mime_type:
        body["mimeType"] = mime_type
    if body_file:
        extra = read_json_file(body_file)
        if isinstance(extra, dict):
            body.update(extra)
    return request_json("POST", "/drive.googleapis.com/drive/v3/files", token, body=body, query=query)


def drive_update_file(token: str, file_id: str, name: str = "", body_file: str = ""):
    body: Dict[str, Any] = {}
    if name:
        body["name"] = name
    if body_file:
        extra = read_json_file(body_file)
        if isinstance(extra, dict):
            body.update(extra)
    return request_json("PATCH", f"/drive.googleapis.com/drive/v3/files/{file_id}", token, body=body)


def drive_comment_body(content: str = "", body_file: str = "", action: str = ""):
    body: Dict[str, Any] = {}
    if body_file:
        extra = read_json_file(body_file)
        if not isinstance(extra, dict):
            die("body-file must contain a JSON object")
        body.update(extra)
    if content:
        body["content"] = content
    if action:
        body["action"] = action
    if not body:
        die("comment command requires --content or --body-file")
    return body


def drive_create_comment(token: str, file_id: str, content: str = "", body_file: str = "", fields: str = "id,content,htmlContent,author,createdTime,modifiedTime,resolved"):
    query = {"fields": fields}
    body = drive_comment_body(content, body_file)
    return request_json("POST", f"/drive.googleapis.com/drive/v3/files/{file_id}/comments", token, body=body, query=query)


def drive_reply_comment(token: str, file_id: str, comment_id: str, content: str = "", body_file: str = "", fields: str = "id,content,htmlContent,author,createdTime,modifiedTime"):
    query = {"fields": fields}
    body = drive_comment_body(content, body_file)
    return request_json("POST", f"/drive.googleapis.com/drive/v3/files/{file_id}/comments/{comment_id}/replies", token, body=body, query=query)


def drive_resolve_comment(token: str, file_id: str, comment_id: str, content: str = "", body_file: str = "", fields: str = "id,content,htmlContent,author,createdTime,modifiedTime,action"):
    query = {"fields": fields}
    body = drive_comment_body(content, body_file, action="resolve")
    return request_json("POST", f"/drive.googleapis.com/drive/v3/files/{file_id}/comments/{comment_id}/replies", token, body=body, query=query)


def drive_update_comment(token: str, file_id: str, comment_id: str, content: str = "", body_file: str = "", fields: str = "id,content,htmlContent,author,createdTime,modifiedTime,resolved"):
    query = {"fields": fields}
    body = drive_comment_body(content, body_file)
    return request_json("PATCH", f"/drive.googleapis.com/drive/v3/files/{file_id}/comments/{comment_id}", token, body=body, query=query)


def drive_update_reply(token: str, file_id: str, comment_id: str, reply_id: str, content: str = "", body_file: str = "", fields: str = "id,content,htmlContent,author,createdTime,modifiedTime,action"):
    query = {"fields": fields}
    body = drive_comment_body(content, body_file)
    return request_json("PATCH", f"/drive.googleapis.com/drive/v3/files/{file_id}/comments/{comment_id}/replies/{reply_id}", token, body=body, query=query)


def drive_delete_comment(token: str, file_id: str, comment_id: str):
    return request_json("DELETE", f"/drive.googleapis.com/drive/v3/files/{file_id}/comments/{comment_id}", token)


def drive_delete_reply(token: str, file_id: str, comment_id: str, reply_id: str):
    return request_json("DELETE", f"/drive.googleapis.com/drive/v3/files/{file_id}/comments/{comment_id}/replies/{reply_id}", token)


def docs_create(token: str, ref: str, title: str, drive_path: str = "", drive_folder_id: str = ""):
    query = add_drive_location_query({}, ref, drive_path, drive_folder_id)
    return request_json("POST", "/docs.googleapis.com/v1/documents", token, body={"title": title}, query=query)


def docs_get(token: str, document_id: str):
    return request_json("GET", f"/docs.googleapis.com/v1/documents/{document_id}", token)


def docs_update(token: str, document_id: str, requests_file: str):
    body = read_json_file(requests_file)
    return request_json("POST", f"/docs.googleapis.com/v1/documents/{document_id}:batchUpdate", token, body=body)


def sheets_create(token: str, ref: str, title: str, drive_path: str = "", drive_folder_id: str = ""):
    query = add_drive_location_query({}, ref, drive_path, drive_folder_id)
    return request_json("POST", "/sheets.googleapis.com/v4/spreadsheets", token, body={"properties": {"title": title}}, query=query)


def sheets_get(token: str, spreadsheet_id: str):
    return request_json("GET", f"/sheets.googleapis.com/v4/spreadsheets/{spreadsheet_id}", token)


def sheets_values_update(token: str, spreadsheet_id: str, range_name: str, values_file: str, value_input_option: str = "USER_ENTERED"):
    values_body = read_json_file(values_file)
    if not isinstance(values_body, dict) or "values" not in values_body:
        die("values-file must be a JSON object containing a top-level 'values' key")
    query = {"valueInputOption": value_input_option}
    return request_json("PUT", f"/sheets.googleapis.com/v4/spreadsheets/{spreadsheet_id}/values/{range_name}", token, body=values_body, query=query)


def sheets_batch_update(token: str, spreadsheet_id: str, requests_file: str):
    body = read_json_file(requests_file)
    return request_json("POST", f"/sheets.googleapis.com/v4/spreadsheets/{spreadsheet_id}:batchUpdate", token, body=body)


def slides_create(token: str, ref: str, title: str, drive_path: str = "", drive_folder_id: str = ""):
    query = add_drive_location_query({}, ref, drive_path, drive_folder_id)
    return request_json("POST", "/slides.googleapis.com/v1/presentations", token, body={"title": title}, query=query)


def slides_get(token: str, presentation_id: str):
    return request_json("GET", f"/slides.googleapis.com/v1/presentations/{presentation_id}", token)


def slides_update(token: str, presentation_id: str, requests_file: str):
    body = read_json_file(requests_file)
    return request_json("POST", f"/slides.googleapis.com/v1/presentations/{presentation_id}:batchUpdate", token, body=body)


def normalize_contact_email(value: str) -> str:
    return str(value or "").strip().lower()


def first_value(items: Any, key: str) -> str:
    if isinstance(items, list):
        for item in items:
            if isinstance(item, dict) and str(item.get(key) or "").strip():
                return str(item.get(key)).strip()
    return ""


def contact_emails(person: Dict[str, Any]) -> List[Dict[str, Any]]:
    out = []
    for email in person.get("emailAddresses") or []:
        if not isinstance(email, dict):
            continue
        address = normalize_contact_email(email.get("value", ""))
        if not address:
            continue
        metadata = email.get("metadata") if isinstance(email.get("metadata"), dict) else {}
        out.append({
            "address": address,
            "type": str(email.get("type") or "").strip(),
            "primary": bool(metadata.get("primary")),
        })
    return out


def contact_primary_email(emails: List[Dict[str, Any]]) -> str:
    for email in emails:
        if email.get("primary") and email.get("address"):
            return str(email["address"])
    if len(emails) == 1:
        return str(emails[0]["address"])
    return ""


def normalize_contact_candidate(person: Dict[str, Any], source: str, query: str) -> Optional[Dict[str, Any]]:
    emails = contact_emails(person)
    if not emails:
        return None
    names = person.get("names") if isinstance(person.get("names"), list) else []
    orgs = person.get("organizations") if isinstance(person.get("organizations"), list) else []
    display_name = first_value(names, "displayName")
    given_name = first_value(names, "givenName")
    family_name = first_value(names, "familyName")
    org_name = first_value(orgs, "name")
    title = first_value(orgs, "title")
    primary_email = contact_primary_email(emails)
    query_norm = query.strip().lower()
    haystacks = [display_name.lower(), given_name.lower(), family_name.lower(), primary_email.lower()]
    reasons = []
    confidence = {"contacts": 0.82, "directory": 0.78, "other_contacts": 0.68}.get(source, 0.6)
    if query_norm:
        if query_norm in [primary_email.lower(), display_name.lower()]:
            confidence += 0.16
            reasons.append("exact_match")
        elif any(value.startswith(query_norm) for value in haystacks if value):
            confidence += 0.1
            reasons.append("prefix_match")
        elif any(query_norm in value for value in haystacks if value):
            confidence += 0.05
            reasons.append("partial_match")
    return {
        "display_name": display_name or primary_email,
        "given_name": given_name,
        "family_name": family_name,
        "emails": emails,
        "primary_email": primary_email,
        "organization": org_name,
        "title": title,
        "sources": [source],
        "resource_names": [str(person.get("resourceName") or "").strip()] if person.get("resourceName") else [],
        "confidence": min(round(confidence, 2), 0.99),
        "match_reasons": reasons or ["source_match"],
    }


def merge_contact_candidates(candidates: List[Dict[str, Any]]) -> List[Dict[str, Any]]:
    merged: Dict[str, Dict[str, Any]] = {}
    for candidate in candidates:
        key = contact_primary_email(candidate.get("emails", []))
        if not key:
            continue
        existing = merged.get(key)
        if existing is None:
            merged[key] = candidate
            continue
        existing["sources"] = sorted(set(existing.get("sources", []) + candidate.get("sources", [])))
        existing["resource_names"] = sorted(set(existing.get("resource_names", []) + candidate.get("resource_names", [])))
        existing["match_reasons"] = sorted(set(existing.get("match_reasons", []) + candidate.get("match_reasons", [])))
        existing["confidence"] = max(existing.get("confidence", 0), candidate.get("confidence", 0))
        for field in ["display_name", "given_name", "family_name", "organization", "title", "primary_email"]:
            if not existing.get(field) and candidate.get(field):
                existing[field] = candidate[field]
    return sorted(merged.values(), key=lambda item: (-float(item.get("confidence", 0)), str(item.get("display_name") or ""), str(item.get("primary_email") or "")))


def contact_people_from_response(payload: Dict[str, Any], directory: bool = False) -> List[Dict[str, Any]]:
    if directory:
        people = payload.get("people")
        return people if isinstance(people, list) else []
    results = payload.get("results")
    if not isinstance(results, list):
        return []
    out = []
    for result in results:
        if isinstance(result, dict) and isinstance(result.get("person"), dict):
            out.append(result["person"])
    return out


def contacts_resolve(token: str, query: str, max_results: int = 10):
    query = query.strip()
    if not query:
        die("contacts resolve requires a non-empty --query")
    page_size = max(1, min(int(max_results or 10), 30))
    read_mask = "names,emailAddresses,organizations,metadata"
    sources = [
        ("directory", "/people.googleapis.com/v1/people:searchDirectoryPeople", {
            "query": query,
            "pageSize": page_size,
            "readMask": read_mask,
            "sources": ["DIRECTORY_SOURCE_TYPE_DOMAIN_PROFILE", "DIRECTORY_SOURCE_TYPE_DOMAIN_CONTACT"],
        }, True),
        ("contacts", "/people.googleapis.com/v1/people:searchContacts", {
            "query": query,
            "pageSize": page_size,
            "readMask": read_mask,
        }, False),
        ("other_contacts", "/people.googleapis.com/v1/otherContacts:search", {
            "query": query,
            "pageSize": page_size,
            "readMask": "names,emailAddresses,metadata",
        }, False),
    ]
    candidates = []
    errors = []
    for source, path, params, directory in sources:
        if source in {"contacts", "other_contacts"}:
            warmup_params = dict(params)
            warmup_params["query"] = ""
            request_json_optional("GET", path, token, query=warmup_params)
        payload, error = request_json_optional("GET", path, token, query=params)
        if error:
            errors.append({"source": source, "error": error})
            continue
        for person in contact_people_from_response(payload or {}, directory=directory):
            candidate = normalize_contact_candidate(person, source, query)
            if candidate is not None:
                candidates.append(candidate)

    merged = merge_contact_candidates(candidates)[:page_size]
    status = "not_found"
    if len(merged) == 1:
        status = "needs_confirmation"
        if len(merged[0].get("emails", [])) > 1 and not merged[0].get("primary_email"):
            status = "needs_email_choice"
    elif len(merged) > 1:
        status = "ambiguous"
    return {
        "status": status,
        "query": query,
        "candidates": merged,
        "errors": errors,
    }


def build_parser():
    parser = argparse.ArgumentParser(
        description="AI Workspace Proxy helper for AI agent skills",
        epilog=(
            "Use product commands such as gmail unread or drive search as helper mode for common workflows. "
            "Use proxy request as full passthrough mode for any Google API request covered by the applied proxy policy."
        ),
    )
    parser.add_argument("--workspace", default="", help="Workspace friendly name or email address")
    parser.add_argument("--agent-motive", default=os.environ.get("AIWP_AGENT_MOTIVE", ""), help="One or two sentences explaining why the request is being made")
    parser.add_argument("--human-approval", default=os.environ.get("AIWP_HUMAN_APPROVAL", ""), help="Short explanation of how the human operator reviewed the action, approved it explicitly, and how that approval was obtained, including the approval wording quoted exactly as written by the human")
    top = parser.add_subparsers(dest="product", required=True)

    # Gmail
    gmail = top.add_parser("gmail", help="Gmail operations")
    gsub = gmail.add_subparsers(dest="cmd", required=True)
    p = gsub.add_parser("unread")
    p.add_argument("--max-results", type=int, default=10)
    p = gsub.add_parser("search")
    p.add_argument("--query", default="")
    p.add_argument("--max-results", type=int, default=10)
    p = gsub.add_parser("get-message")
    p.add_argument("--id", required=True)
    p.add_argument("--format", default="metadata", choices=["minimal", "full", "raw", "metadata"])
    p = gsub.add_parser("get-thread")
    p.add_argument("--id", required=True)
    p.add_argument("--format", default="full", choices=["minimal", "full", "metadata"])
    gsub.add_parser("list-drafts")
    p = gsub.add_parser("get-draft")
    p.add_argument("--id", required=True)
    p = gsub.add_parser("create-draft")
    p.add_argument("--to", required=True)
    p.add_argument("--subject", required=True)
    p.add_argument("--body-file", required=True)
    p.add_argument("--cc", default="")
    p.add_argument("--bcc", default="")
    p.add_argument("--attach", action="append", default=[])
    p = gsub.add_parser("update-draft")
    p.add_argument("--id", required=True)
    p.add_argument("--to", required=True)
    p.add_argument("--subject", required=True)
    p.add_argument("--body-file", required=True)
    p.add_argument("--cc", default="")
    p.add_argument("--bcc", default="")
    p.add_argument("--attach", action="append", default=[])
    p = gsub.add_parser("delete-draft")
    p.add_argument("--id", required=True)
    p = gsub.add_parser("reply-draft")
    p.add_argument("--message-id", required=True)
    p.add_argument("--body-file", required=True)
    p.add_argument("--attach", action="append", default=[])
    gsub.add_parser("list-labels")
    p = gsub.add_parser("get-label")
    p.add_argument("--id", required=True)
    p = gsub.add_parser("create-label")
    p.add_argument("--body-file", required=True)
    p = gsub.add_parser("patch-label")
    p.add_argument("--id", required=True)
    p.add_argument("--body-file", required=True)
    p = gsub.add_parser("delete-label")
    p.add_argument("--id", required=True)
    p = gsub.add_parser("apply-label")
    p.add_argument("--label-id", required=True)
    p.add_argument("--message-id")
    p.add_argument("--thread-id")
    p = gsub.add_parser("remove-label")
    p.add_argument("--label-id", required=True)
    p.add_argument("--message-id")
    p.add_argument("--thread-id")
    p = gsub.add_parser("mark-read")
    p.add_argument("--message-id")
    p.add_argument("--thread-id")
    p = gsub.add_parser("mark-unread")
    p.add_argument("--message-id")
    p.add_argument("--thread-id")
    p = gsub.add_parser("star")
    p.add_argument("--message-id")
    p.add_argument("--thread-id")
    p = gsub.add_parser("unstar")
    p.add_argument("--message-id")
    p.add_argument("--thread-id")
    p = gsub.add_parser("mark-important")
    p.add_argument("--message-id")
    p.add_argument("--thread-id")
    p = gsub.add_parser("mark-not-important")
    p.add_argument("--message-id")
    p.add_argument("--thread-id")
    p = gsub.add_parser("archive")
    p.add_argument("--message-id")
    p.add_argument("--thread-id")
    p = gsub.add_parser("unarchive")
    p.add_argument("--message-id")
    p.add_argument("--thread-id")
    p = gsub.add_parser("mark-spam")
    p.add_argument("--message-id")
    p.add_argument("--thread-id")
    p = gsub.add_parser("unmark-spam")
    p.add_argument("--message-id")
    p.add_argument("--thread-id")
    p = gsub.add_parser("trash")
    p.add_argument("--message-id")
    p.add_argument("--thread-id")
    p = gsub.add_parser("untrash")
    p.add_argument("--message-id")
    p.add_argument("--thread-id")
    p = gsub.add_parser("delete-message")
    p.add_argument("--message-id", required=True)

    # Contacts
    contacts = top.add_parser("contacts", help="Contact and directory lookup")
    csub_contacts = contacts.add_subparsers(dest="cmd", required=True)
    p = csub_contacts.add_parser("resolve", help="Resolve a partial name or email hint to contact candidates")
    p.add_argument("--query", required=True)
    p.add_argument("--max-results", type=int, default=10)

    # Calendar
    cal = top.add_parser("calendar", help="Calendar operations")
    csub = cal.add_subparsers(dest="cmd", required=True)
    csub.add_parser("list-calendars")
    p = csub.add_parser("get-calendar")
    p.add_argument("--calendar-id", required=True)
    p = csub.add_parser("list-events")
    p.add_argument("--calendar-id", required=True)
    p.add_argument("--time-min", default="")
    p.add_argument("--time-max", default="")
    p.add_argument("--query", default="")
    p.add_argument("--max-results", type=int, default=20)
    p = csub.add_parser("get-event")
    p.add_argument("--calendar-id", required=True)
    p.add_argument("--event-id", required=True)

    # Drive
    drv = top.add_parser("drive", help="Drive operations")
    dsub = drv.add_subparsers(dest="cmd", required=True)
    p = dsub.add_parser("search")
    p.add_argument("--query", default="")
    p.add_argument("--ref", default="")
    p.add_argument("--drive-path", default="")
    p.add_argument("--drive-folder-id", default="")
    p.add_argument("--page-size", type=int, default=20)
    p = dsub.add_parser("list-folders")
    p.add_argument("--ref", default="")
    p = dsub.add_parser("refresh-folders")
    p.add_argument("--ref", default="")
    p = dsub.add_parser("get-file")
    p.add_argument("--file-id", required=True)
    p.add_argument("--fields", default="")
    p = dsub.add_parser("download")
    p.add_argument("--file-id", required=True)
    p.add_argument("--save-to", required=True)
    p = dsub.add_parser("export")
    p.add_argument("--file-id", required=True)
    p.add_argument("--mime-type", required=True)
    p.add_argument("--save-to", required=True)
    p = dsub.add_parser("create-file")
    p.add_argument("--ref", required=True)
    p.add_argument("--name", required=True)
    p.add_argument("--mime-type", default="")
    p.add_argument("--body-file", default="")
    p.add_argument("--drive-path", default="")
    p.add_argument("--drive-folder-id", default="")
    p = dsub.add_parser("update-file")
    p.add_argument("--file-id", required=True)
    p.add_argument("--name", default="")
    p.add_argument("--body-file", default="")
    p = dsub.add_parser("create-comment")
    p.add_argument("--file-id", required=True)
    p.add_argument("--content", default="")
    p.add_argument("--body-file", default="")
    p.add_argument("--fields", default="id,content,htmlContent,author,createdTime,modifiedTime,resolved")
    p = dsub.add_parser("reply-comment")
    p.add_argument("--file-id", required=True)
    p.add_argument("--comment-id", required=True)
    p.add_argument("--content", default="")
    p.add_argument("--body-file", default="")
    p.add_argument("--fields", default="id,content,htmlContent,author,createdTime,modifiedTime")
    p = dsub.add_parser("resolve-comment")
    p.add_argument("--file-id", required=True)
    p.add_argument("--comment-id", required=True)
    p.add_argument("--content", default="")
    p.add_argument("--body-file", default="")
    p.add_argument("--fields", default="id,content,htmlContent,author,createdTime,modifiedTime,action")
    p = dsub.add_parser("update-comment")
    p.add_argument("--file-id", required=True)
    p.add_argument("--comment-id", required=True)
    p.add_argument("--content", default="")
    p.add_argument("--body-file", default="")
    p.add_argument("--fields", default="id,content,htmlContent,author,createdTime,modifiedTime,resolved")
    p = dsub.add_parser("update-reply")
    p.add_argument("--file-id", required=True)
    p.add_argument("--comment-id", required=True)
    p.add_argument("--reply-id", required=True)
    p.add_argument("--content", default="")
    p.add_argument("--body-file", default="")
    p.add_argument("--fields", default="id,content,htmlContent,author,createdTime,modifiedTime,action")
    p = dsub.add_parser("delete-comment")
    p.add_argument("--file-id", required=True)
    p.add_argument("--comment-id", required=True)
    p = dsub.add_parser("delete-reply")
    p.add_argument("--file-id", required=True)
    p.add_argument("--comment-id", required=True)
    p.add_argument("--reply-id", required=True)

    # Docs
    docs = top.add_parser("docs", help="Docs operations")
    dsub2 = docs.add_subparsers(dest="cmd", required=True)
    p = dsub2.add_parser("create")
    p.add_argument("--ref", required=True)
    p.add_argument("--title", required=True)
    p.add_argument("--drive-path", default="")
    p.add_argument("--drive-folder-id", default="")
    p = dsub2.add_parser("get")
    p.add_argument("--document-id", required=True)
    p = dsub2.add_parser("update")
    p.add_argument("--document-id", required=True)
    p.add_argument("--requests-file", required=True)

    # Sheets
    sht = top.add_parser("sheets", help="Sheets operations")
    ssub = sht.add_subparsers(dest="cmd", required=True)
    p = ssub.add_parser("create")
    p.add_argument("--ref", required=True)
    p.add_argument("--title", required=True)
    p.add_argument("--drive-path", default="")
    p.add_argument("--drive-folder-id", default="")
    p = ssub.add_parser("get")
    p.add_argument("--spreadsheet-id", required=True)
    p = ssub.add_parser("values-update")
    p.add_argument("--spreadsheet-id", required=True)
    p.add_argument("--range", required=True)
    p.add_argument("--values-file", required=True)
    p.add_argument("--value-input-option", default="USER_ENTERED")
    p = ssub.add_parser("batch-update")
    p.add_argument("--spreadsheet-id", required=True)
    p.add_argument("--requests-file", required=True)

    # Slides
    sld = top.add_parser("slides", help="Slides operations")
    slsub = sld.add_subparsers(dest="cmd", required=True)
    p = slsub.add_parser("create")
    p.add_argument("--ref", required=True)
    p.add_argument("--title", required=True)
    p.add_argument("--drive-path", default="")
    p.add_argument("--drive-folder-id", default="")
    p = slsub.add_parser("get")
    p.add_argument("--presentation-id", required=True)
    p = slsub.add_parser("update")
    p.add_argument("--presentation-id", required=True)
    p.add_argument("--requests-file", required=True)

    # Proxy
    proxy = top.add_parser("proxy", help="Proxy utilities and full passthrough requests")
    psub = proxy.add_subparsers(dest="cmd", required=True)
    p = psub.add_parser("request", help="Full passthrough: call a Google API path through the proxy")
    p.add_argument("--method", required=True, help="HTTP method such as GET, POST, PATCH, PUT, DELETE")
    p.add_argument("--path", required=True, help="Proxy path such as /gmail.googleapis.com/gmail/v1/users/me/messages")
    p.add_argument("--query", action="append", default=[], help="Query parameter as KEY=VALUE; can be repeated")
    p.add_argument("--body-file", default="", help="JSON request body file")
    p.add_argument("--save-to", default="", help="Write raw response bytes to this file instead of printing JSON/text")
    p = psub.add_parser("download-skill", help="Download a fresh AI agent skill zip from the proxy; no --workspace value is needed")
    p.add_argument("--save-to", required=True, help="Local path where the downloaded zip file must be written")
    p.add_argument("--platform", choices=["generic", "openclaw"], default="generic", help="Skill platform to download")

    return parser


def main():
    global ACTIVE_AGENT_MOTIVE, ACTIVE_HUMAN_APPROVAL, ACTIVE_WORKSPACE
    parser = build_parser()
    args = parser.parse_args()
    _, token, default_workspace = load_config()
    ACTIVE_WORKSPACE = (args.workspace or default_workspace).strip()
    ACTIVE_AGENT_MOTIVE = str(args.agent_motive or "").strip()
    ACTIVE_HUMAN_APPROVAL = str(args.human_approval or "").strip()
    workspace_optional = args.product == "proxy" and args.cmd == "download-skill"
    if not ACTIVE_WORKSPACE and not workspace_optional:
        die("A workspace is required. Pass --workspace NAME_OR_EMAIL, or use a config with exactly one workspace.")

    if args.product == "gmail":
        if args.cmd == "unread":
            out = list_unread(token, args.max_results)
        elif args.cmd == "search":
            out = list_messages(token, args.query, args.max_results)
        elif args.cmd == "get-message":
            out = get_message(token, args.id, args.format)
        elif args.cmd == "get-thread":
            out = get_thread(token, args.id, args.format)
        elif args.cmd == "list-drafts":
            out = list_drafts(token)
        elif args.cmd == "get-draft":
            out = get_draft(token, args.id)
        elif args.cmd == "create-draft":
            out = create_draft(token, args.to, args.subject, read_text_file(args.body_file), cc=args.cc, bcc=args.bcc, attach_paths=args.attach)
        elif args.cmd == "update-draft":
            out = update_draft(token, args.id, args.to, args.subject, read_text_file(args.body_file), cc=args.cc, bcc=args.bcc, attach_paths=args.attach)
        elif args.cmd == "delete-draft":
            out = delete_draft(token, args.id)
        elif args.cmd == "reply-draft":
            out = create_reply_draft(token, args.message_id, read_text_file(args.body_file), attach_paths=args.attach)
        elif args.cmd == "list-labels":
            out = list_labels(token)
        elif args.cmd == "get-label":
            out = get_label(token, args.id)
        elif args.cmd == "create-label":
            out = create_label(token, read_json_file(args.body_file))
        elif args.cmd == "patch-label":
            out = patch_label(token, args.id, read_json_file(args.body_file))
        elif args.cmd == "delete-label":
            out = delete_label(token, args.id)
        elif args.cmd == "apply-label":
            if args.message_id:
                out = modify_message_labels(token, args.message_id, add_labels=[args.label_id])
            elif args.thread_id:
                out = modify_thread_labels(token, args.thread_id, add_labels=[args.label_id])
            else:
                die("apply-label requires --message-id or --thread-id")
        elif args.cmd == "remove-label":
            if args.message_id:
                out = modify_message_labels(token, args.message_id, remove_labels=[args.label_id])
            elif args.thread_id:
                out = modify_thread_labels(token, args.thread_id, remove_labels=[args.label_id])
            else:
                die("remove-label requires --message-id or --thread-id")
        elif args.cmd == "mark-read":
            if args.message_id:
                out = modify_message_labels(token, args.message_id, remove_labels=["UNREAD"])
            elif args.thread_id:
                out = modify_thread_labels(token, args.thread_id, remove_labels=["UNREAD"])
            else:
                die("mark-read requires --message-id or --thread-id")
        elif args.cmd == "mark-unread":
            if args.message_id:
                out = modify_message_labels(token, args.message_id, add_labels=["UNREAD"])
            elif args.thread_id:
                out = modify_thread_labels(token, args.thread_id, add_labels=["UNREAD"])
            else:
                die("mark-unread requires --message-id or --thread-id")
        elif args.cmd == "star":
            if args.message_id:
                out = modify_message_labels(token, args.message_id, add_labels=["STARRED"])
            elif args.thread_id:
                out = modify_thread_labels(token, args.thread_id, add_labels=["STARRED"])
            else:
                die("star requires --message-id or --thread-id")
        elif args.cmd == "unstar":
            if args.message_id:
                out = modify_message_labels(token, args.message_id, remove_labels=["STARRED"])
            elif args.thread_id:
                out = modify_thread_labels(token, args.thread_id, remove_labels=["STARRED"])
            else:
                die("unstar requires --message-id or --thread-id")
        elif args.cmd == "mark-important":
            if args.message_id:
                out = modify_message_labels(token, args.message_id, add_labels=["IMPORTANT"])
            elif args.thread_id:
                out = modify_thread_labels(token, args.thread_id, add_labels=["IMPORTANT"])
            else:
                die("mark-important requires --message-id or --thread-id")
        elif args.cmd == "mark-not-important":
            if args.message_id:
                out = modify_message_labels(token, args.message_id, remove_labels=["IMPORTANT"])
            elif args.thread_id:
                out = modify_thread_labels(token, args.thread_id, remove_labels=["IMPORTANT"])
            else:
                die("mark-not-important requires --message-id or --thread-id")
        elif args.cmd == "archive":
            if args.message_id:
                out = modify_message_labels(token, args.message_id, remove_labels=["INBOX"])
            elif args.thread_id:
                out = modify_thread_labels(token, args.thread_id, remove_labels=["INBOX"])
            else:
                die("archive requires --message-id or --thread-id")
        elif args.cmd == "unarchive":
            if args.message_id:
                out = modify_message_labels(token, args.message_id, add_labels=["INBOX"])
            elif args.thread_id:
                out = modify_thread_labels(token, args.thread_id, add_labels=["INBOX"])
            else:
                die("unarchive requires --message-id or --thread-id")
        elif args.cmd == "mark-spam":
            if args.message_id:
                out = modify_message_labels(token, args.message_id, add_labels=["SPAM"])
            elif args.thread_id:
                out = modify_thread_labels(token, args.thread_id, add_labels=["SPAM"])
            else:
                die("mark-spam requires --message-id or --thread-id")
        elif args.cmd == "unmark-spam":
            if args.message_id:
                out = modify_message_labels(token, args.message_id, remove_labels=["SPAM"])
            elif args.thread_id:
                out = modify_thread_labels(token, args.thread_id, remove_labels=["SPAM"])
            else:
                die("unmark-spam requires --message-id or --thread-id")
        elif args.cmd == "trash":
            if args.message_id:
                out = trash_message(token, args.message_id)
            elif args.thread_id:
                out = trash_thread(token, args.thread_id)
            else:
                die("trash requires --message-id or --thread-id")
        elif args.cmd == "untrash":
            if args.message_id:
                out = untrash_message(token, args.message_id)
            elif args.thread_id:
                out = untrash_thread(token, args.thread_id)
            else:
                die("untrash requires --message-id or --thread-id")
        elif args.cmd == "delete-message":
            out = delete_message(token, args.message_id)
        else:
            die("Unknown gmail command")

    elif args.product == "contacts":
        if args.cmd == "resolve":
            out = contacts_resolve(token, args.query, args.max_results)
        else:
            die("Unknown contacts command")

    elif args.product == "calendar":
        if args.cmd == "list-calendars":
            out = list_calendars(token)
        elif args.cmd == "get-calendar":
            out = get_calendar(token, args.calendar_id)
        elif args.cmd == "list-events":
            out = list_events(token, args.calendar_id, args.time_min, args.time_max, args.query, args.max_results)
        elif args.cmd == "get-event":
            out = get_event(token, args.calendar_id, args.event_id)
        else:
            die("Unknown calendar command")

    elif args.product == "drive":
        if args.cmd == "search":
            out = drive_search(token, args.query, args.ref, args.drive_path, args.drive_folder_id, args.page_size)
        elif args.cmd == "list-folders":
            out = drive_folder_tree(token, args.ref)
        elif args.cmd == "refresh-folders":
            out = drive_refresh_folders(token, args.ref)
        elif args.cmd == "get-file":
            out = drive_get_file(token, args.file_id, args.fields)
        elif args.cmd == "download":
            out = drive_download(token, args.file_id, args.save_to)
        elif args.cmd == "export":
            out = drive_export(token, args.file_id, args.mime_type, args.save_to)
        elif args.cmd == "create-file":
            out = drive_create_file(token, args.ref, args.name, args.mime_type, args.body_file, args.drive_path, args.drive_folder_id)
        elif args.cmd == "update-file":
            out = drive_update_file(token, args.file_id, args.name, args.body_file)
        elif args.cmd == "create-comment":
            out = drive_create_comment(token, args.file_id, args.content, args.body_file, args.fields)
        elif args.cmd == "reply-comment":
            out = drive_reply_comment(token, args.file_id, args.comment_id, args.content, args.body_file, args.fields)
        elif args.cmd == "resolve-comment":
            out = drive_resolve_comment(token, args.file_id, args.comment_id, args.content, args.body_file, args.fields)
        elif args.cmd == "update-comment":
            out = drive_update_comment(token, args.file_id, args.comment_id, args.content, args.body_file, args.fields)
        elif args.cmd == "update-reply":
            out = drive_update_reply(token, args.file_id, args.comment_id, args.reply_id, args.content, args.body_file, args.fields)
        elif args.cmd == "delete-comment":
            out = drive_delete_comment(token, args.file_id, args.comment_id)
        elif args.cmd == "delete-reply":
            out = drive_delete_reply(token, args.file_id, args.comment_id, args.reply_id)
        else:
            die("Unknown drive command")

    elif args.product == "docs":
        if args.cmd == "create":
            out = docs_create(token, args.ref, args.title, args.drive_path, args.drive_folder_id)
        elif args.cmd == "get":
            out = docs_get(token, args.document_id)
        elif args.cmd == "update":
            out = docs_update(token, args.document_id, args.requests_file)
        else:
            die("Unknown docs command")

    elif args.product == "sheets":
        if args.cmd == "create":
            out = sheets_create(token, args.ref, args.title, args.drive_path, args.drive_folder_id)
        elif args.cmd == "get":
            out = sheets_get(token, args.spreadsheet_id)
        elif args.cmd == "values-update":
            out = sheets_values_update(token, args.spreadsheet_id, args.range, args.values_file, args.value_input_option)
        elif args.cmd == "batch-update":
            out = sheets_batch_update(token, args.spreadsheet_id, args.requests_file)
        else:
            die("Unknown sheets command")

    elif args.product == "slides":
        if args.cmd == "create":
            out = slides_create(token, args.ref, args.title, args.drive_path, args.drive_folder_id)
        elif args.cmd == "get":
            out = slides_get(token, args.presentation_id)
        elif args.cmd == "update":
            out = slides_update(token, args.presentation_id, args.requests_file)
        else:
            die("Unknown slides command")

    elif args.product == "proxy":
        if args.cmd == "request":
            body = read_json_file(args.body_file) if args.body_file else None
            raw = request_raw(args.method, args.path, token, body=body, query=parse_query_pairs(args.query))
            if args.save_to:
                out = save_bytes(raw, args.save_to)
            else:
                text = raw.decode("utf-8", errors="replace")
                try:
                    out = json.loads(text) if text.strip() else {}
                except json.JSONDecodeError:
                    print(text)
                    return
        elif args.cmd == "download-skill":
            raw = request_raw("GET", "/api/agent-skill/download", token, query={"platform": args.platform})
            out = save_bytes(raw, args.save_to)
        else:
            die("Unknown proxy command")

    else:
        die("Unknown product")

    print(json.dumps(out, indent=2))


if __name__ == "__main__":
    main()
