<!-- capability: calendar_read -->
### Read Calendars And Events

Allowed: list calendars, read calendar metadata, read events, list event instances, and query free/busy.

Example commands:

Use this first to find calendar IDs.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" calendar list-calendars`

Use this to inspect one calendar. Replace `primary` with a calendar ID when needed.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" calendar get-calendar --calendar-id primary`

Use this to list events in a time window. Replace dates with ISO timestamps from the user request.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" calendar list-events --calendar-id primary --time-min '2026-06-11T00:00:00Z' --time-max '2026-06-12T00:00:00Z'`

Use this after `list-events` returns an event `id`.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" calendar get-event --calendar-id primary --event-id EVENT_ID`

Use `proxy request` for event instances and free/busy. Calendar IDs come from `list-calendars`.
<!-- /capability -->

<!-- capability: calendar_events_create_self -->
### Create Or Import Myself-Only Events

Allowed: create or import new calendar events only when the request has no third-party attendees, no room/resource attendees, no additional guests, and no organizer or creator other than the connected user.

Example commands:

Use this to create a personal event. Create `/tmp/event.json` without an `attendees` array unless it contains only the connected mailbox.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" proxy request --method POST --path /calendar.googleapis.com/calendar/v3/calendars/primary/events --body-file /tmp/event.json`

Use this to import a personal event from a raw Calendar event body.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" proxy request --method POST --path /calendar.googleapis.com/calendar/v3/calendars/primary/events/import --body-file /tmp/event.json`
<!-- /capability -->

<!-- capability: calendar_events_create_guests -->
### Create Or Import Events With Third Parties

Allowed: create or import new calendar events that include third-party attendees, room/resource attendees, additional guests, or an organizer or creator other than the connected user.

Example commands:

Use this to create an event with guests. Create `/tmp/event.json` with the real `attendees` array requested by the user.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" proxy request --method POST --path /calendar.googleapis.com/calendar/v3/calendars/primary/events --body-file /tmp/event.json`

Use this to import an event with guests from a raw Calendar event body.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" proxy request --method POST --path /calendar.googleapis.com/calendar/v3/calendars/primary/events/import --body-file /tmp/event.json`
<!-- /capability -->

<!-- capability: calendar_events_update_self -->
### Update Existing Myself-Only Events

Allowed: update an existing calendar event only when both the existing event and the requested update are myself-only.

Example commands:

Use this to edit a personal event. `EVENT_ID` comes from `list-events`; create `/tmp/event_patch.json` without third-party attendees or resources.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" proxy request --method PATCH --path /calendar.googleapis.com/calendar/v3/calendars/primary/events/EVENT_ID --body-file /tmp/event_patch.json`

Use this to replace a personal event. Create `/tmp/event.json` as the full replacement Calendar event body.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" proxy request --method PUT --path /calendar.googleapis.com/calendar/v3/calendars/primary/events/EVENT_ID --body-file /tmp/event.json`
<!-- /capability -->

<!-- capability: calendar_events_update_guests -->
### Update Or Move Events With Third Parties

Allowed: update calendar events involving third parties and move events between calendars. Move operations are treated as third-party-level changes.

Example commands:

Use this to edit an event with guests. `EVENT_ID` comes from `list-events`; create `/tmp/event_patch.json`.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" proxy request --method PATCH --path /calendar.googleapis.com/calendar/v3/calendars/primary/events/EVENT_ID --body-file /tmp/event_patch.json`

Use this to move an event to another calendar. `DESTINATION_CALENDAR_ID` comes from `list-calendars`.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" proxy request --method POST --path /calendar.googleapis.com/calendar/v3/calendars/primary/events/EVENT_ID/move --query destination=DESTINATION_CALENDAR_ID`
<!-- /capability -->

<!-- capability: calendar_events_delete_self -->
### Delete Existing Myself-Only Events

Allowed: delete an existing calendar event only when it has no third-party attendees, no room/resource attendees, no additional guests, and no organizer or creator other than the connected user.

Example commands:

Use this to delete a personal event only after user confirmation. `EVENT_ID` comes from `list-events`.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" proxy request --method DELETE --path /calendar.googleapis.com/calendar/v3/calendars/primary/events/EVENT_ID`
<!-- /capability -->

<!-- capability: calendar_events_delete_guests -->
### Delete Events With Third Parties

Allowed: delete existing calendar events that include third-party attendees, room/resource attendees, additional guests, or an organizer or creator other than the connected user.

Example commands:

Use this to delete an event with guests only after user confirmation. `EVENT_ID` comes from `list-events`.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" proxy request --method DELETE --path /calendar.googleapis.com/calendar/v3/calendars/primary/events/EVENT_ID`
<!-- /capability -->

<!-- capability: calendar_calendars_manage -->
### Manage Calendars And Calendar Lists

Allowed: create, update, clear, or delete calendars and calendar-list entries.

Example commands:

Use this to create a calendar. Create `/tmp/calendar.json`.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" proxy request --method POST --path /calendar.googleapis.com/calendar/v3/calendars --body-file /tmp/calendar.json`

Use this to edit a calendar. `CALENDAR_ID` comes from `list-calendars`.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" proxy request --method PATCH --path /calendar.googleapis.com/calendar/v3/calendars/CALENDAR_ID --body-file /tmp/calendar_patch.json`

Use this to add a calendar to the user's calendar list. Create `/tmp/calendar_list_entry.json`.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" proxy request --method POST --path /calendar.googleapis.com/calendar/v3/users/me/calendarList --body-file /tmp/calendar_list_entry.json`
<!-- /capability -->

<!-- capability: calendar_access_read -->
### Read Calendar Sharing

Allowed: read calendar access-control rules.

Example commands:

Use this to list sharing rules for a calendar.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" proxy request --method GET --path /calendar.googleapis.com/calendar/v3/calendars/CALENDAR_ID/acl`

Use this to inspect one sharing rule. `RULE_ID` comes from the ACL list.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" proxy request --method GET --path /calendar.googleapis.com/calendar/v3/calendars/CALENDAR_ID/acl/RULE_ID`
<!-- /capability -->

<!-- capability: calendar_access_manage -->
### Manage Calendar Sharing

Allowed: create, update, delete, or watch calendar sharing rules.

Example commands:

Use this to add a sharing rule. Create `/tmp/acl_rule.json`.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" proxy request --method POST --path /calendar.googleapis.com/calendar/v3/calendars/CALENDAR_ID/acl --body-file /tmp/acl_rule.json`

Use this to edit a sharing rule. `RULE_ID` comes from the ACL list.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" proxy request --method PATCH --path /calendar.googleapis.com/calendar/v3/calendars/CALENDAR_ID/acl/RULE_ID --body-file /tmp/acl_patch.json`

Use this to remove a sharing rule only after user confirmation.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" proxy request --method DELETE --path /calendar.googleapis.com/calendar/v3/calendars/CALENDAR_ID/acl/RULE_ID`
<!-- /capability -->

<!-- capability: calendar_settings_read -->
### Read Calendar Settings

Allowed: read Google Calendar user settings.

Example commands:

Use this to list available setting IDs.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" proxy request --method GET --path /calendar.googleapis.com/calendar/v3/users/me/settings`

Use this after the settings list returns a setting ID.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" proxy request --method GET --path /calendar.googleapis.com/calendar/v3/users/me/settings/SETTING_ID`
<!-- /capability -->
