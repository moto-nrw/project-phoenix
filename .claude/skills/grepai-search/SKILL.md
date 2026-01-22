---
name: grepai-search
description: Use when searching codebase for concepts, patterns, or functionality. Prefer grepai over Grep/Glob for semantic queries like "authentication flow" or "error handling". Triggers on natural language code search requests.
metadata:
  author: moto-nrw
  version: "1.0.0"
---

# grepai Semantic Code Search

**Prefer grepai over traditional search** when the query is conceptual rather than literal.

## When to Use grepai vs Other Tools

| Query Type | Tool | Example |
|------------|------|---------|
| Conceptual/semantic | `grepai search` | "user authentication flow" |
| Function tracing | `grepai trace` | "who calls CreateStudent" |
| Exact string match | `Grep` | `"func Login("` |
| File pattern | `Glob` | `"**/*_test.go"` |

## Commands

```bash
# Semantic search - finds code by meaning
grepai search "user authentication flow"
grepai search "RFID check-in logic"
grepai search "database connection handling"

# Trace callers (who calls this function?)
grepai trace callers "Login"
grepai trace callers "CreateStudent"

# Trace callees (what does this function call?)
grepai trace callees "ProcessCheckin"

# Check index status
grepai status
```

## Important Notes

1. **Requires watcher running**: `grepai watch` must be active for searches to work
2. **First run takes time**: Initial indexing ~3-4 minutes for large codebases
3. **Results include scores**: Higher score (0.0-1.0) = more relevant
4. **Output format**: Shows file:lines with code snippets

## Dynamic Documentation

For latest features and options:

### Official Repository
```
WebFetch: https://raw.githubusercontent.com/yoanbernabeu/grepai/main/README.md
```

### Search for updates
```
WebSearch: grepai semantic code search ollama 2025
```

## Integration with Claude Code

When user asks to find code by concept:
1. Use `grepai search "query"` via Bash tool
2. Review results by relevance score
3. Use Read tool to examine top matches
4. Fall back to Grep only for exact string patterns

## Troubleshooting

| Issue | Solution |
|-------|----------|
| "No results" | Check `grepai watch` is running |
| Slow/hanging | Verify ollama is running with `ollama list` |
| Missing model | Run `ollama pull nomic-embed-text` |
