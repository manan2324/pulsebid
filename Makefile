SHELL := bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := help

GO ?= go
COMPOSE ?= docker compose
COMPOSE_FILE := deploy/docker-compose.yml
COMPOSE_ENV_FILE ?= $(if $(wildcard .env),.env,.env.example)
DC = $(COMPOSE) --env-file $(COMPOSE_ENV_FILE) -f $(COMPOSE_FILE)
GOLANGCI_LINT_VERSION := v2.15.0
LINT = $(GO) run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
SERVICE ?= bid-gateway
N ?= 8
RATE ?= 5000
DURATION ?= 2m

.PHONY: help up down reset topics build lint fmt test test-int bench fuzz \
        migrate-up migrate-down proto sqlc generate run-campaign run-gateway \
        run-aggregator bidders run-gateway-replicas seed test-api test-report \
        load verify-no-overspend docker-build test-platform test-grpc

help:
	@printf '%s\n' 'make up topics lint test build' 'make test-platform checks all five admin servers and SIGTERM (Linux/macOS).' 'make down stops infrastructure and preserves data.' 'make reset deletes local infrastructure volumes (requires CONFIRM=1).' 'Later-phase targets report their required phase until implemented.'

up:
	$(DC) up -d --wait --wait-timeout 180

down:
	$(DC) --profile apps --profile tools --profile scale down --remove-orphans

reset:
	@test "$(CONFIRM)" = 1 || { echo 'Deletes local data. Run make reset CONFIRM=1 to proceed.' >&2; exit 1; }
	$(DC) --profile apps --profile tools --profile scale down --volumes --remove-orphans

topics:
	@COMPOSE='$(COMPOSE)' COMPOSE_ENV_FILE='$(COMPOSE_ENV_FILE)' bash scripts/create-topics.sh

build:
	$(GO) build ./...

lint:
	$(LINT) fmt --diff
	$(LINT) run ./...

fmt:
	$(LINT) fmt

test:
	$(GO) test -race -cover ./...

test-platform:
	bash scripts/smoke-platform.sh

test-grpc:
	bash scripts/test-grpc.sh

bench:
	$(GO) test -run '^$$' -bench . -benchmem ./...

test-int:
	$(GO) test -race -count=1 -tags=integration -coverpkg=./internal/campaign,./migrations -coverprofile=coverage-integration.out ./internal/campaign ./migrations

migrate-up:
	@COMPOSE_ENV_FILE='$(COMPOSE_ENV_FILE)' bash scripts/migrate.sh up

migrate-down:
	@COMPOSE_ENV_FILE='$(COMPOSE_ENV_FILE)' bash scripts/migrate.sh down

sqlc:
	MSYS_NO_PATHCONV=1 docker run --rm --mount 'type=bind,source=$(CURDIR),target=/src' --workdir /src sqlc/sqlc:1.31.1 generate

proto:
	docker build --file deploy/proto.Dockerfile --tag pulsebid/proto-tools:local .
	MSYS_NO_PATHCONV=1 docker run --rm --mount 'type=bind,source=$(CURDIR),target=/work' pulsebid/proto-tools:local format -w
	MSYS_NO_PATHCONV=1 docker run --rm --mount 'type=bind,source=$(CURDIR),target=/work' pulsebid/proto-tools:local lint
	MSYS_NO_PATHCONV=1 docker run --rm --mount 'type=bind,source=$(CURDIR),target=/work' pulsebid/proto-tools:local generate

generate: proto sqlc

run-campaign:
	@COMPOSE_ENV_FILE='$(COMPOSE_ENV_FILE)' bash scripts/run-campaign.sh

run-gateway:
	$(GO) run ./cmd/bid-gateway

run-aggregator:
	$(GO) run ./cmd/event-aggregator

bidders:
	@bash scripts/run-bidders.sh $(N)

seed:
	RUN_SEED=1 $(GO) run ./cmd/seed

fuzz:
	@bash scripts/phase-required.sh 5 fuzz

run-gateway-replicas:
	@bash scripts/run-gateway-replicas.sh

verify-no-overspend:
	@bash scripts/verify-no-overspend.sh

test-api:
	@COMPOSE='$(COMPOSE)' COMPOSE_ENV_FILE='$(COMPOSE_ENV_FILE)' bash scripts/test-api.sh

test-report:
	@bash scripts/test-report.sh

load:
	@bash scripts/run-loadtest.sh $(RATE) $(DURATION)

docker-build:
	docker build --file deploy/Dockerfile --build-arg SERVICE=$(SERVICE) --tag pulsebid/$(SERVICE):local .
