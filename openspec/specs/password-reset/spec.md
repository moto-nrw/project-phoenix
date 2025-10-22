# password-reset Specification

## Purpose
TBD - created by archiving change add-email-invitation-system. Update Purpose after archive.
## Requirements
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

### Requirement: Password Reset Failure Notifications
The system SHALL notify administrators when password reset emails cannot be delivered after all retries.

#### Scenario: Exhausted reset email retries
- **WHEN** all retry attempts for a password reset email fail
- **THEN** the system emits an admin alert detailing the account email and final error
- **AND** logs an audit event referencing the reset token.

