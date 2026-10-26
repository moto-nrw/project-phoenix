---
status: accepted
---

# Principle: every module, table and cross-owner read has one owner

Principle ADR of 2026-10-19 under
[#2580](https://github.com/moto-nrw/project-phoenix/issues/2580), written for
[#4091](https://github.com/moto-nrw/project-phoenix/issues/4091). It states the
ownership rules that still hold. The ADRs it cites keep their context,
measurements and history; this ADR changes none of them. A new ownership
decision amends this ADR or names it.

> Amended on 2026-10-23 by
> [ADR 0084](0084-traeger-portal-liest-ueber-eine-projektion-und-schreibt-in-der-schule.md):
> point 7 adds the Träger scope for reads of the Träger-Portal.

## Decision

1. **The backend has 19 domain modules and 10 platform modules.**
   `backend/architecture/policy.json` declares them as owners of kind `domain`
   and `platform`, and policy validation pins the exact IDs and kinds
   ([ADR 0010](0010-file-storage-is-the-eighteenth-domain.md),
   [ADR 0012](0012-export-transfer-is-the-tenth-platform.md),
   [ADR 0088](0088-betreuungsvertrag-und-elternabrechnung-sind-eigene-owner-beim-traeger.md)
   adds the Elternabrechnung with its first code in
   [ADR 0115](0115-ferienbeitrag-entsteht-in-der-elternabrechnung-und-wird-bei-bestaetigung-festgehalten.md)):

   | Domain module | Owner ID |
   |---|---|
   | Identity & Access | `identity-access` |
   | People Directory | `people-directory` |
   | School Membership | `school-membership` |
   | Organisation & Tenancy | `organization-tenancy` |
   | School Structure | `school-structure` |
   | School Calendar | `school-calendar` |
   | Facilities | `facilities` |
   | Enrollment | `enrollment` |
   | Care Plan | `care-plan` |
   | Timetable & Activities | `timetable-activities` |
   | Student Presence | `student-presence` |
   | Workforce | `workforce` |
   | Appointments | `appointments` |
   | Communication | `communication` |
   | Device Fleet | `device-fleet` |
   | Meal Plan | `meal-plan` |
   | Feedback | `feedback` |
   | File Storage | `file-storage` |
   | Elternabrechnung (Parent Billing) | `parent-billing` |

   | Platform module | Owner ID |
   |---|---|
   | Tenant Runtime | `tenant-runtime` |
   | Transaction Runtime | `transaction-runtime` |
   | Security Runtime | `security-runtime` |
   | Settings Platform | `settings-platform` |
   | Delivery Platform | `delivery-platform` |
   | Observability | `observability` |
   | Audit Platform | `audit-platform` |
   | Scheduler Runtime | `scheduler-runtime` |
   | Document Rendering | `document-rendering` |
   | Export Transfer | `export-transfer` |

   Adding, renaming or reclassifying a module needs its own architecture
   decision, reflected in #2580, in this list, in the module map of
   `backend/architecture/README.md`, in `CONTEXT.md` and in validation.
   Reusable criteria for choosing a new domain, an existing domain or a
   platform module are not decided yet (ADR 0010, follow-up).
2. **A domain module owns a business concept; a platform module owns a
   runtime, not the meaning of what passes through it.** Document Rendering
   produces output and owns neither its audience nor its storage. File Storage
   owns the managed file lifecycle: metadata, folders, folder grants, quota and
   cleanup intents. The producing domain keeps the access rules for its
   content, as Communication does for announcement attachments (ADR 0010).
   Export Transfer owns transfer configuration, destination validation, the
   SFTP transport and the durable transfer journal, not the exported content
   (ADR 0012).
3. **The other owner kinds are not modules.** `projection` owners serve reads
   across owners (point 6), `workflow` owners coordinate writes across owners
   ([ADR 0067](0067-principle-owners-meet-through-contracts-and-write-together-in-workflows.md)),
   `tool` owners (`architecture-check`, `dev-tools`) are engineering tools, and
   the `composition`, `kernel`, `migration` and `test-support` owners hold the
   roots, the shared kernel, the migrations and the shared fixtures. The
   `inbound` kind is gone; live policies reject it
   ([ADR 0057](0057-only-the-target-is-a-rule.md) and
   [ADR 0060](0060-the-dev-tools-are-a-binary-of-their-own.md), addenda of
   2026-10-18).
4. **Each table has exactly one runtime write owner**, declared in
   `data_objects`. Runtime data belongs to domain and platform modules; the
   `migrations` owner holds only migration bookkeeping such as backfill
   checkpoints and compatibility ledgers. A table that no target owner keeps
   stays a `tables.unclassified` finding until its access and the table are
   removed; nobody invents an owner to quiet the check. How an owner adopts an
   existing table is a reviewed procedure of
   [ADR 0070](0070-principle-only-the-target-is-a-rule-and-debt-leaves-with-evidence.md).
5. **Projections and workflows own no writable data.** A projection owner can
   never be a write owner
   ([ADR 0023](0023-student-enrollment-owner-and-directory-projection.md)); a
   workflow cannot own runtime data (ADR 0012,
   [ADR 0013](0013-staff-offboarding-is-an-application-workflow.md)).
6. **A read across owners goes through the owner's public capability or a
   named tenant-safe read projection.** A `read_projections` entry names its
   package and its exact tables, carries `tenant_safe: true` and never permits
   a write. A projection owner exists only where one statement joins tables of
   several owners; a read through consumer-owned ports needs none
   ([ADR 0037](0037-shift-planning-cutover-replaces-legacy-permissions.md),
   [ADR 0052](0052-time-tracking-dissolution-replaces-legacy-permissions.md)).
   It is named after the read it serves, not after a new concept: the
   Kinderliste is `student-directory-view` (ADR 0023).
   [ADR 0081](0081-owner-granularity-follows-invariants-not-source-size.md#port-composed-read-models-and-adr-0066-point-6)
   retains the existing port-composed Staff Directory and Emergency Snapshot
   owners without SQL grants; it gives selection criteria for future cuts.
   Consumer-port reads do not generally require a projection owner.
7. **`tenant_safe` on a cross-tenant read means operator scope or Träger
   scope.** An installation-wide read runs inside the cross-tenant
   administrative transaction, and its routes sit behind the operator gates.
   A Träger-Portal read runs inside the same transaction behind the Träger
   session, filters on the schools of the session's Träger, and a ratchet
   checks that filter on every query of its package
   ([ADR 0084](0084-traeger-portal-liest-ueber-eine-projektion-und-schreibt-in-der-schule.md)).
   A projection that repeats another
   owner's literal, such as a role name, changes it in the same change as that
   owner
   ([ADR 0033](0033-operator-dashboard-view-is-a-projection-owner.md)).
8. **Decided assignments stand.** The student record is split between People
   Directory (`users.student_profiles`), School Membership
   (`users.student_school_memberships`, with the class and lifecycle commands
   and the class-writes gate) and Care Plan (`users.student_care_profiles`)
   (ADR 0023). The public student ID is the profile ID, never the membership
   ID ([ADR 0025](0025-replace-student-read-projection-grants.md)). Child
   documents (`users.student_documents`) are Care Plan
   records; moving them to File Storage is a separate ownership change
   ([ADR 0035](0035-care-lifecycle-cutover-replaces-legacy-permissions.md)).
   The work-session edit audit stays with Audit Platform (ADR 0052).

## Consequences

- The module lists here, in the architecture README and in `CONTEXT.md` are
  one list. Policy validation fails on a missing, extra, renamed or
  reclassified module.
- `scripts/backend-architecture.sh check` enforces points 4 to 6: one write
  owner per table, foreign reads only through an exact grant, and findings for
  unresolved or unclassified table expressions.
- [ADR 0025](0025-replace-student-read-projection-grants.md) is a historical
  record: its one-time grant replacement is done.

## References

ADR 0010, ADR 0012, ADR 0023 and ADR 0033 carry the decisions; ADR 0025,
ADR 0035, ADR 0037 and ADR 0052 carry the readings named in points 6 and 8,
and ADR 0057 and ADR 0060 the end of the `inbound` kind. The full
rule map is the
[traceability appendix](../../backend/architecture/README.md#appendix-principle-traceability).
