#!/bin/sh
set -e

# Copy SSL certificates from the mounted volume to PostgreSQL's data directory
if [ -d "/ssl-certs-source" ]; then
    echo "Setting up SSL certificates..."
    mkdir -p /var/lib/postgresql/ssl
    
    # Copy certificates to PostgreSQL expected locations
    cp -f /ssl-certs-source/server.crt /var/lib/postgresql/ssl/server.crt
    cp -f /ssl-certs-source/server.key /var/lib/postgresql/ssl/server.key
    cp -f /ssl-certs-source/ca.crt /var/lib/postgresql/ssl/ca.crt
    
    # Set proper permissions
    chown -R postgres:postgres /var/lib/postgresql/ssl
    chmod 700 /var/lib/postgresql/ssl
    chmod 700 /var/lib/postgresql/ssl/certs
    chmod 600 /var/lib/postgresql/ssl/certs/server.key
    chmod 644 /var/lib/postgresql/ssl/certs/server.crt
    chmod 644 /var/lib/postgresql/ssl/certs/ca.crt
    
    echo "SSL certificates configured successfully"
else
    echo "Warning: No SSL certificates found in /ssl-certs-source"
fi

# Call the original postgres entrypoint
exec docker-entrypoint.sh "$@"