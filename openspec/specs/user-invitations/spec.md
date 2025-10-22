# user-invitations Specification

## Purpose
TBD - created by archiving change add-email-invitation-system. Update Purpose after archive.
## Requirements
### Requirement: Admin-Initiated User Invitations
The system SHALL allow administrators to invite users by email with a specific role assignment.

#### Scenario: Create invitation successfully
- **WHEN** an admin creates an invitation with valid email and role_id
- **THEN** a unique invitation token is generated (UUID v4)
- **AND** the invitation is saved with a 48-hour expiry
- **AND** an invitation email is sent to the recipient
- **AND** the system returns the invitation details

#### Scenario: Email already has pending invitation
- **WHEN** an admin creates an invitation for an email with an existing pending invitation
- **THEN** the old invitation is marked as used (invalidated)
- **AND** a new invitation is created and sent

#### Scenario: Pre-fill recipient name
- **WHEN** an admin provides first_name and last_name when creating invitation
- **THEN** these values are stored in the invitation
- **AND** the invitation email uses the provided name
- **AND** the invitation acceptance form pre-fills these fields

### Requirement: Public Invitation Validation
The system SHALL allow anyone with an invitation token to validate it before accepting.

#### Scenario: Valid invitation token
- **WHEN** a user accesses /invite?token={token} with a valid, non-expired, unused token
- **THEN** the system returns invitation details (email, role name, pre-filled name, expiry)
- **AND** displays the invitation acceptance form

#### Scenario: Expired invitation token
- **WHEN** a user accesses /invite?token={token} with an expired token
- **THEN** the system returns ErrInvitationExpired from service layer
- **AND** API handler maps this to 410 Gone status
- **AND** displays "This invitation has expired" message

#### Scenario: Used invitation token
- **WHEN** a user accesses /invite?token={token} with a token that was already accepted
- **THEN** the system returns ErrInvitationUsed from service layer
- **AND** API handler maps this to 410 Gone status
- **AND** displays "This invitation has already been used" message

#### Scenario: Invalid invitation token
- **WHEN** a user accesses /invite?token={token} with a non-existent token
- **THEN** the system returns ErrInvitationNotFound from service layer
- **AND** API handler maps this to 404 Not Found status
- **AND** displays "Invitation not found" message

### Requirement: Invitation Acceptance and Account Creation
The system SHALL create user accounts atomically when invitations are accepted.

#### Scenario: Accept invitation successfully
- **WHEN** a user submits the acceptance form with valid first_name, last_name, and password
- **THEN** the system creates a Person record with the provided name
- **AND** creates an Account record with the email and hashed password
- **AND** assigns the role specified in the invitation
- **AND** marks the invitation as used
- **AND** all operations succeed or fail atomically (transaction)
- **AND** redirects to the login page

#### Scenario: Accept with weak password
- **WHEN** a user submits acceptance with a password that doesn't meet strength requirements
- **THEN** the system returns a 400 Bad Request
- **AND** displays password strength requirements
- **AND** does not create any records

#### Scenario: Accept with mismatched passwords
- **WHEN** a user submits acceptance with password and confirm_password that don't match
- **THEN** the system returns a 400 Bad Request
- **AND** displays "Passwords do not match" error

#### Scenario: Account creation failure rollback
- **WHEN** accepting an invitation and account creation fails
- **THEN** all database changes are rolled back
- **AND** the invitation remains unused
- **AND** the user can retry

#### Scenario: Invitation email already registered
- **WHEN** an invitation is accepted for an email address that already belongs to an active account
- **THEN** the system returns a 409 Conflict
- **AND** displays "This email already has an account" guidance
- **AND** leaves the invitation unused so an administrator can revoke or resend if needed

### Requirement: Invitation Management for Admins
The system SHALL provide admin tools to manage pending invitations.

#### Scenario: List pending invitations
- **WHEN** an admin accesses the invitations page
- **THEN** the system displays all invitations where used_at is NULL and expires_at > now
- **AND** shows email, role name, created_by, expires_at for each
- **AND** provides resend and revoke actions

