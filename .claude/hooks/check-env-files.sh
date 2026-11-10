#!/bin/bash
set -euo pipefail

# Check for required environment files
missing_files=()

if [[ ! -f ".env" ]]; then
  missing_files+=(".env (copy from .env.example)")
fi

if [[ ! -f "backend/dev.env" ]]; then
  missing_files+=("backend/dev.env (copy from backend/dev.env.example)")
fi

if [[ ! -f "frontend/.env.local" ]]; then
  missing_files+=("frontend/.env.local (copy from frontend/.env.example)")
fi

# Check SSL certificates
if [[ ! -f "config/ssl/postgres/certs/server.crt" ]]; then
  echo "⚠ SSL certificates not found" >&2
  echo "  Run: cd config/ssl/postgres && ./create-certs.sh" >&2
fi

if [[ ${#missing_files[@]} -gt 0 ]]; then
  echo "⚠ Missing environment files:" >&2
  for file in "${missing_files[@]}"; do
    echo "  - ${file}" >&2
  done
  echo "" >&2
  echo "Quick setup: ./scripts/setup-dev.sh" >&2
fi

# Always exit 0 (don't block session start)
exit 0
