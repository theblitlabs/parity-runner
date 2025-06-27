#!/bin/bash

set -e

echo "🚀 PLGenesis Runner - Auto Tunnel Startup"
echo "========================================="

# Check if .env exists
if [ ! -f ".env" ]; then
    echo "📝 Creating .env from sample..."
    cp .env.sample .env
fi

# Auto-enable tunneling in .env if not already set
if ! grep -q "RUNNER_TUNNEL_ENABLED=true" .env; then
    echo "🔧 Enabling tunnel configuration..."
    
    # Remove any existing tunnel settings to avoid duplicates
    sed -i.bak '/RUNNER_TUNNEL_/d' .env 2>/dev/null || true
    
    # Add tunnel configuration
    cat >> .env << EOF

# Auto-configured tunnel settings
RUNNER_TUNNEL_ENABLED=true
RUNNER_TUNNEL_TYPE=bore
RUNNER_TUNNEL_SERVER_URL=bore.pub
RUNNER_TUNNEL_PORT=0
RUNNER_TUNNEL_SECRET=
EOF
    
    echo "✅ Tunnel configuration added to .env"
else
    echo "✅ Tunnel already enabled in .env"
fi

# Show current tunnel configuration
echo ""
echo "📋 Current tunnel configuration:"
grep "RUNNER_TUNNEL_" .env | sed 's/^/   /'

echo ""
echo "🔧 Checking bore installation..."

# The tunnel client will auto-install bore if needed, but we can check here too
if ! command -v bore &> /dev/null; then
    echo "📦 bore not found - will be auto-installed on first run"
else
    echo "✅ bore is already installed"
    bore --version
fi

echo ""
echo "🚀 Starting PLGenesis Runner with tunnel support..."
echo "   Tunnel will auto-install bore if needed"
echo "   Webhook will be exposed via bore.pub"
echo "   Press Ctrl+C to stop"
echo ""

# Start the runner
exec ./parity-runner runner --config-path .env 