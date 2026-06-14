#!/usr/bin/env python3
from urllib.parse import quote

from calendar_live_helpers import event_body, run_live_calendar_scenario


def scenario(test):
    setup_policy = test.create_policy("setup", [
        "calendar_events_create_self",
        "calendar_events_create_guests",
        "calendar_read",
    ])
    update_policy = test.create_policy("update-self", [
        "calendar_events_update_self",
        "calendar_read",
    ])

    test.apply_policy(setup_policy)
    self_event_id = test.create_event(event_body(test.summary("self update target"), test.start_time()))
    guest_event_id = test.create_event(event_body(
        test.summary("guest update denied target"),
        test.start_time(1),
        attendees=[test.cfg["third_party_email"]],
    ))

    test.apply_policy(update_policy)
    test.patch_event(self_event_id, {"summary": test.summary("patched self")})
    test.put_event(self_event_id, event_body(test.summary("put self"), test.start_time(2)))

    test.expect_denied(
        "PATCH",
        test.calendar_path(f"events/{quote(self_event_id, safe='')}", {"sendUpdates": "none"}),
        {"attendees": [{"email": test.cfg["third_party_email"]}]},
        "third",
    )
    test.expect_denied(
        "PATCH",
        test.calendar_path(f"events/{quote(guest_event_id, safe='')}", {"sendUpdates": "none"}),
        {"summary": test.summary("deny patch existing guest")},
        "third",
    )


if __name__ == "__main__":
    raise SystemExit(run_live_calendar_scenario("calendar_events_update_self", scenario))
