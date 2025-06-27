#!/bin/bash

echo "🔧 PLGenesis Runner - Tunnel Test"
echo "=================================="

# Check if bore is installed
if ! command -v bore &> /dev/null; then
    echo "❌ bore is not installed. Please run:"
    echo "   ./scripts/install_tunnel.sh"
    exit 1
fi

echo "✅ bore is installed"
bore --version

echo ""
echo "🚀 Testing bore tunnel..."
echo "🔗 Starting tunnel on port 8081..."

# Start bore tunnel in background
bore local 8081 --to bore.pub &
BORE_PID=$!

# Wait a bit for tunnel to establish
sleep 3

# Check if bore is still running
if ! kill -0 $BORE_PID 2>/dev/null; then
    echo "❌ Tunnel failed to start"
    exit 1
fi

echo "✅ Tunnel started successfully (PID: $BORE_PID)"
echo "⏳ Tunnel will run for 10 seconds..."

# Let it run for a while
sleep 10

# Stop the tunnel
echo "🛑 Stopping tunnel..."
kill $BORE_PID
wait $BORE_PID 2>/dev/null

echo "✅ Tunnel stopped successfully!"
echo "🎉 Test completed!"
echo ""
echo "💡 To enable tunneling in parity-runner:"
echo "   Set RUNNER_TUNNEL_ENABLED=true in your .env file" 