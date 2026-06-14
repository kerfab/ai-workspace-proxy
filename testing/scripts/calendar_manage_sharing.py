#!/usr/bin/env python3
from calendar_live_helpers import run_live_calendar_scenario


def acl_body(role, acl_email, rule_id=""):
    scope_type = "user"
    if ":" in rule_id:
        candidate = rule_id.split(":", 1)[0].strip()
        if candidate in {"user", "group", "domain", "default"}:
            scope_type = candidate

    scope = {"type": scope_type}
    if scope_type != "default":
        scope["value"] = acl_email
    return {"role": role, "scope": scope}


def scenario(test):
    acl_email = test.require_acl_email()
    setup_policy = test.create_policy("setup", [
        "calendar_calendars_manage",
    ])
    manage_policy = test.create_policy("manage-sharing", [
        "calendar_access_manage",
    ])

    test.apply_policy(setup_policy)
    calendar_id = test.create_calendar(test.summary("sharing manage target"))

    test.apply_policy(manage_policy)
    rule_id = test.create_acl_rule(calendar_id, acl_body("reader", acl_email), {"sendNotifications": "false"})
    test.patch_acl_rule(calendar_id, rule_id, {"role": "freeBusyReader"})
    test.put_acl_rule(calendar_id, rule_id, acl_body("reader", acl_email, rule_id))
    test.expect_forwarded(
        "POST",
        test.acl_path(calendar_id, suffix="watch"),
        {
            "id": f"aiwp-test-{test.run_id}",
            "type": "web_hook",
            "address": "https://example.invalid/ai-workspace-proxy-test",
        },
    )
    test.delete_acl_rule(calendar_id, rule_id)

    test.expect_denied("GET", test.acl_path(calendar_id))


if __name__ == "__main__":
    raise SystemExit(run_live_calendar_scenario("calendar_access_manage", scenario))
