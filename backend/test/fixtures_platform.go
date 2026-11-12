package test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// CreateTestOperator inserts a platform operator with a unique email and
// registers cleanup via tb.Cleanup. Sites that need an int64 use the ID field.
func CreateTestOperator(tb testing.TB, db *bun.DB) *Operator {
	tb.Helper()
	return CreateTestOperatorWithEmail(tb, db,
		fmt.Sprintf("op-%d@test.local", uniqueFixtureSuffix()), "Test Operator")
}

// CreateTestOperatorWithEmail inserts a platform operator with the given email
// and display name and registers cleanup via tb.Cleanup.
func CreateTestOperatorWithEmail(tb testing.TB, db *bun.DB, email, displayName string) *Operator {
	tb.Helper()

	op := &Operator{
		Email:        email,
		DisplayName:  displayName,
		PasswordHash: "$argon2id$v=19$m=65536,t=3,p=2$placeholder$placeholder",
		Active:       true,
	}

	_, err := db.NewInsert().
		Model(op).
		ModelTableExpr("platform.operators").
		Returning("*").
		Exec(context.Background())
	require.NoError(tb, err, "Failed to create test operator")

	OwnTestOperator(tb, db, op.ID)

	return op
}

// CreateTestOrganization inserts an organization fixture and registers exact-ID cleanup.
func CreateTestOrganization(tb testing.TB, db *bun.DB, organization *Organization) *Organization {
	tb.Helper()
	require.NotNil(tb, organization)
	_, err := db.NewInsert().
		Model(organization).
		ModelTableExpr("platform.organizations").
		Returning("*").
		Exec(context.Background())
	require.NoError(tb, err, "Failed to create test organization")
	tb.Cleanup(func() {
		ctx := context.Background()
		var tenantIDs []int64
		cleanupErr := db.NewSelect().
			TableExpr("platform.schools").
			Column("id").
			Where("organization_id = ?", organization.ID).
			Scan(ctx, &tenantIDs)
		require.NoError(tb, cleanupErr, "Failed to find test organization schools")
		// The Träger's own rows (ADR 0086) carry no school; the tenant
		// cleanup below deletes the organization with foreign-key triggers
		// suspended, so they go first and by their Träger.
		// The pushes of the Träger (ADR 0126) are among them.
		for _, table := range []string{"config.traeger_setting_audit", "config.traeger_setting_values", "platform.push_outbox"} {
			_, cleanupErr = db.NewDelete().TableExpr(table).Where("organization_id = ?", organization.ID).Exec(ctx)
			require.NoError(tb, cleanupErr, "Failed to clean up the Träger rows of %s", table)
		}
		cleanupTenantTestData(tb, db, tenantIDs...)
		_, cleanupErr = db.NewDelete().
			Model((*Organization)(nil)).
			ModelTableExpr("platform.organizations").
			Where("id = ?", organization.ID).
			Exec(ctx)
		require.NoError(tb, cleanupErr, "Failed to clean up test organization")
	})
	return organization
}

// OwnTestOperator registers exact-ID teardown for an operator created through
// a service or repository path.
func OwnTestOperator(tb testing.TB, db *bun.DB, operatorID int64) {
	tb.Helper()
	tb.Cleanup(func() { cleanupOperator(tb, db, operatorID) })
}

// OwnTestOperatorPasskeySession removes an operator passkey ceremony when
// the test ends. A discoverable login names no operator, so its session
// does not cascade with the operator fixture.
func OwnTestOperatorPasskeySession(tb testing.TB, db *bun.DB, sessionID string) {
	tb.Helper()
	tb.Cleanup(func() {
		_, err := db.ExecContext(context.Background(), `DELETE FROM platform.operator_passkey_sessions WHERE id = ?`, sessionID)
		require.NoError(tb, err)
	})
}

// OwnTestAccountPasskeySession removes a school-portal passkey ceremony
// when the test ends. A discoverable login names no account, so its session
// does not cascade with an account fixture.
func OwnTestAccountPasskeySession(tb testing.TB, db *bun.DB, sessionID string) {
	tb.Helper()
	tb.Cleanup(func() {
		_, err := db.ExecContext(context.Background(), `DELETE FROM auth.passkey_sessions WHERE id = ?`, sessionID)
		require.NoError(tb, err)
	})
}

