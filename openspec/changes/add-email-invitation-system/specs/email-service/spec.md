# Email Service Specification

## ADDED Requirements

### Requirement: SMTP Configuration
The system SHALL support configurable SMTP settings for sending emails through any standard SMTP provider.

#### Scenario: SMTP settings configured
- **WHEN** EMAIL_SMTP_HOST, EMAIL_SMTP_PORT, EMAIL_SMTP_USER, EMAIL_SMTP_PASSWORD are set in environment
- **THEN** the system connects to the configured SMTP server for email delivery

#### Scenario: SMTP settings missing
- **WHEN** EMAIL_SMTP_HOST is not configured
- **THEN** the system uses a mock mailer that logs email content instead of sending

#### Scenario: Provider migration
- **WHEN** administrator changes SMTP settings from one provider to another
- **THEN** the system sends emails through the new provider without code changes

### Requirement: Email Template System
The system SHALL render HTML emails using a template system with CSS inlining for email client compatibility.

#### Scenario: HTML email rendering
- **WHEN** an email is sent using a template name and content data
- **THEN** the system renders the HTML template with the provided data
- **AND** inlines all CSS styles using premailer for maximum email client support

#### Scenario: Plain text fallback
- **WHEN** an HTML email is rendered
- **THEN** the system automatically generates a plain text version
- **AND** includes both HTML and plain text in the email for accessibility

#### Scenario: Template variables
- **WHEN** a template includes variables like {{.Name}} or {{.URL}}
- **THEN** the system replaces them with actual values from the content data
- **AND** handles missing variables gracefully without errors

### Requirement: Branded Email Design
The system SHALL use consistent moto branding in all email templates.

#### Scenario: Brand colors applied
- **WHEN** any email is sent
- **THEN** the email uses moto-blue (#5080d8) for headings and buttons
- **AND** uses moto-green (#83cd2d) for highlights
- **AND** includes the moto logo at the top

#### Scenario: Mobile-responsive layout
- **WHEN** an email is viewed on a mobile device
- **THEN** the layout adapts to the smaller screen size
- **AND** buttons and links are easily tappable

### Requirement: Fire-and-Forget Email Sending
The system SHALL send emails asynchronously without blocking API responses.

#### Scenario: Email send success
- **WHEN** an email send is initiated
- **THEN** the email is sent in a goroutine (asynchronous)
- **AND** the operation is logged before and after sending
- **AND** the API response is returned immediately without waiting for SMTP

#### Scenario: Email send failure
- **WHEN** an email fails to send due to SMTP errors
- **THEN** the error is logged with recipient and error details
- **AND** the goroutine exits gracefully
- **AND** the API response has already been returned
- **AND** the underlying operation (e.g., invitation token creation) completed successfully

### Requirement: Mailer Service Injection
The system SHALL inject the mailer service into all services that need to send emails.

#### Scenario: Mailer available in auth service
- **WHEN** the auth service is initialized
- **THEN** it receives a mailer instance via dependency injection
- **AND** can send emails using the mailer interface

#### Scenario: Mock mailer in tests
- **WHEN** tests are running
- **THEN** a mock mailer is injected instead of a real SMTP client
- **AND** tests can verify email sending without actual delivery

### Requirement: Email Audit Logging
The system SHALL log all email sending attempts for audit and debugging purposes.

#### Scenario: Successful email logged
- **WHEN** an email is sent successfully
- **THEN** the system logs recipient, subject, and template name using log.Printf
- **AND** includes timestamp (automatic in log output)

#### Scenario: Failed email logged
- **WHEN** an email fails to send
- **THEN** the system logs the error message using log.Printf
- **AND** includes recipient and template information for debugging

### Requirement: Email From Field Configuration
The system SHALL ensure all emails have a valid From address configured.

#### Scenario: From field configured globally
- **WHEN** the email service is initialized
- **THEN** it loads EMAIL_FROM_NAME and EMAIL_FROM_ADDRESS from configuration
- **AND** stores as default From email for all messages

#### Scenario: From field auto-populated
- **WHEN** an email message is created without a From field
- **THEN** the system automatically populates it with the default From email
- **AND** prevents SMTP send failures due to missing From address

#### Scenario: From field override
- **WHEN** an email message explicitly sets a From field
- **THEN** the system uses the provided From address
- **AND** does not replace it with the default
