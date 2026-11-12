package api

import (
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// seedTraegerHolidaySchools verifies that the Träger hosts the published
// holiday period and plans the separate parallel-week draft (#5689).
func seedTraegerHolidaySchools(rt *Runtime, auth AuthRef) ([]int64, error) {
	schools, err := seedTraegerSchoolIDs(rt.Client, auth)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(schools, rt.Bootstrap.SchoolID) {
		return nil, fmt.Errorf("the Träger does not list school %d to host the holiday weeks", rt.Bootstrap.SchoolID)
	}
	if err := seedParallelHolidayDraft(rt.Client, auth, calendar.TodayDate(), schools); err != nil {
		return nil, err
	}
	return schools, nil
}

// seedParallelHolidayDraft plans the same week at two schools of the Träger
// (#5689). It stays a draft: no family is sent to the manual test school, and
// the Träger-Büro can review the planning of parallel weeks. Like the period
// it moves on a week while the week holds a day without care (#4702): a
// closing day an earlier seed run put into a booked week (#6434) stays.
func seedParallelHolidayDraft(client *Client, auth AuthRef, today calendar.Date, schools []int64) error {
	if len(schools) < 2 {
		return nil
	}
	for shift := 0; ; shift++ {
		draft := seedHolidayPeriodShifted(today, schools[0], shift)
		weeks, _ := draft["weeks"].([]map[string]any)
		parallel := maps.Clone(weeks[0])
		parallel["host_school_id"] = schools[1]
		draft["name"] = "Ferienbetreuung an zwei Schulen (Entwurf)"
		draft["weeks"] = []map[string]any{weeks[0], parallel}
		_, err := client.PostWithAuth(auth, "/traeger/holiday-periods/", draft)
		var refused *APIError
		if shift+1 < seedHolidayPeriodShifts && errors.As(err, &refused) && refused.Code == seedHolidayWeekNoCareDayCode {
			continue
		}
		if err != nil {
			return fmt.Errorf("create the parallel holiday draft: %w", err)
		}
		fmt.Println("  Träger-Büro: draft with the same week at two schools planned")
		return nil
	}
}

// seedHolidayConsentVersion and seedHolidayRetentionDays are what a seeded
// holiday registration agrees to (#6691): the current consent texts with the
// registry default of holiday_registration.retention_days, which the seed
// never changes.
const (
	seedHolidayConsentVersion = 5
	seedHolidayRetentionDays  = 365
)
