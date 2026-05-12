#!/bin/bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
PKG_DIR="${ROOT}/fnnas.telegram-music"
APP_SERVER_DIR="${PKG_DIR}/app/server"
FNPACK="fnpack"

echo "=== Building Telegram Music for fnOS ==="

echo "[1/3] Building frontend..."
cd "${ROOT}/web"
npm install
npx vite build

echo "[2/3] Building Go binary (linux/amd64)..."
cd "${ROOT}"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOTOOLCHAIN=local go build -o telegram-music-server ./cmd/server/

echo "[3/3] Copying binary to package and building .fpk..."
mkdir -p "${APP_SERVER_DIR}"
cp "${ROOT}/telegram-music-server" "${APP_SERVER_DIR}/"
cd "${PKG_DIR}"
${FNPACK} build
mv -f "${PKG_DIR}/fnnas.telegram-music.fpk" "${ROOT}/fnnas.telegram-music.fpk"

echo ""
echo "=== Build complete ==="
echo "Package: ${ROOT}/fnnas.telegram-music.fpk"
ls -lh "${ROOT}/fnnas.telegram-music.fpk"
