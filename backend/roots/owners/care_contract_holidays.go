package owners

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// CareContractHolidaySettings reads settings of several schools in an
// administrative transaction of its own.
type CareContractHolidaySettings interface {
	ResolveManyForTenants(ctx context.Context, tenantIDs []int64, keys []string) (map[int64]*configModels.SettingsSnapshot, error)
}

// NewCareContractSchoolHolidays binds the holidays § 193 BGB moves a
// withdrawal period past (#6350) to School Calendar's statutory holidays of
// each school's federal state (operations.federal_state). The Betreuungsvertrag
// asks inside its Träger transaction, whose row security hides the settings
// of the schools (ADR 0175), so the states are read in a unit of their own
// that shares nothing with it (ADR 0105), as for the Mensa (#4686).
func NewCareContractSchoolHolidays(settings CareContractHolidaySettings, holidays schoolcalendar.HolidayQuery) carecontract.SchoolHolidays {
	if settings == nil || holidays == nil {
		panic("care contract school holidays: the settings and School Calendar's holidays are required")
	}
	return careContractSchoolHolidays{settings: settings, holidays: holidays}
}

type careContractSchoolHolidays struct {
	settings CareContractHolidaySettings
	holidays schoolcalendar.HolidayQuery
}

func (h careContractSchoolHolidays) SchoolHolidayDates(ctx context.Context, schoolIDs []int64, from, to calendar.Date,
) (map[int64]map[calendar.Date]bool, error) {
	result := make(map[int64]map[calendar.Date]bool, len(schoolIDs))
	if len(schoolIDs) == 0 {
		return result, nil
	}
	snapshots, err := h.settings.ResolveManyForTenants(tenant.ContextWithoutTransaction(ctx), schoolIDs,
		[]string{configModels.KeyFederalState})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", schoolcalendar.ErrFederalStateUnavailable, err)
	}
	byRegion := map[string]map[calendar.Date]bool{}
	for _, schoolID := range schoolIDs {
		snapshot := snapshots[schoolID]
		if snapshot == nil {
			return nil, fmt.Errorf("%w: no settings for school %d", schoolcalendar.ErrFederalStateUnavailable, schoolID)
		}
		region, err := snapshot.String(configModels.KeyFederalState)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", schoolcalendar.ErrFederalStateUnavailable, err)
		}
		if byRegion[region] == nil {
			if byRegion[region], err = h.regionHolidays(ctx, region, from, to); err != nil {
				return nil, err
			}
		}
		result[schoolID] = byRegion[region]
	}
	return result, nil
}

// regionHolidays asks School Calendar for the holidays of one region.
func (h careContractSchoolHolidays) regionHolidays(ctx context.Context, region string, from, to calendar.Date) (map[calendar.Date]bool, error) {
	if !h.holidays.ValidHolidayRegion(region) {
		return nil, fmt.Errorf("%w: unsupported region %q", schoolcalendar.ErrFederalStateUnavailable, region)
	}
	dates, err := h.holidays.HolidayDates(ctx, region, from.String(), to.String())
	if err != nil {
		return nil, err
	}
	days := make(map[calendar.Date]bool, len(dates))
	for day := range dates {
		parsed, err := calendar.ParseDate(day)
		if err != nil {
			return nil, fmt.Errorf("care contract school holidays: %w", err)
		}
		days[parsed] = true
	}
	return days, nil
}
