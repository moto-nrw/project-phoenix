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
| Second factor per portal: a challenge carries its portal scope and is redeemable only there; the caterer's factor is voluntary, bound to its account and never to a school's `mfa_mode`, reset only by a moto operator | `docs/agents/contracts.md` Login, refresh, and MFA + ADR 0077 addendum (#4497) |
| Device auth: API key + staff PIN; IoT error codes are a PyrePortal contract | `docs/agents/contracts.md` Ecosystem and IoT |
| GDPR logging: no student names at Info level; retention via `gdpr.*` settings; deletions audited | `backend/CLAUDE.md` GDPR/Privacy Patterns |
| Secrets: never in source; SOPS for deployed envs; gitleaks/lefthook/CI guards | `.claude/rules/security/hardcoded-credentials.md` + `docs/agents/operations.md` Environment Management (SOPS) |
| Never hit production APIs | `.claude/rules/no-production-requests.md` |

## Cryptography — banned algorithms (NEVER use)

- **Hash**: MD2, MD4, MD5, SHA-0, SHA-1
- **Symmetric**: RC2, RC4, Blowfish, DES, 3DES, AES-CBC, AES-ECB
- **Signature**: RSA with PKCS#1 v1.5 padding
- **Key exchange**: static RSA, anonymous Diffie-Hellman, DHE with weak primes

**Use instead**: SHA-256+, AES-256-GCM, ChaCha20, ECDHE. Passwords/PINs: Argon2id (public helpers in `backend/modules/securityruntime/credentials.go`, backed by `backend/modules/securityruntime/internal/domain/userpass/` and bound to consumers at the root — reuse, never roll your own).

## Certificates

When touching X.509 material (`config/ssl/`, PEM strings, crypto calls): flag expired certs as CRITICAL; RSA < 2048 or EC < P-256 and MD5/SHA-1 signatures as high-priority; self-signed server certificates are dev-only. A private CA's self-signed trust anchor is not a self-signed server certificate.

DB transport: use `sslmode=require` for local development; use `verify-full`
with a trusted CA and matching server name for production TCP and external
diagnostics. A same-host alternative needs documented equivalent protection,
effective network/access evidence and a risk decision before affected processing;
a Docker bridge or loopback binding alone is insufficient. Existing deployed
`require`/`disable` declarations are open findings, not an approved exception.
When changing DB clients, networks, ports or certificates, follow the path-specific
[assessment and human verification](../../../docs/operations/db-transportschutz-3842.md).

## File uploads

New upload endpoints must validate like the existing canonical implementation in `backend/modules/dataimport/http/file_upload.go`: size limit, extension check AND content (magic-number) verification — never trust the extension alone.
