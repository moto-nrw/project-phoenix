package active

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Vacation takeover (#2132) guard branches. The happy paths run against the
// real database in staff_vacation_opening_db_test.go; what is exercised here
// are the rejections and the repository-failure paths, which need injected
// errors rather than a DB.

// ============================================================================
// Mocks (prefixed vo)
// ============================================================================

type voOpeningRepoMock struct {
	getByStaffAndYearFunc func(ctx context.Context, staffID int64, year int) (*activeModels.StaffVacationOpening, error)
	createFunc            func(ctx context.Context, opening *activeModels.StaffVacationOpening) error
	deleteFunc            func(ctx context.Context, id any) error
}

func (m *voOpeningRepoMock) Create(ctx context.Context, opening *activeModels.StaffVacationOpening) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, opening)
	}
	return nil
}

func (m *voOpeningRepoMock) FindByID(context.Context, any) (*activeModels.StaffVacationOpening, error) {
	return nil, errors.New("not found")
}

func (m *voOpeningRepoMock) Update(context.Context, *activeModels.StaffVacationOpening) error {
	return nil
}

func (m *voOpeningRepoMock) Delete(ctx context.Context, id any) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, id)
	}
	return nil
}

func (m *voOpeningRepoMock) List(context.Context, *base.QueryOptions) ([]*activeModels.StaffVacationOpening, error) {
	return nil, nil
}

func (m *voOpeningRepoMock) GetByStaffAndYear(ctx context.Context, staffID int64, year int) (*activeModels.StaffVacationOpening, error) {
	if m.getByStaffAndYearFunc != nil {
		return m.getByStaffAndYearFunc(ctx, staffID, year)
	}
	return nil, nil
}

func (m *voOpeningRepoMock) GetByStaffIDsAndYear(context.Context, []int64, int) (map[int64]*activeModels.StaffVacationOpening, error) {
	return nil, nil
}

type voDeletionRepoMock struct {
	createFunc func(ctx context.Context, event *auditModels.TimeTrackingDeletion) error
	created    []*auditModels.TimeTrackingDeletion
}

func (m *voDeletionRepoMock) Create(ctx context.Context, event *auditModels.TimeTrackingDeletion) error {
	m.created = append(m.created, event)
	if m.createFunc != nil {
		return m.createFunc(ctx, event)
	}
	return nil
}

// voService builds a bare service with the takeover repositories wired.
func voService(openingRepo *voOpeningRepoMock) (*staffAbsenceService, *absStaffAbsenceRepoMock, *voDeletionRepoMock) {
	absenceRepo := &absStaffAbsenceRepoMock{}
	deletionRepo := &voDeletionRepoMock{}
	svc := &staffAbsenceService{
		absenceRepo:  absenceRepo,
		quotaRepo:    &absVacationQuotaRepoMock{},
		openingRepo:  openingRepo,
		deletionRepo: deletionRepo,
	}
	return svc, absenceRepo, deletionRepo
}

func voRequest() SetVacationOpeningRequest {
	return SetVacationOpeningRequest{
		EffectiveDate: timezone.TodayDate().AddDays(-1),
		RemainingDays: 17.5,
		Note:          "Übernahme aus Altsystem",
	}
}

// ============================================================================
// Request validation
// ============================================================================

func TestValidateSetVacationOpening(t *testing.T) {
	t.Parallel()

	const (
		staffID   = int64(41)
		decidedBy = int64(42)
	)

	tests := []struct {
		name    string
		staffID int64
		decided int64
		mutate  func(*SetVacationOpeningRequest)
		wantMsg string
	}{
		{name: "valid"},
		{name: "missing staff", staffID: -1, wantMsg: "staff id is required"},
		{name: "missing actor", decided: -1, wantMsg: "decided_by is required"},
		{
			name:    "missing Stichtag",
			mutate:  func(r *SetVacationOpeningRequest) { r.EffectiveDate = timezone.Date{} },
			wantMsg: "effective_date is required",
		},
		{
			name:    "missing Begründung",
			mutate:  func(r *SetVacationOpeningRequest) { r.Note = "   " },
			wantMsg: "note is required",
		},
		{
			name:    "Stichtag today",
			mutate:  func(r *SetVacationOpeningRequest) { r.EffectiveDate = timezone.TodayDate() },
			wantMsg: "must be before today",
		},
		{
			name:    "Stichtag in the future",
			mutate:  func(r *SetVacationOpeningRequest) { r.EffectiveDate = timezone.TodayDate().AddDays(1) },
			wantMsg: "must be before today",
		},
		{
			// A closed vacation year has no live account left to rebaseline —
			// the same bound the bulk import applies (#2132 review).
			name: "Stichtag in a closed vacation year",
			mutate: func(r *SetVacationOpeningRequest) {
				r.EffectiveDate = timezone.NewDate(timezone.TodayDate().Year-1, time.June, 1)
			},
			wantMsg: "must be in the current vacation year",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := voRequest()
			if tc.mutate != nil {
				tc.mutate(&req)
			}
			actualStaff, actualDecided := staffID, decidedBy
			if tc.staffID < 0 {
				actualStaff = 0
			}
			if tc.decided < 0 {
				actualDecided = 0
			}

			err := validateSetVacationOpening(actualStaff, actualDecided, req)

			if tc.wantMsg == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrVacationOpeningInvalid)
			assert.Contains(t, err.Error(), tc.wantMsg)
		})
	}
}