// cleanupOperator removes an operator and its audit-log rows. All other
// operator-scoped tables (tokens, MFA, passkeys) cascade on delete; rows in
// domain tables referencing the operator (announcements)
// must be removed by the caller's own cleanup first.
func cleanupOperator(tb testing.TB, db *bun.DB, operatorID int64) {
	tb.Helper()
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(tb, err)
	defer func() { _ = tx.Rollback() }()

	// MFA audit writes run asynchronously. Lock the operator first so an audit
	// insert cannot land between deleting its logs and deleting the operator.
	_, err = tx.ExecContext(ctx, `SELECT id FROM platform.operators WHERE id = ? FOR UPDATE`, operatorID)
	require.NoError(tb, err)
	_, err = tx.ExecContext(ctx, `DELETE FROM platform.operator_audit_log WHERE operator_id = ?`, operatorID)
	require.NoError(tb, err)
	_, err = tx.ExecContext(ctx, `DELETE FROM platform.operators WHERE id = ?`, operatorID)
	require.NoError(tb, err)
	require.NoError(tb, tx.Commit())
}

// CreateTestOperatorWithPassword inserts a platform operator whose password
// hash verifies the given password (cheap Argon2id parameters), for the
// operator login and password-change flows, and registers cleanup.
func CreateTestOperatorWithPassword(tb testing.TB, db *bun.DB, email, password string) *Operator {
	tb.Helper()

	hash, err := hashPassword(password)
	require.NoError(tb, err, "Failed to hash operator password")
	op := &Operator{
		Email:        email,
		DisplayName:  "Test Operator",
		PasswordHash: hash,
		Active:       true,
	}
	_, err = db.NewInsert().
		Model(op).
		ModelTableExpr("platform.operators").
		Returning("*").
		Exec(context.Background())
	require.NoError(tb, err, "Failed to create test operator with password")

	OwnTestOperator(tb, db, op.ID)

	return op
}

// CreateTestSchoolInOrganization inserts a live school of the organization
// and returns its ID (= tenant ID). The organization fixture's cleanup removes
// it with the organization's other schools.
func CreateTestSchoolInOrganization(tb testing.TB, db *bun.DB, organizationID int64, name string) int64 {
	tb.Helper()
	schoolID := uniqueJWTSafeTenantID()
	token := fmt.Sprintf("%d-%d", schoolID, uniqueFixtureSuffix())
	_, err := db.ExecContext(context.Background(), `
		INSERT INTO platform.schools (id, organization_id, name, slug, subdomain, active)
		VALUES (?, ?, ?, ?, ?, true)`,
		schoolID, organizationID, name, "test-school-"+token, fmt.Sprintf("t%d", schoolID))
	require.NoError(tb, err, "Failed to create test school in organization")
	return schoolID
}

// CreateTestTraegerMember grants the account an active Träger membership
// with the given Träger roles (ADR 0084) and removes both when the test
// ends. The rows carry no tenant, and the tenant cleanup deletes an emptied
// organization with foreign-key triggers suspended, so they need their own
// exact-key teardown.
func CreateTestTraegerMember(tb testing.TB, db *bun.DB, organizationID, accountID int64, roles ...string) {
	tb.Helper()
	ctx := context.Background()
	tb.Cleanup(func() { cleanupTraegerMember(tb, db, organizationID, accountID) })
	_, err := db.ExecContext(ctx, `
		INSERT INTO platform.organization_accounts (organization_id, account_id, status, activated_at)
		VALUES (?, ?, 'active', NOW())`, organizationID, accountID)
	require.NoError(tb, err, "Failed to create test Träger membership")
	for _, role := range roles {
		_, err = db.ExecContext(ctx, `
			INSERT INTO auth.organization_account_roles (organization_id, account_id, role)
			VALUES (?, ?, ?)`, organizationID, accountID, role)
		require.NoError(tb, err, "Failed to create test Träger role")
	}
}

// cleanupTraegerMember removes one Träger membership with its roles.
func cleanupTraegerMember(tb testing.TB, db *bun.DB, organizationID, accountID int64) {
	tb.Helper()
	ctx := context.Background()
	_, err := db.ExecContext(ctx, `DELETE FROM auth.organization_account_roles WHERE organization_id = ? AND account_id = ?`, organizationID, accountID)
	require.NoError(tb, err, "Failed to clean up test Träger roles")
	_, err = db.ExecContext(ctx, `DELETE FROM platform.organization_accounts WHERE organization_id = ? AND account_id = ?`, organizationID, accountID)
	require.NoError(tb, err, "Failed to clean up test Träger membership")
}

