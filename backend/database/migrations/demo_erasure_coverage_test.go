package migrations

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// The demo erasure guard of #6015. platform.erase_closed_demo_visitors
// removes a visitor when its demo school retires. PR #6000 found the care
// contracts of #4678, which outlive their school (ADR 0088 section 9), still
// naming the visitor after the erasure, and no test noticed.
//
// The guard takes its tables from the two registers every table already has:
// a table is in scope when the retention register (tableRetentions) says it
// holds personal data and the data lifecycle catalog places it at a school,
// the Träger (organization), an account, the demo process or a school
// reference. Only moto's own platform data (operators, caterer businesses)
// is out of scope: a visitor is never one of them.
//
// Each table in scope is erased by the function itself (DELETE or a
// rewriting UPDATE in its current definition), goes by ON DELETE CASCADE
// with a table the function deletes, or names its reason in
// demoErasureExceptions. The demo school itself stays soft-deleted, so most
// school tables are such exceptions until the full erasure of retired demo
// schools (#5559) deletes them, the visitor's own free texts included.

// demoErasureScopes are the catalog scopes in scope; only "platform" is not.
var demoErasureScopes = []string{"tenant", "tenantChild", "organization", "account", "demo", "tenantReference", "school"}

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
// erasure path; an entry only with a reason that holds for it.
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
	"parent_billing.run_events":                      "trail of a billing run of the demo Träger: action, time and the Träger-Büro account, names no family",
	"parent_billing.run_letters":                     "payment text and emergency numbers of the confirmation letters of a run of the demo Träger, written by the Träger-Büro, no family",
	"platform.billing_key_date_counts":               "school name and counts of an invoice key date, no person",
	"platform.email_suppressions":                    schoolContentUntil5559,
	"platform.organization_accounts":                 traegerBueroOnly,
	"platform.push_outbox":                           schoolContentUntil5559,
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

// demoErasureExceptionBudget is the recorded size of demoErasureExceptions,
// so every change of the list is a deliberate one: lower it when an entry
// leaves, raise it only together with a reasoned new entry.
const demoErasureExceptionBudget = 189

type cascadeEdge struct{ Child, Parent string }

type demoErasureCoverage struct {
	// InScope are the tables with personal data of a school, Träger or account.
	InScope []string
	// FunctionSQL is the current definition of platform.erase_closed_demo_visitors.
	FunctionSQL string
	// Cascades are the ON DELETE CASCADE foreign keys of the schema.
	Cascades   []cascadeEdge
	Exceptions map[string]string
	Budget     int
}

var sqlErasureStatement = regexp.MustCompile(`(?i)\b(DELETE\s+FROM|UPDATE)\s+(?:ONLY\s+)?([a-z_][a-z0-9_]*\.[a-z_][a-z0-9_]*)`)

// sqlWithoutCommentsAndStrings blanks line comments, nested block comments
// and single-quoted literals (” inside one, E'\” escapes included), so text
// there is never read as a statement. Dollar quotes stay: the function body
// itself is one.
func sqlWithoutCommentsAndStrings(sql string) string {
	var out strings.Builder
	for i := 0; i < len(sql); i++ {
		switch {
		case strings.HasPrefix(sql[i:], "--"):
			for i < len(sql) && sql[i] != '\n' {
				i++
			}
			out.WriteByte('\n')
		case strings.HasPrefix(sql[i:], "/*"):
			depth := 0
			for ; i < len(sql); i++ {
				if strings.HasPrefix(sql[i:], "/*") {
					depth++
					i++
				} else if strings.HasPrefix(sql[i:], "*/") {
					depth--
					i++
					if depth == 0 {
						break
					}
				}
			}
			out.WriteByte(' ')
		case sql[i] == '\'':
			escapes := i > 0 && (sql[i-1] == 'E' || sql[i-1] == 'e')
			for i++; i < len(sql); i++ {
				if escapes && sql[i] == '\\' {
					i++
				} else if sql[i] == '\'' {
					if i+1 < len(sql) && sql[i+1] == '\'' {
						i++
						continue
					}
					break
				}
			}
			out.WriteString("''")
		default:
			out.WriteByte(sql[i])
		}
	}
	return out.String()
}

