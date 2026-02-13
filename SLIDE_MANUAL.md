# Google Slides Creation Manual

This guide explains how to create Google Slides presentations from Markdown using the Google MCP Server.

## Prerequisites

- Google MCP Server configured with authenticated Google account
- Claude Code or other MCP client

## Markdown Format

### Page Breaks

Use `---` (three hyphens) on a separate line to create a new slide:

```markdown
# Slide 1 Title

Content for slide 1

---

## Slide 2 Title

Content for slide 2

---

## Slide 3 Title

Content for slide 3
```

### Supported Markdown Elements

- **Headings**: `#`, `##`, `###` for titles and subtitles
- **Lists**: `-` or `*` for bullet points, `1.` for numbered lists
- **Bold**: `**text**`
- **Code**: `` `inline code` `` or fenced code blocks
- **Tables**: Standard Markdown tables
- **Blockquotes**: `> quoted text`

## MCP Tools

### Create New Presentation

```
mcp__google__slides_markdown_create
```

Parameters:
- `account`: Google account email
- `title`: Presentation title
- `markdown`: Full Markdown content with `---` separators

### Update Existing Presentation

```
mcp__google__slides_markdown_update
```

Parameters:
- `account`: Google account email
- `presentation_id`: ID from the presentation URL
- `markdown`: Full Markdown content

### Append Slides

```
mcp__google__slides_markdown_append
```

Parameters:
- `account`: Google account email
- `presentation_id`: ID from the presentation URL
- `markdown`: Markdown content to append

Use this when hitting API rate limits - append one slide at a time.

## Workflow

### Basic Workflow

1. Write Markdown content with `---` page breaks
2. Use `slides_markdown_create` to generate presentation
3. Get the presentation URL from the response

### Incremental Updates

If you hit rate limits (429 errors):

1. Create an empty presentation or use existing one
2. Use `slides_markdown_append` for each slide individually
3. Wait between requests if needed

### Example

```markdown
# My Presentation

Speaker Name
2026.02.16

---

## Agenda

- Topic 1
- Topic 2
- Topic 3

---

## Topic 1

Details about topic 1

---

## Summary

- Key point 1
- Key point 2

---

## Thank You

Questions?
```

## Tips

- Keep slides concise - Google Slides has limited space
- Avoid complex nested structures
- Code blocks work but may need manual formatting adjustments
- Tables render as text; complex tables may need manual adjustment
- Emojis are supported (👤, 🤖, etc.)

## Rate Limits

Google Slides API has rate limits:
- 60 write requests per minute per user

If you encounter `429 Rate Limit Exceeded`:
- Wait 1-2 minutes before retrying
- Use `slides_markdown_append` to add slides one at a time
