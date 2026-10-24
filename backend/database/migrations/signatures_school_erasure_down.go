package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

// restoreSchoolErasureWithoutSignatures restores the exact five function
// bodies installed before this migration added audit.signatures.
func restoreSchoolErasureWithoutSignatures(ctx context.Context, tx bun.Tx) error {
	if _, err := tx.ExecContext(ctx, signaturesSchoolErasurePlanDownSQL); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, signaturesSchoolErasurePreflightDownSQL); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, signaturesSchoolErasureDeleteDownSQL); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, signaturesSchoolErasurePreviewDownSQL); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, signaturesSchoolErasureResidualDownSQL); err != nil {
		return err
	}
	return nil
}

const signaturesSchoolErasurePlanDownSQL = `
CREATE OR REPLACE FUNCTION platform.school_erasure_plan()
 RETURNS JSONB LANGUAGE sql IMMUTABLE SET search_path = pg_catalog
 AS $plan$ SELECT $manifest$[
  {"name": "active.activity_session_attendance","predicate": "r.tenant_id = $1"},
  {"name": "active.activity_sessions","predicate": "r.tenant_id = $1"},
  {"name": "active.attendance","predicate": "r.tenant_id = $1"},
  {"name": "active.combined_groups","predicate": "r.tenant_id = $1"},
  {"name": "active.excused_absence_requests","predicate": "r.tenant_id = $1"},
  {"name": "active.group_mappings","predicate": "r.tenant_id = $1"},
  {"name": "active.group_supervisors","predicate": "r.tenant_id = $1"},
  {"name": "active.groups","predicate": "r.tenant_id = $1"},
  {"name": "active.presence_backfill_batches","predicate": "r.tenant_id = $1"},
  {"name": "active.presence_backfill_checkpoints","predicate": "r.tenant_id = $1"},
  {"name": "active.scheduled_checkouts","predicate": "r.tenant_id = $1"},
  {"name": "active.staff_absence_audit","predicate": "r.tenant_id = $1"},
  {"name": "active.staff_absence_type_allowance_changes","predicate": "r.tenant_id = $1"},
  {"name": "active.staff_absence_type_allowances","predicate": "r.tenant_id = $1"},
  {"name": "active.staff_absence_types","predicate": "r.tenant_id = $1"},
  {"name": "active.staff_absences","predicate": "r.tenant_id = $1"},
  {"name": "active.staff_balance_adjustments","predicate": "r.tenant_id = $1"},
  {"name": "active.staff_month_balance_snapshots","predicate": "r.tenant_id = $1"},
  {"name": "active.staff_vacation_openings","predicate": "r.tenant_id = $1"},
  {"name": "active.staff_vacation_quota","predicate": "r.tenant_id = $1"},
  {"name": "active.staff_vacation_quota_changes","predicate": "r.tenant_id = $1"},
  {"name": "active.student_status_days","predicate": "r.tenant_id = $1"},
  {"name": "active.visits","predicate": "r.tenant_id = $1"},
  {"name": "active.work_session_breaks","predicate": "r.tenant_id = $1"},
  {"name": "active.work_sessions","predicate": "r.tenant_id = $1"},
  {"name": "activities.categories","predicate": "r.tenant_id = $1"},
  {"name": "activities.group_targets","predicate": "r.tenant_id = $1"},
  {"name": "activities.groups","predicate": "r.tenant_id = $1"},
  {"name": "activities.schedules","predicate": "r.tenant_id = $1"},
  {"name": "activities.student_enrollments","predicate": "r.tenant_id = $1"},
  {"name": "activities.supervisors","predicate": "r.tenant_id = $1"},
  {"name": "audit.attendance_corrections","predicate": "r.tenant_id = $1"},
  {"name": "audit.auth_events","predicate": "r.tenant_id = $1"},
  {"name": "audit.class_list_entry_changes","predicate": "r.tenant_id = $1"},
  {"name": "audit.data_access_log","predicate": "r.tenant_id = $1"},
  {"name": "audit.data_deletions","predicate": "r.tenant_id = $1"},
  {"name": "audit.data_imports","predicate": "r.tenant_id = $1"},
  {"name": "audit.deviation_events","predicate": "r.tenant_id = $1"},
  {"name": "audit.enrollment_deletions","predicate": "r.tenant_id = $1"},
  {"name": "audit.enrollment_offering_adjustments","predicate": "r.tenant_id = $1"},
  {"name": "audit.enrollment_restorations","predicate": "r.tenant_id = $1"},
  {"name": "audit.export_transfers","predicate": "r.tenant_id = $1"},
  {"name": "audit.file_events","predicate": "r.tenant_id = $1"},
  {"name": "audit.guardian_changes","predicate": "r.tenant_id = $1"},
  {"name": "audit.guardian_financial_changes","predicate": "r.tenant_id = $1"},
  {"name": "audit.personnel_number_changes","predicate": "r.tenant_id = $1"},
  {"name": "audit.room_color_migration_backup","predicate": "r.tenant_id = $1"},
  {"name": "audit.staff_master_data_changes","predicate": "r.tenant_id = $1"},
  {"name": "audit.student_consent_changes","predicate": "r.tenant_id = $1"},
  {"name": "audit.student_deletions","predicate": "r.tenant_id = $1"},
  {"name": "audit.student_field_edits","predicate": "r.tenant_id = $1"},
  {"name": "audit.substitution_changes","predicate": "r.tenant_id = $1"},
  {"name": "audit.time_tracking_deletions","predicate": "r.tenant_id = $1"},
  {"name": "audit.unregistered_tag_scans","predicate": "r.tenant_id = $1"},
  {"name": "audit.wc_alias_migration_backup","predicate": "r.tenant_id = $1"},
  {"name": "audit.work_session_edits","predicate": "r.tenant_id = $1"},
  {"name": "auth.account_permissions","predicate": "r.tenant_id = $1"},
  {"name": "auth.account_roles","predicate": "r.tenant_id = $1"},
  {"name": "auth.account_tenants","predicate": "r.tenant_id = $1"},
  {"name": "auth.guardian_invitations","predicate": "r.tenant_id = $1"},
  {"name": "auth.guardian_student_access","predicate": "r.tenant_id = $1"},
  {"name": "auth.invitation_tokens","predicate": "r.tenant_id = $1"},
  {"name": "auth.mfa_email_challenges","predicate": "r.tenant_id = $1"},
  {"name": "auth.mfa_overrides","predicate": "r.tenant_id = $1"},
  {"name": "auth.mfa_trusted_devices","predicate": "r.tenant_id = $1"},
  {"name": "auth.parent_declaration_permission_grants","predicate": "r.tenant_id = $1"},
  {"name": "auth.passkey_sessions","predicate": "r.tenant_id = $1"},
  {"name": "auth.role_permissions","predicate": "r.role_id IN (SELECT p0.id FROM auth.roles AS p0 WHERE p0.tenant_id = $1)","keyColumn": "role_id"},
  {"name": "auth.roles","predicate": "r.tenant_id = $1"},
  {"name": "auth.tokens","predicate": "r.tenant_id = $1"},
  {"name": "calendar.appointment_occurrence_overrides","predicate": "r.tenant_id = $1"},
  {"name": "calendar.appointment_recipient_students","predicate": "r.tenant_id = $1"},
  {"name": "calendar.appointment_recipients","predicate": "r.tenant_id = $1"},
  {"name": "calendar.appointment_reminder_push_deliveries","predicate": "r.tenant_id = $1"},
  {"name": "calendar.appointment_targets","predicate": "r.tenant_id = $1"},
  {"name": "calendar.appointments","predicate": "r.tenant_id = $1"},
  {"name": "calendar.recurrence_rules","predicate": "r.tenant_id = $1"},
  {"name": "calendar.staff_feed_tombstones","predicate": "r.tenant_id = $1","after": ["schedule.activity_instances","schedule.instance_staff","schedule.staff_shifts"]},
  {"name": "config.home_block_policies","predicate": "r.tenant_id = $1"},
  {"name": "config.home_layouts","predicate": "r.tenant_id = $1"},
  {"name": "config.school_setup_dismissals","predicate": "r.tenant_id = $1"},
  {"name": "config.school_setups","predicate": "r.tenant_id = $1"},
  {"name": "config.setting_audit","predicate": "r.tenant_id = $1"},
  {"name": "config.setting_values","predicate": "r.tenant_id = $1"},
  {"name": "config.staff_onboardings","predicate": "r.tenant_id = $1"},
  {"name": "config.staff_target_overrides","predicate": "r.tenant_id = $1"},
  {"name": "config.staff_work_schedules","predicate": "r.tenant_id = $1"},
  {"name": "config.work_time_model_entries","predicate": "r.model_id IN (SELECT p0.id FROM config.work_time_models AS p0 WHERE p0.tenant_id = $1)","keyColumn": "model_id"},
  {"name": "config.work_time_models","predicate": "r.tenant_id = $1"},
  {"name": "display.displays","predicate": "r.tenant_id = $1"},
  {"name": "documents.announcement_attachment_cleanup","predicate": "r.tenant_id = $1"},
  {"name": "documents.announcement_attachments","predicate": "r.tenant_id = $1"},
  {"name": "documents.file_cleanup","predicate": "r.tenant_id = $1"},
  {"name": "documents.files","predicate": "r.tenant_id = $1"},
  {"name": "documents.folder_accounts","predicate": "r.tenant_id = $1"},
  {"name": "documents.folder_roles","predicate": "r.tenant_id = $1"},
  {"name": "documents.folders","predicate": "r.tenant_id = $1"},
  {"name": "education.class_arrival_exceptions","predicate": "r.tenant_id = $1"},
  {"name": "education.class_arrival_times","predicate": "r.tenant_id = $1"},
  {"name": "education.class_teachers","predicate": "r.tenant_id = $1"},
  {"name": "education.grade_transition_class_list_entries","predicate": "r.tenant_id = $1"},
  {"name": "education.grade_transition_class_teachers","predicate": "r.tenant_id = $1"},
  {"name": "education.grade_transition_history","predicate": "r.tenant_id = $1"},
  {"name": "education.grade_transition_mappings","predicate": "r.tenant_id = $1"},
  {"name": "education.grade_transitions","predicate": "r.tenant_id = $1"},
  {"name": "education.group_substitution","predicate": "r.tenant_id = $1"},
  {"name": "education.group_teacher","predicate": "r.tenant_id = $1"},
  {"name": "education.groups","predicate": "r.tenant_id = $1"},
  {"name": "enrollment.care_offering_auto_triggers","predicate": "r.tenant_id = $1"},
  {"name": "enrollment.care_offering_bookings","predicate": "r.tenant_id = $1"},
  {"name": "enrollment.care_offerings","predicate": "r.tenant_id = $1"},
  {"name": "enrollment.change_request_messages","predicate": "r.tenant_id = $1"},
  {"name": "enrollment.change_requests","predicate": "r.tenant_id = $1"},
  {"name": "enrollment.form_schemas","predicate": "r.tenant_id = $1"},
  {"name": "enrollment.late_invites","predicate": "r.tenant_id = $1"},
  {"name": "enrollment.offering_change_requests","predicate": "r.tenant_id = $1"},
  {"name": "enrollment.phases","predicate": "r.tenant_id = $1"},
  {"name": "enrollment.request_child_offering_selections","predicate": "r.tenant_id = $1"},
  {"name": "enrollment.request_child_storage_backfill_checkpoints","predicate": "r.tenant_id = $1"},
  {"name": "enrollment.request_children","predicate": "r.tenant_id = $1"},
  {"name": "enrollment.request_guardians","predicate": "r.tenant_id = $1"},
  {"name": "enrollment.request_reads","predicate": "r.tenant_id = $1"},
  {"name": "enrollment.request_upload_cleanup","predicate": "r.tenant_id = $1","after": ["enrollment.request_uploads"]},
  {"name": "enrollment.request_uploads","predicate": "r.tenant_id = $1"},
  {"name": "enrollment.requests","predicate": "r.tenant_id = $1"},
  {"name": "enrollment.submission_rate_limits","predicate": "r.tenant_id = $1"},
  {"name": "facilities.rooms","predicate": "r.tenant_id = $1"},
  {"name": "feedback.entries","predicate": "r.tenant_id = $1"},
  {"name": "iot.devices","predicate": "r.tenant_id = $1"},
  {"name": "iot.push_subscriptions","predicate": "r.tenant_id = $1"},
  {"name": "iot.pwa_standalone_usage","predicate": "r.tenant_id = $1"},
  {"name": "meta.meal_participation_permission_grants","predicate": "r.student_guardian_id IN (SELECT p0.id FROM users.student_guardian_relationships AS p0 WHERE p0.tenant_id = $1)","keyColumn": "student_guardian_id"},
  {"name": "meta.parent_student_consent_permission_grants","predicate": "r.student_guardian_id IN (SELECT p0.id FROM users.student_guardian_relationships AS p0 WHERE p0.tenant_id = $1)","keyColumn": "student_guardian_id"},
  {"name": "platform.cleanup_runs","predicate": "r.tenant_id = $1"},
  {"name": "platform.demo_school_states","predicate": "r.tenant_id = $1"},
  {"name": "platform.email_delivery","predicate": "r.tenant_id = $1"},
  {"name": "platform.email_outbox","predicate": "r.tenant_id = $1"},
  {"name": "platform.push_outbox","predicate": "r.tenant_id = $1"},
  {"name": "platform.schools","predicate": "r.id = $1"},
  {"name": "platform.storage_backfill_checkpoints","predicate": "r.tenant_id = $1"},
  {"name": "schedule.activity_exceptions","predicate": "r.tenant_id = $1"},
  {"name": "schedule.activity_instances","predicate": "r.tenant_id = $1"},
  {"name": "schedule.calendar_periods","predicate": "r.tenant_id = $1"},
  {"name": "schedule.care_schedule_change_requests","predicate": "r.tenant_id = $1"},
  {"name": "schedule.closing_days","predicate": "r.tenant_id = $1"},
  {"name": "schedule.dateframes","predicate": "r.tenant_id = $1"},
  {"name": "schedule.grade_transition_roster_removals","predicate": "r.tenant_id = $1"},
  {"name": "schedule.instance_staff","predicate": "r.tenant_id = $1"},
  {"name": "schedule.instance_students","predicate": "r.tenant_id = $1"},
  {"name": "schedule.meal_participation_overrides","predicate": "r.tenant_id = $1"},
  {"name": "schedule.meal_participation_schedules","predicate": "r.tenant_id = $1"},
  {"name": "schedule.meal_plan_entries","predicate": "r.tenant_id = $1"},
  {"name": "schedule.meal_sickness_status_history","predicate": "r.tenant_id = $1","after": ["active.student_status_days"]},
  {"name": "schedule.pickup_extension_tasks","predicate": "r.tenant_id = $1"},
  {"name": "schedule.planning_tracks","predicate": "r.tenant_id = $1"},
  {"name": "schedule.recurrence_rules","predicate": "r.tenant_id = $1"},
  {"name": "schedule.shift_types","predicate": "r.tenant_id = $1"},
  {"name": "schedule.staff_shift_series","predicate": "r.tenant_id = $1"},
  {"name": "schedule.staff_shift_series_exceptions","predicate": "r.tenant_id = $1"},
  {"name": "schedule.staff_shifts","predicate": "r.tenant_id = $1"},
  {"name": "schedule.student_arrival_exceptions","predicate": "r.tenant_id = $1"},
  {"name": "schedule.student_arrival_notes","predicate": "r.tenant_id = $1"},
  {"name": "schedule.student_arrival_schedules","predicate": "r.tenant_id = $1"},
  {"name": "schedule.student_departure_exceptions","predicate": "r.tenant_id = $1"},
  {"name": "schedule.student_pickup_exceptions","predicate": "r.tenant_id = $1"},
  {"name": "schedule.student_pickup_notes","predicate": "r.tenant_id = $1"},
  {"name": "schedule.student_pickup_schedules","predicate": "r.tenant_id = $1"},
  {"name": "schedule.timeframes","predicate": "r.tenant_id = $1"},
  {"name": "schedule.timetable_conflict_acks","predicate": "r.tenant_id = $1"},
  {"name": "users.care_withdrawal_completions","predicate": "r.tenant_id = $1"},
  {"name": "users.class_list_entries","predicate": "r.tenant_id = $1"},
  {"name": "users.external_caregivers","predicate": "r.tenant_id = $1"},
  {"name": "users.guardian_financial_data","predicate": "r.tenant_id = $1"},
  {"name": "users.guardian_phone_numbers","predicate": "r.tenant_id = $1"},
  {"name": "users.guardian_profiles","predicate": "r.tenant_id = $1"},
  {"name": "users.notification_preferences","predicate": "r.tenant_id = $1"},
  {"name": "users.parent_announcement_declaration_submissions","predicate": "r.tenant_id = $1"},
  {"name": "users.parent_announcement_declaration_versions","predicate": "r.tenant_id = $1"},
  {"name": "users.parent_announcement_options","predicate": "r.tenant_id = $1"},
  {"name": "users.parent_announcement_reads","predicate": "r.tenant_id = $1"},
  {"name": "users.parent_announcement_responses","predicate": "r.tenant_id = $1"},
  {"name": "users.parent_announcement_targets","predicate": "r.tenant_id = $1"},
  {"name": "users.parent_announcements","predicate": "r.tenant_id = $1"},
  {"name": "users.parent_group_message_recipients","predicate": "r.tenant_id = $1"},
  {"name": "users.parent_group_messages","predicate": "r.tenant_id = $1"},
  {"name": "users.parent_group_thread_reads","predicate": "r.tenant_id = $1"},
  {"name": "users.parent_message_count_preferences","predicate": "r.tenant_id = $1"},
  {"name": "users.parent_message_reads","predicate": "r.tenant_id = $1"},
  {"name": "users.parent_message_threads","predicate": "r.tenant_id = $1"},
  {"name": "users.parent_messages","predicate": "r.tenant_id = $1"},
  {"name": "users.parent_group_threads","predicate": "r.tenant_id = $1"},
  {"name": "users.parent_request_events","predicate": "r.tenant_id = $1"},
  {"name": "users.parent_request_rss_feeds","predicate": "r.tenant_id = $1"},
  {"name": "users.parent_request_share_events","predicate": "r.tenant_id = $1"},
  {"name": "users.persons","predicate": "r.tenant_id = $1"},
  {"name": "users.privacy_consents","predicate": "r.tenant_id = $1"},
  {"name": "users.profiles","predicate": "r.tenant_id = $1"},
  {"name": "users.rfid_cards","predicate": "r.tenant_id = $1"},
  {"name": "users.staff_document_file_cleanup","predicate": "r.tenant_id = $1"},
  {"name": "users.staff_documents","predicate": "r.tenant_id = $1"},
  {"name": "users.staff_employment_profiles","predicate": "r.tenant_id = $1"},
  {"name": "users.staff_financial_data","predicate": "r.tenant_id = $1"},
  {"name": "users.staff_master_data","predicate": "r.tenant_id = $1"},
  {"name": "users.staff_message_participants","predicate": "r.tenant_id = $1"},
  {"name": "users.staff_message_reads","predicate": "r.tenant_id = $1"},
  {"name": "users.staff_message_threads","predicate": "r.tenant_id = $1"},
  {"name": "users.staff_messages","predicate": "r.tenant_id = $1"},
  {"name": "users.staff_notice_acks","predicate": "r.tenant_id = $1"},
  {"name": "users.staff_notices","predicate": "r.tenant_id = $1"},
  {"name": "users.staff_offboarding_cleanup","predicate": "r.tenant_id = $1"},
  {"name": "users.staff_qualifications","predicate": "r.tenant_id = $1"},
  {"name": "users.staff_school_memberships","predicate": "r.tenant_id = $1"},
  {"name": "users.student_care_exit_removals","predicate": "r.tenant_id = $1"},
  {"name": "users.student_care_exit_source_removals","predicate": "r.tenant_id = $1"},
  {"name": "users.student_care_exits","predicate": "r.tenant_id = $1"},
  {"name": "users.student_care_profiles","predicate": "r.tenant_id = $1"},
  {"name": "users.student_companions","predicate": "r.tenant_id = $1"},
  {"name": "users.student_data_change_requests","predicate": "r.tenant_id = $1"},
  {"name": "users.student_document_file_cleanup","predicate": "r.tenant_id = $1"},
  {"name": "users.student_documents","predicate": "r.tenant_id = $1"},
  {"name": "users.student_family_protection_events","predicate": "r.tenant_id = $1"},
  {"name": "users.student_guardian_pickup_permissions","predicate": "r.tenant_id = $1"},
  {"name": "users.student_guardian_relationships","predicate": "r.tenant_id = $1"},
  {"name": "users.student_notes","predicate": "r.tenant_id = $1"},
  {"name": "users.student_profiles","predicate": "r.tenant_id = $1"},
  {"name": "users.student_school_memberships","predicate": "r.tenant_id = $1"},
  {"name": "users.teachers","predicate": "r.tenant_id = $1"}
]$manifest$::jsonb $plan$;
 REVOKE ALL ON FUNCTION platform.school_erasure_plan() FROM PUBLIC, phoenix_auth, phoenix_tenant, phoenix_admin;
`

