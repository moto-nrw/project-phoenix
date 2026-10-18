# #4082 root inventory proof

Every production file in `backend/api/*.go` (381 files, excluding the 131 `testhooks` test fixtures) classified at the closing commit of #4082. Each row is wiring only: binding, type mapping, error mapping, deployment configuration, route mounting, nil guards or calls into owner predicates and commands. No row branches on business state, defines user text, chooses permissions or writes across owners.

| File | Classification |
|---|---|
| `active_authorization.go` | wiring: binding, owner predicate calls (securityruntime), request runtime binding |
| `active_people.go` | wiring: type mapping, nil/found guards |
| `attendance_education_groups.go` | wiring: constructor, type mapping, nil guards, ID projection |
| `attendance_route_identity.go` | wiring: type mapping, nil guards |
| `attendance_staff.go` | wiring: constructor, type mapping, ID-set projection |
| `attendance_staff_names.go` | wiring: constructor, two-owner read joined by ID, owner name function |
| `authorization_observer.go` | wiring: middleware, observability hook |
| `base.go` | wiring: constructors, observability/metrics hooks, middleware chain, route mounting, deployment config (env/viper rate limits, CORS, security logging), error mapping (meal plan error rules, users/feedback failure envelopes), type mapping (student names, meal participants, PWA stats, export columns), nil guards |
| `birthday_composition.go` | wiring: constructor, type mapping, settings-key binding, not-found error mapping |
| `booking_consistency_audit.go` | wiring: constructor, nil guards, type mapping, owner predicates (`IsAlumnus`) and owner constants (`OfferingOrderID`) |
| `calendar_facts.go` | wiring: binding, type mapping; ListActive passes the retained repository its status filter argument (orchestrator decision: wiring, no PD port fits) |
| `calendar_period_usage.go` | wiring: type mapping |
| `calendar_portal.go` | wiring: config binding, nil guards, error mapping (`calendarPushSuppressed`, `MissingUser/MissingStaff`), kind-to-kind mapping onto Delivery's `ParentAppointmentCopy`, type mapping |
| `capability_dates.go` | wiring: delegation to the owner parser (`workforce.ParseTimeTrackingDate`) |
| `care_companion_students.go` | wiring: port binding, type mapping via owner predicates (`peopledirectory.HasCompanionNote`, `users.AccompaniedWeekdays`), owner audited writes (`peopleCompose.AuditedStudentUpdates`), error mapping (no rows/`ErrStudentNotFound` -> nil, `ErrStudentLockBusy` -> `careplan.ErrCompanionLockBusy`, not found -> `ErrCompanionNotFound`) |
| `care_exit_enrollment.go` | wiring: type mapping, owner predicate (`IsApproved`) |
| `care_exit_enrollment_bookings.go` | wiring: constructor, delegation to the careexit workflow |
| `care_lifecycle_owners.go` | wiring: constructor, type mapping, date conversion, audit binding |
| `care_participation.go` | wiring: binding, nil guard, owner call with owner constant (`GroupRosterIncludesPendingWithdrawals`) |
| `care_plan_arrival_baselines.go` | wiring: constructor, nil guards, type mapping, owner functions (`ClassKey`, `Label`) |
| `care_plan_arrival_schedules.go` | wiring: constructor, nil guard |
| `care_plan_arrival_students.go` | wiring: type mapping, owner predicates (`IsAlumnus`, `CareEndedOn`), owner scope constant, error mapping |
| `care_plan_booking_materialization.go` | wiring: constructor with nil guards, field-by-field type mapping between Timetable/Enrollment/People Directory/Audit/Care Plan types, owner predicates (`IsAlumnus`, `IsApproved`, `SchoolClassGradeLevel`, `ActiveOfferingSourcedTemplates`), error mapping (`BookingRowNotFound`, skip `ErrStudentNotFound` on lock), input guards, realtime broadcast binding |
| `care_plan_care_conflict_port.go` | wiring: type mapping, error mapping, nil dependency guards |
| `care_plan_care_request_service.go` | wiring: constructor, option binding, nil guards, realtime event binding, error mapping |
| `care_plan_class_exceptions.go` | wiring: constructor with nil guard, type mapping Timetable <-> Care Plan, owner predicate call (`peopledirectory.HasActiveClass`) |
| `care_plan_class_times.go` | wiring: constructor, nil guard, type mapping |
| `care_plan_companions.go` | wiring: delegation |
| `care_plan_documents.go` | wiring: owner predicates (`securityruntime.CanModifyStudent`, `HasPermission`), type mapping onto audit records, owner-named permission binding (`carePlanHTTP.StudentDocumentPermissions`), nil guards |
| `care_plan_exception_directory.go` | wiring: type mapping, error mapping |
| `care_plan_excused_request_students.go` | wiring: constructor with nil guards and owner default (`carePlanCompose.Today`), type mapping, owner rule call (`careplan.LiveFlagsForApprovedAbsence`) passed to owner audited write |
| `care_plan_excused_requests.go` | wiring: constructor, nil-effect guards, review-scope binding, realtime event binding, type mapping |
| `care_plan_graph_wiring.go` | wiring: wiring structs, completeness validation, adapter func |
| `care_plan_offering_catalog.go` | wiring: constructor, nil guards, type mapping, error/code mapping (`offeringSourceRefusalCodes`), settings-key binding, owner functions |
| `care_plan_pickup_adjustments.go` | wiring: constructor, type mapping, settings-key binding |
| `care_plan_pickup_approval_adapter.go` | wiring: constructor, delegation, owner function (`studentpresence.AttendanceCompletion`), type mapping |
| `care_plan_pickup_change_presence.go` | wiring: port interface |
| `care_plan_pickup_excusal_timetable.go` | wiring: constructor, type mapping |
| `care_plan_pickup_schedules.go` | wiring: constructor, nil guard |
| `care_plan_pickup_students.go` | wiring: type mapping, owner predicate (`PickupEligibleOn`), error mapping |
| `care_plan_request_adapters.go` | wiring: binding, type mapping, error mapping, owner status-set calls (`careplan.PendingExcusedRequestStatuses()`/`DecidedExcusedRequestStatuses()`), nil guard |
| `care_plan_request_correction_adapter.go` | wiring: error mapping, ledger-to-compose type mapping, nil guard |
| `care_plan_request_decision_adapter.go` | wiring: binding, type mapping, owner predicate (`IsAlumnus()`), error mapping |
| `care_plan_request_diff_adapter.go` | wiring: binding, type mapping, owner `Effective()` plan call, nil guards |
| `care_plan_request_edit_adapter.go` | wiring: error mapping |
| `care_plan_request_people.go` | wiring: port interface, type mapping (audit snapshot), owner `StudentPlan.Effective()`, error mapping |
| `care_plan_request_read_adapter.go` | wiring: delegation binding |
| `care_plan_request_records.go` | wiring: port interface declarations |
| `care_plan_request_submission_adapter.go` | wiring: binding, type mapping, nil guards, two owner reads joined by ID |
| `care_plan_schedule_records.go` | wiring: binding, field-by-field type mapping, error mapping, query-option translation, owner `Today()` binding |
| `care_plan_status_day_adapter.go` | wiring: binding, type mapping, error mapping, empty-input guards, owner overview options (`careplan.StatusDayOverviewOptions`) |
| `care_plan_status_directories.go` | wiring: constructor, capability assertion guard, type mapping, delegation |
| `care_plan_student_names.go` | wiring: func-type port binding |
| `care_plan_student_record_adapters.go` | wiring: binding, type mapping, error wrapping, enum translation (`studentLifecycle`, 1:1 plus default Other) |
| `care_plan_weekly_approval_adapter.go` | wiring: port binding for `compose.NewWeeklyApprovals` (careplan compose decides the order lock, save, audit), dependency nil guards, type mapping into `peopledirectory.StudentWrite`, error mapping; note: the People Directory write and the audit write go through owner commands, not root SQL |
| `care_plan_wiring.go` | wiring: constructors, dependency nil guards, logger default, observability hooks, owner binding |
| `care_students.go` | wiring: port interfaces, type mapping (`HasRFIDTag` is a nil/empty projection), delegation to membership commands |
| `care_withdrawal_completions.go` | wiring: late-bound capability guard, type mapping, error wrapping, owner function `careplan.ParticipationBoundary`, sorting for deterministic order |
| `care_withdrawal_settings.go` | wiring: side-effect registration, type assertion guard, error mapping to `settings.BookingAuthorityBlockedError` |
| `caregiver_blocker_adapters.go` | wiring: type mapping |
| `caregiver_capability_audit.go` | wiring: port binding, nil guard, owner event constructor (`auditModels.NewCaregiverCapabilityAuthEvent`) |
| `caregiver_identity.go` | wiring: type mapping |
| `child_quota_composition.go` | wiring: type mapping |
| `class_day_composition.go` | wiring: constructors, nil guards, type mapping, owner-named audit resource constants, realtime announcer binding |
| `class_day_slot_lists.go` | wiring: owner verdict binding (`securityruntime.SlotListStudentAccess`), enum/format type mapping, error mapping |
| `class_list_entries_composition.go` | wiring: resource runtime binding, route protection, error-to-envelope mapping with owner sentinel to registered code |
| `class_list_entry_composition.go` | wiring: late binding, nil guards, owner predicates (`EnrolledStudentIDsByNameAndClass`, `IsEnrolledStudent`), audit type mapping |
| `classday_timetable_rows.go` | wiring: type mapping, filter through owner predicate `timetableModule.GroupTypeHasChildren`, empty-input guard |
| `cleanup_roots.go` | wiring: constructors, nil guards, persistence composition |
| `communication_delivery.go` | wiring: binding, nil guards, type mapping |
| `communication_mail.go` | wiring: binding, nil guards, type mapping, outbox delegation |
| `communication_parent_message_people.go` | wiring: binding, nil guards, owner predicates (`IsAlumnus()`, `IsAuthorizationStudent()`), type mapping |
| `communication_runtime.go` | wiring: tenant/UoW runtime binding |
| `communication_security.go` | wiring: binding to Security Runtime rules (`ResolveStudentReadScope`, `CanReadStudent`, `Fingerprint`) |
| `communication_staff_colleagues.go` | wiring: constructor, type mapping, owner predicate binding |
| `communication_staff_sender.go` | wiring: binding, nil guard, type mapping |
| `communication_viewer_names.go` | wiring: type mapping via owner `FullName()`, nil/blank-name guard, operation stats observability |
| `core_action_analytics.go` | wiring: deployment config (viper PostHog), analytics middleware binding, session claim mapping to owner `analytics.ActorOf`, startup route-classification guard over owner tables/prefixes |
| `data_access_audit.go` | wiring: constructors, nil guards, type mapping |
| `data_import_http_runtime.go` | wiring: middleware chain, runtime binding, two owner reads joined by ID (staff for account), capability assertion guard, owner `usage.Free()`, error-to-envelope mapping (`BusinessRejection` to 409) |
| `database_stats_reader.go` | wiring: constructors, owner count bindings, permission binding via securityruntime.DatabaseStatsPermissions/DatabaseStatsCapabilities |
| `declaration_report_renderer.go` | wiring: type mapping to lists.RecordDocument, sanitization via owner lists.SanitizeUserText |
| `deletion_audit.go` | wiring: nil guards, type mapping to audit.DataDeletion |
| `delivery_adapters.go` | wiring: type mapping, tenant-from-context binding, delegation to delivery.Module |
| `delivery_staff_recipients.go` | wiring: type projection (nil-safe GroupID map), owner predicate workforce.OnDutyStaffIDs |
| `demo_access.go` | wiring: route mounting, deployment config reads (FRONTEND_URL/TENANT_DOMAIN origin derivation), nil guard |
| `device_fleet_settings.go` | wiring: settings key binding, owner rule call (`devicefleet.OnlineWindowFromMinutes`, zero sentinel defers to owner default), error mapping (not found -> `ErrDisplayNotFound`), nil guards, Security Runtime key minting |
| `device_fleet_timetable.go` | wiring: type mapping Timetable -> Device Fleet, owner predicates (`IsPlannedInstanceStatus`, `ParticipantLimit`, `NormalizeWallClock`) |
| `device_open_room.go` | wiring: type mapping, error binding (OpenRoomRefusals), owner actor openroommove.KioskActor() |
| `device_scan_catalog.go` | wiring: nil guard, type mapping timetable<->devicescan |
| `device_scan_people.go` | wiring: nil guards, type mapping, port dispatch (nil teacherIDs -> all students), owner predicates NormalizeTagID/IsAlumnus |
| `display_feature.go` | wiring: settings key binding |
| `display_http.go` | wiring: nil guards, middleware chain, permission binding (Read: permissions.DisplayRead, Manage: permissions.DisplayManage), error mapping to apiCommon envelopes |
| `display_http_router.go` | wiring: router construction |
| `education_group_repository.go` | wiring: constructors, nil guard, type mapping, cross-owner dependency binding |
| `education_service_ports.go` | wiring: port binding, type mapping, owner rule call (`schoolmembership.ActiveExternalCaregivers`), error mapping (no rows -> false), compile-time enum equality guard, audit row mapping with owner constants |
| `education_service_ports_repositories.go` | wiring: type mapping facilities.Room -> RoomProjection |
| `education_teachers.go` | wiring: type mapping, nil guards |
| `emergency_snapshot.go` | wiring: constructor with nil guards, type mapping of owner reads into projection ports, owner filter (`studentpresence.CurrentRoomVisitsFilter`), owner error masking (`SchoolStatusReadFailed`), document rendering mapping (`lists.SanitizeUserText`) |
| `enrollment_application_composition.go` | wiring: settings key binding, constructors, type mapping, mail renderer registry by owner mail kinds, owner sender/logo helpers (`deliveryModule.SchoolSender`, `SchoolLogoURL`) |
| `enrollment_bookings.go` | wiring: type mapping, owner fn EffectiveSelectedDays, constructors |
| `enrollment_care_bookings.go` | wiring: type mapping to careplan.CareOfferingBooking |
| `enrollment_decision_composition.go` | wiring: constructors, nil guards, deployment config (ParentsURL -> FrontendURL fallback), settings key binding, audit type mapping with owner constants |
| `enrollment_decision_people.go` | wiring: type mapping, enum mapping to securityruntime role names, owner validation and presets (row.Validate, StudentGuardianRolePreset, IsFullGuardianRole, ApprovalAllowsRedemption), error binding (not-found -> nil, PG 55P03) |
| `enrollment_deletion_composition.go` | wiring: type mapping, enum mapping of actor/scope onto audit constants, settings key binding, nil guards |
| `enrollment_guardian_autofill.go` | wiring: nil guard, type mapping |
| `enrollment_intake_composition.go` | wiring: constructors, nil guards, owner predicate binding (`securityruntime.HasPermission`, `NotificationEnrollmentSubmitted`), type mapping, owner display name (`PersonDisplayName`) |
| `enrollment_phase_expiry.go` | wiring: type mapping, owner predicate IsAlumnus |
| `enrollment_phase_responses.go` | wiring: type mapping, owner predicate IsActive |
| `enrollment_report_composition.go` | wiring: nil guards, type mapping, settings binding with owner-named fallback constant, audit resource-type constants |
| `enrollment_rollover_composition.go` | wiring: constructors, settings key binding, nil guards |
| `enrollment_route_ports.go` | wiring: nil guards, type mapping |
| `enrollment_rows.go` | wiring: port interface declarations |
| `enrollment_wiring.go` | wiring: constructors, owner binding, settings guard binding via type assertions |
| `facilities_activity_catalog.go` | wiring: type mapping timetable<->facilities |
| `facilities_open_groups.go` | wiring: owner filter binding (ActiveOn today, OpenOnly), owner predicates EndedOn/StartedOn, error mapping to studentpresence.OperationError |
| `facilities_room_usage.go` | wiring: nil guards, error binding (ErrCareOfferingConfigInvalid -> true), owner query binding |
| `filestore_composition.go` | wiring: settings key binding, unit conversion MB->bytes, audit type mapping |
| `grade_transition_owners.go` | wiring: constructors |
| `group_live.go` | wiring: nil guards, type mapping, permission binding (groupLiveHTTP.GroupReadPermission, securityruntime predicates), owner rules (ResolveDayPlanning, IsAtSchoolBeforeCheckIn, ResolveEffectiveStatus, CareDayStatus.Expected), settings prefetch binding |
| `group_projection.go` | wiring: owner read decoration, group-name enrichment via schoolstructure compose helper, type mapping |
| `group_projection_adapters.go` | wiring: constructors, cross-owner projection by ID, group-name enrichment with owner constant `schoolstructure.UnknownGroupName` |
| `guardian_directory_composition.go` | wiring: runtime closures, field-by-field type mapping, tenant tx/rollback binding, error classification of owner sentinels, owner constant `identityaccess.StaffInviteRequiresApproval`, export labels taken from owner `GuardianPaymentListOf` |
| `guardian_service_ports.go` | wiring: port binding, enum/field-name mapping between People Directory ports and audit records, nil guards |
| `guardians_composition.go` | wiring: HTTP adapter binding, error-to-envelope mapping (api/common codes), permission middleware passthrough, claims mapping, type mapping |
| `history_room_names.go` | wiring: owner read projection (ID to name) |
| `history_slots.go` | wiring: constructor, nil guard, type mapping timetable to studentpresence |
| `identity_account_authentication.go` | wiring: constructors, nil guards, deployment config check (`MaxActiveSchools`), error mapping (Organisation & Tenancy demo errors -> Identity & Access, `ErrSchoolNotFound` -> not found), type mapping, owner predicates (`IsDeleted`, `Operational`), observability hook |
| `identity_account_lifecycle.go` | wiring: seam binding, completeness guards, type mapping, owner commands (People Directory, School Membership), owner predicates (`securityruntime.PasswordTooWeak`, `ClassifyGuardianRole`), owner note constant |
| `identity_caller_consumers.go` | wiring: type mapping of caller rows to consumer records |
| `identity_caller_context.go` | wiring: constructor, settings key and owner sentinel binding, type mapping, owner predicates/functions (`IsOpen`, `ListSubstitutedGroupsToday`, `CanViewOperationalOverview`), error mapping (`ErrTeacherNotFound` -> not found) |
| `identity_caller_rows.go` | wiring: ID-to-row loading through owner capability, sentinel mapping, degraded-section reporting on load errors |
| `identity_caregiver_profiles.go` | wiring: owner predicate `identity.IsTeacher()`, sentinel mapping, port binding |
| `identity_contract.go` | wiring: type and sentinel aliases |
| `identity_credentials.go` | wiring: entropy source binding |
| `identity_demo_access_mail.go` | wiring: env-gated configuration (`IsDemoEnvironment`), mail texts and contact from owner (`DemoAccessLinkMail`, `DemoLeadMail`, `DemoContactAddress`), dispatcher binding, delivery-type metadata/logging |
| `identity_guardian_invitation.go` | wiring: binding, nil guards, type mapping (invitation records, mail recipient), owner predicate calls (`IsDeleted`, `GuardianInvitationExpiry`, `SchoolLogoURL`), settings read, error mapping (owner sentinels to parentportal sentinels) |
| `identity_guardian_invitation_mail.go` | wiring: outbox enqueue binding, payload serialization of owner-composed mail, owner kind/idempotency constants, idempotency-conflict error mapping |
| `identity_guardian_invitation_renderer.go` | wiring: payload key schema and deserialization, owner renderers, owner release rule `GuardianWelcomeRelease` mapped to delivery cancel/defer errors |
| `identity_invitation.go` | wiring: dependency binding, owner policy `securityruntime.CanGrantRole`, owner mail and origin functions, delivery-result mapping to `TokenDelivery`, owner constant kinds, configured welcome delay |
| `identity_memo.go` | wiring: context memo lookup |
| `identity_mfa.go` | wiring: settings key binding per tenant, hasher binding, mail dispatch with owner content, delivery-type metadata, async audit ledger write with panic recovery (observability) |
| `identity_operator_authentication.go` | wiring: binding, nil guards, type mapping (audit row, schools, organizations, identity facts), owner predicate calls (`PersonIsLive`, `StaffSchoolByPerson`, `StaffRecordCountsAtSchool`, `LatestPersonsByAccountSchool`, `PasswordTooWeak`), error mapping (`ErrNoRows` to false, `ErrPasswordTooWeak`) |
| `identity_operator_links.go` | wiring: dispatcher binding, owner mail functions, owner email-format rule, delivery-result mapping, failure logging |
| `identity_password_reset.go` | wiring: scope-to-host mapping from config, dispatcher binding, delivery-result mapping; `cleanupResetExpiry` is a placeholder for roots that never issue links |
| `identity_request_cache_middleware.go` | wiring: middleware attaching owner memo |
| `identity_request_tokens.go` | wiring: claims-to-caller type mapping, owner predicate `HasAdminWildcard`, owner `AuditActorName` |
| `identity_root.go` | wiring: composition of all identity seams, required-config fail-fast (TENANT_DOMAIN, operator hostname), viper reads, owner constants (`StaffWelcomeDelay`, `PasskeyRPName`), Delivery-state-to-owner enum mapping, renderer registration |
| `identity_rows.go` | wiring: consumer-owned port interfaces only |
| `identity_runtime_composition.go` | wiring: portal runtimes binding owner scope constants per portal, sentinel predicates, type mapping, error envelope stripping |
| `identity_seams.go` | wiring: sentinel pairing error type |
| `identity_sse_presence.go` | wiring: owner reads, error wrapping |
| `identity_staff_welcome_renderer.go` | wiring: payload schema and deserialization, owner `StaffWelcomeCancelled` / `StaffWelcomeMail`, delivery cancel/defer error mapping |
| `identity_welcome_mail.go` | wiring: owner help-role constants and `StaffHelpRole` + `securityruntime.LeadsSchool`, tenant settings resolution passed to owner `WelcomeHelpLink`, failure logging |
| `import_composition.go` | wiring: Data Import service construction over owner ports, owner rule `schoolmembership.CountsTowardChildQuota`, owner role policy/authorization constructors, test override hook |
| `import_school_lookups.go` | wiring: owner query binding, sentinel-to-absence mapping |
| `iot_error_report_composition.go` | wiring: observability binding, error mapping to IoT sentinels |
| `iot_http_composition.go` | wiring: route resource constructors, middleware chain, enum type mapping |
| `iot_http_metrics.go` | wiring: metrics middleware (observability) |
| `iot_http_runtime.go` | wiring: runtime binding, status/error-to-envelope mapping (api/common), permission middleware passthrough, conflict details envelope |
| `iot_session_end_runtime.go` | wiring: binding, error mapping (sessionend sentinels to sessionsAPI.EndFailure) |
| `kiosk_session_mirror.go` | wiring: binding, nil guard, type mapping |
| `kiosk_timetable.go` | wiring: binding, nil guard, type mapping (MirroredSession to ScheduledInstance, Berlin calendar/time parsing) |
| `manual_partial_absences.go` | wiring: binding, type mapping, error mapping |
| `master_data_decisions.go` | wiring: constructors, nil guards, owner predicate calls (securityruntime.ParentRequestReviewRights, policy.Scope), field-by-field type mapping, error mapping (MasterDataWriteError steps), realtime event binding |
| `membership_repositories.go` | wiring: constructors, binding |
| `membership_repository_support.go` | wiring: error shape mapping, date conversion helpers |
| `notification_http_composition.go` | wiring: nil guards, type mapping, error predicate binding |
| `notification_http_runtime.go` | wiring: route chain binding, runtime field binding, error mapping to api/common envelopes |
| `offering_change_composition.go` | wiring: constructors, nil guards, error mapping (lifecycle sentinels, database error shape), type mapping, owner filter/predicate calls (PendingOfferingChanges, CheckStoredJSON, ValidateParentRequestEvent), ledger and messenger adapters |
| `offering_change_planning.go` | wiring: binding, type mapping, owner function calls (ManualPlanningWindow, CourseGroupsForOfferings) |
| `open_room_move.go` | wiring: constructor, observability hook |
| `open_room_session_presence.go` | wiring: binding, SQL-free ID join, owner predicate calls (IsOpen, IsIndependentRoomSession, ParticipantLimitPtr), error wrapping |
| `open_room_sources.go` | wiring: binding, filter parameter mapping, type mapping |
| `operator_account_directory.go` | wiring: binding, SQL-free projection joining owner reads by ID, owner predicate/ordering calls (`PendingInvitation`, `OrderSchoolAccountListing`, `StaffRecordCountsAtSchool`, `CaregiverStatus`, `CompareAccountsBySchoolName`, `IsDeleted`), nil guards |
| `operator_audit_log.go` | wiring: constructor, nil guard, error wrapping, type mapping |
| `operator_billing.go` | wiring: constructor, binding |
| `operator_booking_authority.go` | wiring: HTTP surface binding, tenant transaction binding |
| `operator_provisioning_adapters.go` | wiring: constructors, nil guards, type mapping, error mapping (unique and FK violations to organizationCompose sentinels, not-found to bool), SQL-free ID joins for the person listing |
| `operator_provisioning_composition.go` | wiring: binding, type mapping (roles, invitations, accounts, PWA usage rows by tenant ID), owner predicate calls (`SystemRoles`, `PlatformSystemRole`, `IsLehrkraftSystemRole`, `IsPlatformCaregiverRole`, `OperatorSchoolInvitation`), settings read, error mapping (identity sentinels to organizationtenancy sentinels, `ErrRoleNotFound` to not found) |
| `operator_routes.go` | wiring: route mounting, middleware chains, rate limiter binding, nil guards |
| `operator_settings_composition.go` | wiring: nil guards, realtime event binding, date type mapping |
| `owner_http_mounts.go` | wiring: route mounting |
| `owner_roots.go` | wiring: constructors, binding, nil guards, type mapping (parent portal students/links/persons/schools, staff members, role classes), error mapping (`ErrAccountNotFound` to nil/empty) |
| `parent_announcement_composition.go` | wiring: nil guard, type mapping |
| `parent_care_plan_arrival_schedules.go` | wiring: nil guard, callback binding, type mapping |
| `parent_care_plan_care_exceptions.go` | wiring: nil guard, callback binding, type mapping |
| `parent_care_plan_commands.go` | wiring: constructors, nil guard |
| `parent_care_plan_excused_absence_requests.go` | wiring: nil guard, callback binding, type mapping |
| `parent_care_plan_guardian_absence_reports.go` | wiring: nil guard, callback binding, type mapping |
| `parent_care_plan_guardian_pickup_exceptions.go` | wiring: nil guard, callback binding, type mapping |
| `parent_care_plan_offering_change_requests.go` | wiring: nil guard, callback binding, type mapping |
| `parent_care_plan_offerings_values.go` | wiring: type mapping |
| `parent_care_plan_pickup_schedules.go` | wiring: nil guard, callback binding, type mapping |
| `parent_care_plan_policy.go` | wiring: owner function binding, error kind to owner sentinel mapping, type mapping |
| `parent_care_plan_requests_values.go` | wiring: type mapping |
| `parent_care_plan_schedules_values.go` | wiring: type mapping, owner predicate call (HasManualPartialAbsence) |
| `parent_care_plan_value_helpers.go` | wiring: generic conversion helpers |
| `parent_care_request_identity.go` | wiring: binding, error mapping |
| `parent_data_request_reads.go` | wiring: callback binding, error mapping, type mapping |
| `parent_delivery_http.go` | wiring: binding, nil guards, type mapping, error mapping (delivery sentinels to parentportal HTTP sentinels) |
| `parent_departure_policy.go` | wiring: constructor binding of the peopledirectory inspector and its owner constant DepartureDayOrder |
| `parent_guardian_changes.go` | wiring: constructor binding (audit guardian change log) |
| `parent_guardian_phone_reads.go` | wiring: callback binding, type mapping, owner normalization and predicate passthrough |
| `parent_guardian_profile_reads.go` | wiring: callback binding, field-by-field type mapping, error mapping (ErrGuardianProfileNotFound) |
| `parent_guardian_relationship_reads.go` | wiring: callback binding, field-by-field type mapping, error mapping (ErrStudentGuardianNotFound), owner permission predicate passthrough |
| `parent_message_people_ports.go` | wiring: constructor binding, type mapping |
| `parent_messaging_announcement_access_port.go` | wiring: nil guard, callback binding, type mapping |
| `parent_messaging_announcements_values.go` | wiring: field-by-field type mapping |
| `parent_messaging_conversation_core_port.go` | wiring: nil guard, callback binding, type mapping (round-trip copy-back) |
| `parent_messaging_declaration_policy_port.go` | wiring: nil guard, delegation of the owner's policy predicates, type mapping |
| `parent_messaging_declarations_values.go` | wiring: field-by-field type mapping |
| `parent_messaging_family_protection_reads_port.go` | wiring: nil guard, callback binding, type mapping |
| `parent_messaging_message_inbox_reads_port.go` | wiring: nil guard, callback binding, type mapping |
| `parent_messaging_message_reads_port.go` | wiring: nil guard, callback binding, type mapping |
| `parent_messaging_message_thread_access_port.go` | wiring: nil guard, callback binding, type mapping |
| `parent_messaging_messages_values.go` | wiring: field-by-field type mapping |
| `parent_messaging_request_share_records_port.go` | wiring: nil guard, callback binding, type mapping |
| `parent_messaging_sharing_values.go` | wiring: field-by-field type mapping |
| `parent_messaging_value_helpers.go` | wiring: generic nil-safe pointer/slice/map mapping helpers |
| `parent_offering_read_ports.go` | wiring: nil guards, type mapping, owner predicate call (row.HasFixedDays()) |
| `parent_person_reads.go` | wiring: callback binding, type mapping |
| `parent_portal_delivery_ports.go` | wiring: nil guards, adapter binding, type mapping |
| `parent_portal_enrollments.go` | wiring: delegation to Enrollment, type mapping into Care Plan vocabulary |
| `parent_portal_messaging_ports.go` | wiring: nil guard, type mapping, owner predicate call (communication.ParentMessagingEnabledForTenant) |
| `parent_portal_platform.go` | wiring: binding of securityruntime and peopledirectory policy functions, realtime event hook, nil guard, error binding |
| `parent_portal_read_ports.go` | wiring: nil guards, type mapping |
| `parent_portal_runtime.go` | wiring: delegation to the tenant runtime |
| `parent_portal_settings_ports.go` | wiring: optional-interface binding, delegation to Settings |
| `parent_request_events.go` | wiring: callback binding, type mapping |
| `parent_request_feed.go` | wiring: projection reader binding, type mapping |
| `parent_request_review_policy.go` | wiring: nil guards, delegation to identityaccess review scope and ReviewReachesGroup, securityruntime predicate mapping into ReviewPermissions |
| `parent_student_audit.go` | wiring: callback binding, type mapping |
| `parent_student_consents.go` | wiring: callback binding, persisted-ID guard, type mapping |
| `parent_student_reads.go` | wiring: callback binding, type mapping, JSON decoding of typed fields |
| `partial_absence_preview.go` | wiring: binding, owner predicate call (`peopledirectory.StudentStillEnrolled`) |
| `people_bindings.go` | wiring: type mapping into an owner predicate call (userModels.EnrolledOn) |
| `people_directory_composition.go` | wiring: constructor binding, owner seam assignment, type mapping, error binding (Renew id==0 to ErrStudentNotFound), observability hook, owner predicate call (LegacyPickupStatus()) |
| `people_directory_roots.go` | wiring: constructors, binding of securityruntime role-preset functions, panics on missing dependencies at wiring time |
| `people_read_ports.go` | wiring: nil guard, SQL-free projection joining person rows to students by ID, type mapping |
| `people_repositories.go` | wiring: constructors, binding, type mapping (assignment to group-ID projection), lock delegation adapter |
| `people_school_directories.go` | wiring: type mapping, owner predicate call (IsDeleted), not-found binding |
| `people_wiring.go` | wiring: constructors, nil guards, default logger, binding of factory closures |
| `person_projection_active.go` | wiring: projection join by ID, owner predicate call (studentpresence.VisitsFromAnotherSchool), nil guards, type mapping |
| `person_projection_adapters.go` | wiring: type mapping, constructor |
| `personal_reminder_notifications.go` | wiring: binding, type mapping, owner function call (workforce.OnDutyStaffIDs), error binding (errors.Is on delivery sentinels) |
| `plan_export.go` | wiring: binding, type mapping (records, wall-clock and date parsing), owner predicate call (IsCancelled), nil-row guards |
| `plan_export_staff_names.go` | wiring: type mapping, nil guards (nil slot kept, label stays with the owner) |
| `platform_wiring.go` | wiring: constructors, nil guards, error wrapping, binding of factory closures |
| `portal_scope.go` | wiring: middleware binding |
| `presence_bindings.go` | wiring: type mapping, filter field mapping, nil guard, constructor |
| `presence_device_directory.go` | wiring: type mapping |
| `presence_events.go` | wiring: transport binding (owner announcement to realtime event and topic mapping), nil guards, observability logging |
| `presence_projections.go` | wiring: type mapping |
| `presence_reads.go` | wiring: binding, tenant-context guard, type mapping over the owner's projection reader |
| `presence_room_directory.go` | wiring: type mapping, nil guard |
| `presence_session_records.go` | wiring: constructors, nil guards, owner constant pass-through (studentpresence.SupervisionStaffIncludesRemoved), projection join by ID, type mapping |
| `presence_wiring.go` | wiring: constructors, nil guards, default logger, binding |
| `presence_work_sessions.go` | wiring: enum type mapping (StampNFC to WorkSessionSourceNFC, otherwise App) |
| `pwa_usage.go` | wiring: route mounting, error binding to api/common envelope |
| `readonly_preview.go` | wiring: middleware binding, claim predicate call |
| `reminder_delivery.go` | wiring: binding, type mapping, nil guard |
| `reminder_identity.go` | wiring: nil guard, error binding (not found to reminder.ErrNotLinkedToStaff) |
| `reminder_people.go` | wiring: type mapping, owner function call (GetFullName), port-to-owner filter mapping (ActiveOn = timezone.TodayDate()), error binding to studentpresence.OperationError |
| `reminder_readers.go` | wiring: clock binding, type mapping, settings key binding (owner-named configModel keys), owner predicate call (IsBinaryPresence), error wrapping |
| `reminder_rooms.go` | wiring: type mapping |
| `reminder_timetable.go` | wiring: type mapping (date and time parsing), error binding (WrapDatabaseError), empty-input guard |
| `request_id.go` | wiring: middleware, observability, error binding to api/common |
| `room_activity_names.go` | wiring: type mapping |
| `room_history_presence.go` | wiring: type mapping |
| `room_occupancy_presence.go` | wiring: type mapping |
| `route_catalog.go` | wiring: route contract loading (embedded golden parse) |
| `runtime_metrics.go` | wiring: observability and metrics hooks, metrics auth middleware |
| `scheduler_ports.go` | wiring: binding, type mapping, nil guards, owner predicate call (timetable.IsPlannedInstanceStatus), settings key verification |
| `scheduler_presence_ports.go` | wiring: binding, type mapping, nil guards, owner function call (careplan.EndOfDayStatusFlagArchive) |
| `scheduler_time_tracking_cleanup.go` | wiring: binding, type mapping, nil guard |
| `scheduler_timetable_ports.go` | wiring: binding, type mapping, nil guards, owner constant pass-through (MaterializationSourceScheduler) |
| `school_calendar_composition.go` | wiring: settings key binding (KeyFederalState), error binding (careplan sentinel to schoolcalendar and timetable sentinels), nil guards, type mapping |
| `school_directories.go` | wiring: type mapping, owner predicate calls (Operational, IsDeleted), not-found binding, projection join by ID, nil guards |
| `school_http_composition.go` | wiring: binding, nil-pointer normalization |
| `school_lookup.go` | wiring: owner query binding, error mapping (ErrSchoolNotFound to found=false), ID dedup and keyed projection |
| `school_membership.go` | wiring: constructor, type mapping (StaffEmploymentProfile <-> workforce.StaffEmployment), error mapping (Workforce to School Membership sentinels) |
| `school_projection_adapters.go` | wiring: type mapping (organizationtenancy.School to presenceservice.School) |
| `school_setup_composition.go` | wiring: constructors, route mounting, middleware chain, permission binding (owner-named schoolSetupHTTP.WritePermissions()), error mapping to api/common envelopes |
| `school_structure_events.go` | wiring: binding (Delivery group events) |
| `security_principal_middleware.go` | wiring: middleware, type mapping (claims to securityruntime.PrincipalFacts) |
| `sentry_session.go` | wiring: observability hook (Sentry tags from token scope via owner predicates IsPlatformScope/IsSchoolScope/IsReadOnlyPreview) |
| `server.go` | wiring: constructors, deployment config validation, worker dependency binding, nil guards, probe/route mounting, observability hooks, HTTP lifecycle |
| `server_error_reporting.go` | wiring: observability hook (Sentry middleware, query-string stripping) |
| `service_graph.go` | wiring: type definitions, config reading (viper/env), error mapping (expectedMissingSubstitutionIdentity), permission binding (staffAdministrationPermitted feeds the field MayManageStaff), nil guard (feedbackCounterOrUnconfigured), outbox adapter type mapping, deployment config (enforceInstanceTimePolicy maps app_env to a flag) |
| `service_graph_compose.go` | wiring: constructors, staged composition, nil guards, deployment config validation (resolveLinks https in production), settings-key binding, owner function calls (MissingPersonnelNumbers, SenderOrDefault, role presets from securityruntime), observability |
| `service_graph_compose_requests.go` | wiring: constructors, renderer registry binding, VAPID config, late binding, owner predicate calls (workforceModule.CanSeeAllSubstitutions, configModels.OverviewOpenToAllStaff), nil guards |
| `service_graph_compose_timetable.go` | wiring: constructors, binding, late-binding closures, deployment config (`enforceInstanceTimePolicy(b.Config.AppEnv)`), observability hook, settings reads by owner key |
| `service_graph_compose_views.go` | wiring: constructors, projection binding, late binding, nil guards, owner predicate binding (GuardianPermissionGranted) |
| `service_graph_tenant_runtime.go` | wiring: reflection-based tenant runtime binding, nil guards |
| `service_graph_wiring.go` | wiring: type definitions for wiring structs, completeness validation (nil guards) |
| `session_activities.go` | wiring: type mapping (timetable.Group to presenceCompose.SessionActivity) |
| `session_devices.go` | wiring: binding (device IDs plus online-window resolver) |
| `session_end.go` | wiring: constructor, observability hook, typed-nil guard |
| `session_route_assembly.go` | wiring: middleware chains |
| `session_token_bindings.go` | wiring: config reading (viper JWT secret/expiry), binding, type mapping (claims to RequestClaims) |
| `settings_adapters.go` | wiring: type mapping (School settings store over organizationtenancy), nil guard |
| `settings_bindings.go` | wiring: settings-key binding for narrow owner ports, owner predicate calls (SchoolPortalWritesClassArrivalExceptions), owner-named fallbacks (AttendanceLogEnabledFallback, RoomDetailVisibleDaysFallback), side-effect registration, snapshot prefetch |
| `settings_composition.go` | wiring: binding, middleware/route runtime, error mapping (status to api/common HTTP envelopes), upload parsing adapters, owner function calls (`PublicLegalDocumentURL`) |
| `settings_permissions.go` | wiring: type mapping (HTTP access enums 1:1 to securityruntime enums), owner permission-set calls (`ConfigAccessPermissions`, `MealPlanAccessPermissions`, `CanEditConfig`), middleware construction |
| `settings_request_cache_middleware.go` | wiring: middleware (settings request cache) |
| `simple_list_export.go` | wiring: type mapping to documentrendering/lists, row-shape guard, owner sanitizer call |
| `sse_http_runtime.go` | wiring: runtime binding (identity, token deadline, tenant unit of work), route mounting, middleware chains, owner predicate (HasAdminWildcard) |
| `staff_admin_composition.go` | wiring: constructor, nil guards, type mapping (staff profile, audit section/resource maps), error mapping (ErrPersonnelNumberConflict to ErrPersonnelNumberTaken), owner predicate (member.IsDeleted()), permission matcher binding (securityruntime.HasPermission) |
| `staff_clock.go` | wiring: constructors, nil guards, type mapping (Workforce sessions/breaks to kiosk), error mapping (Workforce refusals to staffclock kinds) |
| `staff_composition.go` | wiring: constructors, route mounting, error mapping (failure kinds to HTTP envelopes, constraint violation to owner sentinel ErrStaffStillReferenced), type mapping, permission binding (staffPermissionGate takes owner-named permissions), observability |
| `staff_membership_adapters.go` | wiring: legacy-repository type mapping, error mapping, ID-joined person hydration without SQL, owner predicate (schoolmembership.ExternalCaregiver), owner-named permission binding (staffCalendarHTTP.StaffInvitePermission) |
| `staff_membership_composition.go` | wiring: runtime binding, nil guards, type mapping, error mapping (ClassifyStaffWriteFailure, ClassifyStaffSchoolClassFailure, ErrInUse to ErrStaffInUse), owner-named default permission (securityruntime.DefaultStaffAccountPermission) |
| `staff_membership_guest.go` | wiring: type mapping (legacy Guest and filter keys to schoolmembership types), nil guards, error mapping (`repositoryError`/`membershipError`) |
| `staff_membership_models.go` | wiring: type mapping (legacy staff/teacher to schoolmembership values) |
| `staff_membership_teacher.go` | wiring: legacy-repository type mapping, legacy filter translation, ID-joined staff/person hydration, owner calls (schoolmembership.ActiveCaregivers, SortActiveCaregivers), error mapping |
| `staff_notice_names.go` | wiring: projection (account ID to owner FullName), nil guard |
| `staff_offboarding_owners.go` | wiring: constructors, owner binding for the offboarding workflow |
| `staff_projection_auth.go` | wiring: owner function call (schoolmembership.StaffChainsByPerson), type mapping |
| `staff_projection_education.go` | wiring: type mapping (Workforce handovers to School Structure values), ID-joined group/staff attachment, error mapping (ErrHandoverExists), filter translation |
| `statistics_composition.go` | wiring: binding, constructor, observability hook (DebugLogger), owner adapter assembly |
| `statistics_http_runtime.go` | wiring: owner-named permission binding (presenceStudentsHTTP.StatisticsReportPermissions()), claims-to-actor type mapping, renderer binding with nil guard, document type mapping |
| `statistics_inputs.go` | wiring: type mapping between owner types, owner constant/predicate calls (ResourceTypeAttendanceStatistics, HolidaysOverlapping, EnrolledOn), nil guards |
| `student_absence_notifications.go` | wiring: binding, nil guard, type mapping |
| `student_audit_composition.go` | wiring: binding, snapshot type mapping, owner constants (StudentAuditSystemActorID/Name), nil guards |
| `student_change_log_cleanup_composition.go` | wiring: binding, nil guards, settings key binding (KeyGDPRStudentChangeLogRetentionDays), owner record constructor |
| `student_consent_composition.go` | wiring: type mapping, owner predicate calls (PhotoConsentWithdrawnAt, StudentConsentTransitions), action-enum mapping, nil/persisted-ID guards, single-owner trail append |
| `student_deletion_owners.go` | wiring: constructors, binding of owner counters, error propagation |
| `student_display_projection.go` | wiring: SQL-free projection joining two owners' reads by ID, type mapping |
| `student_document_storage_composition.go` | wiring: lazy constructor binding |
| `student_owner_http.go` | wiring: binding owner transports, error binding (errors.Is ErrStudentStatusDayPartialAbsenceConflict), owner-provided label/renderer binding |
| `student_photo_composition.go` | wiring: binding, nil guards, owner function calls (`ApplyPhotoFeatureSetting`, `ApplyStudentPhotoConsent`), owner-named source constant binding (`StudentConsentSourceTenantPortal`), realtime/observability hooks, settings side-effect registration |
| `student_presence_roots.go` | wiring: constructors, composition of owners over one DB, observability hooks, panic on composition error |
| `student_records.go` | wiring: type mapping, nil guards, not-found error binding (affected == 0) |
| `student_root_reads.go` | wiring: binding, type mapping, nil guards |
| `student_schedule_adapters.go` | wiring: constructors |
| `students_owner_ports.go` | wiring: type mapping, nil guards |
| `supervision_dashboard.go` | wiring: binding, nil-guard constructors, owner-named permission binding (ScheduleReadPermission, StudentReadPermission), owner predicate calls (CanViewOperationalOverview, SortSessionsByRoom), settings prefetch keys, type mapping, bulk room resolution by ID, wall-clock formatting |
| `teaching_assignment_adapters.go` | wiring: binding, owner validation calls (ValidateClassAssignment, ValidateGroupAssignment, GroupTeacherBlockers), filter mapping, error wrapping |
| `time_tracking_absence_email.go` | wiring: binding, type mapping, owner-named permission binding (AbsenceApproverPermission), tenant nil guard |
| `time_tracking_audits.go` | wiring: type mapping between owner types, nil guards |
| `time_tracking_composition.go` | wiring: constructors, claims-to-identity mapping, type mapping, tenant-scope binding |
| `time_tracking_documents.go` | wiring: renderer binding, type mapping, nil guard |
| `time_tracking_events.go` | wiring: binding to Delivery producer |
| `time_tracking_ports.go` | wiring: port binding, constructors, type mapping |
| `time_tracking_staff.go` | wiring: type mapping, nil guards, binding to School Membership command, error binding |
| `timetable_advisory_locks.go` | wiring: binding to tenant runtime lock |
| `timetable_audit_ports.go` | wiring: compile-time vocabulary equality check, type mapping, nil guard |
| `timetable_cleanup_composition.go` | wiring: constructor, nil guards, settings key binding (KeyGDPRTimetableRetentionDays), observability log |
| `timetable_conflict_composition.go` | wiring: binding, nil guard, type mapping |
| `timetable_data_composition.go` | wiring: binding, nil guards, type mapping, owner query call (CheckRoomConflict) |
| `timetable_deviation_composition.go` | wiring: binding, dependency guard, type mapping, realtime event mapping |
| `timetable_http_ports.go` | wiring: type mapping, nil guards, owner predicate calls (AttendsOn, ExplainEmptyOfferingRoster, DeriveTemplateRosterMaintenance), error binding (ErrInvalidReportFilter) |
| `timetable_lifecycle_composition.go` | wiring: binding, dependency guard, error binding (ErrParentAnnouncementValidation -> ErrGuardianNoticeInvalid), owner validation call, event mapping |
| `timetable_operation_people.go` | wiring: binding, type mapping |
| `timetable_operations_composition.go` | wiring: binding, settings key binding (incl. scoped-action -> settings key map), owner predicate call (StudentAbsenceEditOpenToAllStaff), type mapping, realtime event mapping |
| `timetable_owner_rows.go` | wiring: type mapping, nil guard |
| `timetable_people_views.go` | wiring: type mapping, generic row mappers |
| `timetable_repositories.go` | wiring: constructors, nil guards, type mapping |
| `timetable_session_end_composition.go` | wiring: binding, nil guard, type mapping, owner predicate calls (AttendanceRowCareDay, Expected, ExemptFromAbsence) |
| `timetable_staffing.go` | wiring: binding (lifecycle as staffing announcer via type assertion), nil-tolerant guard |
| `timetable_students.go` | wiring: type mapping (People Directory enrolled students to Timetable target students, Facilities rooms to RoomRef), binding |
| `timetable_template_composition.go` | wiring: constructors, field-by-field dependency binding, nil guards, error binding (`errors.Is(err, careplan.ErrCareOfferingConfigInvalid)`), School Structure rule binding, realtime event hook |
| `timetable_wiring.go` | wiring: constructors, binding of owners/adapters into TimetableWiring and reminder query dependencies, error wrapping |
| `upload_ports.go` | wiring: binding (common upload helpers bound to objects.UploadFiles) |
| `weekend_plan.go` | wiring: nil guard, binding of an owner setting key (`configSvc.KeyWeekendFollowsFriday`) into a calendar context hook |
| `worker_lease.go` | wiring: constructor, metrics/observability hooks binding |
| `worker_retention.go` | wiring: constructors, binding, nil guards, owner-named deletion-type constants bound to owner retention steps, owner plan/window calls (`RetentionPlan`, `StatusDayRetention`, `StaffRetentionWindow`, `StudentRetentionWindow`), error mapping (`ErrSavepointControl`, `ErrStatusDayRetentionWindow`) |
| `workflow_audits.go` | wiring: constructor, type mapping (workflow values to audit records, owner-named source/deletion-type constants), compile-time enum drift check, nil guard, observability hook |
| `workforce_absence_types_http.go` | wiring: nil guard, runtime binding, permission binding (`TimeTrackingOwn`/`TimeTrackingManage`/`VacationApprove` passed into fields of the same names), error mapping to the common envelope |
| `workforce_admin_capabilities.go` | wiring: capability bundling, nil guards, setting key binding, type mapping (person/staff rows to Workforce public types) |
| `workforce_composition.go` | wiring: constructors, binding of School Membership queries to Workforce ports (ID projection, lock call), dependency binding |
| `workforce_legacy_schedule_repositories.go` | wiring: adapter over the Workforce capability, type mapping between legacy and public records, error binding (`LegacyWorkTimeModelNotFound`), nil guards |
| `workforce_plan_staff.go` | wiring: type mapping (People Directory staff rows to Workforce PlanStaff), error pass-through |
| `workforce_schedule_records.go` | wiring: type mapping between retained schedule/model rows and Workforce compose rows, binding of owner functions (`config.DailyTargetFromSchedule`/`DailyTargetFromModel`), grouping by staff ID |
| `workforce_shift_planning_http.go` | wiring: nil guards, runtime binding, permission binding (`TimeTrackingManage`/`SchedulesRead`/`UsersRead` into fields of the same names), owner predicate call (`common.CanExportInternalPlan`), error mapping of sentinels to common codes and envelopes, including rollback on `ErrStaffShiftDuplicate` |
| `workforce_shift_rows.go` | wiring: compile-time vocabulary guard, constructors, nil guards, type mapping, error binding (Workforce sentinels to Timetable record errors) |
| `workforce_staff_absences.go` | wiring: cross-owner projection joining School Membership and People Directory reads by ID, calling owner predicates/constants (`AbsenceSubjectStaffIDs`, `AbsenceSubjectSearch`, `AbsenceRequestsIncludeOffboardedStaff`), ID de-duplication, error wrapping |
| `workforce_substitutions.go` | wiring: adapter type mapping between the schoolstructure substitution module and the Workforce capability, date parsing bound to the owner error (`workforceModule.InvalidSubstitutionPeriod`), error mapping table |
| `workforce_substitutions_http.go` | wiring: nil guard, runtime binding, error rendering to the common envelope |
| `workforce_work_time_models_http.go` | wiring: nil guard, runtime binding, permission binding (`Manage: permissions.TimeTrackingManage`), error mapping to the common envelope |
