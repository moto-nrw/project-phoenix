package importpkg

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	importModels "github.com/moto-nrw/project-phoenix/models/import"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Booking side of the opening balance import (#2132). The ledger writes go
// through narrow service interfaces, so these tests drive the config against
// recording doubles: what matters is WHICH sides of a row get booked, in which
// order, and how a ledger rejection is reported back to the uploader.

// --- doubles ---------------------------------------------------------------

type recordingOpeningBooker struct {
	events      *[]string
	validateErr error
	createErr   error

	lastMinutes int
	lastNote    string
	lastDate    timezone.Date
}

func (b *recordingOpeningBooker) ValidateOpeningBalance(_ context.Context, _, _ int64, _ timezone.Date, _ int, _ string) error {
	*b.events = append(*b.events, "validate_hours")
	return b.validateErr
}

func (b *recordingOpeningBooker) CreateOpeningBalance(_ context.Context, staffID, decidedBy int64, effectiveDate timezone.Date, balanceMinutes int, note string) (*activeModels.StaffBalanceAdjustment, error) {
	*b.events = append(*b.events, "create_hours")
	b.lastMinutes = balanceMinutes
	b.lastNote = note
	b.lastDate = effectiveDate
	if b.createErr != nil {
		return nil, b.createErr
	}
	return &activeModels.StaffBalanceAdjustment{
		StaffID:       staffID,
		Type:          activeModels.BalanceAdjustmentTypeOpening,
		MinutesDelta:  balanceMinutes,
		EffectiveDate: effectiveDate,
		DecidedBy:     decidedBy,
		Note:          note,
	}, nil
}

type recordingVacationBooker struct {
	events *[]string

	summary       *activeSvc.VacationQuotaSummary
	summaryErr    error
	upsertErr     error
	openingErr    error
	preflightErr  error
	lastEntitled  float64
	lastCarryover float64
	lastRemaining float64
	lastNote      string
}

func (b *recordingVacationBooker) GetVacationQuotaSummary(_ context.Context, staffID int64, year int) (*activeSvc.VacationQuotaSummary, error) {
	*b.events = append(*b.events, "read_quota")
	if b.summaryErr != nil {
		return nil, b.summaryErr
	}
	if b.summary != nil {
		return b.summary, nil
	}
	return &activeSvc.VacationQuotaSummary{StaffID: staffID, Year: year, EntitledDays: 30, CarryoverDays: 0}, nil
}

func (b *recordingVacationBooker) UpsertVacationQuota(_ context.Context, _ int64, _ int, entitled, carryover float64) error {
	*b.events = append(*b.events, "write_quota")
	b.lastEntitled = entitled
	b.lastCarryover = carryover
	return b.upsertErr
}

func (b *recordingVacationBooker) SetVacationOpening(_ context.Context, staffID, decidedBy int64, req activeSvc.SetVacationOpeningRequest) (*activeModels.StaffVacationOpening, error) {
	*b.events = append(*b.events, "create_vacation_opening")
	b.lastRemaining = req.RemainingDays
	b.lastNote = req.Note
	if b.openingErr != nil {
		return nil, b.openingErr
	}
	return &activeModels.StaffVacationOpening{
		StaffID:              staffID,
		Year:                 req.EffectiveDate.Year,
		EffectiveDate:        req.EffectiveDate,
		EnteredRemainingDays: req.RemainingDays,
		DecidedBy:            decidedBy,
	}, nil
}

func (b *recordingVacationBooker) ValidateVacationOpeningAbsencesBefore(_ context.Context, _ int64, _ timezone.Date) error {
	*b.events = append(*b.events, "validate_vacation")
	return b.preflightErr
}

// listerFunc adapts a closure to the repository read side used by the preload.
type listerFunc[T any] func(ctx context.Context, options *modelBase.QueryOptions) ([]T, error)

func (f listerFunc[T]) List(ctx context.Context, options *modelBase.QueryOptions) ([]T, error) {
	return f(ctx, options)
}

// --- helpers ---------------------------------------------------------------

func float64Ptr(v float64) *float64 { return &v }

func intPtr(v int) *int { return &v }

// newBookingConfig wires the preloaded caches from newOpeningConfig with
// recording booking doubles.
func newBookingConfig(events *[]string) (*OpeningBalanceImportConfig, *recordingOpeningBooker, *recordingVacationBooker) {
	hours := &recordingOpeningBooker{events: events}
	vacation := &recordingVacationBooker{events: events}
	c := newOpeningConfig()
	c.BalanceAdjustService = hours
	c.StaffAbsenceService = vacation
	return c, hours, vacation
}

// --- construction & preload ------------------------------------------------

func TestNewOpeningBalanceImportConfig_CarriesRequestScopedValues(t *testing.T) {
	t.Parallel()

	effectiveDate := timezone.NewDate(2026, time.March, 1)

	c := NewOpeningBalanceImportConfig(OpeningBalanceImportDeps{}, effectiveDate, "Übernahme", openingDecidedByID)

	assert.Equal(t, effectiveDate, c.EffectiveDate)
	assert.Equal(t, "Übernahme", c.Note)
	assert.Equal(t, openingDecidedByID, c.DecidedByStaffID)
	assert.Equal(t, "Eröffnungssaldo", c.EntityName())
	// The import is create-only; corrections run through the per-person UI.
	require.NoError(t, c.Update(t.Context(), 1, importModels.OpeningBalanceImportRow{}))
	existing, err := c.FindExisting(t.Context(), importModels.OpeningBalanceImportRow{})
	require.NoError(t, err)
	assert.Nil(t, existing)
}

func TestPreloadReferenceData_BuildsMatchAndDuplicateCaches(t *testing.T) {
	t.Parallel()

	blankPersonnelNumber := "   "
	staffWithBlankNumber := openingStaff(openingStaffBerndID, "", "Bernd", "Schulz")
	staffWithBlankNumber.PersonnelNumber = &blankPersonnelNumber

	c := NewOpeningBalanceImportConfig(OpeningBalanceImportDeps{
		StaffRepo: &testpkg.StaffRepoMock{
			ListAllWithPersonFn: func(_ context.Context) ([]*userModels.Staff, error) {
				return []*userModels.Staff{
					openingStaff(openingStaffAnnaID, "P-100", "Anna", "Lehmann"),
					staffWithBlankNumber,
					// No person relation: nothing to key a name match on.
					{},
				}, nil
			},
		},
		AdjustmentRepo: listerFunc[*activeModels.StaffBalanceAdjustment](
			func(_ context.Context, _ *modelBase.QueryOptions) ([]*activeModels.StaffBalanceAdjustment, error) {
				return []*activeModels.StaffBalanceAdjustment{{StaffID: openingStaffAnnaID}}, nil
			}),
		VacationOpeningRepo: listerFunc[*activeModels.StaffVacationOpening](
			func(_ context.Context, _ *modelBase.QueryOptions) ([]*activeModels.StaffVacationOpening, error) {
				return []*activeModels.StaffVacationOpening{{StaffID: openingStaffBerndID}}, nil
			}),
	}, timezone.NewDate(openingEffectiveYear, openingEffectiveMonth, openingEffectiveDay), "Übernahme", openingDecidedByID)

	require.NoError(t, c.PreloadReferenceData(t.Context()))

	assert.Equal(t, openingStaffAnnaID, c.staffByPersonnelNumber["p-100"].ID)
	assert.NotContains(t, c.staffByPersonnelNumber, "", "a blank Personalnummer must not become a match key")
	assert.Len(t, c.staffByName[staffNameKey("Anna", "Lehmann")], 1)
	assert.True(t, c.staffWithHoursOpening[openingStaffAnnaID])
	assert.True(t, c.staffWithVacOpening[openingStaffBerndID])
}

func TestPreloadReferenceData_PropagatesRepositoryErrors(t *testing.T) {
	t.Parallel()

	boom := errors.New("db down")
	okStaff := &testpkg.StaffRepoMock{
		ListAllWithPersonFn: func(_ context.Context) ([]*userModels.Staff, error) { return nil, nil },
	}
	okAdjustments := listerFunc[*activeModels.StaffBalanceAdjustment](
		func(_ context.Context, _ *modelBase.QueryOptions) ([]*activeModels.StaffBalanceAdjustment, error) {
			return nil, nil
		})

	tests := []struct {
		name    string
		deps    OpeningBalanceImportDeps
		wantMsg string
	}{
		{
			name: "staff",
			deps: OpeningBalanceImportDeps{StaffRepo: &testpkg.StaffRepoMock{
				ListAllWithPersonFn: func(_ context.Context) ([]*userModels.Staff, error) { return nil, boom },
			}},
			wantMsg: "preload staff",
		},
		{
			name: "hours openings",
			deps: OpeningBalanceImportDeps{
				StaffRepo: okStaff,
				AdjustmentRepo: listerFunc[*activeModels.StaffBalanceAdjustment](
					func(_ context.Context, _ *modelBase.QueryOptions) ([]*activeModels.StaffBalanceAdjustment, error) {
						return nil, boom
					}),
			},
			wantMsg: "preload hours openings",
		},
		{
			name: "vacation openings",
			deps: OpeningBalanceImportDeps{
				StaffRepo:      okStaff,
				AdjustmentRepo: okAdjustments,
				VacationOpeningRepo: listerFunc[*activeModels.StaffVacationOpening](
					func(_ context.Context, _ *modelBase.QueryOptions) ([]*activeModels.StaffVacationOpening, error) {
						return nil, boom
					}),
			},
			wantMsg: "preload vacation openings",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := NewOpeningBalanceImportConfig(tc.deps, timezone.TodayDate(), "Übernahme", openingDecidedByID)

			err := c.PreloadReferenceData(t.Context())

			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantMsg)
		})
	}
}