const signaturesSchoolErasurePreflightDownSQL = `
CREATE OR REPLACE FUNCTION platform.school_erasure_preflight(p_school_id BIGINT,p_organization_id BIGINT)
 RETURNS VOID LANGUAGE plpgsql VOLATILE SECURITY INVOKER SET search_path = pg_catalog
 AS $body$
 DECLARE v_plan JSONB := platform.school_erasure_plan();
 BEGIN
   -- Generated static lock set. New FK edges fail the signature check.
   LOCK TABLE
    active.activity_session_attendance,
    active.activity_sessions,
    active.attendance,
    active.combined_groups,
    active.excused_absence_requests,
    active.group_mappings,
    active.group_supervisors,
    active.groups,
    active.presence_backfill_batches,
    active.presence_backfill_checkpoints,
    active.scheduled_checkouts,
    active.staff_absence_audit,
    active.staff_absence_type_allowance_changes,
    active.staff_absence_type_allowances,
    active.staff_absence_types,
    active.staff_absences,
    active.staff_balance_adjustments,
    active.staff_month_balance_snapshots,
    active.staff_vacation_openings,
    active.staff_vacation_quota,
    active.staff_vacation_quota_changes,
    active.student_status_days,
    active.visits,
    active.work_session_breaks,
    active.work_sessions,
    activities.categories,
    activities.group_targets,
    activities.groups,
    activities.schedules,
    activities.student_enrollments,
    activities.supervisors,
    audit.attendance_corrections,
    audit.auth_events,
    audit.class_list_entry_changes,
    audit.data_access_log,
    audit.data_deletions,
    audit.data_imports,
    audit.deviation_events,
    audit.enrollment_deletions,
    audit.enrollment_offering_adjustments,
    audit.enrollment_restorations,
    audit.export_transfers,
    audit.file_events,
    audit.guardian_changes,
    audit.guardian_financial_changes,
    audit.personnel_number_changes,
    audit.room_color_migration_backup,
    audit.staff_master_data_changes,
    audit.student_consent_changes,
    audit.student_deletions,
    audit.student_field_edits,
    audit.substitution_changes,
    audit.time_tracking_deletions,
    audit.unregistered_tag_scans,
    audit.wc_alias_migration_backup,
    audit.work_session_edits,
    auth.account_permissions,
    auth.account_roles,
    auth.account_tenants,
    auth.guardian_invitations,
    auth.guardian_student_access,
    auth.invitation_tokens,
    auth.mfa_email_challenges,
    auth.mfa_overrides,
    auth.mfa_trusted_devices,
    auth.parent_declaration_permission_grants,
    auth.passkey_sessions,
    auth.role_permissions,
    auth.roles,
    auth.tokens,
    calendar.appointment_occurrence_overrides,
    calendar.appointment_recipient_students,
    calendar.appointment_recipients,
    calendar.appointment_reminder_push_deliveries,
    calendar.appointment_targets,
    calendar.appointments,
    calendar.recurrence_rules,
    calendar.staff_feed_tombstones,
    config.home_block_policies,
    config.home_layouts,
    config.school_setup_dismissals,
    config.school_setups,
    config.setting_audit,
    config.setting_values,
    config.staff_onboardings,
    config.staff_target_overrides,
    config.staff_work_schedules,
    config.work_time_model_entries,
    config.work_time_models,
    display.displays,
    documents.announcement_attachment_cleanup,
    documents.announcement_attachments,
    documents.file_cleanup,
    documents.files,
    documents.folder_accounts,
    documents.folder_roles,
    documents.folders,
    education.class_arrival_exceptions,
    education.class_arrival_times,
    education.class_teachers,
    education.grade_transition_class_list_entries,
    education.grade_transition_class_teachers,
    education.grade_transition_history,
    education.grade_transition_mappings,
    education.grade_transitions,
    education.group_substitution,
    education.group_teacher,
    education.groups,
    enrollment.care_offering_auto_triggers,
    enrollment.care_offering_bookings,
    enrollment.care_offerings,
    enrollment.change_request_messages,
    enrollment.change_requests,
    enrollment.form_schemas,
    enrollment.late_invites,
    enrollment.offering_change_requests,
    enrollment.phases,
    enrollment.request_child_offering_selections,
    enrollment.request_child_storage_backfill_checkpoints,
    enrollment.request_children,
    enrollment.request_guardians,
    enrollment.request_reads,
    enrollment.request_upload_cleanup,
    enrollment.request_uploads,
    enrollment.requests,
    enrollment.submission_rate_limits,
    facilities.rooms,
    feedback.entries,
    iot.devices,
    iot.push_subscriptions,
    iot.pwa_standalone_usage,
    meta.meal_participation_permission_grants,
    meta.parent_student_consent_permission_grants,
    platform.cleanup_runs,
    platform.demo_school_states,
    platform.email_delivery,
    platform.email_outbox,
    platform.push_outbox,
    platform.schools,
    platform.storage_backfill_checkpoints,
    schedule.activity_exceptions,
    schedule.activity_instances,
    schedule.calendar_periods,
    schedule.care_schedule_change_requests,
    schedule.closing_days,
    schedule.dateframes,
    schedule.grade_transition_roster_removals,
    schedule.instance_staff,
    schedule.instance_students,
    schedule.meal_participation_overrides,
    schedule.meal_participation_schedules,
    schedule.meal_plan_entries,
    schedule.meal_sickness_status_history,
    schedule.pickup_extension_tasks,
    schedule.planning_tracks,
    schedule.recurrence_rules,
    schedule.shift_types,
    schedule.staff_shift_series,
    schedule.staff_shift_series_exceptions,
    schedule.staff_shifts,
    schedule.student_arrival_exceptions,
    schedule.student_arrival_notes,
    schedule.student_arrival_schedules,
    schedule.student_departure_exceptions,
    schedule.student_pickup_exceptions,
    schedule.student_pickup_notes,
    schedule.student_pickup_schedules,
    schedule.timeframes,
    schedule.timetable_conflict_acks,
    users.care_withdrawal_completions,
    users.class_list_entries,
    users.external_caregivers,
    users.guardian_financial_data,
    users.guardian_phone_numbers,
    users.guardian_profiles,
    users.notification_preferences,
    users.parent_announcement_declaration_submissions,
    users.parent_announcement_declaration_versions,
    users.parent_announcement_options,
    users.parent_announcement_reads,
    users.parent_announcement_responses,
    users.parent_announcement_targets,
    users.parent_announcements,
    users.parent_group_message_recipients,
    users.parent_group_messages,
    users.parent_group_thread_reads,
    users.parent_group_threads,
    users.parent_message_count_preferences,
    users.parent_message_reads,
    users.parent_message_threads,
    users.parent_messages,
    users.parent_request_events,
    users.parent_request_rss_feeds,
    users.parent_request_share_events,
    users.persons,
    users.privacy_consents,
    users.profiles,
    users.rfid_cards,
    users.staff_document_file_cleanup,
    users.staff_documents,
    users.staff_employment_profiles,
    users.staff_financial_data,
    users.staff_master_data,
    users.staff_message_participants,
    users.staff_message_reads,
    users.staff_message_threads,
    users.staff_messages,
    users.staff_notice_acks,
    users.staff_notices,
    users.staff_offboarding_cleanup,
    users.staff_qualifications,
    users.staff_school_memberships,
    users.student_care_exit_removals,
    users.student_care_exit_source_removals,
    users.student_care_exits,
    users.student_care_profiles,
    users.student_companions,
    users.student_data_change_requests,
    users.student_document_file_cleanup,
    users.student_documents,
    users.student_family_protection_events,
    users.student_guardian_pickup_permissions,
    users.student_guardian_relationships,
    users.student_notes,
    users.student_profiles,
    users.student_school_memberships,
    users.teachers
   IN SHARE ROW EXCLUSIVE MODE;
   IF NOT EXISTS (SELECT FROM platform.schools WHERE id = p_school_id AND organization_id = p_organization_id) THEN
    RAISE EXCEPTION 'school root missing or changed' USING ERRCODE = '22023';
   END IF;
   -- Runtime fail-closed protection in addition to the catalog/schema CI guard.
   IF EXISTS (
    SELECT FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
    JOIN pg_attribute a ON a.attrelid = c.oid AND a.attname = 'tenant_id' AND NOT a.attisdropped
    WHERE c.relkind IN ('r','p') AND NOT c.relispartition AND n.nspname !~ '^pg_' AND n.nspname <> 'information_schema'
     AND n.nspname || '.' || c.relname NOT IN (SELECT item->>'name' FROM jsonb_array_elements(v_plan) AS item)
   ) THEN
    RAISE EXCEPTION 'school erasure plan does not cover current tenant schema' USING ERRCODE = '55000';
   END IF;

   -- Fingerprint every incoming FK, including nullable and cascading edges.
   -- A schema change must regenerate and review both order and scope checks.
   IF (SELECT encode(sha256(convert_to(COALESCE(string_agg(
     edge.child_table || '|' || edge.constraint_name || '|' || edge.parent_table || '|' || edge.action || '|' || edge.child_columns || '|' || edge.parent_columns,
     E'\n' ORDER BY edge.child_table COLLATE "C", edge.constraint_name COLLATE "C"), ''), 'UTF8')), 'hex')
    FROM (
     SELECT cn.nspname || '.' || cc.relname AS child_table, f.conname AS constraint_name,
      pn.nspname || '.' || pc.relname AS parent_table, f.confdeltype::text AS action,
      string_agg(ca.attname, ',' ORDER BY key.ordinality) AS child_columns,
      string_agg(pa.attname, ',' ORDER BY key.ordinality) AS parent_columns
     FROM pg_constraint f JOIN pg_class cc ON cc.oid=f.conrelid JOIN pg_namespace cn ON cn.oid=cc.relnamespace
     JOIN pg_class pc ON pc.oid=f.confrelid JOIN pg_namespace pn ON pn.oid=pc.relnamespace
     CROSS JOIN LATERAL unnest(f.conkey,f.confkey) WITH ORDINALITY AS key(child_col,parent_col,ordinality)
     JOIN pg_attribute ca ON ca.attrelid=cc.oid AND ca.attnum=key.child_col
     JOIN pg_attribute pa ON pa.attrelid=pc.oid AND pa.attnum=key.parent_col
     WHERE f.contype='f' AND pn.nspname || '.' || pc.relname IN
      (SELECT item->>'name' FROM jsonb_array_elements(v_plan) AS item)
     GROUP BY f.oid,cn.nspname,cc.relname,f.conname,pn.nspname,pc.relname,f.confdeltype
    ) AS edge) <> '35013a7fec9d1763a2712e0cb4ebd1084c916458d44d5a484f3b0f096414a683' THEN
    RAISE EXCEPTION 'school erasure foreign keys changed; review plan' USING ERRCODE = '55000';
   END IF;
   -- Generated incoming-FK scope checks. Never cascade into foreign rows.
   IF EXISTS (
    SELECT 1 FROM active.excused_absence_requests AS c JOIN users.student_profiles AS p ON c."student_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM active.staff_absence_audit AS c JOIN active.staff_absences AS p ON c."absence_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM active.staff_absences AS c JOIN users.staff_school_memberships AS p ON c."substitute_staff_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM active.staff_vacation_quota AS c JOIN users.staff_school_memberships AS p ON c."staff_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM active.student_status_days AS c JOIN users.student_profiles AS p ON c."student_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM active.visits AS c JOIN users.student_profiles AS p ON c."student_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM activities.groups AS c JOIN education.groups AS p ON c."education_group_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM activities.schedules AS c JOIN schedule.calendar_periods AS p ON c."calendar_period_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM activities.student_enrollments AS c JOIN enrollment.request_children AS p ON c."enrollment_request_child_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM activities.student_enrollments AS c JOIN schedule.calendar_periods AS p ON c."calendar_period_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM activities.supervisors AS c JOIN schedule.calendar_periods AS p ON c."calendar_period_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM audit.attendance_corrections AS c JOIN schedule.activity_instances AS p ON c."instance_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM audit.attendance_corrections AS c JOIN users.student_profiles AS p ON c."student_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM audit.data_access_log AS c JOIN users.student_profiles AS p ON c."student_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM audit.deviation_events AS c JOIN activities.groups AS p ON c."activity_group_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM audit.deviation_events AS c JOIN schedule.activity_instances AS p ON c."instance_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM audit.deviation_events AS c JOIN users.staff_school_memberships AS p ON c."related_staff_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM audit.deviation_events AS c JOIN schedule.staff_shifts AS p ON c."staff_shift_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM audit.deviation_events AS c JOIN users.staff_school_memberships AS p ON c."subject_staff_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM audit.enrollment_offering_adjustments AS c JOIN enrollment.request_children AS p ON c."request_child_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM audit.enrollment_offering_adjustments AS c JOIN enrollment.requests AS p ON c."request_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM audit.enrollment_offering_adjustments AS c JOIN users.student_profiles AS p ON c."student_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM audit.guardian_changes AS c JOIN users.guardian_profiles AS p ON c."guardian_profile_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM audit.guardian_changes AS c JOIN users.student_profiles AS p ON c."student_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM audit.student_consent_changes AS c JOIN users.student_profiles AS p ON c."student_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM audit.student_field_edits AS c JOIN users.student_profiles AS p ON c."student_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM audit.unregistered_tag_scans AS c JOIN iot.devices AS p ON c."device_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM auth.account_roles AS c JOIN auth.roles AS p ON c."role_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM auth.guardian_invitations AS c JOIN users.guardian_profiles AS p ON c."guardian_profile_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM auth.invitation_tokens AS c JOIN auth.roles AS p ON c."role_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM auth.invitation_tokens AS c JOIN users.persons AS p ON c."person_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM auth.role_permissions AS c JOIN auth.roles AS p ON c."role_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.role_id IN (SELECT p0.id FROM auth.roles AS p0 WHERE p0.tenant_id = p_school_id)), FALSE)
    UNION ALL
    SELECT 1 FROM calendar.appointment_recipient_students AS c JOIN users.student_profiles AS p ON c."student_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM calendar.appointment_recipients AS c JOIN users.guardian_profiles AS p ON c."guardian_profile_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM calendar.appointment_recipients AS c JOIN users.staff_school_memberships AS p ON c."staff_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM calendar.appointments AS c JOIN users.staff_school_memberships AS p ON c."organizer_staff_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM calendar.staff_feed_tombstones AS c JOIN users.staff_school_memberships AS p ON c."staff_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM config.staff_work_schedules AS c JOIN users.staff_school_memberships AS p ON c."staff_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM config.work_time_model_entries AS c JOIN config.work_time_models AS p ON c."model_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.model_id IN (SELECT p0.id FROM config.work_time_models AS p0 WHERE p0.tenant_id = p_school_id)), FALSE)
    UNION ALL
    SELECT 1 FROM documents.folder_roles AS c JOIN auth.roles AS p ON c."role_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM education.grade_transition_class_list_entries AS c JOIN education.grade_transitions AS p ON c."transition_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM education.grade_transition_class_teachers AS c JOIN education.grade_transitions AS p ON c."transition_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM enrollment.care_offering_auto_triggers AS c JOIN enrollment.care_offerings AS p ON c."target_care_offering_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM enrollment.care_offering_auto_triggers AS c JOIN enrollment.care_offerings AS p ON c."trigger_care_offering_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM enrollment.care_offerings AS c JOIN activities.groups AS p ON c."activity_group_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM enrollment.care_offerings AS c JOIN enrollment.phases AS p ON c."phase_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM enrollment.change_request_messages AS c JOIN enrollment.change_requests AS p ON c."change_request_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM enrollment.change_requests AS c JOIN enrollment.request_children AS p ON c."request_child_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM enrollment.change_requests AS c JOIN enrollment.requests AS p ON c."request_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM enrollment.late_invites AS c JOIN enrollment.phases AS p ON c."phase_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM enrollment.late_invites AS c JOIN enrollment.requests AS p ON c."used_request_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM enrollment.offering_change_requests AS c JOIN enrollment.request_children AS p ON c."request_child_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM enrollment.offering_change_requests AS c JOIN users.student_profiles AS p ON c."student_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM enrollment.phases AS c JOIN schedule.calendar_periods AS p ON c."calendar_period_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM enrollment.phases AS c JOIN enrollment.form_schemas AS p ON c."form_schema_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM enrollment.phases AS c JOIN enrollment.phases AS p ON c."rollover_source_phase_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM enrollment.request_children AS c JOIN users.student_profiles AS p ON c."created_student_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM enrollment.request_children AS c JOIN users.student_profiles AS p ON c."matched_student_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM enrollment.request_children AS c JOIN enrollment.requests AS p ON c."request_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM enrollment.request_children AS c JOIN enrollment.request_children AS p ON c."rollover_source_child_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM enrollment.request_guardians AS c JOIN users.guardian_profiles AS p ON c."guardian_profile_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM enrollment.request_guardians AS c JOIN enrollment.requests AS p ON c."request_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM enrollment.request_reads AS c JOIN enrollment.requests AS p ON c."request_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM enrollment.request_uploads AS c JOIN enrollment.phases AS p ON c."phase_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM enrollment.request_uploads AS c JOIN enrollment.requests AS p ON c."request_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM enrollment.requests AS c JOIN enrollment.phases AS p ON c."phase_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM enrollment.requests AS c JOIN enrollment.form_schemas AS p ON c."schema_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM iot.devices AS c JOIN iot.devices AS p ON c."transferred_to_device_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM meta.meal_participation_permission_grants AS c JOIN users.student_guardian_relationships AS p ON c."student_guardian_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.student_guardian_id IN (SELECT p0.id FROM users.student_guardian_relationships AS p0 WHERE p0.tenant_id = p_school_id)), FALSE)
    UNION ALL
    SELECT 1 FROM meta.parent_student_consent_permission_grants AS c JOIN users.student_guardian_relationships AS p ON c."student_guardian_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.student_guardian_id IN (SELECT p0.id FROM users.student_guardian_relationships AS p0 WHERE p0.tenant_id = p_school_id)), FALSE)
    UNION ALL
    SELECT 1 FROM platform.email_delivery AS c JOIN users.guardian_profiles AS p ON c."guardian_profile_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM platform.email_delivery AS c JOIN platform.email_outbox AS p ON c."outbox_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM schedule.activity_instances AS c JOIN schedule.calendar_periods AS p ON c."calendar_period_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM schedule.care_schedule_change_requests AS c JOIN users.student_profiles AS p ON c."student_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM schedule.student_arrival_exceptions AS c JOIN users.staff_school_memberships AS p ON c."created_by" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM schedule.student_arrival_exceptions AS c JOIN users.student_profiles AS p ON c."student_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM schedule.student_arrival_notes AS c JOIN users.staff_school_memberships AS p ON c."created_by" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM schedule.student_arrival_notes AS c JOIN users.student_profiles AS p ON c."student_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM schedule.student_arrival_schedules AS c JOIN users.staff_school_memberships AS p ON c."created_by" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM schedule.student_arrival_schedules AS c JOIN users.student_profiles AS p ON c."student_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM schedule.student_pickup_schedules AS c JOIN enrollment.care_offerings AS p ON c."care_offering_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM users.care_withdrawal_completions AS c JOIN audit.enrollment_offering_adjustments AS p ON c."source_adjustment_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM users.care_withdrawal_completions AS c JOIN enrollment.request_children AS p ON c."source_request_child_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM users.parent_announcement_options AS c JOIN users.parent_announcements AS p ON c."announcement_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM users.parent_announcement_reads AS c JOIN users.parent_announcements AS p ON c."announcement_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM users.parent_announcement_responses AS c JOIN users.parent_announcements AS p ON c."announcement_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM users.parent_announcement_responses AS c JOIN users.parent_announcement_options AS p ON c."option_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM users.parent_announcement_responses AS c JOIN users.student_profiles AS p ON c."student_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM users.parent_announcement_targets AS c JOIN users.parent_announcements AS p ON c."announcement_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM users.parent_group_message_recipients AS c JOIN users.parent_group_messages AS p ON c."message_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM users.parent_group_message_recipients AS c JOIN users.parent_group_threads AS p ON c."thread_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM users.parent_group_message_recipients AS c JOIN users.student_profiles AS p ON c."tenant_id" = p."tenant_id" AND c."student_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM users.parent_group_messages AS c JOIN users.parent_group_threads AS p ON c."thread_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM users.parent_group_thread_reads AS c JOIN users.parent_group_threads AS p ON c."thread_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM users.parent_messages AS c JOIN users.parent_group_threads AS p ON c."group_thread_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM users.parent_message_reads AS c JOIN users.parent_message_threads AS p ON c."thread_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM users.parent_message_threads AS c JOIN users.student_profiles AS p ON c."student_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM users.parent_messages AS c JOIN users.student_profiles AS p ON c."student_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM users.parent_messages AS c JOIN users.parent_message_threads AS p ON c."thread_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM users.parent_request_events AS c JOIN users.student_profiles AS p ON c."student_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM users.parent_request_share_events AS c JOIN users.student_profiles AS p ON c."student_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM users.staff_employment_profiles AS c JOIN config.work_time_models AS p ON c."work_time_model_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM users.staff_message_participants AS c JOIN users.staff_message_threads AS p ON c."thread_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM users.staff_message_reads AS c JOIN users.staff_message_threads AS p ON c."thread_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM users.staff_messages AS c JOIN users.staff_message_threads AS p ON c."thread_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM users.staff_notice_acks AS c JOIN users.staff_notices AS p ON c."notice_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM users.student_data_change_requests AS c JOIN users.student_profiles AS p ON c."student_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM users.student_family_protection_events AS c JOIN users.student_profiles AS p ON c."student_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM users.student_notes AS c JOIN activities.groups AS p ON c."activity_group_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM users.student_notes AS c JOIN education.groups AS p ON c."education_group_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
   ) THEN
    RAISE EXCEPTION 'school erasure blocked by foreign-scope reference' USING ERRCODE = '23503';
   END IF;
 END $body$;
 REVOKE ALL ON FUNCTION platform.school_erasure_preflight(BIGINT,BIGINT) FROM PUBLIC, phoenix_auth, phoenix_tenant, phoenix_admin, phoenix_demo, phoenix_school_erasure;
`

