package compose

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The hourly demo run never waits for a lock (#5540): from RetireDemoSchools
// on, every lock is taken with a timeout far below deadlock_timeout or not at
// all, and an order whose lock is in use is deferred to the next run,
// completely unchanged. These tests hold each kind of lock from another
// transaction, in both orders against the run, and compare the deferred
// order with a snapshot.

// quickly is the bound a run that meets a lock in use returns within:
// deadlock_timeout (1 s). Each lock wait of the run ends after lock_timeout
// (100 ms, TestDemoVisitorErasureNeverWaitsForALock); the rest is the work of
// the run, which a loaded test machine slows down. That the run does not wait
// for the holder at all shows in every test: the holder keeps its lock until
// the run has returned, and a waiting run would run into retireNow's guard.
const quickly = time.Second

// retireNow runs the demo run with the named orders (nil: none) and fails
// the test when it does not return quickly.
func retireNow(t *testing.T, queue *organizationtenancy.DemoSchoolQueue, slugs []string) (hidden, erased int) {
	t.Helper()
	type outcome struct {
		hidden, erased int
		err            error
	}
	done := make(chan outcome, 1)
	started := time.Now()
	go func() {
		hidden, erased, err := queue.RetireDemoSchools(t.Context(), slugs)
		done <- outcome{hidden: hidden, erased: erased, err: err}
	}()
	select {
	case result := <-done:
		require.NoError(t, result.err)
		assert.Less(t, time.Since(started), quickly, "the run returns quickly")
		return result.hidden, result.erased
	case <-time.After(5 * time.Second):
		t.Fatal("the run waited for a lock another transaction holds")
		return 0, 0
	}
}

// lockWaits reports whether another session runs a statement containing
// marker and waits for a lock.
func lockWaits(t *testing.T, db *bun.DB, marker string) bool {
	t.Helper()
	var waiting int
	err := db.NewRaw(`SELECT count(*) FROM pg_stat_activity
		WHERE datname = current_database() AND pid <> pg_backend_pid()
			AND wait_event_type = 'Lock' AND strpos(query, ?) > 0`, marker).Scan(t.Context(), &waiting)
	return err == nil && waiting > 0
}

// visitorSnapshot holds, per table, every row of the order the run may
// change, as JSON: names, mails, notifications, the accounts and their
// sessions, memberships, roles, profiles with bio and settings, read
// receipts, push subscriptions, the school and the order.
type visitorSnapshot map[string]string

// snapshotRows selects the rows of a table that belong to the order: ?0 are
// the visitor accounts, ?1 the demo school, ?2 the order.
var snapshotRows = map[string]string{
	"auth.accounts":                    `FROM auth.accounts AS r WHERE r.id IN (?0)`,
	"auth.account_tenants":             `FROM auth.account_tenants AS r WHERE r.account_id IN (?0)`,
	"auth.account_roles":               `FROM auth.account_roles AS r WHERE r.account_id IN (?0)`,
	"auth.tokens":                      `FROM auth.tokens AS r WHERE r.account_id IN (?0)`,
	"auth.guardian_student_access":     `FROM auth.guardian_student_access AS r WHERE r.account_id IN (?0)`,
	"auth.demo_accesses":               `FROM auth.demo_accesses AS r WHERE r.account_id IN (?0) OR r.parent_account_id IN (?0)`,
	"users.profiles":                   `FROM users.profiles AS r WHERE r.account_id IN (?0)`,
	"users.persons":                    `FROM users.persons AS r WHERE r.tenant_id = ?1`,
	"users.guardian_profiles":          `FROM users.guardian_profiles AS r WHERE r.tenant_id = ?1`,
	"users.parent_messages":            `FROM users.parent_messages AS r WHERE r.tenant_id = ?1`,
	"users.parent_message_reads":       `FROM users.parent_message_reads AS r WHERE r.account_id IN (?0)`,
	"enrollment.requests":              `FROM enrollment.requests AS r WHERE r.tenant_id = ?1`,
	"enrollment.request_reads":         `FROM enrollment.request_reads AS r WHERE r.account_id IN (?0)`,
	"audit.guardian_changes":           `FROM audit.guardian_changes AS r WHERE r.tenant_id = ?1`,
	"platform.email_outbox":            `FROM platform.email_outbox AS r WHERE r.tenant_id = ?1`,
	"platform.email_delivery":          `FROM platform.email_delivery AS r WHERE r.tenant_id = ?1`,
	"platform.realtime_notifications":  `FROM platform.realtime_notifications AS r WHERE r.tenant_id = ?1`,
	"iot.push_subscriptions":           `FROM iot.push_subscriptions AS r WHERE r.account_id IN (?0) OR r.tenant_id = ?1`,
	"auth.account_avatar_file_cleanup": `FROM auth.account_avatar_file_cleanup AS r`,
	"platform.schools":                 `FROM platform.schools AS r WHERE r.id = ?1`,
	"platform.demo_school_states":      `FROM (SELECT to_jsonb(s) - 'erasure_deferrals' AS row FROM platform.demo_school_states AS s WHERE s.name = ?2) AS r`,
}

func snapshotVisitor(t *testing.T, db *bun.DB, schoolID int64, accounts visitorAccounts, slug string) visitorSnapshot {
	t.Helper()
	snapshot := make(visitorSnapshot, len(snapshotRows))
	ids := bun.List([]int64{accounts.caregiver, accounts.parent})
	for table, rows := range snapshotRows {
		row := "to_jsonb(r)"
		if table == "platform.demo_school_states" {
			row = "r.row"
		}
		var content string
		require.NoError(t, db.NewRaw(`SELECT COALESCE(jsonb_agg(`+row+` ORDER BY `+row+`::text), '[]'::jsonb)::text `+rows, ids, schoolID, slug).
			Scan(t.Context(), &content), table)
		snapshot[table] = content
	}
	return snapshot
}

// demoOrder is a finished order with one trace of the visitor in every
// place the run changes: names, mails, a notification, a push subscription,
// a profile, a role and a session of each account in the demo school.
type demoOrder struct {
	db       *bun.DB
	schoolID int64
	realID   int64
	accounts visitorAccounts
	queue    *organizationtenancy.DemoSchoolQueue
	slug     string
}

func newDemoOrder(t *testing.T) demoOrder {
	t.Helper()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := t.Context()
	schoolID, _ := testpkg.CreateTestTenant(t, db)
	realID, _ := testpkg.CreateTestTenant(t, db)
	accounts := leaveVisitorTraces(t, db, schoolID)
	var roleID int64
	require.NoError(t, db.NewRaw(`SELECT id FROM auth.roles ORDER BY id LIMIT 1`).Scan(ctx, &roleID))
	for _, accountID := range []int64{accounts.caregiver, accounts.parent} {
		onlyInSchools(t, db, accountID, schoolID)
		for _, statement := range []string{
			`INSERT INTO auth.account_roles (account_id, role_id, tenant_id) VALUES (?0, ?2, ?1)`,
			`INSERT INTO users.profiles (tenant_id, account_id, bio, avatar, settings)
				VALUES (?1, ?0, 'Ich bin Anna Lena', '/uploads/avatars/lock-' || ?0 || '.png', '{"theme": "dunkel"}'::jsonb)`,
			`INSERT INTO enrollment.request_reads (tenant_id, request_id, account_id)
				SELECT ?1, id, ?0 FROM enrollment.requests WHERE tenant_id = ?1`,
			`INSERT INTO users.parent_message_reads (tenant_id, thread_id, account_id)
				SELECT ?1, id, ?0 FROM users.parent_message_threads WHERE tenant_id = ?1`,
			`INSERT INTO auth.tokens (account_id, token, expiry, tenant_id, portal_scope)
				VALUES (?0, encode(sha256(convert_to('lock-' || ?0::text, 'UTF8')), 'hex'), NOW() + INTERVAL '1 day', ?1, 'tenant')`,
		} {
			_, err := db.NewRaw(statement, accountID, schoolID, roleID).Exec(ctx)
			require.NoError(t, err, statement)
		}
		pushSubscription(t, db, schoolID, accountID, "staff")
	}
	_, err := db.NewRaw(`INSERT INTO platform.realtime_notifications (tenant_id, scope, content)
		VALUES (?, 'tenant', jsonb_build_object('title', ?))`, schoolID, "Neue Nachricht von "+visitorFull).Exec(ctx)
	require.NoError(t, err)
	queue, slug := finishedVisitorOrder(t, db, schoolID, accounts)
	return demoOrder{db: db, schoolID: schoolID, realID: realID, accounts: accounts, queue: queue, slug: slug}
}

// newHiddenDemoOrder is an order whose school the run hid an hour ago and
// whose visitor it has still to erase.
func newHiddenDemoOrder(t *testing.T) demoOrder {
	t.Helper()
	order := newDemoOrder(t)
	order.hide(t)
	return order
}

func (o demoOrder) snapshot(t *testing.T) visitorSnapshot {
	t.Helper()
	return snapshotVisitor(t, o.db, o.schoolID, o.accounts, o.slug)
}

// hide retires the school of the order without erasing it, the state the
// erasure finds an order in an hour after it was hidden.
func (o demoOrder) hide(t *testing.T) {
	t.Helper()
	_, err := o.db.NewRaw(`UPDATE platform.schools SET deleted_at = NOW() WHERE id = ?`, o.schoolID).Exec(t.Context())
	require.NoError(t, err)
}

// requireErasedNext runs the next run once nothing holds a lock any more,
// without naming the order: the run finds deferred work in the database
// (#5540). It requires that the run hides a school still open and erases the
// order completely.
func (o demoOrder) requireErasedNext(t *testing.T) {
	t.Helper()
	open := o.snapshot(t)["platform.schools"]
	var visible bool
	require.NoError(t, o.db.NewRaw(`SELECT deleted_at IS NULL FROM platform.schools WHERE id = ?`, o.schoolID).Scan(t.Context(), &visible))
	hidden, erased := retireNow(t, o.queue, nil)
	if visible {
		require.Equal(t, 1, hidden, "the next run hides the school the previous one deferred: %s", open)
	}
	require.Equal(t, 1, erased, "the next run erases the deferred order")
	assert.Equal(t, everyTrace(0), visitorTraces(t, o.db, o.schoolID))
	assert.NotNil(t, stateOfAccount(t, o.db, o.accounts.caregiver).Erased)
	requireNothingLeftToErase(t, o.queue)
}

type step func(ctx context.Context, tx bun.Tx) error

func statement(query string, args ...any) step {
	return func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.NewRaw(query, args...).Exec(ctx)
		return err
	}
}

