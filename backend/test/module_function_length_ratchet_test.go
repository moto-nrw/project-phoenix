package test

import (
	"fmt"
	"go/ast"
	"sort"
	"strings"
	"testing"
)

// TestModuleFunctionLengthRatchet caps function body length in the modules
// that migration #2580 carved out, every root (ADR 0082), and every backend
// tree (ADR 0087).
//
// Why a length gate: #2580 moved 228,306 of 638,717 production LOC into
// backend/modules/ and backend/workflows/, where no gate in this package
// measured function length, so the code that grew fastest was exactly the
// code nothing measured. A 300-line service
// method is the shape .claude/rules/backend-conventions.md keeps legislating
// against from three directions — Rule 4 (thin handlers), Rule 8 (services
// encapsulate, not delegate) and Rule 12 (models hold data, not decisions) —
// without any of them counting lines. This ratchet counts lines, which is the
// crude proxy that catches the cases cognitive complexity misses: long, flat,
// branch-free orchestration that should be three named steps.
//
// The threshold is 60 body lines, measured from the opening brace line to the
// closing brace line inclusive, on named declarations only. Function literals
// are not counted: a long literal is a property of its enclosing declaration,
// which the ratchet already measures.
//
// Allowlist semantics (per function, not per file — four cases):
//
//   - A function NOT in the allowlist must have a body of ≤ 60 lines.
//   - An allowlisted function may never exceed its recorded length.
//   - When a split shortens a function, the test fails until the entry is
//     lowered — the ratchet only turns one way. Never raise a number.
//   - When a function drops to ≤ 60 lines (or is deleted or renamed) it
//     disappears from the scan and its entry must be removed.
//
// Keys are "relative/file.go:FuncName" with methods rendered the way the
// gocognit CLI renders them ("(*service).ListOptions"), so entries here line
// up with the neighbouring complexity ratchet's keys.
//
// Scope: walkModuleRatchetSources covers all backend trees recursively,
// including legacy, migrations, tools, generated code, hooks and test support.
// Only _test.go and hidden directories are excluded. ADR 0087 records the
// category audit and exact additions measured at 8aca1925f7. SQL strings in
// migration bodies count as physical lines, not as Go branching.
//
// Seed: measured 2026-09-18 at merge commit ecf0003369 with a throwaway
// go/ast walk over modules/ and workflows/ (named FuncDecls with a body,
// non-test files), recording fn.Body.Rbrace.Line - fn.Body.Lbrace.Line + 1
// per function:
//
//	cd backend && ../scripts/run-go-toolchain.sh go run ./tmp/moduleFuncLen
//
// The temporary seed helper was not retained. Re-measure the current tree
// with the ratchet itself (a mismatch prints the measured values):
//
//	cd backend && ../scripts/run-go-toolchain.sh go test -tags=testhooks ./test/ \
//	  -run '^TestModuleFunctionLengthRatchet$' -count=1 -v
//
// The first seed was taken at 19feca2822 and re-measured here after the merge
// of origin/development (a47f77b2c3): PR #3408 (issue #3350) moved seven
// over-threshold functions into modules/, which the ratchet cannot absorb
// without a fresh measurement.
//
// 299 of 14,623 functions were over the threshold at seed time (483 over 50
// lines, 83 over 100, 5 over 200). The seed freezes that state; it is not a
// statement that these 299 functions are acceptable.
const moduleFuncLenThreshold = 60

