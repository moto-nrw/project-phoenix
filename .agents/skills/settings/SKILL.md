---
name: settings
description: Use when adding, editing, deleting, or debugging tenant-scoped settings. Triggers on mentions of per-school config, settings registry, setting keys, HasTenantOverride, or config.setting_values.
metadata:
  author: moto-nrw
  version: "1.0.0"
---

# Tenant-Scoped Settings

The canonical resolution policy, file map, and add/edit/delete checklists are in
[the settings rule](../../../.claude/rules/settings-system.md). Read it before
settings work; keep implementation recipes there rather than duplicating them
in this skill.

## Workflow

1. Identify the setting definition, affected consumers, and tenant context.
   Read the relevant files from the rule's file map and existing tests.
2. Apply the rule's two-tier resolution: tenant override → registry default.
   Use typed `Resolve*` methods inside tenant middleware and `Resolve*ForTenant`
   outside it. Handle errors; no consumer-side env fallback or local default.
3. Follow the matching add/edit/delete checklist. Preserve permission checks,
   key constants, audit redaction, and migrations for renamed/deleted keys.
4. Verify override/default behavior and resolution failures in affected tests.
   Run the rule's verification commands and affected backend architecture checks.

## Diagnosis

Trace the definition, the correct tenant's stored value, and the consumer's
resolved result. A value equal to the registry default may still be an explicit
tenant choice. Check `HasTenantOverride` only when you need that distinction;
it is not a prerequisite for `Resolve*` and never authorizes an env lookup.

Treat existing env-fallback chains as migration debt under the canonical rule,
not as a pattern to copy. Investigate and report errors from resolution or
missing service wiring instead of substituting a value.