// writerFirst: another transaction holds a lock the run needs; the run
// returns quickly, defers the order unchanged, and the writer commits.
func (o demoOrder) writerFirst(t *testing.T, steps ...step) {
	t.Helper()
	ctx := t.Context()
	before := o.snapshot(t)
	writer, err := o.db.BeginTx(ctx, &sql.TxOptions{})
	require.NoError(t, err)
	defer func() { _ = writer.Rollback() }()
	for _, run := range steps {
		require.NoError(t, run(ctx, writer))
	}
	_, erased := retireNow(t, o.queue, []string{o.slug})
	assert.Zero(t, erased, "the order waits for the next run")
	assert.Equal(t, before, o.snapshot(t), "a deferred order is completely unchanged")
	require.NoError(t, writer.Commit(), "the writer does not fail beside the run")
}

// erasureFirst: the run has already written rows of the order and waits for
// a lock the writer holds (hold); meanwhile the writer needs one of those
// rows (write, a statement containing marker). That is a lock cycle, which
// before #5540 ended in deadlock_detected for one side. The run must give up
// first: neither fails, and the order is deferred unchanged. The run waits
// at most lock_timeout, so the test repeats until it saw the writer waiting
// for the run.
func (o demoOrder) erasureFirst(t *testing.T, hold []step, marker string, write step) {
	t.Helper()
	ctx := t.Context()
	before := o.snapshot(t)
	type outcome struct {
		erased int
		err    error
	}
	for attempt := 1; attempt <= 30; attempt++ {
		writer, err := o.db.BeginTx(ctx, &sql.TxOptions{})
		require.NoError(t, err)
		for _, run := range hold {
			require.NoError(t, run(ctx, writer))
		}
		erasure := make(chan outcome, 1)
		go func() {
			_, erased, retireErr := o.queue.RetireDemoSchools(ctx, []string{o.slug})
			erasure <- outcome{erased: erased, err: retireErr}
		}()
		var result outcome
		finished := false
		for !finished && !lockWaits(t, o.db, "retire_expired_demo_schools") {
			select {
			case result = <-erasure:
				finished = true
			default:
			}
		}
		writerWaited := false
		var writeErr error
		if !finished {
			written := make(chan error, 1)
			go func() { written <- write(ctx, writer) }()
			wrote := false
			for !finished || !wrote {
				select {
				case result = <-erasure:
					finished = true
				case writeErr = <-written:
					wrote = true
				default:
					if !finished && !wrote && lockWaits(t, o.db, marker) {
						writerWaited = true
					}
				}
			}
		}
		require.NoError(t, result.err, "the run does not fail beside the writer")
		require.NoError(t, writeErr, "the writer does not fail beside the run")
		assert.Zero(t, result.erased, "the order waits for the next run")
		require.NoError(t, writer.Rollback())
		require.Equal(t, before, o.snapshot(t), "a deferred order is completely unchanged")
		if writerWaited {
			t.Logf("the writer waited for the run in attempt %d", attempt)
			return
		}
	}
	t.Fatal("the writer never waited for the run; the cycle was not exercised")
}