// --- booking ---------------------------------------------------------------

func TestOpeningBalanceCreate_BooksOnlyTheSidesTheRowCarries(t *testing.T) {
	t.Parallel()

	t.Run("hours only", func(t *testing.T) {
		events := []string{}
		c, hours, _ := newBookingConfig(&events)

		id, err := c.create(t.Context(), importModels.OpeningBalanceImportRow{
			StaffID:             openingStaffAnnaID,
			HoursBalanceMinutes: intPtr(-330),
		})

		require.NoError(t, err)
		assert.Equal(t, openingStaffAnnaID, id)
		assert.Equal(t, []string{"create_hours"}, events)
		assert.Equal(t, -330, hours.lastMinutes, "a migrated account may start negative")
		assert.Equal(t, c.Note, hours.lastNote, "the file-wide Begründung lands on the booking")
		assert.Equal(t, c.EffectiveDate, hours.lastDate)
	})

	t.Run("vacation only", func(t *testing.T) {
		events := []string{}
		c, _, vacation := newBookingConfig(&events)

		_, err := c.create(t.Context(), importModels.OpeningBalanceImportRow{
			StaffID:               openingStaffAnnaID,
			VacationRemainingDays: float64Ptr(17.5),
		})

		require.NoError(t, err)
		assert.Equal(t, []string{"create_vacation_opening"}, events)
		assert.InDelta(t, 17.5, vacation.lastRemaining, 0.0001)
		assert.Equal(t, c.Note, vacation.lastNote)
	})

	t.Run("both sides in quota-then-takeover order", func(t *testing.T) {
		events := []string{}
		c, _, vacation := newBookingConfig(&events)

		_, err := c.create(t.Context(), importModels.OpeningBalanceImportRow{
			StaffID:               openingStaffAnnaID,
			HoursBalanceMinutes:   intPtr(750),
			VacationEntitledDays:  float64Ptr(28),
			VacationCarryoverDays: float64Ptr(3),
			VacationRemainingDays: float64Ptr(20),
		})

		require.NoError(t, err)
		// The takeover derives taken_before from the quota, so the quota must
		// be written first.
		assert.Equal(t, []string{"create_hours", "read_quota", "write_quota", "create_vacation_opening"}, events)
		assert.InDelta(t, 28, vacation.lastEntitled, 0.0001)
		assert.InDelta(t, 3, vacation.lastCarryover, 0.0001)
	})

	t.Run("empty row books nothing", func(t *testing.T) {
		events := []string{}
		c, _, _ := newBookingConfig(&events)

		_, err := c.create(t.Context(), importModels.OpeningBalanceImportRow{StaffID: openingStaffAnnaID})

		require.NoError(t, err)
		assert.Empty(t, events)
	})
}

