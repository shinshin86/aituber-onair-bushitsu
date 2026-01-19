#!/usr/bin/env bash
set -euo pipefail

OWNER="shinshin86"
REPO="aituber-onair-bushitsu"
BIN_NAME="bushitsu"

usage() {
  cat <<'USAGE'
Usage:
  install.sh [vX.Y.Z]
  install.sh [-h|--help]

Examples:
  install.sh
  install.sh v1.2.3

Environment:
  BIN_DIR   Install directory (default: ~/.local/bin)
USAGE
}

err() {
  echo "Error: $*" >&2
  exit 1
}

info() {
  echo "==> $*"
}

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || err "required command not found: $1"
}

if [[ $# -gt 1 ]]; then
  usage
  err "too many arguments"
fi

if [[ $# -eq 1 ]]; then
  case "$1" in
    -h|--help)
      usage
      exit 0
      ;;
    v[0-9]*.[0-9]*.[0-9]*)
      TAG="$1"
      ;;
    *)
      usage
      err "invalid version: $1 (expected vX.Y.Z)"
      ;;
  esac
else
  TAG="latest"
fi

need_cmd uname
need_cmd curl
need_cmd tar
need_cmd awk
need_cmd install
need_cmd mktemp
need_cmd date

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH_RAW="$(uname -m)"

if [[ "$OS" != "darwin" ]]; then
  err "unsupported OS: $OS (only darwin is supported)"
fi

case "$ARCH_RAW" in
  arm64|aarch64)
    ARCH="arm64"
    ;;
  x86_64|amd64)
    ARCH="amd64"
    ;;
  *)
    err "unsupported CPU architecture: $ARCH_RAW (only arm64/amd64 are supported)"
    ;;
 esac

if [[ "$TAG" == "latest" ]]; then
  info "resolving latest release tag via GitHub API..."
  API_URL="https://api.github.com/repos/${OWNER}/${REPO}/releases/latest"
  TAG="$(curl -fL --retry 3 --retry-delay 1 "$API_URL" \
    | awk -F'"' '/"tag_name"/ {print $4; exit}')"
  [[ -n "$TAG" ]] || err "failed to parse tag_name from GitHub API"
fi

ASSET="bushitsu_${TAG}_darwin_${ARCH}.tar.gz"
CHECKSUMS="checksums.txt"
BASE_URL="https://github.com/${OWNER}/${REPO}/releases/download/${TAG}"

info "target release: ${TAG}"
info "asset: ${ASSET}"

TMPDIR="$(mktemp -d)"
cleanup() {
  rm -rf "$TMPDIR"
}
trap cleanup EXIT

info "downloading checksums..."
curl -fL --retry 3 --retry-delay 1 \
  "${BASE_URL}/${CHECKSUMS}" -o "${TMPDIR}/${CHECKSUMS}" \
  || err "failed to download ${CHECKSUMS}"

EXPECTED_SHA="$(awk -v fn="$ASSET" '$2==fn {print $1; exit}' "${TMPDIR}/${CHECKSUMS}")"
[[ -n "$EXPECTED_SHA" ]] || err "checksum for ${ASSET} not found in ${CHECKSUMS}"

info "downloading asset..."
curl -fL --retry 3 --retry-delay 1 \
  "${BASE_URL}/${ASSET}" -o "${TMPDIR}/${ASSET}" \
  || err "failed to download ${ASSET}"

if command -v shasum >/dev/null 2>&1; then
  ACTUAL_SHA="$(shasum -a 256 "${TMPDIR}/${ASSET}" | awk '{print $1}')"
elif command -v sha256sum >/dev/null 2>&1; then
  ACTUAL_SHA="$(sha256sum "${TMPDIR}/${ASSET}" | awk '{print $1}')"
else
  err "neither shasum nor sha256sum is available"
fi

if [[ "$EXPECTED_SHA" != "$ACTUAL_SHA" ]]; then
  err "checksum mismatch for ${ASSET} (expected ${EXPECTED_SHA}, got ${ACTUAL_SHA})"
fi

info "extracting..."
tar -xzf "${TMPDIR}/${ASSET}" -C "${TMPDIR}" \
  || err "failed to extract ${ASSET}"

BIN_PATH="${TMPDIR}/${BIN_NAME}"
[[ -f "$BIN_PATH" ]] || err "binary not found in archive: ${BIN_NAME}"

BIN_DIR="${BIN_DIR:-$HOME/.local/bin}"
mkdir -p "$BIN_DIR" || err "failed to create BIN_DIR: ${BIN_DIR}"

DEST="${BIN_DIR}/${BIN_NAME}"
if [[ -f "$DEST" ]]; then
  TS="$(date +%Y%m%d%H%M%S)"
  BACKUP="${DEST}.${TS}.bak"
  info "existing binary found, backing up to ${BACKUP}"
  mv "$DEST" "$BACKUP" || err "failed to backup existing binary"
fi

info "installing to ${DEST}"
install -m 0755 "$BIN_PATH" "$DEST" || err "install failed"

echo "Installed ${BIN_NAME} ${TAG} to ${DEST}"
case ":$PATH:" in
  *":${BIN_DIR}:")
    ;;
  *)
    echo "Note: ${BIN_DIR} is not in your PATH."
    echo "Add this to your shell profile:"
    echo "  export PATH=\"${BIN_DIR}:\$PATH\""
    ;;
 esac