func keyShare(accountID int64) step {
	// An earlier statement of the writer that references the account, such
	// as a session insert or a trigger check, holds it with a key share lock.
	return statement(`SELECT 1 FROM auth.accounts WHERE id = ? FOR KEY SHARE`, accountID)
}

// EnsureStaffCalendarFeedToken: an UPDATE of the membership locks the row,
// then its trigger auth.lock_live_account_reference the account.
func calendarToken(accountID, schoolID int64) step {
	return statement(`UPDATE auth.account_tenants SET staff_calendar_feed_token = 'feed-token-hash' /* calendar token */
		WHERE account_id = ? AND tenant_id = ? AND status = 'active'
		AND (staff_calendar_feed_token IS NULL OR staff_calendar_feed_token = '')`, accountID, schoolID)
}

func TestDemoVisitorErasureMeetsACalendarTokenWriterFirst(t *testing.T) {
	t.Parallel()
	order := newHiddenDemoOrder(t)
	order.writerFirst(t, calendarToken(order.accounts.caregiver, order.schoolID))
	order.requireErasedNext(t)
}

func TestDemoVisitorErasureMeetsACalendarTokenErasureFirst(t *testing.T) {
	t.Parallel()
	order := newHiddenDemoOrder(t)
	order.erasureFirst(t, []step{keyShare(order.accounts.caregiver)}, "calendar token",
		calendarToken(order.accounts.caregiver, order.schoolID))
	order.requireErasedNext(t)
}

