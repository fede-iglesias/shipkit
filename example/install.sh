#!/usr/bin/env bash
# install.sh - Parameterizable install script template for shipkit-enabled CLIs.
#
# Purpose: this file is the canonical template for generating per-CLI install scripts.
# It lives in the shipkit mono-repo as example/install.sh and is NOT directly usable
# as shipped. Before use, token substitution must be applied (see below).
#
# Token list (replaced by the release pipeline via sed before publishing to tools repo):
#   <app>                    - CLI lowercase name, e.g. myapp
#   <APP>                    - CLI uppercase name for env vars, e.g. MYAPP
#   <app-cert-identity-regex> - cosign certificate identity regexp, e.g.
#                               https://github\.com/your-org/myapp/.*
#
# Substitution example (run by release.yml before publishing to fede-iglesias/tools):
#   sed \
#     -e 's|<app>|myapp|g' \
#     -e 's|<APP>|MYAPP|g' \
#     -e 's|<app-cert-identity-regex>|https://github\\.com/your-org/myapp/.*|g' \
#     example/install.sh > dist/install/install.sh
#
# After substitution, the rendered script is published to:
#   https://raw.githubusercontent.com/fede-iglesias/tools/main/<app>/install.sh
#
# Usage (after rendering and publishing):
#   curl -fsSL https://raw.githubusercontent.com/fede-iglesias/tools/main/<app>/install.sh | bash
# Override install dir (note: the env var must precede 'bash', not 'curl'):
#   curl -fsSL .../install.sh | <APP>_INSTALL_DIR=/tmp/<app> bash
# Force overwrite without prompt:
#   curl -fsSL .../install.sh | <APP>_FORCE_REINSTALL=1 bash
#
# What this script does:
#   1. Detects OS and architecture.
#   2. Finds the latest <app>-v* release in fede-iglesias/tools.
#   3. Downloads the matching tar.gz asset.
#   4. Verifies the cosign keyless signature if cosign is available (warning, not error, if absent).
#   5. Extracts and installs the binary to INSTALL_DIR (default /usr/local/bin).
#   6. Removes macOS quarantine attribute if on darwin.
#   7. Prints confirmation with the installed version.
#   8. Reminds user to run '<app> install' to finish setup (config dirs, completions, etc.).
#
# What this script does NOT do:
#   - Touch PATH, shell rc files, data dirs, or completions. That is '<app> install'.
#   - Run '<app> install' automatically.
#
# Security notes:
#   - cosign keyless verify uses GitHub Actions OIDC identity.
#   - When cosign is absent a WARNING is printed (stderr) but install continues.
#   - No sudo is used for the xattr step. sudo may be used for install -m 755
#     if INSTALL_DIR is not writable by the current user.

set -euo pipefail

REPO="fede-iglesias/tools"
BIN_NAME="<app>"
INSTALL_DIR="${<APP>_INSTALL_DIR:-/usr/local/bin}"
FORCE_REINSTALL="${<APP>_FORCE_REINSTALL:-0}"

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64) ARCH=amd64 ;;
  arm64|aarch64) ARCH=arm64 ;;
  *) echo "unsupported arch: $ARCH" >&2; exit 1 ;;
esac
case "$OS" in
  darwin|linux) ;;
  *) echo "unsupported os: $OS" >&2; exit 1 ;;
esac

TAG=$(curl -fsSL "https://api.github.com/repos/$REPO/releases?per_page=30" \
  | grep -oE '"tag_name":[[:space:]]*"<app>-v[^"]+"' \
  | head -1 \
  | sed 's/.*"<app>-v\([^"]*\)".*/\1/')

if [ -z "$TAG" ]; then
  echo "no <app> release found in $REPO" >&2
  exit 1
fi

# Re-install detection: prompt if binary already exists and TTY is available.
if [ -e "$INSTALL_DIR/$BIN_NAME" ] && [ "$FORCE_REINSTALL" != "1" ]; then
  if [ -t 0 ]; then
    printf "%s already exists at %s. Overwrite? [y/N] " "$BIN_NAME" "$INSTALL_DIR/$BIN_NAME" >&2
    read -r reply
    case "$reply" in
      y|Y|yes|YES) ;;
      *) echo "aborted" >&2; exit 0 ;;
    esac
  else
    echo "note: $INSTALL_DIR/$BIN_NAME exists, overwriting (pipe from curl - non-interactive)" >&2
  fi
fi

echo "installing <app> v$TAG for $OS/$ARCH..." >&2

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

ASSET="<app>_${TAG}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/$REPO/releases/download/<app>-v$TAG/$ASSET"

curl -fsSL -o "$TMP/$ASSET" "$URL"

if command -v cosign >/dev/null 2>&1; then
  echo "verifying cosign signature..." >&2
  curl -fsSL -o "$TMP/$ASSET.bundle" "$URL.bundle"
  cosign verify-blob \
    --certificate-identity-regexp '<app-cert-identity-regex>' \
    --certificate-oidc-issuer https://token.actions.githubusercontent.com \
    --bundle "$TMP/$ASSET.bundle" \
    "$TMP/$ASSET" >/dev/null
  echo "signature verified" >&2
else
  echo "WARNING: cosign not present, skipping signature verification" >&2
  echo "  to verify manually: brew install cosign, then re-run" >&2
fi

tar -xzf "$TMP/$ASSET" -C "$TMP"

if [ -w "$INSTALL_DIR" ]; then
  install -m 755 "$TMP/$BIN_NAME" "$INSTALL_DIR/$BIN_NAME"
else
  sudo install -m 755 "$TMP/$BIN_NAME" "$INSTALL_DIR/$BIN_NAME"
fi

if [ "$OS" = "darwin" ]; then
  xattr -d com.apple.quarantine "$INSTALL_DIR/$BIN_NAME" 2>/dev/null || true
fi

INSTALLED_VERSION=$("$INSTALL_DIR/$BIN_NAME" --version 2>&1 \
  || echo "(version check skipped, binary may not be on PATH yet)")
echo "<app> installed: $INSTALLED_VERSION" >&2
echo >&2
echo "next step: run '<app> install' to complete setup (config dirs, completions, autostart)" >&2
echo "  override install dir: curl ... | <APP>_INSTALL_DIR=/path bash  (env before 'bash', not before 'curl')" >&2
