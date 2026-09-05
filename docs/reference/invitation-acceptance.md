# Invitation acceptance contract

An invitation offers membership at one school. It is not proof of ownership
of an existing global account.

## Registration and membership

1. New email addresses use the public registration flow, including password
   validation. Acceptance creates the account and school identity once.
2. Existing accounts require an independently authenticated backend access
   token for the invited account. Passwords, MFA enrollment, global active
   state, and memberships at other schools remain unchanged.
3. Tenant, organisation, parent, and school sessions can prove ownership.
   Operator accounts have a separate identity namespace and cannot do so.
   Preview, unfinished MFA, expired, unsigned, and unknown-scope tokens fail.
4. Disabled global accounts cannot join through invitation acceptance, even
   with a previously issued access token.
5. Membership, school identity, role assignment, and one-time consumption
   commit together. Rejection or a provisioning failure rolls back the changes.

The backend validates the session signature, expiry, scope, and account ID.
Frontend session claims and account IDs in request bodies grant no authority.
Normal portal login, including MFA, remains the only UI authentication path.

## Wire contract

`GET /auth/invitations/{token}` returns `requires_account_login` and
`target_portal` (`tenant` or `school`) with the existing invitation details.

`POST /auth/invitations/{token}/accept` accepts registration data for a new
account. Existing accounts provide their backend session in
`Authorization: Bearer ...`; password fields are ignored for that branch.

| Condition | HTTP | Code |
| --- | --- | --- |
| Missing or invalid owner session | 401 | `INVITATION_ACCOUNT_LOGIN_REQUIRED` |
| Session belongs to another account | 403 | `INVITATION_ACCOUNT_MISMATCH` |
| Global account disabled | 403 | `ACCOUNT_INACTIVE` |
| Accepted | 201 | Existing account response shape |
| Expired, revoked, or consumed invitation | 410 | Existing invitation error |

The frontend acceptance proxy reads only the current host's portal session,
forwards its backend token, and preserves refreshed session cookies. It requires
a same-origin request before attaching that session. It never forwards a
caller-supplied account ID or ownership token from the JSON body.

## Copyable links, resend, and imports

Creation responses retain the invitation token for the existing copy-link UI.
Such a link authorizes new-account registration, or presents an existing-account
membership offer requiring independent authentication. It cannot replace an
existing account's credentials or accept on that account's behalf.

Pending-list responses omit tokens. Resend responses contain no token and reuse
the pending invitation. Staff imports and Lehrkraft invitations use the same
acceptance service and ownership check; imported person records are linked only
after acceptance succeeds.

Existing users open the invitation at their usual moto address, sign in in a
separate tab, and return to confirm. The page offers school, parents, and school
portal addresses without transferring session tokens between hosts.
