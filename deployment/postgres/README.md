# PostgreSQL SSL Setup

This directory contains files for configuring SSL in PostgreSQL for secure database connections.

## Development Setup

When setting up the development environment for the first time, you'll need to generate SSL certificates:

```bash
# From the repository root
cd deployment/postgres

# Run the certificate generation script to create self-signed certificates
./create-certs.sh
```

This script will create the necessary SSL certificates in the `ssl` directory:
- `ca.crt` - Certificate Authority certificate
- `ca.key` - Certificate Authority private key
- `server.crt` - Server certificate
- `server.key` - Server private key

## Important Notes

- The certificate files (*.crt, *.key, *.csr, *.srl) are excluded from version control in .gitignore
- Each developer needs to run the certificate generation script on their machine
- For production, use proper CA-signed certificates instead of self-signed ones

## Configuration

The PostgreSQL SSL configuration is defined in `ssl/postgresql.conf` and includes:
- SSL enabled
- Path configuration for certificate files
- Network configuration to listen on all interfaces

## Troubleshooting

If you encounter SSL connection issues:

1. Verify that certificates have been generated:
   ```bash
   ls -la ssl/
   ```

2. Check that PostgreSQL is using the SSL configuration:
   ```bash
   docker-compose logs postgres | grep ssl
   ```

3. If needed, regenerate the certificates:
   ```bash
   rm -f ssl/*.crt ssl/*.key ssl/*.csr ssl/*.srl
   ./create-certs.sh
   ```

## Production Considerations

For production environments:
- Use certificates signed by a trusted Certificate Authority
- Implement certificate rotation procedures
- Consider using `sslmode=verify-full` for stronger verification