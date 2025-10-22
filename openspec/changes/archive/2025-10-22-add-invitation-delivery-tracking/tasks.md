## 1. Specification
- [x] 1.1 Finalize delivery tracking requirements across invitations and password resets.
- [x] 1.2 Validate change set with `openspec validate add-invitation-delivery-tracking --strict`.

## 2. Implementation
- [x] 2.1 Add delivery metadata columns to `auth.invitation_tokens` (and related reset token storage if needed) with migration + model updates.
- [x] 2.2 Introduce tracked email dispatcher that records `email_sent_at`/`email_error` and exposes retry metadata.
- [x] 2.3 Update invitation and password reset services to use the dispatcher and persist delivery results.
- [x] 2.4 Extend repositories and API responses to expose delivery status.
- [x] 2.5 Wire admin notifications for repeated failures (service + logging hooks).

## 3. Validation
- [x] 3.1 Add/extend unit tests for invitation + reset services covering success/failure paths.
- [x] 3.2 Document manual QA: simulate SMTP outage, verify admin sees failure, confirm retry + notification.
