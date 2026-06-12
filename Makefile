.PHONY: build run test integration load lint

COMPOSE ?= docker compose
TOOLS   := $(COMPOSE) --profile tools run --rm tools

# Load test knobs (override on the CLI, e.g. `make load LANG=mixed C=6 N=60`)
LANG ?= py3
C    ?= 10
N    ?= 100
URL  ?= http://localhost:8080/run

build:
	$(COMPOSE) build goboxd

run:
	$(COMPOSE) up goboxd

test:
	$(TOOLS) go test ./...

integration:
	$(TOOLS) env LANGUAGE_CONFIG=/src/configs/languages/languages.yaml go test -tags=integration ./tests/...

load:
	go run scripts/loadtest.go -c $(C) -n $(N) -url $(URL) -lang $(LANG)

lint:
	$(TOOLS) golangci-lint run ./...
