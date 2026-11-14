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
4. Backend follow-ups: the passkey record surface (J1) and the four small
   items of the final round (fix T).
5. Frontend follow-ups: one reading of the sign-in answer (N1) and one
   stale-session sign-out (O1).

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
- J1 done (PR 4). The owner no longer serves passkey rows: the public-typed
  record methods (`ListActiveAccountPasskeys` to
  `ConsumeAccountPasskeySession` and the operator's seven), the record ports
  `AccountPasskeyQuery`, `…Command`, `…Records` and their operator siblings,
  the two embedded engine fields and the public row types
  (`AccountPasskeyCredential`, `AccountPasskeySession`, the operator's two,
  `PasskeySessionPurpose*`) are gone. The ceremonies keep the domain-typed
  record service they always ran on (`application.AccountPasskey`,
  `OperatorPasskey`). The tests drive the real ceremonies with a software
  authenticator (`behavior/passkey_authenticator_test.go`); see Tests, PR 4.
  Not possible: an observer option on the root's auth test module
  (`api.NewAuthTestModule`). Its body is command-effect evidence of 14
  care-plan exception bindings, and a changed exception binding needs an
  approval ADR. The stable operation names are asserted on the record
  service instead.
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

Paths are relative to `frontend/src/`.

- L1 done. `app/api/parent/me/mfa/confirm` copied the caterer's route line
  for line (50 lines, the parents' copy without a test). Both are now
  `createFactorConfirmRoute` (`lib/factor-routes.server.ts`) with their
  path, retry reason and log names; the switch-off routes share
  `currentPasswordBody`. The retry literal stays in each route file:
  `credential-route-coverage.test.ts` reads it there.
- M1 done. `lib/caterer-mfa-api.ts` took `emailCodeFactorApi` apart into
  four functions and the account page put them together again. The page
  now calls `emailCodeFactorApi("/api/caterer/account/mfa")` like the
  parents' settings.
- N1 done (PR 5). `lib/sign-in-answer.ts` reads the answer of every
  portal's password step (`signInAnswer`) and declares the one `MFAStep`;
  the six login pages keep the answer as their code step. The wire
  differences are a table per portal, not code: a missing
  `trusted_device_enabled` means „remember“ for tenant, school and operator
  (pointer field) and „nothing to remember“ for the caterer (`omitempty`), the parents portal
  and the Träger-Portal; only the Träger-Portal refuses an answer it cannot
  read (502). The caterer's invitation page uses the same `MFAStep`. Kept: three transports. `traegerLogin` names the Träger and asks
  for the code on a remembered device, `portalPasswordLogin` builds its
  refusal from the response, `mfa-api` from the parsed body; they differ in
  what they send and throw, not in what they read. Kept: the default of 90
  days stays in `MFAChallengeForm`; the pages' own `?? 90` repeated it.
- O1 done (PR 3 and 5). Clearing the session error from the address and
  the „Sitzung abgelaufen“ notice existed four times (tenant, school,
  Träger, caterer). Now `clearSessionErrorFromUrl` and
  `useSessionExpiredNotice` (`lib/login-session-notice.ts`). Signing a stale
  session out existed on all six login pages; now `signOutStaleSession`
  beside them. It answers null at once when there is nothing to end, so a
  page still reaches its form in the tick it did before. The differences
  that are real stay with their page: operator and parents show their
  waiting state while the sign-out runs, because their form is locked for
  as long as the session carries an error, and the parents page says so
  with „Wiederholen“ when the sign-out fails (`onFailure`). Not real: the
  Träger page started the sign-out without waiting and the three log names;
  one name now, `stale_session_signout_failed`, and the debug line
  `clearing_stale_session` of tenant and school is gone. Kept: the operator
  page reads `SessionExpired` itself; it has no deliberate-logout flag.
- P1 done. `MFAAdminOverrideModal` took `scope` and three optional ids, one
  of which each scope needed, checked by a throw at click time. It takes one
  `target` now (`{kind, …id}`); a target without its id does not compile.
  `EmailCodeFactorSection` lost `headingLevel`, which no caller passed.
- Kept: one `route.ts` per portal and path (the App Router and the route
  guards enumerate them); the trusted-device proxy of the parents portal
  under `/api/parent/auth/mfa/trusted-devices` (`TrustedDevicesSection`
  derives every portal's path the same way); the per-key copy mapping in
  `parent-sign-in-section.tsx` (next-intl checks the keys).

## Final round (after PR 1 to 3)

A last read-only hunt over the merged code found one defect and otherwise
only small items.

- S1 done, bug fix (#7084). A Träger-Portal session could not be renewed
  once the request named a client address: the refresh audited at the
  session's school, a Träger session has none, and only the parent session
  at a Träger fell back to the account's default school. The refresh now
  asks `AccountSession.KeptAtSchool` (the table of fix Q) and files through
  `schoollessAudit`; `traegerLoginAudit` was the same code and is gone.
  Test: `TestTraegerSessionRefreshesWithAClientAddress` (fails on the old
  rule).

## Fix T: the four small items of the final round (done, PR 4)

- T1 done. The realm of a remembered device was read three ways: from the
  context keys, from `DeviceBinding` and from three comparisons in the
  device mail. `DeviceBinding.Realm()` (`internal/domain/account_mfa.go`) is
  the one reading of a binding; the device mail takes its three flags and
  its language from it. Kept: the mail keeps three flags, because
  `identityaccess.TrustedDeviceMail` is the public contract Delivery renders
  and the public package cannot name the domain's realm. Kept: the context
  still carries the Träger through the public `WithTraegerDevice`, which the
  Träger portal binds; `deviceBinding` stays the one place that turns a
  context into a binding.
- T2 done. `Portal.TrustedDevice` was true for every portal. The field and
  its two branches are gone (`mfaChallengeResult`, `challengeDeviceHint`);
  `Portal.Device` says whose devices a portal remembers. No test named the
  field: `TestPortalTableMatchesTheTokenGates` compares claims, shapes,
  `Challenge` and `Enrollment`, so nothing was dropped or rewritten.
- T3 done. „The schools a caterer supplies now“ (caterer of the account, its
  supplies, narrowed to live schools) is `AccountAuthentication.suppliedSchools`.
  The successor school of a refresh (`catererSessionSchool`) and the caterer
  portal (`CatererAccess.catererSchools`, inside its global identity read)
  call it; `findCaterer` is gone. Both read through the same store.
- T4 done. `CatererAccess.tenantless` marked the context by hand, which the
  factor's `bind` already does for its portal. It now calls
  `voluntary().bind`.
- Kept: `changePasswordInTx` (`caterer_password.go`) reads the caterer and its supplies without
  the live-school filter. It only needs a school to file a refused password
  at, reads the supplies after the password check inside the login lock, and
  `suppliedSchools` would move that read ahead of the check.
- Kept: `withoutSchoolLedger` asks the context for a Träger. The Träger
  routes bind `WithTraegerDevice` without the ledger mark.
- Registers: 63 `platform` bindings of the command-effects register changed
  their digest (the functions on their path changed); each carries a
  re-review sentence. No exception binding, no pass-through and no
  transaction site changed.

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
  tables removed: one table of one struct per language cannot hold a
  language without one of the mails. The lengths also caught a language
  without a footer; the test now asserts the footer and its label on every
  rendered mail instead.
- `operator_mfa_flow_internal_test.go`: no change (the renamed
  `resendChallenge` has no test caller).
- Added `TestPublicCodePolicyIsTheDomains` and
  `TestSessionRowsNameATraegerOnlyInTheirKind`.

PR 3:

- `app/caterer/konto/page.test.tsx`: the mock moved from the deleted
  `caterer-mfa-api` to `email-code-factor-api`; assertions unchanged.
- `components/auth/mfa-admin-override-modal.test.tsx` and its stories: the
  props `scope` plus id became `target`; assertions unchanged.
- `app/api/route-cache-policy.test.ts`: `createFactorConfirmRoute` joined
  the register of factories that answer `private, no-store` (it runs through
  the portal's `withAuth`, which the two routes called directly before).
- `lib/personal-storage.test.ts`: the inventory names
  `login-session-notice.ts` as the reader of the logout flag in place of the
  four login pages.

PR 4:

- `integration/account_passkey_test.go` and `operator_passkey_test.go`
  (618 lines) called the removed record methods. Their behaviors moved to
  `behavior/account_passkey_ceremonies_test.go` and
  `operator_passkey_ceremonies_test.go`, which register and sign in through
  the public ceremonies against Postgres:
  - credential lifecycle (identity and time of a new credential, listing
    order, another holder's credential not listed, use recorded with the
    new credential state, a credential only under its own user handle,
    revocation by its holder only, a revoked passkey never verifies):
    `Test…PasskeyCredentialLifecycle`.
  - a credential ID once platform-wide, and the rollback of consumption and
    insert with a retry of the same ceremony:
    `Test…PasskeyRegistrationRollsBackWhenTheCredentialIsTaken`. The old
    test injected a failure after each write into its own transaction; the
    new one fails the second write in the flow's transaction, which rolls
    the first back too.
  - a ceremony once, only for its purpose, only before it expires, and now
    also only for the identity that started it:
    `Test…PasskeySessionIsConsumedOnce`.
  - rows visible from every tenant context, the school of a ceremony taken
    from its row: `Test…PasskeyRecordsAreVisibleFromEveryTenantContext`
    (the account test now also proves the refusal at a foreign school).
  - a passkey revoked during a login is not updated: the old test called
    the use on a revoked row; `Test…PasskeyRevokedDuringALoginIsNotUpdated`
    commits a revocation while the login waits for the row.
  - what the store calls active (a credential only under its own handle
    and only until revoked, no use or second revocation of a revoked one,
    a ceremony row written and consumed once, the rows each statement
    reports): `Test…PasskeyStoreServesActiveCredentialsOnly` and
    `Test…PasskeyStoreConsumesACeremonyOnce`
    (`internal/adapters/postgres`). The ceremonies cannot show these alone:
    the WebAuthn library refuses a foreign handle or a missing credential
    before the store's filter would.
  - a record the domain refuses never reaches the store:
    `Test…PasskeyRecordsValidateBeforeTheStore`; the operator session's
    refusals joined `TestPasskeySessionValidate`.
  - stable operation names, rows and error class:
    `Test…PasskeyRecordsObserveStableOperations`
    (`internal/application`), on the record service with a store double,
    now for all seven operations. The statement count of one was a constant
    of the store, not a measurement.
  - dropped: the texts of the public wrappers („identity access: create
    account passkey: …“) left with the wrappers; the validation they
    wrapped is asserted in `internal/domain/passkey_test.go`. A second
    insert of one ceremony ID cannot happen through a flow that draws the
    ID.
- `integration/{account,operator}_passkey_runtime_evidence_test.go` removed
  (353 lines). They measured the removed record methods for the cutover of
  #2724, ran only with `PHX_TEST_RUNTIME_EVIDENCE=1` and have no evidence
  file; their two contract assertions (replay refused, duplicate refused)
  are in the tests above.
- `inbound/account/behaviortest/identity_lists_growth_query_budget_test.go`:
  the passkey rows of the budget fixture come from
  `testpkg.CreateTestAccountPasskey`; statements, budget and assertions are
  unchanged.
- Added `TestDeviceBindingNamesOneRealm`.

PR 5:

- No existing test changed: the page tests of tenant, operator and caterer
  mock the wire, those of school, parents and Träger their transport, and
  all pass as they were.
- Added `lib/login-session-notice.test.ts`: nothing to end answers at
  once, a stale session is signed out once, a failed sign-out is reported
  and can be tried again.
- Added `lib/sign-in-answer.test.ts`: the code step of every portal, the
  two wire families of a missing remember-device field, the setup step, the
  Träger's choice and its refusal of an unreadable answer.
