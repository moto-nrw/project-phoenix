package test

import "testing"

// The exception register of the demo erasure guard of #6015
// (TestDemoErasureCoversEveryTableWithPersonalData in
// database/migrations/demo_erasure_coverage_test.go, which reads this file).
// It lives among the source ratchets, so scripts/backend-architecture.sh
// check compares it with the pull request's base (ADR 0057 point 5): the
// register may not gain entries and its budget may not rise without an ADR
// the change adds. The step from 36 to 188 entries was the extension of the
// guard to the school tables in #6017.

// The demo school stays soft-deleted, so its tenant rows stay with it: the
// seed's invented school and whatever the visitor typed into it, free texts
// included. They go with the full erasure of retired demo schools (#5559);
// until then a nightly retention step, which also runs for retired demo
// schools (#4728), deletes the rows that have a period.
const schoolContentUntil5559 = "content of the retired demo school, which stays soft-deleted: the seed's school and the visitor's own entries and free texts in it; goes with the full erasure of retired demo schools (#5559), a row with a period earlier by the nightly retention (#4728)"

// Sign-in events keep the period every sign-in event has.
const signInEventsKeepTheirPeriod = "sign-in events (IP address, user agent) keep the 90 days of every sign-in event, decision of the product owner of 02.11.2026 (#5543); the nightly run deletes them in retired demo schools too (#4728)"

// The Träger-Büro of the shared demo Träger is the bootstrap admin of the
// seed (seedTraegerBueroStep); no visitor account is ever a member of it.
const traegerBueroOnly = "Träger-Büro of the shared demo Träger: the seed assigns only its bootstrap admin, never a visitor account (seedTraegerBueroStep)"

