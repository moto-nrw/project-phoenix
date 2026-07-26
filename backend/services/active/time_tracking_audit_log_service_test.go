// Hermetic tests for the cross-staff audit feed (#1417): one tenant, every
// source populated once, then the merged feed asserted on grouping, name
// resolution, filters, and keyset pagination.
package active_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	active "github.com/moto-nrw/project-phoenix/services/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type auditLogFixture struct {
	tenantID int64
	staffA   *userModels.Staff
	staffB   *userModels.Staff
	admin    *userModels.Staff
	repos    *repositories.Factory
	db       *bun.DB
	ctx      context.Context
	svc      active.TimeTrackingAuditLogService
}

// newAuditLogFixture arranges one event per source:
//   - one session-edit save action with TWO field rows (must group to ONE event)
//   - one absence status transition whose actor is stored as an ACCOUNT id
//   - one balance adjustment for staffA
//   - a school-wide month close over staffA+staffB (must group to ONE event)
//   - a reopen for staffA
//   - one deleted adjustment for staffB (tombstone through the service path)
func newAuditLogFixture(t *testing.T) *auditLogFixture {
	t.Helper()

	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	staffA := testpkg.CreateTestStaffForTenant(t, db, tenantID, "Audit", "Betroffene")
	staffB := testpkg.CreateTestStaffForTenant(t, db, tenantID, "Audit", "Zweite")

	// The admin needs the account → person → staff chain IN this tenant: the
	// absence trail stores the actor as an account id and the feed must
	// resolve it. Built inline because no fixture creates the full chain for
	// an explicit tenant.
	adminAccount := testpkg.CreateTestAccount(t, db, "audit.leitung")
	adminPerson := &userModels.Person{FirstName: "Audit", LastName: "Leitung", AccountID: &adminAccount.ID}
	adminPerson.SetTenantID(tenantID)
	require.NoError(t, db.NewInsert().Model(adminPerson).ModelTableExpr("users.persons").Scan(context.Background()))
	admin := &userModels.Staff{PersonID: adminPerson.ID}
	admin.SetTenantID(tenantID)
	require.NoError(t, db.NewInsert().Model(admin).ModelTableExpr("users.staff").Scan(context.Background()))

	repos := repositories.NewFactory(db)
	ctx := testpkg.TenantContext(tenantID)

	t.Cleanup(func() {
		for _, table := range []string{
			"audit.time_tracking_deletions",
			"active.staff_month_balance_snapshots",
			"active.staff_balance_adjustments",
			"active.staff_absence_audit",
			"active.staff_absences",
			"audit.work_session_edits",
			"active.work_sessions",
		} {
			_, _ = db.NewDelete().ModelTableExpr(table+" AS t").Where("t.tenant_id = ?", tenantID).Exec(context.Background())
		}
		testpkg.CleanupStaffFixtures(t, db, staffA.ID, staffB.ID, admin.ID)
		testpkg.CleanupTableRecords(t, db, "users.persons", adminPerson.ID)
		testpkg.CleanupTableRecords(t, db, "auth.accounts", adminAccount.ID)
		testpkg.CleanupTenantTestData(t, db, tenantID)
	})

	// Session + two edit rows from one save action.
	checkIn := time.Date(2025, time.August, 4, 8, 0, 0, 0, time.UTC)
	checkOut := checkIn.Add(8 * time.Hour)
	session := &activeModels.WorkSession{
		StaffID:     staffA.ID,
		Date:        timezone.NewDate(2025, time.August, 4),
		Status:      activeModels.WorkSessionStatusPresent,
		Source:      activeModels.WorkSessionSourceApp,
		CheckInTime: checkIn, CheckOutTime: &checkOut,
		CreatedBy: staffA.ID,
	}
	session.SetTenantID(tenantID)
	require.NoError(t, repos.WorkSession.Create(ctx, session))
	oldIn, newIn := "08:00", "08:30"
	oldOut, newOut := "16:00", "15:00"
	note := "Korrektur nach Rücksprache"
	// One save action = one shared timestamp, exactly like the production
	// writer (work_session_service stamps every edit of a save with the same
	// `now`); the feed groups on that equality.
	editedAt := time.Date(2025, time.August, 5, 14, 0, 0, 0, time.UTC)
	edits := []*auditModels.WorkSessionEdit{
		{SessionID: session.ID, StaffID: staffA.ID, EditedBy: admin.ID, FieldName: "check_in_time", OldValue: &oldIn, NewValue: &newIn, Notes: &note, CreatedAt: editedAt},
		{SessionID: session.ID, StaffID: staffA.ID, EditedBy: admin.ID, FieldName: "check_out_time", OldValue: &oldOut, NewValue: &newOut, Notes: &note, CreatedAt: editedAt},
	}
	for _, e := range edits {
		e.SetTenantID(tenantID)
	}
	require.NoError(t, repos.WorkSessionEdit.CreateBatch(ctx, edits))

	// Absence + status-transition audit row (actor = ACCOUNT id).
	absence := &activeModels.StaffAbsence{
		StaffID:     staffA.ID,
		AbsenceType: activeModels.AbsenceTypeSick,
		DateStart:   timezone.NewDate(2025, time.August, 6),
		DateEnd:     timezone.NewDate(2025, time.August, 7),
		Status:      activeModels.AbsenceStatusReported,
		CreatedBy:   admin.ID,
	}
	absence.SetTenantID(tenantID)
	require.NoError(t, repos.StaffAbsence.Create(ctx, absence))
	fromStatus := activeModels.AbsenceStatusReported
	absenceAudit := &activeModels.StaffAbsenceAudit{
		AbsenceID:  absence.ID,
		FromStatus: &fromStatus,
		ToStatus:   activeModels.AbsenceStatusApproved,
		ActorID:    adminAccount.ID,
		Note:       "Attest liegt vor",
	}
	absenceAudit.SetTenantID(tenantID)
	require.NoError(t, repos.StaffAbsenceAudit.Create(ctx, absenceAudit))

	// Balance adjustment for staffA (stays) and one for staffB (deleted below).
	makeAdjustment := func(staffID int64, day int) *activeModels.StaffBalanceAdjustment {
		adj := &activeModels.StaffBalanceAdjustment{
			StaffID:       staffID,
			Type:          activeModels.BalanceAdjustmentTypePayout,
			MinutesDelta:  -60,
			EffectiveDate: timezone.NewDate(2025, time.August, day),
			Note:          "Auszahlung",
			DecidedBy:     admin.ID,
			DecidedAt:     time.Date(2025, time.August, day, 12, 0, 0, 0, time.UTC),
		}
		adj.SetTenantID(tenantID)
		require.NoError(t, repos.StaffBalanceAdjust.Create(ctx, adj))
		return adj
	}
	makeAdjustment(staffA.ID, 10)
	doomed := makeAdjustment(staffB.ID, 11)

	// School-wide month close (two accounts, one closed_at) + reopen of staffA.
	closedAt := time.Date(2025, time.September, 1, 9, 0, 0, 0, time.UTC)
	reopenedAt := time.Date(2025, time.September, 2, 10, 0, 0, 0, time.UTC)
	reopenReason := "Abschluss war verfrüht"
	for _, snap := range []*activeModels.StaffMonthBalanceSnapshot{
		{StaffID: staffA.ID, Year: 2025, Month: 8, ClosingBalanceMinutes: 30, ClosedAt: closedAt, ClosedBy: admin.ID, CloseReason: "Monatsabschluss August", Source: "admin", ReopenedAt: &reopenedAt, ReopenedBy: &admin.ID, ReopenReason: reopenReason},
		{StaffID: staffB.ID, Year: 2025, Month: 8, ClosingBalanceMinutes: -15, ClosedAt: closedAt, ClosedBy: admin.ID, CloseReason: "Monatsabschluss August", Source: "admin"},
	} {
		snap.SetTenantID(tenantID)
		_, err := db.NewInsert().Model(snap).ModelTableExpr("active.staff_month_balance_snapshots").Exec(ctx)
		require.NoError(t, err)
	}

	// Delete staffB's adjustment through the real service path so the
	// tombstone write is covered end to end.
	adjustSvc := active.NewStaffBalanceAdjustmentService(repos.StaffBalanceAdjust, nil, wtmIntSettings{accountStart: "2025-01-01"}, nil)
	adjustSvc.(interface {
		SetDeletionAudit(auditModels.TimeTrackingDeletionRepository)
	}).SetDeletionAudit(repos.TimeTrackingDeletion)
	require.NoError(t, adjustSvc.DeleteAdjustment(ctx, staffB.ID, doomed.ID, admin.ID))

	return &auditLogFixture{
		tenantID: tenantID, staffA: staffA, staffB: staffB, admin: admin,
		repos: repos, db: db, ctx: ctx,
		svc: active.NewTimeTrackingAuditLogService(repos.TimeTrackingAuditLog, repos.Staff, nil),
	}
}

