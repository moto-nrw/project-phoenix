# Error code registry

`error-registry.json` is the source of truth for API error identities. Each entry has:

- `code`: the stable identity on the wire, `bereich.fehlername` in lowercase. Do not rename or remove a code; add a new one instead.
- `class`: `input`, `permission`, `business_rejection`, `unavailable`, or `server`. The class describes what the user can do next, not just the HTTP status.
- `parameters`: names of structured `details` values that a translated message may interpolate. An empty array means the message takes no values.
- `legacy`: the wire code before the one-time rename (#2506), equal to `code` for codes added later. Phoenix no longer sends it. PyrePortal reads the mapping until its text patterns are gone (#2508).

The registry covers error codes emitted by the existing backend error helpers, domain faults that become API errors, and the IoT and substitution error responses. Success and warning codes and import-row validation codes are not API error identities. Some old codes are reused by several endpoints; they have one mapping. The original count in ADR 0006 was a point-in-time estimate, not a cap.

After editing the registry, run `node scripts/generate-error-codes.mjs`. Commit both generated files: `backend/api/common/error_codes.generated.go` and `frontend/src/lib/error-codes.generated.ts`. The Go constants live in the existing API contract package so no new backend owner is needed. CI runs the generator and `git diff --exit-code` to catch drift. `node --test scripts/generate-error-codes.test.mjs` rejects duplicate codes and legacy mappings.

Call sites never spell a code as a string. Backend handlers pass the generated `common.Code*` constant. A module package that may not import `api/common` declares its code once as a named constant with the registered value. `TestErrorPathsAnswerWithRegisteredCodes` (`backend/api/common/error_code_registry_test.go`) fails on a string at a call site, an unregistered code and a leftover old name. It checks every package-level function parameter named `code` or `fallbackCode` (observability metric labels excepted), `ErrResponse.Code`, `ErrorCode` methods, and the strings a code-returning function can return; a code only known at run time is outside it. In the frontend, compare a received code through `wireErrorCode()` from `~/lib/api-error` or a map typed by `ErrorCode`, so an unregistered literal fails the type check.

Backend message texts are developer diagnostics and may be English. German backend texts that no user sees any more are not maintained. Old display paths that still show the backend text remain until #2520. See ADR 0006 for the code identity and localization decision.
