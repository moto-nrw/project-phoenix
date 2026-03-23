#!/usr/bin/env bash
# ==============================================================================
# Project Phoenix — Interactive Development Environment Setup
#
# Creates all git-ignored config files from templates, generates SSL certs,
# and prints credentials at the end. Idempotent: existing files are never
# overwritten (delete them first if you want to re-run).
#
# Usage:  ./scripts/setup-dev.sh
# ==============================================================================

set -euo pipefail

# ---------- colours (no-op when stdout is not a terminal) --------------------
if [[ -t 1 ]]; then
  GREEN='\033[0;32m'
  YELLOW='\033[1;33m'
  RED='\033[0;31m'
  BOLD='\033[1m'
  DIM='\033[2m'
  NC='\033[0m'
else
  GREEN='' YELLOW='' RED='' BOLD='' DIM='' NC=''
fi

# ---------- helpers ----------------------------------------------------------
info()  { printf "${GREEN}✓${NC} %s\n" "$*"; }
warn()  { printf "${YELLOW}⚠${NC} %s\n" "$*"; }
err()   { printf "${RED}✗${NC} %s\n" "$*" >&2; }
header() { printf "\n${BOLD}[%s]${NC} %s\n" "$1" "$2"; }

generate_secret() { openssl rand -base64 32 | tr -d '/+\n' | cut -c1-44; }
generate_password() { openssl rand -base64 16 | tr -d '/+\n' | cut -c1-22; }

# Prompt with default. Usage: ask "Label" "default_value"
# Sets REPLY to the user's answer (or default on empty input).
ask() {
  local label="$1" default="$2"
  printf "  %s ${DIM}[%s]${NC}: " "$label" "$default"
  read -r REPLY
  REPLY="${REPLY:-$default}"
}

# Prompt yes/no with default Y. Usage: ask_yn "Question?"
# Returns 0 (true) for yes, 1 (false) for no.
ask_yn() {
  local label="$1"
  printf "  %s ${DIM}[Y/n]${NC}: " "$label"
  read -r REPLY
  [[ "${REPLY:-Y}" =~ ^[Yy]$ ]]
}

# Replace a placeholder value in a file. Pure bash, no sed.
# Usage: replace_value FILE "PLACEHOLDER" "NEW_VALUE"
replace_value() {
  local file="$1" placeholder="$2" new_value="$3"
  local tmp="${file}.tmp.$$"
  while IFS= read -r line; do
    printf '%s\n' "${line//$placeholder/$new_value}"
  done < "$file" > "$tmp"
  mv "$tmp" "$file"
}

# ---------- pre-flight checks -----------------------------------------------
if [[ ! -f "docker-compose.example.yml" ]]; then
  err "This script must be run from the repository root."
  exit 1
fi

echo ""
echo "==========================================="
echo " Project Phoenix — Dev Environment Setup"
echo "==========================================="

# ---------- Step 1: Collect credentials --------------------------------------
header "1/4" "Database"

if ask_yn "Generate a random database password?"; then
  DB_PASSWORD="$(generate_password)"
  info "Generated database password"
else
  ask "Database password" "postgres"
  DB_PASSWORD="$REPLY"
fi

header "2/3" "Operator Account (platform admin)"

ask "Email" "operator@example.com"
OPERATOR_EMAIL="$REPLY"

if ask_yn "Generate a random operator password?"; then
  OPERATOR_PASSWORD="$(generate_password)"
  info "Generated operator password"
else
  ask "Password" ""
  OPERATOR_PASSWORD="$REPLY"
  if [[ -z "$OPERATOR_PASSWORD" ]]; then
    OPERATOR_PASSWORD="$(generate_password)"
    warn "Empty password — generated one instead"
  fi
fi

OPERATOR_DISPLAY_NAME="Administrator"

# Secrets the user never needs to choose
JWT_SECRET="$(generate_secret)"
NEXTAUTH_SECRET="$(generate_secret)"
PHOENIX_AUTH_PASSWORD="$(generate_password)"

# ---------- Step 2: Create config files --------------------------------------
header "3/3" "Creating config files"

FILES_CREATED=()

# --- .env (root) ---
if [[ -f ".env" ]]; then
  warn "Skipping .env — already exists"
else
  cp .env.example .env
  replace_value .env "your_secure_db_password"              "$DB_PASSWORD"
  replace_value .env "your_jwt_secret_at_least_32_chars_long" "$JWT_SECRET"
  replace_value .env "your_nextauth_secret_at_least_32_chars" "$NEXTAUTH_SECRET"
  replace_value .env "OPERATOR_EMAIL=operator@example.com"  "OPERATOR_EMAIL=$OPERATOR_EMAIL"
  replace_value .env "your_secure_operator_password"        "$OPERATOR_PASSWORD"
  replace_value .env "OPERATOR_DISPLAY_NAME=Administrator"  "OPERATOR_DISPLAY_NAME=$OPERATOR_DISPLAY_NAME"
  replace_value .env "PHOENIX_AUTH_PASSWORD=phoenix_auth_dev" "PHOENIX_AUTH_PASSWORD=$PHOENIX_AUTH_PASSWORD"
  FILES_CREATED+=(".env")
  info "Created .env"
