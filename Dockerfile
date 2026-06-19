# syntax=docker/dockerfile:1.7

FROM golang:1.22-alpine AS build

RUN apk add --no-cache build-base sqlite-dev

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY src ./src

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=1 \
    go build -trimpath -ldflags="-s -w" -o /out/ai-workspace-proxy ./src

FROM alpine:3.20

RUN apk add --no-cache ca-certificates sqlite-libs tzdata

WORKDIR /app

COPY --from=build /out/ai-workspace-proxy /app/ai-workspace-proxy
COPY docker-entrypoint.sh /app/docker-entrypoint.sh

RUN chmod +x /app/docker-entrypoint.sh \
    && mkdir -p /data/db

ENV APP_NAME="AI Workspace Proxy" \
    APP_BIND_ADDR=":80" \
    DB_PATH="/data/db/ai_workspace_proxy.sqlite3" \
    COOKIE_SECURE="true"

EXPOSE 80

VOLUME ["/data/db"]

ENTRYPOINT ["/app/docker-entrypoint.sh"]
CMD ["/app/ai-workspace-proxy"]
