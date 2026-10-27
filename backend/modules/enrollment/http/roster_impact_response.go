package enrollmenthttp

import (
	"strconv"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
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
}

func newRosterImpactResponse(impact RosterImpact) rosterImpactResponse {
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
		out.NotAdjusted = append(out.NotAdjusted, manualRosterTemplateResponse{
			TemplateID: strconv.FormatInt(template.TemplateID, 10), Name: template.Name,
			Weekdays: nonNilWeekdays(template.Weekdays), ViaTarget: template.ViaTarget,
		})
	}
	return out
}

func nonNilWeekdays(weekdays []int) []int {
	if weekdays == nil {
		return []int{}
	}
	return weekdays
}

// childRosterImpactResponse is one child's roster impact of an approved
// change request.
type childRosterImpactResponse struct {
	ChildID      string               `json:"child_id"`
	StudentID    string               `json:"student_id"`
	FirstName    string               `json:"first_name"`
	RosterImpact rosterImpactResponse `json:"roster_impact"`
}

func newChildRosterImpactResponses(impacts []capability.ChildRosterImpact) []childRosterImpactResponse {
	if len(impacts) == 0 {
		return nil
	}
	out := make([]childRosterImpactResponse, 0, len(impacts))
	for _, impact := range impacts {
		out = append(out, childRosterImpactResponse{
			ChildID: strconv.FormatInt(impact.ChildID, 10), StudentID: strconv.FormatInt(impact.StudentID, 10),
			FirstName: impact.FirstName, RosterImpact: newRosterImpactResponse(impact.Impact),
		})
	}
	return out
}