// TestSetVacationOpening_CurrentYearGuardAlsoAppliesPerStaff pins the guard on
// the route the per-person modal uses, not just on the pure validator.
func TestSetVacationOpening_CurrentYearGuardAlsoAppliesPerStaff(t *testing.T) {
	t.Parallel()

	svc, _, _ := voService(&voOpeningRepoMock{})
	req := voRequest()
	req.EffectiveDate = timezone.NewDate(timezone.TodayDate().Year-1, time.June, 1)

	_, err := svc.SetVacationOpening(context.Background(), 41, 42, req)

	require.ErrorIs(t, err, ErrVacationOpeningInvalid)
	assert.Contains(t, err.Error(), "current vacation year")
}

// ============================================================================
// Booking guards
// ============================================================================

func TestSetVacationOpening_WithoutRepositoryFailsLoudly(t *testing.T) {
	t.Parallel()

	svc := &staffAbsenceService{absenceRepo: &absStaffAbsenceRepoMock{}}

	_, err := svc.SetVacationOpening(context.Background(), 41, 42, voRequest())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "repository is not configured")
}

func TestSetVacationOpening_RejectsSecondTakeoverForTheSameYear(t *testing.T) {
	t.Parallel()

	existing := &activeModels.StaffVacationOpening{EffectiveDate: timezone.TodayDate().AddDays(-30)}
	svc, _, _ := voService(&voOpeningRepoMock{
		getByStaffAndYearFunc: func(context.Context, int64, int) (*activeModels.StaffVacationOpening, error) {
			return existing, nil
		},
	})

	_, err := svc.SetVacationOpening(context.Background(), 41, 42, voRequest())

	require.ErrorIs(t, err, ErrVacationOpeningExists)
	assert.Contains(t, err.Error(), existing.EffectiveDate.String())
}

func TestSetVacationOpening_PropagatesRepositoryFailures(t *testing.T) {
	t.Parallel()

	boom := errors.New("db down")

	t.Run("lock", func(t *testing.T) {
		svc, absenceRepo, _ := voService(&voOpeningRepoMock{})
		absenceRepo.lockStaffAbsenceWritesFunc = func(context.Context, int64) error { return boom }

		_, err := svc.SetVacationOpening(context.Background(), 41, 42, voRequest())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to lock staff absence writes")
	})

	t.Run("existing lookup", func(t *testing.T) {
		svc, _, _ := voService(&voOpeningRepoMock{
			getByStaffAndYearFunc: func(context.Context, int64, int) (*activeModels.StaffVacationOpening, error) {
				return nil, boom
			},
		})

		_, err := svc.SetVacationOpening(context.Background(), 41, 42, voRequest())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to check existing vacation opening")
	})

	t.Run("insert", func(t *testing.T) {
		svc, _, _ := voService(&voOpeningRepoMock{
			createFunc: func(context.Context, *activeModels.StaffVacationOpening) error { return boom },
		})

		_, err := svc.SetVacationOpening(context.Background(), 41, 42, voRequest())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create vacation opening")
	})
}

