# Database Permission Management

This document provides instructions on how to manage user permissions in Project Phoenix.

## Overview

The most reliable way to manage user permissions is to run SQL commands directly in the PostgreSQL Docker container. 

## Grant Permissions Using Docker

The following batch file can be used to grant all permissions to a user:

```bash
.\docker_grant_permissions.bat your.email@example.com
```

This will grant:
- Full admin access (`admin:*`)
- Full system access (`*:*`)
- All room-specific permissions (`rooms:create`, `rooms:read`, etc.)

## Grant Only Room Permissions

To grant only room permissions, you can edit the SQL script:

1. Open `add_admin_permissions.sql`
2. Comment out the sections for admin:* and *:* permissions
3. Run `.\docker_grant_permissions.bat your.email@example.com`

## How It Works

The system works by:
1. Creating a temporary SQL file with the user's email
2. Running it directly in the PostgreSQL Docker container
3. Verifying and displaying the granted permissions

## Technical Background

Permissions in Project Phoenix consist of:
- `resource`: The type of resource (e.g., "rooms", "users", "activities")
- `action`: The type of action (e.g., "create", "read", "update", "delete", "list", "manage")
- `name`: The combination of resource and action (e.g., "rooms:create")

The `admin:*` permission is a special wildcard that grants all permissions.

## Troubleshooting

If you encounter errors:
1. Make sure the PostgreSQL Docker container is running
2. Check that the user with the specified email exists in the database
3. Look at the SQL output for more detailed error messages

## File Structure

- `add_admin_permissions.sql`: SQL script for creating and granting permissions
- `docker_grant_permissions.bat`: Batch file for running the SQL script in Docker
- `temp_permissions.sql`: Temporary file created during execution (auto-deleted afterward)