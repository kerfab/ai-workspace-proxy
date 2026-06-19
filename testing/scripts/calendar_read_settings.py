#!/usr/bin/env python3
# Copyright (c) 2026 Opensense Ltd. (Hong Kong). All rights reserved.
# Proprietary software. No use, copy, modification, distribution, disclosure,
# or reverse engineering is permitted without prior written authorization
# from Opensense Ltd.

from calendar_live_helpers import agent_json, event_body, run_live_calendar_scenario


def scenario(test):
    settings_policy = test.create_policy("read-settings", [
        "calendar_settings_read",
    ])
    test.apply_policy(settings_policy)

    agent_json(test.cfg, "GET", test.calendar_api_path("users/me/settings"))
    agent_json(test.cfg, "GET", test.calendar_api_path("users/me/settings/timezone"))
    print("Read Calendar settings list and timezone setting")

    test.expect_denied(
        "POST",
        test.calendar_path("events", {"sendUpdates": "none"}),
        event_body(test.summary("deny create under settings policy"), test.start_time()),
    )


if __name__ == "__main__":
    raise SystemExit(run_live_calendar_scenario("calendar_settings_read", scenario))
