BINARY  := mdl-demo
GODIR   := go
PKG     := ./cmd/mdl-demo
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X main.version=$(VERSION)

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

# The build context is the repo root.
image:
	sudo podman build -t mdl-demo --build-arg VERSION=$(VERSION) -f container/Containerfile .

# Test container for manual testing inside an mpd VM (see dev/README.md).
TEST_NAME := mpd-test-mdl-demo
VM_ID     := $(shell jq -r .vmId /srv/meta/vm.json 2>/dev/null || hostname | sed 's/^mpd-//')
BRIDGE_IP := 10.163.$(VM_ID).1
run:
	sudo podman rm -f --ignore $(TEST_NAME)
	sudo podman run -d --name $(TEST_NAME) \
		-e MDL_DEMO_PORT=6381 -e MDL_DEMO_NAME="mpd test" \
		-p $(BRIDGE_IP):6381:8081 -p $(BRIDGE_IP):6382:8082 mdl-demo
	@until curl -fs -o /dev/null http://$(BRIDGE_IP):6381/; do sleep 0.2; done
	sudo podman exec $(TEST_NAME) mdl-demo url --site https://site.mdl-demo.$(VM_ID).mpd.test

	@echo ""
	@echo "test console: http://$(BRIDGE_IP):6381"
	@echo ""
