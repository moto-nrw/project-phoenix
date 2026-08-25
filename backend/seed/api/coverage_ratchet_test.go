package api_test

import (
	"database/sql"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/uptrace/bun/driver/pgdriver"
)

// TestSeedCoverageRatchet keeps the demo seeder honest about which tables it
// actually fills. It runs only against a stack that has been through
// `migrate reset` + `seed` + `simulate full-day`, so it skips by default and
// is driven by the seed-smoke CI job:
//
//	SEED_COVERAGE_DSN=postgres://... go test ./seed/api/ -run TestSeedCoverageRatchet
//
// The rules, mirroring the other ratchets in backend/test:
//
//   - A table that ends up EMPTY must be listed in seedCoverageAllowlist with a
//     reason. A new table without seed coverage fails the build.
//   - An allowlisted table that now HAS rows fails too: remove the entry. The
//     ratchet only turns one way, the list may never grow silently.
//
// Entries marked "GAP: prod has N rows" are the real backlog: production
// tenants fill those tables and the seeder does not, so every screen built on
// them looks empty in dev. The counts come from a read-only aggregate scan of
// the production database on 2026-08-20 (row counts only, no personal data).
// Shrink this list by teaching the seeder the missing flow, never by deleting
// the check.
var seedCoverageAllowlist = map[string]string{
	"active.combined_groups":               "empty in prod too",
	"active.group_mappings":                "empty in prod too",
	"active.scheduled_checkouts":           "empty in prod too",
	"active.staff_balance_adjustments":     "GAP: prod has 9 rows",
	"active.staff_month_balance_snapshots": "empty in prod too",
	"active.staff_vacation_openings":       "empty in prod too",
	"active.staff_vacation_quota":          "GAP: prod has 1 rows",
	"active.work_session_breaks":           "GAP: prod has 7 rows",

	"activities.group_targets": "GAP: prod has 29 rows",
	"activities.schedules":     "GAP: prod has 330 rows",

	"audit.class_list_entry_changes":        "not in prod yet (migration newer than the deployed image)",
	"audit.data_access_log":                 "GAP: prod has 149 rows",
	"audit.data_deletions":                  "GAP: prod has 796 rows",
	"audit.data_imports":                    "GAP: prod has 11 rows",
	"audit.deviation_events":                "GAP: prod has 4 rows",
	"audit.enrollment_deletions":            "GAP: prod has 9 rows",
	"audit.enrollment_offering_adjustments": "GAP: prod has 13 rows",
	"audit.enrollment_restorations":         "empty in prod too",
	"audit.guardian_changes":                "GAP: prod has 81 rows",
	"audit.personnel_number_changes":        "empty in prod too",
	"audit.room_color_migration_backup":     "GAP: prod has 38 rows",
	"audit.staff_master_data_changes":       "GAP: prod has 18 rows",
	"audit.student_deletions":               "GAP: prod has 110 rows",
	"audit.time_tracking_deletions":         "GAP: prod has 2 rows",
	"audit.unregistered_tag_scans":          "GAP: prod has 307 rows",
	"audit.wc_alias_migration_backup":       "empty in prod too",

	"auth.accounts_parents":           "empty in prod too",
	"auth.mfa_credentials":            "empty in prod too",
	"auth.mfa_email_challenges":       "empty in prod too",
	"auth.mfa_overrides":              "empty in prod too",
	"auth.mfa_trusted_devices":        "empty in prod too",
	"auth.passkey_credentials":        "empty in prod too",
	"auth.passkey_sessions":           "GAP: prod has 72 rows",
	"auth.password_reset_rate_limits": "GAP: prod has 1 rows",
	"auth.password_reset_tokens":      "empty in prod too",

	"calendar.appointment_occurrence_overrides":     "empty in prod too",
	"calendar.appointment_recipient_students":       "empty in prod too",
	"calendar.appointment_recipients":               "empty in prod too",
	"calendar.appointment_reminder_push_deliveries": "empty in prod too",
	"calendar.appointment_targets":                  "empty in prod too",
	"calendar.appointments":                         "empty in prod too",
	"calendar.recurrence_rules":                     "empty in prod too",

	"config.work_time_model_entries": "empty in prod too",
	"config.work_time_models":        "empty in prod too",

	"display.displays": "empty in prod too",

	"education.class_teachers":                      "empty in prod too",
	"education.grade_transition_class_list_entries": "not in prod yet (migration newer than the deployed image)",
	"education.grade_transition_class_teachers":     "empty in prod too",
	"education.grade_transition_history":            "GAP: prod has 472 rows",
	"education.grade_transition_mappings":           "GAP: prod has 33 rows",
	"education.grade_transitions":                   "GAP: prod has 2 rows",
	"education.group_substitution":                  "GAP: prod has 4 rows",

	"enrollment.care_offering_auto_triggers": "empty in prod too",
	"enrollment.change_request_messages":     "GAP: prod has 2 rows",
	"enrollment.change_requests":             "GAP: prod has 23 rows",
	"enrollment.form_schemas":                "GAP: prod has 53 rows",
	"enrollment.late_invites":                "GAP: prod has 24 rows",
	"enrollment.offering_change_requests":    "GAP: prod has 7 rows",

	"feedback.entries": "empty in prod too",

	"iot.push_subscriptions":   "GAP: prod has 37 rows",
	"iot.pwa_standalone_usage": "not in prod yet (migration newer than the deployed image)",

	"meta.migration_metadata": "empty in prod too",

	"platform.announcement_views":           "GAP: prod has 135 rows",
	"platform.operator_email_change_tokens": "empty in prod too",
	"platform.operator_invitation_tokens":   "empty in prod too",
	"platform.operator_mfa_trusted_devices": "GAP: prod has 1 rows",
	"platform.operator_passkey_credentials": "GAP: prod has 1 rows",
	"platform.operator_passkey_sessions":    "GAP: prod has 4 rows",

	"schedule.activity_exceptions":              "GAP: prod has 2 rows",
	"schedule.calendar_periods":                 "GAP: prod has 10 rows",
	"schedule.closing_days":                     "GAP: prod has 3 rows",
	"schedule.dateframes":                       "empty in prod too",
	"schedule.grade_transition_roster_removals": "empty in prod too",
	"schedule.meal_plan_entries":                "GAP: prod has 12 rows",
	"schedule.planning_tracks":                  "GAP: prod has 7 rows",
	"schedule.recurrence_rules":                 "empty in prod too",
	"schedule.shift_types":                      "GAP: prod has 5 rows",
	"schedule.staff_shift_series":               "empty in prod too",
	"schedule.staff_shift_series_exceptions":    "empty in prod too",
	"schedule.staff_shifts":                     "GAP: prod has 3 rows",
	"schedule.student_arrival_exceptions":       "GAP: prod has 327 rows",
	"schedule.student_arrival_notes":            "GAP: prod has 20 rows",
	"schedule.student_arrival_schedules":        "GAP: prod has 4002 rows",
	"schedule.student_pickup_exceptions":        "GAP: prod has 205 rows",
	"schedule.student_pickup_notes":             "GAP: prod has 53 rows",
	"schedule.timeframes":                       "GAP: prod has 16 rows",
	"schedule.timetable_conflict_acks":          "empty in prod too",

	"users.care_withdrawal_completions":       "requires a booking-authoritative rollout scenario",
	"users.student_care_exit_source_removals": "transient by design: holds source bookings and weekly plans only until a planned care exit is cancelled or takes effect",
	"users.class_list_entries":                "not in prod yet (migration newer than the deployed image)",
	"users.guardian_phone_numbers":            "GAP: prod has 3071 rows",
	"users.guests":                            "empty in prod too",
	"users.notification_preferences":          "GAP: prod has 277 rows",
	"users.parent_announcement_options":       "empty in prod too",
	"users.parent_announcement_reads":         "GAP: prod has 31 rows",
	"users.parent_announcement_responses":     "empty in prod too",
	"users.parent_announcement_targets":       "GAP: prod has 4 rows",
	"users.parent_announcements":              "GAP: prod has 1 rows",
	"users.parent_message_reads":              "GAP: prod has 352 rows",
	"users.persons_guardians":                 "empty in prod too",
	"users.profiles":                          "GAP: prod has 1 rows",
	"users.staff_document_file_cleanup":       "empty in prod too",
	"users.staff_documents":                   "empty in prod too",
	"users.staff_financial_data":              "empty in prod too",
	"users.staff_master_data":                 "GAP: prod has 9 rows",
	"users.staff_qualifications":              "empty in prod too",
	"users.student_companions":                "empty in prod too",
	"users.student_care_exit_removals":        "transient by design (#2487): holds a planned exit's removed plan only until the exit is cancelled or takes effect",
	"users.student_data_change_requests":      "GAP: prod has 97 rows",
	"users.student_document_file_cleanup":     "empty in prod too",
	"users.student_documents":                 "empty in prod too",
	// School file storage (#2596): shipped after the last production
	// measurement, nothing to compare against yet.
	"documents.folders":         "new in #2596, no production rows yet",
	"documents.folder_roles":    "new in #2596, no production rows yet",
	"documents.folder_accounts": "new in #2596, no production rows yet",
	"documents.files":           "new in #2596, no production rows yet",
	"documents.file_cleanup":    "new in #2596, no production rows yet",
	"audit.file_events":         "new in #2596, no production rows yet",
}