// EnsureActiveTenantMapping and AssignAccountRole write with upserts: the
// trigger locks the account, then the upsert waits for a conflicting row.
func membershipUpsert(accountID, schoolID int64) step {
	return statement(`INSERT INTO auth.account_tenants (account_id, tenant_id, status) VALUES (?, ?, 'active') /* membership upsert */
		ON CONFLICT (account_id, tenant_id) DO UPDATE SET status = 'active'`, accountID, schoolID)
}

func roleUpsert(accountID, schoolID int64) step {
	return statement(`INSERT INTO auth.account_roles (account_id, role_id, tenant_id) /* role upsert */
		SELECT ?, id, ? FROM auth.roles ORDER BY id LIMIT 1
		ON CONFLICT DO NOTHING`, accountID, schoolID)
}

func TestDemoVisitorErasureMeetsAMembershipUpsertWriterFirst(t *testing.T) {
	t.Parallel()
	order := newHiddenDemoOrder(t)
	order.writerFirst(t, membershipUpsert(order.accounts.caregiver, order.schoolID))
	order.requireErasedNext(t)
}

func TestDemoVisitorErasureMeetsAMembershipUpsertErasureFirst(t *testing.T) {
	t.Parallel()
	order := newHiddenDemoOrder(t)
	order.erasureFirst(t, []step{keyShare(order.accounts.caregiver)}, "membership upsert",
		membershipUpsert(order.accounts.caregiver, order.schoolID))
	order.requireErasedNext(t)
}

func TestDemoVisitorErasureMeetsARoleUpsertWriterFirst(t *testing.T) {
	t.Parallel()
	order := newHiddenDemoOrder(t)
	order.writerFirst(t, roleUpsert(order.accounts.caregiver, order.schoolID))
	order.requireErasedNext(t)
}

func TestDemoVisitorErasureMeetsARoleUpsertErasureFirst(t *testing.T) {
	t.Parallel()
	order := newHiddenDemoOrder(t)
	order.erasureFirst(t, []step{keyShare(order.accounts.caregiver)}, "role upsert",
		roleUpsert(order.accounts.caregiver, order.schoolID))
	order.requireErasedNext(t)
}

// RemoveRoleFromAccount locks the account, deletes the role and then every
// session of the account; the run deletes the sessions of the demo school,
// then its roles.
func roleRemoval(accountID int64) []step {
	return []step{
		statement(`SELECT id FROM auth.accounts WHERE id = ? FOR UPDATE`, accountID),
		statement(`DELETE FROM auth.account_roles WHERE account_id = ?`, accountID),
	}
}

func sessionRemoval(accountID int64) step {
	return statement(`DELETE FROM auth.tokens WHERE account_id = ? /* session removal */`, accountID)
}

func TestDemoVisitorErasureMeetsARoleRemovalWriterFirst(t *testing.T) {
	t.Parallel()
	order := newHiddenDemoOrder(t)
	order.writerFirst(t, append(roleRemoval(order.accounts.caregiver), sessionRemoval(order.accounts.caregiver))...)
	order.requireErasedNext(t)
}

func TestDemoVisitorErasureMeetsARoleRemovalErasureFirst(t *testing.T) {
	t.Parallel()
	order := newHiddenDemoOrder(t)
	order.erasureFirst(t, roleRemoval(order.accounts.caregiver), "session removal", sessionRemoval(order.accounts.caregiver))
	order.requireErasedNext(t)
}

// UpdateProfile holds the account first (the user name), then upserts the
// bio, in the demo school or in a real school of a shared account.
func profileChange(accountID, schoolID int64) []step {
	return []step{
		statement(`UPDATE auth.accounts SET username = 'profil-' || id WHERE id = ?`, accountID),
		statement(`INSERT INTO users.profiles (account_id, tenant_id, bio, settings) VALUES (?, ?, 'nachher', '{}'::jsonb)
			ON CONFLICT (tenant_id, account_id) DO UPDATE SET bio = EXCLUDED.bio`, accountID, schoolID),
	}
}

func TestDemoVisitorErasureMeetsADemoSchoolProfileChange(t *testing.T) {
	t.Parallel()
	order := newHiddenDemoOrder(t)
	order.writerFirst(t, profileChange(order.accounts.caregiver, order.schoolID)...)
	order.requireErasedNext(t)
}

func TestDemoVisitorErasureMeetsARealSchoolProfileChange(t *testing.T) {
	t.Parallel()
	order := newHiddenDemoOrder(t)
	testpkg.MapAccountToTenant(t, order.db, order.accounts.parent, order.realID)
	order.writerFirst(t, profileChange(order.accounts.parent, order.realID)...)
	_, erased := retireNow(t, order.queue, nil)
	require.Equal(t, 1, erased)
	assert.Nil(t, stateOfAccount(t, order.db, order.accounts.parent).Erased, "the shared account stays")
	var bio string
	require.NoError(t, order.db.NewRaw(`SELECT bio FROM users.profiles WHERE account_id = ? AND tenant_id = ?`,
		order.accounts.parent, order.realID).Scan(t.Context(), &bio))
	assert.Equal(t, "nachher", bio)
}

