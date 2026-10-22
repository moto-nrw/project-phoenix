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

1. **Identity & Access owns the caterer.** The Caterer-Betrieb
   (`auth.caterers`), its accounts (`auth.caterer_accounts`, one row per login
   account in `auth.accounts`), the schools it supplies
   (`auth.caterer_schools`) and the school's invitation
   (`auth.caterer_invitations`) are identity data with `write_owner:
   identity-access`. A caterer account is an ordinary `auth.accounts` row: the
   password, lockout and reset flows stay the existing ones. One caterer
   supplies any number of schools of any Träger; a second school's invitation
   to the same address adds the school to the existing account, confirmed
   with its password. An address that holds another kind of account cannot
   accept.
2. **The supply is Meal Plan data.** The link of a school to a caterer, the
   Belieferung (`schedule.meal_caterer_supplies`), is tenant data of the
   school, under RLS, owned by `mealplan`. The school starts it with an
   invitation and ends it at any time; ending it removes the caterer's access
   to that school at once. In V1 a school has at most one active supply.
3. **A portal scope of its own.** Caterer sessions carry the JWT scope
   `caterer` and no tenant, like parent sessions. `CatererMiddleware` accepts
   only that scope; the tenant, school and parent middlewares reject it, and
   the caterer middleware rejects all of theirs. The portal has its own host
   (`NEXT_PUBLIC_CATERER_HOSTNAME`), NextAuth instance, host-only cookie and
   base path, as the other portals do.
4. **No new administrative transaction (#4088).** The data of each school is
   separated by RLS, and #4088 allows no new administrative transaction.
   The caterer therefore never searches across schools:
   - The invitation goes to the caterer by mail only; the school never sees
     the link, so only the owner of the address can accept. Identity stores
     the link's hash; the queued mail holds the link until the outbox's
     delivery retention removes it, as with the guardian invitation.
   - The invitation link names its school. Preview and acceptance open that
     school's transaction; the account, the caterer, its link to the school,
     the spent invitation and the Meal Plan's supply commit there together.
   - `auth.caterer_schools` names the schools a caterer supplies. The login
     and the portal read it, with the caterer of the account, through the
     existing global identity read (`globalIdentityRead`) the login already
     uses for accounts. The existing school-directory port filters these
     IDs to live, active schools in one bulk read before a tenant is opened.
     The school-list query budget is therefore three statements: the
     caterer, its links and the school directory, independent of school count.
   - The caterer portal is a workflow (`workflows/catererportal`, owner
     `caterer-portal`) in the shape of ADR 0022. It opens each school's own
     tenant transaction and reads the order through the Meal Plan's public
     capability.
   - Ending a supply ends the school's link in the same transaction; the
     caterer loses the school at once.
5. **Numbers by default.** The caterer reads orders, changes to them and
   counts per day and class. Children's names reach it only while the school's
   `gdpr.meal_caterer_names_enabled` is on. The reason a child does not eat
   never reaches it. It reads and confirms only days from the start of its
   supply on, so a new caterer never sees its predecessor's orders. Its only
   write in V1 is confirming an order.
6. **Password first.** Caterer accounts sign in with e-mail and password,
   like parent accounts. A second factor is a follow-up: the existing MFA is
   bound to a school's policy, and a caterer has no school.

## Consequences

- The `chk_tokens_portal_scope` check of `auth.tokens.portal_scope` and every scope switch gain the value `caterer`;
  the wrong-portal tests cover it.
- The policy registers the owner `caterer-portal` (workflow) and assigns the
  four identity tables to `identity-access` and the supply table to
  `mealplan` in the same change.
- A school's role may create a caterer and its account link in the
  acceptance, but never change or delete one. It reads caterers' business
  names, which are no personal data.
- Deployments need `CATERER_URL` and `NEXT_PUBLIC_CATERER_HOSTNAME`, DNS and
  TLS for the new host before the release that contains the portal.
- A caterer account cannot sign in to any other portal, and no other account
  can sign in to the caterer portal.

## Budgets

The caterer reaches Identity & Access through its public module and engine,
like every other flow there. [ADR 0057](0057-only-the-target-is-a-rule.md)
point 5 lets the registers only fall; this decision raises them by exactly
what the caterer adds:

1. `modulePassthroughOwnerBudgets["modules/identityaccess"]` rises from 556
   to 560, and `modulePassthroughOwnerBudgetTotal` from 2,974 to 2,978: the
   engine method that deletes expired caterer invitations, next to the one
   of the guardian invitations, the entries of the caterer flows that
   attach the unit of work and translate the errors, and the read that
   names a school's caterer from its link to the school (the accepted
   invitation that started the supply is deleted after 30 days). The flows
   speak the public types themselves, so no engine forwards them.
