package users_test

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Personnel number write path (#1417 Tranche 2b): unique per tenant, every
// change audited in the same transaction, duplicates mapped to a stable
// sentinel. Test numbers are recognizably artificial — never copy them into
// real configuration.

type payrollScenario struct {
	db    *bun.DB
	repos *repositories.Factory
	svc   usersSvc.PersonService
	ctx   context.Context
}

type serializingStaffRepo struct {
	userModels.StaffRepository
	calls           atomic.Int32
	firstLocked     chan struct{}
	secondAttempted chan struct{}
	secondRead      chan struct{}
	releaseFirst    chan struct{}
}

func (r *serializingStaffRepo) FindByIDForUpdate(ctx context.Context, id int64) (*userModels.Staff, error) {
	call := r.calls.Add(1)
	if call == 2 {
		close(r.secondAttempted)
	}

	staff, err := r.StaffRepository.FindByIDForUpdate(ctx, id)
	if err != nil {
		return nil, err
	}
	switch call {
	case 1:
		close(r.firstLocked)
		<-r.releaseFirst
	case 2:
		close(r.secondRead)
	}
	return staff, nil
}

func newPayrollScenario(t *testing.T) *payrollScenario {
	t.Helper()

	db := testpkg.SetupTestDB(t)

	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	svc := usersSvc.NewPersonService(usersSvc.PersonServiceDependencies{
		PersonRepo:           repos.Person,
		RFIDRepo:             repos.RFIDCard,
		AccountRepo:          repos.Account,
		StudentRepo:          repos.Student,
		StaffRepo:            repos.Staff,
		TeacherRepo:          repos.Teacher,
		PersonnelNumberAudit: repos.PersonnelNumberChange,
		DB:                   db,
	})

	return &payrollScenario{db: db, repos: repos, svc: svc, ctx: testpkg.Ctx(t)}
}

func (s *payrollScenario) auditRows(t *testing.T, staffID int64) []*auditModels.PersonnelNumberChange {
	t.Helper()
	var rows []*auditModels.PersonnelNumberChange
	err := s.db.NewSelect().
		Model(&rows).
		ModelTableExpr(`audit.personnel_number_changes AS "personnel_number_change"`).
		Where(`"personnel_number_change".staff_id = ?`, staffID).
		Order("id ASC").
		Scan(context.Background())
	require.NoError(t, err)
	return rows
}

func TestUpdatePersonnelNumber_SetChangeClear(t *testing.T) {
	t.Parallel()

	s := newPayrollScenario(t)
	staff := testpkg.CreateTestStaff(t, s.db, "Payroll", "Nummer")
	actor := testpkg.CreateTestStaff(t, s.db, "Payroll", "Aktor")

	// Set
	value := "90001"
	updated, err := s.svc.UpdatePersonnelNumber(s.ctx, staff.ID, &value, actor.ID, "Ersteinrichtung")
	require.NoError(t, err)
	require.NotNil(t, updated.PersonnelNumber)
	assert.Equal(t, "90001", *updated.PersonnelNumber)

	rows := s.auditRows(t, staff.ID)
	require.Len(t, rows, 1)
	assert.Equal(t, "", rows[0].OldValue)
	assert.Equal(t, "90001", rows[0].NewValue)
	assert.Equal(t, actor.ID, rows[0].ChangedBy)
	assert.Equal(t, "Ersteinrichtung", rows[0].Note)

	// No-op: same value writes no second audit row
	_, err = s.svc.UpdatePersonnelNumber(s.ctx, staff.ID, &value, actor.ID, "")
	require.NoError(t, err)
	require.Len(t, s.auditRows(t, staff.ID), 1, "a no-op change must not produce an audit row")

	// Change
	changed := " 90002 " // whitespace is normalized away
	updated, err = s.svc.UpdatePersonnelNumber(s.ctx, staff.ID, &changed, actor.ID, "Korrektur")
	require.NoError(t, err)
	assert.Equal(t, "90002", *updated.PersonnelNumber)
	rows = s.auditRows(t, staff.ID)
	require.Len(t, rows, 2)
	assert.Equal(t, "90001", rows[1].OldValue)
	assert.Equal(t, "90002", rows[1].NewValue)

	// Clear
	updated, err = s.svc.UpdatePersonnelNumber(s.ctx, staff.ID, nil, actor.ID, "Austritt")
	require.NoError(t, err)
	assert.Nil(t, updated.PersonnelNumber)
	rows = s.auditRows(t, staff.ID)
	require.Len(t, rows, 3)
	assert.Equal(t, "90002", rows[2].OldValue)
	assert.Equal(t, "", rows[2].NewValue)
}

func TestUpdatePersonnelNumber_DuplicateIsConflictAndLeavesNoAudit(t *testing.T) {
	t.Parallel()

	s := newPayrollScenario(t)
	first := testpkg.CreateTestStaff(t, s.db, "Payroll", "Erste")
	second := testpkg.CreateTestStaff(t, s.db, "Payroll", "Zweite")
	actor := testpkg.CreateTestStaff(t, s.db, "Payroll", "Aktor")

	value := "90010"
	_, err := s.svc.UpdatePersonnelNumber(s.ctx, first.ID, &value, actor.ID, "")
	require.NoError(t, err)

	_, err = s.svc.UpdatePersonnelNumber(s.ctx, second.ID, &value, actor.ID, "")
	require.ErrorIs(t, err, usersSvc.ErrPersonnelNumberTaken)

	assert.Empty(t, s.auditRows(t, second.ID), "the rolled-back change must not leave an audit row")

	// The number can be reassigned after the holder is offboarded (partial
	// index skips soft-deleted rows).
	_, err = s.db.NewUpdate().
		Model((*userModels.Staff)(nil)).
		ModelTableExpr(`users.staff AS "staff"`).
		Set("deleted_at = NOW()").
		Where(`"staff".id = ?`, first.ID).
		Exec(context.Background())
	require.NoError(t, err)

	_, err = s.svc.UpdatePersonnelNumber(s.ctx, second.ID, &value, actor.ID, "")
	require.NoError(t, err, "an offboarded holder must free the number")
}

func TestUpdatePersonnelNumber_SerializesConcurrentAuditValues(t *testing.T) {
	t.Parallel()

	s := newPayrollScenario(t)
	staff := testpkg.CreateTestStaff(t, s.db, "Payroll", "Parallel")
	actor := testpkg.CreateTestStaff(t, s.db, "Payroll", "Aktor")

	staffRepo := &serializingStaffRepo{
		StaffRepository: s.repos.Staff,
		firstLocked:     make(chan struct{}),
		secondAttempted: make(chan struct{}),
		secondRead:      make(chan struct{}),
		releaseFirst:    make(chan struct{}),
	}
	svc := usersSvc.NewPersonService(usersSvc.PersonServiceDependencies{
		PersonRepo:           s.repos.Person,
		RFIDRepo:             s.repos.RFIDCard,
		AccountRepo:          s.repos.Account,
		StudentRepo:          s.repos.Student,
		StaffRepo:            staffRepo,
		TeacherRepo:          s.repos.Teacher,
		PersonnelNumberAudit: s.repos.PersonnelNumberChange,
		DB:                   s.db,
	})

	personnelNumberSeed := staff.ID % 10_000_000
	firstValue := fmt.Sprintf("7%07d1", personnelNumberSeed)
	firstDone := make(chan error, 1)
	go func() {
		_, err := svc.UpdatePersonnelNumber(s.ctx, staff.ID, &firstValue, actor.ID, "Erste Änderung")
		firstDone <- err
	}()

	select {
	case <-staffRepo.firstLocked:
	case err := <-firstDone:
		require.NoError(t, err)
		t.Fatal("first update returned before holding the staff row lock")
	case <-time.After(2 * time.Second):
		t.Fatal("first update did not acquire the staff row lock")
	}

	secondValue := fmt.Sprintf("7%07d2", personnelNumberSeed)
	secondDone := make(chan error, 1)
	go func() {
		_, err := svc.UpdatePersonnelNumber(s.ctx, staff.ID, &secondValue, actor.ID, "Zweite Änderung")
		secondDone <- err
	}()

	select {
	case <-staffRepo.secondAttempted:
	case <-time.After(2 * time.Second):
		close(staffRepo.releaseFirst)
		t.Fatal("second update did not attempt to lock the staff row")
	}

	released := false
	defer func() {
		if !released {
			close(staffRepo.releaseFirst)
		}
	}()
	select {
	case <-staffRepo.secondRead:
		close(staffRepo.releaseFirst)
		released = true
		t.Fatal("second update read the staff row while the first transaction held its lock")
	case <-time.After(150 * time.Millisecond):
		close(staffRepo.releaseFirst)
		released = true
	}

	select {
	case err := <-firstDone:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("first update did not finish")
	}
	select {
	case err := <-secondDone:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("second update did not finish")
	}

	rows := s.auditRows(t, staff.ID)
	require.Len(t, rows, 2)
	assert.Equal(t, "", rows[0].OldValue)
	assert.Equal(t, firstValue, rows[0].NewValue)
	assert.Equal(t, firstValue, rows[1].OldValue)
	assert.Equal(t, secondValue, rows[1].NewValue)
}

func TestUpdateStaffWithTeacher_PreservesConcurrentPersonnelNumber(t *testing.T) {
	t.Parallel()

	s := newPayrollScenario(t)
	staff := testpkg.CreateTestStaff(t, s.db, "Payroll", "Stale")
	actor := testpkg.CreateTestStaff(t, s.db, "Payroll", "Aktor")

	staleStaff, err := s.repos.Staff.FindByID(s.ctx, staff.ID)
	require.NoError(t, err)

	value := fmt.Sprintf("6%07d1", staff.ID%10_000_000)
	_, err = s.svc.UpdatePersonnelNumber(s.ctx, staff.ID, &value, actor.ID, "Ersteinrichtung")
	require.NoError(t, err)

	staleStaff.StaffNotes = "Aktualisierte Notiz"
	_, _, err = s.svc.UpdateStaffWithTeacher(s.ctx, staleStaff, false, "", "", "")
	require.NoError(t, err)

	reloaded, err := s.repos.Staff.FindByID(s.ctx, staff.ID)
	require.NoError(t, err)
	require.NotNil(t, reloaded.PersonnelNumber)
	assert.Equal(t, value, *reloaded.PersonnelNumber)
	assert.Equal(t, "Aktualisierte Notiz", reloaded.StaffNotes)

	rows := s.auditRows(t, staff.ID)
	require.Len(t, rows, 1)
	assert.Equal(t, "", rows[0].OldValue)
	assert.Equal(t, value, rows[0].NewValue)
}

func TestUpdatePersonnelNumber_Validation(t *testing.T) {
	t.Parallel()

	s := newPayrollScenario(t)
	staff := testpkg.CreateTestStaff(t, s.db, "Payroll", "Valid")
	actor := testpkg.CreateTestStaff(t, s.db, "Payroll", "Aktor")

	for _, invalid := range []string{"12a", "1 2", "-5", "1234567890"} {
		v := invalid
		_, err := s.svc.UpdatePersonnelNumber(s.ctx, staff.ID, &v, actor.ID, "")
		require.ErrorIs(t, err, usersSvc.ErrPersonnelNumberInvalid, "value %q must be rejected", invalid)
	}

	// Missing audit wiring refuses the change entirely (fail fast).
	unwired := usersSvc.NewPersonService(usersSvc.PersonServiceDependencies{
		StaffRepo: s.repos.Staff,
		DB:        s.db,
	})
	v := "90020"
	_, err := unwired.UpdatePersonnelNumber(s.ctx, staff.ID, &v, actor.ID, "")
	require.Error(t, err, "without the audit repository no change may happen")
}

func TestUpdatePersonnelNumber_AppearsInAuditFeed(t *testing.T) {
	t.Parallel()

	s := newPayrollScenario(t)
	staff := testpkg.CreateTestStaff(t, s.db, "Payroll", "Feed")
	actor := testpkg.CreateTestStaff(t, s.db, "Payroll", "FeedAktor")

	value := "90030"
	_, err := s.svc.UpdatePersonnelNumber(s.ctx, staff.ID, &value, actor.ID, "Feed-Test")
	require.NoError(t, err)

	entries, err := s.repos.TimeTrackingAuditLog.ListEntries(s.ctx, auditModels.TimeTrackingAuditLogFilter{
		StaffID: staff.ID,
		Sources: []string{auditModels.AuditLogSourcePersonnelNumber},
	})
	require.NoError(t, err)
	require.Len(t, entries, 1)

	entry := entries[0]
	assert.Equal(t, auditModels.AuditLogSourcePersonnelNumber, entry.Source)
	require.NotNil(t, entry.StaffID)
	assert.Equal(t, staff.ID, *entry.StaffID)
	require.NotNil(t, entry.ActorStaffID)
	assert.Equal(t, actor.ID, *entry.ActorStaffID)
	assert.Equal(t, "Feed-Test", entry.Reason)

	var detail struct {
		OldValue string `json:"old_value"`
		NewValue string `json:"new_value"`
	}
	require.NoError(t, json.Unmarshal(entry.Detail, &detail))
	assert.Equal(t, "", detail.OldValue)
	assert.Equal(t, "90030", detail.NewValue)
}
