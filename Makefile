BINARY := bin/seahorse

.PHONY: frontend build dev clean

frontend:
	npm --prefix frontend install
	npm --prefix frontend run build

build: frontend
	go build -o $(BINARY) ./cmd/seahorse

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