2. `moduleComposeBudgets["modules/identityaccess/compose"]` rises from 1,729
   to 1,773, and `moduleComposeBudgetTotal` by the same 44 lines: composing
   the caterer flows beside the engine, as the demo access is, with the
   delivery of their invitation mail, and handing the authentication the
   caterer store for the refresh.
3. `modulePassthroughOwnerBudgets["modules/mealplan"]` rises from 15 to 17,
   and `modulePassthroughOwnerBudgetTotal` from 2,978 to 2,980: the supply's
   end and read each run one statement in the Meal Plan's unit of work. The
   supply methods already speak the public types, so no engine method
   forwards them; the caterer portal hands the invitation, its revocation
   and the sign-in through without a method of its own.
4. `moduleComposeBudgets` gains `workflows/catererportal/compose` with 36
   lines, and `moduleComposeBudgetTotal` rises by the same 36: binding the
   caterer portal to each school's tenant transaction.

Measured again after development contracted the registers and collapsed
the Meal Plan facade on 2026-10-20 (#4451), the same code raises
`modulePassthroughOwnerBudgets["modules/identityaccess"]` from 497 to 501
and, with ADR 0078, `modulePassthroughOwnerBudgetTotal` from 2,119 to
2,124 (from 2,024 to 2,029 after the Student Presence facade collapsed,
#4481). `modules/mealplan` needs no raise any more: its service serves the
public contract itself, so nothing forwards the supply.
`moduleComposeBudgets["modules/identityaccess/compose"]` rises from 1,312 to
1,357 (one more line imports the session dependencies, which development
moved to the public ports), and with ADR 0078 and the 36 lines of
`workflows/catererportal/compose` the total rises from 12,419 to 12,534.
The Meal Plan's public contract splits into `Menu`, `Registration` and
`Ordering`, each at most twelve methods wide; `Capability` embeds all three.

Measured once more after development collapsed the Identity & Access facade
(#4452) the same day: the owner value serves the caterer flows beside the
demo access, so `modulePassthroughOwnerBudgets["modules/identityaccess"]`
rises from 254 to 257 and, with ADR 0078, the total from 1,637 to 1,641.
`moduleComposeBudgets["modules/identityaccess/compose"]` rises from 1,304 to
1,349, and with ADR 0078 and `workflows/catererportal/compose` the total
from 12,407 to 12,522.

Measured again on 2026-10-21, after development drained the pass-throughs
(#4519, #4403) and contracted the compositions further: the same caterer
code adds 3 to `modulePassthroughOwnerBudgets["modules/identityaccess"]`
(218 to 221) and, with ADR 0078, the total rises from 1,418 to 1,422.
`moduleComposeBudgets["modules/identityaccess/compose"]` stays at 1,349
over development's 1,304, and with ADR 0078 and
`workflows/catererportal/compose` the total rises from 12,355 to 12,470.
On 2026-10-22, after development drained more pass-throughs and raised
the compositions for ADR 0076 (staff person repair), the raise stays the
same: `modules/identityaccess` from 215 to 218, the pass-through total from
1,409 to 1,413, and the compose total from 12,408 to 12,523. The meal
ordering budgets moved from ADR 0076 to ADR 0078 when development took the
number.

## Policy

Policy epoch 89 → 90 registers the data-less workflow owner `caterer-portal`
with its five new packages (ADR 0013 path, #3130) and the rules its code
uses: its public contract and ports speak the public Identity & Access and
Meal Plan contracts, its composition binds the tenant runtime, and its HTTP
transport renders through `api/common`. It owns no data and widens no
existing owner.