// TestSetVacationOpening_DerivesTakenBeforeFromTheQuota is the arithmetic the
// whole takeover rests on: the Jahresanspruch stays authoritative and the days
// taken before the Stichtag are derived from it.
func TestSetVacationOpening_DerivesTakenBeforeFromTheQuota(t *testing.T) {
	t.Parallel()

	var created *activeModels.StaffVacationOpening
	svc, _, _ := voService(&voOpeningRepoMock{
		createFunc: func(_ context.Context, opening *activeModels.StaffVacationOpening) error {
			created = opening
			return nil
		},
	})
	svc.quotaRepo = &absVacationQuotaRepoMock{
		getByStaffAndYearFunc: func(context.Context, int64, int) (*activeModels.StaffVacationQuota, error) {
			return &activeModels.StaffVacationQuota{EntitledDays: 28, CarryoverDays: 4}, nil
		},
	}

	opening, err := svc.SetVacationOpening(context.Background(), 41, 42, voRequest())

	require.NoError(t, err)
	require.NotNil(t, created)
	// 28 + 4 − 17,5 entered rest = 14,5 days spent before the Stichtag.
	assert.InDelta(t, 14.5, opening.TakenBeforeDays, 0.0001)
	assert.InDelta(t, 17.5, opening.EnteredRemainingDays, 0.0001)
}

// TestSetVacationOpening_RejectsADerivedValueTheModelWouldRefuse guards the
// path where the entered rest is arithmetically impossible for the quota.
func TestSetVacationOpening_RejectsADerivedValueTheModelWouldRefuse(t *testing.T) {
	t.Parallel()

	svc, _, _ := voService(&voOpeningRepoMock{})
	req := voRequest()
	req.RemainingDays = -990

	_, err := svc.SetVacationOpening(context.Background(), 41, 42, req)

	require.ErrorIs(t, err, ErrVacationOpeningInvalid)
}

// ============================================================================
// Deletion
// ============================================================================

func TestDeleteVacationOpening_WritesTombstoneBeforeDeleting(t *testing.T) {
	t.Parallel()

	opening := &activeModels.StaffVacationOpening{
		StaffID:         41,
		Year:            timezone.TodayDate().Year,
		EffectiveDate:   timezone.TodayDate().AddDays(-10),
		TakenBeforeDays: 5,
		Note:            "Übernahme aus Altsystem",
	}
	opening.ID = 77
	deleted := []any{}
	svc, _, deletionRepo := voService(&voOpeningRepoMock{
		getByStaffAndYearFunc: func(context.Context, int64, int) (*activeModels.StaffVacationOpening, error) {
			return opening, nil
		},
		deleteFunc: func(_ context.Context, id any) error {
			deleted = append(deleted, id)
			return nil
		},
	})

	require.NoError(t, svc.DeleteVacationOpening(context.Background(), 41, 42, opening.Year))

	require.Len(t, deletionRepo.created, 1)
	tombstone := deletionRepo.created[0]
	assert.Equal(t, auditModels.TimeTrackingDeletionSourceVacationOpening, tombstone.Source)
	assert.Equal(t, opening.ID, tombstone.SourceID)
	assert.Equal(t, int64(42), tombstone.DeletedBy)
	assert.Equal(t, opening.Note, tombstone.Note)
	assert.NotEmpty(t, tombstone.Payload, "the tombstone keeps the deleted row's snapshot")
	assert.Equal(t, []any{opening.ID}, deleted)
}

func TestDeleteVacationOpening_Rejects(t *testing.T) {
	t.Parallel()

	boom := errors.New("db down")
	year := timezone.TodayDate().Year

	t.Run("without repository", func(t *testing.T) {
		svc := &staffAbsenceService{absenceRepo: &absStaffAbsenceRepoMock{}}

		err := svc.DeleteVacationOpening(context.Background(), 41, 42, year)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "repository is not configured")
	})

	t.Run("without actor", func(t *testing.T) {
		svc, _, _ := voService(&voOpeningRepoMock{})

		err := svc.DeleteVacationOpening(context.Background(), 41, 0, year)

		require.ErrorIs(t, err, ErrVacationOpeningInvalid)
	})

	t.Run("missing row", func(t *testing.T) {
		svc, _, _ := voService(&voOpeningRepoMock{})

		err := svc.DeleteVacationOpening(context.Background(), 41, 42, year)

		require.ErrorIs(t, err, ErrVacationOpeningNotFound)
	})

	t.Run("lock failure", func(t *testing.T) {
		svc, absenceRepo, _ := voService(&voOpeningRepoMock{})
		absenceRepo.lockStaffAbsenceWritesFunc = func(context.Context, int64) error { return boom }

		err := svc.DeleteVacationOpening(context.Background(), 41, 42, year)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to lock staff absence writes")
	})

	t.Run("load failure", func(t *testing.T) {
		svc, _, _ := voService(&voOpeningRepoMock{
			getByStaffAndYearFunc: func(context.Context, int64, int) (*activeModels.StaffVacationOpening, error) {
				return nil, boom
			},
		})

		err := svc.DeleteVacationOpening(context.Background(), 41, 42, year)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to load vacation opening")
	})

	t.Run("audit write failure keeps the row", func(t *testing.T) {
		deletes := 0
		svc, _, deletionRepo := voService(&voOpeningRepoMock{
			getByStaffAndYearFunc: func(context.Context, int64, int) (*activeModels.StaffVacationOpening, error) {
				return &activeModels.StaffVacationOpening{StaffID: 41, Year: year}, nil
			},
			deleteFunc: func(context.Context, any) error { deletes++; return nil },
		})
		deletionRepo.createFunc = func(context.Context, *auditModels.TimeTrackingDeletion) error { return boom }

		err := svc.DeleteVacationOpening(context.Background(), 41, 42, year)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to write deletion audit")
		assert.Zero(t, deletes, "no tombstone, no delete")
	})

	t.Run("delete failure", func(t *testing.T) {
		svc, _, _ := voService(&voOpeningRepoMock{
			getByStaffAndYearFunc: func(context.Context, int64, int) (*activeModels.StaffVacationOpening, error) {
				return &activeModels.StaffVacationOpening{StaffID: 41, Year: year}, nil
			},
			deleteFunc: func(context.Context, any) error { return boom },
		})

		err := svc.DeleteVacationOpening(context.Background(), 41, 42, year)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to delete vacation opening")
	})

	t.Run("without deletion audit repository", func(t *testing.T) {
		svc, _, _ := voService(&voOpeningRepoMock{
			getByStaffAndYearFunc: func(context.Context, int64, int) (*activeModels.StaffVacationOpening, error) {
				return &activeModels.StaffVacationOpening{StaffID: 41, Year: year}, nil
			},
		})
		svc.deletionRepo = nil

		err := svc.DeleteVacationOpening(context.Background(), 41, 42, year)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "deletion audit repository is not configured")
	})
}