const signaturesSchoolErasureDeleteDownSQL = `
CREATE OR REPLACE FUNCTION platform.school_erasure_delete(p_school_id BIGINT)
 RETURNS JSONB LANGUAGE plpgsql VOLATILE SECURITY INVOKER SET search_path = pg_catalog
 AS $body$
 DECLARE
  v_counts JSONB := '{}'::jsonb;
  v_child_keys JSONB := '{}'::jsonb;
  v_keys JSONB;
  v_count BIGINT;
 BEGIN
   -- Freeze indirect keys before their parents disappear.
   SELECT COALESCE(jsonb_agg(r."role_id"), '[]'::jsonb) INTO v_keys FROM auth.role_permissions AS r WHERE r.role_id IN (SELECT p0.id FROM auth.roles AS p0 WHERE p0.tenant_id = p_school_id);
   v_child_keys := v_child_keys || jsonb_build_object('auth.role_permissions', v_keys);
   SELECT COALESCE(jsonb_agg(r."model_id"), '[]'::jsonb) INTO v_keys FROM config.work_time_model_entries AS r WHERE r.model_id IN (SELECT p0.id FROM config.work_time_models AS p0 WHERE p0.tenant_id = p_school_id);
   v_child_keys := v_child_keys || jsonb_build_object('config.work_time_model_entries', v_keys);
   SELECT COALESCE(jsonb_agg(r."student_guardian_id"), '[]'::jsonb) INTO v_keys FROM meta.meal_participation_permission_grants AS r WHERE r.student_guardian_id IN (SELECT p0.id FROM users.student_guardian_relationships AS p0 WHERE p0.tenant_id = p_school_id);
   v_child_keys := v_child_keys || jsonb_build_object('meta.meal_participation_permission_grants', v_keys);
   SELECT COALESCE(jsonb_agg(r."student_guardian_id"), '[]'::jsonb) INTO v_keys FROM meta.parent_student_consent_permission_grants AS r WHERE r.student_guardian_id IN (SELECT p0.id FROM users.student_guardian_relationships AS p0 WHERE p0.tenant_id = p_school_id);
   v_child_keys := v_child_keys || jsonb_build_object('meta.parent_student_consent_permission_grants', v_keys);
   -- Generated FK/trigger-topological order. School root is last.
   SELECT count(*) INTO v_count FROM active.activity_session_attendance AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM active.activity_session_attendance AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('active.activity_session_attendance', v_count);
   SELECT count(*) INTO v_count FROM active.activity_sessions AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM active.activity_sessions AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('active.activity_sessions', v_count);
   SELECT count(*) INTO v_count FROM active.attendance AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM active.attendance AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('active.attendance', v_count);
   SELECT count(*) INTO v_count FROM active.excused_absence_requests AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM active.excused_absence_requests AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('active.excused_absence_requests', v_count);
   SELECT count(*) INTO v_count FROM active.group_mappings AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM active.group_mappings AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('active.group_mappings', v_count);
   SELECT count(*) INTO v_count FROM active.combined_groups AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM active.combined_groups AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('active.combined_groups', v_count);
   SELECT count(*) INTO v_count FROM active.group_supervisors AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM active.group_supervisors AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('active.group_supervisors', v_count);
   SELECT count(*) INTO v_count FROM active.presence_backfill_batches AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM active.presence_backfill_batches AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('active.presence_backfill_batches', v_count);
   SELECT count(*) INTO v_count FROM active.presence_backfill_checkpoints AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM active.presence_backfill_checkpoints AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('active.presence_backfill_checkpoints', v_count);
   SELECT count(*) INTO v_count FROM active.scheduled_checkouts AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM active.scheduled_checkouts AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('active.scheduled_checkouts', v_count);
   SELECT count(*) INTO v_count FROM active.staff_absence_audit AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM active.staff_absence_audit AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('active.staff_absence_audit', v_count);
   SELECT count(*) INTO v_count FROM active.staff_absence_type_allowance_changes AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM active.staff_absence_type_allowance_changes AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('active.staff_absence_type_allowance_changes', v_count);
   SELECT count(*) INTO v_count FROM active.staff_absence_type_allowances AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM active.staff_absence_type_allowances AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('active.staff_absence_type_allowances', v_count);
   SELECT count(*) INTO v_count FROM active.staff_absences AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM active.staff_absences AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('active.staff_absences', v_count);
   SELECT count(*) INTO v_count FROM active.staff_absence_types AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM active.staff_absence_types AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('active.staff_absence_types', v_count);
   SELECT count(*) INTO v_count FROM active.staff_balance_adjustments AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM active.staff_balance_adjustments AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('active.staff_balance_adjustments', v_count);
   SELECT count(*) INTO v_count FROM active.staff_month_balance_snapshots AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM active.staff_month_balance_snapshots AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('active.staff_month_balance_snapshots', v_count);
   SELECT count(*) INTO v_count FROM active.staff_vacation_openings AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM active.staff_vacation_openings AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('active.staff_vacation_openings', v_count);
   SELECT count(*) INTO v_count FROM active.staff_vacation_quota AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM active.staff_vacation_quota AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('active.staff_vacation_quota', v_count);
   SELECT count(*) INTO v_count FROM active.staff_vacation_quota_changes AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM active.staff_vacation_quota_changes AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('active.staff_vacation_quota_changes', v_count);
   SELECT count(*) INTO v_count FROM active.student_status_days AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM active.student_status_days AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('active.student_status_days', v_count);
   SELECT count(*) INTO v_count FROM active.visits AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM active.visits AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('active.visits', v_count);
   SELECT count(*) INTO v_count FROM active.groups AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM active.groups AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('active.groups', v_count);
   SELECT count(*) INTO v_count FROM active.work_session_breaks AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM active.work_session_breaks AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('active.work_session_breaks', v_count);
   SELECT count(*) INTO v_count FROM activities.group_targets AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM activities.group_targets AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('activities.group_targets', v_count);
   SELECT count(*) INTO v_count FROM activities.schedules AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM activities.schedules AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('activities.schedules', v_count);
   SELECT count(*) INTO v_count FROM activities.student_enrollments AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM activities.student_enrollments AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('activities.student_enrollments', v_count);
   SELECT count(*) INTO v_count FROM activities.supervisors AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM activities.supervisors AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('activities.supervisors', v_count);
   SELECT count(*) INTO v_count FROM audit.attendance_corrections AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM audit.attendance_corrections AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('audit.attendance_corrections', v_count);
   SELECT count(*) INTO v_count FROM audit.auth_events AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM audit.auth_events AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('audit.auth_events', v_count);
   SELECT count(*) INTO v_count FROM audit.class_list_entry_changes AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM audit.class_list_entry_changes AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('audit.class_list_entry_changes', v_count);
   SELECT count(*) INTO v_count FROM audit.data_access_log AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM audit.data_access_log AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('audit.data_access_log', v_count);
   SELECT count(*) INTO v_count FROM audit.data_deletions AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM audit.data_deletions AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('audit.data_deletions', v_count);
   SELECT count(*) INTO v_count FROM audit.data_imports AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM audit.data_imports AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('audit.data_imports', v_count);
   SELECT count(*) INTO v_count FROM audit.deviation_events AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM audit.deviation_events AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('audit.deviation_events', v_count);
   SELECT count(*) INTO v_count FROM audit.enrollment_deletions AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM audit.enrollment_deletions AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('audit.enrollment_deletions', v_count);
   SELECT count(*) INTO v_count FROM audit.enrollment_offering_adjustments AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM audit.enrollment_offering_adjustments AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('audit.enrollment_offering_adjustments', v_count);
   SELECT count(*) INTO v_count FROM audit.enrollment_restorations AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM audit.enrollment_restorations AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('audit.enrollment_restorations', v_count);
   SELECT count(*) INTO v_count FROM audit.export_transfers AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM audit.export_transfers AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('audit.export_transfers', v_count);
   SELECT count(*) INTO v_count FROM audit.file_events AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM audit.file_events AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('audit.file_events', v_count);
   SELECT count(*) INTO v_count FROM audit.guardian_changes AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM audit.guardian_changes AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('audit.guardian_changes', v_count);
   SELECT count(*) INTO v_count FROM audit.guardian_financial_changes AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM audit.guardian_financial_changes AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('audit.guardian_financial_changes', v_count);
   SELECT count(*) INTO v_count FROM audit.personnel_number_changes AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM audit.personnel_number_changes AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('audit.personnel_number_changes', v_count);
   SELECT count(*) INTO v_count FROM audit.room_color_migration_backup AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM audit.room_color_migration_backup AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('audit.room_color_migration_backup', v_count);
   SELECT count(*) INTO v_count FROM audit.staff_master_data_changes AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM audit.staff_master_data_changes AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('audit.staff_master_data_changes', v_count);
   SELECT count(*) INTO v_count FROM audit.student_consent_changes AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM audit.student_consent_changes AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('audit.student_consent_changes', v_count);
   SELECT count(*) INTO v_count FROM audit.student_deletions AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM audit.student_deletions AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('audit.student_deletions', v_count);
   SELECT count(*) INTO v_count FROM audit.student_field_edits AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM audit.student_field_edits AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('audit.student_field_edits', v_count);
   SELECT count(*) INTO v_count FROM audit.substitution_changes AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM audit.substitution_changes AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('audit.substitution_changes', v_count);
   SELECT count(*) INTO v_count FROM audit.time_tracking_deletions AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM audit.time_tracking_deletions AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('audit.time_tracking_deletions', v_count);
   SELECT count(*) INTO v_count FROM audit.unregistered_tag_scans AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM audit.unregistered_tag_scans AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('audit.unregistered_tag_scans', v_count);
   SELECT count(*) INTO v_count FROM audit.wc_alias_migration_backup AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM audit.wc_alias_migration_backup AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('audit.wc_alias_migration_backup', v_count);
   SELECT count(*) INTO v_count FROM audit.work_session_edits AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM audit.work_session_edits AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('audit.work_session_edits', v_count);
   SELECT count(*) INTO v_count FROM active.work_sessions AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM active.work_sessions AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('active.work_sessions', v_count);
   SELECT count(*) INTO v_count FROM auth.account_permissions AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM auth.account_permissions AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('auth.account_permissions', v_count);
   SELECT count(*) INTO v_count FROM auth.account_roles AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM auth.account_roles AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('auth.account_roles', v_count);
   SELECT count(*) INTO v_count FROM auth.account_tenants AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM auth.account_tenants AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('auth.account_tenants', v_count);
   SELECT count(*) INTO v_count FROM auth.guardian_invitations AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM auth.guardian_invitations AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('auth.guardian_invitations', v_count);
   SELECT count(*) INTO v_count FROM auth.guardian_student_access AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM auth.guardian_student_access AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('auth.guardian_student_access', v_count);
   SELECT count(*) INTO v_count FROM auth.invitation_tokens AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM auth.invitation_tokens AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('auth.invitation_tokens', v_count);
   SELECT count(*) INTO v_count FROM auth.mfa_email_challenges AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM auth.mfa_email_challenges AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('auth.mfa_email_challenges', v_count);
   SELECT count(*) INTO v_count FROM auth.mfa_overrides AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM auth.mfa_overrides AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('auth.mfa_overrides', v_count);
   SELECT count(*) INTO v_count FROM auth.mfa_trusted_devices AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM auth.mfa_trusted_devices AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('auth.mfa_trusted_devices', v_count);
   SELECT count(*) INTO v_count FROM auth.parent_declaration_permission_grants AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM auth.parent_declaration_permission_grants AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('auth.parent_declaration_permission_grants', v_count);
   SELECT count(*) INTO v_count FROM auth.passkey_sessions AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM auth.passkey_sessions AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('auth.passkey_sessions', v_count);
   SELECT count(*) INTO v_count FROM auth.role_permissions AS r WHERE r.role_id IN (SELECT p0.id FROM auth.roles AS p0 WHERE p0.tenant_id = p_school_id);
   DELETE FROM auth.role_permissions AS r WHERE r.role_id IN (SELECT p0.id FROM auth.roles AS p0 WHERE p0.tenant_id = p_school_id);
   v_counts := v_counts || jsonb_build_object('auth.role_permissions', v_count);
   SELECT count(*) INTO v_count FROM auth.tokens AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM auth.tokens AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('auth.tokens', v_count);
   SELECT count(*) INTO v_count FROM calendar.appointment_occurrence_overrides AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM calendar.appointment_occurrence_overrides AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('calendar.appointment_occurrence_overrides', v_count);
   SELECT count(*) INTO v_count FROM calendar.appointment_recipient_students AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM calendar.appointment_recipient_students AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('calendar.appointment_recipient_students', v_count);
   SELECT count(*) INTO v_count FROM calendar.appointment_recipients AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM calendar.appointment_recipients AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('calendar.appointment_recipients', v_count);
   SELECT count(*) INTO v_count FROM calendar.appointment_reminder_push_deliveries AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM calendar.appointment_reminder_push_deliveries AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('calendar.appointment_reminder_push_deliveries', v_count);
   SELECT count(*) INTO v_count FROM calendar.appointment_targets AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM calendar.appointment_targets AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('calendar.appointment_targets', v_count);
   SELECT count(*) INTO v_count FROM calendar.recurrence_rules AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM calendar.recurrence_rules AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('calendar.recurrence_rules', v_count);
   SELECT count(*) INTO v_count FROM calendar.appointments AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM calendar.appointments AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('calendar.appointments', v_count);
   SELECT count(*) INTO v_count FROM config.home_block_policies AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM config.home_block_policies AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('config.home_block_policies', v_count);
   SELECT count(*) INTO v_count FROM config.home_layouts AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM config.home_layouts AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('config.home_layouts', v_count);
   SELECT count(*) INTO v_count FROM config.school_setup_dismissals AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM config.school_setup_dismissals AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('config.school_setup_dismissals', v_count);
   SELECT count(*) INTO v_count FROM config.school_setups AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM config.school_setups AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('config.school_setups', v_count);
   SELECT count(*) INTO v_count FROM config.setting_audit AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM config.setting_audit AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('config.setting_audit', v_count);
   SELECT count(*) INTO v_count FROM config.setting_values AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM config.setting_values AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('config.setting_values', v_count);
   SELECT count(*) INTO v_count FROM config.staff_onboardings AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM config.staff_onboardings AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('config.staff_onboardings', v_count);
   SELECT count(*) INTO v_count FROM config.staff_target_overrides AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM config.staff_target_overrides AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('config.staff_target_overrides', v_count);
   SELECT count(*) INTO v_count FROM config.staff_work_schedules AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM config.staff_work_schedules AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('config.staff_work_schedules', v_count);
   SELECT count(*) INTO v_count FROM config.work_time_model_entries AS r WHERE r.model_id IN (SELECT p0.id FROM config.work_time_models AS p0 WHERE p0.tenant_id = p_school_id);
   DELETE FROM config.work_time_model_entries AS r WHERE r.model_id IN (SELECT p0.id FROM config.work_time_models AS p0 WHERE p0.tenant_id = p_school_id);
   v_counts := v_counts || jsonb_build_object('config.work_time_model_entries', v_count);
   SELECT count(*) INTO v_count FROM display.displays AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM display.displays AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('display.displays', v_count);
   SELECT count(*) INTO v_count FROM documents.announcement_attachment_cleanup AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM documents.announcement_attachment_cleanup AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('documents.announcement_attachment_cleanup', v_count);
   SELECT count(*) INTO v_count FROM documents.announcement_attachments AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM documents.announcement_attachments AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('documents.announcement_attachments', v_count);
   SELECT count(*) INTO v_count FROM documents.file_cleanup AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM documents.file_cleanup AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('documents.file_cleanup', v_count);
   SELECT count(*) INTO v_count FROM documents.files AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM documents.files AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('documents.files', v_count);
   SELECT count(*) INTO v_count FROM documents.folder_accounts AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM documents.folder_accounts AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('documents.folder_accounts', v_count);
   SELECT count(*) INTO v_count FROM documents.folder_roles AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM documents.folder_roles AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('documents.folder_roles', v_count);
   SELECT count(*) INTO v_count FROM auth.roles AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM auth.roles AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('auth.roles', v_count);
   SELECT count(*) INTO v_count FROM documents.folders AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM documents.folders AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('documents.folders', v_count);
   SELECT count(*) INTO v_count FROM education.class_arrival_exceptions AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM education.class_arrival_exceptions AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('education.class_arrival_exceptions', v_count);
   SELECT count(*) INTO v_count FROM education.class_arrival_times AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM education.class_arrival_times AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('education.class_arrival_times', v_count);
   SELECT count(*) INTO v_count FROM education.class_teachers AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM education.class_teachers AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('education.class_teachers', v_count);
   SELECT count(*) INTO v_count FROM education.grade_transition_class_list_entries AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM education.grade_transition_class_list_entries AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('education.grade_transition_class_list_entries', v_count);
   SELECT count(*) INTO v_count FROM education.grade_transition_class_teachers AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM education.grade_transition_class_teachers AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('education.grade_transition_class_teachers', v_count);
   SELECT count(*) INTO v_count FROM education.grade_transition_history AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM education.grade_transition_history AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('education.grade_transition_history', v_count);
   SELECT count(*) INTO v_count FROM education.grade_transition_mappings AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM education.grade_transition_mappings AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('education.grade_transition_mappings', v_count);
   SELECT count(*) INTO v_count FROM education.group_substitution AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM education.group_substitution AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('education.group_substitution', v_count);
   SELECT count(*) INTO v_count FROM education.group_teacher AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM education.group_teacher AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('education.group_teacher', v_count);
   SELECT count(*) INTO v_count FROM enrollment.care_offering_auto_triggers AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM enrollment.care_offering_auto_triggers AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('enrollment.care_offering_auto_triggers', v_count);
   SELECT count(*) INTO v_count FROM enrollment.care_offering_bookings AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM enrollment.care_offering_bookings AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('enrollment.care_offering_bookings', v_count);
   SELECT count(*) INTO v_count FROM enrollment.change_request_messages AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM enrollment.change_request_messages AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('enrollment.change_request_messages', v_count);
   SELECT count(*) INTO v_count FROM enrollment.change_requests AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM enrollment.change_requests AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('enrollment.change_requests', v_count);
   SELECT count(*) INTO v_count FROM enrollment.late_invites AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM enrollment.late_invites AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('enrollment.late_invites', v_count);
   SELECT count(*) INTO v_count FROM enrollment.offering_change_requests AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM enrollment.offering_change_requests AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('enrollment.offering_change_requests', v_count);
   SELECT count(*) INTO v_count FROM enrollment.request_child_offering_selections AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM enrollment.request_child_offering_selections AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('enrollment.request_child_offering_selections', v_count);
   SELECT count(*) INTO v_count FROM enrollment.care_offerings AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM enrollment.care_offerings AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('enrollment.care_offerings', v_count);
   SELECT count(*) INTO v_count FROM enrollment.request_child_storage_backfill_checkpoints AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM enrollment.request_child_storage_backfill_checkpoints AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('enrollment.request_child_storage_backfill_checkpoints', v_count);
   SELECT count(*) INTO v_count FROM enrollment.request_uploads AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM enrollment.request_uploads AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('enrollment.request_uploads', v_count);
   SELECT count(*) INTO v_count FROM enrollment.request_upload_cleanup AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM enrollment.request_upload_cleanup AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('enrollment.request_upload_cleanup', v_count);
   SELECT count(*) INTO v_count FROM enrollment.request_children AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM enrollment.request_children AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('enrollment.request_children', v_count);
   SELECT count(*) INTO v_count FROM enrollment.request_guardians AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM enrollment.request_guardians AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('enrollment.request_guardians', v_count);
   SELECT count(*) INTO v_count FROM enrollment.request_reads AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM enrollment.request_reads AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('enrollment.request_reads', v_count);
   SELECT count(*) INTO v_count FROM enrollment.requests AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM enrollment.requests AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('enrollment.requests', v_count);
   SELECT count(*) INTO v_count FROM enrollment.form_schemas AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM enrollment.form_schemas AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('enrollment.form_schemas', v_count);
   SELECT count(*) INTO v_count FROM enrollment.phases AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM enrollment.phases AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('enrollment.phases', v_count);
   SELECT count(*) INTO v_count FROM enrollment.submission_rate_limits AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM enrollment.submission_rate_limits AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('enrollment.submission_rate_limits', v_count);
   SELECT count(*) INTO v_count FROM feedback.entries AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM feedback.entries AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('feedback.entries', v_count);
   SELECT count(*) INTO v_count FROM iot.devices AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM iot.devices AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('iot.devices', v_count);
   SELECT count(*) INTO v_count FROM iot.push_subscriptions AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM iot.push_subscriptions AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('iot.push_subscriptions', v_count);
   SELECT count(*) INTO v_count FROM iot.pwa_standalone_usage AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM iot.pwa_standalone_usage AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('iot.pwa_standalone_usage', v_count);
   SELECT count(*) INTO v_count FROM meta.meal_participation_permission_grants AS r WHERE r.student_guardian_id IN (SELECT p0.id FROM users.student_guardian_relationships AS p0 WHERE p0.tenant_id = p_school_id);
   DELETE FROM meta.meal_participation_permission_grants AS r WHERE r.student_guardian_id IN (SELECT p0.id FROM users.student_guardian_relationships AS p0 WHERE p0.tenant_id = p_school_id);
   v_counts := v_counts || jsonb_build_object('meta.meal_participation_permission_grants', v_count);
   SELECT count(*) INTO v_count FROM meta.parent_student_consent_permission_grants AS r WHERE r.student_guardian_id IN (SELECT p0.id FROM users.student_guardian_relationships AS p0 WHERE p0.tenant_id = p_school_id);
   DELETE FROM meta.parent_student_consent_permission_grants AS r WHERE r.student_guardian_id IN (SELECT p0.id FROM users.student_guardian_relationships AS p0 WHERE p0.tenant_id = p_school_id);
   v_counts := v_counts || jsonb_build_object('meta.parent_student_consent_permission_grants', v_count);
   SELECT count(*) INTO v_count FROM platform.cleanup_runs AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM platform.cleanup_runs AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('platform.cleanup_runs', v_count);
   SELECT count(*) INTO v_count FROM platform.demo_school_states AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM platform.demo_school_states AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('platform.demo_school_states', v_count);
   SELECT count(*) INTO v_count FROM platform.email_delivery AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM platform.email_delivery AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('platform.email_delivery', v_count);
   SELECT count(*) INTO v_count FROM platform.email_outbox AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM platform.email_outbox AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('platform.email_outbox', v_count);
   SELECT count(*) INTO v_count FROM platform.push_outbox AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM platform.push_outbox AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('platform.push_outbox', v_count);
   SELECT count(*) INTO v_count FROM platform.storage_backfill_checkpoints AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM platform.storage_backfill_checkpoints AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('platform.storage_backfill_checkpoints', v_count);
   SELECT count(*) INTO v_count FROM schedule.activity_exceptions AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM schedule.activity_exceptions AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('schedule.activity_exceptions', v_count);
   SELECT count(*) INTO v_count FROM activities.groups AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM activities.groups AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('activities.groups', v_count);
   SELECT count(*) INTO v_count FROM activities.categories AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM activities.categories AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('activities.categories', v_count);
   SELECT count(*) INTO v_count FROM education.groups AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM education.groups AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('education.groups', v_count);
   SELECT count(*) INTO v_count FROM schedule.care_schedule_change_requests AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM schedule.care_schedule_change_requests AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('schedule.care_schedule_change_requests', v_count);
   SELECT count(*) INTO v_count FROM schedule.closing_days AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM schedule.closing_days AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('schedule.closing_days', v_count);
   SELECT count(*) INTO v_count FROM schedule.dateframes AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM schedule.dateframes AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('schedule.dateframes', v_count);
   SELECT count(*) INTO v_count FROM schedule.grade_transition_roster_removals AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM schedule.grade_transition_roster_removals AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('schedule.grade_transition_roster_removals', v_count);
   SELECT count(*) INTO v_count FROM education.grade_transitions AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM education.grade_transitions AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('education.grade_transitions', v_count);
   SELECT count(*) INTO v_count FROM schedule.instance_staff AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM schedule.instance_staff AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('schedule.instance_staff', v_count);
   SELECT count(*) INTO v_count FROM schedule.instance_students AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM schedule.instance_students AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('schedule.instance_students', v_count);
   SELECT count(*) INTO v_count FROM schedule.meal_participation_overrides AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM schedule.meal_participation_overrides AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('schedule.meal_participation_overrides', v_count);
   SELECT count(*) INTO v_count FROM schedule.meal_participation_schedules AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM schedule.meal_participation_schedules AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('schedule.meal_participation_schedules', v_count);
   SELECT count(*) INTO v_count FROM schedule.meal_plan_entries AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM schedule.meal_plan_entries AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('schedule.meal_plan_entries', v_count);
   SELECT count(*) INTO v_count FROM schedule.meal_sickness_status_history AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM schedule.meal_sickness_status_history AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('schedule.meal_sickness_status_history', v_count);
   SELECT count(*) INTO v_count FROM schedule.pickup_extension_tasks AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM schedule.pickup_extension_tasks AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('schedule.pickup_extension_tasks', v_count);
   SELECT count(*) INTO v_count FROM schedule.planning_tracks AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM schedule.planning_tracks AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('schedule.planning_tracks', v_count);
   SELECT count(*) INTO v_count FROM schedule.recurrence_rules AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM schedule.recurrence_rules AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('schedule.recurrence_rules', v_count);
   SELECT count(*) INTO v_count FROM schedule.shift_types AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM schedule.shift_types AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('schedule.shift_types', v_count);
   SELECT count(*) INTO v_count FROM schedule.staff_shift_series_exceptions AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM schedule.staff_shift_series_exceptions AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('schedule.staff_shift_series_exceptions', v_count);
   SELECT count(*) INTO v_count FROM schedule.staff_shift_series AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM schedule.staff_shift_series AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('schedule.staff_shift_series', v_count);
   SELECT count(*) INTO v_count FROM schedule.calendar_periods AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM schedule.calendar_periods AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('schedule.calendar_periods', v_count);
   SELECT count(*) INTO v_count FROM schedule.staff_shifts AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM schedule.staff_shifts AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('schedule.staff_shifts', v_count);
   SELECT count(*) INTO v_count FROM schedule.student_arrival_exceptions AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM schedule.student_arrival_exceptions AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('schedule.student_arrival_exceptions', v_count);
   SELECT count(*) INTO v_count FROM schedule.student_arrival_notes AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM schedule.student_arrival_notes AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('schedule.student_arrival_notes', v_count);
   SELECT count(*) INTO v_count FROM schedule.student_arrival_schedules AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM schedule.student_arrival_schedules AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('schedule.student_arrival_schedules', v_count);
   SELECT count(*) INTO v_count FROM schedule.student_departure_exceptions AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM schedule.student_departure_exceptions AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('schedule.student_departure_exceptions', v_count);
   SELECT count(*) INTO v_count FROM schedule.student_pickup_exceptions AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM schedule.student_pickup_exceptions AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('schedule.student_pickup_exceptions', v_count);
   SELECT count(*) INTO v_count FROM schedule.student_pickup_notes AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM schedule.student_pickup_notes AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('schedule.student_pickup_notes', v_count);
   SELECT count(*) INTO v_count FROM schedule.student_pickup_schedules AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM schedule.student_pickup_schedules AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('schedule.student_pickup_schedules', v_count);
   SELECT count(*) INTO v_count FROM schedule.timeframes AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM schedule.timeframes AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('schedule.timeframes', v_count);
   SELECT count(*) INTO v_count FROM schedule.timetable_conflict_acks AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM schedule.timetable_conflict_acks AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('schedule.timetable_conflict_acks', v_count);
   SELECT count(*) INTO v_count FROM users.care_withdrawal_completions AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM users.care_withdrawal_completions AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('users.care_withdrawal_completions', v_count);
   SELECT count(*) INTO v_count FROM users.class_list_entries AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM users.class_list_entries AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('users.class_list_entries', v_count);
   SELECT count(*) INTO v_count FROM users.external_caregivers AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM users.external_caregivers AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('users.external_caregivers', v_count);
   SELECT count(*) INTO v_count FROM users.guardian_financial_data AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM users.guardian_financial_data AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('users.guardian_financial_data', v_count);
   SELECT count(*) INTO v_count FROM users.guardian_phone_numbers AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM users.guardian_phone_numbers AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('users.guardian_phone_numbers', v_count);
   SELECT count(*) INTO v_count FROM users.notification_preferences AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM users.notification_preferences AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('users.notification_preferences', v_count);
   SELECT count(*) INTO v_count FROM users.parent_announcement_declaration_submissions AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM users.parent_announcement_declaration_submissions AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('users.parent_announcement_declaration_submissions', v_count);
   SELECT count(*) INTO v_count FROM users.parent_announcement_declaration_versions AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM users.parent_announcement_declaration_versions AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('users.parent_announcement_declaration_versions', v_count);
   SELECT count(*) INTO v_count FROM users.parent_announcement_reads AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM users.parent_announcement_reads AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('users.parent_announcement_reads', v_count);
   SELECT count(*) INTO v_count FROM users.parent_announcement_responses AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM users.parent_announcement_responses AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('users.parent_announcement_responses', v_count);
   SELECT count(*) INTO v_count FROM users.parent_announcement_options AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM users.parent_announcement_options AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('users.parent_announcement_options', v_count);
   SELECT count(*) INTO v_count FROM users.parent_announcement_targets AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM users.parent_announcement_targets AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('users.parent_announcement_targets', v_count);
   SELECT count(*) INTO v_count FROM users.parent_announcements AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM users.parent_announcements AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('users.parent_announcements', v_count);
   SELECT count(*) INTO v_count FROM users.parent_group_message_recipients AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM users.parent_group_message_recipients AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('users.parent_group_message_recipients', v_count);
   SELECT count(*) INTO v_count FROM users.parent_group_messages AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM users.parent_group_messages AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('users.parent_group_messages', v_count);
   SELECT count(*) INTO v_count FROM users.parent_group_thread_reads AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM users.parent_group_thread_reads AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('users.parent_group_thread_reads', v_count);
   SELECT count(*) INTO v_count FROM users.parent_message_count_preferences AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM users.parent_message_count_preferences AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('users.parent_message_count_preferences', v_count);
   SELECT count(*) INTO v_count FROM users.parent_message_reads AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM users.parent_message_reads AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('users.parent_message_reads', v_count);
   SELECT count(*) INTO v_count FROM users.parent_messages AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM users.parent_messages AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('users.parent_messages', v_count);
   SELECT count(*) INTO v_count FROM users.parent_group_threads AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM users.parent_group_threads AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('users.parent_group_threads', v_count);
   SELECT count(*) INTO v_count FROM users.parent_message_threads AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM users.parent_message_threads AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('users.parent_message_threads', v_count);
   SELECT count(*) INTO v_count FROM users.parent_request_events AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM users.parent_request_events AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('users.parent_request_events', v_count);
   SELECT count(*) INTO v_count FROM users.parent_request_rss_feeds AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM users.parent_request_rss_feeds AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('users.parent_request_rss_feeds', v_count);
   SELECT count(*) INTO v_count FROM users.parent_request_share_events AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM users.parent_request_share_events AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('users.parent_request_share_events', v_count);
   SELECT count(*) INTO v_count FROM users.privacy_consents AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM users.privacy_consents AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('users.privacy_consents', v_count);
   SELECT count(*) INTO v_count FROM users.profiles AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM users.profiles AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('users.profiles', v_count);
   SELECT count(*) INTO v_count FROM users.rfid_cards AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM users.rfid_cards AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('users.rfid_cards', v_count);
   SELECT count(*) INTO v_count FROM users.staff_document_file_cleanup AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM users.staff_document_file_cleanup AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('users.staff_document_file_cleanup', v_count);
   SELECT count(*) INTO v_count FROM users.staff_documents AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM users.staff_documents AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('users.staff_documents', v_count);
   SELECT count(*) INTO v_count FROM users.staff_employment_profiles AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM users.staff_employment_profiles AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('users.staff_employment_profiles', v_count);
   SELECT count(*) INTO v_count FROM config.work_time_models AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM config.work_time_models AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('config.work_time_models', v_count);
   SELECT count(*) INTO v_count FROM users.staff_financial_data AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM users.staff_financial_data AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('users.staff_financial_data', v_count);
   SELECT count(*) INTO v_count FROM users.staff_master_data AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM users.staff_master_data AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('users.staff_master_data', v_count);
   SELECT count(*) INTO v_count FROM users.staff_message_participants AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM users.staff_message_participants AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('users.staff_message_participants', v_count);
   SELECT count(*) INTO v_count FROM users.staff_message_reads AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM users.staff_message_reads AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('users.staff_message_reads', v_count);
   SELECT count(*) INTO v_count FROM users.staff_messages AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM users.staff_messages AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('users.staff_messages', v_count);
   SELECT count(*) INTO v_count FROM users.staff_message_threads AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM users.staff_message_threads AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('users.staff_message_threads', v_count);
   SELECT count(*) INTO v_count FROM users.staff_notice_acks AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM users.staff_notice_acks AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('users.staff_notice_acks', v_count);
   SELECT count(*) INTO v_count FROM users.staff_notices AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM users.staff_notices AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('users.staff_notices', v_count);
   SELECT count(*) INTO v_count FROM users.staff_offboarding_cleanup AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM users.staff_offboarding_cleanup AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('users.staff_offboarding_cleanup', v_count);
   SELECT count(*) INTO v_count FROM users.staff_qualifications AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM users.staff_qualifications AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('users.staff_qualifications', v_count);
   SELECT count(*) INTO v_count FROM users.student_care_exit_removals AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM users.student_care_exit_removals AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('users.student_care_exit_removals', v_count);
   SELECT count(*) INTO v_count FROM schedule.activity_instances AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM schedule.activity_instances AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('schedule.activity_instances', v_count);
   SELECT count(*) INTO v_count FROM calendar.staff_feed_tombstones AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM calendar.staff_feed_tombstones AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('calendar.staff_feed_tombstones', v_count);
   SELECT count(*) INTO v_count FROM facilities.rooms AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM facilities.rooms AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('facilities.rooms', v_count);
   SELECT count(*) INTO v_count FROM users.student_care_exit_source_removals AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM users.student_care_exit_source_removals AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('users.student_care_exit_source_removals', v_count);
   SELECT count(*) INTO v_count FROM users.student_care_exits AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM users.student_care_exits AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('users.student_care_exits', v_count);
   SELECT count(*) INTO v_count FROM users.student_care_profiles AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM users.student_care_profiles AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('users.student_care_profiles', v_count);
   SELECT count(*) INTO v_count FROM users.student_companions AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM users.student_companions AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('users.student_companions', v_count);
   SELECT count(*) INTO v_count FROM users.student_data_change_requests AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM users.student_data_change_requests AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('users.student_data_change_requests', v_count);
   SELECT count(*) INTO v_count FROM users.student_document_file_cleanup AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM users.student_document_file_cleanup AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('users.student_document_file_cleanup', v_count);
   SELECT count(*) INTO v_count FROM users.student_documents AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM users.student_documents AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('users.student_documents', v_count);
   SELECT count(*) INTO v_count FROM users.student_family_protection_events AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM users.student_family_protection_events AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('users.student_family_protection_events', v_count);
   SELECT count(*) INTO v_count FROM users.student_guardian_pickup_permissions AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM users.student_guardian_pickup_permissions AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('users.student_guardian_pickup_permissions', v_count);
   SELECT count(*) INTO v_count FROM users.student_guardian_relationships AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM users.student_guardian_relationships AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('users.student_guardian_relationships', v_count);
   SELECT count(*) INTO v_count FROM users.guardian_profiles AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM users.guardian_profiles AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('users.guardian_profiles', v_count);
   SELECT count(*) INTO v_count FROM users.student_notes AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM users.student_notes AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('users.student_notes', v_count);
   SELECT count(*) INTO v_count FROM users.student_school_memberships AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM users.student_school_memberships AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('users.student_school_memberships', v_count);
   SELECT count(*) INTO v_count FROM users.student_profiles AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM users.student_profiles AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('users.student_profiles', v_count);
   SELECT count(*) INTO v_count FROM users.teachers AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM users.teachers AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('users.teachers', v_count);
   SELECT count(*) INTO v_count FROM users.staff_school_memberships AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM users.staff_school_memberships AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('users.staff_school_memberships', v_count);
   SELECT count(*) INTO v_count FROM users.persons AS r WHERE r.tenant_id = p_school_id;
   DELETE FROM users.persons AS r WHERE r.tenant_id = p_school_id;
   v_counts := v_counts || jsonb_build_object('users.persons', v_count);
   SELECT count(*) INTO v_count FROM platform.schools AS r WHERE r.id = p_school_id;
   DELETE FROM platform.schools AS r WHERE r.id = p_school_id;
   v_counts := v_counts || jsonb_build_object('platform.schools', v_count);
   PERFORM platform.school_erasure_assert_empty(p_school_id,v_child_keys);
   RETURN v_counts;
 END $body$;
 REVOKE ALL ON FUNCTION platform.school_erasure_delete(BIGINT) FROM PUBLIC, phoenix_auth, phoenix_tenant, phoenix_admin, phoenix_demo, phoenix_school_erasure;
`

