#!/bin/bash
set -e

echo "=== Smoke Test for Telegram Integration ==="
echo ""

# Kill any existing processes
pkill -f test-server || true
pkill -f perrus-cli || true
sleep 2

# Start test server
echo "1. Starting test server on :9999..."
/home/jarancibia/ai/perrus-cli-telegram/test-server &
TEST_SERVER_PID=$!
sleep 2

# Verify server is running
echo "2. Verifying test server is running..."
curl -s http://localhost:9999/health
echo ""
echo "✓ Test server is running"
echo ""

# Start perrus-cli
echo "3. Starting perrus-cli with Telegram config..."
/home/jarancibia/ai/perrus-cli-telegram/perrus-cli start -port=9192 -config=/home/jarancibia/ai/perrus-cli-telegram/config-test.yaml &
PERRUS_PID=$!
sleep 5
echo "✓ Perrus-cli is running"
echo ""

# Let it run successfully for a few cycles
echo "4. Letting it run successfully for 10 seconds..."
sleep 10
echo "✓ Successful monitoring period complete"
echo ""

# Stop test server to trigger failure
echo "5. Stopping test server to trigger failure alert..."
kill $TEST_SERVER_PID
sleep 3
echo "✓ Test server stopped"
echo ""

# Wait for grouped alert to be sent
echo "6. Waiting for Telegram alert (group interval: 2s)..."
sleep 5
echo "✓ Alert should have been sent"
echo ""

# Cleanup
echo "7. Cleaning up..."
kill $PERRUS_PID || true
pkill -f test-server || true
pkill -f perrus-cli || true
echo "✓ Cleanup complete"
echo ""

echo "=== Smoke Test Complete ==="
echo "Please check your Telegram for the alert message!"
