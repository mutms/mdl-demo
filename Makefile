BINARY  := mdl-demo
GODIR   := go
PKG     := ./cmd/mdl-demo
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X main.version=$(VERSION)

# Build with the Go that Debian Trixie ships (golang-go, currently 1.24.x)
# and nothing else.
#
# Go's default GOTOOLCHAIN=auto silently downloads a whole toolchain —
# 210 MB — when go.mod, or any dependency's go.mod, names a newer version
# than the installed one. That happens per machine, at build time, over
# the network, with no warning. `local` turns it into an immediate,
# legible build failure instead.
#
# If you hit that failure: lower the `go` directive in go.mod — do not
# raise the floor above what Trixie packages. (Same rule, same wording,
# as mudev's Makefile.)
export GOTOOLCHAIN = local

.PHONY: build build-static test vet fmt fmt-check tidy clean image run

# Local developer build (native).
build:
	cd $(GODIR) && go build -ldflags "$(LDFLAGS)" -o ../bin/$(BINARY) $(PKG)

# Self-contained static binaries (what the image build stage produces).
build-static:
	cd $(GODIR) && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o ../dist/$(BINARY)-linux-amd64 $(PKG)
	cd $(GODIR) && CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o ../dist/$(BINARY)-linux-arm64 $(PKG)

test:
	cd $(GODIR) && go test ./...

vet:
	cd $(GODIR) && go vet ./...

# Apply canonical Go formatting.
fmt:
	cd $(GODIR) && gofmt -w .

# Fail if anything is not gofmt-clean (for CI / pre-commit).
fmt-check:
	@unformatted=$$(cd $(GODIR) && gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "not gofmt-clean:"; echo "$$unformatted"; exit 1; \
	fi

tidy:
	cd $(GODIR) && go mod tidy

clean:
	rm -rf bin dist

# The build context is the repo root: the Containerfile's build stage
# COPYs go/ into the image. Rootful podman matches the rootful runtimes
# (Apple container, WSL) the image targets.
image:
	sudo podman build -t mdl-demo --build-arg VERSION=$(VERSION) -f containers/base/Containerfile .

# Throwaway local instance for manual testing (see dev/README.md). No --cap-add:
# podman's systemd mode handles cgroups, and --cap-add ALL breaks systemd
# 257's generator sandboxing. (Apple `container` DOES need --cap-add ALL.)
run:
	sudo podman run -d --name demo \
		-p 127.0.0.1:8080:8080 -p 127.0.0.1:8081:8081 mdl-demo
