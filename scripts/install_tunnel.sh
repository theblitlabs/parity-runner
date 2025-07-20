#!/bin/bash

set -e

echo "🔧 PLGenesis Runner - Tunnel Setup"
echo "================================="

# Check if bore is already installed
if command -v bore &> /dev/null; then
    echo "✅ bore is already installed"
    bore --version
    exit 0
fi

echo "📦 Installing bore CLI tool..."

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case $ARCH in
    x86_64) ARCH="x86_64" ;;
    arm64|aarch64) ARCH="aarch64" ;;
    armv7l) ARCH="armv7" ;;
    *) echo "❌ Unsupported architecture: $ARCH"; exit 1 ;;
esac

case $OS in
    darwin)
        echo "🍎 Detected macOS"
        if command -v brew &> /dev/null; then
            echo "🍺 Installing via Homebrew..."
            brew install bore-cli
        else
            echo "❌ Homebrew not found. Please install Homebrew first or install bore manually."
            echo "Visit: https://github.com/ekzhang/bore/releases"
            exit 1
        fi
        ;;
    linux)
        echo "🐧 Detected Linux"
        # Check if cargo is available (fastest option)
        if command -v cargo &> /dev/null; then
            echo "🦀 Installing via Cargo..."
            cargo install bore-cli
        else
            echo "📥 Downloading binary release..."
            LATEST=$(curl -s https://api.github.com/repos/ekzhang/bore/releases/latest | grep -o '"tag_name": "[^"]*' | grep -o '[^"]*$')
            if [ -z "$LATEST" ]; then
                echo "❌ Failed to get latest version"
                exit 1
            fi
            
            URL="https://github.com/ekzhang/bore/releases/download/$LATEST/bore-$LATEST-$ARCH-unknown-$OS-musl.tar.gz"
            TMP_DIR=$(mktemp -d)
            
            echo "⬇️  Downloading: $URL"
            curl -L "$URL" -o "$TMP_DIR/bore.tar.gz"
            
            cd "$TMP_DIR"
            tar -xzf bore.tar.gz
            
            # Install to /usr/local/bin (requires sudo)
            if [ -w "/usr/local/bin" ]; then
                mv bore /usr/local/bin/
            else
                echo "🔐 Installing to /usr/local/bin (requires sudo)..."
                sudo mv bore /usr/local/bin/
            fi
            
            rm -rf "$TMP_DIR"
        fi
        ;;
    *)
        echo "❌ Unsupported OS: $OS"
        echo "Please install bore manually from: https://github.com/ekzhang/bore/releases"
        exit 1
        ;;
esac

# Verify installation
if command -v bore &> /dev/null; then
    echo "✅ bore installed successfully!"
    bore --version
    echo ""
    echo "🚀 You can now enable tunneling in your .env file:"
    echo "   RUNNER_TUNNEL_ENABLED=true"
else
    echo "❌ Installation failed"
    exit 1
fi 