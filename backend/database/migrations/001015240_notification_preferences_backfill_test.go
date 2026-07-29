package migrations

import (
	"context"
	"fmt"
	"testing"
	"time"

	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	iotModels "github.com/moto-nrw/project-phoenix/models/iot"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The backfill exists because two changes land together: consent becomes
// mandatory for every producer, and the school-wide switch defaults to on.
// Without it, every phone registered today would go silent in the same minute.
//
// It therefore has to give a registered device exactly the consent it used to
// act on — and nothing beyond that, because inventing consent is the failure
// this whole epic is built to avoid.
func TestNotificationPreferencesBackfill(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()

	parentAccount := testpkg.CreateTestAccount(t, db, "backfill-parent@example.com")
	adminAccount := testpkg.CreateTestAccount(t, db, "backfill-admin@example.com")
	staffAccount := testpkg.CreateTestAccount(t, db, "backfill-staff@example.com")
	silentAccount := testpkg.CreateTestAccount(t, db, "backfill-silent@example.com")
	optedOutAccount := testpkg.CreateTestAccount(t, db, "backfill-optout@example.com")

	var adminRoleID int64
	require.NoError(t, db.NewSelect().
		TableExpr("auth.roles").
		ColumnExpr("id").
		Where("name = ?", authModels.BaseRoleAdmin).
		Limit(1).
		Scan(ctx, &adminRoleID))
	grantAdmin := func(accountID int64) {
		t.Helper()
		now := time.Now()
		mapping := &authModels.AccountTenant{
			AccountID:   accountID,
			TenantID:    1,
			Status:      authModels.AccountTenantStatusActive,
			ActivatedAt: &now,
		}
		_, err := db.NewInsert().Model(mapping).ModelTableExpr("auth.account_tenants").Exec(ctx)
		require.NoError(t, err)

		adminRole := &authModels.AccountRole{AccountID: accountID, RoleID: adminRoleID}
		adminRole.SetTenantID(1)
		_, err = db.NewInsert().Model(adminRole).ModelTableExpr("auth.account_roles").Exec(ctx)
		require.NoError(t, err)
	}
	grantAdmin(adminAccount.ID)
	grantAdmin(optedOutAccount.ID)

	// A guardian account in the real world always carries an active
	// account_tenants mapping: it is created by the guardian invitation flow,
	// which maps the account to the school. Verified in the seeded dev database,
	// where every parent account has status = 'active' and accounts.active =
	// true. The backfill filters on exactly that pair, so without these rows the
	// fixture would describe a world that does not exist, a parent account
	// belonging to no school at all. Cleaned up by CleanupAuthFixtures below,
	// which deletes auth.account_tenants by account_id.
	mapToTenant := func(accountID int64) {
		t.Helper()
		now := time.Now()
		mapping := &authModels.AccountTenant{
			AccountID:   accountID,
			TenantID:    1,
			Status:      authModels.AccountTenantStatusActive,
			ActivatedAt: &now,
		}
		_, merr := db.NewInsert().Model(mapping).ModelTableExpr("auth.account_tenants").Exec(ctx)
		require.NoError(t, merr)
	}
	mapToTenant(parentAccount.ID)
	mapToTenant(staffAccount.ID)

	// The backfill only writes consent for a school that had switched
	// notifications.dispatch_enabled on itself, because only there was the
	// registered device really being delivered to. This test's subject is what
	// such a school gets, so the override has to exist; the two tests below cover
	// the schools that never switched it on. Do not remove this line as an
	// unmotivated setting: without it the backfill correctly does nothing and
	// every positive assertion here goes empty.
	//
	// Registered with t.Cleanup immediately, so a failure further down still
	// removes the row. A leftover true on the default tenant would silently
	// change any other test that relies on the registry default.
	_, derr := db.ExecContext(ctx, `
		INSERT INTO config.setting_values (tenant_id, setting_key, value)
		VALUES (1, 'notifications.dispatch_enabled', 'true'::jsonb)
		ON CONFLICT (tenant_id, setting_key) DO UPDATE SET value = 'true'::jsonb`)
	require.NoError(t, derr)
	t.Cleanup(func() {
		_, cerr := db.ExecContext(context.Background(), `
			DELETE FROM config.setting_values
			WHERE tenant_id = 1 AND setting_key = 'notifications.dispatch_enabled'`)
		require.NoError(t, cerr)
	})

	insertSubscription := func(accountID int64, portal, endpoint string) {
		t.Helper()
		sub := &iotModels.PushSubscription{
			AccountID: accountID,
			Portal:    portal,
			Endpoint:  endpoint,
			P256dh:    "test-p256dh",
			Auth:      "test-auth",
		}
		sub.SetTenantID(1)
		_, err := db.NewInsert().Model(sub).ModelTableExpr("iot.push_subscriptions").Exec(ctx)
		require.NoError(t, err)
	}

	insertSubscription(parentAccount.ID, iotModels.PushPortalParent, "https://fcm.googleapis.com/backfill-parent")
	// Two devices of one person: the backfill must still produce one row per
	// type, not one per device.
	insertSubscription(adminAccount.ID, iotModels.PushPortalStaff, "https://fcm.googleapis.com/backfill-admin-1")
	insertSubscription(adminAccount.ID, iotModels.PushPortalStaff, "https://fcm.googleapis.com/backfill-admin-2")
	insertSubscription(staffAccount.ID, iotModels.PushPortalStaff, "https://fcm.googleapis.com/backfill-staff")
	insertSubscription(optedOutAccount.ID, iotModels.PushPortalStaff, "https://fcm.googleapis.com/backfill-optout")

	// Somebody who already said no. A backfill must never overwrite a decision.
	_, err := db.ExecContext(ctx, `
		INSERT INTO users.notification_preferences (tenant_id, account_id, notification_type, enabled)
		VALUES (1, ?, 'pickup_upcoming', FALSE)`, optedOutAccount.ID)
	require.NoError(t, err)

	accountIDs := []int64{parentAccount.ID, adminAccount.ID, staffAccount.ID, silentAccount.ID, optedOutAccount.ID}
	t.Cleanup(func() {
		_, cerr := db.NewDelete().
			Table("users.notification_preferences").
			Where("account_id IN (?)", bun.List(accountIDs)).
			Exec(context.Background())
		require.NoError(t, cerr)
		_, cerr = db.NewDelete().
			Table("iot.push_subscriptions").
			Where("account_id IN (?)", bun.List(accountIDs)).
			Exec(context.Background())
		require.NoError(t, cerr)
		testpkg.CleanupAuthFixtures(t, db, accountIDs...)
	})

	require.NoError(t, notificationPreferencesBackfillUp(ctx, db))

	consentOf := func(accountID int64) map[string]bool {
		t.Helper()
		var rows []struct {
			NotificationType string `bun:"notification_type"`
			Enabled          bool   `bun:"enabled"`
		}
		require.NoError(t, db.NewSelect().
			TableExpr("users.notification_preferences").
			ColumnExpr("notification_type, enabled").
			Where("account_id = ?", accountID).
			Scan(ctx, &rows))
		out := make(map[string]bool, len(rows))
		for _, row := range rows {
			out[row.NotificationType] = row.Enabled
		}
		return out
	}

	assert.Equal(t, map[string]bool{"parent_announcement": true}, consentOf(parentAccount.ID),
		"a guardian device delivered exactly one kind of notification")

	assert.Equal(t, map[string]bool{
		"pickup_upcoming":  true,
		"pickup_overdue":   true,
		"activity_start":   true,
		"activity_overdue": true,
	}, consentOf(adminAccount.ID),
		"an admin device gets the four reminder kinds it could already receive, and nothing this epic invented")

	assert.Empty(t, consentOf(staffAccount.ID),
		"a non-admin staff device did not receive the old admin-scoped reminder event")

	assert.Empty(t, consentOf(silentAccount.ID),
		"somebody who never registered a device is not opted into anything")

	optedOut := consentOf(optedOutAccount.ID)
	assert.False(t, optedOut["pickup_upcoming"], "an explicit no must survive the backfill")
	assert.True(t, optedOut["pickup_overdue"], "the untouched kinds are still backfilled")

	// Re-running must be a no-op rather than a duplicate-key failure: migrations
	// are re-applied on restored databases often enough that this matters.
	require.NoError(t, notificationPreferencesBackfillUp(ctx, db))
	assert.False(t, consentOf(optedOutAccount.ID)["pickup_upcoming"])
}

// The backfill may only write consent where the school was actually delivering
// notifications, which means it had switched notifications.dispatch_enabled on
// itself. A school that never did received nothing, so there is no prior
// consent to record, and because this release flips the registry default to
// true, a row written there would arrive together with a switch that suddenly
// reads "on" for somebody who never agreed to a single category. down() is a
// no-op, so nothing takes it back.
func TestNotificationPreferencesBackfillOnlyForSchoolsWithDispatchEnabled(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()

	env := setupBackfillTenants(t, db)

	guardianOn := testpkg.CreateTestAccount(t, db, "backfill-dispatch-on-parent@example.com")
	adminOn := testpkg.CreateTestAccount(t, db, "backfill-dispatch-on-admin@example.com")
	guardianNoOverride := testpkg.CreateTestAccount(t, db, "backfill-dispatch-unset-parent@example.com")
	adminNoOverride := testpkg.CreateTestAccount(t, db, "backfill-dispatch-unset-admin@example.com")
	guardianOff := testpkg.CreateTestAccount(t, db, "backfill-dispatch-off-parent@example.com")

	env.registerAccounts(guardianOn.ID, adminOn.ID, guardianNoOverride.ID, adminNoOverride.ID, guardianOff.ID)

	env.mapAccount(guardianOn.ID, env.enabledTenantID, authModels.AccountTenantStatusActive)
	env.mapAccount(guardianNoOverride.ID, env.noOverrideTenantID, authModels.AccountTenantStatusActive)
	env.mapAccount(guardianOff.ID, env.disabledTenantID, authModels.AccountTenantStatusActive)
	env.grantAdmin(adminOn.ID, env.enabledTenantID)
	env.grantAdmin(adminNoOverride.ID, env.noOverrideTenantID)

	env.insertSubscription(guardianOn.ID, env.enabledTenantID, iotModels.PushPortalParent)
	env.insertSubscription(adminOn.ID, env.enabledTenantID, iotModels.PushPortalStaff)
	env.insertSubscription(guardianNoOverride.ID, env.noOverrideTenantID, iotModels.PushPortalParent)
	env.insertSubscription(adminNoOverride.ID, env.noOverrideTenantID, iotModels.PushPortalStaff)
	env.insertSubscription(guardianOff.ID, env.disabledTenantID, iotModels.PushPortalParent)

	require.NoError(t, notificationPreferencesBackfillUp(ctx, db))

	assert.Equal(t, map[string]bool{"parent_announcement": true}, env.consentOf(guardianOn.ID),
		"a school that switched dispatch on was really delivering, so the consent is recorded")
	assert.Equal(t, map[string]bool{
		"pickup_upcoming":  true,
		"pickup_overdue":   true,
		"activity_start":   true,
		"activity_overdue": true,
	}, env.consentOf(adminOn.ID),
		"same for the admin reminder types at that school")

	assert.Empty(t, env.consentOf(guardianNoOverride.ID),
		"a school without a dispatch_enabled override never delivered, so there is no consent to write")
	assert.Empty(t, env.consentOf(adminNoOverride.ID),
		"the staff block is bound by the same condition as the guardian block")
	assert.Empty(t, env.consentOf(guardianOff.ID),
		"an explicit dispatch_enabled = false is treated exactly like no override at all")
}

// The guardian block must filter on account state the same way the staff block
// does. A family that left the school keeps its old registration; a consent row
// for it would be a permanent, untrue statement about what that person wants,
// and delivery-time checks stopping the notification does not make the row less
// wrong (data minimisation).
func TestNotificationPreferencesBackfillSkipsInactiveGuardians(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()

	env := setupBackfillTenants(t, db)

	activeGuardian := testpkg.CreateTestAccount(t, db, "backfill-guardian-active@example.com")
	deactivatedGuardian := testpkg.CreateTestAccount(t, db, "backfill-guardian-deactivated@example.com")
	leftSchoolGuardian := testpkg.CreateTestAccount(t, db, "backfill-guardian-left@example.com")
	unmappedGuardian := testpkg.CreateTestAccount(t, db, "backfill-guardian-unmapped@example.com")

	env.registerAccounts(activeGuardian.ID, deactivatedGuardian.ID, leftSchoolGuardian.ID, unmappedGuardian.ID)

	env.mapAccount(activeGuardian.ID, env.enabledTenantID, authModels.AccountTenantStatusActive)
	env.mapAccount(deactivatedGuardian.ID, env.enabledTenantID, authModels.AccountTenantStatusActive)
	env.mapAccount(leftSchoolGuardian.ID, env.enabledTenantID, authModels.AccountTenantStatusInactive)
	// unmappedGuardian deliberately gets no account_tenants row at all.

	_, err := db.NewUpdate().
		TableExpr("auth.accounts").
		Set("active = FALSE").
		Where("id = ?", deactivatedGuardian.ID).
		Exec(ctx)
	require.NoError(t, err)

	env.insertSubscription(activeGuardian.ID, env.enabledTenantID, iotModels.PushPortalParent)
	env.insertSubscription(deactivatedGuardian.ID, env.enabledTenantID, iotModels.PushPortalParent)
	env.insertSubscription(leftSchoolGuardian.ID, env.enabledTenantID, iotModels.PushPortalParent)
	env.insertSubscription(unmappedGuardian.ID, env.enabledTenantID, iotModels.PushPortalParent)

	require.NoError(t, notificationPreferencesBackfillUp(ctx, db))

	assert.Equal(t, map[string]bool{"parent_announcement": true}, env.consentOf(activeGuardian.ID),
		"an active guardian at an active mapping is exactly who this backfill is for")
	assert.Empty(t, env.consentOf(deactivatedGuardian.ID),
		"a deactivated account cannot receive anything, so consent must not be invented for it")
	assert.Empty(t, env.consentOf(leftSchoolGuardian.ID),
		"a family that left the school keeps no consent row behind")
	assert.Empty(t, env.consentOf(unmappedGuardian.ID),
		"without a mapping to the school there is no membership the consent could belong to")
}

// backfillTenantEnv holds three throwaway schools that differ only in their
// notifications.dispatch_enabled override, plus the fixture helpers the two
// tests above share. Every ID is created here and read back, never written as a
// literal, so the tests survive any database state.
type backfillTenantEnv struct {
	t                  *testing.T
	db                 *bun.DB
	enabledTenantID    int64
	noOverrideTenantID int64
	disabledTenantID   int64
	adminRoleID        int64
	accountIDs         []int64
}

func setupBackfillTenants(t *testing.T, db *bun.DB) *backfillTenantEnv {
	t.Helper()
	ctx := context.Background()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	var orgID int64
	_, err := db.NewRaw(`
		INSERT INTO platform.organizations (name, slug, active, created_at, updated_at)
		VALUES (?, ?, true, NOW(), NOW())
		RETURNING id`, "Backfill Org "+suffix, "backfill-org-"+suffix).Exec(ctx, &orgID)
	require.NoError(t, err)

	createSchool := func(label string) int64 {
		var id int64
		slug := "backfill-" + label + "-" + suffix
		_, serr := db.NewRaw(`
			INSERT INTO platform.schools (name, slug, subdomain, organization_id, active, created_at, updated_at)
			VALUES (?, ?, ?, ?, true, NOW(), NOW())
			RETURNING id`, "Backfill School "+label+" "+suffix, slug, slug, orgID).Exec(ctx, &id)
		require.NoError(t, serr)
		return id
	}

	env := &backfillTenantEnv{
		t:                  t,
		db:                 db,
		enabledTenantID:    createSchool("dispatch-on"),
		noOverrideTenantID: createSchool("dispatch-unset"),
		disabledTenantID:   createSchool("dispatch-off"),
	}

	require.NoError(t, db.NewSelect().
		TableExpr("auth.roles").
		ColumnExpr("id").
		Where("name = ?", authModels.BaseRoleAdmin).
		Limit(1).
		Scan(ctx, &env.adminRoleID))

	// The override is written as raw jsonb because that is the shape the
	// settings service produces for a boolean (json.Marshal of a Go bool) and
	// the shape the migration matches against.
	setDispatch := func(tenantID int64, value string) {
		_, serr := db.ExecContext(ctx, `
			INSERT INTO config.setting_values (tenant_id, setting_key, value)
			VALUES (?, 'notifications.dispatch_enabled', ?::jsonb)`, tenantID, value)
		require.NoError(t, serr)
	}
	setDispatch(env.enabledTenantID, "true")
	setDispatch(env.disabledTenantID, "false")
	// noOverrideTenantID deliberately gets no row: that is the registry default.

	tenantIDs := []int64{env.enabledTenantID, env.noOverrideTenantID, env.disabledTenantID}
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		if len(env.accountIDs) > 0 {
			_, cerr := db.NewDelete().
				Table("users.notification_preferences").
				Where("account_id IN (?)", bun.List(env.accountIDs)).
				Exec(cleanupCtx)
			require.NoError(t, cerr)
			_, cerr = db.NewDelete().
				Table("iot.push_subscriptions").
				Where("account_id IN (?)", bun.List(env.accountIDs)).
				Exec(cleanupCtx)
			require.NoError(t, cerr)
			testpkg.CleanupAuthFixtures(t, db, env.accountIDs...)
		}
		_, cerr := db.NewDelete().
			Table("config.setting_values").
			Where("tenant_id IN (?)", bun.List(tenantIDs)).
			Exec(cleanupCtx)
		require.NoError(t, cerr)
		_, cerr = db.NewDelete().
			Table("platform.schools").
			Where("id IN (?)", bun.List(tenantIDs)).
			Exec(cleanupCtx)
		require.NoError(t, cerr)
		_, cerr = db.NewDelete().
			Table("platform.organizations").
			Where("id = ?", orgID).
			Exec(cleanupCtx)
		require.NoError(t, cerr)
	})

	return env
}

