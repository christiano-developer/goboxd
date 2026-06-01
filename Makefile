.PHONY: build run test integration load lint

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

load:
	go run scripts/loadtest.go -c 10 -n 100 -url http://localhost:8080/run -lang py3

lint:
	$(TOOLS) golangci-lint run ./...