// Seeded 2026-09-18 from merge commit ecf0003369; module entries drained. Shrink-only:
// additions or raises require a new-numbered ADR cited by the entry (ADR 0070).
// ADR 0082: newly covered sources, measured at 722c5aa147; no headroom.
var moduleFuncLenAllowlist = map[string]int{
	"api/active_test_helpers.go:buildActiveTestModule":                   185,
	"api/auth_test_helpers.go:NewAuthTestModule":                         169,
	"api/calendar_test_helpers.go:NewCalendarTestModule":                 63,
	"api/guardian_invitation_test_helpers.go:lifecycleTestModule":        61,
	"api/guardians_composition.go:newGuardiansResource":                  108,
	"api/import_test_helpers.go:NewImportTestModuleWithOwners":           82,
	"api/iot/compose/compose.go:(*Resource).Router":                      110,
	"api/reminders_test_helpers.go:NewRemindersTestModule":               64,
	"api/root_serve.go:composeServeRoot":                                 511,
	"api/root_test_root.go:composeTestRootWith":                          283,
	"api/serve_mounts_portals.go:newOperatorRouter":                      65,
	"api/serve_mounts_students.go:mountStudentRoutes":                    120,
	"api/serve_mounts_students.go:requestReviewDependencies":             100,
	"api/settings_composition.go:newSettingsResource":                    61,
	"api/student_owner_http_test_helpers.go:NewStudentHTTPTestResources": 176,
	"api/student_owner_http_test_helpers.go:newStudentHTTPTestReviews":   103,
	"api/student_test_helpers.go:newStudentTestModule":                   315,
	"api/timetable_http_test_helpers.go:NewTimetableHTTPTestModule":      85,
	"api/timetable_http_test_helpers.go:newTimetableHTTPTestReads":       78,
	"api/timetable_test_helpers.go:NewTimetableTestModule":               169,
	"api/workforce_test_helpers.go:NewWorkforceTestModule":               102,
	"cmd/devtools/command_adapter.go:(*Adapter).LoginOperator":           65,
	"cmd/gendoc.go:getSettingsSchemas":                                   79,
	// ADR 0087.
	"database/migrations/000000_create_schemas.go:init": 72,
	// ADR 0087.
	"database/migrations/000001001_infrastructure.go:init": 77,
	// ADR 0087.
	"database/migrations/001000001_auth_accounts.go:createAuthAccountsTable": 61,
	// ADR 0087.
	"database/migrations/001000004_auth_roles.go:createAuthRolesTable": 65,
	// ADR 0087.
	"database/migrations/001000005_auth_permissions.go:createAuthPermissionsTable": 85,
	// ADR 0087.
	"database/migrations/001000006_auth_role_permissions.go:createAuthRolePermissionsTable": 101,
	// ADR 0087.
	"database/migrations/001000007_auth_account_roles.go:createAuthAccountRolesTable": 74,
	// ADR 0087.
	"database/migrations/001000008_auth_account_permissions.go:createAuthAccountPermissionsTable": 180,
	// ADR 0087.
	"database/migrations/001001005_facilities_rooms_optional_fields.go:restoreRoomFieldsNotNull": 72,
	// ADR 0087.
	"database/migrations/001002003_users_staff.go:usersStaffUp": 61,
	// ADR 0087.
	"database/migrations/001002006_users_guardians.go:usersPersonsGuardiansUp": 88,
	// ADR 0087.
	"database/migrations/001002007_education_groups.go:createEducationGroupsTable": 86,
	// ADR 0087.
	"database/migrations/001002008_education_group_substitution.go:createEducationGroupSubstitutionTable": 62,
	// ADR 0087.
	"database/migrations/001003003_activities_schedules.go:createActivitiesSchedulesTable": 66,
	// ADR 0087.
	"database/migrations/001003004_activities_supervisors_planned.go:createActivitySupervisorsTable": 108,
	// ADR 0087.
	"database/migrations/001003005001_users_guardian_profiles.go:usersGuardianProfilesUp": 91,
	// ADR 0087.
	"database/migrations/001003005_users_students.go:usersStudentsUp": 90,
	// ADR 0087.
	"database/migrations/001003006_users_students_guardians.go:usersStudentsGuardiansUp": 95,
	// ADR 0087.
	"database/migrations/001003007_users_privacy_consents.go:usersPrivacyConsentsUp": 156,
	// ADR 0087.
	"database/migrations/001003009_iot_devices.go:createIoTDevicesTable": 77,
	// ADR 0087.
	"database/migrations/001004001_active_groups.go:createActiveGroupsTable": 89,
	// ADR 0087.
	"database/migrations/001004002_active_visits.go:createActiveVisitsTable": 78,
	// ADR 0087.
	"database/migrations/001004003_active_groups_supervisors.go:createGroupSupervisorsTable": 71,
	// ADR 0087.
	"database/migrations/001005003_fix_permission_names.go:fixPermissionNames": 139,
	// ADR 0087.
	"database/migrations/001005003_fix_permission_names.go:revertPermissionNames": 88,
	// ADR 0087.
	"database/migrations/001006001_config_settings.go:createConfigSettingsTable": 73,
	// ADR 0087.
	"database/migrations/001006002_create_admin_account.go:createAdminAccount": 99,
	// ADR 0087.
	"database/migrations/001006003_grant_all_permissions_to_admin.go:grantAllPermissionsToAdmin": 104,
	// ADR 0087.
	"database/migrations/001006004_seed_activity_categories.go:removeSeededActivityCategories": 61,
	// ADR 0087.
	"database/migrations/001006005_create_attendance_table.go:createAttendanceTable": 81,
	// ADR 0087.
	"database/migrations/001006008_add_substitution_permissions_to_teacher.go:addSubstitutionPermissionsToRoles": 78,
	// ADR 0087.
	"database/migrations/001006016_auth_guardian_invitations.go:authGuardianInvitationsUp": 73,
	// ADR 0087.
	"database/migrations/001006017_audit_data_imports.go:createAuditDataImportsTable": 75,
	// ADR 0087.
	"database/migrations/001006018_import_performance_indexes.go:createImportPerformanceIndexes": 77,
	// ADR 0087.
	"database/migrations/001006021_restrict_substitution_permissions_to_admin.go:restrictSubstitutionPermissionsDown": 70,
	// ADR 0087.
	"database/migrations/001006021_restrict_substitution_permissions_to_admin.go:restrictSubstitutionPermissionsUp": 65,
	// ADR 0087.
	"database/migrations/001007001_add_missing_permissions.go:addMissingPermissions": 85,
	// ADR 0087.
	"database/migrations/001007004_add_domain_roles_with_permissions.go:addDomainRolesWithPermissions": 178,
	// ADR 0087.
	"database/migrations/0010070061_grade_transitions.go:createGradeTransitionsTables": 135,
	// ADR 0087.
	"database/migrations/001007006_guardian_phone_numbers.go:guardianPhoneNumbersUp": 147,
	// ADR 0087.
	"database/migrations/001008001_create_pickup_schedules.go:createPickupSchedulesUp": 66,
	// ADR 0087.
	"database/migrations/001008003_activities_groups_created_by.go:migrateActivitiesGroupsCreatedBy": 138,
	// ADR 0087.
	"database/migrations/001009001_create_suggestions.go:createSuggestionsTables": 83,
	// ADR 0087.
	"database/migrations/001009004_consolidate_roles.go:consolidateRolesDown": 112,
	// ADR 0087.
	"database/migrations/001009004_consolidate_roles.go:consolidateRolesUp": 91,
	// ADR 0087.
	"database/migrations/001011001_create_platform_schema.go:createPlatformSchemaTables": 143,
	// ADR 0087.
	"database/migrations/001011003_create_operator_account.go:createOperatorAccount": 71,
	// ADR 0087.
	"database/migrations/001012001_unified_suggestion_comments.go:createUnifiedComments": 84,
	// ADR 0087.
	"database/migrations/001012001_unified_suggestion_comments.go:rollbackUnifiedComments": 71,
	// ADR 0087.
	"database/migrations/001012005_add_reader_type_to_reads.go:createReaderType": 82,
	// ADR 0087.
	"database/migrations/001013001_create_organizations_and_schools.go:createOrganizationsAndSchools": 81,
	// ADR 0087.
	"database/migrations/001014001_create_tenant_roles.go:createTenantRoles": 166,
	// ADR 0087.
	"database/migrations/001014001_create_tenant_roles.go:rollbackTenantRoles": 88,
	// ADR 0087.
	"database/migrations/001014002_add_tenant_id_to_all_tables.go:addTenantIDToAllTables": 98,
	// ADR 0087.
	"database/migrations/001014003_migrate_unique_constraints.go:migrateUniqueConstraints": 340,
	// ADR 0087.
	"database/migrations/001014003_migrate_unique_constraints.go:rollbackUniqueConstraints": 323,
	// ADR 0087.
	"database/migrations/001014005_create_tenant_indexes.go:createTenantIndexes": 85,
	// ADR 0087.
	"database/migrations/001014005_create_tenant_indexes.go:rollbackTenantIndexes": 69,
	// ADR 0087.
	"database/migrations/001015001_enable_rls_policies.go:enableRLSPolicies": 71,
	// ADR 0087.
	"database/migrations/001015002_create_composite_fks.go:createCompositeFKs": 64,
	// ADR 0087.
	"database/migrations/001015005_security_invoker_views.go:init": 63,
	// ADR 0087.
	"database/migrations/001015007_disable_legacy_bootstrap_admin.go:disableLegacyBootstrapAdmin": 62,
	// ADR 0087.
	"database/migrations/001015012_drop_legacy_phone_sync_trigger.go:rollbackDropLegacyPhoneSync": 79,
	// ADR 0087.
	"database/migrations/001015013_fix_phoenix_auth_token_grants.go:init": 64,
	// ADR 0087.
	"database/migrations/001015014_fix_device_auth_permissions.go:init": 71,
	// ADR 0087.
	"database/migrations/001015017_backfill_role_tenant_ids.go:backfillRoleTenantIDs": 125,
	// ADR 0087.
	"database/migrations/001015020_add_persons_soft_delete.go:init": 77,
	// ADR 0087.
	"database/migrations/001015022_announcement_org_tenant_targeting.go:createAnnouncementOrgTenantTargeting": 69,
	// ADR 0087.
	"database/migrations/001015025_config_setting_values.go:createSettingValuesTables": 74,
	// ADR 0087.
	"database/migrations/001015026_platform_operator_email_change_tokens.go:createPlatformOperatorEmailChangeTokensTable": 82,
	// ADR 0087.
	"database/migrations/001015027_platform_operator_invitation_tokens.go:createPlatformOperatorInvitationTokensTable": 80,
	// ADR 0087.
	"database/migrations/001015032_create_arrival_schedules.go:createArrivalSchedulesUp": 155,
	// ADR 0087.
	"database/migrations/001015033_create_calendar_periods.go:createCalendarPeriodsUp": 63,
	// ADR 0087.
	"database/migrations/001015034_template_extensions.go:templateExtensionsUp": 121,
	// ADR 0087.
	"database/migrations/001015036_create_activity_instances.go:createActivityInstancesUp": 328,
	// ADR 0087.
	"database/migrations/001015045_rooms_color_unique.go:roomsColorUniqueUp": 78,
	// ADR 0087.
	"database/migrations/001015048_rooms_wc_alias_unique.go:roomsWCAliasUniqueUp": 103,
	// ADR 0087.
	"database/migrations/001015057_guardian_profile_account_fk.go:guardianProfileAccountFKUp": 68,
	// ADR 0087.
	"database/migrations/001015058_create_email_outbox.go:createEmailOutboxUp": 65,
	// ADR 0087.
	"database/migrations/001015063_create_care_offerings.go:createCareOfferingsUp": 71,
	// ADR 0087.
	"database/migrations/001015067_create_enrollment_phases.go:init": 84,
	// ADR 0087.
	"database/migrations/001015068_repoint_enrollment_to_phases.go:init": 127,
	// ADR 0087.
	"database/migrations/001015073_request_children_renewal_columns.go:init": 104,
	// ADR 0087.
	"database/migrations/001015074_form_schemas_add_name.go:init": 103,
	// ADR 0087.
	"database/migrations/001015085_platform_operator_mfa.go:platformOperatorMFAUp": 104,
	// ADR 0087.
	"database/migrations/001015089_drop_mfa_recovery_codes.go:init": 71,
	// ADR 0087.
	"database/migrations/001015091_mfa_trusted_devices_tenant_scope.go:init": 66,
	// ADR 0087.
	"database/migrations/001015092_mfa_overrides_tenant_scope.go:init": 149,
	// ADR 0087.
	"database/migrations/001015102_create_work_time_models.go:createWorkTimeModels": 149,
	// ADR 0087.
	"database/migrations/001015105_vacation_workflow.go:addVacationWorkflow": 133,
	// ADR 0087.
	"database/migrations/001015120_add_staff_soft_delete.go:init": 85,
	// ADR 0087.
	"database/migrations/001015130_students_allowed_departure_modes.go:studentsAllowedDepartureModesUp": 83,
	// ADR 0087.
	"database/migrations/001015133_passkey_credentials.go:passkeyCredentialsUp": 130,
	// ADR 0087.
	"database/migrations/001015138_students_departure_accompanied.go:studentsDepartureAccompaniedDown": 120,
	// ADR 0087.
	"database/migrations/001015138_students_departure_accompanied.go:studentsDepartureAccompaniedUp": 61,
	// ADR 0087.
	"database/migrations/001015139_care_offering_auto_rules.go:init": 77,
	// ADR 0087.
	"database/migrations/001015140_enrollment_offering_adjustments.go:init": 69,
	// ADR 0087.
	"database/migrations/001015144_guardian_change_audit.go:init": 74,
	// ADR 0087.
	"database/migrations/001015148_enrollment_change_requests.go:init": 106,
	// ADR 0087.
	"database/migrations/001015149_parent_messaging.go:parentMessagingDown": 98,
	// ADR 0087.
	"database/migrations/001015149_parent_messaging.go:parentMessagingUp": 310,
	// ADR 0087.
	"database/migrations/001015150_repair_enrollment_change_requests.go:init": 109,
	// ADR 0087.
	"database/migrations/001015151_create_student_data_change_requests.go:createStudentDataChangeRequestsUp": 68,
	// ADR 0087.
	"database/migrations/001015153_late_invites_and_request_sources.go:init": 95,
	// ADR 0087.
	"database/migrations/001015154_repair_student_data_change_requests.go:init": 90,
	// ADR 0087.
	"database/migrations/001015155_parent_messaging_requests.go:parentMessagingRequestsDown": 69,
	// ADR 0087.
	"database/migrations/001015159_staff_shifts.go:staffShiftsUp": 73,
	// ADR 0087.
	"database/migrations/001015161_parent_announcements.go:parentAnnouncementsUp": 168,
	// ADR 0087.
	"database/migrations/001015164_care_schedule_change_requests.go:careScheduleChangeRequestsUp": 62,
	// ADR 0087.
	"database/migrations/001015165_migrate_open_care_requests.go:migrateOpenCareRequestsUp": 123,
	// ADR 0087.
	"database/migrations/001015166_form_schemas_migrate_legacy_departure_fields.go:convertLegacyDepartureFields": 100,
	// ADR 0087.
	"database/migrations/001015166_form_schemas_migrate_legacy_departure_fields.go:formSchemasMigrateLegacyDepartureUp": 113,
	// ADR 0087.
	"database/migrations/001015168_add_is_system_flags.go:init": 83,
	// ADR 0087.
	"database/migrations/001015170_shift_types.go:shiftTypesUp": 76,
	// ADR 0087.
	"database/migrations/001015173_personal_calendar.go:personalCalendarUp": 326,
	// ADR 0087.
	"database/migrations/001015175_create_display_displays.go:createDisplayDisplaysUp": 62,
	// ADR 0087.
	"database/migrations/001015180_enrollment_notification_mode_and_cleanup_grants.go:backfillEnrollmentDecisionNotificationModes": 70,
	// ADR 0087.
	"database/migrations/001015190_staff_shift_series.go:staffShiftSeriesUp": 170,
	// ADR 0087.
	"database/migrations/001015192_staff_shift_flexible_changes.go:init": 137,
	// ADR 0087.
	"database/migrations/001015193_deviation_events_audit.go:init": 101,
	// ADR 0087.
	"database/migrations/001015199_attendance_per_care_slot.go:attendancePerCareSlotUp": 95,
	// ADR 0087.
	"database/migrations/001015200_enrollment_deletions_audit.go:init": 65,
	// ADR 0087.
	"database/migrations/001015203_staff_schedule_rotation_anchor.go:init": 73,
	// ADR 0087.
	"database/migrations/001015204_refresh_token_rotation_recovery.go:init": 71,
	// ADR 0087.
	"database/migrations/001015209_student_companions.go:studentCompanionsUp": 66,
	// ADR 0087.
	"database/migrations/001015222_staff_month_balance_snapshots.go:staffMonthBalanceSnapshotsUp": 95,
	// ADR 0087.
	"database/migrations/001015225_audit_append_only_grants.go:auditAppendOnlyGrantsUp": 136,
	// ADR 0087.
	"database/migrations/001015234_push_subscriptions.go:ensurePushSubscriptions": 63,
	// ADR 0087.
	"database/migrations/001015236_grade_transition_roster_removals.go:gradeTransitionRosterRemovalsUp": 70,
	// ADR 0087.
	"database/migrations/001015241_parent_suggestions.go:upParentSuggestions": 75,
	// ADR 0087.
	"database/migrations/001015249_parent_announcement_polls.go:parentAnnouncementPollsUp": 147,
	// ADR 0087.
	"database/migrations/001015251_staff_stammdaten.go:staffStammdatenUp": 176,
	// ADR 0087.
	"database/migrations/001015255_appointment_reminder_push_delivery_access.go:appointmentReminderPushDeliveryAccessUp": 68,
	// ADR 0087.
	"database/migrations/001015257_staff_documents.go:staffDocumentsUp": 112,
	// ADR 0087.
	"database/migrations/001015258_activity_category_archival.go:activityCategoryArchivalUp": 104,
	// ADR 0087.
	"database/migrations/001015261_opening_balances.go:openingBalancesUp": 76,
	// ADR 0087.
	"database/migrations/001015262_roster_weekday_scope.go:rosterWeekdayScopeDown": 99,
	// ADR 0087.
	"database/migrations/001015262_roster_weekday_scope.go:rosterWeekdayScopeUp": 93,
	// ADR 0087.
	"database/migrations/001015265_group_targets.go:groupTargetsUp": 84,
	// ADR 0087.
	"database/migrations/001015266_template_offering_source.go:templateOfferingSourceUp": 82,
	// ADR 0087.
	"database/migrations/001015271_student_documents.go:studentDocumentsUp": 111,
	// ADR 0087.
	"database/migrations/001015281_template_multi_offering_source.go:templateMultiOfferingSourceDown": 74,
	// ADR 0087.
	"database/migrations/001015281_template_multi_offering_source.go:templateMultiOfferingSourceUp": 89,
	// ADR 0087.
	"database/migrations/001015282_mfa_challenge_portal_binding.go:mfaChallengePortalBindingUp": 65,
	// ADR 0087.
	"database/migrations/001015285_wissingen_departure_modes.go:wissingenDepartureModesUp": 191,
	// ADR 0087.
	"database/migrations/001015293_repair_school_identities.go:repairSchoolIdentitiesUp": 152,
	// ADR 0087.
	"database/migrations/001015299_auto_partial_absence_from_pickup.go:autoPartialAbsenceDown": 81,
	// ADR 0087.
	"database/migrations/001015299_auto_partial_absence_from_pickup.go:autoPartialAbsenceUp": 81,
	// ADR 0087.
	"database/migrations/001015304_pwa_standalone_usage.go:ensurePWAStandaloneUsage": 61,
	// ADR 0087.
	"database/migrations/001015306_class_list_entries.go:classListEntriesUp": 102,
	// ADR 0087.
	"database/migrations/001015312_staff_absence_types.go:staffAbsenceTypesUp": 84,
	// ADR 0087.
	"database/migrations/001015314_template_source_school_classes.go:templateSourceSchoolClassesDownAt": 87,
	// ADR 0087.
	"database/migrations/001015314_template_source_school_classes.go:templateSourceSchoolClassesUp": 66,
	// ADR 0087.
	"database/migrations/001015318_backfill_class_arrival_times.go:backfillClassArrivalTimesUp": 77,
	// ADR 0087.
	"database/migrations/001015319_student_care_exits.go:studentCareExitsUp": 72,
	// ADR 0087.
	"database/migrations/001015320_student_care_exit_removals.go:studentCareExitRemovalsUp": 79,
	// ADR 0087.
	"database/migrations/001015324_care_withdrawal_completions.go:careWithdrawalCompletionsUp": 81,
	// ADR 0087.
	"database/migrations/001015332_file_storage.go:fileStorageUp": 167,
	// ADR 0087.
	"database/migrations/001015335_staff_messaging.go:staffMessagingUp": 179,
	// ADR 0087.
	"database/migrations/001015336_guardian_payment_data.go:guardianPaymentDataUp": 77,
	// ADR 0087.
	"database/migrations/001015338_parent_letter_delivery.go:parentLetterDeliveryUp": 176,
	// ADR 0087.
	"database/migrations/001015342_staff_calendar_feed_tombstones.go:staffCalendarFeedTombstonesUp": 156,
	// ADR 0087.
	"database/migrations/001015352_staff_notices.go:staffNoticesUp": 86,
	// ADR 0087.
	"database/migrations/001015356_custom_leave_allowances.go:customLeaveAllowancesUp": 67,
	// ADR 0087.
	"database/migrations/001015358_audit_command_views.go:auditCommandViewsUp": 110,
	// ADR 0087.
	"database/migrations/001015359_announcement_attachments.go:announcementAttachmentsUp": 155,
	// ADR 0087.
	"database/migrations/001015360_leased_delivery_outbox.go:leasedDeliveryOutboxUp": 196,
	// ADR 0087.
	"database/migrations/001015363_parent_student_consents.go:parentStudentConsentsUp": 88,
	// ADR 0087.
	"database/migrations/001015365_meal_participation.go:mealParticipationUp": 200,
	// ADR 0087.
	"database/migrations/001015368_home_layouts.go:ensureHomeLayouts": 82,
	// ADR 0087.
	"database/migrations/001015376_staff_owner_storage.go:staffOwnerStorageUp": 62,
	// ADR 0087.
	"database/migrations/001015378_presence_expand.go:presenceExpandUp": 92,
	// ADR 0087.
	"database/migrations/001015382_staff_owner_backfill.go:staffOwnerBackfillUp": 61,
	// ADR 0087.
	"database/migrations/001015386_parent_announcement_reminders.go:parentAnnouncementRemindersUp": 79,
	// ADR 0087.
	"database/migrations/001015387_guardian_storage_expand.go:guardianStorageExpandUp": 93,
	// ADR 0087.
	"database/migrations/001015393_student_owner_storage_expand.go:studentOwnerStorageExpandUp": 130,
	// ADR 0087.
	"database/migrations/001015394_student_owner_backfill.go:studentOwnerBackfillUp": 76,
	// ADR 0087.
	"database/migrations/001015395_pickup_extension_tasks.go:pickupExtensionTasksUp": 73,
	// ADR 0087.
	"database/migrations/001015413_guardian_owner_backfill.go:guardianOwnerBackfillUp": 72,
	// ADR 0087.
	"database/migrations/001015419_staff_target_overrides.go:staffTargetOverridesUp": 61,
	// ADR 0087.
	"database/migrations/001015426_school_setups.go:schoolSetupsUp": 80,
	// ADR 0087.
	"database/migrations/001015428_parent_declarations.go:parentDeclarationsUp": 160,
	// ADR 0087.
	"database/migrations/001015434_parent_message_count_scope.go:parentMessageCountScopeUp": 64,
	// ADR 0087.
	"database/migrations/001015438_student_notes.go:studentNotesUp": 119,
	// ADR 0087.
	"database/migrations/001015443_student_departure_exceptions.go:studentDepartureExceptionsUp": 74,
	// ADR 0087.
	"database/migrations/00_migrations.go:DetectVersionCollisions": 76,
	// ADR 0087.
	"database/migrations/00_migrations.go:ValidateMigrations": 61,
	// ADR 0087.
	"database/migrations/guardian_owner_backfill.go:(*guardianOwnerTenantRun).copyBatch": 75,
	// ADR 0087.
	"database/migrations/guardian_owner_backfill.go:(*guardianOwnerTenantRun).tryCopyBatch": 74,
	// ADR 0087.
	"database/migrations/guardian_owner_backfill_verify.go:(*guardianOwnerTenantRun).verifySnapshot": 78,
	// ADR 0087.
	"database/migrations/guardian_owner_contract.go:contractGuardianOwnerStorageChecked": 65,
	// ADR 0087.
	"database/migrations/guardian_owner_contract.go:guardianOwnerContractDataPreflight": 91,
	// ADR 0087.
	"database/migrations/presence_backfill_verify.go:verifyPresenceBackfill": 77,
	// ADR 0087.
	"database/migrations/presence_contract.go:contractPresenceStorageChecked": 85,
	// ADR 0087.
	"database/migrations/presence_contract.go:presenceContractDataPreflight": 99,
	// ADR 0087.
	"database/migrations/request_child_storage_backfill.go:backfillRequestChildStorageTenant": 65,
	// ADR 0087.
	"database/migrations/request_child_storage_backfill.go:reconcileRequestChildStorageFinalDelta": 82,
	// ADR 0087.
	"database/migrations/request_child_storage_compatibility.go:installRequestChildStorageRouting": 72,
	// ADR 0087.
	"database/migrations/request_child_storage_compatibility.go:refreshRequestChildStorageCompatibility": 62,
	// ADR 0087.
	"database/migrations/request_child_storage_contract.go:requestChildStorageContractDataPreflight": 66,
	// ADR 0087.
	"database/migrations/reset.go:resetDatabase": 99,
	// ADR 0087.
	"database/migrations/staff_owner_backfill.go:(*staffOwnerTenantRun).copyBatch": 75,
	// ADR 0087.
	"database/migrations/staff_owner_backfill.go:(*staffOwnerTenantRun).tryCopyBatch": 74,
	// ADR 0087.
	"database/migrations/staff_owner_backfill.go:(*staffOwnerTenantRun).verifySnapshot": 72,
	// ADR 0087.
	"database/migrations/staff_owner_compatibility.go:ValidateStaffOwnerForeignKeys": 78,
	// ADR 0087.
	"database/migrations/staff_owner_compatibility.go:repointStaffForeignKeys": 213,
	// ADR 0087.
	"database/migrations/staff_owner_contract.go:staffOwnerContractDataPreflight": 76,
	// ADR 0087.
	"database/migrations/student_owner_backfill.go:(*studentOwnerTenantRun).copyBatch": 75,
	// ADR 0087.
	"database/migrations/student_owner_backfill.go:(*studentOwnerTenantRun).tryCopyBatch": 74,
	// ADR 0087.
	"database/migrations/student_owner_backfill_verify.go:(*studentOwnerTenantRun).verifySnapshot": 85,
	// ADR 0087.
	"database/migrations/student_owner_compatibility.go:ValidateStudentOwnerForeignKeys": 68,
	// ADR 0087.
	"database/migrations/student_owner_compatibility.go:repointStudentForeignKeys": 176,
	// ADR 0087.
	"database/migrations/student_owner_contract.go:studentOwnerContractDataPreflight": 65,
	// ADR 0087.
	"internal/architecture/composition_owners.go:rootStructInventoryWith": 89,
	// ADR 0087.
	"internal/architecture/composition_ratchet.go:CompareCompositionSurface": 93,
	// ADR 0087.
	"internal/architecture/cycles.go:cyclicComponents": 62,
	// ADR 0087.
	"internal/architecture/import_debt.go:basePackageImports": 74,
	// ADR 0087.
	"internal/architecture/migration_sql.go:migrationSQLBodyDataObjects": 113,
	// ADR 0087.
	"internal/architecture/migration_sql.go:migrationSQLTokens": 69,
	// ADR 0087.
	"internal/architecture/owner_http_rule.go:reviewedOwnerHTTPRule": 83,
	// ADR 0087.
	"internal/architecture/ratchet_budgets.go:CompareRatchetBudgets": 63,
	// ADR 0087.
	"internal/architecture/ratchet_budgets.go:compareRatchetBudgets": 135,
	// ADR 0087.
	"internal/devtools/seedapi/enrollment_bookings_profile.go:(seedEnrollmentBookingsProfileStep).Run": 71,
	// ADR 0087.
	"internal/devtools/seedapi/enrollment_weekly_profile.go:linkDeveloperAdmin": 80,
	// ADR 0087.
	"internal/devtools/seedapi/enrollment_weekly_profile.go:seedWeeklyProfileContents": 87,
	// ADR 0087.
	"internal/devtools/seedapi/enrollment_weekly_profile.go:submitWeeklyProfileRequest": 62,
	// ADR 0087.
	"internal/devtools/seedapi/enrollment_weekly_profile.go:verifyEnrollmentProfileCare": 62,
	// ADR 0087.
	"internal/devtools/seedapi/marketing_daily_life.go:seedMarketingSessions": 62,
	// ADR 0087.
	"internal/devtools/seedapi/parent_enrollment.go:(parentEnrollmentSeedStep).seedEnrollment": 320,
	// ADR 0087.
	"internal/devtools/seedapi/parent_enrollment.go:(parentEnrollmentSeedStep).seedParentAccounts": 62,
	// ADR 0087.
	"internal/devtools/seedapi/parent_enrollment.go:(parentEnrollmentSeedStep).seedParentPortalActions": 76,
	// ADR 0087.
	"internal/devtools/seedapi/seed_extras.go:(seedCareExitsStep).Run": 63,
	// ADR 0087.
	"internal/devtools/seedapi/seed_extras.go:(seedCourseParticipationStep).Run": 63,
	// ADR 0087.
	"internal/devtools/seedapi/seed_phase_periods.go:seedPhaseSchoolYear": 69,
	// ADR 0087.
	"internal/devtools/seedapi/seed_staff_messaging.go:(seedStaffMessagingStep).Run": 107,
	// ADR 0087.
	"internal/devtools/seedapi/seed_staff_notices.go:(seedStaffNoticesStep).Run": 65,
	// ADR 0087.
	"internal/devtools/seedapi/seed_student_notes.go:(seedStudentNotesStep).Run": 88,
	// ADR 0087.
	"internal/devtools/seedapi/seed_time_tracking.go:(seedTimeTrackingHistoryStep).Run": 160,
	// ADR 0087.
	"internal/devtools/seedapi/seed_time_tracking.go:(timeTrackingHistoryPlan).seedStaff": 71,
	// ADR 0087.
	"internal/devtools/seedapi/seeder.go:(*Seeder).printSuccessSummary": 70,
	// ADR 0087.
	"internal/devtools/seedapi/stammdaten.go:(*FixedSeeder).Seed": 119,
	// ADR 0087.
	"internal/devtools/seedapi/stammdaten.go:(*FixedSeeder).seedActivities": 64,
	// ADR 0087.
	"internal/devtools/seedapi/stammdaten.go:(*FixedSeeder).seedGroups": 65,
	// ADR 0087.
	"internal/devtools/seedapi/stammdaten.go:(*FixedSeeder).seedGuardians": 89,
	// ADR 0087.
	"internal/devtools/seedapi/stammdaten.go:(*FixedSeeder).seedPickupSchedules": 123,
	// ADR 0087.
	"internal/devtools/seedapi/stammdaten.go:(*FixedSeeder).seedStaffAccounts": 129,
	// ADR 0087.
	"internal/devtools/seedapi/stammdaten.go:(*FixedSeeder).seedStudents": 119,
	// ADR 0087.
	"internal/devtools/seedapi/workflow_steps.go:fullDemoWorkflow": 78,
	// ADR 0087.
	"internal/devtools/simulate/demo.go:(*DemoTicker).Tick": 84,
	// ADR 0087.
	"internal/devtools/simulate/demo_day.go:(*demoDay).move": 66,
	// ADR 0087.
	"internal/devtools/simulate/demo_day.go:(*demoDay).sync": 64,
	// ADR 0087.
	"internal/devtools/simulate/fullday.go:(seedStaffFeedTombstoneAction).Run": 73,
	// ADR 0087.
	"internal/devtools/simulate/live.go:RunLive": 101,
	// ADR 0087.
	"internal/devtools/simulate/live.go:runLiveTick": 74,
	// ADR 0087.
	"internal/devtools/simulate/status.go:RunStatus": 61,
	// ADR 0087.
	"internal/testdb/idle_unix.go:HoldServer": 93,
	// ADR 0087.
	"observability/prometheus.go:init": 143,
	// ADR 0087.
	"realtime/hub.go:(*Hub).Unregister": 67,
	"roots/owners/parent_messaging_announcement_access_port.go:BindParentAnnouncementAccess": 125,
	"roots/owners/root_identity.go:BuildIdentityAccess":                                      106,
	"roots/owners/root_worker.go:ComposeWorker":                                              358,
	"roots/owners/worker.go:NewWorker":                                                       91,
	// ADR 0087.
	"seed/avatar/avatar.go:tracePath": 64,
	// ADR 0087.
	"test/booking_consistency_audit_fixture.go:VerifyBookingConsistencyAuditIgnoresRuntimeFilteredPlanningRows": 72,
	// ADR 0087.
	"test/fixtures.go:CreateTestCoGuardianForStudent": 61,
	// ADR 0087.
	"test/fixtures.go:CreateTestParentGuardianChain": 77,
	// ADR 0087.
	"test/fixtures.go:cleanupTenantTestData": 107,
	// ADR 0087.
	"test/runtime_checkpoint.go:MeasureDeliveryCheckpoint": 94,
	// ADR 0087.
	"test/runtime_checkpoint.go:MeasureTimetableCheckpoint": 67,
}

