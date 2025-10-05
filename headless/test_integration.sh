#!/bin/bash
set -e

echo "Building headless..."
go build -o headless ./cmd/headless

echo "Starting headless server in background..."
./headless start -port 12345

# Give server time to start
sleep 2

# Trap to ensure cleanup
cleanup() {
    echo "Cleaning up..."
    ./headless -port 12345 stop 2>/dev/null || true
    rm -f test_screenshot.png /tmp/headless-12345.pid
}
trap cleanup EXIT

echo "Testing navigate command..."
if ./headless -port 12345 navigate https://example.com; then
    echo "✓ Navigate command succeeded"
else
    echo "✗ Navigate command failed"
    exit 1
fi

sleep 1

echo "Testing eval command..."
if ./headless -port 12345 eval "document.title"; then
    echo "✓ Eval command succeeded"
else
    echo "✗ Eval command failed"
    exit 1
fi

sleep 1

echo "Testing screenshot command..."
if ./headless -port 12345 screenshot test_screenshot.png; then
    if [ -f test_screenshot.png ]; then
        echo "✓ Screenshot command succeeded"
    else
        echo "✗ Screenshot file not created"
        exit 1
    fi
else
    echo "✗ Screenshot command failed"
    exit 1
fi

sleep 1

echo "Testing read_console command..."
if ./headless -port 12345 read_console; then
    echo "✓ Read console command succeeded"
else
    echo "✗ Read console command failed"
    exit 1
fi

sleep 1

echo "Testing clear_console command..."
if ./headless -port 12345 clear_console; then
    echo "✓ Clear console command succeeded"
else
    echo "✗ Clear console command failed"
    exit 1
fi

sleep 1

echo "Testing resize command..."
if ./headless -port 12345 resize 1280 720; then
    echo "✓ Resize command succeeded"
else
    echo "✗ Resize command failed"
    exit 1
fi

sleep 1

echo "Testing HTTP endpoint..."
if curl -s http://localhost:12345/ | grep -q "Headless Browser Control"; then
    echo "✓ Web interface accessible"
else
    echo "✗ Web interface not accessible"
    exit 1
fi

echo ""
echo "All integration tests passed! ✓"
