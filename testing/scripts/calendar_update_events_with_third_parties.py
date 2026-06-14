#!/usr/bin/env python3
from urllib.parse import quote

from calendar_live_helpers import event_body, run_live_calendar_scenario


def scenario(test):
    setup_policy = test.create_policy("setup", [
        "calendar_events_create_self",
        "calendar_events_create_guests",
        "calendar_read",
    ])
    update_policy = test.create_policy("update-guests", [
        "calendar_events_update_guests",
        "calendar_read",
    ])

    test.apply_policy(setup_policy)
    guest_event_id = test.create_event(event_body(
        test.summary("guest update target"),
        test.start_time(),
        attendees=[test.cfg["third_party_email"]],
    ))
    self_event_to_convert_id = test.create_event(event_body(test.summary("self add guest target"), test.start_time(1)))
    self_event_denied_id = test.create_event(event_body(test.summary("self denied target"), test.start_time(2)))

    test.apply_policy(update_policy)
    test.patch_event(guest_event_id, {"summary": test.summary("patched guest")})
    test.put_event(guest_event_id, event_body(
        test.summary("put guest"),
        test.start_time(3),
        attendees=[test.cfg["third_party_email"]],
    ))
    test.patch_event(self_event_to_convert_id, {
        "attendees": [{"email": test.cfg["third_party_email"]}],
    })

    test.expect_forwarded(
        "POST",
        test.calendar_path(
            f"events/{quote(guest_event_id, safe='')}/move",
            {"destination": test.cfg["calendar_id"], "sendUpdates": "none"},
        ),
    )

    test.expect_denied(
        "PATCH",
        test.calendar_path(f"events/{quote(self_event_denied_id, safe='')}", {"sendUpdates": "none"}),
        {"summary": test.summary("deny self-only patch")},
        "myself-only",
    )


if __name__ == "__main__":
    raise SystemExit(run_live_calendar_scenario("calendar_events_update_guests", scenario))