// tableCoverageQuery lists every application table. Views, system schemas, and
// Bun's migration bookkeeping stay out: the ratchet is about data the seeder
// produces.
const tableCoverageQuery = `
	SELECT table_schema, table_name
	FROM information_schema.tables
	WHERE table_type = 'BASE TABLE'
	  AND table_schema NOT IN ('pg_catalog', 'information_schema')
	  AND table_schema NOT LIKE 'pg\_%'
	  AND (table_schema, table_name) NOT IN (
		('public', 'bun_migrations'),
		('public', 'bun_migration_locks')
	)
	ORDER BY table_schema, table_name`

func TestSeedCoverageRatchet(t *testing.T) {
	t.Parallel()

	dsn := strings.TrimSpace(os.Getenv("SEED_COVERAGE_DSN"))
	if dsn == "" {
		t.Skip("SEED_COVERAGE_DSN not set: needs a seeded stack (migrate reset + seed + simulate full-day)")
	}

	db := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(dsn)))
	defer func() { _ = db.Close() }()

	rows, err := db.Query(tableCoverageQuery)
	if err != nil {
		t.Fatalf("list tables: %v", err)
	}
	var tables []string
	for rows.Next() {
		var schema, name string
		if scanErr := rows.Scan(&schema, &name); scanErr != nil {
			t.Fatalf("scan table row: %v", scanErr)
		}
		tables = append(tables, schema+"."+name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate tables: %v", err)
	}
	if err := rows.Close(); err != nil {
		t.Fatalf("close table cursor: %v", err)
	}
	if len(tables) == 0 {
		t.Fatal("no application tables found: is SEED_COVERAGE_DSN pointing at a migrated database?")
	}

	discovered := make(map[string]struct{}, len(tables))
	for _, table := range tables {
		discovered[table] = struct{}{}
	}

	var uncovered, stale, unknownAllowlist, missingAllowlistReasons []string
	for table, reason := range seedCoverageAllowlist {
		if _, ok := discovered[table]; !ok {
			unknownAllowlist = append(unknownAllowlist, table)
		}
		if strings.TrimSpace(reason) == "" {
			missingAllowlistReasons = append(missingAllowlistReasons, table)
		}
	}
	for _, table := range tables {
		schema, name, _ := strings.Cut(table, ".")
		var filled bool
		query := fmt.Sprintf(`SELECT EXISTS (SELECT 1 FROM %q.%q)`, schema, name)
		if err := db.QueryRow(query).Scan(&filled); err != nil {
			t.Fatalf("probe %s: %v", table, err)
		}
		_, allowed := seedCoverageAllowlist[table]
		switch {
		case !filled && !allowed:
			uncovered = append(uncovered, table)
		case filled && allowed:
			stale = append(stale, table)
		}
	}
	sort.Strings(uncovered)
	sort.Strings(stale)
	sort.Strings(unknownAllowlist)
	sort.Strings(missingAllowlistReasons)

	if len(uncovered) > 0 {
		t.Errorf("%d table(s) hold no seeded data and are not allowlisted:\n  %s\n\n"+
			"Fix: teach the seeder (backend/seed/api) or the simulator (backend/simulate) to\n"+
			"produce this data. Only if that is genuinely impossible, add the table to\n"+
			"seedCoverageAllowlist with a reason.",
			len(uncovered), strings.Join(uncovered, "\n  "))
	}
	if len(stale) > 0 {
		t.Errorf("%d allowlisted table(s) now hold data: remove them from seedCoverageAllowlist:\n  %s",
			len(stale), strings.Join(stale, "\n  "))
	}
	if len(unknownAllowlist) > 0 {
		t.Errorf("%d seedCoverageAllowlist entry or entries do not name an application table:\n  %s",
			len(unknownAllowlist), strings.Join(unknownAllowlist, "\n  "))
	}
	if len(missingAllowlistReasons) > 0 {
		t.Errorf("%d seedCoverageAllowlist entry or entries have no reason:\n  %s",
			len(missingAllowlistReasons), strings.Join(missingAllowlistReasons, "\n  "))
	}

	t.Logf("seed coverage: %d/%d tables filled, %d allowlisted",
		len(tables)-len(seedCoverageAllowlist), len(tables), len(seedCoverageAllowlist))
}
