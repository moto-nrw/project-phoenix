package migrations

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"testing"

	testpkg "github.com/moto-ogs/project-phoenix/test"
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
	"auth.organization_account_roles":                       parent("auth.accounts"),
	"auth.traeger_parent_access":                            own("the parents-portal access at a Träger (ADR 0216 point 10) goes with platform.organizations when the Träger is erased (#4657) and with auth.accounts by cascade; auth.account_is_unused counts it only at an active, undeleted Träger and only for the 30 days of the setup link (1.15.944) or while a holiday registration at that Träger names the account or its unclaimed address (1.15.945), so the account follows the rules of parent accounts"),
	"auth.traeger_invitations":                              own("hourly token-cleanup, DeleteExpiredSchoolInvitations also deletes expired Träger invitations (expires_at < now, #4650)"),
	"audit.traeger_account_changes":                         own("hourly token-cleanup, operator audit log retention, DeleteExpiredOperatorAuditLog also deletes Träger account ledger entries (before 1 January of last year, #4650)"),
	"audit.traeger_document_downloads":                      own("hourly token-cleanup, operator audit log retention, DeleteExpiredOperatorAuditLog also deletes Träger document download ledger entries (before 1 January of last year, #4692, ADR 0157); rows go with their document and with platform.organizations; actor_account_id CASCADE on account delete"),
	"care_contract.contract_documents":                      parent("care_contract.contracts"),
	"care_contract.contract_events":                         own("the trail of the contracts of a Träger keeps every read and write, also of a discarded draft (ADR 0153); it goes with platform.organizations when the Träger is erased (#4657); account_id SET NULL on account delete; the period follows with #4720"),
	"care_contract.contract_mailings":                       parent("care_contract.contract_documents"),
	"care_contract.contract_notices":                        parent("care_contract.contracts"),
	"care_contract.contract_parties":                        parent("care_contract.contracts"),
	"care_contract.contract_reminders":                      parent("care_contract.contracts"),
	"care_contract.contract_signatures":                     parent("care_contract.contract_versions"),
	"care_contract.contract_signing_links":                  parent("care_contract.contracts"),
	"care_contract.contract_templates":                      notPersonal("contract terms of one offering of a Träger (#4678): term, notice, special termination, statute and contracting party, no person"),
	"care_contract.contract_version_services":               parent("care_contract.contract_versions"),
	"care_contract.contract_versions":                       parent("care_contract.contracts"),
	"care_contract.contracts":                               own("a care contract stays when the application and the child it came from go (ADR 0088 section 9, ADR 0153); a draft goes when the Träger-Büro discards it; every contract goes with platform.organizations when the Träger is erased after its export (#4657); the contracts of a retired demo school go with its visitor, SQL platform.erase_closed_demo_visitors (1.15.786); the hourly Beleg retention deletes an ended contract after the period its Träger instructed, once no mandate or debit hangs on it (CareContract.DeleteEndedContracts, #4720, ADR 0184); without an instruction it waits on the clarification list"),
	"platform.organization_accounts":                        parent("auth.accounts"),
	"platform.organization_erasure_orders":                  own("administrative platform.delete_expired_organization_erasure_orders: completed and withdrawn orders expire three years after completion or withdrawal; an open order remains until it is withdrawn or completed (#4657)"),
	"platform.data_return_grants":                           own("administrative tenant return purge: the archive at the end of the retrieval period or on revocation, the rows three years after the retrieval period (ADR 0203)"),
	"platform.data_return_downloads":                        parent("platform.data_return_grants"),
	"platform.school_erasures":                              own("administrative platform.delete_expired_school_erasures: completed evidence expires three years after completion; unresolved orders remain for recovery (#3837)"),
	"active.activity_session_attendance":                    parent("schedule.instance_students"),
	"active.activity_sessions":                              parent("schedule.activity_instances"),
	"active.attendance":                                     own("nightly retention step attendance_retention (api/worker_retention.go), Student Presence DeleteExpiredAttendance (school window gdpr.attendance_retention_days after the care day, #3957)"),
	"active.class_handover_children":                        parent("active.class_handovers"),
	"active.class_handovers":                                own("nightly retention step morning_attendance_retention (roots/owners/worker_retention.go), Student Presence DeleteExpiredMorningAttendance deletes the handovers with the morning attendance (school window gdpr.morning_attendance_retention_days after the school day, #4794)"),
	"active.combined_groups":                                notPersonal("runtime group-combination configuration"),
	"active.excused_absence_requests":                       parent("users.student_profiles"),
	"active.group_mappings":                                 notPersonal("group-to-combination mapping"),
	"active.group_supervisors":                              staffRetention("Student Presence RemoveSupervision"),
	"active.groups":                                         notPersonal("runtime activity/room session rows, no subject column; CASCADE from activities.groups"),
	"active.morning_attendance":                             own("nightly retention step morning_attendance_retention (roots/owners/worker_retention.go), Student Presence DeleteExpiredMorningAttendance (school window gdpr.morning_attendance_retention_days after the school day, #4790)"),
	"active.presence_backfill_batches":                      notPersonal("progress of a data migration, counters and positions only"),
	"active.presence_backfill_checkpoints":                  notPersonal("progress of a data migration, counters and positions only"),
	"active.scheduled_checkouts":                            parent("users.student_profiles"),
	"active.staff_absence_audit":                            parent("active.staff_absences"),
	"active.staff_absence_type_allowance_changes":           staffRetention("Workforce SQL users.erase_retired_staff_records"),
	"active.staff_absence_type_allowances":                  staffRetention("Workforce SQL users.erase_retired_staff_records"),
	"active.staff_absence_types":                            notPersonal("absence type catalog"),
	"active.staff_absences":                                 own("nightly time-tracking-cleanup, CleanupExpiredTimeTrackingData -> staffAbsenceRepo.DeleteOlderThan(date_end)"),
	"active.staff_balance_adjustments":                      staffRetention("Workforce SQL users.erase_retired_staff_records"),
	"active.payroll_dispatches":                             own("payroll-dispatch job, PayrollDispatch.RunDue -> PurgePayrollDispatches (file data when the link expires, the row 400 days after it finished)"),
	"active.payroll_surcharge_rules":                        notPersonal("payroll surcharge rule catalog of the school"),
	"active.payroll_wage_types":                             notPersonal("payroll wage type catalog of the school"),
	"active.staff_month_balance_snapshots":                  staffRetention("Workforce SQL users.erase_retired_staff_records"),
	"active.staff_vacation_openings":                        staffRetention("Workforce SQL users.erase_retired_staff_records"),
	"active.staff_vacation_quota":                           staffRetention("Workforce SQL users.erase_retired_staff_records"),
	"active.staff_vacation_quota_changes":                   staffRetention("Workforce SQL users.erase_retired_staff_records"),
	"active.student_status_days":                            own("nightly retention step status_day_retention (api/worker_retention.go), DeleteStudentStatusDaysBefore"),
	"active.visit_revision_events":                          notPersonal("change counters of a session's open-visit count (#5128), session id only; the nightly close compacts ended sessions and drops deleted ones"),
	"active.visits":                                         own("nightly visit-cleanup, CleanupExpiredVisits -> DeleteExpiredVisits"),
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
	"audit.traeger_export_transfers":                        own("the journal of the handovers of a Träger stays with the export protocol whose files it documents (ADR 0218 section 8); it goes with platform.organizations when the Träger is erased (#4657); actor_account_id SET NULL on account delete"),
	"config.traeger_export_recipients":                      own("a recipient of a Träger is removed by the Träger itself (DELETE /traeger/export-recipient/{id}, ADR 0218, ADR 0222) and goes with platform.organizations when the Träger is erased (#4657); updated_by_account_id SET NULL on account delete"),
	"config.traeger_export_recipient_formats":               own("the format of a Träger goes with its recipient (ON DELETE CASCADE) or back to the download when the Träger reassigns it (PUT /traeger/export-recipient/formats, ADR 0222), and with platform.organizations when the Träger is erased (#4657); updated_by_account_id SET NULL on account delete"),
	"audit.export_transfers":                                own("nightly retention step export_transfer_retention, JournalRetention.DeleteExpiredTransfers -> SQL audit.delete_expired_export_transfers (end of the following year)"),
	"audit.file_events":                                     own("nightly retention step file_event_retention, Command.DeleteExpiredProtocol -> SQL audit.delete_expired_protocol (end of the following year)"),
	"audit.guardian_changes":                                parent("users.student_profiles"),
	"audit.signatures":                                      parent("users.student_profiles"),
	"audit.traeger_signatures":                              parent("care_contract.contract_versions"),
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
	"auth.account_avatar_file_cleanup":                      own("hourly CleanupErasedAccountAvatars removes each intent immediately after file deletion or confirmed absence; failures remain for retry (#4756)"),
	"auth.account_roles":                                    parent("auth.accounts"),
	"auth.account_tenants":                                  parent("auth.accounts"),
	"auth.accounts":                                         own("anonymized, not deleted, so messages, protocols and records of other people keep their reference: hourly token-cleanup, EraseUnusedAccounts -> SQL auth.erase_unused_accounts once no school needs the account (#4043)"),
	"auth.demo_accesses":                                    own("demo process only: hourly demoExpiry -> ExpireDemoAccesses -> DeleteExpiredDemoAccesses"),
	"auth.caterer_accounts":                                 parent("auth.caterers"),
	"auth.caterer_schools":                                  notPersonal("supply of a school: caterer, the school's delivery address, period; the accounts are cleared with the account"),
	"auth.caterer_invitations":                              own("hourly token-cleanup, DeleteExpiredCatererInvitations (30 days after accepted_at, revoked_at, else expires_at)"),
	"auth.caterers":                                         own("hourly token-cleanup after the account erasure, DeleteUnusedCaterers -> SQL auth.delete_unused_caterers: no supply (running or ended), no account left unerased and no open invitation to their addresses; the account links go with it"),
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
	"calendar.holiday_closure_authorizations":               parent("calendar.holiday_closure_events"),
	"calendar.holiday_closure_events":                       parent("calendar.holiday_closure_proposals"),
	"calendar.holiday_closure_proposals":                    own("minute holiday-closure-notifications job, RetireHolidayClosureEvents: 30 days after a resolved command or the last day of an unconfirmed proposal; unresolved commands stay"),
	"calendar.holiday_closure_revisions":                    notPersonal("revision counter of a school's closing days (ADR 0196)"),
	"calendar.parent_conference_bookings":                   parent("calendar.parent_conferences"),
	"calendar.parent_conferences":                           own("nightly retention step parent_conference_retention, ParentConferences.DeleteParentConferencesBefore (days before the start of the running school year on 1 August, with their bookings; the booked appointments go through AppointmentRetention.DeleteParentConferenceAppointmentsBefore, #4801)"),
	"calendar.recurrence_rules":                             notPersonal("recurrence configuration"),
	"calendar.school_vacation_acknowledgements":             notPersonal("a school's decision to plan care in one vacation window (#5345); only the acknowledging account reference, ON DELETE SET NULL"),
	"calendar.school_vacations":                             notPersonal("public school vacations per federal state; the daily sync replaces them (#5345)"),
	"calendar.staff_feed_tombstones":                        own("nightly timetable-cleanup step CleanupExpiredFeedTombstones -> StaffFeedTombstoneRepo.DeleteFeedCancellationsBefore (90 days)"),
	"config.home_block_policies":                            notPersonal("home dashboard block policy configuration"),
	"config.home_layouts":                                   parent("auth.accounts"),
	"config.school_setup_dismissals":                        parent("auth.accounts"),
	"config.school_setups":                                  notPersonal("school setup wizard state"),
	"config.setting_audit":                                  own("nightly retention step setting_audit_retention, AuditRetention.DeleteExpiredSettingChanges -> SettingAuditRetention.deleteExpired -> config.delete_expired_setting_audit (3 years); changed_by SET NULL on account delete"),
	"config.setting_values":                                 notPersonal("tenant setting values (only updated-by account reference, FK NO ACTION)"),
	"config.traeger_setting_audit":                          own("each Träger setting change prunes the Träger's changes older than 3 years in the same Träger transaction, TraegerSettingRepository.RecordChange -> config.delete_expired_traeger_setting_audit (ADR 0107); rows go with platform.organizations; changed_by SET NULL on account delete"),
	"config.traeger_setting_values":                         notPersonal("Träger setting values (only updated-by account reference, FK NO ACTION); rows go with platform.organizations"),
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
	"education.class_homework":                              own("nightly retention step homework_retention (roots/owners/worker_retention.go), Timetable DeleteExpiredHomework (school window gdpr.homework_retention_days after the homework day, #4796)"),
	"education.class_teachers":                              staffRetention("School Membership EraseNextRetiredStaff"),
	"education.grade_transition_class_list_entries":         parent("education.grade_transitions"),
	"education.grade_transition_class_teachers":             parent("education.grade_transitions"),
	"education.grade_transition_history":                    parent("education.grade_transitions"),
	"education.grade_transition_mappings":                   notPersonal("class-to-class mapping"),
	"education.grade_transitions":                           own("nightly retention step grade_transition_retention, GradeTransitionRetention.DeleteExpiredGradeTransitions (school window gdpr.student_retention_days after application, reversal or a draft's last change, #4045)"),
	"education.group_substitution":                          staffRetention("Workforce SQL users.erase_retired_staff_records"),
	"education.group_teacher":                               staffRetention("School Membership EraseNextRetiredStaff"),
	"education.groups":                                      notPersonal("class/group catalog"),
	"education.homework_feedback":                           own("nightly retention step homework_retention (roots/owners/worker_retention.go), Timetable DeleteExpiredHomework (school window gdpr.homework_retention_days after the feedback day, #4796)"),
	"enrollment.care_offering_auto_triggers":                notPersonal("offering trigger configuration"),
	"enrollment.care_offering_bookings":                     parent("enrollment.request_children"),
	"enrollment.care_offerings":                             notPersonal("care offering catalog"),
	"enrollment.change_request_messages":                    parent("enrollment.change_requests"),
	"enrollment.change_requests":                            parent("enrollment.requests"),
	"enrollment.form_schemas":                               notPersonal("enrollment form definitions"),
	"enrollment.holiday_form_documents":                     notPersonal("PDF notices of a Träger holiday period (#5199), no family answers; go with period or Träger; created_by SET NULL on account delete"),
	"enrollment.holiday_period_forms":                       notPersonal("form version of a Träger holiday period (#5198): the questions, no answers; goes with its period; created_by SET NULL on account delete"),
	"enrollment.holiday_allocation_criteria":                notPersonal("allocation criteria of a Träger (#4700): names, questions and priority, no person; they go with the Träger"),
	"enrollment.holiday_allocation_decisions":               parent("enrollment.holiday_registrations"),
	"enrollment.holiday_period_weeks":                       notPersonal("week of a Träger holiday period (#4659): dates and the host school ID, no person"),
	"enrollment.holiday_periods":                            notPersonal("holiday period of a Träger (#4659): dates, hints and status, no person"),
	"enrollment.traeger_care_offering_schools":              notPersonal("school a Träger care offering is copied into, with the last outcome (#4704, ADR 0195): IDs and codes, no person"),
	"enrollment.traeger_care_offering_versions":             notPersonal("released version of a Träger care offering (#4704, ADR 0195): offering fields, prices and contract terms; released_by is an account reference set to NULL with the account"),
	"enrollment.traeger_care_offerings":                     notPersonal("care offering a Träger keeps for its schools (#4704, ADR 0195): offering fields, prices and contract terms, no person"),
	"enrollment.traeger_phase_schools":                      notPersonal("school a Träger phase is copied into, with the last outcome (#4704, ADR 0195): IDs and codes, no person"),
	"enrollment.traeger_phase_versions":                     notPersonal("released version of a Träger phase (#4704, ADR 0195): phase fields; released_by is an account reference set to NULL with the account"),
	"enrollment.traeger_phases":                             notPersonal("enrollment phase a Träger keeps for its schools (#4704, ADR 0195): phase fields and status, no person"),
	"enrollment.holiday_guest_contacts":                     own("hourly holiday-guest-reconciliation -> HolidayGuestStore.PurgeContacts (7 days after the last day of the week) and EndGuest (cancelled guest, #4666, ADR 0120); otherwise with its guest"),
	"enrollment.holiday_guests":                             parent("users.student_profiles"),
	"enrollment.holiday_host_groups":                        own("goes with its group (FK CASCADE) or the school; the group choice of a hosted week holds no personal data besides the ID of the account that chose it (#4666, ADR 0120)"),
	"enrollment.holiday_registration_audit":                 own("hourly token-cleanup, Holiday registration retention -> HolidayRegistrationStore.DeleteExpiredHolidayRegistrations (the days the registration keeps after the last week of the period, a year by default and 92 under the earlier consent texts, #6691, #4663); entries of a registration also go with it; actor_account_id SET NULL on account delete"),
	"enrollment.holiday_registration_deletions":             own("hourly token-cleanup, Holiday registration retention -> HolidayRegistrationStore.DeleteExpiredHolidayRegistrationDeletions (3 years after deleted_at, #4667, ADR 0118); counts per period without names; rows go with platform.organizations"),
	"enrollment.holiday_registration_bookings":              parent("enrollment.holiday_registration_children"),
	"enrollment.holiday_closure_family_keys":                notPersonal("key of a Träger for the family fingerprint of a closing day mail (ADR 0196)"),
	"enrollment.holiday_closure_notices":                    parent("enrollment.holiday_registrations"),
	"enrollment.holiday_closure_sends":                      parent("enrollment.holiday_registrations"),
	"enrollment.holiday_registration_criteria":              parent("enrollment.holiday_registrations"),
	"enrollment.holiday_registration_children":              parent("enrollment.holiday_registrations"),
	"enrollment.holiday_registration_contacts":              parent("enrollment.holiday_registrations"),
	"enrollment.holiday_registration_proofs":                own("hourly token-cleanup, Holiday registration retention -> HolidayRegistrationStore.DeleteUnclaimedHolidayProofs (unclaimed after 24 hours, #5238); claimed proofs go with their registration, period or Träger"),
	"enrollment.holiday_registration_rate_limits":           own("hourly token-cleanup, Holiday registration retention -> HolidayRegistrationStore.DeleteExpiredHolidayRegistrations, and each count of the Träger (24 hours after window_start, #4661)"),
	"enrollment.holiday_registrations":                      own("hourly token-cleanup, Holiday registration retention -> HolidayRegistrationStore.DeleteExpiredHolidayRegistrations (the days the registration keeps after the last week of the period, a year by default and 92 under the earlier consent texts, #6691, spec #4611 user story 121, #4661); a registration with a child of a retired demo school goes with its visitor, SQL platform.erase_closed_demo_visitors (1.15.797, #6015)"),
	"enrollment.holiday_week_time_models":                   notPersonal("time model of a Träger holiday week (#4659): name, times and price, no person"),
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
	"parent_billing.beleg_clarification_reviews":            notPersonal("review of an entry of the clarification list of a Träger (#4720, Löschkonzept section 3): note, escalation with its reason, next review, the reviewing operator; goes with platform.organizations"),
	"parent_billing.beleg_deletions":                        notPersonal("deletion log of the Belege of a Träger (#4720, ADR 0184): counts per type and day, no person; the hourly Beleg retention deletes entries after 3 years, the rest goes with platform.organizations"),
	"parent_billing.beleg_retention_instructions":           notPersonal("documented instruction of a Träger per Beleg type (#4720, ADR 0184): basis with its statutory floor or written justification, legal basis, event, period, the instructing office and the reference of the instruction; goes with platform.organizations"),
	"parent_billing.beleg_retention_suspensions":            notPersonal("suspension of the deletion of a Beleg type or Beleg (#4720, ADR 0184): reason, scope, responsible office, next review; goes with platform.organizations"),
	"parent_billing.beleg_retentions":                       notPersonal("retention record per Beleg (#4720, ADR 0184): type, Beleg ID, days and state, no person; the deletion drops it with its Beleg, the daily evaluation drops the record of a Beleg that went another way, the rest goes with platform.organizations"),
	"parent_billing.care_cost_share_events":                 parent("care_contract.contracts"),
	"parent_billing.care_cost_shares":                       parent("care_contract.contracts"),
	"parent_billing.care_evidence_events":                   parent("care_contract.contracts"),
	"parent_billing.care_evidences":                         parent("care_contract.contracts"),
	"parent_billing.care_invoice_cancellations":             parent("parent_billing.care_invoices"),
	"parent_billing.care_invoice_corrections":               parent("parent_billing.care_invoices"),
	"parent_billing.care_invoice_cost_shares":               parent("parent_billing.care_invoices"),
	"parent_billing.care_invoice_reductions":                parent("parent_billing.care_invoices"),
	"parent_billing.family_child_count_events":              parent("care_contract.contracts"),
	"parent_billing.family_child_counts":                    parent("care_contract.contracts"),
	"parent_billing.care_invoice_positions":                 parent("parent_billing.care_invoices"),
	"parent_billing.care_letter_mailings":                   parent("parent_billing.care_letters"),
	"parent_billing.care_letters":                           parent("parent_billing.care_invoices"),
	"parent_billing.care_meal_positions":                    parent("parent_billing.care_invoices"),
	"parent_billing.care_meal_tariffs":                      notPersonal("how the Träger bills the Mensa of a school from a month on (#4686), no person"),
	"active.attendance_retention_marks":                     notPersonal("a school's day before which the attendance retention may have deleted (#4686, ADR 0168); a date only, removed with a demo school"),
	"schedule.meal_billing_months":                          own("the Mensa-Monatsfreigabe of a school is the record of what the Träger bills (#4686, ADR 0168); it never changes, goes with platform.organizations when the Träger is erased (#4657) and with a retired demo school (1.15.811); the period follows with #4720"),
	"schedule.meal_billing_month_children":                  parent("schedule.meal_billing_months"),
	"schedule.pickup_change_team_notices":                   parent("schedule.care_schedule_change_requests"),
	"schedule.traeger_closing_days":                         notPersonal("closing day of a Träger (#4702): dates, a reason and school IDs, no person; goes with platform.organizations"),
	"parent_billing.care_invoices":                          own("an issued monthly invoice never changes and stays when its contract goes (§ 147 AO, ADR 0088 section 9, ADR 0119); it goes with platform.organizations when the Träger is erased after its export (#4657); the hourly Beleg retention deletes it after the period its Träger instructed and only then (SQL parent_billing.delete_due_belege, 1.15.862, #4720, ADR 0184); without an instruction it waits on the clarification list"),
	"parent_billing.care_run_amendments":                    own("the changes of the review stay with the run whose release they document (ADR 0119 addendum 2026-11-08, #6197); a released run copies them onto its invoices, which keep them; they go with platform.organizations when the Träger is erased (#4657) and, in the demo erasure, with their contract; account_id SET NULL on account delete"),
	"parent_billing.care_run_events":                        own("the trail of a monthly run stays with the issued invoices it documents (ADR 0119); it goes with platform.organizations when the Träger is erased (#4657); actor_account_id SET NULL on account delete"),
	"parent_billing.care_run_letters":                       notPersonal("payment date and free text of the letters of a released monthly run (#4684, ADR 0165), no person"),
	"parent_billing.care_run_holds":                         own("the snapshot of a child a release held back stays with its run as the record why the child was not billed (ADR 0158 E5); it goes with platform.organizations when the Träger is erased (#4657) and with a retired demo school (1.15.805)"),
	"parent_billing.care_runs":                              notPersonal("monthly run of a Träger (#4684): month, status, issue date and sums, no person"),
	"parent_billing.care_notice_runs":                       notPersonal("run of the Jahresbescheide of a Träger (#6219, ADR 0182): year, period, schools, status and sums, no person"),
	"parent_billing.care_notices":                           own("an issued Jahresbescheid never changes and stays when its contract goes (§ 147 AO, ADR 0119, ADR 0182); it goes with platform.organizations when the Träger is erased after its export (#4657) and with a retired demo school (1.15.823); the hourly Beleg retention deletes it after the period its Träger instructed and only then (SQL parent_billing.delete_due_belege, 1.15.862, #4720, ADR 0184); without an instruction it waits on the clarification list"),
	"parent_billing.care_notice_months":                     parent("parent_billing.care_notices"),
	"parent_billing.care_notice_letters":                    parent("parent_billing.care_notices"),
	"parent_billing.care_notice_letter_mailings":            parent("parent_billing.care_notice_letters"),
	"parent_billing.care_notice_run_events":                 own("the trail of a run of Jahresbescheide stays with the notices it documents (ADR 0182); it goes with platform.organizations when the Träger is erased (#4657) and, for a retired demo school, with the contracts it names (1.15.823); actor_account_id SET NULL on account delete"),
	"parent_billing.care_run_schools":                       notPersonal("schools a monthly run bills (#5916): school, month and whether the run is open, no person"),
	"parent_billing.care_tariffs":                           notPersonal("monthly price of a care module per school and weekday (#4684), no person"),
	"parent_billing.care_price_lists":                       notPersonal("head of a price list of a school with its free months (#6196), no person"),
	"parent_billing.contribution_rules":                     notPersonal("contribution rule of a Träger (#4664): kind, parameters, validity and the note of an exemption, no person"),
	"parent_billing.cost_bearers":                           notPersonal("body that pays the holiday contribution of a family in its place (#5627): name, address and file reference of an office, no person"),
	"parent_billing.document_letters":                       parent("parent_billing.documents"),
	"parent_billing.document_mailings":                      parent("parent_billing.document_letters"),
	"parent_billing.document_positions":                     parent("parent_billing.documents"),
	"parent_billing.documents":                              own("an issued invoice never changes and stays when its registration goes (§ 147 AO, ADR 0088 section 9, ADR 0119); it goes with platform.organizations when the Träger is erased after its export (#4657); the invoice of a holiday registration of a retired demo school goes with its visitor, the demo Träger keeps no books (SQL platform.erase_closed_demo_visitors, 1.15.797, #6015); the hourly Beleg retention deletes it after the period its Träger instructed and only then (SQL parent_billing.delete_due_belege, 1.15.862, #4720, ADR 0184); without an instruction it waits on the clarification list"),
	"parent_billing.datev_booked_documents":                 parent("parent_billing.export_files"),
	"parent_billing.export_file_transfers":                  parent("parent_billing.export_files"),
	"parent_billing.export_file_invoices":                   parent("parent_billing.export_files"),
	"parent_billing.export_files":                           own("the export protocol stays with the issued invoices whose files it documents (#4693, ADR 0119); it goes with platform.organizations when the Träger is erased (#4657); its invoice links go with their invoice (export_file_invoices, also in the demo erasure and the Beleg retention of 1.15.862); exported_by_account_id and submitted_by_account_id SET NULL on account delete"),
	"parent_billing.export_mappings":                        notPersonal("field mapping of the Sollstellung of a Träger (#4693): columns, separators, date format and file name, no person"),
	"parent_billing.revenue_accounts":                       notPersonal("revenue account of a Leistung or Angebot of a Träger (#4821), no person"),
	"parent_billing.cost_centers":                           notPersonal("cost center of a school, the holiday care or an Angebot of a Träger (#4821), no person"),
	"parent_billing.family_billing":                         own("a deviation goes with care_contract.contracts; a row a holiday registration used (last_used_on) loses only its contract and deviation (trg_family_billing_detach); a family without a contract (#6692) goes with the hourly Beleg retention, BelegRetention.ApplyBelegRetention -> Store.DeleteExpiredFamilies, on 1 January of the fourth year after last_used_on under the Träger's holiday_document instruction and without its deletion stop, unless a document of the family is kept, logged in parent_billing.beleg_deletions, and with platform.organizations; updated_by_account_id SET NULL on account delete"),
	"parent_billing.child_debtor_numbers":                   own("a number of a child (#7007) goes with the hourly Beleg retention, BelegRetention.ApplyBelegRetention -> Store.DeleteExpiredFamilies -> deleteExpiredChildNumbers, on 1 January of the fourth year after last_used_on under the Träger's holiday_document instruction and without its deletion stop, unless a document of the family is kept, logged in parent_billing.beleg_deletions, and with platform.organizations; updated_by_account_id SET NULL on account delete"),
	"parent_billing.child_debtor_number_events":             own("goes with the hourly Beleg retention, Store.DeleteExpiredFamilies -> deleteExpiredChildNumbers, once its number is gone and it is three years old, and with platform.organizations; actor_account_id SET NULL on account delete"),
	"parent_billing.family_billing_events":                  own("goes with care_contract.contracts; an entry without a contract (#6692) goes with the hourly Beleg retention, Store.DeleteExpiredFamilies, once its row is gone and it is three years old, and with platform.organizations; actor_account_id SET NULL on account delete"),
	"parent_billing.holiday_contribution_lines":             parent("parent_billing.holiday_contributions"),
	"parent_billing.holiday_contributions":                  parent("enrollment.holiday_registration_children"),
	"parent_billing.holiday_sibling_group_members":          parent("enrollment.holiday_registrations"),
	"parent_billing.holiday_sibling_groups":                 own("goes with its last but one registration (trigger drop_lone_holiday_sibling_group on parent_billing.holiday_sibling_group_members, #5628) or with platform.organizations; created_by_account_id SET NULL on account delete"),
	"parent_billing.number_range_counters":                  notPersonal("last invoice number of a Träger per service and year (#4668), no person"),
	"parent_billing.document_kinds":                         notPersonal("kind of document a service of a Träger issues (#4688), no person"),
	"parent_billing.number_ranges":                          notPersonal("pattern of the invoice numbers of a Träger (#4668), no person"),
	"parent_billing.run_correction_dismissals":              parent("parent_billing.document_positions"),
	"parent_billing.run_corrections":                        parent("enrollment.holiday_registration_children"),
	"parent_billing.run_letters":                            own("what the letters of a released run print (payment date, free text, emergency numbers) stays with the letters it made (ADR 0121); it goes with its run once the Beleg retention deleted every document of the run (1.15.862, #4720) and with platform.organizations when the Träger is erased (#4657); created_by_account_id SET NULL on account delete"),
	"parent_billing.run_events":                             own("the trail of a billing run stays with the issued invoices it documents (ADR 0119); it goes with platform.organizations when the Träger is erased (#4657); actor_account_id SET NULL on account delete"),
	"parent_billing.run_due_dates":                          notPersonal("due date of a run per payment method (#4689), no person"),
	"parent_billing.runs":                                   notPersonal("billing run of a Träger (#4668): period, status, issue date and sums, no person"),
	"parent_billing.sepa_creditors":                         notPersonal("SEPA creditor identifier of a Träger, its pre-notification days and its choice for a direct debit without a mandate (#4695), no person"),
	"parent_billing.sepa_debit_files":                       own("the stored PAIN.008 file of a SEPA message (ADR 0187) goes by trigger (drop_sepa_debit_files_with_their_members): an unsubmitted download with the first of its debits, installment notices or mandates (with its contract or any other path) and 30 days after the download (delete_expired_sepa_debit_downloads, hourly cleanup), a submitted original with the last of its debits and notices, and with platform.organizations when the Träger is erased after its Rückgabe (#4657); account IDs SET NULL on account delete"),
	"parent_billing.sepa_debits":                            parent("care_contract.contracts"),
	"parent_billing.sepa_mandate_consents":                  parent("care_contract.contracts"),
	"parent_billing.sepa_mandate_events":                    parent("care_contract.contracts"),
	"parent_billing.sepa_mandates":                          parent("care_contract.contracts"),
	"parent_billing.service_terms":                          notPersonal("payment method, payment term and bundling of a service of a Träger (#4689), no person"),
	"platform.announcement_views":                           parent("auth.accounts"),
	"platform.announcements":                                notPersonal("operator release announcements"),
	"platform.billing_key_date_counts":                      own("platform.delete_expired_billing_key_date_counts: 10 years after the end of the key_date calendar year, restricted billing evidence (ADR 0092)"),
	"platform.billing_settings":                             notPersonal("billing configuration"),
	"platform.cleanup_runs":                                 notPersonal("ledger of last successful cleanup per school and job"),
	"platform.demo_school_file_erasures":                    notPersonal("demo process marker (#5544): school ID, time and object counts per upload namespace, and since #5559 the time and row counts per table of the database erasure; no file name, no value, no person"),
	"platform.demo_school_origins":                          notPersonal("demo provenance (#5544): school ID and time a demo order named the school; no person"),
	"platform.demo_school_states":                           own("demo process only: hourly demoExpiry -> RetireDemoSchools -> RetireMany -> SQL platform.erase_closed_demo_visitors (closed order without demo access; also replaces the visitor's name in the hidden demo schools)"),
	"platform.email_delivery":                               own("nightly retention step delivery_retention, DeleteExpiredDeliveries (30 days after its mail ended, else after created_at)"),
	"platform.email_delivery_events":                        own("nightly retention step delivery_retention, DeleteExpiredDeliveries (with the mail, 30 days after it ended; system and Träger mail through RunOnce's DeleteExpiredSystemEmails by cascade)"),
	"platform.email_suppressions":                           own("nightly retention step delivery_retention, DeleteUnheldEmailSuppressions (no fixed period: the first run after no guardian, account or open application of the school holds the address)"),
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
	"platform.realtime_notifications":                       own("nightly retention step delivery_retention, DeleteExpiredDeliveries (two minutes after the notice, #5504)"),
	"platform.schools":                                      notPersonal("school institution master data"),
	"platform.storage_backfill_checkpoints":                 notPersonal("progress of a data migration, counters only"),
	"platform.traeger_document_reads":                       own("hourly token-cleanup, Träger document read retention -> TraegerDocumentStore.DeleteExpiredTraegerDocumentReads (3 years after read_at, every Träger); each opening by a parent or a school also prunes its Träger's expired reads (#4692, ADR 0157); rows go with their document and with platform.organizations; account_id SET NULL on account delete"),
	"platform.traeger_document_releases":                    own("a release is revoked, never deleted, and goes with its document and with platform.organizations when the Träger is erased (#4692, ADR 0157); released_by and revoked_by SET NULL on account delete"),
	"platform.traeger_documents":                            own("a shared document stays until the Träger-Büro deletes it or the Träger is erased (#4692, ADR 0157); the deletion removes the file at once and keeps the row with deleted_at and deleted_by as the referent of its releases, reads and downloads, which keep their own retention, until it goes with platform.organizations (#5993); created_by and deleted_by SET NULL on account delete"),
	"platform.traeger_document_templates":                   own("every version stays, because issued documents name the version they were written with (#4677, ADR 0152); they go with platform.organizations when the Träger is erased (#4657); created_by SET NULL on account delete"),
	"platform.traeger_bank_accounts":                        own("the Träger-Büro deletes a bank account in the Träger-Portal (ADR 0112); the rest go with platform.organizations when the Träger is erased (#4657)"),
	"platform.traeger_master_data_audit":                    own("each read or change of the Träger master data prunes the Träger's entries older than three years in the same Träger transaction, TraegerMasterDataStore.AppendAudit (ADR 0112); rows go with platform.organizations; actor_account_id SET NULL on account delete"),
	"platform.traeger_profiles":                             own("the Träger-Büro clears or replaces the contact person and removes the logo in the Träger-Portal (ADR 0112); the row goes with platform.organizations when the Träger is erased (#4657)"),
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
	"schedule.group_week_protocol_days":                     parent("schedule.group_week_protocols"),
	"schedule.group_week_protocol_pdfs":                     parent("schedule.group_week_protocols"),
	"schedule.group_week_protocols":                         own("nightly retention step week_protocol_retention (roots/owners/worker_retention.go), Timetable DeleteExpiredWeekProtocols (school window gdpr.attendance_retention_days after the week's Sunday, #4769)"),
	"schedule.instance_staff":                               parent("schedule.activity_instances"),
	"schedule.instance_students":                            parent("schedule.activity_instances"),
	"schedule.meal_order_change_counters":                   notPersonal("last number of a school's meal order changes"),
	"schedule.meal_order_changes":                           notPersonal("late change of a meal order with its class, kept for the bill; the child is cleared with the child (ON DELETE SET NULL), the change stays until the school is deleted"),
	"schedule.meal_order_days":                              notPersonal("days of a meal order, no child data"),
	"schedule.meal_order_lines":                             notPersonal("ordered meal of a day with its class, kept for the bill; the child is cleared with the child (ON DELETE SET NULL), the line stays until the school is deleted"),
	"schedule.meal_registration_changes":                    parent("users.student_profiles"),
	"schedule.meal_orders":                                  notPersonal("meal order header: period, deadline, confirmation; the confirming account is cleared with the account"),
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
	"users.message_attachment_cleanup":                      own("nightly retention step parent_message_retention, DeleteMessagesBefore -> deleteStaleAttachments (30 days after cleaned_at); the file store cleanup tick completes the intents"),
	"users.message_attachments":                             own("a sent file goes with its message (ON DELETE CASCADE from users.parent_messages, users.parent_group_messages, users.staff_messages and their retention steps); an unsent upload after 24 hours in the nightly retention step parent_message_retention (deleteStaleAttachments)"),
	"users.notification_preferences":                        parent("auth.accounts"),
	"users.parent_announcement_declaration_submissions":     parent("users.student_profiles"),
	"users.parent_announcement_declaration_versions":        parent("users.parent_announcements"),
	"users.parent_announcement_options":                     notPersonal("announcement answer options"),
	"users.parent_announcement_reads":                       parent("auth.accounts"),
	"users.parent_announcement_responses":                   parent("users.student_profiles"),
	"users.parent_announcement_targets":                     notPersonal("announcement target configuration"),
	"users.parent_announcements":                            own("nightly retention step parent_announcement_retention, ParentAnnouncementRetention.DeleteExpiredParentAnnouncements (school window gdpr.parent_message_retention_days after expiry, publication or a draft's last change; a declaration with submissions stays until they go with their child, #4045)"),
	"users.parent_group_message_recipients":                 parent("users.parent_group_messages"),
	"users.parent_group_messages":                           own("nightly retention step parent_message_retention, DeleteMessagesBefore deletes group thread messages with the same window (#4784)"),
	"users.parent_group_thread_reads":                       parent("users.parent_group_threads"),
	"users.parent_group_threads":                            own("nightly retention step parent_message_retention deletes group threads left empty (deleteEmptyGroupThreadsSQL)"),
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
	"users.staff_qualification_types":                       notPersonal("the kinds of proof a school tracks (#4808): names, validity and reminder lead, no person"),
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
	"users.student_note_team_notices":                       parent("users.student_notes"),
	"users.student_notes":                                   parent("users.student_profiles"),
	"users.student_profiles":                                own("nightly retention step student_retention, Retention.DeleteExpiredStudents -> student deletion workflow (school window after the last care day, #3962)"),
	"users.student_school_memberships":                      parent("users.student_profiles"),
	"users.survey_answers":                                  parent("users.survey_responses"),
	"users.survey_participations":                           parent("users.surveys"),
	"users.survey_question_options":                         parent("users.survey_questions"),
	"users.survey_questions":                                parent("users.surveys"),
	"users.survey_responses":                                parent("users.surveys"),
	"users.surveys":                                         own("nightly retention steps parent_survey_retention and staff_survey_retention, SurveyRetention.DeleteExpiredParentSurveys / DeleteExpiredStaffSurveys (school window gdpr.parent_message_retention_days or gdpr.staff_message_retention_days after the survey closed, a draft after its last change, #4781)"),
	"users.teachers":                                        staffRetention("School Membership EraseNextRetiredStaff"),
	"workforce.staff_record_changes":                        parent("workforce.staff_records"),
	"workforce.staff_record_links":                          parent("workforce.staff_records"),
	"workforce.staff_records":                               own("the Personalverwaltung dissolves a staff record in the Träger-Portal (DELETE /traeger/staff-records/{id}, #4705), which gives each live Einsatzort its bank and tax values back; the deletion after the end of the last Arbeitsvertrag comes with the contracts (ADR 0089 section 12, #4706); rows go with platform.organizations when the Träger is erased (#4657)"),
	"workforce.staff_releases":                              notPersonal("the release of a Träger names the statement it confirmed and the operator's id; it goes with platform.organizations (#4705)"),
}

func TestEveryTableHasARetentionPath(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	var tables []string
	// phx_test holds the test harness's leftover-gate start state
	// (internal/testdb), which every clone inherits from the template; no
	// migration creates it.
	require.NoError(t, db.NewRaw(`
		SELECT n.nspname || '.' || c.relname
		FROM pg_class AS c
		JOIN pg_namespace AS n ON n.oid = c.relnamespace
		WHERE c.relkind IN ('r', 'p')
		  AND NOT c.relispartition
		  AND n.nspname NOT IN ('pg_catalog', 'information_schema', 'pg_toast', 'phx_test')
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
