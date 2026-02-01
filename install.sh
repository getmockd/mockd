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
        curl -sSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/'
    elif command -v wget >/dev/null 2>&1; then
        wget -qO- "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/'
    else
        error "Neither curl nor wget found. Please install one of them."
    fi
}

# Download file
download() {
    url="$1"
    dest="$2"
    if command -v curl >/dev/null 2>&1; then
        curl -sSL -o "$dest" "$url"
    elif command -v wget >/dev/null 2>&1; then
        wget -qO "$dest" "$url"
    fi
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
    URL="https://github.com/${REPO}/releases/download/${VERSION}/${FILENAME}"
    CHECKSUM_URL="${URL}.sha256"

    # Download binary
    TMPDIR=$(mktemp -d)
    trap 'rm -rf "$TMPDIR"' EXIT

    info "Downloading ${URL}..."
    download "$URL" "${TMPDIR}/${BINARY_NAME}"

    # Verify checksum if sha256sum is available
    if command -v sha256sum >/dev/null 2>&1; then
        info "Verifying checksum..."
        download "$CHECKSUM_URL" "${TMPDIR}/checksum"
        EXPECTED=$(awk '{print $1}' "${TMPDIR}/checksum")
        ACTUAL=$(sha256sum "${TMPDIR}/${BINARY_NAME}" | awk '{print $1}')
        if [ "$EXPECTED" != "$ACTUAL" ]; then
            error "Checksum verification failed!\n  Expected: ${EXPECTED}\n  Got:      ${ACTUAL}"
        fi
        success "Checksum verified"
    elif command -v shasum >/dev/null 2>&1; then
        info "Verifying checksum..."
        download "$CHECKSUM_URL" "${TMPDIR}/checksum"
        EXPECTED=$(awk '{print $1}' "${TMPDIR}/checksum")
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
            --max-time 3 2>/dev/null || true) &
    fi

    # Verify installation
    if command -v mockd >/dev/null 2>&1; then
        INSTALLED_VERSION=$(mockd version 2>&1 | head -1)
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
