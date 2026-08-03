BINARY := bin/seahorse
DIST := dist
PKG_VERSION := $(shell sed -nE "s/.*APP_VERSION = '([^']+)'.*/\1/p" frontend/src/version.ts)
NFPM := go run github.com/goreleaser/nfpm/v2/cmd/nfpm@latest

.PHONY: frontend build build-linux-amd64 build-linux-arm64 \
	package-deb-amd64 package-deb-arm64 package-rpm-amd64 package-rpm-arm64 \
	packages build-all all dev clean

frontend:
	npm --prefix frontend install
	npm --prefix frontend run build

build: frontend
	go build -o $(BINARY) ./cmd/seahorse

# Cross-compiles a fully static linux binary from macOS (or any host), for
# deploying to a Linux server. Runs the actual build inside a Docker container
# instead of just setting GOOS/GOARCH, for two reasons: (1) this project's
# sqlite driver needs cgo, which silently no-ops without a matching C
# toolchain for the target, producing a binary that builds fine but panics at
# runtime; (2) building against musl (Alpine) rather than the host's or a
# Debian container's glibc avoids depending on a specific glibc version, so
# the result runs on any distro/version for that architecture, not just the
# one that happens to match the builder image. Requires Docker Desktop
# running.
#
# These two targets are written out explicitly (rather than a single
# build-linux-% pattern rule) because the GNU Make 3.81 that ships with macOS
# has a long-standing bug where a target that's both .PHONY and matched only
# via a pattern rule silently no-ops instead of running its recipe.
#
# Both start with a Docker reachability check: `docker info`/`docker run`
# hang indefinitely (not a quick error) when Docker Desktop isn't running, so
# without this a plain `make build-all` would just sit there forever with no
# clue why. Races `docker info` against a 5s watchdog rather than using
# `timeout`/`gtimeout`, since neither is guaranteed present on macOS.
build-linux-amd64: frontend
	@docker info >/dev/null 2>&1 & pid=$$!; \
	( sleep 5; kill -9 $$pid ) >/dev/null 2>&1 & watchdog=$$!; \
	if wait $$pid 2>/dev/null; then kill $$watchdog >/dev/null 2>&1; else \
		echo "Docker doesn't seem to be running (no response after 5s) -- start Docker Desktop and try again." >&2; \
		exit 1; \
	fi
	docker run --rm --platform linux/amd64 \
		-v "$(CURDIR)":/src -w /src \
		golang:1.26-alpine \
		sh -c "apk add --no-cache gcc musl-dev >/dev/null && \
			CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build \
			-ldflags '-linkmode external -extldflags \"-static\"' \
			-o bin/seahorse-linux-amd64 ./cmd/seahorse"

build-linux-arm64: frontend
	@docker info >/dev/null 2>&1 & pid=$$!; \
	( sleep 5; kill -9 $$pid ) >/dev/null 2>&1 & watchdog=$$!; \
	if wait $$pid 2>/dev/null; then kill $$watchdog >/dev/null 2>&1; else \
		echo "Docker doesn't seem to be running (no response after 5s) -- start Docker Desktop and try again." >&2; \
		exit 1; \
	fi
	docker run --rm --platform linux/arm64 \
		-v "$(CURDIR)":/src -w /src \
		golang:1.26-alpine \
		sh -c "apk add --no-cache gcc musl-dev >/dev/null && \
			CGO_ENABLED=1 GOOS=linux GOARCH=arm64 go build \
			-ldflags '-linkmode external -extldflags \"-static\"' \
			-o bin/seahorse-linux-arm64 ./cmd/seahorse"

# Packages the linux binaries above as .deb/.rpm using nfpm (a pure-Go
# packager, so this needs neither dpkg-deb nor rpmbuild installed locally, and
# runs the same way on macOS or Linux; fetched on demand via `go run`, not
# vendored). nfpm's own env/template substitution turned out to be unreliable
# across versions, so packaging/nfpm.yaml.tmpl is instead resolved with a
# plain `sed` into a concrete per-arch config before nfpm ever sees it.
# Debian and RPM spell architectures differently for the same thing (deb:
# amd64/arm64, rpm: x86_64/aarch64), hence the differing PKG_ARCH/filename
# per pair below.
package-deb-amd64: build-linux-amd64
	@mkdir -p $(DIST)
	sed -e "s/__PKG_ARCH__/amd64/" -e "s/__PKG_VERSION__/$(PKG_VERSION)/" \
		-e "s|__PKG_BINARY__|bin/seahorse-linux-amd64|" \
		packaging/nfpm.yaml.tmpl > $(DIST)/nfpm-deb-amd64.yaml
	$(NFPM) package --config $(DIST)/nfpm-deb-amd64.yaml --packager deb \
		--target $(DIST)/seahorse_$(PKG_VERSION)_amd64.deb

package-deb-arm64: build-linux-arm64
	@mkdir -p $(DIST)
	sed -e "s/__PKG_ARCH__/arm64/" -e "s/__PKG_VERSION__/$(PKG_VERSION)/" \
		-e "s|__PKG_BINARY__|bin/seahorse-linux-arm64|" \
		packaging/nfpm.yaml.tmpl > $(DIST)/nfpm-deb-arm64.yaml
	$(NFPM) package --config $(DIST)/nfpm-deb-arm64.yaml --packager deb \
		--target $(DIST)/seahorse_$(PKG_VERSION)_arm64.deb

package-rpm-amd64: build-linux-amd64
	@mkdir -p $(DIST)
	sed -e "s/__PKG_ARCH__/x86_64/" -e "s/__PKG_VERSION__/$(PKG_VERSION)/" \
		-e "s|__PKG_BINARY__|bin/seahorse-linux-amd64|" \
		packaging/nfpm.yaml.tmpl > $(DIST)/nfpm-rpm-amd64.yaml
	$(NFPM) package --config $(DIST)/nfpm-rpm-amd64.yaml --packager rpm \
		--target $(DIST)/seahorse-$(PKG_VERSION).x86_64.rpm

package-rpm-arm64: build-linux-arm64
	@mkdir -p $(DIST)
	sed -e "s/__PKG_ARCH__/aarch64/" -e "s/__PKG_VERSION__/$(PKG_VERSION)/" \
		-e "s|__PKG_BINARY__|bin/seahorse-linux-arm64|" \
		packaging/nfpm.yaml.tmpl > $(DIST)/nfpm-rpm-arm64.yaml
	$(NFPM) package --config $(DIST)/nfpm-rpm-arm64.yaml --packager rpm \
		--target $(DIST)/seahorse-$(PKG_VERSION).aarch64.rpm

# Builds every .deb/.rpm combination (amd64 + arm64 today).
packages: package-deb-amd64 package-deb-arm64 package-rpm-amd64 package-rpm-arm64

build-all: packages
all: build-all

# Runs the Go backend on :8585 and the Vite dev server (with hot reload,
# proxying /api to the backend) side by side. Stop both with Ctrl-C.
dev:
	@echo "Starting backend (:8585) and frontend dev server..."
	@trap 'kill 0' EXIT; \
	go run ./cmd/seahorse & \
	npm --prefix frontend run dev & \
	wait

clean:
	rm -rf bin $(DIST) frontend/node_modules internal/web/dist
	git checkout -- internal/web/dist 2>/dev/null || true
