#!/usr/bin/env python3
# Copyright (c) 2026 Opensense Ltd. (Hong Kong). All rights reserved.
# Proprietary software. No use, copy, modification, distribution, disclosure,
# or reverse engineering is permitted without prior written authorization
# from Opensense Ltd.

from urllib.parse import quote

from drive_live_helpers import run_live_drive_scenario


def scenario(test):
    create_policy = test.create_policy("docs-create", ["docs_create"])
    test.apply_policy(create_policy)

    doc_id = test.create_doc("docs create allowed")
    test.expect_denied("GET", test.docs_path(f"documents/{quote(doc_id, safe='')}"))
    test.expect_denied(
        "POST",
        test.docs_path(f"documents/{quote(doc_id, safe='')}:batchUpdate"),
        {"requests": [{"insertText": {"location": {"index": 1}, "text": "denied"}}]},
    )


if __name__ == "__main__":
    raise SystemExit(run_live_drive_scenario("docs_create", scenario))
