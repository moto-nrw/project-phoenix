# Authored SQL contracts

Each scenario has one owner/resource-local JSON file. These are reviewed policy,
not measurements to regenerate. `max` is a ceiling; `exact` additionally proves
the guarded read still executes. Preserve scenario identity when moving a file.

The embedded loader discovers files recursively and rejects duplicate scenario
identities, duplicate JSON fields, unknown/missing fields and negative ceilings.
`TestQueryBudgetRatchet` still checks global test references and rejects local
query hooks. The architecture check with `--base-ref <SHA>` reads the complete
immutable base tree, including historical Go-map registers, and rejects missing
contracts, larger maxima and removal of exactness.
HTTP contracts also carry `route`, the final chi `METHOD /pattern`, including
mount prefixes and parameter names. For multiple measured requests `route` is
a sorted, deduplicated JSON array; ordinary one-request budgets use a string.
Every member must remain in `api/testdata/routes/`. Interior `/*/` mount
artifacts from `chi.Walk` are normalized for the comparison; real terminal
wildcards and trailing leaf slashes are preserved.
Pass `counter.Measurement(statements)` after the request returns, not the bare
SQL slice. Keep this snapshot together when a helper returns SQL or a table
stores several requests. The SQL bucket remains unchanged. `counter.Context`
and the shared HTTP executors retain a request-owned chi context so later
parallel requests cannot reuse the context being inspected.

Every `api.*` scenario needs a nonempty route in the golden, except an explicitly
reasoned non-HTTP scenario in `architecture/query-budget-non-http.json`.
That exception register must remain present and can only lose exact
scenario/reason pairs after introduction. Do not add an exception for an HTTP
test. Plain SQL slices remain supported for repository and use-case budgets.

A contract disappears only with its own explicit route at the immutable Git
base: add a record to `architecture/query-budget-retirements.json` and remove
that route from the router golden. A retained budget whose route leaves is
orphaned until its field names the newly measured route (ADR 0160). Existing
maxima and exactness stay shrink-only.

## Explicit initial route capture

1. Run the existing budget-bearing tests with
   `PHX_QUERY_BUDGET_CAPTURE_ROUTES=/absolute/path/routes.jsonl` through the
   pinned Go toolchain. Start with an empty report; each observation contains
   the resolved route, never an inference from its description. Shared helpers
   must snapshot `counter.Measurement(...)` after routing, including zero-SQL
   requests. Use the production mount in route-sized test routers.
2. Review every routeless API observation against its test. Only a direct
   non-HTTP use-case call gets a scenario-specific reason in the exception
   register. Reports must cover every API budget, including these exceptions.
3. Run `node scripts/capture-query-budget-routes.mjs /absolute/path/routes.jsonl`
   to validate the complete backfill, then repeat with `--write`. The script
   adds only route fields, rejects conflicting observations and refuses to
   overwrite a different existing route. It changes no scenario, `max` or
   `exact` field. Existing bindings are checked even during capture.
4. Run the same targeted tests without the capture variable, plus the
   architecture comparison against the immutable merge base. Normal runs
   never write routes or budget files.

`PHX_QUERY_DUMP=1` prints complete statements in execution order. When
`PHX_QUERY_BUDGET_REPORT` is also set, each JSONL observation includes the same
untruncated SQL in its `statements` array. Without the dump opt-in, reports stay
compact and ordinary budget failures keep their 200-byte statement previews.
