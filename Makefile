# renfe-cli — build, quality gate and security gate.
#
# The gates mirror a Python stack (black/ruff/mypy → gofmt/golangci-lint/vet,
# bandit → gosec, pip-audit → govulncheck/osv-scanner) plus the checks Go needs
# that Python does not: a tidy go.mod, the race detector, and module checksums.

BIN         ?= renfe
COVER_MIN   ?= 45
PKGS        := ./cmd/... ./internal/...

# Pinned so a gate result is reproducible; bump deliberately.
GOLANGCI_VERSION   ?= v2.12.2
GOSEC_VERSION      ?= v2.22.9
GOVULNCHECK_VERSION?= latest
CYCLONEDX_VERSION  ?= v1.9.0

GOBIN := $(shell go env GOPATH)/bin

# Build metadata, so `renfe version` answers for the binary in front of you.
# The release workflow passes the tag in as VERSION; a local build describes
# itself from git.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

.PHONY: build test fmt vet tidy tidy-check lint lint-if-present check clean \
        quality security gate race cover cover-check gosec govulncheck \
        modverify sbom tools help

## build: compile the CLI
build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/renfe

## test: run the unit tests
test:
	go test ./...

## race: run the tests under the race detector
race:
	go test -race ./...

## cover: write a coverage profile
cover:
	go test -coverprofile=coverage.out $(PKGS)
	@go tool cover -func=coverage.out | tail -1

## cover-check: fail when total coverage drops below COVER_MIN
cover-check: cover
	@total=$$(go tool cover -func=coverage.out | awk '/^total:/ {print $$3}' | tr -d '%'); \
	echo "total coverage: $$total% (minimum $(COVER_MIN)%)"; \
	awk -v t="$$total" -v m="$(COVER_MIN)" 'BEGIN { exit (t+0 >= m+0) ? 0 : 1 }' \
		|| { echo "coverage below the floor"; exit 1; }

## fmt: fail on unformatted files (the gofmt equivalent of black --check)
fmt:
	@test -z "$$(gofmt -l .)" || { echo "unformatted:"; gofmt -l .; exit 1; }

## vet: the compiler's own correctness checks
vet:
	go vet ./...

## tidy: rewrite go.mod/go.sum
tidy:
	go mod tidy

## tidy-check: fail when go.mod/go.sum are not tidy
tidy-check:
	@cp go.mod go.mod.bak && cp go.sum go.sum.bak; \
	go mod tidy; \
	if ! cmp -s go.mod go.mod.bak || ! cmp -s go.sum go.sum.bak; then \
		mv go.mod.bak go.mod; mv go.sum.bak go.sum; \
		echo "go.mod/go.sum are not tidy — run: make tidy"; exit 1; \
	fi; \
	rm -f go.mod.bak go.sum.bak

## lint: golangci-lint (staticcheck, revive, errcheck, gocritic, gosec…)
lint:
	golangci-lint run $(PKGS)

# Same as lint, but a missing linter is a warning rather than a failure, so
# `check` works on a machine that has not installed it.
lint-if-present:
	@command -v golangci-lint >/dev/null 2>&1 \
		&& golangci-lint run $(PKGS) \
		|| echo "warning: golangci-lint not installed — CI still runs it"

## modverify: confirm dependencies match their recorded checksums
modverify:
	go mod verify

# The bandit equivalent is gosec, and it runs as one of golangci-lint's linters
# (see .golangci.yml) — that is the copy the security gate relies on, because no
# released standalone gosec parses the Go 1.27 standard library yet: it fails
# with `internal error: package "bufio" without types`. The target below is kept
# for the day that is fixed, and reports the situation instead of failing the
# gate over a tool that cannot run.
## gosec: standalone SAST, best-effort (golangci-lint runs gosec regardless)
gosec:
	@command -v gosec >/dev/null 2>&1 || go install github.com/securego/gosec/v2/cmd/gosec@$(GOSEC_VERSION) || true
	@$(GOBIN)/gosec -quiet -exclude-dir=.codegraph ./... 2>/dev/null \
		|| echo "note: standalone gosec cannot parse this Go toolchain — gosec still runs inside golangci-lint"

## govulncheck: known vulnerabilities in dependencies AND the toolchain
govulncheck:
	@command -v govulncheck >/dev/null 2>&1 || go install golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)
	$(GOBIN)/govulncheck ./...

## sbom: write a CycloneDX SBOM for the module
sbom:
	@command -v cyclonedx-gomod >/dev/null 2>&1 || go install github.com/CycloneDX/cyclonedx-gomod/cmd/cyclonedx-gomod@$(CYCLONEDX_VERSION)
	$(GOBIN)/cyclonedx-gomod mod -licenses -json -output sbom.json
	@echo "wrote sbom.json"

## check: the everyday gate — what you run before every commit
check: fmt vet test build lint-if-present
	@echo "ok"

## quality: formatting, types, lint, tidiness, race, coverage floor
quality: fmt vet tidy-check lint race cover-check
	@echo "quality gate: ok"

## security: SAST (gosec via golangci-lint), dependency vulnerabilities, integrity
security: lint govulncheck modverify
	@echo "security gate: ok"

## gate: both gates — what CI enforces
gate: quality security
	@echo "all gates: ok"

## tools: install the pinned gate tooling
tools:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_VERSION)
	go install github.com/securego/gosec/v2/cmd/gosec@$(GOSEC_VERSION)
	go install golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)
	go install github.com/CycloneDX/cyclonedx-gomod/cmd/cyclonedx-gomod@$(CYCLONEDX_VERSION)

## clean: remove build and report artifacts
clean:
	rm -f $(BIN) coverage.out gosec.sarif golangci.sarif sbom.json

## help: list the targets
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## /  /'