func eventsBySource(events []*active.AuditLogEvent) map[string][]*active.AuditLogEvent {
	m := map[string][]*active.AuditLogEvent{}
	for _, e := range events {
		m[e.Source] = append(m[e.Source], e)
	}
	return m
}

func TestTimeTrackingAuditLog_MergedFeed(t *testing.T) {
	f := newAuditLogFixture(t)

	page, err := f.svc.ListAuditLog(f.ctx, active.AuditLogListRequest{})
	require.NoError(t, err)
	require.NotEmpty(t, page.RetentionCutoff)

	bySource := eventsBySource(page.Events)

	// One event per source; the two edit rows and the two close snapshots
	// each collapse into a single event.
	require.Len(t, bySource[auditModels.AuditLogSourceSessionEdit], 1, "two field edits of one save must group to one event")
	require.Len(t, bySource[auditModels.AuditLogSourceAbsence], 1)
	require.Len(t, bySource[auditModels.AuditLogSourceAdjustment], 1, "the deleted adjustment must vanish from the ledger source")
	require.Len(t, bySource[auditModels.AuditLogSourceMonthClose], 1, "school-wide close must group to one event")
	require.Len(t, bySource[auditModels.AuditLogSourceMonthReopen], 1)
	require.Len(t, bySource[auditModels.AuditLogSourceDeletion], 1)

	// Descending order over the merged feed.
	for i := 1; i < len(page.Events); i++ {
		assert.False(t, page.Events[i].OccurredAt.After(page.Events[i-1].OccurredAt), "feed must be sorted newest first")
	}
	for _, event := range page.Events {
		assert.Positive(t, event.EntryID, "every API event must expose its unique source entry id")
	}

	edit := bySource[auditModels.AuditLogSourceSessionEdit][0]
	require.NotNil(t, edit.Staff)
	assert.Equal(t, f.staffA.ID, edit.Staff.ID)
	assert.Equal(t, "Audit Leitung", edit.Actor.Name)
	assert.False(t, edit.Actor.IsSelf)
	assert.Equal(t, "Korrektur nach Rücksprache", edit.Reason)
	var editDetail struct {
		SessionID   int64  `json:"session_id"`
		SessionDate string `json:"session_date"`
		Fields      []struct {
			Field string  `json:"field"`
			Old   *string `json:"old"`
			New   *string `json:"new"`
		} `json:"fields"`
	}
	require.NoError(t, json.Unmarshal(edit.Detail, &editDetail))
	assert.Equal(t, "2025-08-04", editDetail.SessionDate)
	require.Len(t, editDetail.Fields, 2)

	// Absence actor arrives as an account id and must resolve to the admin.
	absence := bySource[auditModels.AuditLogSourceAbsence][0]
	require.NotNil(t, absence.Actor.StaffID)
	assert.Equal(t, f.admin.ID, *absence.Actor.StaffID)
	assert.Equal(t, "Attest liegt vor", absence.Reason)

	closeEvent := bySource[auditModels.AuditLogSourceMonthClose][0]
	assert.Nil(t, closeEvent.Staff, "school-wide close names no single person")
	var closeDetail struct {
		Year         int `json:"year"`
		Month        int `json:"month"`
		AccountCount int `json:"account_count"`
	}
	require.NoError(t, json.Unmarshal(closeEvent.Detail, &closeDetail))
	assert.Equal(t, 2, closeDetail.AccountCount)

	deletion := bySource[auditModels.AuditLogSourceDeletion][0]
	require.NotNil(t, deletion.Staff)
	assert.Equal(t, f.staffB.ID, deletion.Staff.ID)
	require.NotNil(t, deletion.Actor.StaffID)
	assert.Equal(t, f.admin.ID, *deletion.Actor.StaffID)
	var deletionDetail struct {
		DeletedSource string          `json:"deleted_source"`
		Payload       json.RawMessage `json:"payload"`
	}
	require.NoError(t, json.Unmarshal(deletion.Detail, &deletionDetail))
	assert.Equal(t, auditModels.TimeTrackingDeletionSourceBalanceAdjustment, deletionDetail.DeletedSource)
	assert.Contains(t, string(deletionDetail.Payload), `"minutes_delta"`)
}