// demoErasureTargets returns the tables the function deletes from and the
// tables it deletes from or rewrites. Comments and literals are no
// statements.
func demoErasureTargets(functionSQL string) (deleted, erased map[string]bool) {
	deleted, erased = map[string]bool{}, map[string]bool{}
	for _, match := range sqlErasureStatement.FindAllStringSubmatch(sqlWithoutCommentsAndStrings(functionSQL), -1) {
		table := strings.ToLower(match[2])
		erased[table] = true
		if strings.HasPrefix(strings.ToUpper(match[1]), "DELETE") {
			deleted[table] = true
		}
	}
	return deleted, erased
}

// cascadeClosure returns every table whose rows go with a deleted table, by
// ON DELETE CASCADE over any number of steps, with the step it goes with.
func cascadeClosure(deleted map[string]bool, edges []cascadeEdge) map[string]string {
	via := map[string]string{}
	for changed := true; changed; {
		changed = false
		for _, edge := range edges {
			if _, done := via[edge.Child]; done || deleted[edge.Child] {
				continue
			}
			if _, reached := via[edge.Parent]; deleted[edge.Parent] || reached {
				via[edge.Child] = edge.Parent
				changed = true
			}
		}
	}
	return via
}

func demoErasureCoverageProblems(in demoErasureCoverage) []string {
	var problems []string
	if len(in.InScope) == 0 {
		problems = append(problems, "no table with personal data is in scope; the scope query is broken and the guard would pass vacuously")
	}
	deleted, erased := demoErasureTargets(in.FunctionSQL)
	if len(deleted) == 0 {
		problems = append(problems, "the erasure function deletes from no table; its definition was not read and the guard would pass vacuously")
	}
	cascaded := cascadeClosure(deleted, in.Cascades)
	for _, table := range in.InScope {
		_, viaCascade := cascaded[table]
		reason, excepted := in.Exceptions[table]
		switch {
		case (erased[table] || viaCascade) && excepted:
			problems = append(problems, fmt.Sprintf("%s is erased now; remove it from demoErasureExceptions and lower the budget", table))
		case erased[table] || viaCascade:
		case !excepted:
			problems = append(problems, fmt.Sprintf("%s holds personal data of the demo school or its visitor, but platform.erase_closed_demo_visitors neither erases it nor deletes a table it cascades from; erase it there (with visitorTraceCounts) or give a reason in demoErasureExceptions", table))
		case strings.TrimSpace(reason) == "":
			problems = append(problems, fmt.Sprintf("%s needs a non-empty reason in demoErasureExceptions", table))
		}
	}
	for table := range in.Exceptions {
		if !slices.Contains(in.InScope, table) {
			problems = append(problems, fmt.Sprintf("%s is in demoErasureExceptions but is no table with personal data in scope; remove it", table))
		}
	}
	switch count := len(in.Exceptions); {
	case count > in.Budget:
		problems = append(problems, fmt.Sprintf("%d demo erasure exceptions, more than the %d recorded; erase the new table, or raise the budget with a reasoned entry", count, in.Budget))
	case count < in.Budget:
		problems = append(problems, fmt.Sprintf("%d demo erasure exceptions, fewer than the %d recorded; lower demoErasureExceptionBudget", count, in.Budget))
	}
	sort.Strings(problems)
	return problems
}

