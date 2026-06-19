#!/usr/bin/env python3
# Copyright (c) 2026 Opensense Ltd. (Hong Kong). All rights reserved.
# Proprietary software. No use, copy, modification, distribution, disclosure,
# or reverse engineering is permitted without prior written authorization
# from Opensense Ltd.

from urllib.parse import quote

from calendar_live_helpers import event_body, run_live_calendar_scenario


def scenario(test):
    setup_policy = test.create_policy("setup", [
        "calendar_events_create_self",
        "calendar_events_create_guests",
        "calendar_read",
    ])
    delete_policy = test.create_policy("delete-guests", [
        "calendar_events_delete_guests",
        "calendar_read",
    ])

    test.apply_policy(setup_policy)
    self_event_id = test.create_event(event_body(test.summary("self delete denied target"), test.start_time()))
    guest_event_id = test.create_event(event_body(
        test.summary("guest delete target"),
        test.start_time(1),
        attendees=[test.cfg["third_party_email"]],
    ))

    test.apply_policy(delete_policy)
    test.delete_event(guest_event_id)
    test.expect_denied(
        "DELETE",
        test.calendar_path(f"events/{quote(self_event_id, safe='')}", {"sendUpdates": "none"}),
        want_text="myself-only",
    )


if __name__ == "__main__":
    raise SystemExit(run_live_calendar_scenario("calendar_events_delete_guests", scenario))
