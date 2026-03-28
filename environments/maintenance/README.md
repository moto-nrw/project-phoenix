# Maintenance Page

Static fallback page served by Caddy when backend/frontend containers are unreachable (HTTP 502/503).

## Setup on Server

### 1. Copy the maintenance page

```bash
mkdir -p /var/www/maintenance
scp environments/maintenance/maintenance.html moto-app-server:/var/www/maintenance/
```

### 2. Add `handle_errors` to Caddyfile

Add the following `handle_errors` block **inside each site block** in `/etc/caddy/Caddyfile`:

**For web frontends** (HTML maintenance page):

```caddyfile
# Production wildcard — *.moto-app.de
*.moto-app.de {
    tls {
        dns cloudflare {env.CLOUDFLARE_API_TOKEN}
    }

    # Maintenance page when frontend is down
    handle_errors {
        @maintenance expression `{err.status_code} in [502, 503]`
        handle @maintenance {
            root * /var/www/maintenance
            rewrite * /maintenance.html
            file_server
        }
    }

    reverse_proxy localhost:3000
}

# Staging wildcard — *.staging.moto-app.de
*.staging.moto-app.de {
    tls {
        dns cloudflare {env.CLOUDFLARE_API_TOKEN}
    }

    handle_errors {
        @maintenance expression `{err.status_code} in [502, 503]`
        handle @maintenance {
            root * /var/www/maintenance
            rewrite * /maintenance.html
            file_server
        }
    }

    reverse_proxy localhost:3001
}
```

**For API endpoints** (JSON 503 response):

```caddyfile
api.moto-app.de {
    handle_errors {
        @maintenance expression `{err.status_code} in [502, 503]`
        handle @maintenance {
            header Content-Type "application/json"
            respond `{"status":"maintenance","error":"System wird aktualisiert. Bitte versuchen Sie es in wenigen Minuten erneut."}` 503
        }
    }

    reverse_proxy localhost:8080
}

api-staging.moto-app.de {
    handle_errors {
        @maintenance expression `{err.status_code} in [502, 503]`
        handle @maintenance {
            header Content-Type "application/json"
            respond `{"status":"maintenance","error":"System wird aktualisiert. Bitte versuchen Sie es in wenigen Minuten erneut."}` 503
        }
    }

    reverse_proxy localhost:8081
}
```

### 3. Reload Caddy

```bash
systemctl reload caddy
```

### 4. Test

```bash
# Stop frontend to simulate downtime
cd ~/staging && docker compose stop frontend

# Should show maintenance page
curl -s https://altenberge.staging.moto-app.de | head -5

# Should return JSON 503
curl -s https://api-staging.moto-app.de/health

# Bring it back
docker compose up -d frontend
```

## How It Works

- Caddy stays running even when Docker containers are stopped (during deploys, restarts, etc.)
- When Caddy can't reach the upstream (port 3000/3001/8080/8081), it returns 502
- `handle_errors` catches 502/503 and serves the maintenance page instead
- The page auto-refreshes every 15 seconds, so users automatically return to the app when it comes back
- API endpoints return a JSON 503 so clients can handle it programmatically

## Updating the Page

Edit `maintenance.html` in this repo, commit, and SCP to the server:

```bash
scp environments/maintenance/maintenance.html moto-app-server:/var/www/maintenance/
```
