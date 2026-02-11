---
name: optimizing-form-accessibility
description: Use when optimizing forms for keyboard navigation, focus management, or accessibility. Triggers on login pages, signup forms, checkout flows, or any form where tab order and focus behavior need improvement.
metadata:
  author: moto-nrw
  version: "1.0.0"
---

# Optimizing Form Accessibility

Optimize forms for keyboard navigation and accessibility using current best practices.

## How It Works

1. Fetch current accessibility guidelines from authoritative sources
2. Search for framework-specific patterns (React, Vue, Angular, etc.)
3. Read the form code to optimize
4. Apply fixes based on fetched guidelines
5. Verify with project's quality checks

## Guidelines Sources

**Always fetch fresh guidelines before making changes:**

### Primary Source
```
https://raw.githubusercontent.com/vercel-labs/web-interface-guidelines/main/command.md
```
Use WebFetch with prompt: "Extract all rules related to forms, keyboard navigation, focus management, and accessibility"

### Framework-Specific Search
```
WebSearch: "[framework] form autofocus tabindex keyboard navigation accessibility [current year]"
```
Replace `[framework]` with the project's framework (React, Next.js, Vue, etc.)

### Reference Sources
- MDN Accessibility: `developer.mozilla.org/en-US/docs/Web/Accessibility`
- web.dev Focus: `web.dev/control-focus-with-tabindex/`

## Usage

When a user reports form accessibility issues:

1. **Fetch** current guidelines using WebFetch on the primary source
2. **Search** for framework-specific patterns using WebSearch with current year
3. **Read** the form files the user wants to optimize
4. **Apply** fixes based on the fetched guidelines (not hardcoded rules)
5. **Verify** using project's lint/typecheck commands

## Why Dynamic?

Web accessibility standards evolve:
- CSS pseudo-classes change (`:focus` → `:focus-visible`)
- Framework patterns update (React 19, Next.js 15+)
- Browser support improves

**Never rely on cached knowledge - always fetch current guidelines.**
