#!/usr/bin/env bash
# Argus Universal Installer for Linux and macOS
# Usage: curl -fsSL https://raw.githubusercontent.com/will2469/argus/main/install.sh | bash

set -e

REPO="will2469/argus"
GITHUB_URL="https://github.com/${REPO}"

# Colors for output
BOLD="\033[1m"
GREEN="\033[32m"
BLUE="\033[34m"
YELLOW="\033[33m"
RED="\033[31m"
NC="\033[0m"

info() {
    printf "${BLUE}==>${NC} ${BOLD}%s${NC}\n" "$1"
}

success() {
    printf "${GREEN}==>${NC} ${BOLD}%s${NC}\n" "$1"
}

warn() {
    printf "${YELLOW}warning:${NC} %s\n" "$1"
}

error() {
    printf "${RED}error:${NC} %s\n" "$1" >&2
    exit 1
}

# 1. Detect Operating System
OS="$(uname -s)"
case "${OS}" in
    Linux*)   TARGET_OS="linux" ;;
    Darwin*)  TARGET_OS="darwin" ;;
    *)        error "Unsupported operating system: ${OS}. Argus supports Linux and macOS via this script." ;;
esac

# 2. Detect Architecture
ARCH="$(uname -m)"
case "${ARCH}" in
    x86_64|amd64)   TARGET_ARCH="amd64" ;;
    arm64|aarch64)  TARGET_ARCH="arm64" ;;
    *)              error "Unsupported architecture: ${ARCH}. Argus supports amd64 (x86_64) and arm64 (Apple Silicon / ARM64)." ;;
esac

info "Detected platform: ${TARGET_OS}/${TARGET_ARCH}"

# 3. Determine Latest Release Tag
info "Checking for latest release..."
TAG=""

if command -v curl >/dev/null 2>&1; then
    TAG="$(curl -sSfL "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null | grep '"tag_name":' | head -n1 | sed -E 's/.*"tag_name":[[:space:]]*"([^"]+)".*/\1/')"
fi

if [ -z "${TAG}" ] && command -v curl >/dev/null 2>&1; then
    LATEST_URL="$(curl -sSfLI -o /dev/null -w '%{url_effective}' "${GITHUB_URL}/releases/latest" 2>/dev/null || true)"
    TAG="${LATEST_URL##*/}"
fi

if [ -z "${TAG}" ] || [ "${TAG}" = "latest" ]; then
    TAG="v1.0.0"
    warn "Could not resolve latest release tag via GitHub API, falling back to ${TAG}"
fi

info "Selected version: ${TAG}"

# 4. Prepare Download URL
ARCHIVE_NAME="argus_${TAG}_${TARGET_OS}_${TARGET_ARCH}.tar.gz"
DOWNLOAD_URL="${GITHUB_URL}/releases/download/${TAG}/${ARCHIVE_NAME}"

# 5. Create Temporary Directory
TMP_DIR="$(mktemp -d)"
cleanup() {
    rm -rf "${TMP_DIR}"
}
trap cleanup EXIT INT TERM

info "Downloading ${DOWNLOAD_URL}..."
if command -v curl >/dev/null 2>&1; then
    curl -fsSL "${DOWNLOAD_URL}" -o "${TMP_DIR}/${ARCHIVE_NAME}"
elif command -v wget >/dev/null 2>&1; then
    wget -q "${DOWNLOAD_URL}" -O "${TMP_DIR}/${ARCHIVE_NAME}"
else
    error "Neither curl nor wget is installed. Please install one of them to proceed."
fi

# 6. Extract Binary
info "Extracting binary..."
tar -xzf "${TMP_DIR}/${ARCHIVE_NAME}" -C "${TMP_DIR}"

if [ ! -f "${TMP_DIR}/argus" ]; then
    error "Failed to locate 'argus' binary inside extracted archive."
fi

chmod +x "${TMP_DIR}/argus"

# 7. Select Destination Directory
INSTALL_DIR="/usr/local/bin"
USE_SUDO=false

if [ -w "${INSTALL_DIR}" ]; then
    DEST="${INSTALL_DIR}/argus"
elif command -v sudo >/dev/null 2>&1 && [ -n "${SUDO_USER:-}" ] || (command -v sudo >/dev/null 2>&1 && sudo -n true 2>/dev/null); then
    USE_SUDO=true
    DEST="${INSTALL_DIR}/argus"
else
    INSTALL_DIR="${HOME}/.local/bin"
    mkdir -p "${INSTALL_DIR}"
    DEST="${INSTALL_DIR}/argus"
fi

info "Installing to ${DEST}..."
if [ "${USE_SUDO}" = true ]; then
    sudo cp "${TMP_DIR}/argus" "${DEST}"
    sudo chmod 755 "${DEST}"
else
    cp "${TMP_DIR}/argus" "${DEST}"
    chmod 755 "${DEST}"
fi

# macOS quarantine removal
if [ "${TARGET_OS}" = "darwin" ]; then
    xattr -d com.apple.quarantine "${DEST}" 2>/dev/null || true
fi

# 8. Check and Configure PATH if installed to user home
PATH_NEEDS_UPDATE=false
case ":${PATH}:" in
    *":${INSTALL_DIR}:"*) ;;
    *) PATH_NEEDS_UPDATE=true ;;
esac

if [ "${PATH_NEEDS_UPDATE}" = true ]; then
    warn "${INSTALL_DIR} is not in your current PATH."
    SHELL_PROFILE=""
    
    if [ -n "${ZSH_VERSION:-}" ] || [ -f "${HOME}/.zshrc" ]; then
        SHELL_PROFILE="${HOME}/.zshrc"
    elif [ -f "${HOME}/.bashrc" ]; then
        SHELL_PROFILE="${HOME}/.bashrc"
    elif [ -f "${HOME}/.profile" ]; then
        SHELL_PROFILE="${HOME}/.profile"
    fi

    if [ -n "${SHELL_PROFILE}" ]; then
        EXPORT_CMD="export PATH=\"${INSTALL_DIR}:\$PATH\""
        if ! grep -qs "${INSTALL_DIR}" "${SHELL_PROFILE}"; then
            echo "" >> "${SHELL_PROFILE}"
            echo "# Argus Static Analyzer" >> "${SHELL_PROFILE}"
            echo "${EXPORT_CMD}" >> "${SHELL_PROFILE}"
            info "Added ${INSTALL_DIR} to ${SHELL_PROFILE}."
        fi
        export PATH="${INSTALL_DIR}:${PATH}"
    fi
fi

# 9. Verify Installation
success "Argus ${TAG} successfully installed to ${DEST}!"
printf "\n"
if command -v argus >/dev/null 2>&1; then
    argus --version || true
else
    "${DEST}" --version || true
fi

printf "\n${BOLD}Quick Start:${NC}\n"
printf "  argus --help\n"
printf "  argus --dirs=. --migrations=migrations\n\n"