func TestModuleFunctionLengthRatchet(t *testing.T) {
	t.Parallel()

	backendRoot, err := findBackendRoot()
	if err != nil {
		t.Skipf("Could not find backend root: %v", err)
		return
	}

	lengths, err := moduleFuncLenScan(backendRoot)
	if err != nil {
		t.Fatalf("function-length scan failed: %v", err)
	}

	violations := moduleFuncLenViolations(lengths, moduleFuncLenAllowlist)
	if len(violations) > 0 {
		sort.Strings(violations)
		t.Errorf("Module function-length ratchet check failed (%d issue(s)):\n\n%s",
			len(violations), strings.Join(violations, "\n\n"))
	}
}

// moduleFuncLenViolations compares per-function body lengths over the
// threshold against the allowlist. Like the complexity ratchet it cannot reuse
// ratchetViolations: entries are capped lengths rather than hit counts, and an
// allowlisted function falling to or below the threshold leaves the scan
// entirely, which is the ratchet-down signal rather than a deleted file.
func moduleFuncLenViolations(lengths, allowlist map[string]int) []string {
	var violations []string
	for fn, got := range lengths {
		allowed, ok := allowlist[fn]
		switch {
		case !ok:
			violations = append(violations, fmt.Sprintf(
				"[func-length] %s has a %d-line body (limit %d), and is not in the allowlist.\n  Split the function into named steps — do not add new entries.",
				fn, got, moduleFuncLenThreshold))
		case got > allowed:
			violations = append(violations, fmt.Sprintf(
				"[func-length] %s has a %d-line body, allowed %d.\n  The function grew. Extract a step — never raise the allowlist.", fn, got, allowed))
		case got < allowed:
			violations = append(violations, fmt.Sprintf(
				"[func-length] %s has a %d-line body, allowlist says %d.\n  Nice — ratchet the entry down to %d so the improvement cannot regress.", fn, got, allowed, got))
		}
	}
	for fn, allowed := range allowlist {
		if _, ok := lengths[fn]; !ok {
			violations = append(violations, fmt.Sprintf(
				"[func-length] %s is allowlisted at %d lines but is now ≤ %d (or was deleted/renamed).\n  Remove the entry.",
				fn, allowed, moduleFuncLenThreshold))
		}
	}
	return violations
}

