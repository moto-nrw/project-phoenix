package owners

import (
	"context"
	"fmt"
	"slices"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
	"github.com/moto-ogs/project-phoenix/modules/organizationtenancy"
	"github.com/moto-ogs/project-phoenix/modules/schoolcalendar"
	"github.com/moto-ogs/project-phoenix/modules/schoolcalendar/traegercalendar"
	"github.com/moto-ogs/project-phoenix/modules/schoolclosingdaysview"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/moto-ogs/project-phoenix/tenant"
)

// TraegerCalendarOrganizations lists the schools of a Träger.
type TraegerCalendarOrganizations interface {
	ListSchoolsByOrganization(ctx context.Context, organizationID int64) ([]organizationtenancy.School, error)
}

// NewTraegerCalendarSchools binds School Calendar's view of a Träger's schools
// (#4702) to Organisation & Tenancy and to the federal state of each school,
// read as for the Betreuungsvertrag (#6350). Deleted schools are left out.
func NewTraegerCalendarSchools(organizations TraegerCalendarOrganizations, settings CareContractHolidaySettings) traegercalendar.Schools {
	if organizations == nil || settings == nil {
		panic("Träger calendar schools: Organisation & Tenancy and the settings are required")
	}
	return traegerCalendarSchools{organizations: organizations, settings: settings}
}

type traegerCalendarSchools struct {
	organizations TraegerCalendarOrganizations
	settings      CareContractHolidaySettings
}

