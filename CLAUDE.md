# Claude Code Instructions for Google MCP Server

This document contains specific instructions for Claude Code when working with the Google MCP Server project.

## Project Overview

This is a Go-based MCP (Model Context Protocol) server that integrates with Google APIs including Calendar, Drive, Gmail, Sheets, and Docs. The server uses OAuth 2.0 for authentication and provides tools and resources accessible through MCP-compatible clients.

## Development Guidelines

### Important Rules
- **NEVER create test scripts or demo scripts** - Testing should be done through the MCP interface or existing tools
- **DO NOT create standalone test programs** - Use the MCP server directly for testing functionality
- **AVOID creating example/demo files** - The MCP server itself is the interface for testing
- **NEVER write Go programs to call Google APIs directly** - Always use MCP tools or existing server functionality
- **DO NOT create any executable programs** - If MCP tools are unavailable, investigate why and report the issue instead

### Code Style
- Use Go idioms and best practices
- Follow the existing code structure and patterns
- Keep functions focused and single-purpose
- Use meaningful variable and function names
- Add error handling for all API calls

### Testing
Before committing any changes, always run:
```bash
go test ./...
go fmt ./...
```

### Building
To build the project:
```bash
go build -o google-mcp-server .
```

## Service Implementation Status

### Fully Implemented with Multi-Account Support
- **Calendar Service** (`calendar/`): All 8 tools + multi-account support via `calendar/multi_account.go`
- **Drive Service** (`drive/`): All 19 tools including Markdown support + multi-account support via `drive/multi_account.go`
- **Gmail Service** (`gmail/`): 3 core tools + multi-account support via `gmail/multi_account.go`
- **Account Management** (`accounts/`): 5 tools for managing multiple Google accounts

### Basic Implementation
- **Sheets Service** (`sheets/`): 3 basic tools (get/update values, get spreadsheet)
- **Docs Service** (`docs/`): 3 basic tools (get/create/update documents)

## Common Tasks

### Adding a New Tool to a Service

1. Add the tool definition in `<service>/tools.go` or `<service>/multi_account.go` in the `GetTools()` method
2. Implement the handler in `HandleToolCall()` method (consider multi-account support)
3. Add the corresponding client method in `<service>/client.go`
4. For multi-account tools, update the `MultiAccountHandler` in `<service>/multi_account.go`
5. Update documentation in README.md

### Updating Dependencies
```bash
go get -u ./...
go mod tidy
```

### Running Tests
```bash
make test
# or with coverage
make test-coverage
```

## API Rate Limits to Consider

- **Calendar**: 1,000,000 queries/day
- **Drive**: 1,000,000,000 queries/day  
- **Gmail**: 250 quota units/user/second
- **Sheets**: 100 requests/100 seconds
- **Docs**: 60 requests/minute

Always implement exponential backoff for rate limit errors.

## Security Considerations

- Never log or expose OAuth tokens
- Store tokens with restricted file permissions (0600)
- Validate all user inputs before API calls
- Use context timeouts for long-running operations

## Known Issues and TODOs

### High Priority
- [x] Multi-account support for Calendar, Drive, and Gmail
- [ ] Complete remaining Gmail tools (send, reply, labels, etc.)
- [ ] Add comprehensive unit tests for all services
- [ ] Implement batch operations for better performance

### Medium Priority
- [ ] Add request caching where appropriate
- [ ] Implement webhook support for real-time updates
- [ ] Add metrics and monitoring capabilities
- [ ] Create Docker container configuration

### Low Priority
- [ ] Add CLI configuration wizard
- [ ] Implement service-specific rate limiting
- [ ] Add request/response logging options
- [ ] Create web-based configuration UI

## Debugging Tips

### OAuth Issues
- Check if token files exist: `~/.google-mcp-accounts/*.json`
- For single account legacy mode: `~/.google-mcp-token.json` (auto-migrated to multi-account)
- Verify all required APIs are enabled in Google Cloud Console
- Ensure redirect URI matches exactly: `http://localhost:8080/callback`
- Use `accounts_list` to see authenticated accounts
- Use `accounts_add` to add new accounts

