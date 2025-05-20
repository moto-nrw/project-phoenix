#!/bin/bash
# Development environment setup script for Project Phoenix
# This script sets up a secure development environment by generating
# configuration files from templates and generating SSL certificates.

# Exit on error
set -e

echo "==============================================="
echo "Project Phoenix - Development Environment Setup"
echo "==============================================="

# Check if running from repository root
if [ ! -f "docker-compose.example.yml" ]; then
  echo "Error: This script must be run from the repository root"
  exit 1
fi

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Function to generate random string for secrets
generate_secret() {
  openssl rand -base64 32 | tr -d '\n'
}

# 1. Creating environment files from templates
echo -e "\n${YELLOW}Creating environment files from templates...${NC}"

# 1.1 Main .env file
if [ ! -f ".env" ]; then
  echo "Creating .env file..."
  cp .env.example .env
  
  # Generate secure random secrets
  JWT_SECRET=$(generate_secret)
  NEXTAUTH_SECRET=$(generate_secret)
  DB_PASSWORD=$(openssl rand -base64 16 | tr -d '\n')
  
  # Replace placeholders with generated values
  sed -i.bak "s/your_secure_db_password/$DB_PASSWORD/g" .env
  sed -i.bak "s/your_jwt_secret_at_least_32_chars_long/$JWT_SECRET/g" .env
  sed -i.bak "s/your_nextauth_secret_at_least_32_chars/$NEXTAUTH_SECRET/g" .env
  rm .env.bak
  echo -e "${GREEN}Created .env file with secure random secrets${NC}"
else
  echo -e "${YELLOW}Skipping .env - file already exists${NC}"
fi

# 1.2 Backend dev.env file
if [ ! -f "backend/dev.env" ]; then
  echo "Creating backend/dev.env file..."
  cp backend/dev.env.example backend/dev.env
  
  # Use the same JWT secret for consistency
  grep -q "AUTH_JWT_SECRET" .env && JWT_SECRET=$(grep "AUTH_JWT_SECRET" .env | cut -d'=' -f2)
  
  # Replace placeholders
  sed -i.bak "s/your_jwt_secret_at_least_32_chars_long/$JWT_SECRET/g" backend/dev.env
  sed -i.bak "s/your_secure_admin_password/admin_dev_password/g" backend/dev.env
  rm backend/dev.env.bak
  echo -e "${GREEN}Created backend/dev.env file${NC}"
else
  echo -e "${YELLOW}Skipping backend/dev.env - file already exists${NC}"
fi

# 1.3 Frontend .env.local file
if [ ! -f "frontend/.env.local" ]; then
  echo "Creating frontend/.env.local file..."
  cp frontend/.env.local.example frontend/.env.local
  
  # Use the same NextAuth secret for consistency
  grep -q "NEXTAUTH_SECRET" .env && NEXTAUTH_SECRET=$(grep "NEXTAUTH_SECRET" .env | cut -d'=' -f2)
  
  # Replace placeholders
  sed -i.bak "s/your_nextauth_secret/$NEXTAUTH_SECRET/g" frontend/.env.local
  sed -i.bak "s/your_auth_secret_key/$NEXTAUTH_SECRET/g" frontend/.env.local
  rm frontend/.env.local.bak
  echo -e "${GREEN}Created frontend/.env.local file${NC}"
else
  echo -e "${YELLOW}Skipping frontend/.env.local - file already exists${NC}"
fi

# 1.4 Docker compose file
if [ ! -f "docker-compose.yml" ]; then
  echo "Creating docker-compose.yml file..."
  cp docker-compose.example.yml docker-compose.yml
  echo -e "${GREEN}Created docker-compose.yml file${NC}"
else
  echo -e "${YELLOW}Skipping docker-compose.yml - file already exists${NC}"
fi

# 2. Generate SSL certificates for development
echo -e "\n${YELLOW}Setting up SSL certificates...${NC}"

# Check if certificates already exist
if [ ! -d "config/ssl/postgres/certs" ] || [ ! -f "config/ssl/postgres/certs/server.crt" ]; then
  echo "Generating PostgreSQL SSL certificates..."
  cd config/ssl/postgres
  chmod +x create-certs.sh
  ./create-certs.sh
  cd ../../..
  echo -e "${GREEN}SSL certificates generated${NC}"
else
  echo -e "${YELLOW}Skipping SSL certificate generation - certificates already exist${NC}"
fi

# 3. Check Docker and Docker Compose
echo -e "\n${YELLOW}Checking Docker and Docker Compose installation...${NC}"

# Check for Docker
if ! command -v docker &> /dev/null; then
  echo -e "${RED}Error: Docker is not installed. Please install Docker first.${NC}"
  echo "Visit https://docs.docker.com/get-docker/ for installation instructions."
  exit 1
fi

# Check for Docker Compose - supports both V2 (built into Docker) and V1 (standalone)
DOCKER_COMPOSE_FOUND=false

# Try Docker Compose V2 (docker compose)
if docker compose version &> /dev/null; then
  DOCKER_COMPOSE_FOUND=true
  echo -e "${GREEN}Docker Compose V2 detected${NC}"
# Try Docker Compose V1 (docker-compose)
elif command -v docker-compose &> /dev/null; then
  DOCKER_COMPOSE_FOUND=true
  echo -e "${GREEN}Docker Compose V1 detected${NC}"
fi

if [ "$DOCKER_COMPOSE_FOUND" = false ]; then
  echo -e "${RED}Error: Docker Compose is not installed. Please install Docker Compose first.${NC}"
  echo "Visit https://docs.docker.com/compose/install/ for installation instructions."
  exit 1
fi

echo -e "${GREEN}Docker and Docker Compose are properly installed${NC}"

# 4. Setup complete
echo -e "\n${GREEN}Development environment setup complete!${NC}"
echo -e "You can now start the development environment with: ${YELLOW}docker-compose up -d${NC}"
echo -e "Access the application at: ${YELLOW}http://localhost:3000${NC}"
echo -e "API will be available at: ${YELLOW}http://localhost:8080${NC}"

echo -e "\n${YELLOW}Security Reminder:${NC}"
echo "- NEVER commit .env, dev.env, or .env.local files to Git"
echo "- Keep SSL certificates out of version control"
echo "- For more information, read docs/security.md"

exit 0