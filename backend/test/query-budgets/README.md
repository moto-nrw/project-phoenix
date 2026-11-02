# Authored SQL contracts

Each scenario has one owner/resource-local JSON file. These are reviewed policy,
not measurements to regenerate. `max` is a ceiling; `exact` additionally proves
the guarded read still executes. Preserve scenario identity when moving a file.

The embedded loader discovers files recursively and rejects duplicate scenario
identities, duplicate JSON fields, unknown/missing fields and negative ceilings.
`TestQueryBudgetRatchet` still checks global test references and rejects local
query hooks. The architecture check with `--base-ref <SHA>` reads the complete
immutable base tree, including historical Go-map registers, and rejects missing
contracts, larger maxima and removal of exactness. New scenarios need measured
counts and a referencing test. Lower a ceiling when an optimization removes SQL.
