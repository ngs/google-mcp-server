#!/usr/bin/env python3
import json
import subprocess
import threading
import time
import sys

def read_claude_md():
    """Read CLAUDE.md content"""
    try:
        with open("CLAUDE.md", "r") as f:
            return f.read()
    except FileNotFoundError:
        print("CLAUDE.md file not found")
        return None

def main():
    # Read CLAUDE.md content
    claude_content = read_claude_md()
    if not claude_content:
        return 1
    
    print("Starting MCP server...")
    
    # Start the MCP server
    process = subprocess.Popen(
        ["./google-mcp-server"],
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        bufsize=1,
        universal_newlines=True
    )
    
    def send_request(request):
        """Send a JSON-RPC request"""
        request_str = json.dumps(request) + "\n"
        print(f"Sending: {request_str.strip()}")
        process.stdin.write(request_str)
        process.stdin.flush()
    
    def read_response():
        """Read a response from stdout"""
        try:
            line = process.stdout.readline()
            if line:
                print(f"Received: {line.strip()}")
                return json.loads(line.strip())
        except json.JSONDecodeError as e:
            print(f"JSON decode error: {e}")
            print(f"Raw line: {repr(line)}")
        except Exception as e:
            print(f"Error reading response: {e}")
        return None
    
    try:
        # Wait a moment for server to start
        time.sleep(1)
        
        # Initialize
        print("Initializing...")
        init_request = {
            "jsonrpc": "2.0",
            "id": 1,
            "method": "initialize",
            "params": {
                "protocolVersion": "2024-11-05",
                "capabilities": {"tools": {}},
                "clientInfo": {"name": "claude-doc-creator", "version": "1.0.0"}
            }
        }
        send_request(init_request)
        init_response = read_response()
        
        if not init_response or "error" in init_response:
            print(f"Initialization failed: {init_response}")
            return 1
        
        print("Initialization successful!")
        
        # Send initialized notification
        initialized_notification = {
            "jsonrpc": "2.0",
            "method": "initialized"
        }
        send_request(initialized_notification)
        
        # Get tools list
        print("Getting tools...")
        tools_request = {
            "jsonrpc": "2.0",
            "id": 2,
            "method": "tools/list"
        }
        send_request(tools_request)
        tools_response = read_response()
        print(f"Tools: {tools_response}")
        
        # Create document
        print("Creating document...")
        create_request = {
            "jsonrpc": "2.0",
            "id": 3,
            "method": "tools/call",
            "params": {
                "name": "docs_document_create",
                "arguments": {
                    "title": "Claude Code Instructions for Google MCP Server"
                }
            }
        }
        send_request(create_request)
        create_response = read_response()
        
        if not create_response or "error" in create_response:
            print(f"Document creation failed: {create_response}")
            return 1
        
        print(f"Document created: {create_response}")
        
        # Extract document ID
        result = create_response.get("result", {})
        content = result.get("content", [])
        
        document_id = None
        if content and len(content) > 0:
            text_content = content[0].get("text", "{}")
            try:
                doc_data = json.loads(text_content)
                document_id = doc_data.get("documentId")
            except json.JSONDecodeError:
                print("Failed to parse document creation response")
                return 1
        
        if not document_id:
            print("Could not extract document ID")
            return 1
        
        print(f"Document ID: {document_id}")
        
        # Add content using markdown formatting
        print("Adding content to document...")
        format_request = {
            "jsonrpc": "2.0",
            "id": 4,
            "method": "tools/call",
            "params": {
                "name": "docs_document_format",
                "arguments": {
                    "document_id": document_id,
                    "markdown_content": claude_content,
                    "mode": "replace"
                }
            }
        }
        send_request(format_request)
        format_response = read_response()
        
        if not format_response or "error" in format_response:
            print(f"Document formatting failed: {format_response}")
            return 1
        
        print(f"Document formatted successfully: {format_response}")
        print(f"Document URL: https://docs.google.com/document/d/{document_id}/edit")
        
        return 0
        
    except KeyboardInterrupt:
        print("Interrupted by user")
        return 1
    except Exception as e:
        print(f"Error: {e}")
        return 1
    finally:
        process.terminate()
        try:
            process.wait(timeout=5)
        except subprocess.TimeoutExpired:
            process.kill()

if __name__ == "__main__":
    sys.exit(main())