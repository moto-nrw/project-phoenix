# Untangle: Identity & Access sign-in and second factor

Decision log of the structural refactor of `backend/modules/identityaccess`
(sign-in, sessions, second factor), the portal wiring around it and the
frontend sign-in and second-factor screens. Each fix stands above the
problems it removes. Status: `done` (with the PR), `open`, `kept` (a special
case the system still needs) or `dropped` (with the reason).

Paths without a prefix are relative to `backend/modules/identityaccess/`.

One test for every fix: what changes together ends up together, what does
not is not tied, and everything depends on what changes less.

## Plan of the PRs

1. Backend: portal facts and the voluntary factor (fixes A to E).
2. Backend: mail copy table and second-factor machinery (fixes F to K).
3. Frontend: sign-in and second-factor screens (fixes L to P).

## Fix A: a portal fact decides, not a portal name (done, PR 1)

A property of a portal is a field of `domain.Portal`
(`internal/domain/portal.go`). Shared code reads the field; no shared rule
names one portal.

- A1 done. `session_mfa.go`: the validation skipped the enrollment read with
  `scope == PortalParent.Name`. Now `Portal.FactorAtValidation` (caterer
  true). Same statements; budget `identity.session_validate.parent` unchanged.
- A2 done. Which remembered devices a call means was decided by name in the
  mint (`recheckTrustedDevice`), again in each portal's `bind` and in the
  Träger sign-in. Now `Portal.Device` (`DeviceRealm`) and one
  `portalDeviceContext`. `WithParentDevice` and `ParentDeviceOf` left the
  public package: nothing outside the owner called them.
  `WithTraegerDevice` stays public: the Träger portal binds it.
- A3 kept. `domain/mfa_mail.go` `ForPortal`: the parents' code mail is the
  parents' copy in the parent's language. That is a fact of that mail
  family, not a rule another portal could share.
- A4 done. `session_validation.go`: two `platform` branches in one function.
  Now each path prepares its check (`operatorSessionCheck`,
  `accountSessionCheck`) and the one administrative transaction runs it. A
  first cut with a transaction per path added a site to the enumerated
  register `test/transaction_boundaries.json`; the shape above keeps the
  site.
- A5 done. `session_refresh.go`: the caterer's moving storage school was
  decided by name in three places. Now `Portal.MovesSchool`,
  `lockSuccessorSchool` (reads the portal's own `keptAt`) and
  `successorSchoolID`.

## Fix B: one voluntary factor, two thin portals (done, PR 1)

`voluntaryFactor` owns the flow of a portal without a school: `answer`
(gate, then challenge or mint), `redeem` (verify, prepare the device, mint,
remember), `enable` (confirm, enroll, mint), `proven` (mint with a proof),
switch-off and devices. A portal supplies who belongs to it (`member`), the
session it mints (`mint`) and its refusal for a factor that is not on.

- B1 done. Two wrapper layers in two styles, each with its own
  `mintProvenSession`. Both portals now run `runFlow` over the factor; the
  caterer's private one-line forwards (`login`, `signInGate`, `verifyMFA`,
  `resendMFA`, `confirmMFA`) are gone. The caterer's passkey sign-in mints
  through `proven` too.
- B2 done. The factor is built once per portal.
- B3 done. `errCatererMFAUnavailable` is gone; `catererHost` moved to the
  passkey flows that read it.
- B4 done. `ComposeParentAccess` no longer swallows a construction error:
  the constructor it called repeated the composition's own guard, so the two
  are one function without an error path.
- B5 done. The parents' shapes were aliases of `Caterer*` types, and
  `CatererSession.Email` existed for the parents. The shapes are now
  `Voluntary*` (`voluntary_factor.go`), and both portals alias them.
- Kept: the per-portal method names of the public contract
  (`VerifyCatererMFA`, `VerifyParentMFA`, ...) and with them the one-line
  adapters in `caterer_mfa.go` and `parent_mfa.go`. One neutral interface
  would remove them, but it re-keys 16 ports of the command-effects register
  and turns three factor methods into registered pass-throughs (ADR 0075
  budget is at its floor). That needs its own decision. Until then each
  adapter reaches its factor through `voluntary()`, as the parents' did
  before: the pass-through scan counts a call through a receiver field, and
  spelling the adapters that way would register sixteen of them.

