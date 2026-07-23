BINARY := bin/seahorse
LINUX_BINARY := bin/seahorse-linux-amd64

.PHONY: frontend build build-linux-amd64 dev clean

frontend:
	npm --prefix frontend install
	npm --prefix frontend run build

build: frontend
	go build -o $(BINARY) ./cmd/seahorse

# Cross-compiles a fully static linux/amd64 binary from macOS (or any host),
# for deploying to a Linux server. Runs the actual build inside a Docker
# container instead of just setting GOOS/GOARCH, for two reasons: (1) this
# project's sqlite driver needs cgo, which silently no-ops without a matching
# C toolchain for the target, producing a binary that builds fine but panics
# at runtime; (2) building against musl (Alpine) rather than the host's or a
# Debian container's glibc avoids depending on a specific glibc version, so
# the result runs on any x86_64 Linux distro/version, not just the one that
# happens to match the builder image. Requires Docker Desktop running.
build-linux-amd64: frontend
	docker run --rm --platform linux/amd64 \
		-v "$(CURDIR)":/src -w /src \
		golang:1.26-alpine \
		sh -c "apk add --no-cache gcc musl-dev >/dev/null && \
			CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build \
			-ldflags '-linkmode external -extldflags \"-static\"' \
			-o $(LINUX_BINARY) ./cmd/seahorse"

# Runs the Go backend on :8585 and the Vite dev server (with hot reload,
# proxying /api to the backend) side by side. Stop both with Ctrl-C.
dev:
	@echo "Starting backend (:8585) and frontend dev server..."
	@trap 'kill 0' EXIT; \
	go run ./cmd/seahorse & \
	npm --prefix frontend run dev & \
	wait

clean:
	rm -rf $(BINARY) frontend/node_modules internal/web/dist
	git checkout -- internal/web/dist 2>/dev/null || true