### MCP Connection Issues
- Use `--debug` flag (when implemented) for verbose logging
- Check JSON-RPC message format in server/mcp.go
- Verify stdio stream handling is working correctly

### API Errors
- Check service-specific quotas in Google Cloud Console
- Verify OAuth scopes match required permissions
- Look for rate limiting errors (implement backoff)

## File Structure Reference

```
.
├── auth/           # OAuth authentication logic and multi-account management
├── accounts/       # Account management tools and handlers
├── calendar/       # Google Calendar service
│   ├── client.go   # API client wrapper
│   ├── tools.go    # MCP tool implementations
│   └── resources.go # MCP resource implementations
├── drive/          # Google Drive service (same structure)
├── gmail/          # Gmail service with multi-account support
├── sheets/         # Sheets service (needs expansion)
├── docs/           # Docs service (needs expansion)
├── server/         # MCP server implementation
├── config/         # Configuration management
└── main.go         # Entry point
```

## Contact and Support

For questions about implementation details or architectural decisions, refer to:
- MCP Specification: https://spec.modelcontextprotocol.io/
- Google API Documentation: https://developers.google.com/apis-explorer
- Project Repository: https://github.com/ngs/google-mcp-server

## Release Process

1. Run tests: `make test`
2. Set version and create tag: `make set-version VERSION=v0.x.x`
3. Push tag: `git push origin v0.x.x`
4. GitHub Actions will handle the rest via GoReleaser

Note: The `make set-version` command will:
- Regenerate server/version.go with the new VERSION const (strips 'v' prefix for internal version)
- Commit the version change
- Create an annotated git tag with the version message (keeps 'v' prefix for tag)

## Performance Optimization Tips

- Use batch requests where possible (especially for Sheets/Docs)
- Implement pagination for large result sets
- Cache frequently accessed resources
- Use goroutines for parallel API calls (with proper rate limiting)
- Minimize API calls by fetching only required fields

## Error Handling Pattern

Always follow this pattern for API errors:
```go
result, err := apiCall()
if err != nil {
    return nil, fmt.Errorf("failed to perform operation: %w", err)
}
```

This ensures proper error wrapping and context throughout the call stack.

## Google Docs Creation Guidelines

When creating or uploading documents to Google Docs:
- **For Markdown content**: Always use `mcp__google__drive_markdown_upload` tool instead of `mcp__google__docs_document_create` followed by `mcp__google__docs_document_update`
- **For updating existing Google Docs with Markdown**: Use `mcp__google__drive_markdown_replace` tool
- The `drive_markdown_upload` tool properly converts Markdown to Google Docs format with correct formatting
- The `docs_document_create` and `docs_document_update` tools should only be used for plain text content without Markdown formatting

## Google Slides Guidelines

### Font Guidelines

**IMPORTANT**: When applying monospace fonts for code content in Google Slides:
- **ALWAYS use "Courier New"** - NOT "Courier"
- This applies to:
  - Inline code (backtick-wrapped text)
  - Code blocks (triple-backtick-wrapped text)
  - Any code-related content formatting
- Google Slides recognizes "Courier New" as the standard monospace font
- Using "Courier" instead of "Courier New" will result in incorrect font rendering

### Layout Selection Logic

The Slides service automatically selects the appropriate layout based on slide content:

1. **TITLE Layout**: Used when a slide contains:
   - Exactly 2 headings with no other content (any slide)
   - Only headings and no other content (first slide only)
   - Perfect for title slides and section dividers

2. **TITLE_AND_BODY Layout**: Default layout for regular content slides containing:
   - Mixed content (headings + text/bullets/code)
   - Standard presentation content

3. **TITLE_ONLY Layout**: Used for slides containing:
   - Tables (provides more space for table content)

### Implementation Notes

- Title slide detection is implemented in `slides/markdown.go` in the `CreateSlidesFromMarkdown` function
- The `populateSlideWithTitleLayout` function handles TITLE layout population
- Test coverage includes `TestTitleSlideDetection` and `TestPopulateSlideWithTitleLayoutLogic`