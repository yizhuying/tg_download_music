#!/bin/bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
PKG_DIR="${ROOT}/fnnas.tg-music"
APP_SERVER_DIR="${PKG_DIR}/app/server"

# Locate fnpack
FNPACK="${ROOT}/fnpack-1.2.1-darwin-arm64"
if [ ! -f "${FNPACK}" ]; then
    FNPACK="$(command -v fnpack 2>/dev/null || true)"
fi
if [ -z "${FNPACK}" ] || [ ! -f "${FNPACK}" ]; then
    echo "Error: fnpack not found. Place fnpack-1.2.1-darwin-arm64 in project root or add to PATH."
    exit 1
fi

echo "=== Building Telegram Music for fnOS ==="

# [1] Build frontend
echo "[1/3] Building frontend..."
cd "${ROOT}/frontend"
npm install
npx vite build

# [2] Build Go binary
echo "[2/3] Building Go binary (linux/amd64)..."
cd "${ROOT}/server"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o "${ROOT}/tg-music-server" ./cmd/server/

# [3] Build fnOS package
echo "[3/3] Copying binary to package and building .fpk..."
mkdir -p "${APP_SERVER_DIR}"
cp "${ROOT}/tg-music-server" "${APP_SERVER_DIR}/"
cd "${PKG_DIR}"
${FNPACK} build
mv -f "${PKG_DIR}/tg-music.fpk" "${ROOT}/tg-music.fpk"

echo ""
echo "=== Build complete ==="
echo "Package: ${ROOT}/tg-music.fpk"
ls -lh "${ROOT}/tg-music.fpk"
