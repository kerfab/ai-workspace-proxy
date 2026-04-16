AI Workspace Proxy
===================

Originally developed by Fabien Kerbouci for The Sandbox / Animoca Brands.

Proof-of-concept notice
-----------------------

This project is provided as tested proof-of-concept software. Its purpose is to demonstrate how a middleware proxy can help enforce stronger security controls when AI agents access Google Workspace services.

In particular, it shows how a proxy can reduce operational risk even when Google OAuth scopes are broader than the exact set of actions that should be allowed to an AI agent. It also hides the permissive Google OAuth credentials to AI agents, by delivering a middleware API key instead.

Overview
--------
AI Workspace Proxy is a single-binary Go HTTP service that:
- lets users sign in to the proxy with Google
- only admits users whose Google email belongs to the configured allowed domain
- lets each user separately connect their AI Agent, through the proxy, to Google Workspace account via OAuth
- stores Google Workspace tokens server-side only
- issues a separate proxy API token to each user
- exposes relay endpoints for selected Google Workspace APIs
- enforces a server-side whitelist before forwarding requests to Google APIs
- stores operational state in SQLite and files only

The proxy may be granted advanced access to Google Workspace services, but it does not expose that full power directly to AI agents. Instead, it strictly inspects incoming HTTP requests and only relays operations that are explicitly permitted by policy.json. This allows the proxy to prevent dangerous or unwanted actions, ensuring that AI agents can use only the approved subset of capabilities.

Supported Google Workspace services/actions supported in this build
-------------------------------------------------------------------
The current `policy.json` allows the following proxy operations:

Gmail
- list messages
- read a message
- read a message attachment
- list threads
- read a thread
- list drafts
- read a draft
- create a draft
- update a draft
- list labels
- read a label
- create a label
- patch a label
- delete a label
- modify message labels
- modify thread labels

Practical Gmail behaviors enabled by the current label body policy:
- mark an email or thread as read by removing the `UNREAD` label
- archive an email or thread by removing the `INBOX` label

Google Calendar
- list the user calendar list
- read a calendar-list entry
- list events from accessible calendars
- read an individual event

Google Drive
- search/list files
- read file metadata
- export compatible Google Workspace files through Drive export
- create Drive files
- update Drive files through PATCH or PUT

Google Docs
- create a document
- read a document
- update a document through `documents.batchUpdate`

Google Sheets
- create a spreadsheet
- read a spreadsheet
- read spreadsheet values
- update spreadsheet values
- batch-update spreadsheet values
- batch-update spreadsheet structure/content

Google Slides
- create a presentation
- read a presentation
- read an individual presentation page
- batch-update a presentation

The exact allowlist is defined by `policy.json`, not by this README. If `policy.json` changes, the effective surface of the proxy changes with it.

Important scope note
--------------------
The current `policy.json` is no longer read-only. It authorizes Gmail modification, Calendar read access, Drive create/update/read access, and create/read/update access for Docs, Sheets, and Slides.

To support the current policy, the Google Workspace OAuth client should request and be configured for scopes broad enough to cover those actions. In practice, that means a grant at least as broad as:
- `https://www.googleapis.com/auth/gmail.modify`
- `https://www.googleapis.com/auth/gmail.labels`
- `https://www.googleapis.com/auth/calendar.readonly`
- `https://www.googleapis.com/auth/drive`
- `https://www.googleapis.com/auth/documents`
- `https://www.googleapis.com/auth/spreadsheets`
- `https://www.googleapis.com/auth/presentations`

The proxy then enforces a stricter policy at the relay layer with `policy.json`.

Important login and admin note
------------------------------
- Proxy login with Google is separate from the later Google Workspace OAuth step.
- Users may log in only if their Google email is verified and matches `ALLOWED_EMAIL_DOMAIN`.
- Admin access is driven only by `ADMIN_EMAILS`.
- At least one admin email must be configured at startup.
- During Google Workspace connect, the selected account must match the signed-in proxy user email.

Default routes
--------------
User:
- GET  /                             dashboard or login page
- GET  /auth/google/login            proxy login via Google
- GET  /auth/google/callback
- POST /logout
- GET  /auth/workspace/connect       start Google Workspace OAuth connection (must be logged in)
- GET  /auth/workspace/callback
- POST /auth/workspace/disconnect
- GET  /api/token/reveal             reveal proxy API token (web session required)
- POST /api/token/rotate             rotate the proxy API token
- POST /workspace/drive-folders/add
- POST /workspace/drive-folders/{id}/update
- POST /workspace/drive-folders/{id}/delete

Backward-compatible aliases:
- GET  /auth/gmail/connect
- GET  /auth/gmail/callback
- POST /auth/gmail/disconnect

Admin:
- GET  /admin/users
- GET  /admin/users/{id}
- POST /admin/users/{id}/suspend
- POST /admin/users/{id}/unsuspend
- POST /admin/users/{id}/delete

