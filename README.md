# Google MCP Server

A Model Context Protocol (MCP) server that integrates with Google APIs (Calendar, Drive, Gmail, Photos, Sheets, Docs) to provide seamless access to Google services through MCP-compatible clients.

## Features

- **Multi-Service Support**: Integrated support for Google Calendar, Drive, Gmail, Photos, Sheets, and Docs
- **OAuth 2.0 Authentication**: Secure authentication with automatic token refresh
- **MCP Protocol Compliant**: Fully compliant with the Model Context Protocol specification
- **Cross-Platform**: Works on Windows, macOS, and Linux
- **Configurable**: Flexible configuration through JSON files or environment variables

## Quick Start

### macOS Quick Setup (Recommended)

```bash
# Install via Homebrew
brew tap ngs/tap
brew install google-mcp-server

# Configure Claude Desktop (Apple Silicon)
echo '{
  "mcpServers": {
    "google": {
      "command": "/opt/homebrew/bin/google-mcp-server"
    }
  }
}' > ~/Library/Application\ Support/Claude/claude_desktop_config.json

# Run to authenticate (first time only)
google-mcp-server
```

For detailed setup instructions, see below.

### Prerequisites

1. Google Cloud Project with APIs enabled
2. OAuth 2.0 credentials from Google Cloud Console
3. Go 1.23 or later (only for building from source)

### Installation

#### Using Homebrew (macOS/Linux)

```bash
brew tap ngs/tap
brew install google-mcp-server
```

#### From Source

```bash
git clone https://github.com/ngs/google-mcp-server.git
cd google-mcp-server
go build
```

#### Using Go Install

```bash
go install go.ngs.io/google-mcp-server@latest
```

#### Pre-built Binaries

