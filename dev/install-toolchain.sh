#!/usr/bin/env bash
# dev/install-toolchain.sh
# Checks for Go and Rust; installs missing ones on macOS (arm64 / x86_64).
# Run: bash dev/install-toolchain.sh

set -euo pipefail

GO_VERSION="1.23.4"
ARCH=$(uname -m)   # arm64 or x86_64

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

ok()   { echo -e "${GREEN}[ok]${NC}  $*"; }
warn() { echo -e "${YELLOW}[--]${NC}  $*"; }
err()  { echo -e "${RED}[!!]${NC}  $*"; }

echo ""
echo "━━━  Antariksh dev toolchain check  ━━━"
echo ""

# ── 1. Go ─────────────────────────────────────────────────────────────────────

if command -v go &>/dev/null; then
    INSTALLED_GO=$(go version | awk '{print $3}' | sed 's/go//')
    ok "Go ${INSTALLED_GO} already installed at $(command -v go)"
else
    warn "Go not found — installing Go ${GO_VERSION}..."

    if [[ "$ARCH" == "arm64" ]]; then
        PKG="go${GO_VERSION}.darwin-arm64.pkg"
    else
        PKG="go${GO_VERSION}.darwin-amd64.pkg"
    fi

    URL="https://go.dev/dl/${PKG}"
    TMPFILE="/tmp/${PKG}"

    echo "  Downloading ${URL}..."
    curl -fSL "$URL" -o "$TMPFILE"

    echo "  Installing (sudo required)..."
    sudo installer -pkg "$TMPFILE" -target /
    rm -f "$TMPFILE"

    # Verify
    if command -v go &>/dev/null; then
        ok "Go $(go version | awk '{print $3}') installed at $(command -v go)"
    else
        err "Go install completed but 'go' not in PATH."
        echo "  Add to your shell profile:"
        echo "    export PATH=\$PATH:/usr/local/go/bin"
    fi
fi

echo ""

# ── 2. Rust (via rustup) ──────────────────────────────────────────────────────

if command -v rustc &>/dev/null; then
    INSTALLED_RUST=$(rustc --version | awk '{print $2}')
    ok "Rust ${INSTALLED_RUST} already installed at $(command -v rustc)"
    ok "cargo: $(cargo --version)"
else
    warn "Rust not found — installing via rustup..."

    curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | \
        sh -s -- -y --default-toolchain stable --profile default

    # Source the env so rustc is visible in this session
    # shellcheck source=/dev/null
    source "$HOME/.cargo/env"

    if command -v rustc &>/dev/null; then
        ok "Rust $(rustc --version | awk '{print $2}') installed"
        ok "cargo: $(cargo --version)"
    else
        err "rustup ran but 'rustc' still not in PATH."
        echo "  Restart your terminal or run: source \$HOME/.cargo/env"
    fi
fi

echo ""

# ── 3. Extras needed for this project ────────────────────────────────────────

echo "━━━  Checking extras  ━━━"
echo ""

# protoc (needed by fc-driver tonic/prost)
if command -v protoc &>/dev/null; then
    ok "protoc $(protoc --version)"
else
    warn "protoc not found — install with: brew install protobuf"
fi

# golangci-lint
if command -v golangci-lint &>/dev/null; then
    ok "golangci-lint $(golangci-lint --version | head -1)"
else
    warn "golangci-lint not found — install with:"
    echo "  brew install golangci-lint"
fi

# Docker (for dev stack)
if command -v docker &>/dev/null; then
    ok "docker $(docker --version)"
else
    warn "docker not found — install Docker Desktop from https://www.docker.com/products/docker-desktop/"
fi

# Nomad CLI (to submit jobs)
if command -v nomad &>/dev/null; then
    ok "nomad $(nomad --version | head -1)"
else
    warn "nomad CLI not found — install with: brew install nomad"
fi

echo ""
echo "━━━  Done. Summary  ━━━"
echo ""
go version    2>/dev/null && true
rustc --version 2>/dev/null && true
cargo --version 2>/dev/null && true
echo ""
