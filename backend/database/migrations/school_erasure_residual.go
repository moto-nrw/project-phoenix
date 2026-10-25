package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

// Internal static SQL phase of ADR 0092. No runtime role may call it
// directly; only the owning SECURITY DEFINER entry point orchestrates it.
func installSchoolErasureResidual(ctx context.Context, tx bun.Tx) error {
	_, err := tx.ExecContext(ctx, schoolErasureResidualSQL)
	return err
}

const schoolErasureResidualSQL = `CREATE OR REPLACE FUNCTION platform.school_erasure_assert_empty(p_school_id BIGINT,v_child_keys JSONB)
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
    SELECT 1 FROM active.class_handover_children AS r WHERE r.tenant_id = p_school_id
    UNION ALL
    SELECT 1 FROM active.class_handovers AS r WHERE r.tenant_id = p_school_id
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
    SELECT 1 FROM active.morning_attendance AS r WHERE r.tenant_id = p_school_id
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
    SELECT 1 FROM audit.signatures AS r WHERE r.tenant_id = p_school_id
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
