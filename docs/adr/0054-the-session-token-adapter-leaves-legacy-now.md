---
status: accepted
---

# The session token adapter leaves legacy/ now

Decision for [#3487](https://github.com/moto-nrw/project-phoenix/issues/3487)
under [#2580](https://github.com/moto-nrw/project-phoenix/issues/2580). It
keeps the substance of
[ADR 0031](0031-the-session-token-adapter-stays-and-moves-last.md) and replaces
its timing.

## Context

ADR 0031 made `modules/identityaccess/legacy/jwt` the permanent session token
adapter of `identity-access`. It said the package would leave `legacy/` with
one relocation, after the carrier tickets had removed their debt on it.

Measured on `6343f9082d`, policy epoch 40:

- Five baseline entries name the package, against 31 when ADR 0031 was
  written: `modules/identityaccess/behavior` (#3446), `api` in production and in
  internal test (#2750), `services` (#2747) and `test` (#2748). Eight of the
  eleven carriers ADR 0031 named are closed.
- 254 files import the package (150 production files in 35 packages). 53 rules
  permit `identity-access`/`adapter`, none of them temporary. It is still the
  only package with that role.
- No Go file imports `github.com/lestrrat-go/jwx/v3/jwa` directly. It arrives
  only through `go-chi/jwtauth`, so classifying it would itself fail as
  `external.stale`.
- The wait has become circular. #2747 and #2748 point at #3487 (ADR 0047, the
  WORKPLAN), and #3487 waits for their keys to go.

## Decision

- ADR 0031's substance stands. The package is the one permanent session token
  adapter. HTTP owners read claims through it, nobody adds a second home for
  session middleware or claims, and the package does not grow a dependency on
  process configuration, the tenant runtime or persistence.
- The timing changes. #3487 runs now, after the policy epoch of ADR 0053
  ([#4001](https://github.com/moto-nrw/project-phoenix/issues/4001)), because
  both raise the epoch and #4001 removes three of the five entries. One
  relocation declaration (ADR 0020) moves the package to
  `modules/identityaccess/sessiontoken`, with the package clause
  `sessiontoken`. The new path drops the `legacy/` signal from the imports of 29
  owners and no longer clashes with the `jwt` package of `lestrrat-go`.
- The relocation renames the entries that remain and keeps each one under its
  carrier's issue. After #4001 that should be `services` (#2747) and `api` in
  both scopes (#2750). This is the purpose of ADR 0020. It is not a relocation
  standing in for completion: #3487's exit is the path, and the carriers' exits
  stay theirs.
- The inactive relocation record `auth/jwt ->
  modules/identityaccess/legacy/jwt` is removed or rewritten, so that relocation
  validation, which requires every `to` path to be classified, still passes.
- The `jwa` question of ADR 0031 is closed without a classification.

## Considered options

1. **Keep waiting until the carriers are gone.** Rejected. The carriers point
   at #3487, so the wait never ends by itself.
2. **A JWT-agnostic public principal for the HTTP owners.** Not now. It would
   touch about 250 read calls in 150 files, need about 29 new cross-owner
   rules, and reverse ADR 0031 while the claims struct already is the
   principal. The rename keeps that option open as a separate decision.

## Consequences

- `modules/identityaccess/legacy` disappears, and with it its entry in the
  legacy LOC budget.
- Documentation that names the old path changes in the same pull request:
  `backend/CLAUDE.md`, `.claude/rules/backend-conventions.md` rule 9,
  `docs/agents/contracts.md`, the architecture README and the WORKPLAN.
