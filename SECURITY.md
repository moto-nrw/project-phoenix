# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| main    | :white_check_mark: |
| development | :white_check_mark: |

## Reporting a Vulnerability

We take the security of Project Phoenix seriously. If you discover a security vulnerability, please report it responsibly.

### How to Report

**Please do NOT report security vulnerabilities through public GitHub issues.**

Instead, please use one of the following methods:

1. **GitHub Security Advisories** (Preferred): Use the "Report a vulnerability" button in the Security tab of this repository to submit a private vulnerability report.

2. **Email**: Contact the maintainers directly if GitHub Security Advisories is not available.

### What to Include

When reporting a vulnerability, please include:

- **Description**: A clear description of the vulnerability
- **Impact**: The potential impact and severity
- **Steps to Reproduce**: Detailed steps to reproduce the issue
- **Affected Versions**: Which versions are affected
- **Suggested Fix**: If you have suggestions for how to fix the issue

### Response Timeline

- **Initial Response**: We will acknowledge receipt within 48 hours
- **Status Update**: We will provide a status update within 7 days
- **Resolution**: We aim to resolve critical vulnerabilities within 30 days

### Safe Harbor

We consider security research conducted in accordance with this policy to be:

- Authorized in accordance with any applicable anti-hacking laws
- Exempt from restrictions in our Terms of Service that would interfere with security research

We will not pursue civil or criminal action against researchers who:

- Follow this responsible disclosure policy
- Make good faith efforts to avoid privacy violations, data destruction, and service interruption
- Do not exploit vulnerabilities beyond what is necessary to demonstrate the issue

## Security Best Practices

### For Users

- **SSL/TLS**: Verify the transport mode for each environment and client path; see [Database Connections](#database-connections) and the open transport finding below.
- **Secrets**: Never commit real secrets, API keys, or credentials to the repository
- **Environment Files**: Use `.env` files for local configuration (they are gitignored)
- **Password Security**: Use strong passwords (minimum 8 characters, mixed case, numbers, special characters)

### For Contributors

- Run `golangci-lint` before submitting PRs to catch security issues
- Never hardcode secrets in code - use environment variables
- Validate all user input, especially in API endpoints
- Follow OWASP guidelines for web application security
- Review the [CLAUDE.md](CLAUDE.md) file for project-specific security patterns

## GDPR Compliance

This project handles student data. The following controls are implemented in code;
this is not a claim of legal compliance or verified production operation:

- **Data Minimization**: Only collect necessary data
- **Data Retention**: Automated cleanup of expired visit records
- **Access Control**: Role-based permissions restrict data access
- **Audit Logging**: All data deletions are logged
- **Tenant erasure**: Internal CLI for database and local files, with resumable
  phase evidence. External systems, backups and production verification remain
  human tasks; organization-root erasure is still open. See the
  [tenant erasure runbook](docs/operations/tenant-erasure.md).

See the privacy-related documentation in [CLAUDE.md](CLAUDE.md) for implementation details.

### Data in Transit Encryption

#### External Traffic (Public Internet)

All external traffic uses TLS/HTTPS encryption:

| Endpoint | Protocol | Certificate |
|----------|----------|-------------|
| `https://moto-app.de` | HTTPS/TLS 1.2+ | Valid SSL certificate |
| `https://api.moto-app.de` | HTTPS/TLS 1.2+ | Valid SSL certificate |

#### Internal Container Traffic (Risk Assessment)

The checked-in Compose environments use bridge networks, not Docker
`internal: true` networks. Frontend-to-backend traffic uses HTTP. Published
ports and other containers on the same network are additional access paths;
"Docker-internal" alone does not prove isolation or adequate transport protection.
Runtime firewall rules and host topology have not been verified by this document.

#### Database Connections

These are the modes in the checked-in configuration, not a production audit:

| Client path | Configured mode |
|-------------|-----------------|
| Production app and embedded worker | `sslmode=require`; PostgreSQL TLS is enabled, minimum TLS 1.2. No server identity verification is configured in the app DSN. |
| Staging and demo app | `sslmode=disable`; no database TLS is configured on these paths. |
| Local development app templates | `sslmode=require`, without server identity verification. The local test DSN uses `sslmode=disable`. |
| Migration/preflight and demo-runtime jobs | DSNs come from SOPS; their effective TLS modes cannot be established from the plaintext repository configuration. |
| Backup and restore in the PostgreSQL container | Local Unix socket, not TCP; TLS does not apply to that socket path. |

The [DB transport assessment and operational evidence](docs/operations/db-transportschutz-3842.md)
records the remaining finding, each client path, and the missing operational
proof. `require` demands encryption but does not verify server identity.
The documented remedy for TCP paths is `verify-full` with a trusted CA and a
matching server name, or a demonstrated equivalent same-host protection with
a recorded risk decision. This document does not approve the current transport
configuration or claim GDPR compliance.

## Acknowledgments

We appreciate the security community's efforts in helping keep Project Phoenix secure. Contributors who report valid security issues will be acknowledged (with permission) in our release notes.
