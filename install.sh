#!/bin/sh
# install.sh - Install walkline on Mac/Linux/WSL/Git-Bash
# This script does NOT run on native Windows (cmd.exe/PowerShell). Use install.ps1 instead.

set -e

OWNER="${OWNER:-Pantho-Haque}"
REPO="${REPO:-walkline}"
VERSION="${WALKLINE_VERSION:-}"

# Detect OS
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$OS" in
  darwin*) OS="darwin" ;;
  linux*)  OS="linux" ;;
  mingw*|msys*) OS="windows" ;;
  *)
    echo "Error: Unsupported OS: $OS" >&2
    echo "On native Windows, use install.ps1 instead." >&2
    exit 1
    ;;
esac

# Detect arch
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *)
    echo "Error: Unsupported architecture: $ARCH" >&2
    exit 1
    ;;
esac

# Check for valid platform combination
case "${OS}_${ARCH}" in
  darwin_amd64|darwin_arm64|linux_amd64|linux_arm64|windows_amd64) ;;
  *)
    echo "Error: No release available for ${OS}/${ARCH}" >&2
    exit 1
    ;;
esac

# Resolve version
if [ -z "$VERSION" ]; then
  echo "Resolving latest release..."
  VERSION="$(curl -fsSL "https://api.github.com/repos/${OWNER}/${REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')"
  if [ -z "$VERSION" ]; then
    echo "Error: Could not resolve latest release" >&2
    exit 1
  fi
fi

echo "Installing walkline ${VERSION} for ${OS}/${ARCH}"

# Set up file extensions
ext=".tar.gz"
archive_name="walkline_${VERSION#v}_${OS}_${ARCH}.tar.gz"
if [ "$OS" = "windows" ]; then
  ext=".zip"
  archive_name="walkline_${VERSION#v}_${OS}_${ARCH}.zip"
fi
binary_name="walkline"
if [ "$OS" = "windows" ]; then
  binary_name="walkline.exe"
fi

# Create temp directory with cleanup trap
tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

# Download release
echo "Downloading ${archive_name}..."
cd "$tmpdir"
curl -fsSL -o "release${ext}" "https://github.com/${OWNER}/${REPO}/releases/download/${VERSION}/${archive_name}"
curl -fsSL -o checksums.txt "https://github.com/${OWNER}/${REPO}/releases/download/${VERSION}/checksums.txt"

# Verify checksum
echo "Verifying checksum..."
if command -v sha256sum >/dev/null 2>&1; then
  sha256sum --check checksums.txt --status
elif command -v shasum >/dev/null 2>&1; then
  shasum -a 256 --check checksums.txt --status
else
  echo "Error: Neither sha256sum nor shasum available for verification" >&2
  exit 1
fi

# Extract
echo "Extracting..."
tar xzf "release${ext}"
rm "release${ext}" checksums.txt

# Determine install location
if [ -w /usr/local/bin ]; then
  install_dir="/usr/local/bin"
else
  install_dir="${HOME}/.local/bin"
  mkdir -p "$install_dir"
fi

# Install binary
cp "$binary_name" "${install_dir}/"
chmod +x "${install_dir}/${binary_name}"

# Clean up (trap will also run this, but being explicit)
rm -rf "$tmpdir"

echo ""
echo "Installed: ${install_dir}/${binary_name}"

# Check PATH
case ":${PATH}:" in
  *":${install_dir}:"*) ;;
  *)
    echo ""
    echo "WARNING: ${install_dir} is not in your PATH."
    echo "Add the following to your shell rc file:"
    echo "  export PATH=\"${install_dir}:\$PATH\""
    ;;
esac

# Set up git hooks template
echo ""
echo "Setting up git hooks template..."
if "${install_dir}/${binary_name}" install; then
    echo ""
    echo "walkline is ready to use!"
    echo ""
    echo "Next steps:"
    echo "  1. To track existing repos: walkline scan <directory>"
    echo "  2. New repos are automatically instrumented going forward"
else
    echo ""
    echo "walkline binary installed, but 'walkline install' failed."
    echo "Run 'walkline install' manually to set up the git hooks template."
fi
