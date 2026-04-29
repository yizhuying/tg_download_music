.PHONY: build dev clean

build:
	cd web && npm install && npx vite build
	go build -o telegram-music ./cmd/server/

dev:
	@echo "Start Go server: go run ./cmd/server/"
	@echo "Then in another terminal: cd web && npm run dev"

clean:
	rm -f telegram-music
	rm -rf cmd/server/dist
