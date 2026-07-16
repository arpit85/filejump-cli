BINARY   := filejump
VERSION  := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  := -X main.version=$(VERSION)
PKGS     := $(shell go list ./... | grep -v /cmd | true)

.PHONY: build run test vet clean release install

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

run: build
	./$(BINARY) $(ARGS)

test:
	go test ./...

vet:
	go vet ./...

install: build
	cp $(BINARY) $(shell go env GOPATH)/bin/

clean:
	rm -f $(BINARY) dist

# Cross-compile binaries into dist/ for common platforms.
release: clean
	mkdir -p dist
	@for target in \
		"darwin/amd64" "darwin/arm64" \
		"linux/amd64" "linux/arm64" \
		"windows/amd64" "windows/arm64"; do \
		os=$${target%/*}; arch=$${target#*/}; ext=""; \
		[ $$os = windows ] && ext=".exe"; \
		echo "  -> $$os/$$arch"; \
		GOOS=$$os GOARCH=$$arch go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-$$os-$$arch$$ext .; \
	done
	@ls -lh dist/
