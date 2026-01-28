---
name: github-labels
description: Use when creating or editing GitHub issues in moto-nrw/project-phoenix. Enforces label conventions - NEVER create new labels.
---

# GitHub Label Guidelines

Project Phoenix uses a fixed set of GitHub labels. **NEVER create new labels.**

## Approved Labels

### Type (required - pick one)
| Label | Use for |
|-------|---------|
| `bug` | Something isn't working |
| `feature` | New functionality |
| `ogs demanded feature` | Feature requested by OGS stakeholders |
| `epic` | Large multi-issue initiative |
| `maintenance` | Refactoring, tech debt, cleanup |

### Priority (required - pick one)
| Label | Use for |
|-------|---------|
| `priority: high` | Must have before launch |
| `priority: medium` | Important but can wait |
| `priority: low` | Nice to have |

### Deploy (optional)
| Label | Use for |
|-------|---------|
| `deploy-staging` | Auto-deploy to staging on merge |
| `deploy-demo` | Auto-deploy to demo on merge |

## Rules

1. **Use ONLY these labels** - never create new ones
2. **Use GitHub Assignees** for ownership (not labels)
3. **Every issue needs**: one Type label + one Priority label
4. **Deploy labels** are optional, add when PR is ready

## Commands

```bash
# Add labels
gh issue edit <number> --add-label "bug" --add-label "priority: medium"

# Remove labels
gh issue edit <number> --remove-label "feature"

# List available labels
gh label list --repo moto-nrw/project-phoenix
```