// demoErasureExceptions are the tables in scope the erasure leaves on
// purpose, each with its reason. A new table with personal data gets an
// erasure path, not an entry. ADR 0187 adds parent_billing.sepa_debit_files,
// which drops by trigger with its debits. ADR 0208 adds
// users.message_attachments and users.message_attachment_cleanup, which stay
// with the messages of the school.
var demoErasureExceptions = map[string]string{
	"active.activity_session_attendance":             schoolContentUntil5559,
	"active.activity_sessions":                       schoolContentUntil5559,
	"active.attendance":                              schoolContentUntil5559,
	"active.class_handover_children":                 schoolContentUntil5559,
	"active.class_handovers":                         schoolContentUntil5559,
	"active.excused_absence_requests":                schoolContentUntil5559,
	"active.group_supervisors":                       schoolContentUntil5559,
	"active.morning_attendance":                      schoolContentUntil5559,
	"active.payroll_dispatches":                      schoolContentUntil5559,
	"active.scheduled_checkouts":                     schoolContentUntil5559,
	"active.staff_absence_audit":                     schoolContentUntil5559,
	"active.staff_absence_type_allowance_changes":    schoolContentUntil5559,
	"active.staff_absence_type_allowances":           schoolContentUntil5559,
	"active.staff_absences":                          schoolContentUntil5559,
	"active.staff_balance_adjustments":               schoolContentUntil5559,
	"active.staff_month_balance_snapshots":           schoolContentUntil5559,
	"active.staff_vacation_openings":                 schoolContentUntil5559,
	"active.staff_vacation_quota":                    schoolContentUntil5559,
	"active.staff_vacation_quota_changes":            schoolContentUntil5559,
	"active.student_status_days":                     schoolContentUntil5559,
	"active.visits":                                  schoolContentUntil5559,
	"active.work_session_breaks":                     schoolContentUntil5559,
	"active.work_sessions":                           schoolContentUntil5559,
	"activities.student_enrollments":                 schoolContentUntil5559,
	"activities.supervisors":                         schoolContentUntil5559,
	"audit.attendance_corrections":                   schoolContentUntil5559,
	"audit.auth_events":                              signInEventsKeepTheirPeriod,
	"audit.class_list_entry_changes":                 schoolContentUntil5559,
	"audit.data_access_log":                          schoolContentUntil5559,
	"audit.data_deletions":                           schoolContentUntil5559,
	"audit.deviation_events":                         schoolContentUntil5559,
	"audit.enrollment_deletions":                     schoolContentUntil5559,
	"audit.enrollment_offering_adjustments":          schoolContentUntil5559,
	"audit.enrollment_restorations":                  schoolContentUntil5559,
	"audit.export_transfers":                         schoolContentUntil5559,
	"audit.file_events":                              schoolContentUntil5559,
	"audit.guardian_financial_changes":               schoolContentUntil5559,
	"audit.personnel_number_changes":                 schoolContentUntil5559,
	"audit.staff_master_data_changes":                schoolContentUntil5559,
	"audit.student_consent_changes":                  schoolContentUntil5559,
	"audit.student_deletions":                        schoolContentUntil5559,
	"audit.student_field_edits":                      schoolContentUntil5559,
	"audit.substitution_changes":                     schoolContentUntil5559,
	"audit.time_tracking_deletions":                  schoolContentUntil5559,
	"audit.traeger_account_changes":                  traegerBueroOnly,
	"audit.traeger_document_downloads":               traegerBueroOnly + "; only the Träger-Büro downloads",
	"audit.unregistered_tag_scans":                   schoolContentUntil5559,
	"audit.work_session_edits":                       schoolContentUntil5559,
	"auth.account_avatar_file_cleanup":               "the erasure writes the avatar file intent itself; the hourly drain deletes it with the file",
	"auth.caterer_accounts":                          "caterer login of a caterer business; the demo seed gives no visitor account a caterer membership",
	"auth.caterer_invitations":                       schoolContentUntil5559,
	"auth.guardian_invitations":                      schoolContentUntil5559,
	"auth.invitation_tokens":                         schoolContentUntil5559,
	"auth.organization_account_roles":                traegerBueroOnly,
	"auth.parent_declaration_permission_grants":      schoolContentUntil5559,
	"auth.traeger_invitations":                       traegerBueroOnly + "; invitations name synthetic example.org addresses",
	"calendar.appointment_occurrence_overrides":      schoolContentUntil5559,
	"calendar.appointment_recipient_students":        schoolContentUntil5559,
	"calendar.appointment_recipients":                schoolContentUntil5559,
	"calendar.appointment_reminder_push_deliveries":  schoolContentUntil5559,
	"calendar.appointments":                          schoolContentUntil5559,
	"calendar.parent_conference_bookings":            schoolContentUntil5559,
	"calendar.parent_conferences":                    schoolContentUntil5559,
	"calendar.staff_feed_tombstones":                 schoolContentUntil5559,
	"config.setting_audit":                           schoolContentUntil5559,
	"config.staff_target_overrides":                  schoolContentUntil5559,
	"config.staff_work_schedules":                    schoolContentUntil5559,
	"config.traeger_setting_audit":                   traegerBueroOnly,
	"documents.announcement_attachment_cleanup":      schoolContentUntil5559,
	"documents.announcement_attachments":             schoolContentUntil5559,
	"documents.file_cleanup":                         schoolContentUntil5559,
	"documents.files":                                schoolContentUntil5559,
	"education.class_homework":                       schoolContentUntil5559,
	"education.class_teachers":                       schoolContentUntil5559,
	"education.grade_transition_class_list_entries":  schoolContentUntil5559,
	"education.grade_transition_class_teachers":      schoolContentUntil5559,
	"education.grade_transition_history":             schoolContentUntil5559,
	"education.grade_transitions":                    schoolContentUntil5559,
	"education.group_substitution":                   schoolContentUntil5559,
	"education.group_teacher":                        schoolContentUntil5559,
	"education.homework_feedback":                    schoolContentUntil5559,
	"enrollment.care_offering_bookings":              schoolContentUntil5559,
	"enrollment.change_request_messages":             schoolContentUntil5559,
	"enrollment.change_requests":                     schoolContentUntil5559,
	"enrollment.holiday_guest_contacts":              schoolContentUntil5559,
	"enrollment.holiday_guests":                      schoolContentUntil5559,
	"enrollment.holiday_host_groups":                 schoolContentUntil5559,
	"enrollment.holiday_registration_deletions":      "counts per holiday period of the demo Träger, no name",
	"enrollment.holiday_registration_rate_limits":    "abuse counter of the public holiday form, expires after a day",
	"enrollment.late_invites":                        schoolContentUntil5559,
	"enrollment.offering_change_requests":            schoolContentUntil5559,
	"enrollment.request_child_offering_selections":   schoolContentUntil5559,
	"enrollment.request_children":                    schoolContentUntil5559,
	"enrollment.request_guardians":                   schoolContentUntil5559,
	"enrollment.request_upload_cleanup":              schoolContentUntil5559,
	"enrollment.request_uploads":                     schoolContentUntil5559,
	"enrollment.submission_rate_limits":              schoolContentUntil5559,
	"feedback.entries":                               schoolContentUntil5559,
	"meta.meal_participation_permission_grants":      schoolContentUntil5559,
	"meta.parent_student_consent_permission_grants":  schoolContentUntil5559,
	"parent_billing.holiday_sibling_groups":          "a sibling group of the Träger-Büro names no person itself; its members go with their registrations, and a group left alone drops by trigger (drop_lone_holiday_sibling_group)",
	"parent_billing.sepa_debit_files":                "ADR 0187: the stored PAIN.008 file of a message drops by trigger (drop_sepa_debit_files_with_their_members), an unsubmitted download with the first of its debits, installment notices or mandates, a submitted original with the last of its debits and notices, and those go with the demo visitor's contracts",
	"platform.billing_key_date_counts":               "school name and counts of an invoice key date, no person",
	"platform.email_suppressions":                    schoolContentUntil5559,
	"platform.organization_accounts":                 traegerBueroOnly,
	"platform.traeger_bank_accounts":                 traegerBueroOnly + "; invented bank details of the demo Träger",
	"platform.traeger_document_reads":                "an opening keeps only the school ID and the account ID, which the erasure anonymizes; expires three years after the opening (#4692)",
	"platform.traeger_document_releases":             traegerBueroOnly + "; a release names the school, not a person",
	"platform.traeger_document_templates":            traegerBueroOnly,
	"platform.traeger_documents":                     traegerBueroOnly,
	"platform.traeger_master_data_audit":             traegerBueroOnly,
	"platform.traeger_profiles":                      traegerBueroOnly + "; invented master data of the demo Träger",
	"schedule.activity_exceptions":                   schoolContentUntil5559,
	"schedule.activity_instances":                    schoolContentUntil5559,
	"schedule.care_schedule_change_requests":         schoolContentUntil5559,
	"schedule.grade_transition_roster_removals":      schoolContentUntil5559,
	"schedule.group_week_protocol_days":              schoolContentUntil5559,
	"schedule.group_week_protocol_pdfs":              schoolContentUntil5559,
	"schedule.group_week_protocols":                  schoolContentUntil5559,
	"schedule.instance_staff":                        schoolContentUntil5559,
	"schedule.instance_students":                     schoolContentUntil5559,
	"schedule.meal_participation_overrides":          schoolContentUntil5559,
	"schedule.meal_participation_schedules":          schoolContentUntil5559,
	"schedule.meal_registration_changes":             schoolContentUntil5559,
	"schedule.meal_sickness_status_history":          schoolContentUntil5559,
	"schedule.pickup_extension_tasks":                schoolContentUntil5559,
	"schedule.staff_shift_series":                    schoolContentUntil5559,
	"schedule.staff_shift_series_exceptions":         schoolContentUntil5559,
	"schedule.staff_shifts":                          schoolContentUntil5559,
	"schedule.student_arrival_exceptions":            schoolContentUntil5559,
	"schedule.student_arrival_notes":                 schoolContentUntil5559,
	"schedule.student_arrival_schedules":             schoolContentUntil5559,
	"schedule.student_departure_exceptions":          schoolContentUntil5559,
	"schedule.student_pickup_exceptions":             schoolContentUntil5559,
	"schedule.student_pickup_notes":                  schoolContentUntil5559,
	"schedule.student_pickup_schedules":              schoolContentUntil5559,
	"users.care_withdrawal_completions":              schoolContentUntil5559,
	"users.class_list_entries":                       schoolContentUntil5559,
	"users.external_caregivers":                      schoolContentUntil5559,
	"users.guardian_financial_data":                  schoolContentUntil5559,
	"users.guardian_phone_numbers":                   schoolContentUntil5559,
	"users.message_attachment_cleanup":               schoolContentUntil5559,
	"users.message_attachments":                      schoolContentUntil5559,
	"users.parent_announcement_declaration_versions": schoolContentUntil5559,
	"users.parent_announcement_responses":            schoolContentUntil5559,
	"users.parent_announcements":                     schoolContentUntil5559,
	"users.parent_group_message_recipients":          schoolContentUntil5559,
	"users.parent_group_messages":                    schoolContentUntil5559,
	"users.parent_group_thread_reads":                schoolContentUntil5559,
	"users.parent_group_threads":                     schoolContentUntil5559,
	"users.parent_message_reads":                     schoolContentUntil5559,
	"users.parent_message_threads":                   schoolContentUntil5559,
	"users.parent_request_events":                    schoolContentUntil5559,
	"users.parent_request_share_events":              schoolContentUntil5559,
	"users.privacy_consents":                         schoolContentUntil5559,
	"users.staff_document_file_cleanup":              schoolContentUntil5559,
	"users.staff_documents":                          schoolContentUntil5559,
	"users.staff_employment_profiles":                schoolContentUntil5559,
	"users.staff_financial_data":                     schoolContentUntil5559,
	"users.staff_master_data":                        schoolContentUntil5559,
	"users.staff_message_participants":               schoolContentUntil5559,
	"users.staff_message_reads":                      schoolContentUntil5559,
	"users.staff_message_threads":                    schoolContentUntil5559,
	"users.staff_messages":                           schoolContentUntil5559,
	"users.staff_notices":                            schoolContentUntil5559,
	"users.staff_offboarding_cleanup":                schoolContentUntil5559,
	"users.staff_qualifications":                     schoolContentUntil5559,
	"users.staff_school_memberships":                 schoolContentUntil5559,
	"users.student_care_exit_removals":               schoolContentUntil5559,
	"users.student_care_exit_source_removals":        schoolContentUntil5559,
	"users.student_care_exits":                       schoolContentUntil5559,
	"users.student_care_profiles":                    schoolContentUntil5559,
	"users.student_companions":                       schoolContentUntil5559,
	"users.student_data_change_requests":             schoolContentUntil5559,
	"users.student_document_file_cleanup":            schoolContentUntil5559,
	"users.student_documents":                        schoolContentUntil5559,
	"users.student_family_protection_events":         schoolContentUntil5559,
	"users.student_guardian_pickup_permissions":      schoolContentUntil5559,
	"users.student_guardian_relationships":           schoolContentUntil5559,
	"users.student_notes":                            schoolContentUntil5559,
	"users.student_profiles":                         schoolContentUntil5559,
	"users.student_school_memberships":               schoolContentUntil5559,
	"users.survey_answers":                           schoolContentUntil5559,
	"users.survey_participations":                    schoolContentUntil5559,
	"users.survey_question_options":                  schoolContentUntil5559,
	"users.survey_questions":                         schoolContentUntil5559,
	"users.survey_responses":                         schoolContentUntil5559,
	"users.surveys":                                  schoolContentUntil5559,
	"users.teachers":                                 schoolContentUntil5559,
}