## Fix C: a portal's refusals live with the portal (done, PR 1)

- C1 done. `ParentLoginRules` and `ParentSecondFactorRules` moved from
  `inbound/portalauth` to `inbound/parentauth/errors.go`.
- C2 done. `portalauth.ListTrustedDevices`, `RevokeTrustedDevice` and the new
  `SwitchOffFactor` take the portal's refusal (`portalauth.Refusal`). The
  caterer's copies of all three handlers and the parents' copy of the
  switch-off are gone.
- C3 done, bug fix. The parents' device routes answered through `MFARules`
  alone, so every refusal of the session tables answered 500 there: a
  module without the factor (now 503), an ended or deleted account (401),
  an invalid token (401), an unknown school (404), the wrong-portal and
  no-role refusals (403). They now use the parents' table like the other
  settings routes, and an unclassified error answers the fixed text
  "parent second factor failed" instead of the error's own. Test:
  `TestParentTrustedDevicesAnswerLikeTheSecondFactorSettings` (fails on the
  old table).
- Open: the caterer's password change sits in
  `workflows/catererportal/http/account.go`, its switch-off in
  `inbound/catererauth` (third copy of the wrong-password field refusal).

## Fix D: remove what nothing calls (done, PR 1)

- D1 done. `LoginParentWithAudit` (public, application, `inbound/account`
  port): no production caller, and it skipped the second-factor gate. Tests
  sign in through `Parents().LoginParent`.
- D2 done. `FindSchoolPortalTenant`: no caller.
- D3 done. `traegerParentOrganizations` forwarded to
  `liveTraegerParentOrganizations`.
- D4 dropped. `SessionProvesInvitationOwnership` and `PortalOrg.Claim`: the
  audit called the case unreachable; `TestNativeInvitationOwnershipProof`
  pins it. It stays.

## Fix E: one copy of a rule (PR 1)

- E1 done. The live-school filter existed three times; the caterer's two
  copies now call `liveSchoolIDs`.
- E2 kept. `findSchoolPortalTenantForAccount(…, requireBooked bool)`: two
  functions would each open an administrative transaction, one more site in
  `test/transaction_boundaries.json`.
- E3 done. `keptAtNoSchool` replaces two identical closures.

## Fix F: the parents' mail copy is one table (done, PR 2)

- F1 done. Four tables keyed by language became `parentCopies`, one struct
  per language. The rendered content of the code mail, the device notice
  and the reset notice in every language is byte-identical (dumped before
  and after, `cmp`).

## Fix G to K: second-factor machinery (PR 2)

- G1 done. Public constants nothing read are gone
  (`MFATrustedDeviceCookieDefaultDays`, `AdminRoleName`, `MFAMode*`,
  `MFAEmailCodeLength`, `MFAEmailRateLimit*`, `MFALockoutDuration`,
  `OperatorMFAChallengeTTL`, `OperatorMFARateLimitWindow`,
  `OperatorMFATrustedDeviceDuration`), with the comment that named a
  fallback the gate does not have. The three that tests read stay and are
  pinned to the domain's values (`TestPublicCodePolicyIsTheDomains`): the
  domain may not import the public package.
- H1 done in part. The send cap is one rule (`codeSendCap`), and the
  operator flow draws its code with `newMFAEmailCode` like the account
  flow. Kept: the device token, cookie check and revoke loop stay twins;
  they differ in table, binding and audit ledger, and a shared helper would
  hold two or three lines each.
- I1 done in part. `OperatorMFAFlows.ResendChallenge` is private: one
  public name per operation (pass-through budget 207 to 206). Kept:
  `Enroll`, `Disable` and `IssueTrustedDevice` beside their `…Operator…`
  names. Each holds an administrative transaction, and
  `test/transaction_boundaries.json` accepts no renamed site.
