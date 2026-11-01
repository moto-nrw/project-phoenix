package careplanhttp

import (
	"strconv"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

// rosterImpactResponse is the wire form of a booking change's effect on the
// child's Regeltermine (#5102). Weekdays are ISO (1 = Monday); template IDs
// travel as strings like every other ID.
type rosterImpactResponse struct {
	EffectiveFrom string                           `json:"effective_from"`
	Adjusted      []adjustedRosterTemplateResponse `json:"adjusted"`
	NotAdjusted   []manualRosterTemplateResponse   `json:"not_adjusted"`
}

type adjustedRosterTemplateResponse struct {
	TemplateID      string `json:"template_id"`
	Name            string `json:"name"`
	RemovedWeekdays []int  `json:"removed_weekdays"`
	AddedWeekdays   []int  `json:"added_weekdays"`
}

type manualRosterTemplateResponse struct {
	TemplateID string `json:"template_id"`
	Name       string `json:"name"`
	Weekdays   []int  `json:"weekdays"`
	ViaTarget  bool   `json:"via_target"`
	// Until is the day a later booking carries the Regeltermin again
	// (exclusive, #5497); null when none does. "Entfernen" ends there.
	Until *string `json:"until"`
}

func newRosterImpactResponse(impact careplan.RosterImpact) rosterImpactResponse {
	out := rosterImpactResponse{
		EffectiveFrom: impact.EffectiveFrom.String(),
		Adjusted:      make([]adjustedRosterTemplateResponse, 0, len(impact.Adjusted)),
		NotAdjusted:   make([]manualRosterTemplateResponse, 0, len(impact.NotAdjusted)),
	}
	for _, template := range impact.Adjusted {
		out.Adjusted = append(out.Adjusted, adjustedRosterTemplateResponse{
			TemplateID: strconv.FormatInt(template.TemplateID, 10), Name: template.Name,
			RemovedWeekdays: nonNilWeekdays(template.RemovedWeekdays), AddedWeekdays: nonNilWeekdays(template.AddedWeekdays),
		})
	}
	for _, template := range impact.NotAdjusted {
		response := manualRosterTemplateResponse{
			TemplateID: strconv.FormatInt(template.TemplateID, 10), Name: template.Name,
			Weekdays: nonNilWeekdays(template.Weekdays), ViaTarget: template.ViaTarget,
		}
		if template.Until != nil {
			response.Until = new(template.Until.String())
		}
		out.NotAdjusted = append(out.NotAdjusted, response)
	}
	return out
}

func nonNilWeekdays(weekdays []int) []int {
	if weekdays == nil {
		return []int{}
	}
	return weekdays
}
