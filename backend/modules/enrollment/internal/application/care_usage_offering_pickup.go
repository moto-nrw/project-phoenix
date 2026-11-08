package application

import (
	"strings"

	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"
	offeringContract "github.com/moto-nrw/project-phoenix/modules/enrollment/offeringcontract"
)

// careUsageRequestedPickupByDay is the Gehzeit a request child asks for per
// weekday: the parents' answer to a pickup question, and on the days it
// leaves open the Gehzeit of the booked offerings (#6044). A school that
// books its Gehzeit through offering variants then sees it in the phase
// report, its pickup filter and the export, not „–“.
//
// The offering part follows the care plan projection
// (careplan/internal/domain.ProjectOfferingLinks): only active offerings
// that count as care, a fixed offering without selected days covers its
// available days, and the later time wins when two booked offerings carry
// one for the same day. The form answer keeps precedence, as the maintained
// weekly time does over an offering time in the care plan.
func careUsageRequestedPickupByDay(
	formAnswers map[string]string,
	links []*enrollmentNative.RequestChildOfferingRecord,
	offeringByID map[int64]*offeringContract.CareOfferingRecord,
) map[string]string {
	out := make(map[string]string, len(formAnswers))
	for day, pickupTime := range careUsageOfferingPickupByDay(links, offeringByID) {
		out[day] = pickupTime
	}
	for day, pickupTime := range formAnswers {
		out[day] = pickupTime
	}
	return out
}

func careUsageOfferingPickupByDay(
	links []*enrollmentNative.RequestChildOfferingRecord,
	offeringByID map[int64]*offeringContract.CareOfferingRecord,
) map[string]string {
	out := map[string]string{}
	for _, link := range links {
		if link == nil {
			continue
		}
		offering := offeringByID[link.CareOfferingID]
		if offering == nil || !offering.IsActive || !offering.CountsAsCare || len(offering.PickupTimes) == 0 {
			continue
		}
		days := link.SelectedDays
		if len(days) == 0 && offering.DaysOfWeekMode == offeringContract.DaysOfWeekModeFixed {
			days = offering.AvailableDays
		}
		for _, day := range days {
			key := strings.ToLower(strings.TrimSpace(day))
			pickupTime := strings.TrimSpace(offering.PickupTimes[key])
			// HH:MM compares in clock order as text.
			if pickupTime != "" && pickupTime > out[key] {
				out[key] = pickupTime
			}
		}
	}
	return out
}
