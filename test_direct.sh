#!/bin/bash

# Create a test script to interact with MCP server
echo "Testing MCP server directly..."

# Create FIFO pipes for communication
mkfifo mcp_in mcp_out 2>/dev/null

# Start the MCP server in background
timeout 30s ./google-mcp-server < mcp_in > mcp_out &
MCP_PID=$!

# Give server time to start
sleep 2

# Send initialize request
echo '{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {"protocolVersion": "2024-11-05", "capabilities": {"tools": {}}, "clientInfo": {"name": "test-client", "version": "1.0.0"}}}' > mcp_in

# Read response
timeout 5s cat mcp_out

# Clean up
kill $MCP_PID 2>/dev/null
rm -f mcp_in mcp_out