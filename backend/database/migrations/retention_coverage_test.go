package migrations

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// The retention guard of #3952. Every table of the schema states how its rows
// leave the database, so no table with personal data appears without a
// deletion path. The retention list docs/reference/loeschfristen.md names the
// periods behind the paths.

type retentionClass string

const (
	// ownRetention: a job of the table's owner deletes expired rows; the
	// detail names the job.
	ownRetention retentionClass = "own retention"
	// withParent: the rows go with their parent row through ON DELETE
	// CASCADE; the detail names the parent table. The guard checks the
	// cascade, and the parent needs a deletion path or a gap of its own.
	withParent retentionClass = "with parent"
	// noPersonalData: the table holds no personal data; the detail says why.
	noPersonalData retentionClass = "no personal data"
	// controllerDeletes: content the school keeps as long as it needs it and
	// deletes itself as the controller, as the AVV states (#4045, ADR 0073); no window
	// fits rows without an end, such as a file in use or a child on a class
	// list. The detail names the school's deletion path and what goes
	// automatically.
	controllerDeletes retentionClass = "school deletes"
	// specGap: personal data still without a deletion path; the detail names
	// the open ticket of spec #3949 that adds it. The list only shrinks.
	specGap retentionClass = "spec gap"
	// openGap: personal data without a deletion path that no ticket of the
	// spec covers yet; the detail says what is kept. The list only shrinks.
	openGap retentionClass = "open gap"
)

type tableRetention struct {
	class  retentionClass
	detail string
}

func own(job string) tableRetention      { return tableRetention{class: ownRetention, detail: job} }
func parent(table string) tableRetention { return tableRetention{class: withParent, detail: table} }
func notPersonal(why string) tableRetention {
	return tableRetention{class: noPersonalData, detail: why}
}
func schoolDeletes(how string) tableRetention {
	return tableRetention{class: controllerDeletes, detail: how}
}

// openSpecTickets are the tickets of spec #3949 that still add a deletion
// path; a gap names its ticket as tableRetention{class: specGap, detail:
// ticket}. When a ticket lands, its tables move to own retention or with
// parent and the ticket leaves this list.
var openSpecTickets = []string{}

// The gap lists only shrink: closing a gap lowers its count, a new gap fails.
const (
	specGapTables = 0
	openGapTables = 0
	// schoolDeletesTables keeps the class of #4045 (ADR 0073) closed: a new
	// table with personal data gets a deletion path, not this class.
	schoolDeletesTables = 2
)

// staffRetention is the deletion path of a retired staff member's records
// (#3963). The membership stays as a tombstone that records of other people
// name, so its rows do not go through ON DELETE CASCADE: the nightly step
// deletes them through their owners.
func staffRetention(owner string) tableRetention {
	return own("nightly retention step staff_retention, Retention.DeleteExpiredStaff (school time-tracking window after the exit, #3963) -> " + owner)
}

