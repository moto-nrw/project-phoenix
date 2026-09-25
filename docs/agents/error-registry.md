# Error code registry

`error-registry.json` is the source of truth for API error identities. Each entry has:

- `code`: the future stable identity, `bereich.fehlername` in lowercase. Once an identity is shipped, do not rename or remove it.
- `class`: `input`, `permission`, `business_rejection`, `unavailable`, or `server`. The class describes what the user can do next, not just the HTTP status.
- `parameters`: names of structured `details` values that a translated message may interpolate. An empty array means the message takes no values.
- `legacy`: the current wire code. The old-to-new mapping is kept for the later one-time rename; this step does not change any API response.

The registry covers error codes emitted by the existing backend error helpers, domain faults that become API errors, and the IoT and substitution error responses. Success and warning codes and import-row validation codes are not API error identities. Some old codes are reused by several endpoints; they have one mapping. The original count in ADR 0006 was a point-in-time estimate, not a cap.

After editing the registry, run `node scripts/generate-error-codes.mjs`. Commit both generated files: `backend/api/common/error_codes.generated.go` and `frontend/src/lib/error-codes.generated.ts`. The Go constants live in the existing API contract package so no new backend owner is needed. CI runs the generator and `git diff --exit-code` to catch drift. `node --test scripts/generate-error-codes.test.mjs` rejects duplicate codes and legacy mappings.

See ADR 0006 for the code identity and localization decision. This foundation deliberately leaves existing response codes and user-visible behavior unchanged.
