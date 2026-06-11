APP_NAME=AI Workspace Proxy
BINARY=ai-workspace-proxy
ROOT_DIR:=$(CURDIR)
SRC_DIR:=$(ROOT_DIR)/src
BIN_DIR:=$(ROOT_DIR)/bin
DB_DIR:=$(ROOT_DIR)/db
LOG_DIR:=$(ROOT_DIR)/logs

.PHONY: all build clean run fmt docker

all: build

build:
	mkdir -p $(BIN_DIR) $(DB_DIR) $(LOG_DIR)
	CGO_ENABLED=1 go build -o $(BIN_DIR)/$(BINARY) ./src

run: build
	APP_NAME="$(APP_NAME)" \
	DB_PATH=$(DB_DIR)/ai_workspace_proxy.sqlite3 \
	DENIED_LOG_PATH=$(LOG_DIR)/denied.log \
	./$(BIN_DIR)/$(BINARY)

fmt:
	gofmt -w $(SRC_DIR)/*.go

docker:
	DOCKER_BUILDKIT=1 BUILDX_GIT_INFO=false docker build -t $(BINARY) .

clean:
	rm -f $(BIN_DIR)/$(BINARY)
