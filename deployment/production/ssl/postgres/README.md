# Production PostgreSQL SSL Setup

This directory should contain production SSL certificates for PostgreSQL. For security reasons, these files are not included in the repository and must be provided during deployment.

## Required Files

For production deployment, you should place the following files in this directory:

```
postgres/
├── postgresql.conf          # PostgreSQL SSL configuration
└── certs/                   # Certificate directory
    ├── server.crt           # Server certificate (signed by a trusted CA)
    ├── server.key           # Server private key (keep secure!)
    └── ca.crt               # CA certificate
```

## Production Certificate Options

For production, you should use one of the following:

1. **Certificates signed by a public Certificate Authority (CA)**
   - More secure, trusted by default by clients
   - Requires domain name for the database (even for internal use)
   - Example providers: Let's Encrypt, DigiCert, Comodo

2. **Certificates signed by your organization's internal CA**
   - Good for internal applications
   - Requires distributing CA certificate to clients
   - Provides good security with proper management

3. **Self-signed certificates (not recommended for production)**
   - If you must use self-signed certificates in production, generate them with:
   - Strong key (4096-bit RSA minimum)
   - Long validity period with regular rotation
   - Proper subject information

## Configuration

The PostgreSQL configuration in docker-compose.prod.yml already mounts this directory to the correct location in the container.

Use `sslmode=verify-full` in your connection strings for maximum security in production.