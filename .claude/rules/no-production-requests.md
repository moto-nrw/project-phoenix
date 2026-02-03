# No Production API Requests

**ABSOLUTE RULE: Claude MUST NEVER execute HTTP requests (curl, wget, fetch, etc.) to any `moto-app.de` or `moto.nrw` domain.**

This includes but is not limited to:
- `api.moto-app.de`
- `*.moto-app.de`
- `*.moto.nrw`
- Any other production/staging moto domains

## What to do instead

1. **Always use `localhost`** when constructing API commands
2. If the user asks you to run a command against a moto-app address, **REFUSE** and explain:
   - "I cannot execute requests against production domains. Here is the command with localhost — please run it yourself if you need to target production."
3. **Warn the user** clearly if they are about to execute against a non-localhost address:
   - "WARNING: This targets a production/staging environment, not localhost. Proceed with caution."

## Why

- Production data integrity must be protected
- Accidental writes/mutations to production are irreversible
- Only humans should make the conscious decision to hit production endpoints