// All roots retain coverage and thresholds after a move (ADRs 0064, 0082).
// moduleFuncLenScan returns the body length of every named function over the
// threshold in the shared backend source scope, keyed
// "relpath:FuncName" with methods rendered like the gocognit CLI. Declarations
// without a body (assembly stubs, cgo) are skipped; function literals are not
// counted.
func moduleFuncLenScan(backendRoot string) (map[string]int, error) {
	lengths := make(map[string]int)
	err := walkModuleRatchetSources(backendRoot, func(path, rel string) error {
		fset, file, parseErr := parseGoSourceCached(path, nil)
		if parseErr != nil {
			return fmt.Errorf("parse %s: %w", rel, parseErr)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			length := fset.Position(fn.Body.Rbrace).Line - fset.Position(fn.Body.Lbrace).Line + 1
			if length > moduleFuncLenThreshold {
				lengths[rel+":"+moduleFuncLenDisplayName(fn)] = length
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return lengths, nil
}

// moduleFuncLenDisplayName renders a function name the way the gocognit CLI
// does: plain functions as "name", methods as "(recv).name" / "(*recv).name".
func moduleFuncLenDisplayName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}
	return "(" + moduleFuncLenTypeExpr(fn.Recv.List[0].Type) + ")." + fn.Name.Name
}

func moduleFuncLenTypeExpr(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return "*" + moduleFuncLenTypeExpr(e.X)
	case *ast.IndexExpr: // generic receiver
		return moduleFuncLenTypeExpr(e.X)
	case *ast.IndexListExpr:
		return moduleFuncLenTypeExpr(e.X)
	default:
		return fmt.Sprintf("%T", expr)
	}
}
