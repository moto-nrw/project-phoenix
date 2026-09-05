package api_test

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
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
// Intentional exemptions are durable semantics, not coverage debt. They are
// limited to security/session artifacts, one-time migration backups, legacy
// compatibility tables, and transient lifecycle state.
var seedCoverageExemptions = map[string]string{
	"active.combined_groups":               "empty in prod too",
	"active.group_mappings":                "empty in prod too",
	"active.scheduled_checkouts":           "empty in prod too",
	"active.staff_month_balance_snapshots": "empty in prod too",
	"active.staff_vacation_openings":       "empty in prod too",

	"audit.class_list_entry_changes":    "not in prod yet (migration newer than the deployed image)",
	"audit.enrollment_restorations":     "empty in prod too",
	"audit.personnel_number_changes":    "empty in prod too",
	"audit.room_color_migration_backup": "one-time migration snapshot; only installations with legacy reserved room colors can contain rows",
	"audit.wc_alias_migration_backup":   "empty in prod too",

	"auth.accounts_parents":           "empty in prod too",
	"auth.mfa_credentials":            "empty in prod too",
	"auth.mfa_email_challenges":       "empty in prod too",
	"auth.mfa_overrides":              "empty in prod too",
	"auth.mfa_trusted_devices":        "empty in prod too",
	"auth.passkey_credentials":        "empty in prod too",
	"auth.passkey_sessions":           "short-lived WebAuthn challenge state; fake challenges would be invalid and cleanup removes them",
	"auth.password_reset_rate_limits": "short-lived abuse-control state; deliberately created only by password-reset traffic",
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

	"enrollment.care_offering_auto_triggers": "empty in prod too",

	"feedback.entries": "empty in prod too",

	"iot.push_subscriptions":   "browser/VAPID-bound state; a server-side seed cannot create an honest browser subscription",
	"iot.pwa_standalone_usage": "not in prod yet (migration newer than the deployed image)",

	"meta.meal_participation_permission_grants":     "one-time migration ledger; only upgrades with pre-existing guardian relationships can contain rows",
	"meta.migration_metadata":                       "empty in prod too",
	"meta.parent_student_consent_permission_grants": "one-time migration ledger; only upgrades with pre-existing guardian relationships can contain rows",

	"platform.operator_email_change_tokens": "empty in prod too",
	"platform.operator_invitation_tokens":   "empty in prod too",
	"platform.operator_mfa_trusted_devices": "cryptographically bound trusted-device state; never forge in demo data",
	"platform.operator_passkey_credentials": "hardware/browser-bound WebAuthn credential; never forge in demo data",
	"platform.operator_passkey_sessions":    "short-lived WebAuthn challenge state; fake challenges would be invalid",

	"schedule.dateframes":                       "empty in prod too",
	"schedule.grade_transition_roster_removals": "empty in prod too",
	"schedule.recurrence_rules":                 "empty in prod too",
	"schedule.staff_shift_series":               "empty in prod too",
	"schedule.staff_shift_series_exceptions":    "empty in prod too",
	"schedule.timetable_conflict_acks":          "empty in prod too",

	"users.class_list_entries":                "not in prod yet (migration newer than the deployed image)",
	"users.guests":                            "empty in prod too",
	"users.parent_announcement_options":       "empty in prod too",
	"users.parent_announcement_responses":     "empty in prod too",
	"users.persons_guardians":                 "empty in prod too",
	"users.profiles":                          "legacy compatibility table; current account provisioning uses persons plus typed staff/guardian records",
	"users.staff_document_file_cleanup":       "empty in prod too",
	"users.staff_documents":                   "empty in prod too",
	"users.staff_financial_data":              "empty in prod too",
	"users.staff_qualifications":              "empty in prod too",
	"users.student_companions":                "empty in prod too",
	"users.student_care_exit_removals":        "transient by design (#2487): holds a planned exit's removed plan only until the exit is cancelled or takes effect",
	"users.student_care_exit_source_removals": "transient by design: holds source bookings and weekly plans only until a planned care exit is cancelled or takes effect",
	"users.student_document_file_cleanup":     "empty in prod too",
	"users.student_documents":                 "empty in prod too",
}

// Coverage debt is deliberately separate from durable exemptions. Adding an
// entry is an explicit regression and fails TestSeedCoverageRatchet.
var seedCoverageDebt = map[string]string{}

// Transient tables may be populated during a seed flow and empty again after
// their lifecycle completes. Unlike exemptions, a populated transient table
// is not stale classification debt.
var seedCoverageTransient = map[string]string{
	"auth.invitation_tokens": "short-lived invitation state; successful acceptance consumes the seeded tokens",
	"platform.push_outbox":   "transient delivery state that requires an honest browser/VAPID-bound push subscription, which the server-side seed cannot create",
}

var seedCoverageAllowlist = mergeCoverageClassifications(seedCoverageExemptions, seedCoverageTransient, seedCoverageDebt)

func mergeCoverageClassifications(classifications ...map[string]string) map[string]string {
	merged := make(map[string]string)
	for _, classification := range classifications {
		for table, reason := range classification {
			if _, exists := merged[table]; exists {
				panic("duplicate seed coverage classification: " + table)
			}
			merged[table] = reason
		}
	}
	return merged
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
	if len(seedCoverageDebt) != 0 {
		t.Fatalf("seed coverage debt must stay empty; implement real flows instead: %v", seedCoverageDebt)
	}

	dsn := strings.TrimSpace(os.Getenv("SEED_COVERAGE_DSN"))
	if dsn == "" {
		t.Skip("SEED_COVERAGE_DSN not set: needs a seeded stack (migrate reset + seed + simulate full-day)")
	}

	db := testpkg.OpenPostgresSQL(dsn)
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
		case filled && allowed && seedCoverageTransient[table] == "":
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
