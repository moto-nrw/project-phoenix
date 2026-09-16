package timetracking

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #3256: the Urlaubskontingent may not go negative. These tests pin where the
// Resturlaub is checked and what counts against it.

func quotaOf(entitled float64) func(context.Context, int64, int) (*StaffVacationQuota, error) {
	return func(context.Context, int64, int) (*StaffVacationQuota, error) {
		return &StaffVacationQuota{EntitledDays: entitled}, nil
	}
}

// yearRowsOnly returns rows only for the whole-year read of the quota
// summary, so the overlap check of the booked range stays empty.
func yearRowsOnly(rows ...*StaffAbsence) func(context.Context, int64, Date, Date) ([]*StaffAbsence, error) {
	return func(_ context.Context, _ int64, from, to Date) ([]*StaffAbsence, error) {
		if from.Month() == time.January && from.Day() == 1 && to.Month() == time.December && to.Day() == 31 {
			return rows, nil
		}
		return nil, nil
	}
}

func vacationRow(id int64, status string, start, end Date, days float64) *StaffAbsence {
	return &StaffAbsence{
		Model:       Model{ID: id},
		StaffID:     100,
		AbsenceType: AbsenceTypeVacation,
		DateStart:   start,
		DateEnd:     end,
		Status:      status,
		WorkingDays: &days,
	}
}

func TestAbsRequestVacation_RejectsBeyondRemainingQuota(t *testing.T) {
	t.Parallel()

	svc, absRepo, _ := absSetupService()
	svc.quotaRepo = &absVacationQuotaRepoMock{getByStaffAndYearFunc: quotaOf(3)}
	// One open request already holds 2 of the 3 days.
	absRepo.getByStaffAndDateRangeFunc = yearRowsOnly(
		vacationRow(7, AbsenceStatusRequested, NewDate(2027, 3, 1), NewDate(2027, 3, 2), 2),
	)
	absRepo.createFunc = func(context.Context, *StaffAbsence) error {
		t.Fatal("a request beyond the Resturlaub must not be stored")
		return nil
	}

	result, err := svc.RequestVacation(context.Background(), 100, RequestVacationRequest{
		DateStart: "2027-02-15",
		DateEnd:   "2027-02-16",
	})

	require.ErrorIs(t, err, ErrVacationQuotaExceeded)
	assert.Nil(t, result)
}

func TestAbsRequestVacation_AcceptsExactlyRemainingQuota(t *testing.T) {
	t.Parallel()

	svc, absRepo, _ := absSetupService()
	svc.quotaRepo = &absVacationQuotaRepoMock{getByStaffAndYearFunc: quotaOf(3)}
	absRepo.getByStaffAndDateRangeFunc = yearRowsOnly(
		vacationRow(7, AbsenceStatusRequested, NewDate(2027, 3, 1), NewDate(2027, 3, 1), 1),
		// Declined and canceled requests hold nothing.
		vacationRow(8, AbsenceStatusDeclined, NewDate(2027, 4, 1), NewDate(2027, 4, 9), 7),
	)
	created := false
	absRepo.createFunc = func(context.Context, *StaffAbsence) error {
		created = true
		return nil
	}

	_, err := svc.RequestVacation(context.Background(), 100, RequestVacationRequest{
		DateStart: "2027-02-15",
		DateEnd:   "2027-02-16",
	})

	require.NoError(t, err)
	assert.True(t, created)
}

func TestAbsApproveAbsence_RejectsBeyondRemainingQuota(t *testing.T) {
	t.Parallel()

	svc, absRepo, _ := absSetupService()
	svc.quotaRepo = &absVacationQuotaRepoMock{getByStaffAndYearFunc: quotaOf(2)}
	pending := vacationRow(5, AbsenceStatusRequested, NewDate(2027, 2, 15), NewDate(2027, 2, 16), 2)
	absRepo.findByIDFunc = func(context.Context, any) (*StaffAbsence, error) {
		copyOf := *pending
		return &copyOf, nil
	}
	absRepo.getByStaffAndDateRangeFunc = yearRowsOnly(
		pending,
		vacationRow(6, AbsenceStatusApproved, NewDate(2027, 1, 11), NewDate(2027, 1, 11), 1),
	)
	absRepo.updateFunc = func(context.Context, *StaffAbsence) error {
		t.Fatal("an approval beyond the Resturlaub must not be stored")
		return nil
	}

	_, err := svc.ApproveAbsence(context.Background(), pending.ID, 1, 2, "")

	require.ErrorIs(t, err, ErrVacationQuotaExceeded)
}

