# Configuration Directory

This directory contains configuration files for various components of the
system.

## Directory Structure

```
config/
├── ssl/                    # SSL certificates and configuration
│   ├── README.md           # SSL documentation
│   └── postgres/           # PostgreSQL SSL configuration
│       ├── certs/          # Generated certificates (not in git)
│       ├── create-certs.sh # Script to generate certificates
│       ├── postgresql.conf # PostgreSQL SSL settings
│       └── README.md       # PostgreSQL-specific instructions
└── ...                     # Other configuration categories
```

## Purpose

The config directory provides a central location for all configuration files,
separate from application code and deployment scripts. This separation follows
the principle that configuration should be maintained independently from code.

## Usage

- Development configuration is stored here and used directly
- Scripts for generating development configuration are included
- Sensitive files (like certificates) are excluded from git
- Each developer needs to generate their own sensitive configuration
- For production, refer to the deployment directory
