# Development tasks for phonenumber. CI runs these same targets, so a local
# run and a pipeline run measure the same thing.

COVERAGE   := coverage.out
# Generated files are data, not code. Every other function, the generator's
# included, must stay under a CRAP score of 30: complexity squared against
# missing coverage.
CRAP_FLAGS := --exclude '.*_gen(_test)?\.go' --threshold 30
FUZZTIME   ?= 30s

.PHONY: all check fmt vet test lint vuln cover crap crap-baseline fuzz generate sync-upstream bench mutants install-hooks clean

all: fmt check

# check runs everything CI checks except fuzzing: the linters (gofmt
# included), vet, the tests with the race detector, known vulnerabilities and
# the CRAP threshold. The tests include one that fails when the generated
# metadata differs from what resources/ produces.
check: lint vet test vuln crap

fmt:
	gofmt -s -w .

vet:
	go vet ./...

test:
	go test -race ./...

lint:
	@command -v golangci-lint >/dev/null || { \
		echo "golangci-lint not found: see https://golangci-lint.run/welcome/install/"; exit 1; \
	}
	golangci-lint run

vuln:
	go run golang.org/x/vuln/cmd/govulncheck@v1.7.0 ./...

# Produce the coverage profile the CRAP scan reads, so the scan does not run
# the suite a second time.
cover:
	go test -coverprofile=$(COVERAGE) ./...
	go tool cover -func=$(COVERAGE) | tail -1

crap: cover
	go run github.com/padiazg/go-crap@v0.5.1 scan ./... $(CRAP_FLAGS) --coverage-profile=$(COVERAGE) --fail-above --no-progress

# Regenerate the baseline used for regression comparison. Commit the result
# only when the new scores are intended.
crap-baseline: cover
	go run github.com/padiazg/go-crap@v0.5.1 scan ./... $(CRAP_FLAGS) --coverage-profile=$(COVERAGE) -f json -o crap-baseline.json --no-progress

# Fuzz every target in every package in turn. CI gives each 30s; run longer
# after changing the parser: make fuzz FUZZTIME=10m.
fuzz:
	@for pkg in $$(go list ./...); do \
		for target in $$(go test -list '^Fuzz' $$pkg | grep '^Fuzz'); do \
			echo "fuzzing $$pkg $$target for $(FUZZTIME)"; \
			go test -run '^$$' -fuzz "^$$target\$$" -fuzztime=$(FUZZTIME) $$pkg || exit 1; \
		done; \
	done

# Regenerate the metadata after updating resources/.
generate:
	go generate ./...

# Copy resources/ from upstream's latest release and regenerate. CI runs this
# daily and opens a pull request; see .github/workflows/upstream-sync.yml.
sync-upstream:
	scripts/sync-upstream.sh

bench:
	go test -run '^$$' -bench=. -benchmem -benchtime=200ms -count=10 ./...

# Mutation testing. Not part of CI, as a run is slow. Pinned, because
# releases after v0.4.0 fail Go checksum verification:
#
#   go install github.com/go-gremlins/gremlins/cmd/gremlins@v0.4.0
mutants:
	gremlins unleash --timeout-coefficient 12 .

# install-hooks points git at .githooks. Run it after cloning: core.hooksPath
# is local to your copy of the repository, so it is never cloned with it.
install-hooks:
	git config core.hooksPath .githooks

clean:
	go clean -testcache
	rm -f -- $(COVERAGE)