func (e *backfillTenantEnv) registerAccounts(ids ...int64) {
	e.accountIDs = append(e.accountIDs, ids...)
}

func (e *backfillTenantEnv) mapAccount(accountID, tenantID int64, status string) {
	e.t.Helper()
	now := time.Now()
	mapping := &authModels.AccountTenant{
		AccountID:   accountID,
		TenantID:    tenantID,
		Status:      status,
		ActivatedAt: &now,
	}
	_, err := e.db.NewInsert().Model(mapping).ModelTableExpr("auth.account_tenants").Exec(context.Background())
	require.NoError(e.t, err)
}

func (e *backfillTenantEnv) grantAdmin(accountID, tenantID int64) {
	e.t.Helper()
	e.mapAccount(accountID, tenantID, authModels.AccountTenantStatusActive)

	adminRole := &authModels.AccountRole{AccountID: accountID, RoleID: e.adminRoleID}
	adminRole.SetTenantID(tenantID)
	_, err := e.db.NewInsert().Model(adminRole).ModelTableExpr("auth.account_roles").Exec(context.Background())
	require.NoError(e.t, err)
}

func (e *backfillTenantEnv) insertSubscription(accountID, tenantID int64, portal string) {
	e.t.Helper()
	sub := &iotModels.PushSubscription{
		AccountID: accountID,
		Portal:    portal,
		Endpoint:  fmt.Sprintf("https://fcm.googleapis.com/backfill-%d-%d-%s", accountID, tenantID, portal),
		P256dh:    "test-p256dh",
		Auth:      "test-auth",
	}
	sub.SetTenantID(tenantID)
	_, err := e.db.NewInsert().Model(sub).ModelTableExpr("iot.push_subscriptions").Exec(context.Background())
	require.NoError(e.t, err)
}

func (e *backfillTenantEnv) consentOf(accountID int64) map[string]bool {
	e.t.Helper()
	var rows []struct {
		NotificationType string `bun:"notification_type"`
		Enabled          bool   `bun:"enabled"`
	}
	require.NoError(e.t, e.db.NewSelect().
		TableExpr("users.notification_preferences").
		ColumnExpr("notification_type, enabled").
		Where("account_id = ?", accountID).
		Scan(context.Background(), &rows))
	out := make(map[string]bool, len(rows))
	for _, row := range rows {
		out[row.NotificationType] = row.Enabled
	}
	return out
}
