package carecontract

import "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"

// ChangeImpact preserves the approval's consequences for the school to review.
// IDs travel as strings, matching the existing care-plan impact presentation.
type ChangeImpact struct {
	EffectiveFrom   calendar.Date            `json:"effective_from"`
	Adjusted        []ChangeAdjustedTemplate `json:"adjusted"`
	NotAdjusted     []ChangeManualTemplate   `json:"not_adjusted"`
	PickupOverrides []ChangePickupOverride   `json:"pickup_overrides"`
}

type ChangeAdjustedTemplate struct {
	TemplateID      string `json:"template_id"`
	Name            string `json:"name"`
	RemovedWeekdays []int  `json:"removed_weekdays"`
	AddedWeekdays   []int  `json:"added_weekdays"`
}

type ChangeManualTemplate struct {
	TemplateID string         `json:"template_id"`
	Name       string         `json:"name"`
	Weekdays   []int          `json:"weekdays"`
	ViaTarget  bool           `json:"via_target"`
	Until      *calendar.Date `json:"until"`
}

type ChangePickupOverride struct {
	Weekday    int           `json:"weekday"`
	Date       calendar.Date `json:"date"`
	StoredTime string        `json:"stored_time"`
	BookedTime string        `json:"booked_time"`
	ResetNow   bool          `json:"reset_now"`
}
