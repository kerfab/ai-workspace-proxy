#!/usr/bin/env python3
# Copyright (c) 2026 Opensense Ltd. (Hong Kong). All rights reserved.
# Proprietary software. No use, copy, modification, distribution, disclosure,
# or reverse engineering is permitted without prior written authorization
# from Opensense Ltd.

from urllib.parse import quote

from calendar_live_helpers import agent_json, event_body, run_live_calendar_scenario


def scenario(test):
    setup_policy = test.create_policy("setup", [
        "calendar_events_create_self",
        "calendar_read",
    ])
    read_policy = test.create_policy("read", [
        "calendar_read",
    ])

    test.apply_policy(setup_policy)
    event_id = test.create_event(event_body(test.summary("read target"), test.start_time()))

    test.apply_policy(read_policy)
    calendar_id = quote(test.cfg["calendar_id"], safe="")
    agent_json(test.cfg, "GET", test.calendar_api_path("users/me/calendarList", {"maxResults": "10"}))
    agent_json(test.cfg, "GET", test.calendar_api_path(f"users/me/calendarList/{calendar_id}"))
    agent_json(test.cfg, "GET", test.calendar_api_path(f"calendars/{calendar_id}"))
    agent_json(test.cfg, "GET", test.calendar_path("events", {
        "singleEvents": "true",
        "maxResults": "10",
        "q": test.summary("read target"),
    }))
    agent_json(test.cfg, "GET", test.calendar_path(f"events/{event_id}"))
    agent_json(test.cfg, "POST", test.calendar_api_path("freeBusy"), {
        "timeMin": test.start_time(-1).isoformat().replace("+00:00", "Z"),
        "timeMax": test.start_time(4).isoformat().replace("+00:00", "Z"),
        "items": [{"id": test.cfg["calendar_id"]}],
    })
    print("Read calendar list, calendar metadata, events, event details, and free/busy data")

    test.expect_denied(
        "POST",
        test.calendar_path("events", {"sendUpdates": "none"}),
        event_body(test.summary("deny create under read policy"), test.start_time(5)),
    )


if __name__ == "__main__":
    raise SystemExit(run_live_calendar_scenario("calendar_read", scenario))
