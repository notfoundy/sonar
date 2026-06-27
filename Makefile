BINARY   := sonar
DIST     := dist
COVERAGE := coverage.out

# Local build for the current OS/arch.
.PHONY: build
build:
	go build -o $(BINARY) .

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: vet
vet:
	go vet ./...

.PHONY: test
test:
	go test ./...

# Produce the coverage report consumed by the scanner (sonar.go.coverage.reportPaths).
.PHONY: coverage
coverage:
	go test -coverprofile=$(COVERAGE) ./...

# Generate coverage then scan the local server (requires 'sonar up').
.PHONY: scan
scan: build coverage
	./$(BINARY) scan

.PHONY: run
run: build
	./$(BINARY)

# Cross-compiled release binaries.
.PHONY: release
release:
	@mkdir -p $(DIST)
	@set -e; \
	for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64; do \
		os=$${target%/*}; arch=$${target#*/}; \
		out=$(DIST)/$(BINARY)-$$os-$$arch; \
		if [ "$$os" = "windows" ]; then out=$$out.exe; fi; \
		echo "building $$out"; \
		GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 go build -o $$out .; \
	done

.PHONY: clean
clean:
	rm -rf $(BINARY) $(DIST)