// demoErasureCoverageOfSchema reads the scope, the function and the
// cascades from the migrated schema.
func demoErasureCoverageOfSchema(t *testing.T) demoErasureCoverage {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	var tables []string
	require.NoError(t, db.NewRaw(`
		SELECT n.nspname || '.' || c.relname
		FROM pg_class AS c JOIN pg_namespace AS n ON n.oid = c.relnamespace
		WHERE c.relkind IN ('r', 'p') AND NOT c.relispartition
		  AND n.nspname NOT IN ('pg_catalog', 'information_schema', 'pg_toast')`).Scan(ctx, &tables))
	scopes := map[string]string{}
	for _, entry := range readLifecycleCatalog(t).Tables {
		scopes[entry.Name] = entry.Scope
	}
	var inScope []string
	for _, table := range tables {
		retention, classified := tableRetentions[table]
		if classified && retention.class != noPersonalData && slices.Contains(demoErasureScopes, scopes[table]) {
			inScope = append(inScope, table)
		}
	}
	var functionSQL string
	require.NoError(t, db.NewRaw(`SELECT pg_get_functiondef('platform.erase_closed_demo_visitors()'::regprocedure)`).Scan(ctx, &functionSQL))
	var cascades []cascadeEdge
	require.NoError(t, db.NewRaw(`
		SELECT DISTINCT cn.nspname || '.' || cc.relname AS child, pn.nspname || '.' || pc.relname AS parent
		FROM pg_constraint AS f
		JOIN pg_class AS cc ON cc.oid = f.conrelid JOIN pg_namespace AS cn ON cn.oid = cc.relnamespace
		JOIN pg_class AS pc ON pc.oid = f.confrelid JOIN pg_namespace AS pn ON pn.oid = pc.relnamespace
		WHERE f.contype = 'f' AND f.confdeltype = 'c'`).Scan(ctx, &cascades))
	sort.Strings(inScope)
	return demoErasureCoverage{
		InScope: inScope, FunctionSQL: functionSQL, Cascades: cascades,
		Exceptions: demoErasureExceptions, Budget: demoErasureExceptionBudget,
	}
}

func TestDemoErasureCoversEveryTableWithPersonalData(t *testing.T) {
	t.Parallel()
	coverage := demoErasureCoverageOfSchema(t)
	require.Empty(t, demoErasureCoverageProblems(coverage),
		"demo erasure coverage (#6015): every table with personal data outside the school is erased by platform.erase_closed_demo_visitors, goes with one of its tables by cascade, or has a reason in demoErasureExceptions")
}

// The gap of PR #6000 is caught: without its contract deletion, the parties
// and every other contract table outliving the school are reported.
func TestDemoErasureGuardCatchesTheContractGapOfPR6000(t *testing.T) {
	t.Parallel()
	coverage := demoErasureCoverageOfSchema(t)
	withoutContracts := regexp.MustCompile(`(?i)DELETE\s+FROM\s+care_contract\.contracts\b`)
	require.True(t, withoutContracts.MatchString(coverage.FunctionSQL), "the erasure deletes the contracts of the demo school (1.15.786)")
	coverage.FunctionSQL = withoutContracts.ReplaceAllString(coverage.FunctionSQL, "SELECT 1 FROM care_contract.contracts")
	problems := strings.Join(demoErasureCoverageProblems(coverage), "\n")
	for _, table := range []string{"care_contract.contracts", "care_contract.contract_parties", "care_contract.contract_documents", "care_contract.contract_mailings"} {
		require.Contains(t, problems, table+" holds personal data of the demo school or its visitor")
	}
}

func coverageFixture() demoErasureCoverage {
	return demoErasureCoverage{
		InScope: []string{"care.contracts", "care.parties", "care.party_notes", "care.trail"},
		FunctionSQL: `BEGIN
			-- DELETE FROM care.trail is only a comment
			DELETE FROM care.contracts WHERE school_id = ANY (hidden_schools);
		END`,
		Cascades: []cascadeEdge{
			{Child: "care.party_notes", Parent: "care.parties"},
			{Child: "care.parties", Parent: "care.contracts"},
		},
		Exceptions: map[string]string{},
		Budget:     0,
	}
}

