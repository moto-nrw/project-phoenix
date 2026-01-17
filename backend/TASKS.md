## Iteration 2026-01-15_18:56:10

**Changed:** added S3/MinIO storage adapter, switched avatar reads/cleanup to storage interface

**Files:** api/base.go, api/usercontext/api.go, internal/adapter/storage/local.go, internal/adapter/storage/s3.go, internal/core/port/storage.go, services/usercontext/avatar_service.go, services/usercontext/profile_handlers.go, go.mod, go.sum

**Commit:** 85e934a8

---

## Iteration 2026-01-15_19:04:54

**Changed:** Moved HTTP API handlers under internal/adapter/handler/http and updated imports.

**Files:** backend/TASKS.md backend/cmd/gendoc.go backend/cmd/serve.go backend/internal/adapter/handler/http/active/analytics.go backend/internal/adapter/handler/http/active/api.go backend/internal/adapter/handler/http/active/checkout.go backend/internal/adapter/handler/http/active/checkout_helpers.go backend/internal/adapter/handler/http/active/combined_handlers.go backend/internal/adapter/handler/http/active/errors.go backend/internal/adapter/handler/http/active/group_mappings.go backend/internal/adapter/handler/http/active/groups.go backend/internal/adapter/handler/http/active/supervisors.go backend/internal/adapter/handler/http/active/types.go backend/internal/adapter/handler/http/active/visits.go backend/internal/adapter/handler/http/activities/api.go backend/internal/adapter/handler/http/activities/errors.go backend/internal/adapter/handler/http/activities/handlers_activities.go backend/internal/adapter/handler/http/activities/handlers_enrollments.go backend/internal/adapter/handler/http/activities/handlers_schedules.go backend/internal/adapter/handler/http/activities/handlers_supervisors.go backend/internal/adapter/handler/http/activities/types.go backend/internal/adapter/handler/http/auth/account_handlers.go backend/internal/adapter/handler/http/auth/api.go backend/internal/adapter/handler/http/auth/errors.go backend/internal/adapter/handler/http/auth/invitation_handlers.go backend/internal/adapter/handler/http/auth/parent_account_handlers.go backend/internal/adapter/handler/http/auth/password_handlers.go backend/internal/adapter/handler/http/auth/permission_handlers.go backend/internal/adapter/handler/http/auth/role_handlers.go backend/internal/adapter/handler/http/auth/types.go backend/internal/adapter/handler/http/base.go backend/internal/adapter/handler/http/common/errors.go backend/internal/adapter/handler/http/common/request.go backend/internal/adapter/handler/http/common/response.go backend/internal/adapter/handler/http/common/student_data_snapshot.go backend/internal/adapter/handler/http/common/student_locations.go backend/internal/adapter/handler/http/config/api.go backend/internal/adapter/handler/http/config/errors.go backend/internal/adapter/handler/http/config/retention_handlers.go backend/internal/adapter/handler/http/database/api.go backend/internal/adapter/handler/http/database/errors.go backend/internal/adapter/handler/http/feedback/api.go backend/internal/adapter/handler/http/feedback/errors.go backend/internal/adapter/handler/http/groups/api.go backend/internal/adapter/handler/http/groups/errors.go backend/internal/adapter/handler/http/groups/group_crud.go backend/internal/adapter/handler/http/groups/group_transfer.go backend/internal/adapter/handler/http/groups/student_helpers.go backend/internal/adapter/handler/http/guardians/api.go backend/internal/adapter/handler/http/guardians/authorization.go backend/internal/adapter/handler/http/guardians/guardian_crud_handlers.go backend/internal/adapter/handler/http/guardians/handlers_invitation.go backend/internal/adapter/handler/http/guardians/relationship_handlers.go backend/internal/adapter/handler/http/guardians/types.go backend/internal/adapter/handler/http/import/api.go backend/internal/adapter/handler/http/import/file_upload.go backend/internal/adapter/handler/http/iot/api.go backend/internal/adapter/handler/http/iot/attendance/handlers.go backend/internal/adapter/handler/http/iot/attendance/resource.go backend/internal/adapter/handler/http/iot/attendance/types.go backend/internal/adapter/handler/http/iot/checkin/handlers.go backend/internal/adapter/handler/http/iot/checkin/helpers.go backend/internal/adapter/handler/http/iot/checkin/resource.go backend/internal/adapter/handler/http/iot/checkin/schulhof.go backend/internal/adapter/handler/http/iot/checkin/types.go backend/internal/adapter/handler/http/iot/checkin/workflow.go backend/internal/adapter/handler/http/iot/common/errors.go backend/internal/adapter/handler/http/iot/common/helpers.go backend/internal/adapter/handler/http/iot/common/types.go backend/internal/adapter/handler/http/iot/data/handlers.go backend/internal/adapter/handler/http/iot/data/resource.go backend/internal/adapter/handler/http/iot/data/types.go backend/internal/adapter/handler/http/iot/devices/handlers.go backend/internal/adapter/handler/http/iot/devices/resource.go backend/internal/adapter/handler/http/iot/devices/types.go backend/internal/adapter/handler/http/iot/feedback/handlers.go backend/internal/adapter/handler/http/iot/feedback/resource.go backend/internal/adapter/handler/http/iot/feedback/types.go backend/internal/adapter/handler/http/iot/rfid/handlers.go backend/internal/adapter/handler/http/iot/rfid/resource.go backend/internal/adapter/handler/http/iot/rfid/types.go backend/internal/adapter/handler/http/iot/sessions/handlers.go backend/internal/adapter/handler/http/iot/sessions/helpers.go backend/internal/adapter/handler/http/iot/sessions/resource.go backend/internal/adapter/handler/http/iot/sessions/timeout.go backend/internal/adapter/handler/http/iot/sessions/types.go backend/internal/adapter/handler/http/rooms/api.go backend/internal/adapter/handler/http/rooms/errors.go backend/internal/adapter/handler/http/schedules/api.go backend/internal/adapter/handler/http/schedules/errors.go backend/internal/adapter/handler/http/schedules/handlers_dateframe.go backend/internal/adapter/handler/http/schedules/handlers_recurrence.go backend/internal/adapter/handler/http/schedules/handlers_timeframe.go backend/internal/adapter/handler/http/server.go backend/internal/adapter/handler/http/sse/api.go backend/internal/adapter/handler/http/sse/resource.go backend/internal/adapter/handler/http/sse/sse_helpers.go backend/internal/adapter/handler/http/staff/api.go backend/internal/adapter/handler/http/staff/errors.go backend/internal/adapter/handler/http/staff/pin_handlers.go backend/internal/adapter/handler/http/staff/staff_helpers.go backend/internal/adapter/handler/http/staff/substitution_handlers.go backend/internal/adapter/handler/http/staff/types.go backend/internal/adapter/handler/http/students/api.go backend/internal/adapter/handler/http/students/authorization.go backend/internal/adapter/handler/http/students/crud_handlers.go backend/internal/adapter/handler/http/students/errors.go backend/internal/adapter/handler/http/students/list_helpers.go backend/internal/adapter/handler/http/students/location_handlers.go backend/internal/adapter/handler/http/students/privacy_handlers.go backend/internal/adapter/handler/http/students/requests.go backend/internal/adapter/handler/http/students/responses.go backend/internal/adapter/handler/http/students/rfid_handlers.go backend/internal/adapter/handler/http/students/visit_handlers.go backend/internal/adapter/handler/http/substitutions/api.go backend/internal/adapter/handler/http/usercontext/api.go backend/internal/adapter/handler/http/usercontext/errors.go backend/internal/adapter/handler/http/users/api.go backend/internal/adapter/handler/http/users/errors.go backend/internal/adapter/handler/http/users/handlers.go backend/simulator/iot/client.go backend/simulator/iot/engine.go backend/simulator/iot/engine_checkin.go backend/simulator/iot/simulator.go backend/simulator/iot/state.go

