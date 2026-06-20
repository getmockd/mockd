#!/bin/sh
# mockd installer script
#
# Usage:
#   curl -sSL https://get.mockd.io | sh                    # latest stable
#   curl -sSL https://get.mockd.io | VERSION=v0.2.1 sh     # specific version
#   curl -sSL https://get.mockd.io | INSTALL_DIR=./bin sh   # custom location
#
# Environment variables:
#   VERSION           - Version to install (default: latest release)
#   INSTALL_DIR       - Install directory (default: /usr/local/bin)
#   MOCKD_NO_TELEMETRY - Set to 1 to disable anonymous install analytics

set -e

REPO="getmockd/mockd"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
BINARY_NAME="mockd"
TELEMETRY_URL="https://api.mockd.io/api/telemetry/install"

# Base URLs. Overridable for testing; default to GitHub.
GITHUB_DOWNLOAD_BASE="${MOCKD_DOWNLOAD_BASE:-https://github.com/${REPO}/releases/download}"
GITHUB_API_BASE="${MOCKD_API_BASE:-https://api.github.com/repos/${REPO}}"

# Colors (if terminal supports it)
if [ -t 1 ]; then
    RED='\033[0;31m'
    GREEN='\033[0;32m'
    YELLOW='\033[0;33m'
    CYAN='\033[0;36m'
    NC='\033[0m'
else
    RED=''
    GREEN=''
    YELLOW=''
    CYAN=''
    NC=''
fi

info() { printf "${CYAN}==>${NC} %s\n" "$1"; }
success() { printf "${GREEN}==>${NC} %s\n" "$1"; }
warn() { printf "${YELLOW}warning:${NC} %s\n" "$1"; }
error() { printf "${RED}error:${NC} %s\n" "$1" >&2; exit 1; }

# Detect OS
detect_os() {
    case "$(uname -s)" in
        Linux*)  echo "linux" ;;
        Darwin*) echo "darwin" ;;
        MINGW*|MSYS*|CYGWIN*) echo "windows" ;;
        *) error "Unsupported operating system: $(uname -s)" ;;
    esac
}

# Detect architecture
detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64)  echo "amd64" ;;
        aarch64|arm64) echo "arm64" ;;
        *) error "Unsupported architecture: $(uname -m)" ;;
    esac
}

# Get latest version from GitHub
get_latest_version() {
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "${GITHUB_API_BASE}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/'
    elif command -v wget >/dev/null 2>&1; then
        wget -qO- "${GITHUB_API_BASE}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/'
    else
        error "Neither curl nor wget found. Please install one of them."
    fi
}

# Download a file to a destination. Returns non-zero (and leaves no body behind)
# on HTTP errors such as 404/5xx. Without --fail, curl writes the server's error
# page (e.g. "Not Found") to the destination and still exits 0, which then gets
# mis-parsed downstream as a checksum/binary — see
# https://github.com/getmockd/mockd/issues/34
download() {
    url="$1"
    dest="$2"
    if command -v curl >/dev/null 2>&1; then
        # --fail: exit non-zero on HTTP >= 400 without emitting the error body.
        curl -fsSL -o "$dest" "$url"
    elif command -v wget >/dev/null 2>&1; then
        # wget exits non-zero on 404, but -O still creates/truncates the file;
        # remove the stale destination on failure so callers never see a body.
        wget -qO "$dest" "$url" || { rm -f "$dest"; return 1; }
    else
        return 1
    fi
}

# A valid sha256 hex digest is exactly 64 lowercase hex characters.
is_sha256() {
    case "$1" in
        "" | *[!0-9a-f]*) return 1 ;;
        *) [ "${#1}" -eq 64 ] ;;
    esac
}

# Download and parse the expected checksum for the binary. Fails loudly with an
# actionable message rather than letting a missing/garbage checksum slip through
# and surface as a confusing "Expected: Not" comparison failure.
fetch_expected_checksum() {
    if ! download "$CHECKSUM_URL" "${TMPDIR}/checksum"; then
        error "Could not download checksum from ${CHECKSUM_URL}\n  The release asset may be missing or still publishing. Try again shortly, or pin a known-good version with VERSION=<tag>."
    fi
    expected=$(awk '{print $1}' "${TMPDIR}/checksum")
    if ! is_sha256 "$expected"; then
        error "Downloaded checksum is not a valid sha256 digest (got: '${expected}')\n  The release asset may be corrupt or missing. Try again shortly, or pin a known-good version with VERSION=<tag>."
    fi
    printf '%s' "$expected"
}

