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

# Test container for manual testing inside an mpd VM (see dev/README.md).
# The name and ports 6381/6382 are a contract with mpd's `mdl-demo` project
# type: its runtime caddy publishes https://mdl-demo.<vm>.mpd.test → :6381
# and https://site.mdl-demo.<vm>.mpd.test → :6382, reaching this VM at its
# bridge address — hence no 127.0.0.1 bind (the vmnet is host-only anyway).
# One test demo per VM: the previous one is removed first. Once the console
# answers (identity adopted into state), the container is told its public
# URLs so the install form suggests the caddy address. Export
# MDL_DEMO_PASSWORD to skip the first-access password form.
TEST_NAME := mpd-test-mdl-demo
VM_ID     := $(shell jq -r .vmId /srv/meta/vm.json 2>/dev/null || hostname | sed 's/^mpd-//')
run:
	sudo podman rm -f --ignore $(TEST_NAME)
	sudo podman run -d --name $(TEST_NAME) \
		-e MDL_DEMO_PORT=6381 -e MDL_DEMO_NAME="mpd test" \
		$(if $(MDL_DEMO_PASSWORD),-e MDL_DEMO_PASSWORD='$(MDL_DEMO_PASSWORD)') \
		-p 6381:8081 -p 6382:8082 mdl-demo
	@until curl -fs -o /dev/null http://127.0.0.1:6381/; do sleep 0.2; done
	sudo podman exec $(TEST_NAME) mdl-demo url \
		--console https://mdl-demo.$(VM_ID).mpd.test --site https://site.mdl-demo.$(VM_ID).mpd.test