// tableRetentions classifies every table of the schema.
var tableRetentions = map[string]tableRetention{
	"platform.school_erasures":                              own("administrative platform.delete_expired_school_erasures: completed evidence expires three years after completion; unresolved orders remain for recovery (#3837)"),
	"active.activity_session_attendance":                    parent("schedule.instance_students"),
	"active.activity_sessions":                              parent("schedule.activity_instances"),
	"active.attendance":                                     own("nightly retention step attendance_retention (api/worker_retention.go), Student Presence DeleteExpiredAttendance (school window gdpr.attendance_retention_days after the care day, #3957)"),
	"active.combined_groups":                                notPersonal("runtime group-combination configuration"),
	"active.excused_absence_requests":                       parent("users.student_profiles"),
	"active.group_mappings":                                 notPersonal("group-to-combination mapping"),
	"active.group_supervisors":                              staffRetention("Student Presence RemoveSupervision"),
	"active.groups":                                         notPersonal("runtime activity/room session rows, no subject column; CASCADE from activities.groups"),
	"active.presence_backfill_batches":                      notPersonal("progress of a data migration, counters and positions only"),
	"active.presence_backfill_checkpoints":                  notPersonal("progress of a data migration, counters and positions only"),
	"active.scheduled_checkouts":                            parent("users.student_profiles"),
	"active.staff_absence_audit":                            parent("active.staff_absences"),
	"active.staff_absence_type_allowance_changes":           staffRetention("Workforce SQL users.erase_retired_staff_records"),
	"active.staff_absence_type_allowances":                  staffRetention("Workforce SQL users.erase_retired_staff_records"),
	"active.staff_absence_types":                            notPersonal("absence type catalog"),
	"active.staff_absences":                                 own("nightly time-tracking-cleanup, CleanupExpiredTimeTrackingData -> staffAbsenceRepo.DeleteOlderThan(date_end)"),
	"active.staff_balance_adjustments":                      staffRetention("Workforce SQL users.erase_retired_staff_records"),
	"active.staff_month_balance_snapshots":                  staffRetention("Workforce SQL users.erase_retired_staff_records"),
	"active.staff_vacation_openings":                        staffRetention("Workforce SQL users.erase_retired_staff_records"),
	"active.staff_vacation_quota":                           staffRetention("Workforce SQL users.erase_retired_staff_records"),
	"active.staff_vacation_quota_changes":                   staffRetention("Workforce SQL users.erase_retired_staff_records"),
	"active.student_status_days":                            own("nightly retention step status_day_retention (api/worker_retention.go), DeleteStudentStatusDaysBefore"),
	"active.visits":                                         own("nightly visit-cleanup, CleanupExpiredVisits -> DeleteCompletedVisitsBefore"),
	"active.work_session_breaks":                            parent("active.work_sessions"),
	"active.work_sessions":                                  own("nightly time-tracking-cleanup, CleanupExpiredTimeTrackingData -> DeleteWorkSessionsOlderThan(date)"),
	"activities.categories":                                 notPersonal("activity category catalog"),
	"activities.group_targets":                              notPersonal("activity targeting configuration"),
	"activities.groups":                                     notPersonal("activity definitions (name, notes, capacity); only staff link is created_by FK (NO ACTION)"),
	"activities.schedules":                                  notPersonal("activity weekly schedule configuration"),
	"activities.student_enrollments":                        parent("users.student_profiles"),
	"activities.supervisors":                                staffRetention("Timetable capability DeletePlannedSupervisorsByStaff"),
	"audit.attendance_corrections":                          parent("users.student_profiles"),
	"audit.auth_events":                                     own("nightly retention step auth_event_retention, DeleteExpiredAuthEvents -> SQL audit.delete_expired_auth_events"),
	"audit.class_list_entry_changes":                        own("nightly retention step class_list_change_retention, Command.DeleteExpiredProtocol -> SQL audit.delete_expired_protocol (end of the following year)"),
	"audit.data_access_log":                                 own("nightly retention step data_access_log_retention, DeleteExpiredDataAccessLog -> SQL audit.delete_expired_data_access_log"),
	"audit.data_deletions":                                  own("nightly retention step deletion_log_retention, DeleteExpiredDeletionLog -> SQL audit.delete_expired_data_deletions"),
	"audit.data_imports":                                    parent("auth.accounts"),
	"audit.deviation_events":                                own("nightly timetable-cleanup, DeleteDeviationEventsBefore -> SQL audit.delete_expired_deviation_events"),
	"audit.enrollment_deletions":                            own("nightly retention step enrollment_deletion_log_retention, Command.DeleteExpiredProtocol -> SQL audit.delete_expired_protocol (3 years)"),
	"audit.enrollment_offering_adjustments":                 parent("users.student_profiles"),
	"audit.enrollment_restorations":                         own("nightly retention step enrollment_restoration_log_retention, Command.DeleteExpiredProtocol -> SQL audit.delete_expired_protocol (3 years)"),
	"audit.export_transfers":                                own("nightly retention step export_transfer_retention, JournalRetention.DeleteExpiredTransfers -> SQL audit.delete_expired_export_transfers (end of the following year)"),
	"audit.file_events":                                     own("nightly retention step file_event_retention, Command.DeleteExpiredProtocol -> SQL audit.delete_expired_protocol (end of the following year)"),
	"audit.guardian_changes":                                parent("users.student_profiles"),
	"audit.guardian_financial_changes":                      own("nightly retention step guardian_financial_change_retention, Command.DeleteExpiredProtocol -> SQL audit.delete_expired_protocol (3 years)"),
	"audit.personnel_number_changes":                        own("nightly retention step personnel_number_change_retention, Command.DeleteExpiredProtocol -> SQL audit.delete_expired_protocol (3 years)"),
	"audit.room_color_migration_backup":                     notPersonal("one-off migration backup of room names"),
	"audit.staff_master_data_changes":                       own("nightly retention step staff_master_data_change_retention, Command.DeleteExpiredProtocol -> SQL audit.delete_expired_protocol (3 years)"),
	"audit.student_consent_changes":                         parent("users.student_profiles"),
	"audit.student_deletions":                               own("nightly retention step student_deletion_log_retention, Command.DeleteExpiredProtocol -> SQL audit.delete_expired_protocol (3 years, like the deletion log)"),
	"audit.student_field_edits":                             own("nightly student-change-log-cleanup, CleanupExpiredChangeLog -> DeleteOlderThan -> SQL audit.delete_expired_student_field_edits"),
	"audit.substitution_changes":                            own("nightly retention step substitution_change_retention, Command.DeleteExpiredProtocol -> SQL audit.delete_expired_protocol (end of the following year)"),
	"audit.time_tracking_deletions":                         own("nightly retention step time_tracking_deletion_log_retention, Command.DeleteExpiredProtocol -> SQL audit.delete_expired_protocol (3 years)"),
	"audit.unregistered_tag_scans":                          own("nightly retention step unregistered_tag_scan_retention, unregisteredTagScanService.DeleteExpired -> SQL audit.delete_expired_unregistered_tag_scans"),
	"audit.wc_alias_migration_backup":                       notPersonal("one-off migration backup of alias names"),
	"audit.work_session_edits":                              parent("active.work_sessions"),
	"auth.account_permissions":                              parent("auth.accounts"),
	"auth.account_roles":                                    parent("auth.accounts"),
	"auth.account_tenants":                                  parent("auth.accounts"),
	"auth.accounts":                                         own("anonymized, not deleted, so messages, protocols and records of other people keep their reference: hourly token-cleanup, EraseUnusedAccounts -> SQL auth.erase_unused_accounts once no school needs the account (#4043)"),
	"auth.demo_accesses":                                    own("demo process only: hourly demoExpiry -> ExpireDemoAccesses -> DeleteExpiredDemoAccesses"),
	"auth.guardian_invitations":                             own("hourly token-cleanup, DeleteExpiredGuardianInvitations (30 days after accepted_at, else expires_at)"),
	"auth.guardian_student_access":                          parent("users.student_guardian_relationships"),
	"auth.invitation_tokens":                                own("hourly token-cleanup, DeleteExpiredSchoolInvitations (expires_at < now)"),
	"auth.mfa_credentials":                                  parent("auth.accounts"),
	"auth.mfa_email_challenges":                             own("hourly token-cleanup, DeleteExpiredMFAEmailChallenges (7 days after expires_at)"),
	"auth.mfa_overrides":                                    parent("auth.accounts"),
	"auth.mfa_trusted_devices":                              parent("auth.accounts"),
	"auth.parent_declaration_permission_grants":             parent("users.student_guardian_relationships"),
	"auth.passkey_credentials":                              parent("auth.accounts"),
	"auth.passkey_sessions":                                 own("hourly token-cleanup, DeleteExpiredPasskeySessions (7 days after expires_at)"),
	"auth.password_reset_rate_limits":                       own("hourly token-cleanup, DeleteStalePasswordResetWindows"),
	"auth.password_reset_tokens":                            own("hourly token-cleanup, DeleteSpentPasswordResetTokens (expired or used)"),
	"auth.permissions":                                      notPersonal("permission catalog"),
	"auth.role_permissions":                                 notPersonal("role-permission mapping"),
	"auth.roles":                                            notPersonal("role catalog"),
	"auth.tokens":                                           own("hourly token-cleanup, CleanupExpiredTokens -> DeleteExpiredAccountSessions (expiry < now) plus wipe of deactivated accounts' sessions"),
	"calendar.appointment_occurrence_overrides":             parent("calendar.appointments"),
	"calendar.appointment_recipient_students":               parent("users.student_profiles"),
	"calendar.appointment_recipients":                       parent("calendar.appointments"),
	"calendar.appointment_reminder_push_deliveries":         parent("users.guardian_profiles"),
	"calendar.appointment_targets":                          notPersonal("appointment target configuration"),
	"calendar.appointments":                                 own("nightly retention steps appointment_retention, AppointmentRetention.DeleteExpiredAppointments (school window gdpr.timetable_retention_days after the last day; endless series stay, #4045), and appointment_tombstone_retention, DeleteFeedTombstonesBefore (90 days after cancellation or deletion)"),
	"calendar.recurrence_rules":                             notPersonal("recurrence configuration"),
	"calendar.staff_feed_tombstones":                        own("nightly timetable-cleanup step CleanupExpiredFeedTombstones -> StaffFeedTombstoneRepo.DeleteFeedCancellationsBefore (90 days)"),
	"config.home_block_policies":                            notPersonal("home dashboard block policy configuration"),
	"config.home_layouts":                                   parent("auth.accounts"),
	"config.school_setup_dismissals":                        parent("auth.accounts"),
	"config.school_setups":                                  notPersonal("school setup wizard state"),
	"config.setting_audit":                                  own("nightly retention step setting_audit_retention, AuditRetention.DeleteExpiredSettingChanges -> SettingAuditRetention.DeleteChangedBefore (3 years); changed_by SET NULL on account delete"),
	"config.setting_values":                                 notPersonal("tenant setting values (only updated-by account reference, FK NO ACTION)"),
	"config.staff_onboardings":                              parent("auth.accounts"),
	"config.staff_target_overrides":                         staffRetention("Workforce SQL users.erase_retired_staff_records"),
	"config.staff_work_schedules":                           staffRetention("Workforce SQL users.erase_retired_staff_records"),
	"config.work_time_model_entries":                        notPersonal("work time model configuration"),
	"config.work_time_models":                               notPersonal("work time model catalog"),
	"display.displays":                                      notPersonal("display device registry (name, token hash)"),
	"documents.announcement_attachment_cleanup":             own("nightly retention step file_cleanup_retention, DeleteFinishedCleanups -> SQL documents.delete_finished_announcement_attachment_cleanups (30 days after cleaned_at)"),
	"documents.announcement_attachments":                    parent("users.parent_announcements"),
	"documents.file_cleanup":                                own("nightly retention step file_cleanup_retention, DeleteFinishedCleanups -> SQL documents.delete_finished_file_cleanups (30 days after cleaned_at)"),
	"documents.files":                                       schoolDeletes("the school's file storage: a file in use stays until the school deletes it (Service.DeleteFile, DeleteFolder); the removed row goes in the nightly retention step removed_file_retention -> SQL documents.delete_removed_files (30 days after its bytes, #4045)"),
	"documents.folder_accounts":                             parent("auth.accounts"),
	"documents.folder_roles":                                notPersonal("folder audience by role"),
	"documents.folders":                                     notPersonal("school folder names"),
	"education.class_arrival_exceptions":                    notPersonal("class-level arrival exceptions"),
	"education.class_arrival_times":                         notPersonal("class-level arrival times"),
	"education.class_teachers":                              staffRetention("School Membership EraseNextRetiredStaff"),
	"education.grade_transition_class_list_entries":         parent("education.grade_transitions"),
	"education.grade_transition_class_teachers":             parent("education.grade_transitions"),
	"education.grade_transition_history":                    parent("education.grade_transitions"),
	"education.grade_transition_mappings":                   notPersonal("class-to-class mapping"),
	"education.grade_transitions":                           own("nightly retention step grade_transition_retention, GradeTransitionRetention.DeleteExpiredGradeTransitions (school window gdpr.student_retention_days after application, reversal or a draft's last change, #4045)"),
	"education.group_substitution":                          staffRetention("Workforce SQL users.erase_retired_staff_records"),
	"education.group_teacher":                               staffRetention("School Membership EraseNextRetiredStaff"),
	"education.groups":                                      notPersonal("class/group catalog"),
	"enrollment.care_offering_auto_triggers":                notPersonal("offering trigger configuration"),
	"enrollment.care_offering_bookings":                     parent("enrollment.request_children"),
	"enrollment.care_offerings":                             notPersonal("care offering catalog"),
	"enrollment.change_request_messages":                    parent("enrollment.change_requests"),
	"enrollment.change_requests":                            parent("enrollment.requests"),
	"enrollment.form_schemas":                               notPersonal("enrollment form definitions"),
	"enrollment.late_invites":                               own("nightly retention step settled_enrollment_retention (api/worker_retention.go), DeleteLateInvitesSettledBefore; used invites go with their request (DeleteRequestTree)"),
	"enrollment.offering_change_requests":                   parent("users.student_profiles"),
	"enrollment.phases":                                     notPersonal("enrollment phase configuration"),
	"enrollment.request_child_offering_selections":          parent("enrollment.request_children"),
	"enrollment.request_child_storage_backfill_checkpoints": notPersonal("progress of a data migration, counters only"),
	"enrollment.request_children":                           parent("enrollment.requests"),
	"enrollment.request_guardians":                          parent("enrollment.requests"),
	"enrollment.request_reads":                              parent("auth.accounts"),
	"enrollment.request_upload_cleanup":                     own("nightly retention step enrollment_upload_retention, RequestUploads.DeleteExpiredUploads -> DeleteFinishedUploadCleanups (30 days after cleaned_at); the file store cleanup tick completes the intents"),
	"enrollment.request_uploads":                            own("nightly retention step enrollment_upload_retention, RequestUploads.DeleteExpiredUploads (unclaimed after 24 hours, a decided request's files after enrollment.upload_retention_days); also go with their request or phase"),
	"enrollment.requests":                                   own("nightly retention step settled_enrollment_retention (withdrawn and lapsed, CleanupSettledEnrollments) and the rejected cleanup (CleanupRejectedEnrollments); accepted requests go with the child (#3962)"),
	"enrollment.submission_rate_limits":                     own("nightly retention step submission_rate_limit_retention, DeleteExpiredSubmissionRateLimits -> Store.CleanupExpired (24 hours after window_start)"),
	"facilities.rooms":                                      notPersonal("room catalog"),
	"feedback.entries":                                      own("nightly visit-cleanup run, FeedbackCleaner.DeleteExpired -> Cleanup -> DeleteOlderThan"),
	"iot.devices":                                           notPersonal("device registry (registered_by person FK SET NULL)"),
	"iot.push_subscriptions":                                own("hourly token-cleanup reconcile, DeleteOrphanedSubscriptions (token family gone)"),
	"iot.pwa_standalone_usage":                              own("nightly pwa-usage-cleanup, CleanupExpiredUsage -> SQL iot.delete_pwa_standalone_usage_before"),
	"meta.meal_participation_permission_grants":             parent("users.student_guardian_relationships"),
	"meta.migration_metadata":                               notPersonal("schema bookkeeping"),
	"meta.parent_student_consent_permission_grants":         parent("users.student_guardian_relationships"),
	"platform.announcement_views":                           parent("auth.accounts"),
	"platform.announcements":                                notPersonal("operator release announcements"),
	"platform.billing_key_date_counts":                      own("platform.delete_expired_billing_key_date_counts: 10 years after the end of the key_date calendar year, restricted billing evidence (ADR 0092)"),
	"platform.billing_settings":                             notPersonal("billing configuration"),
	"platform.cleanup_runs":                                 notPersonal("ledger of last successful cleanup per school and job"),
	"platform.demo_school_states":                           own("demo process only: hourly demoExpiry -> RetireDemoSchools -> RetireMany -> SQL platform.erase_closed_demo_visitors (closed order without demo access; also replaces the visitor's name in the hidden demo schools)"),
	"platform.email_delivery":                               own("nightly retention step delivery_retention, DeleteExpiredDeliveries (30 days after its mail ended, else after created_at)"),
	"platform.email_outbox":                                 own("nightly retention step delivery_retention, DeleteExpiredDeliveries (30 days after sent, dead letter or cancelled; guardian welcome keeps only its key)"),
	"platform.operator_audit_log":                           own("hourly token-cleanup, operator audit log retention, DeleteExpiredOperatorAuditLog (before 1 January of last year)"),
	"platform.operator_email_change_tokens":                 own("hourly token-cleanup, CleanupOperatorEmailChanges -> DeleteStaleOperatorEmailChanges"),
	"platform.operator_invitation_tokens":                   own("hourly token-cleanup, DeleteExpiredOperatorInvitations"),
	"platform.operator_mfa_credentials":                     parent("platform.operators"),
	"platform.operator_mfa_email_challenges":                own("hourly token-cleanup, DeleteExpiredMFAEmailChallenges (7 days after expires_at)"),
	"platform.operator_mfa_trusted_devices":                 parent("platform.operators"),
	"platform.operator_passkey_credentials":                 parent("platform.operators"),
	"platform.operator_passkey_sessions":                    own("hourly token-cleanup, DeleteExpiredPasskeySessions (7 days after expires_at)"),
	"platform.operator_refresh_tokens":                      own("hourly token-cleanup, DeleteExpiredOperatorSessions (expiry < now)"),
	"platform.operators":                                    own("hourly token-cleanup, deactivated operator cleanup, DeleteDeactivatedOperators (deactivated before 1 January of last year)"),
	"platform.organizations":                                notPersonal("organization (Träger) master data"),
	"platform.push_outbox":                                  own("nightly retention step delivery_retention, DeleteExpiredDeliveries (30 days after sent, dead letter or cancelled)"),
	"platform.schools":                                      notPersonal("school institution master data"),
	"platform.storage_backfill_checkpoints":                 notPersonal("progress of a data migration, counters only"),
	"platform.worker_leases":                                notPersonal("Worker leadership lease"),
	"public.bun_migration_locks":                            notPersonal("schema bookkeeping"),
	"public.bun_migrations":                                 notPersonal("schema bookkeeping"),
	"schedule.activity_exceptions":                          own("nightly timetable-cleanup, DeleteActivityExceptionsBefore"),
	"schedule.activity_instances":                           own("nightly timetable-cleanup, CleanupExpiredTimetableData -> DeleteActivityInstancesBefore"),
	"schedule.calendar_periods":                             notPersonal("calendar period catalog"),
	"schedule.care_schedule_change_requests":                parent("users.student_profiles"),
	"schedule.closing_days":                                 notPersonal("closing day calendar"),
	"schedule.dateframes":                                   notPersonal("date frame catalog"),
	"schedule.grade_transition_roster_removals":             parent("users.student_profiles"),
	"schedule.instance_staff":                               parent("schedule.activity_instances"),
	"schedule.instance_students":                            parent("schedule.activity_instances"),
	"schedule.meal_participation_overrides":                 parent("users.student_profiles"),
	"schedule.meal_participation_schedules":                 parent("users.student_profiles"),
	"schedule.meal_plan_entries":                            notPersonal("meal plan"),
	"schedule.meal_sickness_status_history":                 parent("users.student_profiles"),
	"schedule.pickup_extension_tasks":                       parent("users.student_profiles"),
	"schedule.planning_tracks":                              notPersonal("planning track catalog"),
	"schedule.recurrence_rules":                             notPersonal("recurrence configuration"),
	"schedule.shift_types":                                  notPersonal("shift type catalog"),
	"schedule.staff_shift_series":                           staffRetention("Workforce SQL users.erase_retired_staff_records"),
	"schedule.staff_shift_series_exceptions":                parent("schedule.staff_shift_series"),
	"schedule.staff_shifts":                                 staffRetention("Workforce SQL users.erase_retired_staff_records"),
	"schedule.student_arrival_exceptions":                   parent("users.student_profiles"),
	"schedule.student_arrival_notes":                        parent("users.student_profiles"),
	"schedule.student_arrival_schedules":                    parent("users.student_profiles"),
	"schedule.student_departure_exceptions":                 parent("users.student_profiles"),
	"schedule.student_pickup_exceptions":                    parent("users.student_profiles"),
	"schedule.student_pickup_notes":                         parent("users.student_profiles"),
	"schedule.student_pickup_schedules":                     parent("users.student_profiles"),
	"schedule.timeframes":                                   notPersonal("timeframe catalog"),
	"schedule.timetable_conflict_acks":                      parent("auth.accounts"),
	"users.care_withdrawal_completions":                     own("nightly retention step care_withdrawal_retention, DeleteFinishedWithdrawalCompletions (365 days after resolved_at; pending ones stay)"),
	"users.class_list_entries":                              schoolDeletes("children without care on the school's class lists: the school removes a child that left (RemoveClassListEntry) and the grade transition removes graduating classes; no window fits a child still at the school (#4045)"),
	"users.guardian_financial_data":                         parent("users.guardian_profiles"),
	"users.guardian_phone_numbers":                          parent("users.guardian_profiles"),
	"users.guardian_profiles":                               own("nightly retention step guardian_retention (api/worker_retention.go), People Directory DeleteExpiredChildlessGuardians (school window gdpr.guardian_retention_days after childless_since, #4043)"),
	"users.external_caregivers":                             staffRetention("School Membership EraseNextRetiredStaff"),
	"users.notification_preferences":                        parent("auth.accounts"),
	"users.parent_announcement_declaration_submissions":     parent("users.student_profiles"),
	"users.parent_announcement_declaration_versions":        parent("users.parent_announcements"),
	"users.parent_announcement_options":                     notPersonal("announcement answer options"),
	"users.parent_announcement_reads":                       parent("auth.accounts"),
	"users.parent_announcement_responses":                   parent("users.student_profiles"),
	"users.parent_announcement_targets":                     notPersonal("announcement target configuration"),
	"users.parent_announcements":                            own("nightly retention step parent_announcement_retention, ParentAnnouncementRetention.DeleteExpiredParentAnnouncements (school window gdpr.parent_message_retention_days after expiry, publication or a draft's last change; a declaration with submissions stays until they go with their child, #4045)"),
	"users.parent_message_count_preferences":                parent("auth.accounts"),
	"users.parent_message_reads":                            parent("users.parent_message_threads"),
	"users.parent_message_threads":                          own("nightly retention step parent_message_retention deletes threads left empty (deleteEmptyParentThreadsSQL)"),
	"users.parent_messages":                                 own("nightly retention step parent_message_retention, DeleteMessagesBefore (open requests kept)"),
	"users.parent_request_events":                           parent("users.student_profiles"),
	"users.parent_request_rss_feeds":                        parent("auth.accounts"),
	"users.parent_request_share_events":                     parent("users.student_profiles"),
	"users.persons":                                         own("anonymized, not deleted, so records of other people keep their reference: the student deletion (#3962) and the nightly retention step staff_retention (#3963) -> AnonymizeRetiredStaffPerson replace name, birthday, tag and account"),
	"users.privacy_consents":                                parent("users.student_profiles"),
	"users.profiles":                                        parent("auth.accounts"),
	"users.rfid_cards":                                      notPersonal("RFID tag registry"),
	"users.staff_document_file_cleanup":                     own("nightly retention step staff_cleanup_retention, DeleteFinishedStaffCleanups -> SQL users.delete_finished_staff_document_file_cleanups (30 days after cleaned_at)"),
	"users.staff_documents":                                 own("sick notes: nightly retention step staff_sick_note_retention (#3961); every other category: staff_retention -> users.erase_retired_staff_records after the exit (#3963)"),
	"users.staff_employment_profiles":                       staffRetention("Workforce SQL users.erase_retired_staff_records"),
	"users.staff_financial_data":                            staffRetention("Workforce SQL users.erase_retired_staff_records"),
	"users.staff_master_data":                               staffRetention("Workforce SQL users.erase_retired_staff_records"),
	"users.staff_message_participants":                      parent("users.staff_message_threads"),
	"users.staff_message_reads":                             parent("users.staff_message_threads"),
	"users.staff_message_threads":                           own("nightly staff-message-cleanup, CleanupExpiredMessages -> ThreadStore.DeleteEmpty"),
	"users.staff_messages":                                  own("nightly staff-message-cleanup, CleanupExpiredMessages -> MessageStore.DeleteOlderThan"),
	"users.staff_notice_acks":                               parent("auth.accounts"),
	"users.staff_notices":                                   own("nightly retention step staff_notice_retention, StaffNoticeRetention.DeleteExpiredStaffNotices (school window gdpr.staff_message_retention_days after the last valid day or the switch-off; an active open-ended notice stays, #4045)"),
	"users.staff_offboarding_cleanup":                       own("nightly retention step staff_cleanup_retention, DeleteFinishedStaffCleanups -> DeleteCompletedOffboardingCleanups (30 days after completed_at)"),
	"users.staff_qualifications":                            staffRetention("Workforce SQL users.erase_retired_staff_records"),
	"users.staff_school_memberships":                        staffRetention("School Membership EraseNextRetiredStaff erases the records; the tombstone keeps only IDs that records of other people name, the person is anonymized"),
	"users.student_care_exit_removals":                      own("activate-students job, CareExitEffector.ApplyDueEffects -> discardRemovals -> DiscardCareExitRemovals when the exit takes effect"),
	"users.student_care_exit_source_removals":               own("activate-students job, ApplyDueEffects -> DiscardCareExitRemovals"),
	"users.student_care_exits":                              parent("users.student_profiles"),
	"users.student_care_profiles":                           parent("users.student_school_memberships"),
	"users.student_companions":                              parent("users.student_profiles"),
	"users.student_data_change_requests":                    parent("users.student_profiles"),
	"users.student_document_file_cleanup":                   own("nightly retention step student_document_cleanup_retention, DeleteFinishedStudentDocumentCleanups -> SQL users.delete_finished_student_document_file_cleanups (30 days after cleaned_at)"),
	"users.student_documents":                               parent("users.student_profiles"),
	"users.student_family_protection_events":                parent("users.student_profiles"),
	"users.student_guardian_pickup_permissions":             parent("users.student_guardian_relationships"),
	"users.student_guardian_relationships":                  parent("users.student_profiles"),
	"users.student_notes":                                   parent("users.student_profiles"),
	"users.student_profiles":                                own("nightly retention step student_retention, Retention.DeleteExpiredStudents -> student deletion workflow (school window after the last care day, #3962)"),
	"users.student_school_memberships":                      parent("users.student_profiles"),
	"users.teachers":                                        staffRetention("School Membership EraseNextRetiredStaff"),
}

