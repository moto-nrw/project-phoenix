# Password Reset Specification

## MODIFIED Requirements

### Requirement: Password Reset Initiation
The system SHALL allow users to request a password reset via email.

#### Scenario: Request password reset with valid email
- **WHEN** a user submits their email on the forgot password page
- **THEN** the system creates a password reset token with 30-minute expiry
- **AND** sends an email with a reset link to {FRONTEND_URL}/reset-password?token={token}
- **AND** invalidates any existing reset tokens for that account
- **AND** returns success message regardless of whether email exists (prevents enumeration)

#### Scenario: Request password reset with non-existent email
- **WHEN** a user submits an email that doesn't match any account
- **THEN** the system returns the same success message as valid email
- **AND** does not send any email
- **AND** does not reveal that the email doesn't exist

#### Scenario: Password reset email delivered
- **WHEN** a valid password reset is requested
- **THEN** an email is sent asynchronously (goroutine) with:
  - Subject: "Password Reset Request"
  - Reset URL with query parameter: {FRONTEND_URL}/reset-password?token={token}
  - Expiry notice (30 minutes)
  - Security disclaimer ("If you didn't request this...")
  - Moto branding and colors
- **AND** the email send does not block the API response

#### Scenario: Password reset email send failure
- **WHEN** the email fails to send due to SMTP errors
- **THEN** the error is logged for investigation
- **AND** the reset token is still created
- **AND** the API returns success to the user
- **AND** the user can potentially access the reset link another way

### Requirement: Password Reset Completion
The system SHALL allow users to reset their password using a valid reset token.

#### Scenario: Reset password with valid token
- **WHEN** a user submits a new password with a valid, non-expired, unused token
- **THEN** the password is hashed and updated
- **AND** the reset token is marked as used
- **AND** all other tokens for that account are invalidated
- **AND** the user is redirected to the login page
- **AND** can immediately login with the new password

#### Scenario: Reset password with expired token
- **WHEN** a user tries to reset password with a token older than 30 minutes
- **THEN** the system returns a 400 Bad Request
- **AND** displays "This reset link has expired" message
- **AND** provides a link to request a new reset

#### Scenario: Reset password with used token
- **WHEN** a user tries to reset password with a token that was already used
- **THEN** the system returns a 400 Bad Request
- **AND** displays "This reset link has already been used" message

#### Scenario: Reset password with weak password
- **WHEN** a user submits a password that doesn't meet strength requirements
- **THEN** the system returns a 400 Bad Request
- **AND** displays password strength requirements
- **AND** does not update the password

### Requirement: Password Reset Rate Limiting
The system SHALL limit password reset requests to prevent abuse using per-email tracking.

#### Scenario: Rate limit enforcement
- **WHEN** a user requests more than 3 password resets for the same email within 1 hour
- **THEN** the system returns a 429 Too Many Requests
- **AND** displays "Too many reset attempts. Please try again later."
- **AND** includes a `Retry-After` response header indicating when the user may retry
- **AND** does not create a new token or send an email
- **AND** the attempt count is tracked in auth.password_reset_rate_limits table

#### Scenario: Rate limit reset after cooldown
- **WHEN** 1 hour has passed since the window_start timestamp
- **THEN** the rate limit counter resets to 1
- **AND** the window_start is updated to current time
- **AND** the user can request password resets again

#### Scenario: Rate limit tracking per email
- **WHEN** password reset is requested for email A
- **THEN** the rate limit is tracked separately from email B
- **AND** each email has its own 1-hour sliding window

#### Scenario: Rate limit storage
- **WHEN** a rate limit record is created
- **THEN** it is stored in auth.password_reset_rate_limits with email as primary key
- **AND** includes attempts count and window_start timestamp
- **AND** old records are cleaned up daily (> 24 hours old)

### Requirement: Password Reset Token Security
The system SHALL ensure reset tokens are cryptographically secure and short-lived.

#### Scenario: Secure token generation
- **WHEN** a password reset is initiated
- **THEN** the token is generated using UUID v4 (crypto-secure)
- **AND** the token is unique in the database
- **AND** the token is URL-safe

#### Scenario: Short expiry for security
- **WHEN** a reset token is created
- **THEN** it expires after 30 minutes (not 24 hours)
- **AND** cannot be used after expiry
- **AND** minimizes attack window if email is compromised
- **AND** the expiry is set at token creation time in InitiatePasswordReset

#### Scenario: Single-use token
- **WHEN** a reset token is used to change password
- **THEN** the used flag is set
- **AND** the token cannot be reused
- **AND** all other tokens for the account are also invalidated

### Requirement: Password Reset Audit Logging
The system SHALL log all password reset activities for security auditing.

#### Scenario: Reset request logged
- **WHEN** a password reset is requested
- **THEN** the system logs the email and timestamp using log.Printf
- **AND** logs include operation identifier for tracing

#### Scenario: Password change logged
- **WHEN** a password is successfully reset
- **THEN** the system logs the token and timestamp using log.Printf
- **AND** the API handler logs completion separately

#### Scenario: Failed reset attempts logged
- **WHEN** a reset fails due to invalid/expired token
- **THEN** the system logs the failure reason and token
- **AND** enables detection of abuse patterns through log analysis

#### Scenario: Email send logged
- **WHEN** a password reset email is sent
- **THEN** the mailer logs recipient, subject, and template before sending
- **AND** logs success or failure after SMTP attempt
- **AND** all email operations are traceable through logs

## ADDED Requirements

### Requirement: Password Reset Email Template
The system SHALL use a branded email template for password reset notifications.

#### Scenario: Reset email branding
- **WHEN** a password reset email is sent
- **THEN** it uses the moto logo, moto-blue (#5080d8) colors
- **AND** matches the visual style of other system emails
- **AND** includes a prominent "Reset Password" CTA button

#### Scenario: Reset email clarity
- **WHEN** a user receives the reset email
- **THEN** the email clearly explains what to do next
- **AND** prominently displays the expiry time (30 minutes)
- **AND** includes a disclaimer for users who didn't request the reset

### Requirement: Frontend Password Reset Flow
The system SHALL provide a complete frontend workflow for password reset.

#### Scenario: Request password reset
- **WHEN** a user clicks "Forgot Password" on the login page
- **THEN** a password reset modal is displayed
- **AND** user can enter their email address
- **AND** receives a success message after submission (regardless of email existence)

#### Scenario: Reset password page
- **WHEN** a user clicks the reset link in the email
- **THEN** they are directed to /reset-password?token={token} (query parameter)
- **AND** see a form to enter new password and confirm password
- **AND** see password strength requirements
- **AND** can submit to complete the reset

#### Scenario: Successful reset redirect
- **WHEN** a password is reset successfully
- **THEN** the user is redirected to the login page
- **AND** sees a success message "Password reset successfully. Please login."
- **AND** can immediately login with the new password

#### Scenario: Rate limit error display
- **WHEN** a password reset request is rate limited
- **THEN** the modal displays "Too many reset attempts. Please try again later."
- **AND** the user cannot submit another request
- **AND** a countdown or retry time is shown
