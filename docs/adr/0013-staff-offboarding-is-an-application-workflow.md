---
status: accepted
---

# Staff Offboarding is an application workflow

The staff lifecycle cutover in [#2709](https://github.com/moto-nrw/project-phoenix/issues/2709)
implements the cross-module workflow required by
[#2580](https://github.com/moto-nrw/project-phoenix/issues/2580).
Add `staff-offboarding` as a workflow owner, with no runtime data ownership.
This registration implements the existing lifecycle-workflow decision in
[#2580, Implementation Decisions §8](https://github.com/moto-nrw/project-phoenix/issues/2580)
and the atomic cutover specified in #2709; it does not introduce a new domain.
It coordinates authorization, preview consistency, lock order and one UnitOfWork;
Membership, Workforce, Timetable, People Directory and Identity & Access retain
their mutations, with Audit and durable cleanup delegated to their owners.

Putting the orchestration inside Membership or Workforce would make that domain
responsible for other owners' lifecycle rules. The separate workflow keeps the
18 domain and 10 platform owners unchanged and does not transfer any table.
Its cutover replaces the repository-coordinating service once, without a second
runtime provider or direct workflow SQL.

## Policy registration

[#3130](https://github.com/moto-nrw/project-phoenix/issues/3130), linked under
[#2580's decision](https://github.com/moto-nrw/project-phoenix/issues/2580#issuecomment-5593113472),
records the evaluator prerequisite. Policy epoch 3 permits a reviewed
workflow addition only when every package of that owner is new in the
candidate. Policy validation still rejects workflow-owned writable data;
existing-package reclassification, existing-owner import expansions, and
legacy-baseline growth remain forbidden. This decision was authorized in the
implementation session on 2026-09-09.

## Durable cleanup

Workforce owns the additive `users.staff_offboarding_cleanup` outbox. Retirement
and its staff-level intent commit together. The worker commits a two-minute
lease before touching files, then completes or reschedules the job using its
tenant, staff ID, lease token and expiry. Missing files are already removed;
failed removals remain pending. Existing document rows retain completion marks
so a crash between file deletion and job completion is retryable. Directory
and scheduler recovery enqueue work and wake this same worker.

The after-commit callback is a losable wake-up hint, not the authority to remove
files. A lost callback is recovered by the existing five-minute scheduler.
Identity retains its existing durable account-wide token-wipe mechanism.
Neither mechanism restores removed bytes or credentials on a repeated request.
