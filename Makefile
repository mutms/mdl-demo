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

# Remove every mdl-demo test container — the default `mpd-test-mdl-demo` and any
# PORT= variants (mpd-test-mdl-demo-<port>). Leaves other projects' containers
# alone. `-` + `xargs -r`: a no-op when none exist.
.PHONY: clean-test
clean-test:
	-sudo podman ps -a --format '{{.Names}}' | grep '^mpd-test-mdl-demo' | xargs -r sudo podman rm -f

# Full teardown of mdl-demo's podman footprint: all test containers, the image,
# and reclaimable space (dangling images + build cache). Does NOT remove other
# projects' tagged images or any volumes. For a VM-wide sweep that also drops
# other projects' unused images, run `sudo podman system prune -a` yourself.
.PHONY: clean-podman
clean-podman: clean-test
	-sudo podman rmi -f mdl-demo
	-sudo podman image prune -f
	-sudo podman builder prune -f

# The build context is the repo root. Each build retags mdl-demo, so the
# previous one goes dangling (<none>) — a few GB when the offline mirrors are
# baked in. Prune just those afterwards to keep disk in check; the build-cache
# layers are left alone so the next build stays fast. For a fuller sweep (image +
# cache + test containers) use `make clean-podman`.
image:
	sudo podman build -t mdl-demo --build-arg VERSION=$(VERSION) -f container/Containerfile .
	-sudo podman image prune -f

# Test container for manual testing inside an mpd VM (see dev/README.md).
# PORT overrides the console port (site = PORT+1) so several test containers can
# run side by side — a separate one per person/agent. The default 6381 keeps the
# bare name for back-compat; any other port suffixes it. `make run PORT=6391`,
# then `make hotpatch PORT=6391` targets that same container.
PORT      ?= 6381
SITEPORT  := $(shell expr $(PORT) + 1)
VM_ID     := $(shell jq -r .vmId /srv/meta/vm.json 2>/dev/null || hostname | sed 's/^mpd-//')
BRIDGE_IP := 10.163.$(VM_ID).1
ifeq ($(PORT),6381)
TEST_NAME ?= mpd-test-mdl-demo
SITE_URL  ?= https://mdl-demo.$(VM_ID).mpd.test
else
TEST_NAME ?= mpd-test-mdl-demo-$(PORT)
SITE_URL  ?= https://mdl-demo-$(PORT).$(VM_ID).mpd.test
endif
run:
	sudo podman rm -f --ignore $(TEST_NAME)
	sudo podman run -d --name $(TEST_NAME) \
		-e MDL_DEMO_PORT=$(PORT) \
		-p $(BRIDGE_IP):$(PORT):8081 -p $(BRIDGE_IP):$(SITEPORT):8082 mdl-demo
	@until curl -fs -o /dev/null http://$(BRIDGE_IP):$(PORT)/; do sleep 0.2; done
	sudo podman exec $(TEST_NAME) mdl-demo url --site $(SITE_URL)

	@echo ""
	@echo "test console: http://$(BRIDGE_IP):$(PORT)  ($(TEST_NAME))"
	@echo ""

# Hot-patch the running test container with a freshly built binary — far faster
# than `make image` for iterating on Go/template/CSS/JS changes (all embedded in
# the binary). NOTE: PHP under container/php/ is NOT updated this way — rebuild the image
# for that. The site, database and dataroot survive the restart; the console's
# epoch changes, so open browser tabs reload themselves.
#
# The binary is swapped via a temp name + mv: the old file is the running PID 1,
# so it cannot be truncated in place (ETXTBSY), but renaming over it is fine.
.PHONY: hotpatch
hotpatch:
	cd $(GODIR) && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o ../dist/$(BINARY)-linux-amd64 $(PKG)
	sudo podman cp dist/$(BINARY)-linux-amd64 $(TEST_NAME):/usr/bin/mdl-demo.new
	sudo podman exec $(TEST_NAME) mv -f /usr/bin/mdl-demo.new /usr/bin/mdl-demo
	sudo podman restart $(TEST_NAME)
	@echo "hot-patched $(TEST_NAME) with $(VERSION)"

	@echo ""
	@echo "test console: http://$(BRIDGE_IP):$(PORT)  ($(TEST_NAME))"
	@echo ""