// OwnTraegerHolidayPeriods removes the holiday periods a test creates for the
// Träger, with their weeks and time models, when the test ends (#4659). The
// rows carry no tenant, and the tenant cleanup deletes an emptied
// organization with foreign-key triggers suspended, so they need their own
// exact-key teardown.
func OwnTraegerHolidayPeriods(tb testing.TB, db *bun.DB, organizationID int64) {
	tb.Helper()
	tb.Cleanup(func() {
		// The periods take their registrations along (#4661); the abuse
		// counters, the deletion log (#4667) and the Träger's mails belong
		// to the Träger itself.
		_, err := db.ExecContext(context.Background(), `DELETE FROM enrollment.holiday_periods WHERE organization_id = ?`, organizationID)
		require.NoError(tb, err, "Failed to clean up test holiday periods")
		// The criteria of the allocation belong to the Träger (#4700); the
		// answers and decisions went with the registrations.
		_, err = db.ExecContext(context.Background(), `DELETE FROM enrollment.holiday_allocation_criteria WHERE organization_id = ?`, organizationID)
		require.NoError(tb, err, "Failed to clean up test holiday criteria")
		_, err = db.ExecContext(context.Background(), `DELETE FROM enrollment.holiday_registration_deletions WHERE organization_id = ?`, organizationID)
		require.NoError(tb, err, "Failed to clean up the test holiday registration deletion log")
		_, err = db.ExecContext(context.Background(), `DELETE FROM enrollment.holiday_registration_rate_limits WHERE organization_id = ?`, organizationID)
		require.NoError(tb, err, "Failed to clean up test holiday registration counters")
		_, err = db.ExecContext(context.Background(), `DELETE FROM platform.email_outbox WHERE organization_id = ?`, organizationID)
		require.NoError(tb, err, "Failed to clean up test Träger mails")
		// A confirmation remembers the family and its number at the Träger
		// (#6692); a family without a contract does not go with one.
		for _, table := range []string{"parent_billing.family_billing_events", "parent_billing.family_billing"} {
			_, err = db.ExecContext(context.Background(), `DELETE FROM `+table+` WHERE organization_id = ?`, organizationID)
			require.NoError(tb, err, "Failed to clean up the test families of the holiday care")
		}
	})
}

// OwnTraegerClosingDays removes the closing days a test plans for the Träger
// (#4702) when the test ends.
func OwnTraegerClosingDays(tb testing.TB, db *bun.DB, organizationID int64) {
	tb.Helper()
	tb.Cleanup(func() {
		_, err := db.ExecContext(context.Background(), `DELETE FROM schedule.traeger_closing_days WHERE organization_id = ?`, organizationID)
		require.NoError(tb, err, "Failed to clean up test Träger closing days")
	})
}

// OwnTraegerContributionRules removes the contribution rules, the billing
// runs with their invoices, the cost bearers and the number ranges a test
// creates for the Träger (#4664, #4668, #5627) when the test ends. The frozen contributions go with
// the children of their registrations.
func OwnTraegerContributionRules(tb testing.TB, db *bun.DB, organizationID int64) {
	tb.Helper()
	tb.Cleanup(func() {
		for _, table := range []string{
			// The export protocol and the field mapping (#4693); a file's
			// invoice links go with it.
			"parent_billing.export_files", "parent_billing.export_mappings",
			// An evidence holds its income level (#4685): it goes first.
			"parent_billing.care_evidences",
			"parent_billing.contribution_rules", "parent_billing.runs", "parent_billing.care_runs", "parent_billing.care_tariffs",
			"parent_billing.care_price_lists",
			// The runs of the Jahresbescheide take their notices, months and
			// trail with them (#6219).
			"parent_billing.care_notice_runs",
			// The Mensa prices and the schools' released Mensa months (#4686);
			// the Mensa positions go with the runs.
			"parent_billing.care_meal_tariffs", "schedule.meal_billing_months",
			"parent_billing.cost_bearers",
			"parent_billing.number_range_counters", "parent_billing.number_ranges", "parent_billing.document_kinds",
			"parent_billing.service_terms",
			// The creditor of the SEPA direct debit (#4695); the mandates go
			// with the care contracts.
			"parent_billing.sepa_creditors",
			// The stored PAIN.008 files of the downloads (ADR 0187).
			"parent_billing.sepa_debit_files",
			// The revenue accounts and cost centers (#4821).
			"parent_billing.revenue_accounts", "parent_billing.cost_centers",
		} {
			_, err := db.ExecContext(context.Background(), `DELETE FROM `+table+` WHERE organization_id = ?`, organizationID)
			require.NoError(tb, err, "Failed to clean up test Elternabrechnung rows")
		}
	})
}

// OwnTraegerCareContracts removes the care contracts of the Träger, their
// templates and trail and the Träger's mails (#4678) when the test ends. The
// rows carry no tenant, so the tenant cleanup does not reach them.
func OwnTraegerCareContracts(tb testing.TB, db *bun.DB, organizationID int64) {
	tb.Helper()
	// The contract letter reads the master data, which logs the read.
	OwnTraegerMasterData(tb, db, organizationID)
	tb.Cleanup(func() {
		for _, table := range []string{
			"care_contract.contracts", "care_contract.contract_templates", "care_contract.contract_events", "platform.email_outbox",
		} {
			_, err := db.ExecContext(context.Background(), `DELETE FROM `+table+` WHERE organization_id = ?`, organizationID)
			require.NoError(tb, err, "Failed to clean up test care contract rows")
		}
	})
}