func TestOpeningBalanceCreate_QuotaKeepsUnsuppliedComponent(t *testing.T) {
	t.Parallel()

	events := []string{}
	c, _, vacation := newBookingConfig(&events)
	vacation.summary = &activeSvc.VacationQuotaSummary{EntitledDays: 30, CarryoverDays: 4}

	// Only the Übertrag column is filled: the persisted Jahresanspruch must
	// survive untouched.
	_, err := c.create(t.Context(), importModels.OpeningBalanceImportRow{
		StaffID:               openingStaffAnnaID,
		VacationCarryoverDays: float64Ptr(1.5),
	})

	require.NoError(t, err)
	assert.InDelta(t, 30, vacation.lastEntitled, 0.0001)
	assert.InDelta(t, 1.5, vacation.lastCarryover, 0.0001)
}

func TestOpeningBalanceCreate_RejectsRowWithoutResolvedStaff(t *testing.T) {
	t.Parallel()

	events := []string{}
	c, _, _ := newBookingConfig(&events)

	_, err := c.Create(t.Context(), importModels.OpeningBalanceImportRow{HoursBalanceMinutes: intPtr(60)})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "ohne aufgelöste Person")
	assert.Empty(t, events)
}

func TestOpeningBalanceCreate_RequiresATransactionForItsSavepoint(t *testing.T) {
	t.Parallel()

	// A failed row must roll back its own writes without losing the rest of
	// the file, which is what the savepoint is for — no transaction, no row.
	events := []string{}
	c, _, _ := newBookingConfig(&events)

	_, err := c.Create(t.Context(), importModels.OpeningBalanceImportRow{
		StaffID:             openingStaffAnnaID,
		HoursBalanceMinutes: intPtr(60),
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "transaction is required")
}

func TestOpeningBalanceCreate_TranslatesLedgerRejections(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		err     error
		wantMsg string
	}{
		{
			name:    "second hours opening",
			err:     activeSvc.ErrOpeningAlreadyExists,
			wantMsg: "bereits ein Stundenkonto-Eröffnungssaldo",
		},
		{
			name:    "closed month",
			err:     activeSvc.ErrAdjustmentInClosedMonth,
			wantMsg: "bereits abgeschlossen",
		},
		{
			name:    "dependent reset",
			err:     activeSvc.ErrAdjustmentHasDependentReset,
			wantMsg: "spätere Stundenkonto-Buchungen",
		},
		{
			name:    "unmapped error passes through",
			err:     errors.New("connection reset"),
			wantMsg: "connection reset",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			events := []string{}
			c, hours, _ := newBookingConfig(&events)
			hours.createErr = tc.err

			_, err := c.create(t.Context(), importModels.OpeningBalanceImportRow{
				StaffID:             openingStaffAnnaID,
				HoursBalanceMinutes: intPtr(60),
			})

			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantMsg)
		})
	}
}