func TestDemoErasureCoverageProblems(t *testing.T) {
	t.Parallel()
	trailMissing := "care.trail holds personal data of the demo school or its visitor"
	tests := []struct {
		name   string
		change func(*demoErasureCoverage)
		want   []string
	}{
		{name: "a table without erasure path is reported, a comment is no statement", change: func(*demoErasureCoverage) {}, want: []string{trailMissing}},
		{name: "an erasure path only in a block comment or a literal fails", change: func(c *demoErasureCoverage) {
			c.FunctionSQL += "\n/* UPDATE care.trail SET name = 'Demo'; /* nested */ DELETE FROM care.trail; */" +
				"\nRAISE LOG 'DELETE FROM care.trail', E'it''s \\' UPDATE care.trail';"
		}, want: []string{trailMissing}},
		{name: "a statement after a comment and a literal still counts", change: func(c *demoErasureCoverage) {
			c.FunctionSQL += "\n/* note */ PERFORM 'a''b'; DELETE FROM care.trail;"
		}},
		{name: "a cascade over two steps counts, a rewrite counts", change: func(c *demoErasureCoverage) {
			c.FunctionSQL += "\nUPDATE care.trail SET name = 'Demo Besuch';"
		}},
		{name: "a cascade from a table the function only rewrites does not count", change: func(c *demoErasureCoverage) {
			c.FunctionSQL = "UPDATE care.contracts SET name = 'Demo'; DELETE FROM care.trail;"
		}, want: []string{"care.parties holds personal data", "care.party_notes holds personal data"}},
		{name: "a reasoned exception within budget passes", change: func(c *demoErasureCoverage) {
			c.Exceptions = map[string]string{"care.trail": "kept for the Träger"}
			c.Budget = 1
		}},
		{name: "an exception without reason fails", change: func(c *demoErasureCoverage) {
			c.Exceptions = map[string]string{"care.trail": " "}
			c.Budget = 1
		}, want: []string{"care.trail needs a non-empty reason"}},
		{name: "an exception for an erased table fails", change: func(c *demoErasureCoverage) {
			c.Exceptions = map[string]string{"care.trail": "kept", "care.parties": "kept"}
			c.Budget = 2
		}, want: []string{"care.parties is erased now"}},
		{name: "an exception outside the scope fails", change: func(c *demoErasureCoverage) {
			c.Exceptions = map[string]string{"care.trail": "kept", "care.gone": "kept"}
			c.Budget = 2
		}, want: []string{"care.gone is in demoErasureExceptions but is no table"}},
		{name: "the budget only falls", change: func(c *demoErasureCoverage) {
			c.Exceptions = map[string]string{"care.trail": "kept"}
		}, want: []string{"1 demo erasure exceptions, more than the 0 recorded"}},
		{name: "a lower count lowers the budget", change: func(c *demoErasureCoverage) {
			c.Exceptions = map[string]string{"care.trail": "kept"}
			c.Budget = 2
		}, want: []string{"1 demo erasure exceptions, fewer than the 2 recorded"}},
		{name: "an empty scope does not pass vacuously", change: func(c *demoErasureCoverage) {
			c.InScope = nil
		}, want: []string{"no table with personal data is in scope"}},
		{name: "an unread function does not pass vacuously", change: func(c *demoErasureCoverage) {
			c.FunctionSQL = ""
		}, want: []string{"deletes from no table", "care.contracts holds personal data", "care.parties holds personal data", "care.party_notes holds personal data", trailMissing}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			coverage := coverageFixture()
			tt.change(&coverage)
			problems := demoErasureCoverageProblems(coverage)
			require.Len(t, problems, len(tt.want), "problems: %q", problems)
			for _, want := range tt.want {
				require.True(t, slices.ContainsFunc(problems, func(p string) bool { return strings.Contains(p, want) }), "want %q in %q", want, problems)
			}
		})
	}
}
