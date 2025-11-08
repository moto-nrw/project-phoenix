# SSL Configuration

This directory contains SSL certificates and configuration for various services
in the project.

## Directory Structure

```
ssl/
├── postgres/               # PostgreSQL SSL configuration
│   ├── certs/              # Generated certificates (not in git)
│   │   ├── ca.crt          # Certificate Authority certificate
│   │   ├── ca.key          # Certificate Authority private key
│   │   ├── server.crt      # Server certificate
│   │   ├── server.key      # Server private key
│   │   └── ...             # Other certificate files
│   ├── create-certs.sh     # Script to generate self-signed certificates
│   ├── postgresql.conf     # PostgreSQL SSL configuration
│   └── README.md           # PostgreSQL SSL documentation
└── ...                     # Other service SSL configurations
```

## Security Notes

- Certificate files (_.crt, _.key, _.csr, _.srl) are excluded from git
- Each developer must generate their own certificates for development
- Production should use proper CA-signed certificates
- Never commit sensitive cryptographic material to the repository
