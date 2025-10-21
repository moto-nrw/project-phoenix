# User Invitation Delivery Tracking

## MODIFIED Requirements

### Requirement: Admin-Initiated User Invitations
The system SHALL allow administrators to invite users by email with a specific role assignment and persist delivery status for each invitation email.

#### Scenario: Invitation email sent successfully
- **WHEN** an admin creates an invitation and the email is delivered
- **THEN** the invitation record stores `email_sent_at` with the send timestamp
- **AND** clears any previous `email_error` value
- **AND** the API response includes `delivery_status = "sent"`

#### Scenario: Invitation email send failure
- **WHEN** an admin creates an invitation and the email send fails
- **THEN** the invitation record stores the failure reason in `email_error`
- **AND** `delivery_status = "failed"` is returned to the admin API
- **AND** the system schedules retries up to the configured limit

## ADDED Requirements

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
