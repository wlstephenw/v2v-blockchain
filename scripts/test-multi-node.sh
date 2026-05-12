#!/bin/bash
# Multi-node test script for V2V Blockchain (Task 10.9)

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
DATA_DIR="/tmp/v2v-test"

echo "=== V2V Blockchain Multi-Node Test (4 Validator Nodes for PBFT) ==="
echo ""

# Cleanup function
cleanup() {
    echo ""
    echo "=== Cleaning up ==="
    for pid in $NODE1_PID $NODE2_PID $NODE3_PID $NODE4_PID; do
        if [ -n "$pid" ]; then
            kill $pid 2>/dev/null || true
        fi
    done
    rm -rf $DATA_DIR
    echo "Cleanup complete"
}

trap cleanup EXIT

# Build the node
echo "Building v2v-node..."
cd $PROJECT_DIR
go build -o /tmp/v2v-node ./cmd/v2v-node

echo "Build complete"
echo ""

# Create data directories
mkdir -p $DATA_DIR/node1 $DATA_DIR/node2 $DATA_DIR/node3 $DATA_DIR/node4

# Start node 1 (validator - primary)
echo "Starting Node 1 (validator - primary) on port 8081..."
/tmp/v2v-node start \
    --data-dir $DATA_DIR/node1 \
    --api-port 8081 \
    --p2p-port 10001 \
    --validator \
    --log-level info &
NODE1_PID=$!
sleep 3

# Start node 2 (validator)
echo "Starting Node 2 (validator) on port 8082..."
/tmp/v2v-node start \
    --data-dir $DATA_DIR/node2 \
    --api-port 8082 \
    --p2p-port 10002 \
    --validator \
    --bootstrap /ip4/127.0.0.1/tcp/10001 \
    --log-level info &
NODE2_PID=$!
sleep 2

# Start node 3 (validator)
echo "Starting Node 3 (validator) on port 8083..."
/tmp/v2v-node start \
    --data-dir $DATA_DIR/node3 \
    --api-port 8083 \
    --p2p-port 10003 \
    --validator \
    --bootstrap /ip4/127.0.0.1/tcp/10001 \
    --log-level info &
NODE3_PID=$!
sleep 2

# Start node 4 (validator)
echo "Starting Node 4 (validator) on port 8084..."
/tmp/v2v-node start \
    --data-dir $DATA_DIR/node4 \
    --api-port 8084 \
    --p2p-port 10004 \
    --validator \
    --bootstrap /ip4/127.0.0.1/tcp/10001 \
    --log-level info &
NODE4_PID=$!
sleep 2

echo ""
echo "=== All nodes started ==="
echo ""

# Test health endpoints
echo "Testing health endpoints..."
for port in 8081 8082 8083 8084; do
    echo -n "Node on port $port: "
    if curl -s http://localhost:$port/health > /dev/null; then
        echo "OK"
    else
        echo "FAILED"
    fi
done

echo ""
echo "Testing node status..."
for port in 8081 8082 8083 8084; do
    echo "Node $port status:"
    curl -s http://localhost:$port/api/v1/node/status | head -20 || echo "Failed to get status"
    echo ""
done

echo ""
echo "=== Test Complete ==="
echo ""
echo "Nodes are running. Press Ctrl+C to stop."
echo ""

# Wait for user interrupt
wait
