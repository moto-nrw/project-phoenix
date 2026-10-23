---
status: accepted
---

# Exact method evidence supplements pass-through floors

Decision of 2026-10-23 for [#4738](https://github.com/moto-nrw/project-phoenix/issues/4738).
First implementation cut of ADR 0083 section 3. ADR 0089 is accepted on
`development`; this PR uses the following decision number, 0090.

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
   Changing a root translation to an unchanged forward fails even when the owner's
   sum stays equal. Relocating debt to a new identity also needs a new ADR;
   renaming a file in one package retains evidence. Root bindings keep the
   existing seven consumer-attributed prefixes/files without double counting.
3. Scan **all backend trees**, not another location allowlist. The ADR 0087
   exclusions remain: hidden directories and `_test.go` only. Roots, new trees,
   legacy, tools, generated sources and fixtures cannot become an unmeasured
   destination. The scope binds a deterministic FNV-128a source digest of
   `predicate.go`, `scanner.go`, `forms.go` and `register.go`, embedded from the
   compiled sources, including scan roots, filters and comparison logic. This is
   a change detector, not a cryptographic trust boundary. Compare it against the
   PR base: any change to those sources needs a newly numbered ADR, even after
   regenerating the snapshot. Source-only formatting changes conservatively
   require the same review as semantic changes. Package role/kind metadata
   separates tools, migrations and untagged test support; `go:build` constraints separate files requiring `testhooks` regardless
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
5. Align with ADR 0057 point 5: a newly numbered ADR is needed only for an
   increase in debt or a changed scope/scan semantics (point 3). A new
   `forward-debt` or `guard-debt` entry, or a change from another form into either
   debt form, needs an ADR number absent at the immutable PR base and present
   in this change, cited **on that entry**. A switch between debt forms is also
   reviewed as a new form. Body, attribution and reason updates with the same
   form, including `unclassified-evidence` to `unclassified-evidence`, and
   improvements from debt to evidence or removal update only the register.
   They remain source-checked and visible in review, without a new ADR.
   A removed entry cannot fund another. For each missing base entry, inspect
   the head package directly, independently of the scan's traversal filters:
   the method must actually be gone or no longer satisfy the pass-through
   predicate. A still-matching method is an error even with a new scope ADR.
   `scripts/backend-architecture.sh check` validates actual source plus this
   delta after the existing architecture checks, using the same `--base-ref`.
   The host inventory applies when the CLI checks its host repository, including
   explicit `--project` pointing to the host through a symlink. Independent
   synthetic projects have their own contracts, even when they use moto’s module name. A missing
   host inventory is always an error.
   Bootstrap needs a scope decision and decisions for its initial debt entries:
   removing this ADR makes the initial register red. Existing numeric budget
   comparison is unchanged.
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
only the reviewed entries, citing a new decision only for worsened debt or
changed scan semantics. It is not an acceptor
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