// A school or Träger membership written while the run checks the accounts:
// the run defers the order unchanged, and the next run sees the membership
// and keeps the account. An erased account cannot be taken on afterwards.
func requireMembershipKeepsTheAccount(t *testing.T, membership func(order demoOrder) step) {
	t.Helper()
	order := newHiddenDemoOrder(t)
	caregiver := stateOfAccount(t, order.db, order.accounts.caregiver)
	order.writerFirst(t, membership(order))
	_, erased := retireNow(t, order.queue, nil)
	require.Equal(t, 1, erased)
	assert.Equal(t, caregiver, stateOfAccount(t, order.db, order.accounts.caregiver),
		"an account that got a real membership while the run ran keeps its address and access")
	parent := stateOfAccount(t, order.db, order.accounts.parent)
	assert.Equal(t, fmt.Sprintf("demo-besuch-%d@example.invalid", order.accounts.parent), parent.Email)
	assert.False(t, parent.Active)
	assert.NotNil(t, parent.Erased, "the demo-only account is erased")
	_, err := order.db.NewRaw(`INSERT INTO auth.account_tenants (account_id, tenant_id, status) VALUES (?, ?, 'active')`,
		order.accounts.parent, order.realID).Exec(t.Context())
	require.ErrorContains(t, err, "cannot reference an erased account")
	requireNothingLeftToErase(t, order.queue)
}

func TestDemoVisitorErasureDefersWhileASchoolMembershipIsWritten(t *testing.T) {
	t.Parallel()
	requireMembershipKeepsTheAccount(t, func(order demoOrder) step {
		return statement(`INSERT INTO auth.account_tenants (account_id, tenant_id, status) VALUES (?, ?, 'active')`,
			order.accounts.caregiver, order.realID)
	})
}

func TestDemoVisitorErasureDefersWhileATraegerMembershipIsWritten(t *testing.T) {
	t.Parallel()
	requireMembershipKeepsTheAccount(t, func(order demoOrder) step {
		return statement(`INSERT INTO platform.organization_accounts (organization_id, account_id, status, activated_at)
			SELECT organization_id, ?, 'active', NOW() FROM platform.schools WHERE id = ?`, order.accounts.caregiver, order.realID)
	})
}

// The locks of the run itself: the run lock, the seed and state locks of an
// order, the school row it hides and the state row it erases. Held by
// another transaction, each makes the run return quickly with the order
// unchanged; the next run handles it.
func runLock() step {
	return statement(`SELECT pg_advisory_xact_lock(hashtextextended('platform.demo_school_states:visitor-erasure', 0))`)
}

func seedLock(slug string) step {
	return statement(`SELECT pg_advisory_xact_lock(hashtextextended('demo school seed:' || ?, 3461))`, slug)
}

func stateLock(slug string) step {
	return statement(`SELECT pg_advisory_xact_lock(hashtextextended('platform.demo_school_states:' || ?, 0))`, slug)
}

func schoolRow(schoolID int64) step {
	return statement(`SELECT id FROM platform.schools WHERE id = ? FOR UPDATE`, schoolID)
}

func stateRow(slug string) step {
	return statement(`SELECT name FROM platform.demo_school_states WHERE name = ? FOR UPDATE`, slug)
}

// tableLock holds a table the run reads exclusively.
func tableLock(table string) step {
	return statement(`LOCK TABLE ` + table + ` IN ACCESS EXCLUSIVE MODE`)
}

// openOrderStates put an order in a state in which it is open (#5540): its
// school hidden in whatever state its seed left it, or failed. A ready
// order no access names whose school is still open is open for the
// retirement, which hides it.
var openOrderStates = map[string][]string{
	"waiting, hidden": {`UPDATE platform.demo_school_states SET status = 'preparing', claimed_at = NULL WHERE name = ?0`,
		`UPDATE platform.schools SET deleted_at = NOW() WHERE id = ?1`},
	"seeding, hidden": {`UPDATE platform.demo_school_states SET status = 'preparing', claimed_at = NOW() WHERE name = ?0`,
		`UPDATE platform.schools SET deleted_at = NOW() WHERE id = ?1`},
	"ready, hidden":  {`UPDATE platform.schools SET deleted_at = NOW() WHERE id = ?1`},
	"failed, hidden": {`UPDATE platform.demo_school_states SET status = 'failed' WHERE name = ?0`, `UPDATE platform.schools SET deleted_at = NOW() WHERE id = ?1`},
	"ready, open":    nil,
}

// runLocks are every lock on the path of the run.
var runLocks = map[string]func(order demoOrder) step{
	"run lock":                    func(demoOrder) step { return runLock() },
	"seed lock":                   func(order demoOrder) step { return seedLock(order.slug) },
	"state lock":                  func(order demoOrder) step { return stateLock(order.slug) },
	"school row":                  func(order demoOrder) step { return schoolRow(order.schoolID) },
	"state row":                   func(order demoOrder) step { return stateRow(order.slug) },
	"platform.demo_school_states": func(demoOrder) step { return tableLock("platform.demo_school_states") },
	"platform.schools":            func(demoOrder) step { return tableLock("platform.schools") },
	"auth.demo_accesses":          func(demoOrder) step { return tableLock("auth.demo_accesses") },
}