func (s traegerCalendarSchools) TraegerSchools(ctx context.Context, organizationID int64) ([]traegercalendar.SchoolState, error) {
	schools, err := s.organizations.ListSchoolsByOrganization(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	schools = slices.DeleteFunc(schools, func(school organizationtenancy.School) bool { return school.IsDeleted() })
	ids := make([]int64, 0, len(schools))
	for _, school := range schools {
		ids = append(ids, school.ID)
	}
	states, err := schoolFederalStates(ctx, s.settings, ids)
	if err != nil {
		return nil, err
	}
	result := make([]traegercalendar.SchoolState, 0, len(schools))
	for _, school := range schools {
		result = append(result, traegercalendar.SchoolState{ID: school.ID, Name: school.Name, FederalState: states[school.ID]})
	}
	slices.SortFunc(result, func(a, b traegercalendar.SchoolState) int {
		if a.Name != b.Name {
			if a.Name < b.Name {
				return -1
			}
			return 1
		}
		return int(a.ID - b.ID)
	})
	return result, nil
}

// HolidayCalendarDays is what the holiday weeks read of School Calendar: the
// statutory holidays of a federal state and the closing days of the school in
// context, those its Träger planned for it included.
type HolidayCalendarDays interface {
	ValidHolidayRegion(string) bool
	ListHolidays(ctx context.Context, region, from, to string) ([]schoolcalendar.Holiday, error)
	ListClosingDays(ctx context.Context, filter schoolcalendar.ClosingDayFilter) ([]schoolcalendar.ClosingDay, error)
}

// NewHolidaySchoolCalendar binds Enrollment's days without care of its host
// schools (#4702) to School Calendar. The statutory holidays come from each
// school's federal state through the same read as the Betreuungsvertrag's
// (#6350); the closing days are read in a school transaction of each host
// school, which shares nothing with the caller's Träger transaction
// (ADR 0105), as for the Mensa (#4686).
func NewHolidaySchoolCalendar(settings CareContractHolidaySettings, days HolidayCalendarDays, organizations TraegerCalendarOrganizations) enrollment.HolidaySchoolCalendar {
	if settings == nil || days == nil || organizations == nil {
		panic("holiday school calendar: settings, School Calendar and Organisation & Tenancy are required")
	}
	return holidaySchoolCalendar{settings: settings, days: days, organizations: organizations}
}

type holidaySchoolCalendar struct {
	organizations TraegerCalendarOrganizations
	settings      CareContractHolidaySettings
	days          HolidayCalendarDays
}

func (c holidaySchoolCalendar) HolidayNoCareDays(ctx context.Context, schoolIDs []int64, from, to calendar.Date,
) (map[int64][]enrollment.HolidayNoCareDay, error) {
	result := make(map[int64][]enrollment.HolidayNoCareDay, len(schoolIDs))
	if len(schoolIDs) == 0 {
		return result, nil
	}
	if err := c.authorizeHostSchools(ctx, schoolIDs); err != nil {
		return nil, err
	}
	states, err := schoolFederalStates(ctx, c.settings, schoolIDs)
	if err != nil {
		return nil, err
	}
	byRegion := map[string][]enrollment.HolidayNoCareDay{}
	for _, schoolID := range schoolIDs {
		region := states[schoolID]
		if _, done := byRegion[region]; !done {
			if byRegion[region], err = c.regionHolidays(ctx, region, from, to); err != nil {
				return nil, err
			}
		}
		closing, err := c.schoolClosingDays(ctx, schoolID, from, to)
		if err != nil {
			return nil, err
		}
		result[schoolID] = mergeNoCareDays(byRegion[region], closing)
	}
	return result, nil
}

func (c holidaySchoolCalendar) authorizeHostSchools(ctx context.Context, schoolIDs []int64) error {
	// Authorize every supplied school before crossing into school transactions
	// or reading settings. A refusal must reveal none of a foreign calendar.
	organizationID := tenant.TraegerTransactionFromContext(ctx)
	if organizationID <= 0 {
		return enrollment.ErrHolidayHostSchoolOutsideTraeger
	}
	schools, err := c.organizations.ListSchoolsByOrganization(ctx, organizationID)
	if err != nil {
		return err
	}
	for _, id := range schoolIDs {
		if !slices.ContainsFunc(schools, func(school organizationtenancy.School) bool {
			return school.ID == id && school.OrganizationID == organizationID && !school.IsDeleted()
		}) {
			return enrollment.ErrHolidayHostSchoolOutsideTraeger
		}
	}
	return nil
}

func (c holidaySchoolCalendar) regionHolidays(ctx context.Context, region string, from, to calendar.Date) ([]enrollment.HolidayNoCareDay, error) {
	if !c.days.ValidHolidayRegion(region) {
		return nil, fmt.Errorf("%w: unsupported region %q", schoolcalendar.ErrFederalStateUnavailable, region)
	}
	holidays, err := c.days.ListHolidays(ctx, region, from.String(), to.String())
	if err != nil {
		return nil, err
	}
	result := make([]enrollment.HolidayNoCareDay, 0, len(holidays))
	for _, holiday := range holidays {
		day, err := calendar.ParseDate(holiday.Date)
		if err != nil {
			return nil, fmt.Errorf("holiday school calendar: %w", err)
		}
		result = append(result, enrollment.HolidayNoCareDay{Date: day, Kind: enrollment.HolidayNoCareHoliday, Name: holiday.Name})
	}
	return result, nil
}

// schoolClosingDays reads the school's closing days in a school transaction
// of its own and expands them into the days of [from, to].
func (c holidaySchoolCalendar) schoolClosingDays(ctx context.Context, schoolID int64, from, to calendar.Date) ([]enrollment.HolidayNoCareDay, error) {
	id, err := tenant.NewTenantID(schoolID)
	if err != nil {
		return nil, err
	}
	var ranges []schoolcalendar.ClosingDay
	err = tenant.WithinTenant(tenant.ContextWithoutTransaction(ctx), id, func(schoolCtx context.Context) error {
		ranges, err = c.days.ListClosingDays(schoolCtx, schoolcalendar.ClosingDayFilter{OverlappingFrom: from.String(), OverlappingTo: to.String()})
		return err
	})
	if err != nil {
		return nil, err
	}
	var result []enrollment.HolidayNoCareDay
	for _, closing := range ranges {
		start, err := calendar.ParseDate(closing.StartDate)
		if err != nil {
			return nil, fmt.Errorf("holiday school calendar: %w", err)
		}
		end, err := calendar.ParseDate(closing.EndDate)
		if err != nil {
			return nil, fmt.Errorf("holiday school calendar: %w", err)
		}
		for day := max(start, from); !day.After(min(end, to)); day = day.AddDays(1) {
			result = append(result, enrollment.HolidayNoCareDay{Date: day, Kind: enrollment.HolidayNoCareClosingDay, Name: closing.Reason,
				ClosingDays: []enrollment.HolidayClosingDayRef{{ID: closing.ID, Traeger: closing.Traeger}}})
		}
	}
	return result, nil
}

// mergeNoCareDays orders the days by date and keeps one entry per date; a
// statutory holiday names a day that is also a closing day. The entry names
// every closing day of its date, so a changed Träger closing day can leave
// its own stored version out (#6457).
func mergeNoCareDays(holidays, closing []enrollment.HolidayNoCareDay) []enrollment.HolidayNoCareDay {
	merged := append(slices.Clone(holidays), closing...)
	slices.SortStableFunc(merged, func(a, b enrollment.HolidayNoCareDay) int { return a.Date.Compare(b.Date) })
	result := make([]enrollment.HolidayNoCareDay, 0, len(merged))
	for _, day := range merged {
		if n := len(result); n > 0 && result[n-1].Date == day.Date {
			result[n-1].ClosingDays = append(result[n-1].ClosingDays, day.ClosingDays...)
			continue
		}
		day.ClosingDays = slices.Clone(day.ClosingDays)
		result = append(result, day)
	}
	return result
}

// NewSchoolClosingDayReads binds the current-membership projection (ADR 0194).
func NewSchoolClosingDayReads() schoolcalendar.ClosingDayReads { return schoolclosingdaysview.New() }
