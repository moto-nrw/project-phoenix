package application

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// schoolVacationSyncAheadDays is how far the sync reads ahead of today: two
// years, so the next school year is known before it starts.
const schoolVacationSyncAheadDays = 730

// SchoolVacationDependencies wire the school vacations (#5345). FederalState
// resolves the tenant's operations.federal_state; nil serves only the sync,
// and every school read answers schoolcalendar.ErrFederalStateUnavailable.
// Today names the Berlin calendar day, nil means BerlinToday.
// AdminTransaction opens the cross-tenant transaction of the sync.
type SchoolVacationDependencies struct {
	Store            ports.SchoolVacationStore
	Source           ports.SchoolVacationSource
	FederalState     func(context.Context) (string, error)
	Today            ports.Today
	AdminTransaction ports.AdminTransaction
}

// SchoolVacationService serves the vacations of the tenant's federal state,
// the school's confirmations of planned care and the platform-wide sync.
type SchoolVacationService struct {
	deps SchoolVacationDependencies
}

var (
	_ schoolcalendar.SchoolVacations    = (*SchoolVacationService)(nil)
	_ schoolcalendar.SchoolVacationSync = (*SchoolVacationService)(nil)
	_ schoolcalendar.SchoolVacationSeed = (*SchoolVacationService)(nil)
)

func NewSchoolVacations(deps SchoolVacationDependencies) (*SchoolVacationService, error) {
	if deps.Store == nil || deps.Source == nil || deps.AdminTransaction == nil {
		return nil, errors.New("school calendar school vacations: store, source and transaction are required")
	}
	if deps.FederalState == nil {
		deps.FederalState = func(context.Context) (string, error) {
			return "", errors.New("school vacations: no federal state in this composition")
		}
	}
	if deps.Today == nil {
		deps.Today = BerlinToday
	}
	return &SchoolVacationService{deps: deps}, nil
}

func (s *SchoolVacationService) TenantSchoolVacations(ctx context.Context, from, to string) (schoolcalendar.SchoolVacationList, error) {
	if err := validateSchoolVacationWindow(from, to); err != nil {
		return schoolcalendar.SchoolVacationList{}, err
	}
	region, err := s.region(ctx)
	if err != nil {
		return schoolcalendar.SchoolVacationList{}, err
	}
	vacations, err := s.deps.Store.ListSchoolVacations(ctx, region, from, to)
	if err != nil {
		return schoolcalendar.SchoolVacationList{}, err
	}
	synced, err := s.deps.Store.LastSchoolVacationSync(ctx, region)
	if err != nil {
		return schoolcalendar.SchoolVacationList{}, err
	}
	return schoolcalendar.SchoolVacationList{
		Region:    schoolcalendar.HolidayRegion{Code: region, Name: HolidayRegionName(region)},
		Vacations: vacations, SyncedAt: synced,
	}, nil
}

func (s *SchoolVacationService) AcknowledgeSchoolVacation(ctx context.Context, acknowledgement schoolcalendar.SchoolVacationAcknowledgement) error {
	if err := validateSchoolVacationRange(acknowledgement.StartDate, acknowledgement.EndDate); err != nil {
		return err
	}
	region, err := s.region(ctx)
	if err != nil {
		return err
	}
	exists, err := s.deps.Store.HasSchoolVacation(ctx, region, acknowledgement.StartDate, acknowledgement.EndDate)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("%w: %s %s to %s", schoolcalendar.ErrSchoolVacationNotFound, region, acknowledgement.StartDate, acknowledgement.EndDate)
	}
	return s.deps.Store.AcknowledgeSchoolVacation(ctx, region, acknowledgement.StartDate, acknowledgement.EndDate, acknowledgement.AccountID)
}

// SyncSchoolVacations reads the vacations of every federal state from the
// start of the running school year to two years ahead, then replaces the
// stored windows of each state the answer names, all in one administrative
// transaction. The request runs before the transaction opens. A failed
// request changes nothing; a state the answer does not name keeps its rows.
func (s *SchoolVacationService) SyncSchoolVacations(ctx context.Context) (schoolcalendar.SchoolVacationSyncResult, error) {
	from, to, err := schoolVacationSyncWindow(s.deps.Today())
	if err != nil {
		return schoolcalendar.SchoolVacationSyncResult{}, err
	}
	records, err := s.deps.Source(ctx, from, to)
	if err != nil {
		return schoolcalendar.SchoolVacationSyncResult{}, fmt.Errorf("fetch school vacations: %w", err)
	}
	byRegion := groupSchoolVacations(records)
	result := schoolcalendar.SchoolVacationSyncResult{}
	err = s.deps.AdminTransaction(ctx, func(txCtx context.Context) error {
		result = schoolcalendar.SchoolVacationSyncResult{}
		for _, region := range sortedRegions(byRegion) {
			stored, replaceErr := s.deps.Store.ReplaceSchoolVacations(txCtx, region, from, to, byRegion[region])
			if replaceErr != nil {
				return fmt.Errorf("replace school vacations of %s: %w", region, replaceErr)
			}
			result.Regions++
			result.Vacations += stored
		}
		return nil
	})
	if err != nil {
		return schoolcalendar.SchoolVacationSyncResult{}, err
	}
	return result, nil
}

