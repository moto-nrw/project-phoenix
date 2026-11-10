# Maintenance page

Caddy serves `maintenance.html` when an app upstream returns 502/503.
The API sites return a JSON 503 instead.

## Install or update the static page

Edit the page in this repository and commit it. Copy the committed file to the
app server's `/var/www/maintenance/maintenance.html` using the approved SSH
access. Keep the directory and file readable by Caddy.

```bash
scp environments/maintenance/maintenance.html moto-app-server:/var/www/maintenance/
```

## Change the proxy configuration

[`deploy/caddy/Caddyfile`](../../deploy/caddy/Caddyfile) is the source of truth.
It already defines and imports `maintenance-html` and `maintenance-json`.
Change that versioned file, run `node scripts/check-caddyfile.mjs`, and merge
the change into `main`. The `caddy-deploy.yml` workflow validates and deploys it.

Follow the [Caddy operations guide](../../docs/agents/operations.md#caddy-reverse-proxy)
for deployment, recovery and the manual fallback. Do not hand-edit the live
`/etc/caddy/Caddyfile`: deployment aborts if it does not match a version from
`main`. Bring any existing hand edit into the repository before retrying.

## Verification

In a disposable local environment, make the configured upstream unavailable
and verify the HTML fallback and API JSON 503, then restore that upstream.
Do not stop shared or deployed services to test this page.

The HTML page refreshes every 15 seconds so users return to the app once its
upstream is available again. Updating the static page alone needs no Caddy reload.
