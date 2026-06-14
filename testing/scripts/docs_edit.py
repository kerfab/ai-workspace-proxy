#!/usr/bin/env python3
from urllib.parse import quote

from drive_live_helpers import run_live_drive_scenario


def scenario(test):
    setup_policy = test.create_policy("setup", ["docs_create", "docs_read", "docs_edit"])
    test.apply_policy(setup_policy)
    doc_id = test.create_doc("docs edit fixture")

    edit_policy = test.create_policy("docs-edit", ["docs_edit"])
    test.apply_policy(edit_policy)

    test.batch_update_doc(doc_id, test.name("docs edit allowed"))
    test.expect_denied("GET", test.docs_path(f"documents/{quote(doc_id, safe='')}"))
    test.expect_denied(
        "POST",
        test.docs_path("documents", include_ref=True),
        {"title": test.name("deny docs create")},
    )


if __name__ == "__main__":
    raise SystemExit(run_live_drive_scenario("docs_edit", scenario))