main() {
    info "Installing mockd..."

    OS=$(detect_os)
    ARCH=$(detect_arch)
    info "Detected: ${OS}/${ARCH}"

    # Determine version (user-specified or latest)
    if [ -n "${VERSION:-}" ]; then
        # Ensure it has the v prefix
        case "$VERSION" in
            v*) ;;
            *)  VERSION="v${VERSION}" ;;
        esac
        info "Requested version: ${VERSION}"
    else
        VERSION=$(get_latest_version)
        if [ -z "$VERSION" ]; then
            error "Failed to determine latest version. Check your internet connection."
        fi
        info "Latest version: ${VERSION}"
    fi

    # Build download URL
    FILENAME="${BINARY_NAME}-${OS}-${ARCH}"
    if [ "$OS" = "windows" ]; then
        FILENAME="${FILENAME}.exe"
    fi
    URL="${GITHUB_DOWNLOAD_BASE}/${VERSION}/${FILENAME}"
    CHECKSUM_URL="${URL}.sha256"

    # Download binary
    TMPDIR=$(mktemp -d)
    trap 'rm -rf "$TMPDIR"' EXIT

    info "Downloading ${URL}..."
    if ! download "$URL" "${TMPDIR}/${BINARY_NAME}"; then
        error "Could not download ${URL}\n  The release asset may be missing or still publishing. Check that ${VERSION} exists for ${OS}/${ARCH}, try again shortly, or pin a version with VERSION=<tag>."
    fi

    # Verify checksum if a sha256 tool is available. A missing or malformed
    # checksum is treated as a hard error (not silently skipped), since it most
    # likely means the binary download itself is bad.
    if command -v sha256sum >/dev/null 2>&1; then
        info "Verifying checksum..."
        EXPECTED=$(fetch_expected_checksum)
        ACTUAL=$(sha256sum "${TMPDIR}/${BINARY_NAME}" | awk '{print $1}')
        if [ "$EXPECTED" != "$ACTUAL" ]; then
            error "Checksum verification failed!\n  Expected: ${EXPECTED}\n  Got:      ${ACTUAL}"
        fi
        success "Checksum verified"
    elif command -v shasum >/dev/null 2>&1; then
        info "Verifying checksum..."
        EXPECTED=$(fetch_expected_checksum)
        ACTUAL=$(shasum -a 256 "${TMPDIR}/${BINARY_NAME}" | awk '{print $1}')
        if [ "$EXPECTED" != "$ACTUAL" ]; then
            error "Checksum verification failed!\n  Expected: ${EXPECTED}\n  Got:      ${ACTUAL}"
        fi
        success "Checksum verified"
    else
        warn "sha256sum not found, skipping checksum verification"
    fi

    # Install
    chmod +x "${TMPDIR}/${BINARY_NAME}"

    # Create install dir if needed
    if [ ! -d "$INSTALL_DIR" ]; then
        mkdir -p "$INSTALL_DIR" 2>/dev/null || {
            info "Creating ${INSTALL_DIR} (requires sudo)..."
            sudo mkdir -p "$INSTALL_DIR"
        }
    fi

    if [ -w "$INSTALL_DIR" ]; then
        mv "${TMPDIR}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
    else
        info "Installing to ${INSTALL_DIR} (requires sudo)..."
        sudo mv "${TMPDIR}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
    fi

    # Anonymous install telemetry (disable with MOCKD_NO_TELEMETRY=1)
    if [ "${MOCKD_NO_TELEMETRY:-0}" != "1" ]; then
        (curl -sSL -X POST "$TELEMETRY_URL" \
            -H "Content-Type: application/json" \
            -d "{\"os\":\"${OS}\",\"arch\":\"${ARCH}\",\"version\":\"${VERSION}\",\"method\":\"script\"}" \
            --max-time 3 >/dev/null 2>&1 || true) &
    fi

    # Verify installation
    if [ -x "${INSTALL_DIR}/${BINARY_NAME}" ]; then
        INSTALLED_VERSION=$("${INSTALL_DIR}/${BINARY_NAME}" version 2>&1 | head -1)
        success "mockd installed successfully!"
        printf "  %s\n" "$INSTALLED_VERSION"
        printf "\n"
        printf "  Get started:\n"
        printf "    ${CYAN}mockd start${NC}                          # Start the server\n"
        printf "    ${CYAN}mockd add --path /api/hello${NC}          # Add a mock\n"
        printf "    ${CYAN}curl localhost:4280/api/hello${NC}         # Test it\n"
        printf "\n"
        printf "  Docs: ${CYAN}https://mockd.io/quickstart${NC}\n"
    else
        warn "mockd was installed to ${INSTALL_DIR}/${BINARY_NAME} but is not in your PATH."
        printf "  Add ${INSTALL_DIR} to your PATH, or run:\n"
        printf "    ${INSTALL_DIR}/${BINARY_NAME} version\n"
    fi
}

main
