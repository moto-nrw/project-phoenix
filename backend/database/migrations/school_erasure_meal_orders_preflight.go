package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

// 1.15.505 (#3873): the installed preflight that also locks the meal
// ordering tables. Their foreign keys to platform.schools, to
// users.student_profiles and among themselves change the fingerprint.
func installMealOrderErasurePreflight(ctx context.Context, tx bun.Tx) error {
	_, err := tx.ExecContext(ctx, mealOrderErasurePreflightSQL)
	return err
}

const mealOrderErasurePreflightSQL = `CREATE OR REPLACE FUNCTION platform.school_erasure_preflight(p_school_id BIGINT,p_organization_id BIGINT)
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
    active.morning_attendance,
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
    schedule.meal_order_changes,
    schedule.meal_order_lines,
    schedule.meal_order_days,
    schedule.meal_orders,
    schedule.meal_registration_changes,
    auth.caterer_invitations,
    auth.caterer_schools,
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
    ) AS edge) <> '7f5e3da6c132db9507276c29fc9eb0c3bd033487c654f43170f79b0e007e9e8c' THEN
    RAISE EXCEPTION 'school erasure foreign keys changed; review plan' USING ERRCODE = '55000';
   END IF;
   -- Generated incoming-FK scope checks. Never cascade into foreign rows.
   IF EXISTS (
    SELECT 1 FROM active.excused_absence_requests AS c JOIN users.student_profiles AS p ON c."student_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM active.morning_attendance AS c JOIN users.class_list_entries AS p ON c."class_list_entry_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
    UNION ALL
    SELECT 1 FROM active.morning_attendance AS c JOIN users.staff_school_memberships AS p ON c."recorded_by_staff_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)
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
