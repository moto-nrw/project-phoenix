# Deployment Directory

This directory contains deployment configurations for different environments.

## Directory Structure

```
deployment/
├── production/              # Production deployment
│   ├── docker-compose.prod.yml  # Production Docker Compose file
│   ├── Caddyfile           # Caddy server configuration
│   ├── .env.example        # Production environment template
│   └── ssl/                # Production SSL files (not in git)
│       └── postgres/       # PostgreSQL SSL configuration
│           ├── postgresql.conf  # PostgreSQL SSL configuration
│           └── README.md   # Instructions for production SSL setup
└── ...                     # Other deployment environments
```

## Purpose

The deployment directory provides configuration specific to deploying the application in various environments. This includes Docker configurations, server setups, and environment-specific settings.

## Usage

- Development uses the root docker-compose.yml
- Production deployment uses files in the production directory
- SSL configurations specific to deployment environments are here
- Sensitive files (like certificates) should never be committed to git