fi

# --- docker-compose.yml ---
if [[ -f "docker-compose.yml" ]]; then
  warn "Skipping docker-compose.yml — already exists"
else
  cp docker-compose.example.yml docker-compose.yml
  FILES_CREATED+=("docker-compose.yml")
  info "Created docker-compose.yml"
fi

# --- backend/dev.env ---
if [[ -f "backend/dev.env" ]]; then
  warn "Skipping backend/dev.env — already exists"
else
  cp backend/dev.env.example backend/dev.env
  replace_value backend/dev.env "your_jwt_secret_at_least_32_chars_long" "$JWT_SECRET"
  replace_value backend/dev.env "your_db_password"                       "$DB_PASSWORD"
  replace_value backend/dev.env "OPERATOR_EMAIL=operator@example.com"    "OPERATOR_EMAIL=$OPERATOR_EMAIL"
  replace_value backend/dev.env "your_secure_operator_password"          "$OPERATOR_PASSWORD"
  replace_value backend/dev.env "OPERATOR_DISPLAY_NAME=Administrator"    "OPERATOR_DISPLAY_NAME=$OPERATOR_DISPLAY_NAME"
  replace_value backend/dev.env "PHOENIX_AUTH_PASSWORD=phoenix_auth_dev"  "PHOENIX_AUTH_PASSWORD=$PHOENIX_AUTH_PASSWORD"
  FILES_CREATED+=("backend/dev.env")
  info "Created backend/dev.env"
fi

# --- frontend/.env.local ---
if [[ -f "frontend/.env.local" ]]; then
  warn "Skipping frontend/.env.local — already exists"
else
  cp frontend/.env.example frontend/.env.local
  replace_value frontend/.env.local 'your_nextauth_secret'   "$NEXTAUTH_SECRET"
  replace_value frontend/.env.local 'your_auth_secret_key'   "$NEXTAUTH_SECRET"
  FILES_CREATED+=("frontend/.env.local")
  info "Created frontend/.env.local"
fi

# ---------- Step 3: SSL certificates ----------------------------------------
echo ""
if [[ -f "config/ssl/postgres/certs/server.crt" ]]; then
  warn "Skipping SSL certificates — already exist"
else
  echo "  Generating PostgreSQL SSL certificates..."
  (cd config/ssl/postgres && chmod +x create-certs.sh && ./create-certs.sh) > /dev/null 2>&1
  info "Generated SSL certificates"
fi

# ---------- Step 4: Docker check --------------------------------------------
echo ""
if ! command -v docker &> /dev/null; then
  err "Docker is not installed. Install it from https://docs.docker.com/get-docker/"
  exit 1
fi

if docker compose version &> /dev/null; then
  info "Docker Compose V2 detected"
elif command -v docker-compose &> /dev/null; then
  info "Docker Compose V1 detected"
else
  err "Docker Compose is not installed. Install it from https://docs.docker.com/compose/install/"
  exit 1
fi

# ---------- Summary ----------------------------------------------------------
echo ""
echo "==========================================="
echo " Setup Complete!"
echo "==========================================="
echo ""

if [[ ${#FILES_CREATED[@]} -gt 0 ]]; then
  printf "${BOLD}Files created:${NC}\n"
  for f in "${FILES_CREATED[@]}"; do
    echo "  • $f"
  done
  echo ""
fi

printf "${BOLD}Your credentials:${NC}\n"
echo ""
printf "  ${DIM}Database${NC}\n"
echo "  Password:          $DB_PASSWORD"
echo ""
printf "  ${DIM}Operator (platform admin)${NC}\n"
echo "  Email:             $OPERATOR_EMAIL"
echo "  Password:          $OPERATOR_PASSWORD"
echo ""
printf "  ${YELLOW}⚠ Save these credentials! They won't be shown again.${NC}\n"
echo ""
printf "${BOLD}Next steps:${NC}\n"
echo "  1. docker compose up -d"
echo "  2. docker compose run server go run . seed \\"
echo "       --email $OPERATOR_EMAIL \\"
echo "       --password '$OPERATOR_PASSWORD' \\"
echo "       --pin 1234 \\"
echo "       --url http://server:8080"
echo ""
echo "  3. Operator dashboard: http://operator.localhost:3000"
echo "  4. Tenant login:       http://localhost:3000"
echo "     (use any staff account from the seeder output)"
echo ""
printf "  ${DIM}SMTP is not configured — email features (invitations, password${NC}\n"
printf "  ${DIM}reset) won't work. That's normal for local development.${NC}\n"
echo ""
