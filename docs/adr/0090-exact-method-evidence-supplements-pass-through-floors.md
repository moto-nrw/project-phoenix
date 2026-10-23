---
status: accepted
---

# Exact method evidence supplements pass-through floors

Decision of 2026-10-23 for [#4738](https://github.com/moto-nrw/project-phoenix/issues/4738).
First implementation cut of ADR 0083 section 3. ADR 0089 is reserved by open
PR #4758; 0090 was the next free number on development and in open PRs.

## Decision and scope

1. Keep `TestModulePassthroughRatchet`, every owner budget, its hook-inclusive
   total, and all five ADR 0075 forms and four named exceptions unchanged.
   The shared Go predicate reproduces its four AST conditions verbatim. This
   is a supplement, not the full semantic successor; do not subtract hooks
   from the old budget or reinterpret its floors from these syntax labels.
2. Add `architecture/passthrough.json`: exact backend-relative package, receiver
   type and method, attributed owner, build category, syntactic form, reason,
   normalized declaration text and decision. The structural evidence includes
   signature and body, ignores comments/formatting and is filename-independent.
   Keeping the normalized AST rendering itself avoids hash collisions and
   cryptographic permissions for test tooling.
   Changing a translation to an unchanged forward fails even when the owner's
   sum stays equal. A package relocation changes the identity and fails;
   renaming a file in one package retains evidence. Root bindings keep the
   existing seven consumer-attributed prefixes/files without double counting.
3. Scan **all backend trees**, not another location allowlist. The ADR 0087
   exclusions remain: hidden directories and `_test.go` only. Roots, new trees,
   legacy, tools, generated sources and fixtures cannot become an unmeasured
   destination. The scope manifest records the exclusions and the four-condition
   predicate; compare it against the PR base. A new scope needs a newly numbered
   ADR. Package role/kind metadata separates tools, migrations and untagged test
   support; `go:build` constraints separate files requiring `testhooks` regardless
   of platform. Tagged sites do not consume production entries. Existing source,
   import, tenant and composition checks still cover every category.
4. The register is **finite evidence, not a new blanket floor**. Each root
   `forward-debt` and `guard-debt` entry is individually named, body-bound debt
   accepted only at this initial cut under this ADR. Its reason requires removal
   or deepening, not deletion of behavior to improve a number. There is no
   fungible owner allowance. Non-root unchanged/guarded forwards retain neutral evidence labels so error
   wrappers and other established scanner-limit floors are not demoted to debt.
   Other bodies are `unclassified-evidence`, not
   automatically translations or approved new floors. The four ADR 0075
   exceptions are recognized only at their exact packages/types/methods.
   Nil guards and method renames alone earn no translation classification.
   Syntactic casts and field copies remain unclassified: no claim of vocabulary
   translation is made. Existing ADR 0075 floor eligibility is unchanged.
5. New or modified registrations, including bodies, forms, attribution and
   reasons, need an ADR number absent at the immutable PR base and present in
   this change, cited **on that entry**. A removed entry cannot fund another.
   `scripts/backend-architecture.sh check` validates actual source plus this
   delta after the existing architecture checks, using the same `--base-ref`.
   The host inventory applies when the CLI checks its host repository, including
   explicit `--project` pointing to the host. Independent synthetic projects
   have their own contracts, even when they use moto’s module name. A missing
   host inventory is always an error.
   Bootstrap is subject to exactly the same rule: removing this ADR makes the
   initial register red. Existing numeric budget comparison is unchanged.
6. Retire the executable Python snapshot after comparing its complete root
   method identity set with the Go snapshot at the measured base. The tested
   Go scanner and `passthrough-snapshot` command replace its copied predicate.
   Historical semantic counts remain in ADR 0082/0083, with their stated limits.

The dead-code gate follows both tooling packages into the standalone architecture
command: `internal/architecture` and `internal/passthrough` are measured from that
production entry point, not from the unrelated backend server. Both remain
checked with backend test entry points too. Negative shell fixtures require the
CLI filter to include both packages and reject findings in either context; no
backend production delegation is excluded. Snapshot export has a behavioral
JSON/identity/body/decision test, not an artificial liveness exemption.

## Evidence and reproduction

Baseline: `e417e90b4377d3b1cfe73b168bb8d6b45e145d7c` (fetched before branching).
No runtime capability, workflow, permission, owner edge or domain source changes.
Only `internal/passthrough` is added under the existing Test Support owner;
`go/build/constraint` and `go/format` are classified as standard library.
No import rule, permission or policy epoch is added. Parallel #4739 and #4740 files are not edited.

```sh
# Reviewable snapshot, no implicit registry writes:
PHOENIX_ARCHITECTURE_PROJECT="$PWD/backend" scripts/run-go-toolchain.sh /bin/sh -c \
  'cd scripts/backend-architecture && go run . passthrough-snapshot'
# Gate, including immutable-base comparison:
scripts/run-go-toolchain.sh scripts/backend-architecture.sh check
(cd backend && ../scripts/run-go-toolchain.sh go test ./internal/passthrough -count=1)
```

The snapshot command records this initial decision; future source changes edit
only the reviewed entries and cite their own new decision. It is not an acceptor
or a rebaseline switch. Generated registry lines are evidence for review, not a
second checker. Missing/duplicate/stale/changed entries fail the source check.
The PR records exact category counts, root identity equivalence, negative ADR
probe and ratchet-suite wall times after final verification.

## Explicit limits and follow-up proposal

This first cut closes same-sum swaps and moves to roots with executable fixtures,
separates hook costs and keeps the existing floors authoritative. It does **not**
claim the full semantic successor is complete; use `Refs #4738`.

A follow-up (estimated 3–5 engineering days plus review) should map every
unclassified body to the exact ADR 0075 form or named finite debt, resolving
bound port/row/error types rather than treating syntax as translation. Include
same-vocabulary Device Fleet casts/copies and all guard-only root bindings.
Retire the owner-sum gate only after that classification and its negative
fixtures cover every retained floor. Pure-delegation removal is separate,
behavior-tested work, not part of this evidence cut.

The four-condition predicate still misses unexported methods, sister/free
function delegation, aliases, and bodies over 14 lines. A follow-up scanner
change must name those new finite sites and preserve positive/negative fixtures;
this cut does not silently alter the predicate or raise any existing budget.
