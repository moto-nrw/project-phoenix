#!/bin/sh
set -e

# Copy SSL certificates from the mounted volume to PostgreSQL's data directory
if [ -d "/ssl-certs-source" ]; then
    echo "Copying SSL certificates..."
    mkdir -p /var/lib/postgresql/ssl
    cp -f /ssl-certs-source/* /var/lib/postgresql/ssl/ 2>/dev/null || true
    
    # Set proper permissions on SSL files
    chmod 600 /var/lib/postgresql/ssl/server.key 2>/dev/null || true
    chmod 644 /var/lib/postgresql/ssl/server.crt 2>/dev/null || true
    chmod 644 /var/lib/postgresql/ssl/ca.crt 2>/dev/null || true
    
    # Change ownership to postgres user
    chown -R postgres:postgres /var/lib/postgresql/ssl 2>/dev/null || true
    
    echo "SSL certificates copied and permissions set."
else
    echo "Warning: No SSL certificates found in /ssl-certs-source"
fi

# Call the original postgres entrypoint
exec docker-entrypoint.sh "$@"