func TestAbsApproveAbsence_IgnoresOtherOpenRequests(t *testing.T) {
	t.Parallel()

	svc, absRepo, _ := absSetupService()
	svc.quotaRepo = &absVacationQuotaRepoMock{getByStaffAndYearFunc: quotaOf(2)}
	pending := vacationRow(5, AbsenceStatusRequested, NewDate(2027, 2, 15), NewDate(2027, 2, 16), 2)
	absRepo.findByIDFunc = func(context.Context, any) (*StaffAbsence, error) {
		copyOf := *pending
		return &copyOf, nil
	}
	// A second open request of 2 days would not fit next to this one, but it
	// is decided on its own; approving the first must still work.
	absRepo.getByStaffAndDateRangeFunc = yearRowsOnly(
		pending,
		vacationRow(6, AbsenceStatusRequested, NewDate(2027, 5, 3), NewDate(2027, 5, 4), 2),
	)

	result, err := svc.ApproveAbsence(context.Background(), pending.ID, 1, 2, "")

	require.NoError(t, err)
	assert.Equal(t, AbsenceStatusApproved, result.Status)
}

func TestAbsCreateAbsenceFor_BooksVacationDirectly(t *testing.T) {
	t.Parallel()

	svc, absRepo, _ := absSetupService()
	svc.quotaRepo = &absVacationQuotaRepoMock{getByStaffAndYearFunc: quotaOf(5)}
	absRepo.getByStaffAndDateRangeFunc = yearRowsOnly(
		vacationRow(7, AbsenceStatusApproved, NewDate(2027, 1, 4), NewDate(2027, 1, 5), 2),
	)
	var stored *StaffAbsence
	absRepo.createFunc = func(_ context.Context, entity *StaffAbsence) error {
		stored = entity
		entity.ID = 42
		return nil
	}

	// Mon 15.02. to Wed 17.02.: exactly the 3 remaining days.
	result, err := svc.CreateAbsenceFor(context.Background(), 100, 200, nil, CreateAbsenceRequest{
		AbsenceType: AbsenceTypeVacation,
		DateStart:   "2027-02-15",
		DateEnd:     "2027-02-17",
		Note:        "mündlich abgesprochen",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, stored)
	assert.Equal(t, AbsenceStatusReported, stored.Status, "no request is created")
	assert.Equal(t, AbsenceTypeVacation, stored.AbsenceType)
	assert.Equal(t, int64(200), stored.CreatedBy)
	require.NotNil(t, stored.WorkingDays)
	assert.Equal(t, 3.0, *stored.WorkingDays)
}

func TestAbsCreateAbsenceFor_RejectsVacationBeyondQuota(t *testing.T) {
	t.Parallel()

	svc, absRepo, _ := absSetupService()
	svc.quotaRepo = &absVacationQuotaRepoMock{getByStaffAndYearFunc: quotaOf(5)}
	absRepo.getByStaffAndDateRangeFunc = yearRowsOnly(
		vacationRow(7, AbsenceStatusApproved, NewDate(2027, 1, 4), NewDate(2027, 1, 5), 2),
		// Open requests hold their days against a direct booking too.
		vacationRow(8, AbsenceStatusQuestion, NewDate(2027, 6, 1), NewDate(2027, 6, 1), 1),
	)
	absRepo.createFunc = func(context.Context, *StaffAbsence) error {
		t.Fatal("a booking beyond the Resturlaub must not be stored")
		return nil
	}

	_, err := svc.CreateAbsenceFor(context.Background(), 100, 200, nil, CreateAbsenceRequest{
		AbsenceType: AbsenceTypeVacation,
		DateStart:   "2027-02-15",
		DateEnd:     "2027-02-17",
	})

	require.ErrorIs(t, err, ErrVacationQuotaExceeded)
}

func TestAbsCreateAbsenceFor_DirectVacationChecksEachYear(t *testing.T) {
	t.Parallel()

	svc, absRepo, _ := absSetupService()
	// Enough days in 2026, none left in 2027.
	svc.quotaRepo = &absVacationQuotaRepoMock{getByStaffAndYearFunc: func(_ context.Context, _ int64, year int) (*StaffVacationQuota, error) {
		if year == 2027 {
			return &StaffVacationQuota{EntitledDays: 0}, nil
		}
		return &StaffVacationQuota{EntitledDays: 10}, nil
	}}
	absRepo.getByStaffAndDateRangeFunc = yearRowsOnly()
	absRepo.createFunc = func(context.Context, *StaffAbsence) error {
		t.Fatal("the 2027 share does not fit")
		return nil
	}

	_, err := svc.CreateAbsenceFor(context.Background(), 100, 200, nil, CreateAbsenceRequest{
		AbsenceType: AbsenceTypeVacation,
		DateStart:   "2026-12-31",
		DateEnd:     "2027-01-04",
	})

	require.ErrorIs(t, err, ErrVacationQuotaExceeded)
	assert.Contains(t, err.Error(), "2027")
}

func TestAbsCreateAbsenceFor_DirectVacationHalfDay(t *testing.T) {
	t.Parallel()

	svc, absRepo, _ := absSetupService()
	svc.quotaRepo = &absVacationQuotaRepoMock{getByStaffAndYearFunc: quotaOf(0.5)}
	absRepo.getByStaffAndDateRangeFunc = yearRowsOnly()
	var stored *StaffAbsence
	absRepo.createFunc = func(_ context.Context, entity *StaffAbsence) error {
		stored = entity
		return nil
	}

	_, err := svc.CreateAbsenceFor(context.Background(), 100, 200, nil, CreateAbsenceRequest{
		AbsenceType: AbsenceTypeVacation,
		DateStart:   "2027-02-15",
		DateEnd:     "2027-02-15",
		HalfDay:     true,
	})
	require.NoError(t, err)
	require.NotNil(t, stored)
	require.NotNil(t, stored.WorkingDays)
	assert.Equal(t, 0.5, *stored.WorkingDays)

	_, err = svc.CreateAbsenceFor(context.Background(), 100, 200, nil, CreateAbsenceRequest{
		AbsenceType: AbsenceTypeVacation,
		DateStart:   "2027-02-15",
		DateEnd:     "2027-02-16",
		HalfDay:     true,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exactly one date")
}

func TestAbsCreateAbsenceFor_DirectVacationRejectsOverlap(t *testing.T) {
	t.Parallel()

	svc, absRepo, _ := absSetupService()
	absRepo.getByStaffAndDateRangeFunc = func(context.Context, int64, Date, Date) ([]*StaffAbsence, error) {
		return []*StaffAbsence{{
			Model: Model{ID: 9}, StaffID: 100, AbsenceType: AbsenceTypeSick,
			DateStart: NewDate(2027, 2, 15), DateEnd: NewDate(2027, 2, 15), Status: AbsenceStatusReported,
		}}, nil
	}
	absRepo.createFunc = func(context.Context, *StaffAbsence) error {
		t.Fatal("overlapping vacation must not be stored")
		return nil
	}

	_, err := svc.CreateAbsenceFor(context.Background(), 100, 200, nil, CreateAbsenceRequest{
		AbsenceType: AbsenceTypeVacation,
		DateStart:   "2027-02-15",
		DateEnd:     "2027-02-15",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "dates overlap")
}

func TestAbsDirectVacation_IsManagerControlled(t *testing.T) {
	t.Parallel()

	svc, absRepo, _ := absSetupService()
	direct := vacationRow(11, AbsenceStatusReported, NewDate(2027, 2, 15), NewDate(2027, 2, 15), 1)
	absRepo.findByIDFunc = func(context.Context, any) (*StaffAbsence, error) {
		copyOf := *direct
		return &copyOf, nil
	}
	deleted := false
	absRepo.deleteFunc = func(context.Context, any) error {
		deleted = true
		return nil
	}

	err := svc.DeleteOwnAbsence(context.Background(), direct.StaffID, nil, direct.ID)
	require.ErrorIs(t, err, ErrManagerControlledAbsence)
	assert.False(t, deleted)

	note := "anders"
	_, err = svc.UpdateAbsence(context.Background(), direct.StaffID, nil, direct.ID, UpdateAbsenceRequest{Note: &note})
	require.ErrorIs(t, err, ErrManagerControlledAbsence)

	require.NoError(t, svc.DeleteAbsenceFor(context.Background(), direct.StaffID, 200, nil, direct.ID))
	assert.True(t, deleted, "the Leitung can remove its own entry")
}
