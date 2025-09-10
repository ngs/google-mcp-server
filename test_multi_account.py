#!/usr/bin/env python3
"""Test script for multi-account functionality"""

import json
import subprocess
import sys

def send_mcp_request(method, params=None):
    """Send an MCP request and get response"""
    request = {
        "jsonrpc": "2.0",
        "method": method,
        "params": params or {},
        "id": 1
    }
    
    # Start the server process
    process = subprocess.Popen(
        ["./google-mcp-server"],
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True
    )
    
    # Send request
    process.stdin.write(json.dumps(request) + "\n")
    process.stdin.flush()
    
    # Read response
    response_line = process.stdout.readline()
    
    # Terminate process
    process.terminate()
    
    try:
        return json.loads(response_line)
    except json.JSONDecodeError:
        return {"error": f"Failed to parse response: {response_line}"}

def test_accounts_tools():
    """Test account management tools"""
    print("Testing Multi-Account Functionality")
    print("=" * 50)
    
    # Test listing accounts
    print("\n1. Testing accounts_list tool...")
    response = send_mcp_request("tools/call", {
        "name": "accounts_list",
        "arguments": {}
    })
    
    if "result" in response:
        result = response["result"]
        if isinstance(result, dict) and "content" in result:
            content = json.loads(result["content"][0]["text"]) if isinstance(result["content"], list) else result
            print(f"   Found {content.get('count', 0)} account(s)")
            if "accounts" in content:
                for acc in content["accounts"]:
                    print(f"   - {acc.get('email')} (Active: {acc.get('active')})")
    else:
        print(f"   Error: {response.get('error', 'Unknown error')}")
    
    # Test account details
    print("\n2. Testing accounts_details tool...")
    response = send_mcp_request("tools/call", {
        "name": "accounts_details",
        "arguments": {}
    })
    
    if "result" in response:
        print("   Account details retrieved successfully")
    else:
        print(f"   Error: {response.get('error', 'Unknown error')}")
    
    # Test calendar list (should use existing account)
    print("\n3. Testing calendar_list with existing account...")
    response = send_mcp_request("tools/call", {
        "name": "calendar_list",
        "arguments": {}
    })
    
    if "result" in response:
        print("   Calendar list retrieved successfully")
    else:
        print(f"   Error: {response.get('error', 'Unknown error')}")
    
    print("\n" + "=" * 50)
    print("Multi-account functionality test complete!")

if __name__ == "__main__":
    test_accounts_tools()