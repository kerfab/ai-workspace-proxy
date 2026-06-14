#!/usr/bin/env python3
from calendar_live_helpers import event_body, run_live_calendar_scenario


def scenario(test):
    create_policy = test.create_policy("create-self", [
        "calendar_events_create_self",
        "calendar_read",
    ])
    test.apply_policy(create_policy)

    start = test.start_time()
    test.create_event(event_body(test.summary("create self"), start))
    test.import_event(event_body(
        test.summary("import self"),
        test.start_time(1),
        i_cal_uid=f"aiwp-{test.run_id}-create-self-import@ai-workspace-proxy.test",
    ))

    test.expect_denied(
        "POST",
        test.calendar_path("events", {"sendUpdates": "none"}),
        event_body(test.summary("deny create guest"), test.start_time(2), attendees=[test.cfg["third_party_email"]]),
        "third",
    )
    test.expect_denied(
        "POST",
        test.calendar_path("events/import"),
        event_body(
            test.summary("deny import guest"),
            test.start_time(3),
            attendees=[test.cfg["third_party_email"]],
            i_cal_uid=f"aiwp-{test.run_id}-create-self-guest-import@ai-workspace-proxy.test",
        ),
        "third",
    )


if __name__ == "__main__":
    raise SystemExit(run_live_calendar_scenario("calendar_events_create_self", scenario))
