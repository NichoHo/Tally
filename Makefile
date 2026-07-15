# Tally: payments ledger. Most targets run inside Docker so no local Go, protoc,
# or golangci-lint install is required. Docker is the only prerequisite.

MODULE      := github.com/nickho/tally
DB_URL      := postgres://tally:tally@postgres:5432/tally?sslmode=disable
TEST_DB_URL := postgres://tally:tally@localhost:5433/tally_test?sslmode=disable
GO_IMAGE    := golang:1.22

.PHONY: up down migrate seed test lint proto tidy web help

help:
	@echo "targets: up down migrate seed test lint proto tidy web"

web: ## run the Next.js dev server (needs Node 20+)
	cd web && npm install && npm run dev

up: ## start postgres + ledger + gateway
	docker compose up --build -d

down: ## stop everything
	docker compose down

migrate: ## run database migrations
	docker compose run --rm migrate

seed: ## insert demo accounts and transfers (gateway must be up)
	./scripts/seed.sh

# proto regenerates the gRPC code from proto/ledger.proto and refreshes go.sum.
proto:
	docker run --rm -v "$(CURDIR)":/work -w /work $(GO_IMAGE) bash -c '\
		apt-get update >/dev/null && apt-get install -y protobuf-compiler >/dev/null && \
		go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.34.2 && \
		go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.4.0 && \
		export PATH="$$PATH:$$(go env GOPATH)/bin" && \
		protoc --go_out=. --go_opt=module=$(MODULE) \
		       --go-grpc_out=. --go-grpc_opt=module=$(MODULE) proto/ledger.proto && \
		go mod tidy'

tidy: ## refresh go.mod/go.sum
	docker run --rm -v "$(CURDIR)":/work -w /work $(GO_IMAGE) go mod tidy

# test spins up a throwaway Postgres on a docker network and runs the Go suite
# (integration + concurrency tests) and the Python fraud tests against it.
TEST_NET := tally-test-net
TEST_PG_URL := postgres://tally:tally@tally-test-db:5432/tally_test?sslmode=disable

test:
	docker network create $(TEST_NET) >/dev/null 2>&1 || true
	docker rm -f tally-test-db >/dev/null 2>&1 || true
	docker run -d --name tally-test-db --network $(TEST_NET) \
		-e POSTGRES_USER=tally -e POSTGRES_PASSWORD=tally -e POSTGRES_DB=tally_test \
		postgres:16-alpine >/dev/null
	@echo "waiting for test postgres..."
	@until docker exec tally-test-db pg_isready -U tally -d tally_test >/dev/null 2>&1; do sleep 1; done
	docker run --rm --network $(TEST_NET) -v "$(CURDIR)":/work -w /work \
		-v tally-gomod:/go/pkg/mod \
		-e TEST_DATABASE_URL="$(TEST_PG_URL)" \
		$(GO_IMAGE) go test ./services/...
	docker build -q -t tally-fraud-test ./services/fraud >/dev/null
	docker run --rm --network $(TEST_NET) \
		-e TEST_DATABASE_URL="$(TEST_PG_URL)" \
		tally-fraud-test sh -c "pip install -q pytest && python -m pytest -q"
	docker rm -f tally-test-db >/dev/null 2>&1 || true

lint:
	docker run --rm -v "$(CURDIR)":/work -w /work golangci/golangci-lint:v1.60-alpine golangci-lint run ./services/...
	docker run --rm -v "$(CURDIR)":/work -w /work ghcr.io/astral-sh/ruff:0.5.0 check services/fraud
	cd web && npm run lint