// OwnTraegerMasterData removes the master data a test creates for the Träger
// (#4653) when the test ends: profile, bank accounts and their log. The rows
// carry no tenant, so they need their own exact-key teardown like the
// holiday periods.
func OwnTraegerMasterData(tb testing.TB, db *bun.DB, organizationID int64) {
	tb.Helper()
	tb.Cleanup(func() {
		for _, table := range []string{"platform.traeger_master_data_audit", "platform.traeger_document_templates", "platform.traeger_bank_accounts", "platform.traeger_profiles"} {
			_, err := db.ExecContext(context.Background(), `DELETE FROM `+table+` WHERE organization_id = ?`, organizationID)
			require.NoError(tb, err, "Failed to clean up test Träger master data")
		}
	})
}

// SoftDeleteTestSchool marks a school deleted, as the operator's school
// deletion does; its rows stay with its tenant cleanup.
func SoftDeleteTestSchool(tb testing.TB, db *bun.DB, schoolID int64) {
	tb.Helper()
	_, err := db.ExecContext(context.Background(), `UPDATE platform.schools SET deleted_at = NOW() WHERE id = ?`, schoolID)
	require.NoError(tb, err, "Failed to soft-delete test school")
}

// HolidayRegistrationFixture is a holiday registration a test wrote: its
// period and the registration.
type HolidayRegistrationFixture struct {
	PeriodID       int64
	RegistrationID int64
}

// CreateTestHolidayRegistration writes a holiday period of the Träger named
// periodName with one care week from 15.10.2029 to 19.10.2029 at hostSchool
// and the time model "bis 16 Uhr", and a registration of guardianEmail with
// one holiday guest per first name, each with a place in that week (ADR
// 0216). The period and the registration go when the test ends.
func CreateTestHolidayRegistration(tb testing.TB, db *bun.DB, organizationID, hostSchool int64, periodName, guardianEmail string, firstNames ...string) HolidayRegistrationFixture {
	tb.Helper()
	OwnTraegerHolidayPeriods(tb, db, organizationID)
	ctx := context.Background()
	var fixture HolidayRegistrationFixture
	var week, model int64
	require.NoError(tb, db.NewRaw(`INSERT INTO enrollment.holiday_periods (organization_id, name, registration_opens_on, registration_closes_on)
		VALUES (?, ?, DATE '2029-09-01', DATE '2029-09-30') RETURNING id`, organizationID, periodName).Scan(ctx, &fixture.PeriodID))
	require.NoError(tb, db.NewRaw(`INSERT INTO enrollment.holiday_period_weeks (organization_id, holiday_period_id, starts_on, ends_on, host_school_id)
		VALUES (?, ?, DATE '2029-10-15', DATE '2029-10-19', ?) RETURNING id`, organizationID, fixture.PeriodID, hostSchool).Scan(ctx, &week))
	require.NoError(tb, db.NewRaw(`INSERT INTO enrollment.holiday_week_time_models (organization_id, holiday_period_week_id, name, starts_at, ends_at, price_cents)
		VALUES (?, ?, 'bis 16 Uhr', '08:00', '16:00', 4500) RETURNING id`, organizationID, week).Scan(ctx, &model))
	require.NoError(tb, db.NewRaw(`INSERT INTO enrollment.holiday_registrations (organization_id, holiday_period_id, guardian_first_name,
			guardian_last_name, guardian_email, guardian_phone, privacy_consent_at)
		VALUES (?, ?, 'Eva', 'Muster', ?, '0151 2345678', NOW()) RETURNING id`, organizationID, fixture.PeriodID, guardianEmail).Scan(ctx, &fixture.RegistrationID))
	for i, firstName := range firstNames {
		var child int64
		require.NoError(tb, db.NewRaw(`INSERT INTO enrollment.holiday_registration_children (organization_id, holiday_period_id,
				holiday_registration_id, position, first_name, last_name, birth_date, guest_school_name)
			VALUES (?, ?, ?, ?, ?, 'Muster', DATE '2019-03-01', 'Grundschule Melle') RETURNING id`,
			organizationID, fixture.PeriodID, fixture.RegistrationID, i+1, firstName).Scan(ctx, &child))
		_, err := db.ExecContext(ctx, `INSERT INTO enrollment.holiday_registration_bookings (organization_id, holiday_period_id,
				holiday_registration_child_id, holiday_period_week_id, holiday_week_time_model_id, placement)
			VALUES (?, ?, ?, ?, ?, 'placed')`, organizationID, fixture.PeriodID, child, week, model)
		require.NoError(tb, err)
	}
	return fixture
}
