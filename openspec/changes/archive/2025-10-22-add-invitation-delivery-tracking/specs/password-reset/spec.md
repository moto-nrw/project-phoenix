# Password Reset Delivery Tracking

## ADDED Requirements

### Requirement: Password Reset Email Delivery Tracking

The system SHALL track delivery status for password reset emails.

#### Scenario: Reset email delivery tracked
- **WHEN** a password reset email is sent
- **THEN** the reset token record stores delivery metadata
- **AND** the system tracks success/failure status

### Requirement: Password Reset Initiation
The system SHALL allow users to request a password reset via email and track delivery status.

#### Scenario: Reset email delivered successfully
- **WHEN** a valid password reset is requested and the email sends successfully
- **THEN** the reset token record stores `email_sent_at`
- **AND** clears any previous `email_error`
- **AND** the API exposes `delivery_status = "sent"` for operational monitoring.

#### Scenario: Reset email send failure
- **WHEN** the password reset email fails to send
- **THEN** the reset token record captures the error reason in `email_error`
- **AND** sets `delivery_status = "failed"`
- **AND** queues retries up to the configured limit while the token remains valid.

## ADDED Requirements

### Requirement: Password Reset Failure Notifications
The system SHALL notify administrators when password reset emails cannot be delivered after all retries.

#### Scenario: Exhausted reset email retries
- **WHEN** all retry attempts for a password reset email fail
- **THEN** the system emits an admin alert detailing the account email and final error
- **AND** logs an audit event referencing the reset token.
