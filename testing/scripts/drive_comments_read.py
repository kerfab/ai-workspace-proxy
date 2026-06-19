#!/usr/bin/env python3
# Copyright (c) 2026 Opensense Ltd. (Hong Kong). All rights reserved.
# Proprietary software. No use, copy, modification, distribution, disclosure,
# or reverse engineering is permitted without prior written authorization
# from Opensense Ltd.

from urllib.parse import quote

from drive_live_helpers import run_live_drive_scenario


def scenario(test):
    setup_policy = test.create_policy("setup", [
        "docs_create",
        "drive_comments_create",
        "drive_comments_reply",
        "drive_comments_read",
    ])
    test.apply_policy(setup_policy)
    doc_id = test.create_doc_via_drive("drive comments fixture")
    comment_id = test.create_comment(doc_id, "comment fixture")
    reply_id = test.create_reply(doc_id, comment_id, "reply fixture")

    read_policy = test.create_policy("drive-comments-read", ["drive_comments_read"])
    test.apply_policy(read_policy)

    base = f"files/{quote(doc_id, safe='')}/comments"
    comment_path = f"{base}/{quote(comment_id, safe='')}"
    replies_path = f"{comment_path}/replies"
    reply_path = f"{replies_path}/{quote(reply_id, safe='')}"

    test.expect_success("GET", test.drive_path(base, extra_query={"fields": "comments(id,content)"}))
    test.expect_success("GET", test.drive_path(comment_path, extra_query={"fields": "id,content"}))
    test.expect_success("GET", test.drive_path(replies_path, extra_query={"fields": "replies(id,content)"}))
    test.expect_success("GET", test.drive_path(reply_path, extra_query={"fields": "id,content"}))

    test.expect_denied("POST", test.drive_path(base), {"content": test.name("deny comment create")})
    test.expect_denied("PATCH", test.drive_path(comment_path), {"content": test.name("deny comment patch")})
    test.expect_denied("PUT", test.drive_path(comment_path), {"content": test.name("deny comment update")})
    test.expect_denied("DELETE", test.drive_path(comment_path))
    test.expect_denied("POST", test.drive_path(replies_path), {"content": test.name("deny reply create")})
    test.expect_denied("PATCH", test.drive_path(reply_path), {"content": test.name("deny reply patch")})
    test.expect_denied("PUT", test.drive_path(reply_path), {"content": test.name("deny reply update")})
    test.expect_denied("DELETE", test.drive_path(reply_path))


if __name__ == "__main__":
    raise SystemExit(run_live_drive_scenario("drive_comments_read", scenario))