// ============================================================================
// Reads and the reverse guard
// ============================================================================

func TestGetVacationOpening(t *testing.T) {
	t.Parallel()

	t.Run("requires a staff id", func(t *testing.T) {
		svc, _, _ := voService(&voOpeningRepoMock{})

		_, err := svc.GetVacationOpening(context.Background(), 0, 2026)

		require.ErrorIs(t, err, ErrVacationOpeningInvalid)
	})

	t.Run("without repository there is no takeover", func(t *testing.T) {
		svc := &staffAbsenceService{absenceRepo: &absStaffAbsenceRepoMock{}}

		opening, err := svc.GetVacationOpening(context.Background(), 41, 2026)

		require.NoError(t, err)
		assert.Nil(t, opening)
	})
}

// TestRejectVacationBeforeOpening covers the reverse direction: an already
// booked takeover must block a later vacation absence that would land before
// its Stichtag (those days are part of the entered Resturlaub already).
func TestRejectVacationBeforeOpening(t *testing.T) {
	t.Parallel()

	year := timezone.TodayDate().Year
	cutoff := timezone.NewDate(year, time.June, 8) // Monday

	t.Run("ignores non-vacation absences", func(t *testing.T) {
		svc, _, _ := voService(&voOpeningRepoMock{
			getByStaffAndYearFunc: func(context.Context, int64, int) (*activeModels.StaffVacationOpening, error) {
				return &activeModels.StaffVacationOpening{EffectiveDate: cutoff}, nil
			},
		})

		err := svc.rejectVacationBeforeOpening(context.Background(), &activeModels.StaffAbsence{
			AbsenceType: activeModels.AbsenceTypeSick,
			DateStart:   cutoff.AddDays(-3),
			DateEnd:     cutoff.AddDays(-3),
		})

		require.NoError(t, err)
	})

	t.Run("passes without a takeover", func(t *testing.T) {
		svc, _, _ := voService(&voOpeningRepoMock{})

		err := svc.rejectVacationBeforeOpening(context.Background(), &activeModels.StaffAbsence{
			AbsenceType: activeModels.AbsenceTypeVacation,
			DateStart:   cutoff.AddDays(-3),
			DateEnd:     cutoff.AddDays(-3),
		})

		require.NoError(t, err)
	})

	t.Run("blocks a working day before the Stichtag", func(t *testing.T) {
		svc, _, _ := voService(&voOpeningRepoMock{
			getByStaffAndYearFunc: func(context.Context, int64, int) (*activeModels.StaffVacationOpening, error) {
				return &activeModels.StaffVacationOpening{EffectiveDate: cutoff}, nil
			},
		})

		err := svc.rejectVacationBeforeOpening(context.Background(), &activeModels.StaffAbsence{
			AbsenceType: activeModels.AbsenceTypeVacation,
			DateStart:   cutoff.AddDays(-3), // Friday
			DateEnd:     cutoff.AddDays(-3),
		})

		require.ErrorIs(t, err, ErrVacationOpeningAbsencesBeforeCutoff)
	})

	t.Run("allows an absence that starts on the Stichtag", func(t *testing.T) {
		svc, _, _ := voService(&voOpeningRepoMock{
			getByStaffAndYearFunc: func(context.Context, int64, int) (*activeModels.StaffVacationOpening, error) {
				return &activeModels.StaffVacationOpening{EffectiveDate: cutoff}, nil
			},
		})

		err := svc.rejectVacationBeforeOpening(context.Background(), &activeModels.StaffAbsence{
			AbsenceType: activeModels.AbsenceTypeVacation,
			DateStart:   cutoff,
			DateEnd:     cutoff.AddDays(2),
		})

		require.NoError(t, err)
	})

	t.Run("propagates lookup failures", func(t *testing.T) {
		svc, _, _ := voService(&voOpeningRepoMock{
			getByStaffAndYearFunc: func(context.Context, int64, int) (*activeModels.StaffVacationOpening, error) {
				return nil, errors.New("db down")
			},
		})

		err := svc.rejectVacationBeforeOpening(context.Background(), &activeModels.StaffAbsence{
			AbsenceType: activeModels.AbsenceTypeVacation,
			DateStart:   cutoff.AddDays(-3),
			DateEnd:     cutoff.AddDays(-3),
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to check vacation opening")
	})
}

// TestRejectVacationAbsencesBefore covers the forward guard: declined and
// canceled absences do not count, an approved one before the Stichtag does.
func TestRejectVacationAbsencesBefore(t *testing.T) {
	t.Parallel()

	year := timezone.TodayDate().Year
	cutoff := timezone.NewDate(year, time.June, 8) // Monday
	friday := cutoff.AddDays(-3)

	tests := []struct {
		name      string
		absences  []*activeModels.StaffAbsence
		wantBlock bool
	}{
		{
			name: "approved vacation before the Stichtag blocks",
			absences: []*activeModels.StaffAbsence{{
				AbsenceType: activeModels.AbsenceTypeVacation,
				Status:      activeModels.AbsenceStatusApproved,
				DateStart:   friday, DateEnd: friday,
			}},
			wantBlock: true,
		},
		{
			name: "declined vacation does not count",
			absences: []*activeModels.StaffAbsence{{
				AbsenceType: activeModels.AbsenceTypeVacation,
				Status:      activeModels.AbsenceStatusDeclined,
				DateStart:   friday, DateEnd: friday,
			}},
		},
		{
			name: "canceled vacation does not count",
			absences: []*activeModels.StaffAbsence{{
				AbsenceType: activeModels.AbsenceTypeVacation,
				Status:      activeModels.AbsenceStatusCanceled,
				DateStart:   friday, DateEnd: friday,
			}},
		},
		{
			name: "sick leave does not count against the vacation quota",
			absences: []*activeModels.StaffAbsence{{
				AbsenceType: activeModels.AbsenceTypeSick,
				Status:      activeModels.AbsenceStatusApproved,
				DateStart:   friday, DateEnd: friday,
			}},
		},
		{
			name: "vacation on or after the Stichtag is fine",
			absences: []*activeModels.StaffAbsence{{
				AbsenceType: activeModels.AbsenceTypeVacation,
				Status:      activeModels.AbsenceStatusApproved,
				DateStart:   cutoff, DateEnd: cutoff.AddDays(4),
			}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc, absenceRepo, _ := voService(&voOpeningRepoMock{})
			absenceRepo.getByStaffAndDateRangeFunc = func(context.Context, int64, timezone.Date, timezone.Date) ([]*activeModels.StaffAbsence, error) {
				return tc.absences, nil
			}

			err := svc.ValidateVacationOpeningAbsencesBefore(context.Background(), 41, cutoff)

			if tc.wantBlock {
				require.ErrorIs(t, err, ErrVacationOpeningAbsencesBeforeCutoff)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestRejectVacationAbsencesBefore_PropagatesLookupFailures(t *testing.T) {
	t.Parallel()

	svc, absenceRepo, _ := voService(&voOpeningRepoMock{})
	absenceRepo.getByStaffAndDateRangeFunc = func(context.Context, int64, timezone.Date, timezone.Date) ([]*activeModels.StaffAbsence, error) {
		return nil, errors.New("db down")
	}

	err := svc.ValidateVacationOpeningAbsencesBefore(context.Background(), 41, timezone.TodayDate())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to check absences for vacation opening")
}
