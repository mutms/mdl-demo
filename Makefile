BINARY  := mdl-demo
GODIR   := go
PKG     := ./cmd/mdl-demo
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X main.version=$(VERSION)

# The `go` directive in go/go.mod picks the compiler: the go command
# fetches that toolchain itself when the installed one is older
# (GOTOOLCHAIN=auto, the default). Bump the directive to move to a newer Go.

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
	sudo podman build -t mdl-demo --build-arg VERSION=$(VERSION) -f container/Containerfile .

# Test container for manual testing inside an mpd VM (see dev/README.md).
# The name and ports 6381/6382 are a contract with mpd's `mdl-demo` project
# type: its runtime caddy publishes https://mdl-demo.<vm>.mpd.test → :6381
# and https://site.mdl-demo.<vm>.mpd.test → :6382, reaching this VM at its
# bridge address — hence no 127.0.0.1 bind (the vmnet is host-only anyway).
# One test demo per VM: the previous one is removed first. Once the console
# answers (identity adopted into state), the container is told the site's
# public URL so the install form suggests the caddy address. The console
# itself is NOT published that way — it answers to localhost and IP addresses
# only (go/internal/webui/auth.go), so reach it at http://<vm-ip>:6381.
TEST_NAME := mpd-test-mdl-demo
VM_ID     := $(shell jq -r .vmId /srv/meta/vm.json 2>/dev/null || hostname | sed 's/^mpd-//')
run:
	sudo podman rm -f --ignore $(TEST_NAME)
	sudo podman run -d --name $(TEST_NAME) \
		-e MDL_DEMO_PORT=6381 -e MDL_DEMO_NAME="mpd test" \
		-p 6381:8081 -p 6382:8082 mdl-demo
	@until curl -fs -o /dev/null http://127.0.0.1:6381/; do sleep 0.2; done
	sudo podman exec $(TEST_NAME) mdl-demo url --site https://site.mdl-demo.$(VM_ID).mpd.test
