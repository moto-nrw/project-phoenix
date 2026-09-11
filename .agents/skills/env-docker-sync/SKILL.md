---
name: env-docker-sync
description: Use when adding, removing, or renaming environment variables, modifying docker-compose files, or when lefthook flags env sync warnings. Triggers on .env changes, docker-compose edits, or "missing keys" errors.
metadata:
  author: moto-nrw
  version: "1.0.0"
---

# Environment & Docker Sync

Read `.claude/rules/env-docker-sync.md` before changing env keys, Docker wiring,
or resolving sync warnings. It is the canonical inventory, fail-fast policy,
exception list, and checklist for both local and SOPS-managed deployments.

1. Identify the consumer and whether the value belongs in infrastructure env
   or tenant settings. Trace OS env vs config-file reads before adding wiring.
2. Follow the matching backend/frontend checklist, including deployed files
   and frontend build args. Edit encrypted files only with SOPS.
3. Run the listed sync checks directly and inspect their exit status and output.
   Missing tools and nonzero exits are failures, never `OK`. Report any check
   that could not run; do not print secret values in diagnostics.
