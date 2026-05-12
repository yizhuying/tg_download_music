.PHONY: build build-frontend build-server dev clean

build: build-frontend build-server

build-server:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOTOOLCHAIN=local go build -o telegram-music-server ./cmd/server/

build-frontend:
	cd web && npm install && npx vite build

dev:
	@echo "Terminal 1: go run ./cmd/server/ -port 8080"
	@echo "Terminal 2: cd web && npm run dev"

clean:
	rm -f telegram-music
	rm -rf cmd/server/dist