**Commit:** 329fd422e30dabb40c9f592e7a4e9eb5c6a1f7b4

---

## Iteration 2026-01-15_19:10:52

**Changed:** Moved database repositories into internal adapter path and updated imports.

**Files:** internal/adapter/repository/postgres/**, services/**, internal/adapter/handler/http/base.go, cmd/cleanup_helpers.go, auth/authorize/policies/student_visits_test.go, internal/README.md

**Commit:** d06f957f

---

## Iteration 2026-01-15_19:20:30

**Changed:** Removed JWT config defaults and random secret handling, added MustTokenAuth usage across HTTP handlers

**Files:** auth/jwt/tokenauth.go, cmd/serve.go, internal/adapter/handler/http/active/api.go, internal/adapter/handler/http/activities/api.go, internal/adapter/handler/http/auth/api.go, internal/adapter/handler/http/config/api.go, internal/adapter/handler/http/database/api.go, internal/adapter/handler/http/feedback/api.go, internal/adapter/handler/http/groups/api.go, internal/adapter/handler/http/guardians/api.go, internal/adapter/handler/http/import/api.go, internal/adapter/handler/http/iot/api.go, internal/adapter/handler/http/rooms/api.go, internal/adapter/handler/http/schedules/api.go, internal/adapter/handler/http/sse/api.go, internal/adapter/handler/http/staff/api.go, internal/adapter/handler/http/students/api.go, internal/adapter/handler/http/substitutions/api.go, internal/adapter/handler/http/usercontext/api.go, internal/adapter/handler/http/users/api.go

**Commit:** 7c893730

---

## Iteration 2026-01-15_19:26:50

**Changed:** require explicit PORT env for server startup

**Files:** cmd/serve.go, internal/adapter/handler/http/server.go

**Commit:** 60804ea5

---

## Iteration 2026-01-15_19:34:49

**Changed:** moved config service and shared service errors into internal/core/service; updated imports

**Files:** internal/core/service/config/config_service.go, internal/core/service/config/config_service_test.go, internal/core/service/config/errors.go, internal/core/service/config/interface.go, internal/core/service/config/timeout_settings.go, internal/core/service/errors/errors.go, services/factory.go, services/feedback/errors.go, internal/adapter/handler/http/config/api.go, internal/adapter/handler/http/config/errors.go, internal/adapter/handler/http/iot/api.go, internal/adapter/handler/http/iot/sessions/resource.go

**Commit:** 2168b9ef

---

## Iteration 2026-01-15_19:40:39

**Changed:** enforce explicit CORS_ALLOWED_ORIGINS in production when CORS is enabled

**Files:** internal/adapter/handler/http/base.go

**Commit:** b9ad53d6

---

## Iteration 2026-01-15_19:49:04

**Changed:** required ADMIN_EMAIL and ADMIN_PASSWORD for admin account migration; removed defaults

**Files:** database/migrations/001006002_create_admin_account.go

**Commit:** c2b929ab

---

## Iteration 2026-01-15_19:58:03

**Changed:** Moved domain models into internal/core/domain and updated imports

**Files:** internal/core/domain/* (moved), updated imports across services/adapters/tests

**Commit:** 6ba225fe632e31d07b86170161d497dab1ca2044

---

## Iteration 2026-01-15_20:07:18

**Changed:** require explicit invitation/password reset expiry env values and set test defaults

**Files:** services/factory.go, test/helpers.go, CLAUDE.md

**Commit:** e762c8bc

---

## Iteration 2026-01-15_20:15:10

**Changed:** required test config via env; removed hardcoded localhost references in Go code

**Files:** test/helpers.go, internal/adapter/handler/http/server.go

**Commit:** b3e66365

---

## Iteration 2026-01-15_20:25:33

**Changed:** enforced explicit opt-in and non-production guard for local storage; documented storage env vars

**Files:** internal/adapter/handler/http/base.go, dev.env.example

**Commit:** 7780a081e7d137526d583eb06b59f68fd4389650

---

## Iteration 2026-01-15_20:34:34

**Changed:** moved usercontext service to internal/core/service and updated imports

**Files:** internal/core/service/usercontext/*, services/factory.go, internal/adapter/handler/http/*, architecture.svg, deps.dot

**Commit:** c197d0c502c883737d064dcc41f3bf734e912b9c

---

## Iteration 2026-01-15_20:49:45

**Changed:** moved password hashing into core domain auth package; updated callers

**Files:** README.md, database/migrations/001006002_create_admin_account.go, internal/core/domain/auth/account.go, internal/core/domain/auth/password.go, seed/fixed/persons.go, services/auth/auth_service.go, services/auth/password_helpers.go

**Commit:** 335d4c0f

---

## Iteration 2026-01-15_20:58:52

**Changed:** tightened local storage config validation and normalized public URL generation (plus staged service refactor moves)

**Files:** internal/adapter/storage/local.go; internal/core/service/... (renames); TASKS.md

**Commit:** a7fd1b99e2d3557b2cab3d7473de63ac934b24d7

---

## Iteration 2026-01-15_21:06:13

**Changed:** moved auth adapters under internal/adapter/middleware and updated imports

**Files:** internal/adapter/middleware/{authorize,device,jwt}/*, internal/adapter/handler/http/*, internal/core/service/*, seed/fixed/auth.go

**Commit:** 6604c5aadec69910c3f3d68f2bd0c3bc710aa034

---

## Iteration 2026-01-15_21:12:24

**Changed:** moved core logging to core logger package to remove adapter dependency

**Files:** internal/core/logger/logger.go, internal/core/service/active/broadcast_helpers.go, internal/core/service/active/visit_helpers.go, internal/core/service/activities/activity_service.go, internal/core/service/auth/account_management.go, internal/core/service/auth/account_metadata.go, internal/core/service/auth/auth_service.go, internal/core/service/auth/invitation_email.go, internal/core/service/auth/invitation_service.go, internal/core/service/auth/password_reset.go, internal/core/service/auth/registration.go, internal/core/service/auth/token_refresh.go, internal/core/service/database/database_service.go, internal/core/service/factory.go, internal/core/service/scheduler/scheduler.go

**Commit:** d2414cb7

---

## Iteration 2026-01-15_21:30:00

**Changed:** Enforced APP_ENV as required config and aligned tests

**Files:** backend/RALPH_LAYER_REFACTOR.md backend/RALPH_LOOP_TASK_codex.md backend/TARGET_ARCHITECTURE_codex.mmd backend/TASKS.md backend/cmd/cleanup_helpers.go backend/database/database_config.go backend/internal/adapter/handler/http/base.go backend/internal/adapter/services/factory.go backend/internal/core/service/active/active_group_service_test.go backend/internal/core/service/active/cleanup_service_test.go backend/internal/core/service/active/combined_group_service_test.go backend/internal/core/service/active/session_conflict_test.go backend/internal/core/service/activities/activity_service_test.go backend/internal/core/service/auth/auth_core_test.go backend/internal/core/service/iot/iot_service_test.go backend/internal/core/service/users/guardian_service_test.go backend/internal/core/service/users/person_service_test.go backend/test/helpers.go

**Commit:** 49c2c362

---

## Iteration 2026-01-15_22:03:36

**Changed:** Removed core -> adapter imports by moving auth/device context, permissions, mail dispatch types, and token provider into core ports; updated services/factory and tests.

**Files:** internal/core/port/*.go internal/core/port/permissions/constants.go internal/core/service/auth/*.go internal/core/service/database/*.go internal/core/service/users/*.go internal/core/service/active/*.go internal/adapter/middleware/* internal/adapter/mailer/dispatcher.go internal/adapter/services/factory.go

**Commit:** 7d46940b

---

## Iteration 2026-01-15_22:40:14

**Changed:** Removed hardcoded log level default in serve command

**Files:** cmd/serve.go

**Commit:** cfadb5cf9b105b81235ba85f7127585554bc615c

**Note:** working tree was dirty at start

---

## Iteration 2026-01-15_22:47:37

**Changed:** Restrict config file loading to dev/test unless explicit --config

**Files:** cmd/root.go

**Commit:** c8c2892a

**Note:** working tree was dirty at start

---

## Iteration 2026-01-15_22:59:22

**Changed:** replaced local avatar storage with memory backend; updated storage env defaults; included pre-existing scheduler/middleware updates

**Files:** backend/LEARNINGS.md backend/RALPH_LOOP_TASK.md backend/dev.env.example backend/internal/README.md backend/internal/adapter/handler/http/base.go backend/internal/adapter/handler/http/scheduler_config.go backend/internal/adapter/handler/http/server.go backend/internal/adapter/middleware/device/device_auth.go backend/internal/adapter/middleware/request_logger.go backend/internal/adapter/storage/local.go backend/internal/adapter/storage/memory.go backend/internal/core/port/storage.go backend/internal/core/service/scheduler/config.go backend/internal/core/service/scheduler/scheduler.go backend/internal/core/service/scheduler/scheduler_test.go

**Commit:** f499586eb856773725aedbbc124b56450c799e41

**Note:** working tree was dirty at start

---

## Iteration 2026-01-15_23:05:44

**Changed:** moved database package + migrations under internal/adapter/repository/postgres and updated imports

**Files:** cmd/cleanup_helpers.go, cmd/migrate.go, cmd/seed.go, internal/adapter/handler/http/base.go, internal/adapter/repository/postgres/database/*, internal/adapter/repository/postgres/migrations/*, internal/adapter/repository/postgres/sample_group.sql, test/helpers.go, test_dbconn.go

**Commit:** b4b5ff403848c59f8bdf339c06c1f5b430d3bb81

---

## Iteration 2026-01-15_23:15:32

**Changed:** enforced explicit rate-limit configuration and removed defaults

**Files:** internal/adapter/handler/http/base.go, internal/adapter/services/factory.go, internal/core/service/auth/auth_service.go, internal/core/service/auth/password_reset_integration_test.go, internal/core/service/auth/password_reset_rate_limit_test.go, internal/core/service/auth/refactor_verification_test.go

**Commit:** f699c887ed12f9313cc1be5aefffa8e60c3d4d80

---

## Iteration 2026-01-15_23:21:50

**Changed:** replaced direct logrus usage in usercontext core services with core logger

**Files:** internal/core/service/usercontext/group_operations.go internal/core/service/usercontext/profile_handlers.go

**Commit:** 6d09eeb74eaffd015c9f2c182c03e83e06b6406a

---

## Iteration 2026-01-15_23:32:33

**Changed:** require OGS_DEVICE_PIN at startup and inject into device middleware

**Files:** internal/adapter/handler/http/base.go, internal/adapter/handler/http/iot/api.go, internal/adapter/handler/http/students/api.go, internal/adapter/middleware/device/device_auth.go

**Commit:** 28373868763aa5f888ab7b4b54476815f0094685

---

## Iteration 2026-01-15_23:44:09

**Changed:** Reduced auth token repository interface surface and updated token repo tests/constructor

**Files:** internal/core/domain/auth/repository.go, internal/adapter/repository/postgres/auth/token.go, internal/adapter/repository/postgres/auth/token_repository_test.go

**Commit:** ca8472ba052e7ca93504a5e2da88114a1ebb1c6c

---

## Iteration 2026-01-15_23:53:36

**Changed:** Introduced core logger interface with logrus adapter and removed direct logrus dependency in core

**Files:** internal/core/logger/logger.go, internal/adapter/logger/logger.go, internal/core/service/usercontext/avatar_service.go, internal/adapter/services/factory.go

**Commit:** 006003c39a6f991faa17ea0cd6b2c6b3c6638f69

---

## Iteration 2026-01-16_00:07:58

**Changed:** Moved auth repository interfaces to core/port/auth; updated adapters/services; fixed substitution test to use UTC

**Files:** internal/core/port/auth/repository.go, internal/core/domain/auth/repository.go, internal/adapter/repository/postgres/auth/*.go, internal/adapter/repository/postgres/factory.go, internal/core/service/auth/invitation_service.go, internal/core/service/auth/repositories.go, internal/core/service/database/repositories.go, internal/core/service/usercontext/usercontext_service.go, internal/core/service/usercontext/usercontext_service_test.go, internal/core/service/users/guardian_service.go, internal/core/service/users/person_service.go

**Commit:** 2a705469ca959ca7de68322cbc12c5e2b8840d3a

---

## Iteration 2026-01-16_00:17:32

**Changed:** moved Schulhof activity constants into internal core domain and updated references

**Files:** internal/core/domain/activities/constants.go, internal/adapter/handler/http/iot/checkin/workflow.go, internal/adapter/handler/http/iot/checkin/schulhof.go, seed/fixed/activities.go

**Commit:** 6c7c6ce8fb345dcae377b5d1c83a94503d436d66

---

## Iteration 2026-01-16_00:51:25

**Changed:** moved repository interfaces from domain to core/port and updated imports across services/adapters

**Files:** backend/internal/adapter/repository/postgres/active/attendance_repository.go backend/internal/adapter/repository/postgres/active/combined_group.go backend/internal/adapter/repository/postgres/active/group.go backend/internal/adapter/repository/postgres/active/group_mapping.go backend/internal/adapter/repository/postgres/active/group_supervisor.go backend/internal/adapter/repository/postgres/active/visits.go backend/internal/adapter/repository/postgres/activities/category.go backend/internal/adapter/repository/postgres/activities/group.go backend/internal/adapter/repository/postgres/activities/schedule.go backend/internal/adapter/repository/postgres/activities/student_enrollment.go backend/internal/adapter/repository/postgres/activities/supervisor_planned.go backend/internal/adapter/repository/postgres/audit/data_deletion.go backend/internal/adapter/repository/postgres/config/setting.go backend/internal/adapter/repository/postgres/config/setting_repository_test.go backend/internal/adapter/repository/postgres/education/group.go backend/internal/adapter/repository/postgres/education/group_substitution.go backend/internal/adapter/repository/postgres/education/group_teacher.go backend/internal/adapter/repository/postgres/facilities/room.go backend/internal/adapter/repository/postgres/factory.go backend/internal/adapter/repository/postgres/feedback/entry.go backend/internal/adapter/repository/postgres/iot/device.go backend/internal/adapter/repository/postgres/migrations/main.go backend/internal/adapter/repository/postgres/schedule/dateframe.go backend/internal/adapter/repository/postgres/schedule/recurrence_rule.go backend/internal/adapter/repository/postgres/schedule/timeframe.go backend/internal/adapter/repository/postgres/users/guardian_profile.go backend/internal/adapter/repository/postgres/users/guest.go backend/internal/adapter/repository/postgres/users/person.go backend/internal/adapter/repository/postgres/users/person_guardian.go backend/internal/adapter/repository/postgres/users/privacy_consent.go backend/internal/adapter/repository/postgres/users/profile.go backend/internal/adapter/repository/postgres/users/rfid_card.go backend/internal/adapter/repository/postgres/users/staff.go backend/internal/adapter/repository/postgres/users/student.go backend/internal/adapter/repository/postgres/users/student_guardian.go backend/internal/adapter/repository/postgres/users/teacher.go backend/internal/core/domain/active/attendance.go backend/internal/core/port/active/attendance_repository.go backend/internal/core/port/active/repository.go backend/internal/core/port/activities/repository.go backend/internal/core/port/audit/repository.go backend/internal/core/port/config/repository.go backend/internal/core/port/education/repository.go backend/internal/core/port/facilities/repository.go backend/internal/core/port/feedback/repository.go backend/internal/core/port/iot/repository.go backend/internal/core/port/schedule/repository.go backend/internal/core/port/users/repository.go backend/internal/core/service/active/active_service.go backend/internal/core/service/active/cleanup_service.go backend/internal/core/service/active/transaction_helpers.go backend/internal/core/service/activities/activity_service.go backend/internal/core/service/auth/invitation_service.go backend/internal/core/service/auth/repositories.go backend/internal/core/service/auth/test_helpers_test.go backend/internal/core/service/config/config_service.go backend/internal/core/service/config/config_service_test.go backend/internal/core/service/database/repositories.go backend/internal/core/service/education/education_service.go backend/internal/core/service/facilities/facility_service.go backend/internal/core/service/feedback/feedback_service.go backend/internal/core/service/import/import_service.go backend/internal/core/service/import/relationship_resolver.go backend/internal/core/service/import/student_import_config.go backend/internal/core/service/iot/iot_service.go backend/internal/core/service/schedule/schedule_service.go backend/internal/core/service/usercontext/usercontext_service.go backend/internal/core/service/users/guardian_service.go backend/internal/core/service/users/person_service.go backend/internal/core/service/users/student_service.go 

**Commit:** 27d524ee8e7e584cb0109f72a5cdf8d223601bbc

---

## Sync 2026-01-16_01:51:57

**Merged:** ac0c2598 from development

**Conflicts resolved:** ~40 files (modify/delete in api/services/test fixtures, file moves for handler tests, content merges in config/substitutions/active/facilities)

---

## Iteration 2026-01-16_02:21:00

**Changed:** Split test fixtures into domain-focused files

**Files:** test/fixtures.go, test/fixtures_auth.go, test/fixtures_education.go

**Commit:** 9d4f3024

---

## Iteration 2026-01-16_02:30:40

**Changed:** require explicit config flag for loading .env

**Files:** cmd/root.go

**Commit:** 5f6ccb5a3ed0efebb7c6cb512c6ebee10a8ddf95

---

## Iteration 2026-01-16_03:00:34

**Changed:** Use env vars for JWT config in API test setup

**Files:** internal/adapter/handler/http/testutil/helpers.go

**Commit:** e95383b1

---

## Iteration 2026-01-16_03:08:40

**Changed:** Require CORS_ALLOWED_ORIGINS when CORS is enabled

**Files:** internal/adapter/handler/http/base.go

**Commit:** 23fcc4aae8283ede13b7f546876de64daaaef5e7

**Note:** working tree was dirty at start

---

## Iteration 2026-01-16_03:15:38

**Changed:** Require explicit scheduler env config when schedulers are enabled

**Files:** internal/adapter/handler/http/scheduler_config.go

**Commit:** bde8d6fac07fdf40dbcc635d53222560e028515b

---

## Iteration 2026-01-16_03:23:53

**Changed:** Move HTTP test helpers to test/testutil and update imports

**Files:** internal/adapter/handler/http/*_test.go, test/testutil/helpers.go, test/testutil/helpers_test.go

**Commit:** 2d61f54233583be1266c8d075d579d95277192b7

---

## Iteration 2026-01-16_03:32:23

**Changed:** Preserve active visit repo errors with wrapped DB failure context

**Files:** internal/core/service/active/visit_operations.go

**Commit:** 901a4c48

---

## Iteration 2026-01-16_03:56:41

**Changed:** Block config file usage when APP_ENV=production

**Files:** cmd/root.go

**Commit:** 2d237d8cf757a7f185b11c6aefe63452e88cc96c

---

## Iteration 2026-01-16_04:12:13

**Changed:** split invitation service helpers and account logic into separate files

**Files:** internal/core/service/auth/invitation_service.go, internal/core/service/auth/invitation_service_helpers.go, internal/core/service/auth/invitation_account.go

**Commit:** b67732392d6abd061e427bb296f757e2c3131f57

---

## Iteration 2026-01-16_04:57:06

**Changed:** split active group repository interfaces into read/write/relations and updated services/factories

**Files:** backend/cmd/simulate.go backend/deps.svg backend/internal/adapter/handler/http/active/checkout_helpers.go backend/internal/adapter/handler/http/base.go backend/internal/adapter/handler/http/iot/attendance/handlers.go backend/internal/adapter/handler/http/iot/attendance/handlers_test.go backend/internal/adapter/handler/http/iot/checkin/helpers.go backend/internal/adapter/handler/http/iot/checkin/resource.go backend/internal/adapter/handler/http/iot/checkin/resource_test.go backend/internal/adapter/handler/http/iot/checkin/workflow_helpers.go backend/internal/adapter/handler/http/iot/checkin/workflow_processing.go backend/internal/adapter/handler/http/iot/checkin/workflow_types.go backend/internal/adapter/handler/http/iot/data/resource.go backend/internal/adapter/handler/http/iot/data/resource_test.go backend/internal/adapter/handler/http/iot/devices/resource.go backend/internal/adapter/handler/http/iot/devices/resource_test.go backend/internal/adapter/handler/http/iot/feedback/resource.go backend/internal/adapter/handler/http/iot/feedback/resource_test.go backend/internal/adapter/handler/http/iot/rfid/resource.go backend/internal/adapter/handler/http/iot/rfid/resource_test.go backend/internal/adapter/handler/http/iot/sessions/resource.go backend/internal/adapter/handler/http/iot/sessions/resource_test.go backend/internal/adapter/mailer/types_test.go backend/internal/adapter/middleware/authorize/policies/student_visits_test.go backend/internal/adapter/middleware/device/device_auth.go backend/internal/adapter/middleware/device/device_auth_helpers_test.go backend/internal/adapter/middleware/jwt/claims_helpers_test.go backend/internal/adapter/repository/postgres/active/group.go backend/internal/adapter/repository/postgres/factory.go backend/internal/adapter/repository/postgres/users/privacy_consent.go backend/internal/adapter/repository/postgres/users/privacy_consent_repository_test.go backend/internal/adapter/services/factory.go backend/internal/core/port/active/repository.go backend/internal/core/port/users/repository.go backend/internal/core/service/active/active_service.go backend/internal/core/service/active/analytics_methods.go backend/internal/core/service/active/attendance_service.go backend/internal/core/service/active/broadcast_helpers.go backend/internal/core/service/active/dashboard_helpers.go backend/internal/core/service/active/session_lifecycle.go backend/internal/core/service/active/session_start.go backend/internal/core/service/active/supervisor_handlers.go backend/internal/core/service/active/transaction_helpers.go backend/internal/core/service/facilities/facility_service.go backend/internal/core/service/usercontext/usercontext_service.go backend/internal/core/service/users/guardian_invitation.go

**Commit:** 111e4d45dde944c2b256d40671fa1bc799b14870

---

## Iteration 2026-01-16_05:06:35

**Changed:** tightened simulator base_url validation (scheme + host) and added tests

**Files:** simulator/iot/config.go, simulator/iot/config_test.go

**Commit:** 01f3c84e7face8abea6d37ea854572de7be39cb7

---

## Iteration 2026-01-16_05:22:04

**Changed:** split group substitution repository into base + relations interfaces; updated factories/services/tests

**Files:** internal/core/port/education/repository.go, internal/adapter/repository/postgres/education/group_substitution.go, internal/adapter/repository/postgres/factory.go, internal/core/service/education/education_service.go, internal/core/service/education/substitution_helpers.go, internal/core/service/usercontext/usercontext_service.go, internal/core/service/usercontext/group_operations.go, internal/adapter/services/factory.go, internal/core/service/education/education_service_test.go, internal/adapter/middleware/authorize/policies/student_visits_test.go, internal/core/service/usercontext/usercontext_service_test.go, internal/adapter/repository/postgres/education/group_substitution_repository_test.go

**Commit:** cc3f4389f27e6d036b328f10aa6f7a7d578a3d09

---

## Sync 2026-01-17_00:22:43

**Merged:** 03d89e00 from development

**Conflicts resolved:** content (multiple files); file-location (tests); modify/delete (legacy paths)

---

## Iteration 2026-01-17_00:33:25

**Changed:** split FixedSeeder seed helpers into focused files

**Files:** seed/api/fixed.go, seed/api/fixed_rooms.go, seed/api/fixed_people.go, seed/api/fixed_groups.go, seed/api/fixed_students.go, seed/api/fixed_guardians.go, seed/api/fixed_devices.go, seed/api/fixed_roles.go

**Commit:** 499057b8

---

## Sync 2026-01-17_00:35:27

**Merged:** 6ae13579 from development

**Conflicts resolved:** 0

---

## Iteration 2026-01-17_00:46:40

**Changed:** Added wide-event request logging middleware with user/context enrichment

**Files:** internal/adapter/middleware/wide_event.go, internal/adapter/middleware/wide_event_middleware.go, internal/adapter/middleware/jwt/authenticator.go, internal/adapter/handler/http/base.go, internal/adapter/handler/http/active/visits.go

**Commit:** 3598693efa7b963269536b6eacd6184bb92d431b

---

## Iteration 2026-01-17_00:50:56

**Changed:** removed dead request logger middleware

**Files:** internal/adapter/middleware/request_logger.go

**Commit:** 3486bdc7

---

## Iteration 2026-01-17_00:58:15

**Changed:** surface repository errors when fetching group teachers

**Files:** internal/core/service/education/education_service.go

**Commit:** 350f155f8637969516f935ff52feb4573df3269b

---

## Iteration 2026-01-17_01:18:57

**Changed:** Align IoT feedback handler logging with wide-event fields

**Files:** internal/adapter/handler/http/iot/feedback/handlers.go

**Commit:** 4ece8be026962c33530a6f6fa8ae6e5845a21ba4

---

## Sync 2026-01-17_01:50:17

**Merged:** 4472418c from development

**Conflicts resolved:** 13 file-location, 1 content

---

## Iteration 2026-01-17_01:54:54

**Changed:** regenerated dependency graphs

**Files:** architecture.svg, deps.dot, deps.svg, TASKS.md

**Commit:** 97cf4acb1baa43b6a5732ef8857b88871d625995

---

## Iteration 2026-01-17_02:09:31

**Changed:** Replaced IoT check-in per-request logs with wide-event context and error recording

**Files:** internal/adapter/handler/http/iot/checkin/handlers.go, internal/adapter/handler/http/iot/checkin/helpers.go, internal/adapter/handler/http/iot/checkin/schulhof.go, internal/adapter/handler/http/iot/checkin/workflow_helpers.go, internal/adapter/handler/http/iot/checkin/workflow_processing.go, internal/adapter/handler/http/iot/checkin/wide_event_helpers.go

**Commit:** 74d054cafb1139ed841aaf62f49a1124704c7c55

---

## Iteration 2026-01-17_02:20:26

**Changed:** Split simulator engine attendance and supervisor swap logic into dedicated files

**Files:** simulator/iot/engine.go, simulator/iot/engine_attendance.go, simulator/iot/engine_helpers.go, simulator/iot/engine_supervisor_swap.go

**Commit:** ab7cc59638e21f51c56d594b40e8b62beee1662d

---

## Iteration 2026-01-17_02:33:04

**Changed:** enforce required EMAIL_FROM values in SMTP mailer

**Files:** internal/adapter/mailer/smtp.go

**Commit:** 88647ed5d8c61461f77ebd35efa21dc6d21048b0

---

## Iteration 2026-01-17_02:54:04

**Changed:** Fail fast when config file cannot be read

**Files:** backend/cmd/root.go

**Commit:** 47ee1b25

---

## Iteration 2026-01-17_03:13:43

**Changed:** capture wide-event error context for common and IoT error renderers

**Files:** internal/adapter/handler/http/common/errors.go, internal/adapter/handler/http/iot/common/errors.go

**Commit:** 0e582d137652fef451e27e7ee25056452d8b61f1

---

