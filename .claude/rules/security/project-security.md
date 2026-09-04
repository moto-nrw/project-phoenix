---
paths:
  - "backend/**/*.go"
  - "frontend/src/**/*.ts"
  - "frontend/src/**/*.tsx"
---

# Project Security Invariants

The project-specific security reference. The load-bearing rules live where they are enforced — this file indexes them and holds the few items with no other home.

## The invariants (and where they are documented)

| Invariant | Canonical source |
|---|---|
| Tenant isolation: RLS + `phoenix_tenant` role per request; a forgotten `WHERE tenant_id = ?` is a cross-tenant data leak — queries belong in repositories | `docs/agents/contracts.md` Tenant boundary + `.claude/rules/backend-conventions.md` Rule 11 (CI-ratcheted) |
| Portal session isolation: per-portal cookies + JWT `scope` claim; middleware rejects out-of-scope tokens | `docs/agents/contracts.md` Portal session isolation |
| Device auth: API key + staff PIN; IoT error strings are a PyrePortal contract | `docs/agents/contracts.md` Ecosystem and IoT |
| GDPR logging: no student names at Info level; retention via `gdpr.*` settings; deletions audited | `backend/CLAUDE.md` GDPR/Privacy Patterns |
| Secrets: never in source; SOPS for deployed envs; gitleaks/lefthook/CI guards | `.claude/rules/security/hardcoded-credentials.md` + `docs/agents/operations.md` Environment Management (SOPS) |
| Never hit production APIs | `.claude/rules/no-production-requests.md` |

## Cryptography — banned algorithms (NEVER use)

- **Hash**: MD2, MD4, MD5, SHA-0, SHA-1
- **Symmetric**: RC2, RC4, Blowfish, DES, 3DES, AES-CBC, AES-ECB
- **Signature**: RSA with PKCS#1 v1.5 padding
- **Key exchange**: static RSA, anonymous Diffie-Hellman, DHE with weak primes

**Use instead**: SHA-256+, AES-256-GCM, ChaCha20, ECDHE. Passwords/PINs: Argon2id (existing helpers in `backend/auth/userpass/` and `services/auth/password_helpers.go` — reuse, never roll your own).

## Certificates

When touching X.509 material (`config/ssl/`, PEM strings, crypto calls): flag expired certs as CRITICAL; RSA < 2048 or EC < P-256 and MD5/SHA-1 signatures as high-priority; self-signed (issuer == subject) is dev-only. DB connections: `sslmode=require` in dev, `verify-full` in production.

## File uploads

New upload endpoints must validate like the existing canonical implementation in `backend/api/import/file_upload.go`: size limit, extension check AND content (magic-number) verification — never trust the extension alone.