// visitorData is the snapshot without the school row: hiding the school is
// no visitor data, and its deleted_at may stay set while the erasure waits.
func visitorData(snapshot visitorSnapshot) visitorSnapshot {
	data := make(visitorSnapshot, len(snapshot))
	for table, rows := range snapshot {
		if table != "platform.schools" {
			data[table] = rows
		}
	}
	return data
}

// requireFullyErased checks that nothing of the visitor is left.
func (o demoOrder) requireFullyErased(t *testing.T) {
	t.Helper()
	assert.NotContains(t, demoOrderNames(t, o.db), o.slug, "the order is erased")
	assert.Equal(t, everyTrace(0), visitorTraces(t, o.db, o.schoolID))
	for _, accountID := range []int64{o.accounts.caregiver, o.accounts.parent} {
		assert.NotNil(t, stateOfAccount(t, o.db, accountID).Erased, "account %d is erased", accountID)
	}
	snapshot := o.snapshot(t)
	for _, table := range []string{"users.profiles", "auth.tokens", "auth.account_roles", "auth.account_tenants",
		"iot.push_subscriptions", "platform.email_outbox", "platform.realtime_notifications", "enrollment.request_reads"} {
		assert.Equal(t, "[]", snapshot[table], "%s keeps nothing of the visitor", table)
	}
}

// Every open order, in every state, meets every lock of the run: the run
// returns quickly without an error and leaves the visitor's data as it is or
// erases it completely. Once the lock is gone, runs that are told nothing and
// seed nothing erase it completely (#5540).
func TestDemoRunErasesEveryOpenOrderOnceItsLockIsGone(t *testing.T) {
	t.Parallel()
	for state, setup := range openOrderStates {
		for lock, hold := range runLocks {
			t.Run(state+"/"+lock, func(t *testing.T) {
				order := newDemoOrder(t)
				ctx := t.Context()
				for _, statement := range setup {
					_, err := order.db.NewRaw(statement, order.slug, order.schoolID).Exec(ctx)
					require.NoError(t, err, statement)
				}
				before := visitorData(order.snapshot(t))
				holder, err := order.db.BeginTx(ctx, &sql.TxOptions{})
				require.NoError(t, err)
				defer func() { _ = holder.Rollback() }()
				require.NoError(t, hold(order)(ctx, holder))
				_, erased := retireNow(t, order.queue, []string{order.slug})
				require.NoError(t, holder.Rollback())
				if erased == 0 {
					assert.Equal(t, before, visitorData(order.snapshot(t)), "a deferred order keeps the visitor's data as it was")
				}
				for run := 0; run < 3 && slices.Contains(demoOrderNames(t, order.db), order.slug); run++ {
					retireNow(t, order.queue, nil)
				}
				order.requireFullyErased(t)
			})
		}
	}
}

// An order still waiting for its seed whose access expired: the run closes
// it. Deferred while its seed lock is held, it is not lost: the demo process
// seeds it, and the next run, told nothing, finds it ready without an access
// and retires and erases it (#5540).
func TestDemoRunFindsADeferredWaitingOrderOnceSeeded(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := t.Context()
	schoolID, _ := testpkg.CreateTestTenant(t, db)
	queue, err := NewDemoSchoolQueue(db)
	require.NoError(t, err)
	schools, err := NewDemoSchools(db)
	require.NoError(t, err)
	slug := orderDemoSchool(t, db, "OGS Nord")

	holder, err := db.BeginTx(ctx, &sql.TxOptions{})
	require.NoError(t, err)
	defer func() { _ = holder.Rollback() }()
	require.NoError(t, seedLock(slug)(ctx, holder))
	hidden, erased := retireNow(t, queue, []string{slug})
	assert.Zero(t, hidden)
	assert.Zero(t, erased)
	assert.Equal(t, organizationtenancy.DemoSchoolPreparing, demoSchoolProgress(t, db, slug).Status, "the deferred order still waits")
	require.NoError(t, holder.Rollback())

	_, err = queue.ClaimDemoSchoolOrder(ctx)
	require.NoError(t, err)
	require.NoError(t, schools.RememberDemoSchool(ctx, slug, organizationtenancy.DemoSchoolState{SchoolID: schoolID, SeedJSON: []byte(`{}`)}))
	require.NoError(t, queue.FinishDemoSchoolOrder(ctx, slug, 0, 0))
	hidden, erased = retireNow(t, queue, nil)
	assert.Equal(t, 1, hidden, "the next run hides the seeded school no access names")
	assert.Equal(t, 1, erased)
	assert.Nil(t, demoSchoolProgress(t, db, slug))
}

