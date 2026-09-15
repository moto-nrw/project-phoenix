---
status: accepted
---

# Test roles may import test infrastructure directly

[#3215](https://github.com/moto-nrw/project-phoenix/issues/3215) asks whether a
test role may reach the ORM, the HTTP router and the test framework directly, or
must go through its owner's facade. The migration programme in
[#2580](https://github.com/moto-nrw/project-phoenix/issues/2580) answers yes for
exactly these three external classes.

The policy already expressed this permission 65 times, once per owner and role,
under `target_class` `orm-sql` and `http-router`, and twice owner-agnostically
for `test-support`. Every one of those rules says the same thing: a test may
talk to the database it verifies and to the router it drives. Repeating that per
owner produced 133 baseline entries across 18 open Contract tickets whose only
content was "this owner's test has not been granted the same permission yet".
Granting it per owner as each ticket lands is work with no design content.

An HTTP adapter test that may not import `net/http` or `chi` asserts through a
proxy for no benefit. A repository test that may not open a transaction cannot
prove tenant isolation. Neither facade exists to hide infrastructure from the
test that exercises it. The facade rule stays in force where it carries meaning:
production roles, first-party targets, and every external class that is not
test infrastructure.

## Policy registration

Policy epoch 6 registers one owner-agnostic rule per test role and
infrastructure class (`external.<class>.<role>`), replaces the 65 owner-specific
rules they subsume, and removes the 133 baseline entries they resolve. The
comparison in `backend/internal/architecture/policy_compare.go` admits this
shape, and only this shape, under a reviewed epoch: no `source_owner`, a test
role, a `target_class` in `orm-sql`, `http-router` or `test`, and no production
scope unless the role is `test-support`. Owner-specific grants, first-party
targets, production roles and every other external class remain under the
ordinary loosening guards.

This decision changes no runtime write owner, table, HTTP route, status code or
error string. It does not permit a test to bypass a domain boundary: a test may
still not import another owner's application or domain package unless a rule
names that seam.
