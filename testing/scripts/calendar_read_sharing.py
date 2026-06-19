#!/usr/bin/env python3
# Copyright (c) 2026 Opensense Ltd. (Hong Kong). All rights reserved.
# Proprietary software. No use, copy, modification, distribution, disclosure,
# or reverse engineering is permitted without prior written authorization
# from Opensense Ltd.

from calendar_live_helpers import TestFailure, agent_json, run_live_calendar_scenario


def scenario(test):
    setup_policy = test.create_policy("setup", [
        "calendar_calendars_manage",
        "calendar_access_read",
    ])
    read_policy = test.create_policy("read-sharing", [
        "calendar_access_read",
    ])

    test.apply_policy(setup_policy)
    calendar_id = test.create_calendar(test.summary("sharing read target"))

    test.apply_policy(read_policy)
    _, acl_list = agent_json(test.cfg, "GET", test.acl_path(calendar_id))
    items = acl_list.get("items") if isinstance(acl_list, dict) else None
    if not items:
        raise TestFailure(f"ACL list did not contain any sharing rules: {acl_list}")
    rule_id = str(items[0].get("id") or "").strip()
    if not rule_id:
        raise TestFailure(f"ACL list first item did not contain an id: {items[0]}")
    agent_json(test.cfg, "GET", test.acl_path(calendar_id, rule_id))
    print(f"Read ACL list and ACL rule: {rule_id}")

    test.expect_denied(
        "POST",
        test.acl_path(calendar_id, extra_query={"sendNotifications": "false"}),
        {"role": "reader", "scope": {"type": "user", "value": "nobody@example.com"}},
    )


if __name__ == "__main__":
    raise SystemExit(run_live_calendar_scenario("calendar_access_read", scenario))
