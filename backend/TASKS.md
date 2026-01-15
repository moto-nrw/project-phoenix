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

