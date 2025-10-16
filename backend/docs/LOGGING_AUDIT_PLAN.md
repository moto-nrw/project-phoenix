# Logging Audit Overview

## Current State Summary
- Structured logrus logger exists in `backend/logging/logger.go` with Chi middleware injecting request-scoped entries.
- Application handlers, services, CLI tools, and migrations still import the stdlib `log` package and emit unstructured messages.
- Security logging uses a standalone stdlib-based logger, and no dedicated audit sink is configured; everything currently goes to stdout/stderr.

## Remediation Plan
1. **Unify the logging API**: expose helpers such as `logging.FromContext(ctx)`, `logging.Info/Error/Audit(ctx, msg, fields)` on top of the existing logrus instance; document them as the only supported interface.
2. **Enforce adoption**: add lint/CI rules blocking new imports of `log` (outside `backend/logging`), then codemod the existing call sites listed below to the helpers.
3. **Define audit semantics**: agree on an audit event schema (actor, action, target, result, correlation IDs, timestamp) and implement a `logging.Audit` helper that fans out to a protected sink.
4. **Provision audit storage**: configure logrus hooks/forwarders to deliver audit events to an append-only, access-controlled destination (e.g., S3 with Object Lock, SIEM, or dedicated DB table) in JSON; ensure encryption and retention policies.
5. **Retention & DSAR compliance**: document retention windows (e.g., ops logs 180 days, audit logs 3 years) and build automated purge routines plus DSAR response procedures.
6. **Operational readiness**: update runbooks/onboarding docs, add dashboards & alerts driven by structured fields, and integrate audit log review into incident-response workflows.

## Stdlib `log` Usage Inventory
- backend/services/scheduler/scheduler.go — 38
- backend/services/database/database_service.go — 8
- backend/services/auth/auth_service.go — 9
- backend/email/mockMailer.go — 2
- backend/test_dbconn.go — 1
- backend/services/activities/activity_service.go — 2
- backend/logging/logger.go — 1
- backend/services/usercontext/usercontext_service.go — 1
- backend/api/usercontext/api.go — 56
- backend/api/active/api.go — 92
- backend/api/feedback/api.go — 21
- backend/api/schedules/api.go — 73
- backend/api/rooms/api.go — 23
- backend/api/groups/api.go — 30
- backend/api/server.go — 6
- backend/auth/jwt/tokenauth.go — 4
- backend/api/auth/api.go — 112
- backend/api/config/api.go — 33
- backend/api/database/api.go — 2
- backend/api/activities/api.go — 110
- backend/api/users/handlers.go — 34
- backend/api/staff/api.go — 39
- backend/api/iot/api.go — 135
- backend/database/migrations/001003006_users_students_guardians.go — 2
- backend/database/migrations/001001004_schedule_recurrence_rules.go — 2
- backend/api/students/api.go — 56
- backend/database/migrations/001002004_users_teachers.go — 2
- backend/database/migrations/001002007_education_groups.go — 2
- backend/database/migrations/001003003_activities_schedules.go — 2
- backend/database/migrations/001004003_active_groups_supervisors.go — 2
- backend/database/migrations/001004006_audit_auth_events_new.go — 2
- backend/database/migrations/001003008_activities_student_enrollments.go — 2
- backend/database/migrations/001005002_feedback_entries.go — 2
- backend/database/migrations/main.go — 11
- backend/database/migrations/001003004_activities_supervisors_planned.go — 2
- backend/database/migrations/001000007_auth_account_roles.go — 2
- backend/database/migrations/001003007_users_privacy_consents.go — 2
- backend/database/migrations/001000002_auth_tokens.go — 2
- backend/database/migrations/001004002_active_visits.go — 2
- backend/database/migrations/001000005_auth_permissions.go — 2
- backend/database/migrations/001003009_iot_devices.go — 2
- backend/seed/seed.go — 6
- backend/cmd/migrate.go — 1
- backend/database/migrations/001006007_create_scheduled_checkouts_table.go — 4
- backend/database/migrations/001000003_auth_password_reset_tokens.go — 2
- backend/seed/runtime/seeder.go — 5
- backend/database/migrations/001002008_education_group_substitution.go — 2
- backend/database/migrations/001003005_users_students.go — 2
- backend/seed/fixed/rooms.go — 1
- backend/database/migrations/001002003_users_staff.go — 2
- backend/database/migrations/001000004_auth_roles.go — 2
- backend/database/migrations/001004004_active_combined_groups.go — 2
- backend/cmd/seed.go — 4
- backend/database/migrations/001004005_active_group_mappings.go — 2
- backend/seed/fixed/students.go — 3
- backend/database/migrations/001006001_config_settings.go — 2
- backend/seed/fixed/activities.go — 6
- backend/database/migrations/001001003_schedule_dateframes.go — 2
- backend/seed/fixed/auth.go — 1
- backend/database/migrations/001002001_users_persons.go — 2
- backend/seed/fixed/staff.go — 2
- backend/database/migrations/001002005_users_guests.go — 2
- backend/cmd/gendoc.go — 7
- backend/seed/fixed/persons.go — 3
- backend/database/migrations/001000006_auth_role_permissions.go — 2
- backend/seed/fixed/devices.go — 1
- backend/cmd/cleanup.go — 16
- backend/database/migrations/001004001_active_groups.go — 2
- backend/database/migrations/001000008_auth_account_permissions.go — 2
- backend/seed/fixed/education.go — 2
- backend/database/migrations/001004007_auth_token_family_new.go — 2
- backend/database/migrations/001000009_auth_accounts_parents.go — 2
- backend/cmd/serve.go — 1
- backend/database/migrations/001001002_schedule_timeframes.go — 2
- backend/database/migrations/001006008_add_substitution_permissions_to_teacher.go — 7
- backend/database/migrations/001001001_facilities_rooms.go — 2
- backend/database/migrations/001002002_users_profiles.go — 2
- backend/seed/fixed/seeder.go — 1
- backend/database/migrations/001002000_users_rfid_cards.go — 2
- backend/database/migrations/001002006_users_guardians.go — 2
- backend/database/migrations/001006004_seed_activity_categories.go — 3
- backend/database/migrations/001003001_activities_categories.go — 2
- backend/database/migrations/001003002_activities_groups.go — 2
- backend/database/migrations/001000001_auth_accounts.go — 2
- backend/database/migrations/001006005_create_attendance_table.go — 2
