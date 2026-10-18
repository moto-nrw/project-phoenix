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
   account in `auth.accounts`) and the school's invitation
   (`auth.caterer_invitations`) are identity data with `write_owner:
   identity-access`. A caterer account is an ordinary `auth.accounts` row: the
   password, lockout and reset flows stay the existing ones.
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
4. **One tenant transaction per school.** The caterer portal is a workflow
   (`workflows/catererportal`, owner `caterer-portal`) in the shape of ADR
   0022. It resolves the caterer's active supplies, then reads each school's
   order in that school's own tenant transaction through the Meal Plan's
   public capability. There is no query across schools.
5. **Numbers by default.** The caterer reads orders, changes to them and
   counts per day and class. Children's names reach it only while the school's
   `gdpr.meal_caterer_names_enabled` is on. The reason a child does not eat
   never reaches it. Its only write in V1 is confirming an order.
6. **Password first.** Caterer accounts sign in with e-mail and password,
   like parent accounts. A second factor is a follow-up: the existing MFA is
   bound to a school's policy, and a caterer has no school.

## Consequences

- `auth.token_portal_scope` and every scope switch gain the value `caterer`;
  the wrong-portal tests cover it.
- The policy registers the owner `caterer-portal` (workflow) and assigns the
  three identity tables to `identity-access` and the supply table to
  `mealplan` in the same change.
- Deployments need `CATERER_URL` and `NEXT_PUBLIC_CATERER_HOSTNAME`, DNS and
  TLS for the new host before the release that contains the portal.
- A caterer account cannot sign in to any other portal, and no other account
  can sign in to the caterer portal.