func TestOpeningBalanceCreate_TranslatesVacationRejections(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		err     error
		wantMsg string
	}{
		{
			name:    "second takeover",
			err:     activeSvc.ErrVacationOpeningExists,
			wantMsg: "bereits eine Urlaubs-Übernahme",
		},
		{
			name:    "absences before the Stichtag",
			err:     activeSvc.ErrVacationOpeningAbsencesBeforeCutoff,
			wantMsg: "doppelt zählen",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			events := []string{}
			c, _, vacation := newBookingConfig(&events)
			vacation.openingErr = tc.err

			_, err := c.create(t.Context(), importModels.OpeningBalanceImportRow{
				StaffID:               openingStaffAnnaID,
				VacationRemainingDays: float64Ptr(10),
			})

			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantMsg)
		})
	}
}

func TestOpeningBalanceCreate_ReportsQuotaFailures(t *testing.T) {
	t.Parallel()

	t.Run("read", func(t *testing.T) {
		events := []string{}
		c, _, vacation := newBookingConfig(&events)
		vacation.summaryErr = errors.New("db down")

		_, err := c.create(t.Context(), importModels.OpeningBalanceImportRow{
			StaffID:              openingStaffAnnaID,
			VacationEntitledDays: float64Ptr(30),
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "urlaubskonto konnte nicht gelesen werden")
	})

	t.Run("write", func(t *testing.T) {
		events := []string{}
		c, _, vacation := newBookingConfig(&events)
		vacation.upsertErr = errors.New("db down")

		_, err := c.create(t.Context(), importModels.OpeningBalanceImportRow{
			StaffID:              openingStaffAnnaID,
			VacationEntitledDays: float64Ptr(30),
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "urlaubsanspruch konnte nicht gespeichert werden")
	})
}

// --- preview guards --------------------------------------------------------

func TestOpeningBalanceValidate_ReportsLedgerPreflightRejection(t *testing.T) {
	t.Parallel()

	events := []string{}
	c, hours, _ := newBookingConfig(&events)
	hours.validateErr = activeSvc.ErrAdjustmentInClosedMonth

	row := &importModels.OpeningBalanceImportRow{
		FirstName: "Anna", LastName: "Lehmann", HoursBalance: "12,5",
	}
	errs := c.Validate(t.Context(), row)

	require.Contains(t, errorCodes(errs), "opening_preflight_failed")
	assert.Contains(t, errs[0].Message, "bereits abgeschlossen")
}

func TestOpeningBalanceValidate_ReportsVacationPreflightRejection(t *testing.T) {
	t.Parallel()

	events := []string{}
	c, _, vacation := newBookingConfig(&events)
	vacation.preflightErr = activeSvc.ErrVacationOpeningAbsencesBeforeCutoff

	row := &importModels.OpeningBalanceImportRow{
		FirstName: "Anna", LastName: "Lehmann", VacationRemaining: "17,5",
	}
	errs := c.Validate(t.Context(), row)

	require.Contains(t, errorCodes(errs), "vacation_opening_preflight_failed")
}

func TestOpeningBalanceValidate_ReportsQuotaLookupFailure(t *testing.T) {
	t.Parallel()

	events := []string{}
	c, _, vacation := newBookingConfig(&events)
	vacation.summaryErr = errors.New("db down")

	row := &importModels.OpeningBalanceImportRow{
		FirstName: "Anna", LastName: "Lehmann", VacationRemaining: "17,5",
	}
	errs := c.Validate(t.Context(), row)

	assert.Contains(t, errorCodes(errs), "vacation_quota_lookup_failed")
}

func TestOpeningBalanceValidate_RejectsDerivedTakeoverOutsideTheModelRange(t *testing.T) {
	t.Parallel()

	events := []string{}
	c, _, vacation := newBookingConfig(&events)
	// Entitled + carryover − Resturlaub must stay inside NUMERIC(5,1).
	vacation.summary = &activeSvc.VacationQuotaSummary{EntitledDays: 30, CarryoverDays: 0}

	row := &importModels.OpeningBalanceImportRow{
		FirstName: "Anna", LastName: "Lehmann", VacationRemaining: "-990",
	}
	errs := c.Validate(t.Context(), row)

	require.Contains(t, errorCodes(errs), "invalid_vacation_opening")
}

func TestOpeningBalanceValidate_DerivesTakeoverFromTheRowsOwnQuotaColumns(t *testing.T) {
	t.Parallel()

	events := []string{}
	c, _, vacation := newBookingConfig(&events)
	// Persisted quota is 30 + 0, but the same row raises it to 28 + 4. The
	// derived takeover must use the uploaded values, not the stored ones.
	vacation.summary = &activeSvc.VacationQuotaSummary{EntitledDays: 30, CarryoverDays: 0}

	row := &importModels.OpeningBalanceImportRow{
		FirstName:         "Anna",
		LastName:          "Lehmann",
		VacationEntitled:  "28",
		VacationCarryover: "4",
		VacationRemaining: "12",
	}
	errs := c.Validate(t.Context(), row)

	require.Empty(t, errs)
	require.NotNil(t, row.VacationEntitledDays)
	assert.InDelta(t, 28, *row.VacationEntitledDays, 0.0001)
}
