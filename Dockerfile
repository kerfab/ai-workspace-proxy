FROM golang:1.22-alpine AS build

RUN apk add --no-cache build-base sqlite-dev

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY src ./src
COPY policy.json ./policy.json

RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/ai-workspace-proxy ./src

FROM alpine:3.20

RUN apk add --no-cache ca-certificates sqlite-libs tzdata

WORKDIR /app

COPY --from=build /out/ai-workspace-proxy /app/ai-workspace-proxy
COPY policy.json /app/policy.json
COPY docker-entrypoint.sh /app/docker-entrypoint.sh

RUN chmod +x /app/docker-entrypoint.sh \
    && mkdir -p /data/db /data/logs

ENV APP_NAME="AI Workspace Proxy" \
    APP_BIND_ADDR=":80" \
    DB_PATH="/data/db/ai_workspace_proxy.sqlite3" \
    POLICY_PATH="/app/policy.json" \
    DENIED_LOG_PATH="/data/logs/denied.log" \
    COOKIE_SECURE="true"

EXPOSE 80

VOLUME ["/data/db", "/data/logs"]

ENTRYPOINT ["/app/docker-entrypoint.sh"]
CMD ["/app/ai-workspace-proxy"]
