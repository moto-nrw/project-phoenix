#!/bin/bash
# Script to generate self-signed SSL certificates for PostgreSQL
# IMPORTANT: For production, use certificates from a trusted Certificate Authority
# This script is for development purposes only

# Set secure output directory that should be gitignored
OUTPUT_DIR="certs"

# Create directory structure if it doesn't exist
mkdir -p "$OUTPUT_DIR"
cd "$OUTPUT_DIR"

# Show warning
echo "WARNING: Generating self-signed certificates for DEVELOPMENT ONLY"
echo "These certificates are not suitable for production use!"
echo "OUTPUT DIRECTORY: $OUTPUT_DIR"

# Generate CA key and certificate
# Using 365 days (1 year) validity instead of 3650 days (10 years) for better security
openssl req -new -x509 -days 365 -nodes -out ca.crt -keyout ca.key -subj "/CN=postgres-ca"

# Generate server key
openssl genrsa -out server.key 2048

# Create a certificate signing request (CSR)
openssl req -new -key server.key -out server.csr -subj "/CN=postgres"

# Create a signed certificate for the server with Subject Alternative Names
openssl x509 -req -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out server.crt -days 365 \
    -extfile <(printf "subjectAltName=DNS:postgres,DNS:localhost,IP:127.0.0.1")

# Set proper permissions - restrictive for security
chmod 600 server.key ca.key
chmod 644 server.crt ca.crt

echo "-------------------------------------------"
echo "SSL certificates have been generated in $(pwd)"
echo ""
echo "SECURITY REMINDER:"
echo "1. These certificates are for DEVELOPMENT only"
echo "2. Keep private keys (.key files) secure"
echo "3. Never commit these files to version control"
echo "4. For production, use trusted CA certificates"
echo "5. These certificates will expire in 1 year (365 days)"
echo "6. Run this script again to regenerate expired certificates"
echo "-------------------------------------------"