// demoErasureExceptionBudget is the recorded size of demoErasureExceptions.
// It only falls: lower it when an entry leaves.
// ADR 0187: the stored PAIN.008 files of the Träger-Rückgabe (+1).
// ADR 0208: the photos and files of chat messages and their cleanup intents
// stay with the messages they belong to (+2).
const demoErasureExceptionBudget = 189

func TestDemoErasureExceptionsKeepTheirBudget(t *testing.T) {
	t.Parallel()
	if len(demoErasureExceptions) != demoErasureExceptionBudget {
		t.Fatalf("%d demo erasure exceptions, but the budget records %d; lower the budget with the register", len(demoErasureExceptions), demoErasureExceptionBudget)
	}
	for table, reason := range demoErasureExceptions {
		if reason == "" {
			t.Errorf("%s needs a reason", table)
		}
	}
}

// demoErasureOutboxKindExceptions are the kinds of a Träger in an outbox
// table the erasure leaves on purpose, keyed "<table> <kind>", each with its
// reason (#6342). A row of the Träger outlives the school it is about, so
// the erasure has to name its kind; the kinds are declared in the schema
// (chk_email_outbox_traeger_kind, chk_push_outbox_traeger_kind), and
// TestDemoErasureCoversEveryOutboxKind (database/migrations) reads both. A
// new kind gets an erasure path, not an entry.
var demoErasureOutboxKindExceptions = map[string]string{}

// demoErasureOutboxKindExceptionBudget is the recorded size of
// demoErasureOutboxKindExceptions. It only falls.
const demoErasureOutboxKindExceptionBudget = 0

func TestDemoErasureOutboxKindExceptionsKeepTheirBudget(t *testing.T) {
	t.Parallel()
	if len(demoErasureOutboxKindExceptions) != demoErasureOutboxKindExceptionBudget {
		t.Fatalf("%d outbox kind exceptions, but the budget records %d; lower the budget with the register",
			len(demoErasureOutboxKindExceptions), demoErasureOutboxKindExceptionBudget)
	}
	for kind, reason := range demoErasureOutboxKindExceptions {
		if reason == "" {
			t.Errorf("%s needs a reason", kind)
		}
	}
}
