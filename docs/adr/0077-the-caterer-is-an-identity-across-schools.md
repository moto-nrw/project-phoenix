---
status: accepted
---

# The caterer is an identity across schools

Decision for [#3873](https://github.com/moto-nrw/project-phoenix/issues/3873)
under [#2580](https://github.com/moto-nrw/project-phoenix/issues/2580).

## Context

A school orders lunch from a caterer. One caterer supplies several schools,
often of different Träger, and wants one place to see what each of them
ordered. Until now every moto account reached data through a membership in
one school (`auth.account_tenants`) or, for parents, through a relationship to
a child. A caterer has neither: it is not part of a school's team, and it must
not see children, only the numbers a school orders, and names only when that
school switches them on.

Organisation & Tenancy owns Träger and schools; a caterer belongs to neither
and works across Träger. Identity & Access already owns the one identity that
spans schools without belonging to their staff: the parent account, which
reaches every school of its children and runs each request in that school's
tenant transaction.

## Decision

1. **Identity & Access owns the caterer and its supplies.** The
   Caterer-Betrieb (`auth.caterers`), its accounts (`auth.caterer_accounts`,
   one row per login account in `auth.accounts`), the school's invitation
   (`auth.caterer_invitations`) and the supply (`auth.caterer_schools`) are
   identity data with `write_owner: identity-access`. A caterer account is an
   ordinary `auth.accounts` row: the password, lockout and reset flows stay
   the existing ones. One caterer supplies any number of schools of any
   Träger; a second school's invitation to the same address adds the school
   to the existing account, confirmed with its password. An address that
   holds another kind of account cannot accept.
2. **The supply has one record.** The Belieferung, which caterer supplies a
   school, where it delivers, since and until when and who started and ended
   it, is `auth.caterer_schools`: tenant data of the school under its RLS,
   like `auth.account_tenants` is for school accounts. The acceptance starts
   it from the invitation's delivery address; the school ends it at any time,
   and ending it removes the caterer's access to that school at once. In V1 a
   school has at most one active supply (`uq_caterer_school_active`). The
   Meal Plan keeps no copy: it owns the orders and the caterer's
   confirmation, and the caterer portal hands it the start of the running
   supply, read in the same school transaction.
3. **A portal scope of its own.** Caterer sessions carry the JWT scope
   `caterer` and no tenant, like parent sessions. `CatererMiddleware` accepts
   only that scope; the tenant, school and parent middlewares reject it, and
   the caterer middleware rejects all of theirs. The portal has its own host
   (`NEXT_PUBLIC_CATERER_HOSTNAME`), NextAuth instance, host-only cookie and
   base path, as the other portals do.
4. **One read across schools, in the shape of ADR 0084.**
   [ADR 0084](0084-traeger-portal-liest-ueber-eine-projektion-und-schreibt-in-der-schule.md)
   lets the Träger-Portal read across schools in exactly one place, in an
   administrative transaction, with a filter a ratchet checks. The caterer
   follows the same pattern:
   - The only read across schools is the caterer's list of schools: the
     caterer of the session's account (`auth.caterer_accounts`) and its
     running supplies (`auth.caterer_schools`). It runs in the existing
     global identity read (`globalIdentityRead`), the administrative
     transaction the login already uses to resolve an account, and adds no
     transaction site: `backend/test/transaction_boundaries.json` stays
     shrink-only. Every statement of Identity & Access on
     `auth.caterer_schools` filters on the school in scope or, for this list,
     on the caterer; `caterer_schools_filter_internal_test.go` is the
     ratchet. The school-directory port then filters the IDs to live, active
     schools in one bulk read. The school-list query budget is three
     statements, independent of school count.
   - It needs no projection owner: the read joins no table of a second owner
     ([ADR 0066](0066-principle-every-module-table-and-cross-owner-read-has-one-owner.md)
     point 6), and its filter is the caterer, not a Träger's schools.
   - Everything else runs in one school's own tenant transaction under RLS.
     The invitation goes to the caterer by mail only; the school never sees
     the link, so only the owner of the address can accept. Identity stores
     the link's hash; the queued mail holds the link until the outbox's
     delivery retention removes it, as with the guardian invitation. The
     link names its school; preview and acceptance open that school's
     transaction, where the account, the caterer, the spent invitation and
     the supply commit together.
   - The caterer portal is a workflow (`workflows/catererportal`, owner
     `caterer-portal`) in the shape of ADR 0022. It opens each school's own
     tenant transaction, checks the supply there again and reads the orders
     through the Meal Plan's public capability.
5. **The workflow owns the order and the lock.** Reading or confirming an
   order depends on the supply of another owner, so the protocol is the
   workflow's ([ADR 0067](0067-principle-owners-meet-through-contracts-and-write-together-in-workflows.md)
   addendum, condition 3). In the school's transaction the caterer portal
   takes the advisory lock `caterer-school:<school>` before Identity & Access
   creates an invitation, accepts one or ends the supply, and before it reads
   the supply and the Meal Plan confirms an order against it. Identity &
   Access and the Meal Plan run only their own commands and take no lock of
   their own; neither calls back into the workflow
   ([ADR 0071](0071-principle-owner-dependencies-point-one-way.md)). Reads
   take no lock.
6. **Numbers by default.** The caterer reads orders, changes to them and
   counts per day and class. Children's names reach it only while the school's
   `gdpr.meal_caterer_names_enabled` is on. The reason a child does not eat
   never reaches it. It reads and confirms only days from the start of its
   supply on, so a new caterer never sees its predecessor's orders. Its only
   write in V1 is confirming an order.
7. **Password first.** Caterer accounts sign in with e-mail and password,
   like parent accounts. A second factor is a follow-up: the existing MFA is
   bound to a school's policy, and a caterer has no school.

## Consequences

- The `chk_tokens_portal_scope` check of `auth.tokens.portal_scope` and every scope switch gain the value `caterer`;
  the wrong-portal tests cover it.
- The policy registers the owner `caterer-portal` (workflow) and assigns the
  four identity tables to `identity-access` in the same change. The Meal Plan
  gains no table.
- `auth.caterers` and `auth.caterer_accounts` are global. A school's role
  reads them, but holds no write on them: the acceptance creates a caterer
  and its account link only through `auth.create_invited_caterer`, a
  `SECURITY DEFINER` function that requires an open invitation of the
  transaction's school and the account of the invited address. Caterers'
  business names are no personal data.
- Deployments need `CATERER_URL` and `NEXT_PUBLIC_CATERER_HOSTNAME`, DNS and
  TLS for the new host before the release that contains the portal.
- A caterer account cannot sign in to any other portal, and no other account
  can sign in to the caterer portal.

## Budgets

The caterer reaches Identity & Access through its public module and engine,
like every other flow there. [ADR 0057](0057-only-the-target-is-a-rule.md)
point 5 lets the registers only fall; this decision raises them by exactly
what the caterer adds. Together with
[ADR 0078](0078-meal-ordering-raises-the-meal-plan-compose-budgets.md):

1. `modulePassthroughOwnerBudgets["modules/identityaccess"]` rises from 213
   to 215: the revocation of an invitation and the read of the school's
   supply attach the unit of work and translate the errors around one store
   call each.
2. `moduleComposeBudgets["modules/identityaccess/compose"]` rises from 1,303
   to 1,348: composing the caterer flows beside the engine, as the demo
   access is, with the delivery of their invitation mail, and handing the
   authentication the caterer store for the refresh.
3. `moduleComposeBudgets` gains `workflows/catererportal/compose` with 37
   lines: binding the caterer portal to each school's tenant transaction and
   to the supply lock.
4. `modulePassthroughOwnerBudgets["modules/mealplan"]` falls from 3 to 1: the
   supply left the Meal Plan.
5. With ADR 0078, `modulePassthroughOwnerBudgetTotal` is 1,408 (1,407 on
   development) and `moduleComposeBudgetTotal` 12,498 (12,382).

The Meal Plan's public contract splits into `Menu`, `Registration` and
`Ordering`, each at most twelve methods wide; `Capability` embeds all three.

## Policy

Policy epoch 92 → 93 registers the data-less workflow owner `caterer-portal`
with its five new packages (ADR 0013 path, #3130) and the rules its code
uses: its public contract and ports speak the public Identity & Access and
Meal Plan contracts, its composition binds the tenant runtime, and its HTTP
transport renders through `api/common`. It owns no data and widens no
existing owner.
