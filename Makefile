.PHONY: help clean test bench prof-cpu sniff tidy

GO_TEST     := CGO_ENABLED=1 GOMAXPROCS=1 go test -count=1 -race -v -coverprofile=./coverage.out
GO_BENCH    := CGO_ENABLED=0 GOMAXPROCS=1 go test -count=1 -benchmem -bench=.
GO_PROF_CPU := CGO_ENABLED=0 GOMAXPROCS=1 go test -count=1 -cpuprofile=cpu.prof -bench=.

help:                   # Displays this list
	@echo; grep "^[a-z][a-zA-Z0-9_<> -]\+:" Makefile | sed -E "s/:[^#]*?#?(.*)?/\r\t\t\1/" | sed "s/^/ make /"; echo
	@echo " Usage: make <TARGET> [ARGS=...]"; echo

clean:                  # Removes build/test artifacts
	@2>/dev/null rm ./coverage.html || true
	@find . -type f | grep "\.out$$"  | xargs -I{} rm {};
	@find . -type f | grep "\.test$$" | xargs -I{} rm {};
	@find . -type f | grep "\.prof$$" | xargs -I{} rm {};

test: clean             # Runs tests with -race  (pick: ARGS="-run=<Name>")
	$(GO_TEST) $(ARGS) .
	@go tool cover -html=./coverage.out -o ./coverage.html
	@echo "coverage: <\e[32mfile://$(PWD)/coverage.html\e[0m>"

bench: clean            # Artificial benchmarks  (pick: ARGS="-bench=<Name>")
	$(GO_BENCH) $(ARGS) .

prof-cpu: clean         # Creates CPU profile    (pick: ARGS="-bench=<Name>")
	$(GO_PROF_CPU) $(ARGS) .
	go tool pprof -top cpu.prof | head -40

sniff:                  # Checks format and runs linter (void on success)
	@find . -type f -not -path "*/\.*" -name "*.go" | xargs -I{} gofmt -d {}
	@go vet ./... || true
	@>/dev/null which revive || (echo "Missing a linter, install with:  go install github.com/mgechev/revive" && false)
	@revive -config .revive.toml ./...

tidy:                   # Formats source files, cleans go.mod
	@find . -type f -not -path "*/\.*" -name "*.go" | xargs -I{} gofmt -w {}
	@go mod tidy
