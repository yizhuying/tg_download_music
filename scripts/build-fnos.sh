#!/bin/bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
PKG_DIR="${ROOT}/fnnas.tg-music"
package_name="TuneGram"

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
BUILD_TIME=$(date '+%Y-%m-%d_%H:%M:%S')
VERSION=$(cd "${ROOT}" && git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS="-X main.version=${VERSION} -X main.buildTime=${BUILD_TIME}"
cd "${ROOT}/server"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "${LDFLAGS}" -o "${ROOT}/tg-music-server" ./cmd/server/

# [3] Build fnOS package
# Package version: git tag when HEAD is tagged, otherwise 0.1.<commit count>
if TAG=$(cd "${ROOT}" && git describe --tags --exact-match 2>/dev/null); then
    APP_VERSION="${TAG#v}"
else
    APP_VERSION="0.1.$(cd "${ROOT}" && git rev-list --count HEAD)"
fi
echo "[3/3] Building .fpk (version ${APP_VERSION})..."

# Stage a copy so version injection and the binary never modify the source package
STAGE_DIR=$(mktemp -d)
trap 'rm -rf "${STAGE_DIR}"' EXIT
cp -R "${PKG_DIR}/." "${STAGE_DIR}/"
mkdir -p "${STAGE_DIR}/app/server"
cp "${ROOT}/tg-music-server" "${STAGE_DIR}/app/server/"
sed "s/^version[[:space:]]*=.*/version               = ${APP_VERSION}/" \
    "${PKG_DIR}/manifest" > "${STAGE_DIR}/manifest"
cd "${STAGE_DIR}"
"${FNPACK}" build
mv -f "${STAGE_DIR}/${package_name}.fpk" "${ROOT}/${package_name}.fpk"

echo ""
echo "=== Build complete ==="
echo "Package: ${ROOT}/${package_name}.fpk (version ${APP_VERSION})"
ls -lh "${ROOT}/${package_name}.fpk"