// SeedSchoolVacations stores the given windows for every federal state in
// one administrative transaction, replacing the stored windows between the
// first and the last seeded day. It reads nothing from the external source.
func (s *SchoolVacationService) SeedSchoolVacations(ctx context.Context, vacations []schoolcalendar.SchoolVacation) (int, error) {
	if len(vacations) == 0 {
		return 0, nil
	}
	from, to := vacations[0].StartDate, vacations[0].EndDate
	for _, vacation := range vacations {
		if err := validateSchoolVacationRange(vacation.StartDate, vacation.EndDate); err != nil {
			return 0, err
		}
		from, to = min(from, vacation.StartDate), max(to, vacation.EndDate)
	}
	stored := 0
	err := s.deps.AdminTransaction(ctx, func(txCtx context.Context) error {
		stored = 0
		for _, region := range SchoolVacationRegions() {
			records := make([]domain.SchoolVacationRecord, 0, len(vacations))
			for _, vacation := range vacations {
				records = append(records, domain.SchoolVacationRecord{Region: region, Name: vacation.Name, StartDate: vacation.StartDate, EndDate: vacation.EndDate})
			}
			count, err := s.deps.Store.ReplaceSchoolVacations(txCtx, region, from, to, records)
			if err != nil {
				return fmt.Errorf("seed school vacations of %s: %w", region, err)
			}
			stored += count
		}
		return nil
	})
	return stored, err
}

// SchoolVacationRegions are the federal states the sync stores, the same
// regions as the statutory holidays.
func SchoolVacationRegions() []string {
	regions := make([]string, 0, len(holidayRegions))
	for region := range holidayRegions {
		regions = append(regions, region)
	}
	sort.Strings(regions)
	return regions
}

func groupSchoolVacations(records []domain.SchoolVacationRecord) map[string][]domain.SchoolVacationRecord {
	byRegion := make(map[string][]domain.SchoolVacationRecord)
	for _, record := range records {
		if !ValidHolidayRegion(record.Region) {
			continue
		}
		byRegion[record.Region] = append(byRegion[record.Region], record)
	}
	return byRegion
}

func sortedRegions(byRegion map[string][]domain.SchoolVacationRecord) []string {
	regions := make([]string, 0, len(byRegion))
	for region := range byRegion {
		regions = append(regions, region)
	}
	sort.Strings(regions)
	return regions
}

// schoolVacationSyncWindow is [1 August of the running school year, today +
// two years]: at most 1095 days, the longest window the source answers.
func schoolVacationSyncWindow(today string) (string, string, error) {
	day, err := calendar.ParseDate(today)
	if err != nil {
		return "", "", fmt.Errorf("school vacation sync: invalid today %q: %w", today, err)
	}
	year := day.Year()
	if day.Month() < 8 {
		year--
	}
	start := calendar.NewDate(year, 8, 1)
	return start.String(), day.AddDays(schoolVacationSyncAheadDays).String(), nil
}

func (s *SchoolVacationService) region(ctx context.Context) (string, error) {
	region, err := s.deps.FederalState(ctx)
	if err != nil {
		return "", fmt.Errorf("%w: %w", schoolcalendar.ErrFederalStateUnavailable, err)
	}
	if !ValidHolidayRegion(region) {
		return "", fmt.Errorf("%w: unsupported region %q", schoolcalendar.ErrFederalStateUnavailable, region)
	}
	return region, nil
}

func validateSchoolVacationRange(from, to string) error {
	start, err := calendar.ParseDate(from)
	if err != nil {
		return fmt.Errorf("%w: from must be a date in YYYY-MM-DD format", schoolcalendar.ErrInvalidSchoolVacationWindow)
	}
	end, err := calendar.ParseDate(to)
	if err != nil {
		return fmt.Errorf("%w: to must be a date in YYYY-MM-DD format", schoolcalendar.ErrInvalidSchoolVacationWindow)
	}
	if end.Before(start) {
		return fmt.Errorf("%w: to must not be before from", schoolcalendar.ErrInvalidSchoolVacationWindow)
	}
	return nil
}

func validateSchoolVacationWindow(from, to string) error {
	if err := validateSchoolVacationRange(from, to); err != nil {
		return err
	}
	start, _ := calendar.ParseDate(from)
	end, _ := calendar.ParseDate(to)
	if start.DaysUntil(end)+1 > schoolcalendar.MaxSchoolVacationWindowDays {
		return fmt.Errorf("%w: window exceeds %d days", schoolcalendar.ErrInvalidSchoolVacationWindow, schoolcalendar.MaxSchoolVacationWindowDays)
	}
	return nil
}
