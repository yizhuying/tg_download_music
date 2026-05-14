VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS := -X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME)

.PHONY: build build-frontend build-server dev clean fnos

build: build-frontend build-server

build-server:
	cd server && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOTOOLCHAIN=local go build -ldflags "$(LDFLAGS)" -o ../../tg-music-server ./cmd/server/

build-frontend:
	cd frontend && npm install && npx vite build

dev:
	@echo "Terminal 1: cd server && go run ./cmd/server/ -port 45116"
	@echo "Terminal 2: cd frontend && npm run dev"

clean:
	rm -f tg-music
	rm -f tg-music-server
	rm -rf server/cmd/server/dist

fnos:
	bash scripts/build-fnos.sh
