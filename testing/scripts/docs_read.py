#!/usr/bin/env python3
from urllib.parse import quote

from drive_live_helpers import run_live_drive_scenario


def scenario(test):
    setup_policy = test.create_policy("setup", ["docs_create", "docs_read"])
    test.apply_policy(setup_policy)
    doc_id = test.create_doc("docs read fixture")

    read_policy = test.create_policy("docs-read", ["docs_read"])
    test.apply_policy(read_policy)

    test.expect_success("GET", test.docs_path(f"documents/{quote(doc_id, safe='')}"))
    test.expect_denied(
        "POST",
        test.docs_path("documents", include_ref=True),
        {"title": test.name("deny docs create")},
    )
    test.expect_denied(
        "POST",
        test.docs_path(f"documents/{quote(doc_id, safe='')}:batchUpdate"),
        {"requests": [{"insertText": {"location": {"index": 1}, "text": "denied"}}]},
    )


if __name__ == "__main__":
    raise SystemExit(run_live_drive_scenario("docs_read", scenario))