func TestTimeTrackingAuditLog_Filters(t *testing.T) {
	f := newAuditLogFixture(t)

	t.Run("staff filter keeps grouped events the person is part of", func(t *testing.T) {
		page, err := f.svc.ListAuditLog(f.ctx, active.AuditLogListRequest{StaffID: f.staffA.ID})
		require.NoError(t, err)
		bySource := eventsBySource(page.Events)
		assert.Len(t, bySource[auditModels.AuditLogSourceSessionEdit], 1)
		assert.Len(t, bySource[auditModels.AuditLogSourceMonthClose], 1, "staffA is inside the grouped close")
		assert.Empty(t, bySource[auditModels.AuditLogSourceDeletion], "the deletion hit staffB only")
	})

	t.Run("actor filter", func(t *testing.T) {
		page, err := f.svc.ListAuditLog(f.ctx, active.AuditLogListRequest{ActorStaffID: f.admin.ID})
		require.NoError(t, err)
		require.NotEmpty(t, page.Events)
		for _, e := range page.Events {
			require.NotNil(t, e.Actor.StaffID)
			assert.Equal(t, f.admin.ID, *e.Actor.StaffID)
		}
		// Every fixture event was acted by the admin, so nothing is lost.
		assert.Len(t, page.Events, 6)
	})

	t.Run("source filter", func(t *testing.T) {
		page, err := f.svc.ListAuditLog(f.ctx, active.AuditLogListRequest{
			Sources: []string{auditModels.AuditLogSourceAdjustment, auditModels.AuditLogSourceDeletion},
		})
		require.NoError(t, err)
		require.Len(t, page.Events, 2)
	})

	t.Run("date range", func(t *testing.T) {
		from := timezone.NewDate(2025, time.September, 1)
		to := timezone.NewDate(2025, time.September, 1)
		page, err := f.svc.ListAuditLog(f.ctx, active.AuditLogListRequest{From: &from, To: &to})
		require.NoError(t, err)
		bySource := eventsBySource(page.Events)
		assert.Len(t, bySource[auditModels.AuditLogSourceMonthClose], 1)
		assert.Empty(t, bySource[auditModels.AuditLogSourceMonthReopen], "reopen happened on Sep 2")
	})

	t.Run("to date includes the final fractional second", func(t *testing.T) {
		day := timezone.NewDate(2025, time.September, 3)
		finalFraction := time.Date(2025, time.September, 3, 23, 59, 59, 500_000_000, timezone.Berlin)
		nextMidnight := day.AddDays(1).BerlinMidnight()
		for sourceID, occurredAt := range map[int64]time.Time{
			9001: finalFraction,
			9002: nextMidnight,
		} {
			require.NoError(t, f.repos.TimeTrackingDeletion.Create(f.ctx, &auditModels.TimeTrackingDeletion{
				StaffID:    f.staffA.ID,
				Source:     auditModels.TimeTrackingDeletionSourceAbsence,
				SourceID:   sourceID,
				DeletedBy:  f.admin.ID,
				Payload:    json.RawMessage(`{"test":true}`),
				Note:       "Datumsgrenze",
				OccurredAt: occurredAt,
			}))
		}

		page, err := f.svc.ListAuditLog(f.ctx, active.AuditLogListRequest{
			From:    &day,
			To:      &day,
			Sources: []string{auditModels.AuditLogSourceDeletion},
		})
		require.NoError(t, err)
		require.Len(t, page.Events, 1)
		assert.Equal(t, finalFraction, page.Events[0].OccurredAt)
	})

	t.Run("unknown source is invalid", func(t *testing.T) {
		_, err := f.svc.ListAuditLog(f.ctx, active.AuditLogListRequest{Sources: []string{"payroll"}})
		require.ErrorIs(t, err, active.ErrAuditLogInvalid)
	})
}

func TestTimeTrackingAuditLog_KeysetPagination(t *testing.T) {
	f := newAuditLogFixture(t)

	all, err := f.svc.ListAuditLog(f.ctx, active.AuditLogListRequest{})
	require.NoError(t, err)
	require.Len(t, all.Events, 6)
	require.Empty(t, all.NextCursor)

	seen := map[string]bool{}
	key := func(e *active.AuditLogEvent) string { return e.Source + e.OccurredAt.String() + string(e.Detail) }

	cursor := ""
	var pages int
	for {
		page, err := f.svc.ListAuditLog(f.ctx, active.AuditLogListRequest{Limit: 2, Cursor: cursor})
		require.NoError(t, err)
		for _, e := range page.Events {
			require.False(t, seen[key(e)], "cursor pages must not overlap")
			seen[key(e)] = true
		}
		pages++
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
		require.LessOrEqual(t, pages, 6, "pagination must terminate")
	}
	assert.Len(t, seen, 6, "every event appears exactly once across pages")

	_, err = f.svc.ListAuditLog(f.ctx, active.AuditLogListRequest{Cursor: "kaputt"})
	require.ErrorIs(t, err, active.ErrAuditLogInvalid)
}
