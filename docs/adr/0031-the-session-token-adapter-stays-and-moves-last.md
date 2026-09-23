---
status: accepted
---

# The session token adapter stays and moves last

[#3226](https://github.com/moto-nrw/project-phoenix/issues/3226) rewrote the
Identity & Access persistence and token layer natively, but could not delete
`modules/identityaccess/legacy/jwt`.
[#3487](https://github.com/moto-nrw/project-phoenix/issues/3487) asked where
the session middleware and the signer should live instead. We decide under
[#2580](https://github.com/moto-nrw/project-phoenix/issues/2580): the package
is the permanent session token adapter of `identity-access`. It gets no
replacement. It leaves the `legacy/` path once, at the end, when the carrier
tickets have removed their debt on it.

## Why the package is already the target

Measured on `cf45b8dcf6`, policy epoch 18:

- It is the only package classified `identity-access`/`adapter`.
- 50 target rules permit an import of `identity-access`/`adapter`, 26 of them
  in production scope. They state the purpose in their descriptions, for
  example `facilities.compose.identity-adapter`: "Facilities HTTP composition
  reads authenticated permission claims." Already migrated module packages
  read claims through these rules today. A rule names an owner and a role,
  never a path, so the rules keep applying after a rename.
- Since [PR #3488](https://github.com/moto-nrw/project-phoenix/pull/3488) the
  package reads no process configuration, does not know the tenant runtime
  (`jwt.TenantScope` port), and neither generates nor persists a signing key.
  All 17 tuples of
  [#2725](https://github.com/moto-nrw/project-phoenix/issues/2725) are gone.

What remains "legacy" is the path, plus 31 exact baseline tuples that name the
package (13 production, 8 internal-test, 10 external-test). Those tuples belong
to the importing side: #2736 (6), #2748 (5), #2731, #2734 and #2742 (3 each),
#2728, #2732, #2733, #2738 and #2750 (2 each), and #2747 (1).

## Considered options

1. **A native session HTTP package now**, with
   `github.com/lestrrat-go/jwx/v3/jwa` classified as `crypto`. Rejected for
   now. The 31 tuples are keyed by package path, so a new path turns every one
   of them into a new violation unless a relocation
   ([ADR 0020](0020-a-relocated-package-keeps-its-debt.md)) renames them.
   That is a broad policy change in the same window as the carrier tickets,
   and it buys only a path. The middleware would be the same code.
2. **Wait without deciding.** Rejected. #3226 would stay open until the last
   carrier closes and keep #2725,
   [#3230](https://github.com/moto-nrw/project-phoenix/issues/3230) and
   [#3231](https://github.com/moto-nrw/project-phoenix/issues/3231) blocked,
   although their work does not depend on the path. It would also leave open
   what those tickets should build against.

## Decision

- Handlers that move into their owner modules keep importing the adapter for
  `Authenticator`, `ClaimsFromCtx`, `PermissionsFromCtx`,
  `ActorAccountIDFromCtx` and the scope gates. #3230 and #3231 build against
  it unchanged. Nobody adds a second home for session middleware or claims.
- The package is closed for growth in the meantime: no new dependency on
  process configuration, the tenant runtime or persistence comes back.
- #3226 closes on what it delivered. Its remaining exit criterion, the
  deletion of the `legacy/jwt` path, moves to #3487.
- #3487 runs last and does one thing. When the carrier tuples above are gone,
  it renames the package out of `legacy/` with one relocation declaration
  under a new policy epoch, and decides the `jwa` classification in that same
  epoch if the signer still needs it. `services → legacy/jwt` (#2747) is
  resolved there or by #2747, whichever comes first. #3487 blocks no other
  ticket.

## Consequences

- The `modules/identityaccess/legacy` LOC budget keeps counting the adapter
  until the rename. That number is the path, not open work.
- The rename is not a relocation disguised as completion. ADR 0020 renames
  keys and grants nothing, and by then the carriers' keys no longer exist, so
  the declaration moves a package that only target rules point at.
- A change that needs session state outside HTTP, such as a worker or the
  scheduler, takes it from the Identity & Access public capability, not from
  this adapter.