func TestEveryTableHasARetentionPath(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	var tables []string
	require.NoError(t, db.NewRaw(`
		SELECT n.nspname || '.' || c.relname
		FROM pg_class AS c
		JOIN pg_namespace AS n ON n.oid = c.relnamespace
		WHERE c.relkind IN ('r', 'p')
		  AND NOT c.relispartition
		  AND n.nspname NOT IN ('pg_catalog', 'information_schema', 'pg_toast')
		ORDER BY 1`).Scan(ctx, &tables))

	var problems []string
	for _, table := range tables {
		if _, ok := tableRetentions[table]; !ok {
			problems = append(problems, fmt.Sprintf("%s has no deletion path, no reasoned exception and no gap entry", table))
		}
	}
	for table, retention := range tableRetentions {
		if !slices.Contains(tables, table) {
			problems = append(problems, fmt.Sprintf("%s is listed but no such table exists", table))
		}
		if retention.detail == "" {
			problems = append(problems, fmt.Sprintf("%s (%s) needs a non-empty reason", table, retention.class))
		}
		problems = append(problems, checkRetentionEntry(ctx, t, db, table, retention)...)
	}
	problems = append(problems, checkOpenSpecTickets()...)
	problems = append(problems, checkGapBudget(specGap, specGapTables)...)
	problems = append(problems, checkGapBudget(openGap, openGapTables)...)
	problems = append(problems, checkGapBudget(controllerDeletes, schoolDeletesTables)...)

	sort.Strings(problems)
	require.Empty(t, problems, "retention coverage: classify each table in tableRetentions and the retention list docs/reference/loeschfristen.md")
}

