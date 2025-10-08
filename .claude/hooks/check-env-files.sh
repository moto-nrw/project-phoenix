#!/bin/bash

# Check for required environment files
missing_files=()

if [ ! -f ".env" ]; then
  missing_files+=(".env")
fi

if [ ! -f "backend/dev.env" ]; then
  missing_files+=("backend/dev.env")
fi

if [ ! -f "frontend/.env.local" ]; then
  missing_files+=("frontend/.env.local")
fi

# Check SSL certificates
if [ ! -f "config/ssl/postgres/certs/server.crt" ]; then
  echo "⚠ SSL certificates not found"
  echo "  Run: cd config/ssl/postgres && ./create-certs.sh"
fi

if [ ${#missing_files[@]} -gt 0 ]; then
  echo "⚠ Missing environment files:"
  for file in "${missing_files[@]}"; do
    echo "  - $file (copy from $file.example)"
  done
  echo ""
  echo "Quick setup: ./scripts/setup-dev.sh"
fi

# Always exit 0 (don't block session start)
exit 0
