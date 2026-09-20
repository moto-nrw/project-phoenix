---
status: accepted
---

# Migration evidence gates preserve provenance

The design interview for [#3415](https://github.com/moto-nrw/project-phoenix/issues/3415)
settled the following contract, confirmed on 2026-09-19. Acceptance records
the agreed design, not implementation authorization. This record does not
claim that the validator implements the contract or that
[#2580](https://github.com/moto-nrw/project-phoenix/issues/2580) has been updated.

## Settled decisions

### Checkpoint references remain strict

Every migration evidence file references the latest accepted checkpoint.
Accepting a checkpoint updates all references in the same change, including
the migration template. Validation and a regression test must enforce this.

The reference identifies the current accepted checkpoint, not a rerun of the
ticket's measurements. Existing evidence retains its original provenance.
We accept the maintenance cost rather than add merge-time acceptance tracking.

### Evidence must distinguish measurements from justified omissions

Every required runtime-evidence field must explicitly contain a measurement
or a concrete non-applicability reason. Bare placeholders such as `N/A` and
`TODO` are insufficient. CI checks completeness and representation; review
checks truth, applicability and whether the evidence supports the cutover.
Migration metrics use explicit states: `measured` with evidence,
`not_applicable` with a reason, and `historical_gap` under the frozen exception
below. Checkpoint measurement representation is unchanged by this ticket.
Reject bare placeholders and unchanged template example values in real
evidence files. Review remains responsible for detecting unsupported claims
that pass these structural checks.

A `measured` metric needs an observed result, an identifiable evidence source
and workload context. Qualitative observations qualify; planned measurements
and code-derived expectations do not. Classify existing evidence metric by
metric rather than wrapping every nonempty string as a measurement. For
example, SQL acquisition duration is not measured lock-wait evidence.

### Historical gaps remain visible

Already-merged work may retain narrowly identified, explicitly documented
gaps in historical evidence. Missing measurements must not be described as
non-applicable when the metric applied. New evidence must satisfy the full
contract.

Freeze exceptions by exact file and metric at one immutable pre-gate commit.
Each exception requires a written explanation. Updating a checkpoint reference
preserves the exception; new measurements or removal claims cannot inherit it.
Use commit `e4bb1a38c94180ae337a68726bd565bcfe23ea00` as the immutable
pre-gate inventory anchor. It is the checkout's HEAD before implementation,
not a moving branch reference.

A historical record with exempted gaps cannot cover a new removal, even when
it already lists that key as pending. Resolve its gaps with complete evidence
or add a separate complete record for the same issue and keys. Preserve the
original provenance rather than describing old evidence as newly measured.

This is a deliberate exception to #3415's measurement-or-reason requirement:
landing the gate should neither fabricate evidence nor require rerunning every
old migration. The exception must not provide an escape for future work.
Record this change in #2580 when publishing the agreed contract.

### Ratchet keys describe a ticket's scope

`exact_ratchet_keys` describes scoped debt across multiple PRs, not a claim
that every listed key disappears in the current PR. Report keys separately as
removed in this diff, already absent, or still pending. Every actual debt
removal must belong to a non-template ticket. Pending debt does not by itself
fail evidence validation.

Newly introduced key claims must match debt in the comparison base or candidate
baseline, accounting for verified relocations. A key absent from both cannot
be introduced as an unsupported historical claim. Preserve claims already
present at the pre-gate anchor without retrospective backfill.

Report raw baseline additions and removals separately. Distinguish verified
relocations from retired debt, following
[ADR 0020](0020-a-relocated-package-keeps-its-debt.md); renaming a package does
not retire its debt. Existing architecture checks retain authority over
whether additions and relocations are allowed.

### File discovery fails closed

Validate every JSON file directly under `backend/architecture/` except named
non-ticket documents: `policy.json`, `composition.json`,
`runtime-checkpoints.json`, and `contract-active-2737-progress.json`.
Document why each is excluded. A missing `ticket_kind` is an error, not a way
to escape discovery.

Validate both shipped templates, including the current checkpoint reference
on the migration template. Templates are examples, not cutover evidence;
their keys never satisfy removal coverage.

Use complete, clearly synthetic examples in the two named templates. Their
structures pass normal validation; real evidence files cannot use unchanged
template example values. Template identity comes from the two known files,
not a ticket-local flag that arbitrary evidence can set.

### Compare the changes belonging to each CI event

Ordinary PRs use the event's immutable PR base SHA. Merge queues use the
merge-group base SHA. Protected-branch pushes use the pre-push SHA. Missing
or unusable required history fails explicitly; comparing a push to itself is
not a fallback.

Same-repository `development` to `main` release promotion retains its existing
exact-tree and successful-development-push-CI checks. It validates all current
evidence files but reuses verified development coverage rather than re-auditing
pre-gate history against `main`. This exception does not extend to ordinary
PRs or excuse invalid checkpoint references.

Rollout clarification, confirmed on 2026-09-20: the first release push to
`main` may also reuse verified development evidence when its base predates the
gate. Require a direct development parent or the fast-forward commit, successful
development push CI, ancestry and exact-tree equality. This avoids demanding
backfill for pre-gate retirements; absence of proof fails the release.

The new evidence gate reports additions but does not independently outlaw
additions already allowed by the architecture policy, including verified
relocations and permitted conversions of existing imports to recorded debt.

Changes in known evidence-source directories, including
`docs/runtime-checkpoints/` and `docs/operations/`, must also trigger the
evidence gate. Check that declared local evidence sources exist. Linked raw
artifacts are evidence sources, not ticket-schema documents. Remote-source
truth and measurement quality remain review-owned; deterministic validation
does not fetch remote evidence.

## Implementation acceptance

1. CI and a local test validate the entire discovered inventory, including both
   templates, against the real checkpoint registry. Missing ticket kinds and
   superseded references fail. A checkpoint-acceptance test covers the required
   reference updates.
2. Tests reject omitted metrics, bare placeholders, copied template evidence,
   unauthorized historical exceptions and new removal coverage supplied only
   by historical-gap records. Existing measurements retain their provenance.
3. `validate-ticket --base-ref <sha>` checks aggregate removal coverage and
   reports raw additions/removals, verified relocations and pending scoped
   keys. Fixtures cover unclaimed removals and unsupported new historical
   claims; verify the accounting against a real PR diff.
4. Tests cover event-specific bases, unavailable history, evidence-source-only
   routing and verified release promotion. Existing architecture guards remain
   authoritative; this ticket changes neither policy ownership nor the baseline.
5. Document the executable contract in `backend/architecture/README.md` and
   record the agreed exception and `validate-ticket` CI requirement on #2580
   when publishing. Run required backend and changed-code checks during
   implementation; documentation checks alone do not establish gate behavior.

The wire schema transition and CLI inventory plumbing are implementation
choices within this contract, not further policy decisions. Preserve the
single-ticket validation use case while adding aggregate CI validation.

No product-domain term has been introduced; the product glossary is unchanged.