Download pre-built binaries from the [releases page](https://github.com/ngs/google-mcp-server/releases) for:
- macOS (Intel & Apple Silicon)
- Linux (x86_64 & ARM64)
- Windows (x86_64)

### Setup

1. **Create a Google Cloud Project**
   - Go to [Google Cloud Console](https://console.cloud.google.com)
   - Create a new project or select an existing one
   - Enable the required APIs:
     - Google Calendar API
     - Google Drive API
     - Gmail API
     - Photos Library API
     - Google Sheets API
     - Google Docs API

2. **Create OAuth 2.0 Credentials**
   - In Google Cloud Console, go to "APIs & Services" > "Credentials"
   - Click "Create Credentials" > "OAuth client ID"
   - Choose "Desktop app" as the application type
   - Download the credentials JSON file

3. **Configure the Server**

   Create a `config.json` file:

   ```json
   {
     "oauth": {
       "client_id": "YOUR_CLIENT_ID.apps.googleusercontent.com",
       "client_secret": "YOUR_CLIENT_SECRET",
       "redirect_uri": "http://localhost:8080/callback"
     },
     "services": {
       "calendar": {"enabled": true},
       "drive": {"enabled": true},
       "gmail": {"enabled": true},
       "photos": {"enabled": true},
       "sheets": {"enabled": true},
       "docs": {"enabled": true}
     }
   }
   ```

   Or use environment variables:
   ```bash
   export GOOGLE_CLIENT_ID="YOUR_CLIENT_ID.apps.googleusercontent.com"
   export GOOGLE_CLIENT_SECRET="YOUR_CLIENT_SECRET"
   ```

4. **Run the Server**

   ```bash
   ./google-mcp-server
   ```

   On first run, it will open a browser for authentication. Grant the requested permissions to proceed.

## Available Tools

### Google Calendar
- `calendar_list` - List all accessible calendars
- `calendar_events_list` - List events with date range filtering
- `calendar_event_create` - Create new events
- `calendar_event_update` - Update existing events
- `calendar_event_delete` - Delete events
- `calendar_event_get` - Get event details
- `calendar_freebusy_query` - Query free/busy information
- `calendar_event_search` - Search for events

### Google Drive
- `drive_files_list` - List files and folders
- `drive_files_search` - Search for files
- `drive_file_download` - Download files
- `drive_file_upload` - Upload files
- `drive_file_get_metadata` - Get file metadata
- `drive_file_update_metadata` - Update file metadata
- `drive_folder_create` - Create folders
- `drive_file_move` - Move files
- `drive_file_copy` - Copy files
- `drive_file_delete` - Delete files
- `drive_file_trash` - Move files to trash
- `drive_file_restore` - Restore files from trash
- `drive_shared_link_create` - Create shareable links
- `drive_permissions_list` - List file permissions
- `drive_permissions_create` - Grant permissions
- `drive_permissions_delete` - Remove permissions

### Google Gmail
- `gmail_messages_list` - List email messages
- `gmail_messages_search` - Search emails
- `gmail_message_get` - Get email details
- (Additional tools in full implementation)

### Google Photos
- `photos_albums_list` - List photo albums
- `photos_album_get` - Get album details
- (Additional tools in full implementation)

### Google Sheets
- `sheets_spreadsheet_get` - Get spreadsheet metadata
- `sheets_values_get` - Get cell values
- `sheets_values_update` - Update cell values
- (Additional tools in full implementation)

### Google Docs
- `docs_document_get` - Get document content
- `docs_document_create` - Create new documents
- (Additional tools in full implementation)

## Usage Examples

### With Claude Desktop

#### macOS (Homebrew Installation)

If you installed via Homebrew, add to your Claude Desktop configuration (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "google": {
      "command": "/opt/homebrew/bin/google-mcp-server"
    }
  }
}
```

For Intel Macs, use `/usr/local/bin/google-mcp-server` instead.

#### Other Installations

Add to your Claude Desktop configuration:

```json
{
  "mcpServers": {
    "google": {
      "command": "/path/to/google-mcp-server"
    }
  }
}
```

### With Claude Code

Claude Code users can set up the Google MCP Server using either the `claude mcp add` command or manual configuration:

#### Method 1: Using claude mcp add (Recommended)

1. **Install the server** (if not already installed):
   ```bash
   # Using Homebrew (recommended for macOS)
   brew tap ngs/tap
   brew install google-mcp-server
   
   # Or build from source
   git clone https://github.com/ngs/google-mcp-server.git
   cd google-mcp-server
   go build
   ```

2. **Add the MCP server to Claude Code**:
   ```bash
   # If installed via Homebrew (Apple Silicon)
   claude mcp add google /opt/homebrew/bin/google-mcp-server
   
   # If installed via Homebrew (Intel Mac)
   claude mcp add google /usr/local/bin/google-mcp-server
   
   # If built from source
   claude mcp add google /path/to/your/google-mcp-server/google-mcp-server
   ```

3. **Configure OAuth credentials**:
   - Follow the Google Cloud Project setup steps above
   - Create a `config.json` file with your OAuth credentials in the current directory or home directory
   - Or set environment variables in your shell profile:
     ```bash
     export GOOGLE_CLIENT_ID="YOUR_CLIENT_ID.apps.googleusercontent.com"
     export GOOGLE_CLIENT_SECRET="YOUR_CLIENT_SECRET"
     ```

4. **Authenticate** (first time only):
   ```bash
   # Run the server directly to complete OAuth flow
   google-mcp-server
   # This will open a browser for authentication
   # Grant the requested permissions
   # The token will be saved to ~/.google-mcp-token.json
   ```

5. **Restart Claude Code** to apply the changes:
   ```bash
   # The MCP server will be available after restarting Claude Code
   ```

#### Method 2: Manual Configuration

1. **Install the server** (same as Method 1, step 1)

2. **Configure OAuth credentials** (same as Method 1, step 3)

3. **Authenticate** (same as Method 1, step 4)

4. **Manually configure Claude Code**:
   
   Add the MCP server to your Claude Code configuration. Create or edit `~/.claude/config.json`:
   
   ```json
   {
     "mcpServers": {
       "google": {
         "command": "/opt/homebrew/bin/google-mcp-server"
       }
     }
   }
   ```
   
   Or if you built from source:
   ```json
   {
     "mcpServers": {
       "google": {
         "command": "/path/to/your/google-mcp-server/google-mcp-server"
       }
     }
   }
   ```

#### Verify the Setup

In Claude Code, you can test the connection by asking:
- "List my Google calendars"
- "Show my recent Gmail messages"
- "List files in my Google Drive"

#### Troubleshooting Claude Code Setup

- **Check MCP server list**: Run `claude mcp list` to verify the server is registered
- **Remove and re-add**: If issues persist, use `claude mcp remove google` then add it again
- **Ensure executable permissions**: `chmod +x /path/to/google-mcp-server`
- **Verify token file**: Check that `~/.google-mcp-token.json` exists after authentication
- **Test server directly**: Run `google-mcp-server --version` to ensure it works
- **Check Claude Code logs**: Look for MCP server errors in Claude Code output

### Programmatic Usage

```python
# Example: List upcoming calendar events
response = mcp_client.call_tool(
    "calendar_events_list",
    {
        "calendar_id": "primary",
        "time_min": "2024-01-01T00:00:00Z",
        "max_results": 10
    }
)
```

## Configuration

### Configuration File

The server looks for configuration in the following locations (in order):
1. `config.json` in the current directory
2. `config.local.json` in the current directory
3. `~/.google-mcp-server/config.json`
4. `/etc/google-mcp-server/config.json`

### Environment Variables

- `GOOGLE_CLIENT_ID` - OAuth client ID
- `GOOGLE_CLIENT_SECRET` - OAuth client secret
- `GOOGLE_REDIRECT_URI` - OAuth redirect URI
- `GOOGLE_TOKEN_FILE` - Token storage location
- `DISABLE_<SERVICE>` - Disable specific services (e.g., `DISABLE_GMAIL=true`)
- `LOG_LEVEL` - Logging level (debug, info, warn, error)

## Google Workspace Support

This server supports both personal Google accounts (@gmail.com) and Google Workspace accounts. For Workspace accounts:

1. **Admin Consent**: Your Workspace administrator may need to approve the application
2. **Domain Restrictions**: Some organizations restrict third-party app access
3. **API Limitations**: Certain APIs may be disabled by your organization

### Workspace Setup Guide

See [WORKSPACE_SETUP.md](WORKSPACE_SETUP.md) for detailed instructions on configuring the server for Google Workspace environments.

## Security Considerations

- **Token Storage**: OAuth tokens are stored locally in `~/.google-mcp-token.json` with restricted permissions
- **Scopes**: Only request the minimum necessary scopes for your use case
- **Credentials**: Never commit OAuth credentials to version control
- **Network**: Use HTTPS for all API communications

## Troubleshooting

### Common Issues

1. **Authentication Errors**
   - Ensure OAuth credentials are correctly configured
   - Check that all required APIs are enabled in Google Cloud Console
   - Verify redirect URI matches configuration

2. **Permission Denied**
   - Confirm you've granted all requested permissions during OAuth flow
   - For Workspace accounts, check with your administrator

3. **Rate Limiting**
   - The server implements exponential backoff for rate limits
   - Consider reducing request frequency if issues persist

4. **Token Expiration**
   - Tokens are automatically refreshed
   - If refresh fails, re-authenticate by deleting the token file

## Development

### Building from Source

```bash
# Clone the repository
git clone https://github.com/ngs/google-mcp-server.git
cd google-mcp-server

# Install dependencies
go mod download

# Build
go build

# Run tests
go test ./...

# Run with race detector
go test -race ./...
```

### Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

See [CONTRIBUTING.md](CONTRIBUTING.md) for detailed guidelines.

## API Rate Limits

Each Google API has its own rate limits:

- **Calendar**: 1,000,000 queries/day
- **Drive**: 1,000,000,000 queries/day
- **Gmail**: 250 quota units/user/second
- **Photos**: 10,000 requests/day
- **Sheets**: 100 requests/100 seconds
- **Docs**: 60 requests/minute

The server implements automatic retry with exponential backoff when limits are reached.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- Based on the [Dropbox MCP](https://github.com/ngs/dropbox-mcp) implementation pattern
- Built on the [Model Context Protocol](https://spec.modelcontextprotocol.io/) specification
- Uses Google API Go Client Libraries

## Support

For issues, questions, or contributions, please visit the [GitHub repository](https://github.com/ngs/google-mcp-server).

## Roadmap

- [ ] Full implementation of all Gmail tools
- [ ] Complete Photos API integration
- [ ] Advanced Sheets operations (charts, pivots)
- [ ] Docs formatting and collaboration features
- [ ] Batch operations optimization
- [ ] Webhook support for real-time updates
- [ ] Multi-account management UI
- [ ] Docker container support