- J1 open. The public-typed passkey record methods
  (`internal/ports/records.go`, `AccountPasskeyQuery` and its siblings)
  serve only `integration/{account,operator}_passkey_test.go`. Removing
  them means rewriting 618 test lines against the domain errors; that is a
  change of its own.
- K1 kept. `AccountMFAFlows.IssueTrustedDevice` reads its prepared policy
  from a context key. Passing the policy as a parameter moves the
  administrative transaction into a new function, which the transaction
  register refuses. The public method has no production caller but about 36
  test call sites; removing it is a change of its own.
- Kept: the code length exists in the domain and in `api/common` (wire
  validation). `api/common` may not read the owner's domain.

## Fix Q: the sub-portal facts are one table (done, PR 2)

A session of the tenant or the parents portal that names a Träger follows
its own rules (Träger school, Träger parent). Which kinds name a Träger,
whether their rows name a school and how a token says so was decided in six
places by kind or portal name.

- Q1 done. `domain.traegerKinds` lists the kinds that name a Träger with
  the school their rows demand; `SessionKind.NamesTraeger`,
  `AccountSession.traegerShapeHolds` and `KeptAtSchool` read it, and so does
  `sameTraeger` in the refresh.
- Q2 done. `Portal.TraegerKind` names a portal's sub-portal
  (`SessionKindOf` reads it), and `Portal.OrgNamesSession` says whether the
  org_id of a token names the Träger of its session
  (`SessionClaims.SessionKind`, `refreshNamesTokenTraeger`).
- Proof: `TestSessionRowsNameATraegerOnlyInTheirKind` passes unchanged on
  the old rules and on the table.
- R1 kept. The gate's `IsRequired` (`internal/ports/policy.go`) takes an
  `email` its only implementation ignores. Without it
  `CapabilityGate.IsRequired` changes its registered pass-through text and
  form, which the register accepts only with a new decision.

## Fix L to P: frontend (PR 3)

- L1 `app/api/parent/me/mfa/{confirm,disable}` copy the caterer's routes.
- M1 `lib/caterer-mfa-api.ts` takes `emailCodeFactorApi` apart and the
  account page puts it together again.
- N1 Three mappings of the sign-in answer and six local `MFAStep` types.
- O1 Session notice and stale-session handling copied on the sign-in pages.
- P1 `mfa-admin-override-modal`: `scope` plus three optional ids with
  runtime throws; `headingLevel` never passed.

## Special cases that stay

- The parent validation does not read the enrollment (contract and query
  budget); only its place changes (A1).
- `ErrParentMFANeedsSchool`: a lost factor is reset through a school of the
  account.
- A device that cannot be prepared or stored never fails the sign-in.
- The parents portal's refusals come before the code is mailed.
- The caterer's sign-in answer uses `bool,omitempty`, the other portals a
  pointer: the JSON differs.
- The two send-cap checks: the early one saves the hash and the mail, the
  one under the lock is the race-safe one.
- The retry in `persistNewSession`: bounded, only on a family collision of
  concurrent sign-ins.
- The portal table exists in `tokenprimitives` and in the domain: the domain
  may not import its owner's packages; a test pins them to each other.
- Parent mail templates stay apart from the German ones: other wording,
  `lang`, no label styling for non-Latin scripts.

## Tests

PR 1:

- `LoginParentWithAudit` call sites (`behavior/traeger_parent_session_test.go`,
  `behavior/auth_service_shim_test.go`, `api/password_login_protection_test.go`,
  `internal/application/mint_live_account_internal_test.go`): the call
  changed to the parents portal's sign-in, every assertion is unchanged.
- Added `TestParentTrustedDevicesAnswerLikeTheSecondFactorSettings`.

PR 2:

- `parent_security_mails_test.go`: three `assert.Len` comparisons of the
  tables removed. One table of one struct per language cannot hold a
  language with a missing part; the test still renders every language.
- `operator_mfa_flow_internal_test.go`: no change (the renamed
  `resendChallenge` has no test caller).
- Added `TestPublicCodePolicyIsTheDomains` and
  `TestSessionRowsNameATraegerOnlyInTheirKind`.