#### Scenario: Resend invitation email
- **WHEN** an admin clicks "Resend" for a pending invitation
- **THEN** the system sends the invitation email again
- **AND** updates the invitation's updated_at timestamp
- **AND** does NOT change the expiry time or token

#### Scenario: Revoke invitation
- **WHEN** an admin clicks "Revoke" for a pending invitation
- **THEN** the system marks the invitation as used (soft delete)
- **AND** the invitation can no longer be accepted
- **AND** removes it from the pending list

#### Scenario: Resend expired invitation
- **WHEN** an admin tries to resend an expired invitation
- **THEN** the system returns a 400 Bad Request
- **AND** displays "Cannot resend expired invitation" error
- **AND** suggests creating a new invitation

### Requirement: Invitation Email Content
The system SHALL send branded invitation emails with secure tokens.

#### Scenario: Invitation email delivered
- **WHEN** an invitation is created
- **THEN** an email is sent to the recipient with:
  - Subject: "You're Invited to Project Phoenix"
  - Personalized greeting (if name provided)
  - Role information
  - Invitation URL with token
  - Expiry notice (48 hours)
  - Moto branding and colors

#### Scenario: Invitation URL format
- **WHEN** generating an invitation email
- **THEN** the invitation URL is: {FRONTEND_URL}/invite?token={token} (query parameter, not path param)
- **AND** the token is a UUID v4 (cryptographically secure)
- **AND** the URL uses HTTPS in production (validated at service initialization)

### Requirement: Invitation Token Security
The system SHALL ensure invitation tokens are cryptographically secure and time-limited.

#### Scenario: Secure token generation
- **WHEN** an invitation is created
- **THEN** the token is generated using UUID v4 (crypto-secure)
- **AND** the token is unique in the database
- **AND** the token is URL-safe

#### Scenario: Token expiry enforcement
- **WHEN** 48 hours pass after invitation creation
- **THEN** the token is considered expired
- **AND** cannot be used for account creation
- **AND** validation returns 410 Gone

#### Scenario: Single-use token
- **WHEN** an invitation token is used to create an account
- **THEN** the used_at timestamp is set
- **AND** the token cannot be reused
- **AND** subsequent attempts return 410 Gone

### Requirement: Invitation Token Cleanup
The system SHALL automatically clean up expired and used invitation tokens.

#### Scenario: Daily automated cleanup
- **WHEN** the nightly cleanup job runs
- **THEN** all invitations where (expires_at < now OR used_at IS NOT NULL) are deleted
- **AND** the count of deleted tokens is logged

#### Scenario: Manual cleanup command
- **WHEN** an admin runs `go run main.go cleanup invitations`
- **THEN** expired and used tokens are deleted immediately
- **AND** the result is displayed in the terminal

### Requirement: Invitation Permissions
The system SHALL restrict invitation management to users with appropriate permissions.

#### Scenario: Create invitation requires permission
- **WHEN** a user without users:create permission tries to create an invitation
- **THEN** the system returns 403 Forbidden
- **AND** no invitation is created

#### Scenario: List invitations requires permission
- **WHEN** a user without users:list permission tries to list invitations
- **THEN** the system returns 403 Forbidden

#### Scenario: Manage invitations requires permission
- **WHEN** a user without users:manage permission tries to resend or revoke
- **THEN** the system returns 403 Forbidden
- **AND** no changes are made

#### Scenario: Accept invitation is public
- **WHEN** any user (authenticated or not) accesses /invite?token={token}
- **THEN** they can view and accept the invitation
- **AND** no authentication is required

### Requirement: Invitation Delivery Visibility
The system SHALL expose invitation email delivery status to administrators.

#### Scenario: List pending invitations with status
- **WHEN** an admin lists pending invitations
- **THEN** each record includes delivery fields (`delivery_status`, `email_sent_at`, `email_error` if present)
- **AND** invitations with repeated failures trigger an admin-visible alert message.

#### Scenario: Resend invitation clears failure state
- **WHEN** an admin resends an invitation after a failure
- **THEN** the resend attempt resets `delivery_status` to `pending`
- **AND** records the next success or failure details after the attempt completes
- **AND** increments a stored retry counter used for alerting.

