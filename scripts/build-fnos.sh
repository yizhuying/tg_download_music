#!/bin/bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
PKG_DIR="${ROOT}/fnnas.telegram-music"
APP_SERVER_DIR="${PKG_DIR}/app/server"

# Locate Go toolchain
GO_BIN="/opt/development/language/go/1.25.10/bin/go"
if [ ! -f "${GO_BIN}" ]; then
	GO_BIN="$(command -v go 2>/dev/null || true)"
fi
if [ -z "${GO_BIN}" ]; then
	echo "Error: go not found"
	exit 1
fi

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

echo "[1/3] Building frontend..."
cd "${ROOT}/web"
npm install
npx vite build

echo "[2/3] Building Go binary (linux/amd64)..."
cd "${ROOT}"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 ${GO_BIN} build -o telegram-music-server ./cmd/server/

echo "[3/3] Copying binary to package and building .fpk..."
mkdir -p "${APP_SERVER_DIR}"
cp "${ROOT}/telegram-music-server" "${APP_SERVER_DIR}/"
cd "${PKG_DIR}"
${FNPACK} build
mv -f "${PKG_DIR}/telegram-music.fpk" "${ROOT}/telegram-music.fpk"

echo ""
echo "=== Build complete ==="
echo "Package: ${ROOT}/telegram-music.fpk"
ls -lh "${ROOT}/telegram-music.fpk"