const signaturesSchoolErasurePreviewDownSQL = `

 CREATE OR REPLACE FUNCTION platform.school_erasure_preview(p_school BIGINT,p_operator BIGINT)
 RETURNS JSONB LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $body$
 DECLARE v_counts JSONB := '{}';
 BEGIN
 IF NOT EXISTS (SELECT FROM platform.operators WHERE id=p_operator AND active) THEN
 RAISE EXCEPTION 'active operator required' USING ERRCODE='42501'; END IF;
 IF NOT EXISTS (SELECT FROM platform.schools WHERE id=p_school) THEN
 RAISE EXCEPTION 'existing school required' USING ERRCODE='22023'; END IF;
 -- Frozen count projection of the installed erasure plan, guarded by CLI tests.
 v_counts := v_counts || jsonb_build_object('active.activity_session_attendance',(SELECT count(*) FROM active.activity_session_attendance AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('active.activity_sessions',(SELECT count(*) FROM active.activity_sessions AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('active.attendance',(SELECT count(*) FROM active.attendance AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('active.combined_groups',(SELECT count(*) FROM active.combined_groups AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('active.excused_absence_requests',(SELECT count(*) FROM active.excused_absence_requests AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('active.group_mappings',(SELECT count(*) FROM active.group_mappings AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('active.group_supervisors',(SELECT count(*) FROM active.group_supervisors AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('active.groups',(SELECT count(*) FROM active.groups AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('active.presence_backfill_batches',(SELECT count(*) FROM active.presence_backfill_batches AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('active.presence_backfill_checkpoints',(SELECT count(*) FROM active.presence_backfill_checkpoints AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('active.scheduled_checkouts',(SELECT count(*) FROM active.scheduled_checkouts AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('active.staff_absence_audit',(SELECT count(*) FROM active.staff_absence_audit AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('active.staff_absence_type_allowance_changes',(SELECT count(*) FROM active.staff_absence_type_allowance_changes AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('active.staff_absence_type_allowances',(SELECT count(*) FROM active.staff_absence_type_allowances AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('active.staff_absence_types',(SELECT count(*) FROM active.staff_absence_types AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('active.staff_absences',(SELECT count(*) FROM active.staff_absences AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('active.staff_balance_adjustments',(SELECT count(*) FROM active.staff_balance_adjustments AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('active.staff_month_balance_snapshots',(SELECT count(*) FROM active.staff_month_balance_snapshots AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('active.staff_vacation_openings',(SELECT count(*) FROM active.staff_vacation_openings AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('active.staff_vacation_quota',(SELECT count(*) FROM active.staff_vacation_quota AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('active.staff_vacation_quota_changes',(SELECT count(*) FROM active.staff_vacation_quota_changes AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('active.student_status_days',(SELECT count(*) FROM active.student_status_days AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('active.visits',(SELECT count(*) FROM active.visits AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('active.work_session_breaks',(SELECT count(*) FROM active.work_session_breaks AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('active.work_sessions',(SELECT count(*) FROM active.work_sessions AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('activities.categories',(SELECT count(*) FROM activities.categories AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('activities.group_targets',(SELECT count(*) FROM activities.group_targets AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('activities.groups',(SELECT count(*) FROM activities.groups AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('activities.schedules',(SELECT count(*) FROM activities.schedules AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('activities.student_enrollments',(SELECT count(*) FROM activities.student_enrollments AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('activities.supervisors',(SELECT count(*) FROM activities.supervisors AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('audit.attendance_corrections',(SELECT count(*) FROM audit.attendance_corrections AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('audit.auth_events',(SELECT count(*) FROM audit.auth_events AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('audit.class_list_entry_changes',(SELECT count(*) FROM audit.class_list_entry_changes AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('audit.data_access_log',(SELECT count(*) FROM audit.data_access_log AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('audit.data_deletions',(SELECT count(*) FROM audit.data_deletions AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('audit.data_imports',(SELECT count(*) FROM audit.data_imports AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('audit.deviation_events',(SELECT count(*) FROM audit.deviation_events AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('audit.enrollment_deletions',(SELECT count(*) FROM audit.enrollment_deletions AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('audit.enrollment_offering_adjustments',(SELECT count(*) FROM audit.enrollment_offering_adjustments AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('audit.enrollment_restorations',(SELECT count(*) FROM audit.enrollment_restorations AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('audit.export_transfers',(SELECT count(*) FROM audit.export_transfers AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('audit.file_events',(SELECT count(*) FROM audit.file_events AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('audit.guardian_changes',(SELECT count(*) FROM audit.guardian_changes AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('audit.guardian_financial_changes',(SELECT count(*) FROM audit.guardian_financial_changes AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('audit.personnel_number_changes',(SELECT count(*) FROM audit.personnel_number_changes AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('audit.room_color_migration_backup',(SELECT count(*) FROM audit.room_color_migration_backup AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('audit.staff_master_data_changes',(SELECT count(*) FROM audit.staff_master_data_changes AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('audit.student_consent_changes',(SELECT count(*) FROM audit.student_consent_changes AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('audit.student_deletions',(SELECT count(*) FROM audit.student_deletions AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('audit.student_field_edits',(SELECT count(*) FROM audit.student_field_edits AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('audit.substitution_changes',(SELECT count(*) FROM audit.substitution_changes AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('audit.time_tracking_deletions',(SELECT count(*) FROM audit.time_tracking_deletions AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('audit.unregistered_tag_scans',(SELECT count(*) FROM audit.unregistered_tag_scans AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('audit.wc_alias_migration_backup',(SELECT count(*) FROM audit.wc_alias_migration_backup AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('audit.work_session_edits',(SELECT count(*) FROM audit.work_session_edits AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('auth.account_permissions',(SELECT count(*) FROM auth.account_permissions AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('auth.account_roles',(SELECT count(*) FROM auth.account_roles AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('auth.account_tenants',(SELECT count(*) FROM auth.account_tenants AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('auth.guardian_invitations',(SELECT count(*) FROM auth.guardian_invitations AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('auth.guardian_student_access',(SELECT count(*) FROM auth.guardian_student_access AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('auth.invitation_tokens',(SELECT count(*) FROM auth.invitation_tokens AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('auth.mfa_email_challenges',(SELECT count(*) FROM auth.mfa_email_challenges AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('auth.mfa_overrides',(SELECT count(*) FROM auth.mfa_overrides AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('auth.mfa_trusted_devices',(SELECT count(*) FROM auth.mfa_trusted_devices AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('auth.parent_declaration_permission_grants',(SELECT count(*) FROM auth.parent_declaration_permission_grants AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('auth.passkey_sessions',(SELECT count(*) FROM auth.passkey_sessions AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('auth.role_permissions',(SELECT count(*) FROM auth.role_permissions AS r WHERE r.role_id IN (SELECT p0.id FROM auth.roles AS p0 WHERE p0.tenant_id = p_school)));
 v_counts := v_counts || jsonb_build_object('auth.roles',(SELECT count(*) FROM auth.roles AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('auth.tokens',(SELECT count(*) FROM auth.tokens AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('calendar.appointment_occurrence_overrides',(SELECT count(*) FROM calendar.appointment_occurrence_overrides AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('calendar.appointment_recipient_students',(SELECT count(*) FROM calendar.appointment_recipient_students AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('calendar.appointment_recipients',(SELECT count(*) FROM calendar.appointment_recipients AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('calendar.appointment_reminder_push_deliveries',(SELECT count(*) FROM calendar.appointment_reminder_push_deliveries AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('calendar.appointment_targets',(SELECT count(*) FROM calendar.appointment_targets AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('calendar.appointments',(SELECT count(*) FROM calendar.appointments AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('calendar.recurrence_rules',(SELECT count(*) FROM calendar.recurrence_rules AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('calendar.staff_feed_tombstones',(SELECT count(*) FROM calendar.staff_feed_tombstones AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('config.home_block_policies',(SELECT count(*) FROM config.home_block_policies AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('config.home_layouts',(SELECT count(*) FROM config.home_layouts AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('config.school_setup_dismissals',(SELECT count(*) FROM config.school_setup_dismissals AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('config.school_setups',(SELECT count(*) FROM config.school_setups AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('config.setting_audit',(SELECT count(*) FROM config.setting_audit AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('config.setting_values',(SELECT count(*) FROM config.setting_values AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('config.staff_onboardings',(SELECT count(*) FROM config.staff_onboardings AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('config.staff_target_overrides',(SELECT count(*) FROM config.staff_target_overrides AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('config.staff_work_schedules',(SELECT count(*) FROM config.staff_work_schedules AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('config.work_time_model_entries',(SELECT count(*) FROM config.work_time_model_entries AS r WHERE r.model_id IN (SELECT p0.id FROM config.work_time_models AS p0 WHERE p0.tenant_id = p_school)));
 v_counts := v_counts || jsonb_build_object('config.work_time_models',(SELECT count(*) FROM config.work_time_models AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('display.displays',(SELECT count(*) FROM display.displays AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('documents.announcement_attachment_cleanup',(SELECT count(*) FROM documents.announcement_attachment_cleanup AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('documents.announcement_attachments',(SELECT count(*) FROM documents.announcement_attachments AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('documents.file_cleanup',(SELECT count(*) FROM documents.file_cleanup AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('documents.files',(SELECT count(*) FROM documents.files AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('documents.folder_accounts',(SELECT count(*) FROM documents.folder_accounts AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('documents.folder_roles',(SELECT count(*) FROM documents.folder_roles AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('documents.folders',(SELECT count(*) FROM documents.folders AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('education.class_arrival_exceptions',(SELECT count(*) FROM education.class_arrival_exceptions AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('education.class_arrival_times',(SELECT count(*) FROM education.class_arrival_times AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('education.class_teachers',(SELECT count(*) FROM education.class_teachers AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('education.grade_transition_class_list_entries',(SELECT count(*) FROM education.grade_transition_class_list_entries AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('education.grade_transition_class_teachers',(SELECT count(*) FROM education.grade_transition_class_teachers AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('education.grade_transition_history',(SELECT count(*) FROM education.grade_transition_history AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('education.grade_transition_mappings',(SELECT count(*) FROM education.grade_transition_mappings AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('education.grade_transitions',(SELECT count(*) FROM education.grade_transitions AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('education.group_substitution',(SELECT count(*) FROM education.group_substitution AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('education.group_teacher',(SELECT count(*) FROM education.group_teacher AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('education.groups',(SELECT count(*) FROM education.groups AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('enrollment.care_offering_auto_triggers',(SELECT count(*) FROM enrollment.care_offering_auto_triggers AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('enrollment.care_offering_bookings',(SELECT count(*) FROM enrollment.care_offering_bookings AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('enrollment.care_offerings',(SELECT count(*) FROM enrollment.care_offerings AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('enrollment.change_request_messages',(SELECT count(*) FROM enrollment.change_request_messages AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('enrollment.change_requests',(SELECT count(*) FROM enrollment.change_requests AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('enrollment.form_schemas',(SELECT count(*) FROM enrollment.form_schemas AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('enrollment.late_invites',(SELECT count(*) FROM enrollment.late_invites AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('enrollment.offering_change_requests',(SELECT count(*) FROM enrollment.offering_change_requests AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('enrollment.phases',(SELECT count(*) FROM enrollment.phases AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('enrollment.request_child_offering_selections',(SELECT count(*) FROM enrollment.request_child_offering_selections AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('enrollment.request_child_storage_backfill_checkpoints',(SELECT count(*) FROM enrollment.request_child_storage_backfill_checkpoints AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('enrollment.request_children',(SELECT count(*) FROM enrollment.request_children AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('enrollment.request_guardians',(SELECT count(*) FROM enrollment.request_guardians AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('enrollment.request_reads',(SELECT count(*) FROM enrollment.request_reads AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('enrollment.request_upload_cleanup',(SELECT count(*) FROM enrollment.request_upload_cleanup AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('enrollment.request_uploads',(SELECT count(*) FROM enrollment.request_uploads AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('enrollment.requests',(SELECT count(*) FROM enrollment.requests AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('enrollment.submission_rate_limits',(SELECT count(*) FROM enrollment.submission_rate_limits AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('facilities.rooms',(SELECT count(*) FROM facilities.rooms AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('feedback.entries',(SELECT count(*) FROM feedback.entries AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('iot.devices',(SELECT count(*) FROM iot.devices AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('iot.push_subscriptions',(SELECT count(*) FROM iot.push_subscriptions AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('iot.pwa_standalone_usage',(SELECT count(*) FROM iot.pwa_standalone_usage AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('meta.meal_participation_permission_grants',(SELECT count(*) FROM meta.meal_participation_permission_grants AS r WHERE r.student_guardian_id IN (SELECT p0.id FROM users.student_guardian_relationships AS p0 WHERE p0.tenant_id = p_school)));
 v_counts := v_counts || jsonb_build_object('meta.parent_student_consent_permission_grants',(SELECT count(*) FROM meta.parent_student_consent_permission_grants AS r WHERE r.student_guardian_id IN (SELECT p0.id FROM users.student_guardian_relationships AS p0 WHERE p0.tenant_id = p_school)));
 v_counts := v_counts || jsonb_build_object('platform.cleanup_runs',(SELECT count(*) FROM platform.cleanup_runs AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('platform.demo_school_states',(SELECT count(*) FROM platform.demo_school_states AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('platform.email_delivery',(SELECT count(*) FROM platform.email_delivery AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('platform.email_outbox',(SELECT count(*) FROM platform.email_outbox AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('platform.push_outbox',(SELECT count(*) FROM platform.push_outbox AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('platform.schools',(SELECT count(*) FROM platform.schools AS r WHERE r.id = p_school));
 v_counts := v_counts || jsonb_build_object('platform.storage_backfill_checkpoints',(SELECT count(*) FROM platform.storage_backfill_checkpoints AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('schedule.activity_exceptions',(SELECT count(*) FROM schedule.activity_exceptions AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('schedule.activity_instances',(SELECT count(*) FROM schedule.activity_instances AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('schedule.calendar_periods',(SELECT count(*) FROM schedule.calendar_periods AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('schedule.care_schedule_change_requests',(SELECT count(*) FROM schedule.care_schedule_change_requests AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('schedule.closing_days',(SELECT count(*) FROM schedule.closing_days AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('schedule.dateframes',(SELECT count(*) FROM schedule.dateframes AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('schedule.grade_transition_roster_removals',(SELECT count(*) FROM schedule.grade_transition_roster_removals AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('schedule.instance_staff',(SELECT count(*) FROM schedule.instance_staff AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('schedule.instance_students',(SELECT count(*) FROM schedule.instance_students AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('schedule.meal_participation_overrides',(SELECT count(*) FROM schedule.meal_participation_overrides AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('schedule.meal_participation_schedules',(SELECT count(*) FROM schedule.meal_participation_schedules AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('schedule.meal_plan_entries',(SELECT count(*) FROM schedule.meal_plan_entries AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('schedule.meal_sickness_status_history',(SELECT count(*) FROM schedule.meal_sickness_status_history AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('schedule.pickup_extension_tasks',(SELECT count(*) FROM schedule.pickup_extension_tasks AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('schedule.planning_tracks',(SELECT count(*) FROM schedule.planning_tracks AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('schedule.recurrence_rules',(SELECT count(*) FROM schedule.recurrence_rules AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('schedule.shift_types',(SELECT count(*) FROM schedule.shift_types AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('schedule.staff_shift_series',(SELECT count(*) FROM schedule.staff_shift_series AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('schedule.staff_shift_series_exceptions',(SELECT count(*) FROM schedule.staff_shift_series_exceptions AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('schedule.staff_shifts',(SELECT count(*) FROM schedule.staff_shifts AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('schedule.student_arrival_exceptions',(SELECT count(*) FROM schedule.student_arrival_exceptions AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('schedule.student_arrival_notes',(SELECT count(*) FROM schedule.student_arrival_notes AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('schedule.student_arrival_schedules',(SELECT count(*) FROM schedule.student_arrival_schedules AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('schedule.student_departure_exceptions',(SELECT count(*) FROM schedule.student_departure_exceptions AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('schedule.student_pickup_exceptions',(SELECT count(*) FROM schedule.student_pickup_exceptions AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('schedule.student_pickup_notes',(SELECT count(*) FROM schedule.student_pickup_notes AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('schedule.student_pickup_schedules',(SELECT count(*) FROM schedule.student_pickup_schedules AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('schedule.timeframes',(SELECT count(*) FROM schedule.timeframes AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('schedule.timetable_conflict_acks',(SELECT count(*) FROM schedule.timetable_conflict_acks AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('users.care_withdrawal_completions',(SELECT count(*) FROM users.care_withdrawal_completions AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('users.class_list_entries',(SELECT count(*) FROM users.class_list_entries AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('users.external_caregivers',(SELECT count(*) FROM users.external_caregivers AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('users.guardian_financial_data',(SELECT count(*) FROM users.guardian_financial_data AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('users.guardian_phone_numbers',(SELECT count(*) FROM users.guardian_phone_numbers AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('users.guardian_profiles',(SELECT count(*) FROM users.guardian_profiles AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('users.notification_preferences',(SELECT count(*) FROM users.notification_preferences AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('users.parent_announcement_declaration_submissions',(SELECT count(*) FROM users.parent_announcement_declaration_submissions AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('users.parent_announcement_declaration_versions',(SELECT count(*) FROM users.parent_announcement_declaration_versions AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('users.parent_announcement_options',(SELECT count(*) FROM users.parent_announcement_options AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('users.parent_announcement_reads',(SELECT count(*) FROM users.parent_announcement_reads AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('users.parent_announcement_responses',(SELECT count(*) FROM users.parent_announcement_responses AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('users.parent_announcement_targets',(SELECT count(*) FROM users.parent_announcement_targets AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('users.parent_announcements',(SELECT count(*) FROM users.parent_announcements AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('users.parent_group_message_recipients',(SELECT count(*) FROM users.parent_group_message_recipients AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('users.parent_group_messages',(SELECT count(*) FROM users.parent_group_messages AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('users.parent_group_thread_reads',(SELECT count(*) FROM users.parent_group_thread_reads AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('users.parent_message_count_preferences',(SELECT count(*) FROM users.parent_message_count_preferences AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('users.parent_message_reads',(SELECT count(*) FROM users.parent_message_reads AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('users.parent_message_threads',(SELECT count(*) FROM users.parent_message_threads AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('users.parent_messages',(SELECT count(*) FROM users.parent_messages AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('users.parent_group_threads',(SELECT count(*) FROM users.parent_group_threads AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('users.parent_request_events',(SELECT count(*) FROM users.parent_request_events AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('users.parent_request_rss_feeds',(SELECT count(*) FROM users.parent_request_rss_feeds AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('users.parent_request_share_events',(SELECT count(*) FROM users.parent_request_share_events AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('users.persons',(SELECT count(*) FROM users.persons AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('users.privacy_consents',(SELECT count(*) FROM users.privacy_consents AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('users.profiles',(SELECT count(*) FROM users.profiles AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('users.rfid_cards',(SELECT count(*) FROM users.rfid_cards AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('users.staff_document_file_cleanup',(SELECT count(*) FROM users.staff_document_file_cleanup AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('users.staff_documents',(SELECT count(*) FROM users.staff_documents AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('users.staff_employment_profiles',(SELECT count(*) FROM users.staff_employment_profiles AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('users.staff_financial_data',(SELECT count(*) FROM users.staff_financial_data AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('users.staff_master_data',(SELECT count(*) FROM users.staff_master_data AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('users.staff_message_participants',(SELECT count(*) FROM users.staff_message_participants AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('users.staff_message_reads',(SELECT count(*) FROM users.staff_message_reads AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('users.staff_message_threads',(SELECT count(*) FROM users.staff_message_threads AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('users.staff_messages',(SELECT count(*) FROM users.staff_messages AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('users.staff_notice_acks',(SELECT count(*) FROM users.staff_notice_acks AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('users.staff_notices',(SELECT count(*) FROM users.staff_notices AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('users.staff_offboarding_cleanup',(SELECT count(*) FROM users.staff_offboarding_cleanup AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('users.staff_qualifications',(SELECT count(*) FROM users.staff_qualifications AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('users.staff_school_memberships',(SELECT count(*) FROM users.staff_school_memberships AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('users.student_care_exit_removals',(SELECT count(*) FROM users.student_care_exit_removals AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('users.student_care_exit_source_removals',(SELECT count(*) FROM users.student_care_exit_source_removals AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('users.student_care_exits',(SELECT count(*) FROM users.student_care_exits AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('users.student_care_profiles',(SELECT count(*) FROM users.student_care_profiles AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('users.student_companions',(SELECT count(*) FROM users.student_companions AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('users.student_data_change_requests',(SELECT count(*) FROM users.student_data_change_requests AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('users.student_document_file_cleanup',(SELECT count(*) FROM users.student_document_file_cleanup AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('users.student_documents',(SELECT count(*) FROM users.student_documents AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('users.student_family_protection_events',(SELECT count(*) FROM users.student_family_protection_events AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('users.student_guardian_pickup_permissions',(SELECT count(*) FROM users.student_guardian_pickup_permissions AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('users.student_guardian_relationships',(SELECT count(*) FROM users.student_guardian_relationships AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('users.student_notes',(SELECT count(*) FROM users.student_notes AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('users.student_profiles',(SELECT count(*) FROM users.student_profiles AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('users.student_school_memberships',(SELECT count(*) FROM users.student_school_memberships AS r WHERE r.tenant_id = p_school));
 v_counts := v_counts || jsonb_build_object('users.teachers',(SELECT count(*) FROM users.teachers AS r WHERE r.tenant_id = p_school));
 RETURN v_counts;
 END $body$;
`

