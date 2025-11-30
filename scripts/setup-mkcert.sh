#!/bin/bash

# Mkcert setup helper for Mnemosyne
# This script generates trusted certificates for local network use

set -e

echo "🔐 Mnemosyne Certificate Setup"
echo "=============================="
echo ""

# Check if mkcert is installed
if ! command -v mkcert &> /dev/null; then
    echo "❌ mkcert is not installed."
    echo ""
    echo "Install it first:"
    echo "  macOS:   brew install mkcert"
    echo "  Windows: choco install mkcert"
    echo "  Linux:   apt install mkcert (or your package manager)"
    echo ""
    exit 1
fi

echo "✓ mkcert is installed"
echo ""

# Install the local CA if not already done
echo "📜 Installing local CA (may require sudo)..."
mkcert -install
echo ""

# Get local IPs
echo "🌐 Detecting local IP addresses..."
LOCAL_IPS=""

if [[ "$OSTYPE" == "darwin"* ]]; then
    # macOS
    LOCAL_IPS=$(ifconfig | grep 'inet ' | grep -v '127.0.0.1' | awk '{print $2}' | tr '\n' ' ')
elif [[ "$OSTYPE" == "linux-gnu"* ]]; then
    # Linux
    LOCAL_IPS=$(hostname -I 2>/dev/null || ip -4 addr show | grep -oP '(?<=inet\s)\d+(\.\d+){3}' | grep -v '127.0.0.1' | tr '\n' ' ')
else
    # Windows (Git Bash / MSYS)
    LOCAL_IPS=$(ipconfig | grep -i "IPv4" | sed 's/.*: //' | tr '\n' ' ')
fi

echo "Found IPs: $LOCAL_IPS"
echo ""

# Create certs directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CERT_DIR="$SCRIPT_DIR/../certs"
mkdir -p "$CERT_DIR"

# Generate certificate
echo "🔑 Generating certificates..."
cd "$CERT_DIR"

# Build the domains list
DOMAINS="localhost 127.0.0.1 $LOCAL_IPS"
echo "Domains: $DOMAINS"
echo ""

mkcert -cert-file server.crt -key-file server.key $DOMAINS

echo ""
echo "✅ Certificates generated successfully!"
echo ""
echo "📁 Certificate location:"
echo "   $CERT_DIR/server.crt"
echo "   $CERT_DIR/server.key"
echo ""
echo "📱 To access from phones/tablets without warnings:"
echo "   1. Find the CA file: mkcert -CAROOT"
echo "   2. Copy rootCA.pem to your device"
echo "   3. Install it as a trusted certificate"
echo "   → See docs/TRUSTED_CERTIFICATES.md for detailed instructions"
echo ""
echo "⚙️  Update your config.json:"
echo '   "use_mkcert": true'
echo ""

