# Email Service Delivery Tracking

## MODIFIED Requirements

### Requirement: Asynchronous Email Dispatch
The system SHALL send emails asynchronously while reporting delivery outcomes back to calling services.

#### Scenario: Email send success acknowledged
- **WHEN** a service enqueues an email
- **THEN** the dispatcher returns immediately
- **AND** persists a success callback marking the message `sent` with timestamp
- **AND** emits structured logs for auditing

#### Scenario: Email send failure reported
- **WHEN** the dispatcher receives an SMTP error
- **THEN** it records the error message (truncated to safe length)
- **AND** marks the message `failed`
- **AND** schedules a retry according to policy
- **AND** surfaces the failure through the service callback.

## ADDED Requirements

### Requirement: Email Delivery Retry Policy
The system SHALL retry failed email sends with bounded exponential backoff.

#### Scenario: Transient SMTP outage
- **WHEN** sending fails due to a transient SMTP error
- **THEN** the dispatcher retries up to the configured maximum attempts (default 3)
- **AND** waits between attempts using exponential backoff (e.g., 1m, 5m, 15m)
- **AND** stops retrying once a send succeeds.

#### Scenario: Persistent failure alerting
- **WHEN** all retry attempts are exhausted
- **THEN** the dispatcher marks the message `failed`
- **AND** triggers an admin alert hook with the final error details.
