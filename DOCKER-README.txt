AI Workspace Proxy Docker Bundle
=================================

Files
-----
- Dockerfile
- docker-entrypoint.sh
- .dockerignore

Assumptions
-----------
- Build context is the root of the proxy project
- The project contains:
  - go.mod
  - go.sum
  - policy.json
  - src/

Runtime behavior
----------------
- The container listens on TCP/80 on all interfaces
- The SQLite DB is stored under /data/db
- The denied-request logs are stored under /data/logs
- Mount those two host paths separately

Important environment variables
-------------------------------
Required at runtime:
- APP_BASE_URL
- GOOGLE_WORKSPACE_CLIENT_ID
- GOOGLE_WORKSPACE_CLIENT_SECRET
- PROXY_ENCRYPTION_KEY
- ALLOWED_EMAIL_DOMAINS
- ADMIN_EMAILS

See file config.env.example for more fine-tuning variables.

Build
-----
docker build -t ai-workspace-proxy .

Run
---
docker run -d \
  --name ai-workspace-proxy \
  -p 80:80 \
  -v /YOUR/HOST/DB:/data/db \
  -v /YOUR/HOST/LOGS:/data/logs \
  -e APP_BASE_URL="http://YOUR-HOSTNAME-OR-IP" \
  -e GOOGLE_WORKSPACE_CLIENT_ID="..." \
  -e GOOGLE_WORKSPACE_CLIENT_SECRET="..." \
  -e PROXY_ENCRYPTION_KEY="..." \
  -e ALLOWED_EMAIL_DOMAINS="YOUR_COMPANY_DOMAIN.com,gmail.com" \
  -e ADMIN_EMAILS="admin@YOUR_COMPANY_DOMAIN.com" \
  -e MAX_REQUEST_BODY_BYTES="30000000" \
  ai-workspace-proxy

Default paths inside container
------------------------------
- DB_PATH=/data/db/ai_workspace_proxy.sqlite3
- DENIED_LOG_PATH=/data/logs/denied.log

Notes
-----
- If you publish the container on a different host port, APP_BASE_URL must match the URL users actually browse to.
  Example: if you run -p 8080:80, then APP_BASE_URL should be http://localhost:8080
- Google OAuth redirect URIs must match APP_BASE_URL exactly, including port if not default http/https for 80/443.