Proxy:
- ANY  /gmail.googleapis.com/*       policy-gated relay to Gmail API
- ANY  /calendar.googleapis.com/*    policy-gated relay to Google Calendar API
- ANY  /drive.googleapis.com/*       policy-gated relay to Google Drive API
- ANY  /docs.googleapis.com/*        policy-gated relay to Google Docs API
- ANY  /sheets.googleapis.com/*      policy-gated relay to Google Sheets API
- ANY  /slides.googleapis.com/*      policy-gated relay to Google Slides API

Google Cloud admin setup
------------------------
1. Create or select a Google Cloud project dedicated to the proxy.
2. Enable the APIs used by this build:
   - Gmail API
   - Google Calendar API
   - Google Drive API
   - Google Docs API
   - Google Sheets API
   - Google Slides API
3. Configure Google Auth Platform / OAuth consent:
   - set Branding information
   - choose the correct Audience (typically Internal for a company-only deployment)
   - declare the scopes required by this proxy
4. Create a Web application OAuth client.
5. Add authorized redirect URIs for the proxy.

Required redirect URIs
----------------------
The proxy uses these callback paths:
- `<APP_BASE_URL>/auth/google/callback`
- `<APP_BASE_URL>/auth/workspace/callback`

Examples:
- `https://workspace-proxy.company.com/auth/google/callback`
- `https://workspace-proxy.company.com/auth/workspace/callback`

Use the hostname allocated by your company for the proxy, and keep `APP_BASE_URL` synchronized with that externally visible hostname. If you expose the proxy on a non-default port, the port must be part of both `APP_BASE_URL` and the authorized redirect URIs. Example:
- `https://workspace-proxy.company.com:8443/auth/google/callback`
- `https://workspace-proxy.company.com:8443/auth/workspace/callback`

Legacy compatibility:
- older builds may also use `/auth/gmail/callback`, but current builds should primarily use `/auth/workspace/callback`.

Google Cloud admin checklist
----------------------------
- Enable the six Google Workspace APIs listed above.
- Create a Web application OAuth client.
- Add the exact redirect URIs used by the proxy.
- Copy the client ID and client secret into the proxy environment:
  - `GOOGLE_WORKSPACE_CLIENT_ID`
  - `GOOGLE_WORKSPACE_CLIENT_SECRET`
- Keep `APP_BASE_URL` synchronized with the real URL users browse to.
- If users reconnect after a scope expansion, ensure the OAuth client and consent screen have already been updated to include the new scopes.

Environment variables
---------------------
Required:
- `APP_BASE_URL`                Example: `https://workspace-proxy.company.com`
- `GOOGLE_WORKSPACE_CLIENT_ID`
- `GOOGLE_WORKSPACE_CLIENT_SECRET`
- `PROXY_ENCRYPTION_KEY`        32 raw bytes, or 64 hex chars, or base64 for 32 bytes
- `ALLOWED_EMAIL_DOMAIN`        Example: `company.com`
- `ADMIN_EMAILS`                comma-separated admin emails; at least one required

Other optional:
- `APP_BIND_ADDR`               default `:8080`
- `APP_NAME`                    default `AI Workspace Proxy`
- `DB_PATH`                     default `./db/ai_workspace_proxy.sqlite3`
- `POLICY_PATH`                 default `./policy.json`
- `DENIED_LOG_PATH`             default `./logs/denied.log`
- `SESSION_COOKIE_NAME`         default `ai_workspace_proxy_session`
- `COOKIE_SECURE`               default `false`
- `HTTP_CLIENT_TIMEOUT_SEC`     default `30`
- `SESSION_TTL_HOURS`           default `168`
- `MAX_REQUEST_BODY_BYTES`      default `10485760`

Allowed AI Drive folders
------------------------
Users can configure up to 5 allowed Google Drive folders from the dashboard. Each folder has:
- a unique Reference Name
- an extracted internal folder ID
- allowed file types (Docs, Sheets, Slides, generic Drive files)

For Drive/Docs/Sheets/Slides write requests, pass the query parameter:
- `driveRef=<Reference Name>`

The proxy resolves that Reference Name to a stored folder configuration and enforces folder-level restrictions server-side.

Known limitations and deployment recommendations
------------------------------------------------
- The proxy does not currently terminate TLS itself. If users require HTTPS/TLS encryption, place it behind an HTTPS-capable reverse proxy or access layer. A common pattern is to front it with Cloudflare Tunnel and/or Cloudflare Access, or with an internal reverse proxy/load balancer that handles TLS.
- The proxy should not be exposed directly to the public Internet as-is. It is intended for internal use. Prefer to publish it only behind your company's network controls such as a VPN, Zero Trust access gateway, or an access-control layer such as Cloudflare Access.
- Cloudflare Tunnel is a strong option when you want to avoid opening inbound ports or exposing a public IP address for the origin.
- The proxy itself supports Google Sign-In for application login. It does not natively authenticate users directly against third-party identity providers such as Okta.
- If your company uses another IdP such as Okta, you can still place the proxy behind an external access-control layer (for example Cloudflare Access) that authenticates users with that IdP before they ever reach the proxy.
- The README describes the intended/current build behavior, but `policy.json` remains the authoritative allowlist.
- This is an internal proxy and still deserves a security review before production use.

Operational notes
-----------------
- SQLite runs in WAL mode.
- Login OAuth state is stored safely even before a user record exists.
- Denied requests are logged to a rotating file set capped at 5 files total.
- Gmail relay requests only accept `/users/me/...` or `/users/<connected_account_email>/...` and normalize outbound Gmail requests to `/users/me/`.
- Calendar, Drive, Docs, Sheets, and Slides relays are all limited by `policy.json`.

Build and run
-------------
1. `./setup.sh`
2. Copy `config.env.example` to your own env file and fill the variables
3. `source` that env file
4. `make build`
5. `make run`

Example values
--------------
- `ALLOWED_EMAIL_DOMAIN=company.com`
- `ADMIN_EMAILS=admin@company.com`
