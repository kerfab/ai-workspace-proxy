#!/usr/bin/env python3
from urllib.parse import quote

from drive_live_helpers import TestFailure, run_live_drive_scenario


def scenario(test):
    setup_policy = test.create_policy("setup", [
        "docs_create",
        "drive_metadata_read",
    ])
    test.apply_policy(setup_policy)
    doc_id = test.create_doc_via_drive("drive metadata fixture")

    read_policy = test.create_policy("drive-metadata-read", ["drive_metadata_read"])
    test.apply_policy(read_policy)

    test.expect_success("GET", test.drive_path("files", include_ref=True, extra_query={
        "pageSize": "10",
        "fields": "files(id,name,mimeType,parents)",
        "q": f"name contains '{test.run_id}'",
    }))
    test.expect_success("GET", test.drive_path("about", extra_query={
        "fields": "user,storageQuota",
    }))

    _, token_payload = test.expect_success("GET", test.drive_path("changes/startPageToken", extra_query={
        "fields": "startPageToken",
    }))
    page_token = str(token_payload.get("startPageToken") or "").strip()
    if not page_token:
        raise TestFailure(f"startPageToken response did not contain startPageToken: {token_payload}")
    test.expect_success("GET", test.drive_path("changes", extra_query={
        "pageToken": page_token,
        "fields": "changes(fileId,time),newStartPageToken,nextPageToken",
    }))

    _, drives_payload = test.expect_success("GET", test.drive_path("drives", extra_query={
        "pageSize": "10",
        "fields": "drives(id,name)",
    }))
    drive_id = first_id(drives_payload, "drives") or "aiwp-test-missing-drive"
    test.expect_forwarded("GET", test.drive_path(f"drives/{quote(drive_id, safe='')}", extra_query={
        "fields": "id,name",
    }))

    _, apps_payload = test.expect_forwarded("GET", test.drive_path("apps", extra_query={
        "fields": "*",
    }))
    app_id = first_id(apps_payload, "apps") or first_id(apps_payload, "items") or "aiwp-test-missing-app"
    test.expect_forwarded("GET", test.drive_path(f"apps/{quote(app_id, safe='')}", extra_query={
        "fields": "*",
    }))

    test.expect_success("GET", test.drive_path(f"files/{quote(doc_id, safe='')}/listLabels", extra_query={
        "fields": "labels(id,revisionId,fields)",
    }))

    revisions = test.list_revisions(doc_id)
    revision_id = first_id({"revisions": revisions}, "revisions") or "1"
    test.expect_forwarded("GET", test.drive_path(
        f"files/{quote(doc_id, safe='')}/revisions/{quote(revision_id, safe='')}",
        extra_query={"fields": "id,modifiedTime,keepForever"},
    ))

    test.expect_denied("POST", test.drive_path("drives/aiwp-test-missing-drive/hide"))
    test.expect_denied("POST", test.drive_path("drives/aiwp-test-missing-drive/unhide"))
    test.expect_denied(
        "POST",
        test.drive_path(f"files/{quote(doc_id, safe='')}/modifyLabels"),
        {"labelModifications": []},
    )
    test.expect_denied(
        "PATCH",
        test.drive_path(f"files/{quote(doc_id, safe='')}/revisions/{quote(revision_id, safe='')}"),
        {"keepForever": True},
    )
    test.expect_denied(
        "DELETE",
        test.drive_path(f"files/{quote(doc_id, safe='')}/revisions/{quote(revision_id, safe='')}"),
    )


def first_id(payload, key):
    values = payload.get(key) if isinstance(payload, dict) else None
    if not isinstance(values, list):
        return ""
    for value in values:
        if isinstance(value, dict) and str(value.get("id") or "").strip():
            return str(value["id"]).strip()
    return ""


if __name__ == "__main__":
    raise SystemExit(run_live_drive_scenario("drive_metadata_read", scenario))
