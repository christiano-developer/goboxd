.PHONY: build run test integration lint

COMPOSE ?= docker compose
TOOLS   := $(COMPOSE) --profile tools run --rm tools

build:
	$(COMPOSE) build goboxd

run:
	$(COMPOSE) up goboxd

test:
	$(TOOLS) go test ./...

integration:
	$(TOOLS) env LANGUAGE_CONFIG=/src/configs/languages/languages.yaml go test -tags=integration ./tests/...

lint:
	$(TOOLS) golangci-lint run ./...