const signaturesSchoolErasureResidualDownSQL = `
CREATE OR REPLACE FUNCTION platform.school_erasure_assert_empty(p_school_id BIGINT,v_child_keys JSONB)
 RETURNS VOID LANGUAGE plpgsql VOLATILE SECURITY INVOKER SET search_path = pg_catalog
 AS $body$
 BEGIN
   -- Generated rest checks include frozen keys for indirect children.
   IF EXISTS (
    SELECT 1 FROM active.activity_session_attendance AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM active.activity_sessions AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM active.attendance AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM active.combined_groups AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM active.excused_absence_requests AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM active.group_mappings AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM active.group_supervisors AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM active.groups AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM active.presence_backfill_batches AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM active.presence_backfill_checkpoints AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM active.scheduled_checkouts AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM active.staff_absence_audit AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM active.staff_absence_type_allowance_changes AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM active.staff_absence_type_allowances AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM active.staff_absence_types AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM active.staff_absences AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM active.staff_balance_adjustments AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM active.staff_month_balance_snapshots AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM active.staff_vacation_openings AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM active.staff_vacation_quota AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM active.staff_vacation_quota_changes AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM active.student_status_days AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM active.visits AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM active.work_session_breaks AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM active.work_sessions AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM activities.categories AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM activities.group_targets AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM activities.groups AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM activities.schedules AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM activities.student_enrollments AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM activities.supervisors AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM audit.attendance_corrections AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM audit.auth_events AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM audit.class_list_entry_changes AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM audit.data_access_log AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM audit.data_deletions AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM audit.data_imports AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM audit.deviation_events AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM audit.enrollment_deletions AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM audit.enrollment_offering_adjustments AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM audit.enrollment_restorations AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM audit.export_transfers AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM audit.file_events AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM audit.guardian_changes AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM audit.guardian_financial_changes AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM audit.personnel_number_changes AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM audit.room_color_migration_backup AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM audit.staff_master_data_changes AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM audit.student_consent_changes AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM audit.student_deletions AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM audit.student_field_edits AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM audit.substitution_changes AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM audit.time_tracking_deletions AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM audit.unregistered_tag_scans AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM audit.wc_alias_migration_backup AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM audit.work_session_edits AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM auth.account_permissions AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM auth.account_roles AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM auth.account_tenants AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM auth.guardian_invitations AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM auth.guardian_student_access AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM auth.invitation_tokens AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM auth.mfa_email_challenges AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM auth.mfa_overrides AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM auth.mfa_trusted_devices AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM auth.parent_declaration_permission_grants AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM auth.passkey_sessions AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM auth.role_permissions AS r WHERE to_jsonb(r."role_id") IN (SELECT value FROM jsonb_array_elements(v_child_keys->'auth.role_permissions'))
    UNION ALL
    SELECT 1 FROM auth.roles AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM auth.tokens AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM calendar.appointment_occurrence_overrides AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM calendar.appointment_recipient_students AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM calendar.appointment_recipients AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM calendar.appointment_reminder_push_deliveries AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM calendar.appointment_targets AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM calendar.appointments AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM calendar.recurrence_rules AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM calendar.staff_feed_tombstones AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM config.home_block_policies AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM config.home_layouts AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM config.school_setup_dismissals AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM config.school_setups AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM config.setting_audit AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM config.setting_values AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM config.staff_onboardings AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM config.staff_target_overrides AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM config.staff_work_schedules AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM config.work_time_model_entries AS r WHERE to_jsonb(r."model_id") IN (SELECT value FROM jsonb_array_elements(v_child_keys->'config.work_time_model_entries'))
    UNION ALL
    SELECT 1 FROM config.work_time_models AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM display.displays AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM documents.announcement_attachment_cleanup AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM documents.announcement_attachments AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM documents.file_cleanup AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM documents.files AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM documents.folder_accounts AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM documents.folder_roles AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM documents.folders AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM education.class_arrival_exceptions AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM education.class_arrival_times AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM education.class_teachers AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM education.grade_transition_class_list_entries AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM education.grade_transition_class_teachers AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM education.grade_transition_history AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM education.grade_transition_mappings AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM education.grade_transitions AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM education.group_substitution AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM education.group_teacher AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM education.groups AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM enrollment.care_offering_auto_triggers AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM enrollment.care_offering_bookings AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM enrollment.care_offerings AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM enrollment.change_request_messages AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM enrollment.change_requests AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM enrollment.form_schemas AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM enrollment.late_invites AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM enrollment.offering_change_requests AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM enrollment.phases AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM enrollment.request_child_offering_selections AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM enrollment.request_child_storage_backfill_checkpoints AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM enrollment.request_children AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM enrollment.request_guardians AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM enrollment.request_reads AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM enrollment.request_upload_cleanup AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM enrollment.request_uploads AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM enrollment.requests AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM enrollment.submission_rate_limits AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM facilities.rooms AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM feedback.entries AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM iot.devices AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM iot.push_subscriptions AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM iot.pwa_standalone_usage AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM meta.meal_participation_permission_grants AS r WHERE to_jsonb(r."student_guardian_id") IN (SELECT value FROM jsonb_array_elements(v_child_keys->'meta.meal_participation_permission_grants'))
    UNION ALL
    SELECT 1 FROM meta.parent_student_consent_permission_grants AS r WHERE to_jsonb(r."student_guardian_id") IN (SELECT value FROM jsonb_array_elements(v_child_keys->'meta.parent_student_consent_permission_grants'))
    UNION ALL
    SELECT 1 FROM platform.cleanup_runs AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM platform.demo_school_states AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM platform.email_delivery AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM platform.email_outbox AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM platform.push_outbox AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM platform.schools AS r WHERE r.id = p_school_id
    UNION ALL
    SELECT 1 FROM platform.storage_backfill_checkpoints AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM schedule.activity_exceptions AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM schedule.activity_instances AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM schedule.calendar_periods AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM schedule.care_schedule_change_requests AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM schedule.closing_days AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM schedule.dateframes AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM schedule.grade_transition_roster_removals AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM schedule.instance_staff AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM schedule.instance_students AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM schedule.meal_participation_overrides AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM schedule.meal_participation_schedules AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM schedule.meal_plan_entries AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM schedule.meal_sickness_status_history AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM schedule.pickup_extension_tasks AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM schedule.planning_tracks AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM schedule.recurrence_rules AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM schedule.shift_types AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM schedule.staff_shift_series AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM schedule.staff_shift_series_exceptions AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM schedule.staff_shifts AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM schedule.student_arrival_exceptions AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM schedule.student_arrival_notes AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM schedule.student_arrival_schedules AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM schedule.student_departure_exceptions AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM schedule.student_pickup_exceptions AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM schedule.student_pickup_notes AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM schedule.student_pickup_schedules AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM schedule.timeframes AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM schedule.timetable_conflict_acks AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM users.care_withdrawal_completions AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM users.class_list_entries AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM users.external_caregivers AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM users.guardian_financial_data AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM users.guardian_phone_numbers AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM users.guardian_profiles AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM users.notification_preferences AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM users.parent_announcement_declaration_submissions AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM users.parent_announcement_declaration_versions AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM users.parent_announcement_options AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM users.parent_announcement_reads AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM users.parent_announcement_responses AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM users.parent_announcement_targets AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM users.parent_announcements AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM users.parent_group_message_recipients AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM users.parent_group_messages AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM users.parent_group_thread_reads AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM users.parent_group_threads AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM users.parent_message_count_preferences AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM users.parent_message_reads AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM users.parent_message_threads AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM users.parent_messages AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM users.parent_request_events AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM users.parent_request_rss_feeds AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM users.parent_request_share_events AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM users.persons AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM users.privacy_consents AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM users.profiles AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM users.rfid_cards AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM users.staff_document_file_cleanup AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM users.staff_documents AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM users.staff_employment_profiles AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM users.staff_financial_data AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM users.staff_master_data AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM users.staff_message_participants AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM users.staff_message_reads AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM users.staff_message_threads AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM users.staff_messages AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM users.staff_notice_acks AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM users.staff_notices AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM users.staff_offboarding_cleanup AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM users.staff_qualifications AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM users.staff_school_memberships AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM users.student_care_exit_removals AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM users.student_care_exit_source_removals AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM users.student_care_exits AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM users.student_care_profiles AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM users.student_companions AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM users.student_data_change_requests AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM users.student_document_file_cleanup AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM users.student_documents AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM users.student_family_protection_events AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM users.student_guardian_pickup_permissions AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM users.student_guardian_relationships AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM users.student_notes AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM users.student_profiles AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM users.student_school_memberships AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM users.teachers AS r WHERE r.tenant_id = p_school_id
   ) THEN
    RAISE EXCEPTION 'school erasure left scoped rows' USING ERRCODE = '55000';
   END IF;
 END $body$;
 REVOKE ALL ON FUNCTION platform.school_erasure_assert_empty(BIGINT,JSONB) FROM PUBLIC, phoenix_auth, phoenix_tenant, phoenix_admin, phoenix_demo, phoenix_school_erasure;
`
