---
name: slack-formatter
description: Format text for Slack messages. Use when user wants to share content in Slack, format messages for Slack, or copy text to clipboard for Slack. Converts markdown to Slack-compatible mrkdwn syntax.
---

# Slack Message Formatter

Format content for Slack using their `mrkdwn` syntax (NOT standard Markdown).

## Quick Reference

### Slack mrkdwn Syntax

| Format | Slack Syntax | Example |
|--------|--------------|---------|
| Bold | `*text*` | `*important*` |
| Italic | `_text_` | `_emphasis_` |
| Strikethrough | `~text~` | `~deleted~` |
| Inline code | `` `text` `` | `` `code` `` |
| Code block | ` ``` ` (triple backticks) | See below |
| Link | `<URL\|text>` | `<https://example.com\|Click here>` |
| Bullet list | `• item` or `* item` | `• First item` |
| Numbered list | `1. item` | `1. First item` |
| Quote | `> text` | `> quoted text` |
| User mention | `<@USERID>` | `<@U123ABC>` |
| Channel | `<#CHANNELID>` | `<#C123ABC>` |
| Emoji | `:emoji_name:` | `:rocket:` |

### What Slack Does NOT Support

- `**bold**` (use `*bold*` instead)
- `## Headers` (use `*Bold Text*` instead)
- Tables with `| |` syntax (not supported at all)
- Horizontal rules `---` (not rendered)
- Complex ASCII art (gets mangled)
- Syntax highlighting in code blocks
- Nested formatting
- Images via markdown

## Conversion Rules

When converting content for Slack:

1. **Headers**: Replace `## Header` with `*Header*` (bold)
2. **Bold**: Replace `**text**` with `*text*`
3. **Tables**: Convert to bullet lists or simple text
4. **Diagrams**: Simplify to basic ASCII in code blocks, or describe in text
5. **Links**: Convert `[text](url)` to `<url|text>`
6. **Lists**: Use `•` for bullets, `1.` for numbered

## Code Block Example

```
This is a code block in Slack.
No syntax highlighting available.
Keep it simple and readable.
```

## Workflow

When user asks to format for Slack:

1. **Convert** the content using rules above
2. **Simplify** any tables or complex diagrams
3. **Copy to clipboard** using `pbcopy` (macOS) or `xclip` (Linux)
4. **Confirm** with user that it's ready to paste

### Copy to Clipboard Command

```bash
# macOS
cat << 'EOF' | pbcopy
Your formatted content here
EOF

# Linux
cat << 'EOF' | xclip -selection clipboard
Your formatted content here
EOF
```

## Example Conversion

### Input (Standard Markdown)

```markdown
## Important Update

**Key changes:**
- Feature A added
- Bug B fixed

| Status | Count |
|--------|-------|
| Done   | 5     |
| Pending| 3     |

Check the [documentation](https://docs.example.com).
```

### Output (Slack mrkdwn)

```
*Important Update*

*Key changes:*
• Feature A added
• Bug B fixed

*Status:*
• Done: 5
• Pending: 3

Check the <https://docs.example.com|documentation>.
```

## Tips for Good Slack Messages

1. **Keep it short** - Slack is for quick communication
2. **Use emoji sparingly** - They work but don't overdo it
3. **Code blocks for anything monospace** - Diagrams, logs, code
4. **Bold for emphasis** - `*key point*`
5. **Bullet points** - Easier to scan than paragraphs
6. **Break up long messages** - Use line breaks liberally

## Reference Links

For more details on Slack formatting:
- Official: https://slack.com/help/articles/202288908-Format-your-messages-in-Slack
- API Reference: https://api.slack.com/reference/surfaces/formatting
- Markdown Guide: https://www.markdownguide.org/tools/slack/

## Common Emoji Shortcodes

| Emoji | Code |
|-------|------|
| ✅ | `:white_check_mark:` |
| ❌ | `:x:` |
| 🚀 | `:rocket:` |
| 💡 | `:bulb:` |
| ⚠️ | `:warning:` |
| 🎉 | `:tada:` |
| 👍 | `:+1:` |
| 🔥 | `:fire:` |
| 📝 | `:memo:` |
| 🐛 | `:bug:` |