// Many orders wait for a lock at once: the run stops at its budget, well
// below deadlock_timeout, and the next runs, told nothing, erase all of them
// once the locks are gone (#5540).
func TestDemoRunKeepsItsBudgetWhenManyOrdersWait(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := t.Context()
	queue, err := NewDemoSchoolQueue(db)
	require.NoError(t, err)
	schools, err := NewDemoSchools(db)
	require.NoError(t, err)
	names := make([]string, 0, 14)
	for i := range 14 {
		names = append(names, fmt.Sprintf("OGS Nord %d", i))
	}
	slugs := hiddenOrders(t, db, queue, schools, names...)
	holder, err := db.BeginTx(ctx, &sql.TxOptions{})
	require.NoError(t, err)
	defer func() { _ = holder.Rollback() }()
	for _, slug := range slugs {
		require.NoError(t, seedLock(slug)(ctx, holder))
	}

	_, erased := retireNow(t, queue, slugs)
	assert.Zero(t, erased)
	require.NoError(t, holder.Rollback())
	for run := 0; run < 3 && len(demoOrderNames(t, db)) > 0; run++ {
		retireNow(t, queue, nil)
	}
	assert.Empty(t, demoOrderNames(t, db), "the next runs erase every order")
}

// hiddenOrders orders, seeds and hides one demo school per name, the state
// an order is in an hour after its last access expired.
func hiddenOrders(t *testing.T, db *bun.DB, queue *organizationtenancy.DemoSchoolQueue, schools *organizationtenancy.DemoSchools, names ...string) []string {
	t.Helper()
	ctx := t.Context()
	slugs := make([]string, 0, len(names))
	for _, name := range names {
		schoolID, _ := testpkg.CreateTestTenant(t, db)
		slug := orderDemoSchool(t, db, name)
		_, err := queue.ClaimDemoSchoolOrder(ctx)
		require.NoError(t, err)
		require.NoError(t, schools.RememberDemoSchool(ctx, slug, organizationtenancy.DemoSchoolState{SchoolID: schoolID, SeedJSON: []byte(`{}`)}))
		require.NoError(t, queue.FinishDemoSchoolOrder(ctx, slug, 0, 0))
		_, err = db.NewRaw(`UPDATE platform.schools SET deleted_at = NOW() WHERE id = ?`, schoolID).Exec(ctx)
		require.NoError(t, err)
		slugs = append(slugs, slug)
	}
	return slugs
}

// Orders that stay locked run after run do not starve the others behind
// them: a deferral puts an order behind the orders not deferred as often,
// so the run's budget reaches the unlocked orders (#5540).
func TestDemoRunReachesTheOrdersBehindLockedOnes(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := t.Context()
	queue, err := NewDemoSchoolQueue(db)
	require.NoError(t, err)
	schools, err := NewDemoSchools(db)
	require.NoError(t, err)
	locked := hiddenOrders(t, db, queue, schools, "OGS A 1", "OGS A 2", "OGS A 3", "OGS A 4", "OGS A 5", "OGS A 6", "OGS A 7")
	free := hiddenOrders(t, db, queue, schools, "OGS Z 1", "OGS Z 2")
	holder, err := db.BeginTx(ctx, &sql.TxOptions{})
	require.NoError(t, err)
	defer func() { _ = holder.Rollback() }()
	for _, slug := range locked {
		require.NoError(t, seedLock(slug)(ctx, holder))
	}

	for run := 0; run < 4; run++ {
		retireNow(t, queue, nil)
	}
	remaining := demoOrderNames(t, db)
	for _, slug := range free {
		assert.NotContains(t, remaining, slug, "an order behind locked ones is erased while they stay locked")
	}
	assert.Len(t, remaining, len(locked))
}

// openOrders orders and seeds one demo school per name that no access
// names any more and that is still open: the run retires it.
func openOrders(t *testing.T, db *bun.DB, queue *organizationtenancy.DemoSchoolQueue, schools *organizationtenancy.DemoSchools, names ...string) []string {
	t.Helper()
	slugs := hiddenOrders(t, db, queue, schools, names...)
	_, err := db.NewRaw(`UPDATE platform.schools SET deleted_at = NULL
		WHERE id IN (SELECT tenant_id FROM platform.demo_school_states WHERE name IN (?))`, bun.List(slugs)).Exec(t.Context())
	require.NoError(t, err)
	return slugs
}

// Open orders that stay locked do not take the erasure's turn: the
// retirement and the erasure each have a budget of their own, so the visitor
// of a hidden order is erased while they stay locked (#5540).
func TestDemoRunErasesWhileTheRetirementWaits(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := t.Context()
	queue, err := NewDemoSchoolQueue(db)
	require.NoError(t, err)
	schools, err := NewDemoSchools(db)
	require.NoError(t, err)
	locked := openOrders(t, db, queue, schools, "OGS A 1", "OGS A 2", "OGS A 3", "OGS A 4", "OGS A 5", "OGS A 6", "OGS A 7")
	hidden := hiddenOrders(t, db, queue, schools, "OGS Z 1")
	holder, err := db.BeginTx(ctx, &sql.TxOptions{})
	require.NoError(t, err)
	defer func() { _ = holder.Rollback() }()
	for _, slug := range locked {
		require.NoError(t, seedLock(slug)(ctx, holder))
	}

	for run := 0; run < 3; run++ {
		retireNow(t, queue, nil)
	}
	assert.NotContains(t, demoOrderNames(t, db), hidden[0], "the hidden order is erased while the open ones stay locked")
}

