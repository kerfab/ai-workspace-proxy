#!/usr/bin/env python3
# Copyright (c) 2026 Opensense Ltd. (Hong Kong). All rights reserved.
# Proprietary software. No use, copy, modification, distribution, disclosure,
# or reverse engineering is permitted without prior written authorization
# from Opensense Ltd.

from calendar_live_helpers import agent_json, run_live_calendar_scenario


def scenario(test):
    manage_policy = test.create_policy("manage-calendars", [
        "calendar_calendars_manage",
    ])
    test.apply_policy(manage_policy)

    calendar_id = test.create_calendar(test.summary("manage target"))
    test.patch_calendar(calendar_id, {"summary": test.summary("patched calendar")})
    test.put_calendar(calendar_id, {
        "summary": test.summary("updated calendar"),
        "timeZone": "UTC",
    })
    test.expect_forwarded("POST", test.calendar_resource_path(calendar_id, "clear"))

    agent_json(test.cfg, "PATCH", test.calendar_list_path(calendar_id), {
        "summaryOverride": test.summary("patched list entry"),
        "hidden": False,
        "selected": True,
    })
    print(f"Patched calendar list entry: {calendar_id}")
    agent_json(test.cfg, "PUT", test.calendar_list_path(calendar_id), {
        "id": calendar_id,
        "summaryOverride": test.summary("updated list entry"),
        "hidden": False,
        "selected": True,
    })
    print(f"Updated calendar list entry: {calendar_id}")
    test.expect_forwarded(
        "DELETE",
        test.calendar_list_path(f"aiwp-nonexistent-{test.run_id}"),
    )
    test.expect_forwarded(
        "POST",
        test.calendar_list_path(),
        {"id": f"aiwp-nonexistent-{test.run_id}@group.calendar.google.com"},
    )

    test.delete_calendar(calendar_id)
    test.expect_denied("GET", test.calendar_list_path())


if __name__ == "__main__":
    raise SystemExit(run_live_calendar_scenario("calendar_calendars_manage", scenario))
