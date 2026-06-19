#!/usr/bin/env python3
# Copyright (c) 2026 Opensense Ltd. (Hong Kong). All rights reserved.
# Proprietary software. No use, copy, modification, distribution, disclosure,
# or reverse engineering is permitted without prior written authorization
# from Opensense Ltd.

from calendar_live_helpers import event_body, run_live_calendar_scenario


def scenario(test):
    create_policy = test.create_policy("create-guests", [
        "calendar_events_create_guests",
        "calendar_read",
    ])
    test.apply_policy(create_policy)

    start = test.start_time()
    test.create_event(event_body(
        test.summary("create guest"),
        start,
        attendees=[test.cfg["third_party_email"]],
    ))
    test.import_event(event_body(
        test.summary("import guest"),
        test.start_time(1),
        attendees=[test.cfg["third_party_email"]],
        i_cal_uid=f"aiwp-{test.run_id}-create-guest-import@ai-workspace-proxy.test",
    ))

    test.expect_denied(
        "POST",
        test.calendar_path("events", {"sendUpdates": "none"}),
        event_body(test.summary("deny create self"), test.start_time(2)),
        "myself-only",
    )
    test.expect_denied(
        "POST",
        test.calendar_path("events/import"),
        event_body(
            test.summary("deny import self"),
            test.start_time(3),
            i_cal_uid=f"aiwp-{test.run_id}-create-guest-self-import@ai-workspace-proxy.test",
        ),
        "myself-only",
    )


if __name__ == "__main__":
    raise SystemExit(run_live_calendar_scenario("calendar_events_create_guests", scenario))
