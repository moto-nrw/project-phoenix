package api

import (
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
// the Träger-Büro can review the planning of parallel weeks.
func seedParallelHolidayDraft(client *Client, auth AuthRef, today calendar.Date, schools []int64) error {
	if len(schools) < 2 {
		return nil
	}
	draft := seedHolidayPeriod(today, schools[0])
	weeks, _ := draft["weeks"].([]map[string]any)
	parallel := maps.Clone(weeks[0])
	parallel["host_school_id"] = schools[1]
	draft["name"] = "Ferienbetreuung an zwei Schulen (Entwurf)"
	draft["weeks"] = []map[string]any{weeks[0], parallel}
	if _, err := client.PostWithAuth(auth, "/traeger/holiday-periods/", draft); err != nil {
		return fmt.Errorf("create the parallel holiday draft: %w", err)
	}
	fmt.Println("  Träger-Büro: draft with the same week at two schools planned")
	return nil
}
