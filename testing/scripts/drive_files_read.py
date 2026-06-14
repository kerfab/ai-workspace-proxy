#!/usr/bin/env python3
from urllib.parse import quote

from drive_live_helpers import run_live_drive_scenario


def scenario(test):
    setup_policy = test.create_policy("setup", [
        "drive_files_create",
        "drive_files_read",
    ])
    test.apply_policy(setup_policy)
    text_file_id = test.create_drive_text_file("drive download fixture")

    read_policy = test.create_policy("drive-files-read", ["drive_files_read"])
    test.apply_policy(read_policy)

    test.expect_success("GET", test.drive_path(f"files/{quote(text_file_id, safe='')}", extra_query={
        "fields": "id,name,mimeType,parents",
    }))
    test.expect_forwarded("GET", test.drive_path(f"files/{quote(text_file_id, safe='')}", extra_query={
        "alt": "media",
    }))

    test.expect_denied(
        "POST",
        test.drive_path("files", include_ref=True),
        {"name": test.name("deny drive create"), "mimeType": "text/plain"},
    )
    test.expect_denied(
        "PATCH",
        test.drive_path(f"files/{quote(text_file_id, safe='')}"),
        {"name": test.name("deny drive patch")},
    )
    test.expect_denied("DELETE", test.drive_path(f"files/{quote(text_file_id, safe='')}"))
    test.expect_denied("GET", test.drive_path("files", include_ref=True, extra_query={
        "pageSize": "10",
        "fields": "files(id,name,mimeType,parents)",
        "q": f"name contains '{test.run_id}'",
    }))
    test.expect_denied("GET", test.drive_path("about", extra_query={
        "fields": "user,storageQuota",
    }))


if __name__ == "__main__":
    raise SystemExit(run_live_drive_scenario("drive_files_read", scenario))