// State rows that stay locked do not starve the orders behind them: the
// erasure does not wait for a state row, so a locked one costs the run's
// budget nothing (#5540).
func TestDemoRunReachesTheOrdersBehindLockedStateRows(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := t.Context()
	queue, err := NewDemoSchoolQueue(db)
	require.NoError(t, err)
	schools, err := NewDemoSchools(db)
	require.NoError(t, err)
	locked := hiddenOrders(t, db, queue, schools, "OGS A 1", "OGS A 2", "OGS A 3", "OGS A 4")
	free := hiddenOrders(t, db, queue, schools, "OGS Z 1", "OGS Z 2")
	holder, err := db.BeginTx(ctx, &sql.TxOptions{})
	require.NoError(t, err)
	defer func() { _ = holder.Rollback() }()
	for _, slug := range locked {
		require.NoError(t, stateRow(slug)(ctx, holder))
	}

	for run := 0; run < 3; run++ {
		retireNow(t, queue, nil)
	}
	remaining := demoOrderNames(t, db)
	for _, slug := range free {
		assert.NotContains(t, remaining, slug, "an order behind locked state rows is erased while they stay locked")
	}
	assert.Len(t, remaining, len(locked))
}

// Locked state rows take no place in the erasure's batch: the erasure picks
// its orders past them, so a free order behind fifty locked ones is erased in
// the first run, and the locked ones, still open, once they are free, without
// being named (#5540).
func TestDemoRunErasesAFreeOrderBehindFiftyLockedStateRows(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := t.Context()
	queue, err := NewDemoSchoolQueue(db)
	require.NoError(t, err)
	schools, err := NewDemoSchools(db)
	require.NoError(t, err)
	names := make([]string, 0, 50)
	for i := range 50 {
		names = append(names, fmt.Sprintf("OGS A %02d", i))
	}
	locked := hiddenOrders(t, db, queue, schools, names...)
	free := hiddenOrders(t, db, queue, schools, "OGS Z 1")
	holder, err := db.BeginTx(ctx, &sql.TxOptions{})
	require.NoError(t, err)
	defer func() { _ = holder.Rollback() }()
	for _, slug := range locked {
		require.NoError(t, stateRow(slug)(ctx, holder))
	}

	_, erased := retireNow(t, queue, nil)
	assert.Equal(t, 1, erased, "the first run erases the free order")
	assert.NotContains(t, demoOrderNames(t, db), free[0])
	assert.Len(t, demoOrderNames(t, db), len(locked), "the locked orders stay open")
	require.NoError(t, holder.Rollback())

	for run := 0; run < 5 && len(demoOrderNames(t, db)) > 0; run++ {
		retireNow(t, queue, nil)
	}
	assert.Empty(t, demoOrderNames(t, db), "the next runs, told nothing, erase the orders once they are free")
}

// An order meets a lock once per run: it is deferred, counted and logged
// once, by the retirement while its school is open, by the erasure once it
// is hidden (#5540).
func TestDemoRunDefersAnOrderOncePerRun(t *testing.T) {
	t.Parallel()
	for name, hidden := range map[string]bool{"school open": false, "school hidden": true} {
		t.Run(name, func(t *testing.T) {
			order := newDemoOrder(t)
			if hidden {
				order.hide(t)
			}
			ctx := t.Context()
			holder, err := order.db.BeginTx(ctx, &sql.TxOptions{})
			require.NoError(t, err)
			defer func() { _ = holder.Rollback() }()
			require.NoError(t, seedLock(order.slug)(ctx, holder))
			for run := 1; run <= 2; run++ {
				retireNow(t, order.queue, []string{order.slug})
				var deferrals int
				require.NoError(t, order.db.NewRaw(`SELECT erasure_deferrals FROM platform.demo_school_states WHERE name = ?`, order.slug).Scan(ctx, &deferrals))
				assert.Equal(t, run, deferrals, "one deferral per run")
			}
		})
	}
}

// An order the run defers is counted in its state row; a long run of
// deferrals becomes a warning (#5540).
func TestDemoVisitorErasureCountsItsDeferrals(t *testing.T) {
	t.Parallel()
	order := newHiddenDemoOrder(t)
	ctx := t.Context()
	holder, err := order.db.BeginTx(ctx, &sql.TxOptions{})
	require.NoError(t, err)
	defer func() { _ = holder.Rollback() }()
	require.NoError(t, keyShare(order.accounts.caregiver)(ctx, holder))
	for range 2 {
		_, erased := retireNow(t, order.queue, nil)
		assert.Zero(t, erased)
	}
	var deferrals int
	require.NoError(t, order.db.NewRaw(`SELECT erasure_deferrals FROM platform.demo_school_states WHERE name = ?`, order.slug).Scan(ctx, &deferrals))
	assert.Equal(t, 2, deferrals)
	require.NoError(t, holder.Rollback())
	order.requireErasedNext(t)
	assert.Empty(t, demoOrderNames(t, order.db), "an erased order leaves no state row, counter included")
}