func checkRetentionEntry(ctx context.Context, t *testing.T, db *testpkg.DB, table string, retention tableRetention) []string {
	t.Helper()
	switch retention.class {
	case ownRetention, noPersonalData, controllerDeletes, openGap:
		return nil
	case specGap:
		if !slices.Contains(openSpecTickets, retention.detail) {
			return []string{fmt.Sprintf("%s names %q, which is no open ticket of spec #3949", table, retention.detail)}
		}
		return nil
	case withParent:
		parentEntry, ok := tableRetentions[retention.detail]
		if !ok {
			return []string{fmt.Sprintf("%s goes with %s, which is not classified", table, retention.detail)}
		}
		if parentEntry.class == noPersonalData {
			return []string{fmt.Sprintf("%s goes with %s, which never deletes personal data", table, retention.detail)}
		}
		var cascades bool
		require.NoError(t, db.NewRaw(`
			SELECT EXISTS (
				SELECT 1 FROM pg_constraint
				WHERE contype = 'f' AND confdeltype = 'c'
				  AND conrelid = ?::regclass AND confrelid = ?::regclass
			)`, table, retention.detail).Scan(ctx, &cascades))
		if !cascades {
			return []string{fmt.Sprintf("%s claims to go with %s, but no ON DELETE CASCADE foreign key connects them", table, retention.detail)}
		}
		return nil
	default:
		return []string{fmt.Sprintf("%s has the unknown class %q", table, retention.class)}
	}
}

// checkOpenSpecTickets keeps the gap list equal to the open tickets: a
// ticket without a gap table is done and leaves openSpecTickets.
func checkOpenSpecTickets() []string {
	var problems []string
	for _, ticket := range openSpecTickets {
		found := false
		for _, retention := range tableRetentions {
			if retention.class == specGap && retention.detail == ticket {
				found = true
				break
			}
		}
		if !found {
			problems = append(problems, fmt.Sprintf("%s has no gap table left; remove it from openSpecTickets", ticket))
		}
	}
	return problems
}

// checkGapBudget keeps a gap list at its recorded size: it may only shrink,
// and the recorded size follows it down.
func checkGapBudget(class retentionClass, budget int) []string {
	count := 0
	for _, retention := range tableRetentions {
		if retention.class == class {
			count++
		}
	}
	switch {
	case count > budget:
		return []string{fmt.Sprintf("%d %s tables, more than the %d recorded; give the new table a deletion path instead", count, class, budget)}
	case count < budget:
		return []string{fmt.Sprintf("%d %s tables, fewer than the %d recorded; lower the recorded count", count, class, budget)}
	}
	return nil
}
