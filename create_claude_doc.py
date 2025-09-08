#!/usr/bin/env python3
import json
import subprocess
import sys

def send_mcp_request(method, params=None):
    """Send an MCP request to the server"""
    request = {
        "jsonrpc": "2.0",
        "id": 1,
        "method": method
    }
    if params:
        request["params"] = params
    
    # Start the MCP server process
    process = subprocess.Popen(
        ["./google-mcp-server"],
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True
    )
    
    # Send request
    request_str = json.dumps(request) + "\n"
    stdout, stderr = process.communicate(input=request_str)
    
    if stderr:
        print(f"Error: {stderr}", file=sys.stderr)
        return None
    
    try:
        response = json.loads(stdout.strip())
        return response
    except json.JSONDecodeError as e:
        print(f"Failed to parse response: {e}", file=sys.stderr)
        print(f"Raw output: {stdout}", file=sys.stderr)
        return None

def main():
    # Read CLAUDE.md content
    try:
        with open("CLAUDE.md", "r") as f:
            claude_md_content = f.read()
    except FileNotFoundError:
        print("CLAUDE.md file not found", file=sys.stderr)
        return 1
    
    # Initialize MCP server
    print("Initializing MCP server...")
    init_response = send_mcp_request("initialize", {
        "protocolVersion": "2024-11-05",
        "capabilities": {
            "tools": {}
        },
        "clientInfo": {
            "name": "claude-doc-creator",
            "version": "1.0.0"
        }
    })
    
    if not init_response or "error" in init_response:
        print("Failed to initialize MCP server", file=sys.stderr)
        if init_response:
            print(f"Error: {init_response.get('error', 'Unknown error')}", file=sys.stderr)
        return 1
    
    # Create document
    print("Creating Google Docs document...")
    create_response = send_mcp_request("tools/call", {
        "name": "docs_document_create",
        "arguments": {
            "title": "Claude Code Instructions for Google MCP Server"
        }
    })
    
    if not create_response or "error" in create_response:
        print("Failed to create document", file=sys.stderr)
        if create_response:
            print(f"Error: {create_response.get('error', 'Unknown error')}", file=sys.stderr)
        return 1
    
    # Get document ID from response
    result = create_response.get("result", {})
    content = result.get("content", [])
    if not content:
        print("No content in create response", file=sys.stderr)
        return 1
    
    document_id = None
    for item in content:
        if item.get("type") == "text":
            try:
                doc_data = json.loads(item.get("text", "{}"))
                document_id = doc_data.get("documentId")
                break
            except json.JSONDecodeError:
                continue
    
    if not document_id:
        print("Could not extract document ID", file=sys.stderr)
        return 1
    
    print(f"Created document with ID: {document_id}")
    
    # Format the content with markdown formatting
    print("Adding content to document...")
    format_response = send_mcp_request("tools/call", {
        "name": "docs_document_format",
        "arguments": {
            "document_id": document_id,
            "markdown_content": claude_md_content,
            "mode": "replace"
        }
    })
    
    if not format_response or "error" in format_response:
        print("Failed to format document", file=sys.stderr)
        if format_response:
            print(f"Error: {format_response.get('error', 'Unknown error')}", file=sys.stderr)
        return 1
    
    print("Successfully created Google Docs document with CLAUDE.md content!")
    print(f"Document ID: {document_id}")
    print(f"Document URL: https://docs.google.com/document/d/{document_id}/edit")
    
    return 0

if __name__ == "__main__":
    sys.exit(main())