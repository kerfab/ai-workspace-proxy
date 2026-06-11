<!-- capability: calendar_read -->
### Read Calendars And Events

Allowed: list calendars, read calendar metadata, read events, list event instances, and query free/busy.

Example commands:

Use this first to find calendar IDs.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" calendar list-calendars`

Use this to inspect one calendar. Replace `primary` with a calendar ID when needed.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" calendar get-calendar --calendar-id primary`

Use this to list events in a time window. Replace dates with ISO timestamps from the user request.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" calendar list-events --calendar-id primary --time-min '2026-06-11T00:00:00Z' --time-max '2026-06-12T00:00:00Z'`

Use this after `list-events` returns an event `id`.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" calendar get-event --calendar-id primary --event-id EVENT_ID`

Use `proxy request` for event instances and free/busy. Calendar IDs come from `list-calendars`.
<!-- /capability -->

<!-- capability: calendar_events_write -->
### Create And Edit Calendar Events

Allowed: create, update, move, import, and delete calendar events.

Example commands:

Use this to create an event. Create `/tmp/event.json` with the Calendar event body.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" proxy request --method POST --path /calendar.googleapis.com/calendar/v3/calendars/primary/events --body-file /tmp/event.json`

Use this to edit an event. `EVENT_ID` comes from `list-events`; create `/tmp/event_patch.json`.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" proxy request --method PATCH --path /calendar.googleapis.com/calendar/v3/calendars/primary/events/EVENT_ID --body-file /tmp/event_patch.json`

Use this to delete an event only after user confirmation.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" proxy request --method DELETE --path /calendar.googleapis.com/calendar/v3/calendars/primary/events/EVENT_ID`
<!-- /capability -->

<!-- capability: calendar_calendars_manage -->
### Manage Calendars And Calendar Lists

Allowed: create, update, clear, or delete calendars and calendar-list entries.

Example commands:

Use this to create a calendar. Create `/tmp/calendar.json`.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" proxy request --method POST --path /calendar.googleapis.com/calendar/v3/calendars --body-file /tmp/calendar.json`

Use this to edit a calendar. `CALENDAR_ID` comes from `list-calendars`.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" proxy request --method PATCH --path /calendar.googleapis.com/calendar/v3/calendars/CALENDAR_ID --body-file /tmp/calendar_patch.json`

Use this to add a calendar to the user's calendar list. Create `/tmp/calendar_list_entry.json`.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" proxy request --method POST --path /calendar.googleapis.com/calendar/v3/users/me/calendarList --body-file /tmp/calendar_list_entry.json`
<!-- /capability -->

<!-- capability: calendar_access_read -->
### Read Calendar Sharing

Allowed: read calendar access-control rules.

Example commands:

Use this to list sharing rules for a calendar.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" proxy request --method GET --path /calendar.googleapis.com/calendar/v3/calendars/CALENDAR_ID/acl`

Use this to inspect one sharing rule. `RULE_ID` comes from the ACL list.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" proxy request --method GET --path /calendar.googleapis.com/calendar/v3/calendars/CALENDAR_ID/acl/RULE_ID`
<!-- /capability -->

<!-- capability: calendar_access_manage -->
### Manage Calendar Sharing

Allowed: create, update, delete, or watch calendar sharing rules.

Example commands:

Use this to add a sharing rule. Create `/tmp/acl_rule.json`.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" proxy request --method POST --path /calendar.googleapis.com/calendar/v3/calendars/CALENDAR_ID/acl --body-file /tmp/acl_rule.json`

Use this to edit a sharing rule. `RULE_ID` comes from the ACL list.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" proxy request --method PATCH --path /calendar.googleapis.com/calendar/v3/calendars/CALENDAR_ID/acl/RULE_ID --body-file /tmp/acl_patch.json`

Use this to remove a sharing rule only after user confirmation.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" proxy request --method DELETE --path /calendar.googleapis.com/calendar/v3/calendars/CALENDAR_ID/acl/RULE_ID`
<!-- /capability -->

<!-- capability: calendar_settings_read -->
### Read Calendar Settings

Allowed: read Google Calendar user settings.

Example commands:

Use this to list available setting IDs.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" proxy request --method GET --path /calendar.googleapis.com/calendar/v3/users/me/settings`

Use this after the settings list returns a setting ID.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" proxy request --method GET --path /calendar.googleapis.com/calendar/v3/users/me/settings/SETTING_ID`
<!-- /capability -->
