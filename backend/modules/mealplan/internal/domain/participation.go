package domain

import "time"

type Weekday int

type ParticipationSource string

const (
	ParticipationNone     ParticipationSource = "none"
	ParticipationRegular  ParticipationSource = "regular"
	ParticipationOverride ParticipationSource = "override"
	ParticipationSick     ParticipationSource = "sick"
)

type ParticipationSchedule struct {
	EffectiveFrom Date
	Weekdays      []Weekday
}

type DayOverride struct {
	Date          Date
	Participating bool
}

type SickDay struct {
	ID         int64
	Date       Date
	ChangedAt  time.Time
	ReportedAt time.Time
	ClearedAt  *time.Time
}

type ParticipationData struct {
	Schedules []ParticipationSchedule
	Overrides []DayOverride
	SickDays  []SickDay
}

type ParticipationDay struct {
	Date          Date
	Participating bool
	Source        ParticipationSource
	Changeable    bool
}

type ParticipationPlan struct {
	Weekdays      []Weekday
	EffectiveFrom Date
	CutoffTime    string
	Days          []ParticipationDay
}

type DailyCandidate struct {
	StudentID      int64
	FirstName      string
	LastName       string
	SchoolClass    string
	Regular        bool
	Override       *bool
	SickReportedAt *time.Time
	SickClearedAt  *time.Time
}

type DailyParticipant struct {
	StudentID   int64
	FirstName   string
	LastName    string
	SchoolClass string
}

type DailyList struct {
	Date         Date
	CutoffTime   string
	Participants []DailyParticipant
}
