# Error code registry

`error-registry.json` is the source of truth for API error identities. Each entry has:

- `code`: the stable identity on the wire, `bereich.fehlername` in lowercase. Do not rename or remove a code; add a new one instead.
- `class`: `input`, `permission`, `business_rejection`, `unavailable`, or `server`. The class describes what the user can do next, not just the HTTP status.
- `parameters`: names of structured `details` values that a translated message may interpolate. An empty array means the message takes no values.
- `legacy`: the wire code before the one-time rename (#2506), equal to `code` for codes added later. Phoenix no longer sends it. PyrePortal still accepts these old names when it reads a code.

The registry covers error codes emitted by the existing backend error helpers, domain faults that become API errors, and the IoT and substitution error responses. Success and warning codes and import-row validation codes are not API error identities. Some old codes are reused by several endpoints; they have one mapping. The original count in ADR 0006 was a point-in-time estimate, not a cap.

After editing the registry, run `node scripts/generate-error-codes.mjs`. Commit
the Go entry file `backend/api/common/error_codes.generated.go`, every numbered
`backend/api/common/error_codes_NNN.generated.go` shard, and
`frontend/src/lib/error-codes.generated.ts`. Regeneration deletes obsolete
numbered shards. The Go constants stay in the existing API contract package;
no new backend owner is needed. Chunks contain at most 300 codes and remain
inside the ordinary source ratchets (ADR 0087, #4746 addendum). CI runs
`node scripts/generate-error-codes.mjs --check` to reject missing, modified or
obsolete generated output without writing. `node --test
scripts/generate-error-codes.test.mjs` checks registry validity, chunking and
generated-file freshness.

Call sites never spell a code as a string. Backend handlers pass the generated `common.Code*` constant. A module package that may not import `api/common` declares its code once as a named constant with the registered value. `TestErrorPathsAnswerWithRegisteredCodes` (`backend/api/common/error_code_registry_test.go`) fails on a string at a call site, an unregistered code and a leftover old name. It checks every package-level function parameter named `code` or `fallbackCode`, `ErrResponse.Code`, `ErrorCode` methods, and the strings a code-returning function can return; a code only known at run time is outside it. `TestErrorWritesUseTheSharedEnvelope` fails on an error write outside the shared envelope (`http.Error`, an error body without numeric `status`, `detail` and `code`, or with `error`/`message`). In the frontend, compare a received code through `wireErrorCode()` from `~/lib/api-error` or a map typed by `ErrorCode`, so an unregistered literal fails the type check.

Backend message texts are developer diagnostics and may be English. German backend texts that no user sees any more are not maintained. The text travels in `detail`; the frontend shows the catalog text for the code. Display and envelope rules: `.claude/rules/error-path.md`. See ADR 0006 for the code identity and localization decision.
