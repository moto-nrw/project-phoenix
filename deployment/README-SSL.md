# PostgreSQL SSL Configuration

This directory contains files for configuring SSL in PostgreSQL for secure database connections.

## Setup Instructions

1. Generate SSL certificates:

```bash
# Navigate to the deployment/postgres directory
cd deployment/postgres

# Run the certificate generation script
./create-certs.sh
```

2. This will create the following files in the `deployment/postgres/ssl` directory:
   - `ca.crt` - Certificate Authority certificate
   - `ca.key` - Certificate Authority private key
   - `server.crt` - Server certificate
   - `server.key` - Server private key
   - `server.csr` - Certificate signing request (not needed after setup)

3. The Docker Compose files are already configured to:
   - Mount the certificates in the PostgreSQL container
   - Apply the PostgreSQL configuration to enable SSL
   - Set database connection strings to use `sslmode=require`

## Production Setup

For production environments, you should:

1. Use properly signed certificates instead of self-signed ones
2. Ensure certificate files have appropriate permissions (600 for key files)
3. Consider using `sslmode=verify-full` with proper CA verification

## Verifying SSL Connection

To verify that SSL is working properly:

```bash
# Connect to PostgreSQL using psql with SSL
psql "postgres://postgres:postgres@localhost:5432/postgres?sslmode=require"

# Check SSL status
\d
# Should show "SSL connection (protocol: TLSv1.3, cipher: TLS_AES_256_GCM_SHA384, bits: 256, compression: off)"
```

## Troubleshooting

If SSL connection fails:

1. Check PostgreSQL logs:
```bash
docker-compose logs postgres
```

2. Verify certificate permissions and paths
3. Test SSL connection with lower requirements first (`sslmode=prefer`)
4. Ensure the correct configuration file is being loaded

## Security Notes

- The configuration uses `sslmode=require` which encrypts traffic but doesn't verify certificates
- For full security, consider upgrading to `sslmode=verify-full` with proper CA verification
- Never expose database ports directly to the internet, even with SSL enabled
- Regularly rotate certificates following security best practices