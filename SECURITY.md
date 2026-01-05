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

- **SSL/TLS**: Always use SSL-encrypted database connections (`sslmode=require` or higher)
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

This project handles student data and implements GDPR-compliant data handling:

- **Data Minimization**: Only collect necessary data
- **Data Retention**: Automated cleanup of expired visit records
- **Access Control**: Role-based permissions restrict data access
- **Audit Logging**: All data deletions are logged
- **Right to Erasure**: Hard delete functionality for student data

See the privacy-related documentation in [CLAUDE.md](CLAUDE.md) for implementation details.

### Data in Transit Encryption

#### External Traffic (Public Internet)

All external traffic uses TLS/HTTPS encryption:

| Endpoint | Protocol | Certificate |
|----------|----------|-------------|
| `https://moto-app.de` | HTTPS/TLS 1.2+ | Valid SSL certificate |
| `https://api.moto-app.de` | HTTPS/TLS 1.2+ | Valid SSL certificate |

#### Internal Container Traffic (Risk Assessment)

**Architecture**: Frontend and backend containers communicate via HTTP within Docker's internal network.

**Risk Assessment per GDPR Article 32**:

| Factor | Assessment |
|--------|------------|
| **Network Isolation** | Docker internal network is isolated from external access; traffic does not traverse public networks |
| **Host Boundary** | All container traffic remains within a single host machine |
| **External Exposure** | None - internal network not routable from outside |
| **Data Sensitivity** | Student PII (names, attendance) - medium sensitivity |
| **Threat Model** | Attacker would need host-level access (root/Docker socket) |
| **Compensating Controls** | UFW firewall (deny incoming by default), SSH access, Docker socket not exposed on TCP |

**Conclusion**: Internal HTTP traffic is acceptable under GDPR Article 32's risk-based approach because:

1. Traffic is isolated within Docker's internal network
2. No data traverses public or untrusted networks
3. Host-level security controls provide the security boundary
4. The cost/benefit of internal TLS does not justify the complexity for single-host deployments

**Reference**: [GDPR Article 32](https://gdpr-info.eu/art-32-gdpr/) requires "appropriate technical measures" based on risk assessment, not absolute encryption requirements.

#### Database Connections

PostgreSQL connections use SSL encryption:

```bash
# Connection string requires SSL
DB_DSN=postgres://user:pass@host:5432/db?sslmode=require
```

- Development: `sslmode=require` (encrypted, no cert verification)
- Production: `sslmode=require` with valid certificates

## Acknowledgments

We appreciate the security community's efforts in helping keep Project Phoenix secure. Contributors who report valid security issues will be acknowledged (with permission) in our release notes.
