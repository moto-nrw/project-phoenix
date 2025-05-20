#!/bin/bash
# Script to generate self-signed SSL certificates for PostgreSQL

# Create directory structure
mkdir -p ssl
cd ssl

# Generate CA key and certificate
openssl req -new -x509 -days 3650 -nodes -out ca.crt -keyout ca.key -subj "/CN=postgres-ca"

# Generate server key
openssl genrsa -out server.key 2048

# Create a certificate signing request (CSR)
openssl req -new -key server.key -out server.csr -subj "/CN=postgres"

# Create a signed certificate for the server
openssl x509 -req -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out server.crt -days 3650

# Set proper permissions
chmod 600 server.key ca.key
chmod 644 server.crt ca.crt

echo "SSL certificates have been generated"
echo "Place them in a volume mount for PostgreSQL"