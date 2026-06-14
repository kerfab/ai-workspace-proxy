## How To Use These Examples

- Commands are examples. Replace uppercase placeholders such as `CALENDAR_ID` or `EVENT_ID` with real values from earlier command output.
- The `--workspace` value is already set to this workspace's friendly name. Keep it unless the user explicitly asks for another workspace.
- File paths such as `/tmp/request.json` mean the agent must create that local file before running the command.
- `CALENDAR_ID` usually comes from `list-calendars`; use `primary` for the main calendar. `EVENT_ID` comes from `list-events`.
- For event write policies, the proxy treats attendees other than the connected mailbox, room/resource attendees, `additionalGuests`, or an organizer or creator other than the connected mailbox as third-party involvement.
- Updates and deletes may require the proxy to inspect the existing Google Calendar event before forwarding the user's requested operation.
