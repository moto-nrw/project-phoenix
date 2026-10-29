package application

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Participant lists of the hosting school (#4665, ADR 0116). The caller runs
// every operation in the tenant transaction of the hosting school. The store
// reads only weeks the school hosts and only children of confirmed
// registrations; this service decides when the access ends: after the last
// day of the week, by the calendar day of its clock.

// HolidayHostStore reads the hosted weeks and their confirmed children in
// the tenant transaction of the hosting school. today is the first day
// whose weeks still count: a week that ended before it is no longer read.
type HolidayHostStore interface {
	ListHostedHolidayWeeks(ctx context.Context, schoolID int64, today calendar.Date) ([]enrollment.HostedHolidayWeek, error)
	// ReadHostedHolidayWeek reads one hosted week with its time models and
	// its confirmed children; found is false for a week the school does not
	// host or that has ended.
	ReadHostedHolidayWeek(ctx context.Context, schoolID, weekID int64, today calendar.Date) (week enrollment.HostedHolidayWeek, models []enrollment.HolidayTimeModelCount, participants []enrollment.HolidayParticipant, found bool, err error)
}

// HolidayHostParticipants serves enrollment.HolidayHostParticipants.
type HolidayHostParticipants struct {
	store HolidayHostStore
	log   enrollment.HolidayParticipantAccessLog
	today func() calendar.Date
}

var _ enrollment.HolidayHostParticipants = (*HolidayHostParticipants)(nil)

// NewHolidayHostParticipants builds the participant lists; a nil today uses
// the Berlin calendar day.
func NewHolidayHostParticipants(store HolidayHostStore, log enrollment.HolidayParticipantAccessLog, today func() calendar.Date) *HolidayHostParticipants {
	if store == nil || log == nil {
		panic("enrollment: the holiday host store and the access log are required")
	}
	if today == nil {
		today = calendar.TodayDate
	}
	return &HolidayHostParticipants{store: store, log: log, today: today}
}

// HostedHolidayWeeks lists the hosted weeks that have not ended.
func (h *HolidayHostParticipants) HostedHolidayWeeks(ctx context.Context, host enrollment.HolidayHost) ([]enrollment.HostedHolidayWeek, error) {
	if host.SchoolID <= 0 {
		return nil, errors.New("enrollment: the hosting school is required")
	}
	weeks, err := h.store.ListHostedHolidayWeeks(ctx, host.SchoolID, h.today())
	if err != nil {
		return nil, err
	}
	if weeks == nil {
		weeks = []enrollment.HostedHolidayWeek{}
	}
	return weeks, nil
}

// HolidayParticipants returns the participant list and records the read.
func (h *HolidayHostParticipants) HolidayParticipants(ctx context.Context, host enrollment.HolidayHost, weekID int64) (enrollment.HolidayParticipantList, error) {
	return h.read(ctx, host, weekID, "")
}

// ExportHolidayParticipants returns the participant list and records the
// export.
func (h *HolidayHostParticipants) ExportHolidayParticipants(ctx context.Context, host enrollment.HolidayHost, weekID int64, format string) (enrollment.HolidayParticipantList, error) {
	if format == "" {
		return enrollment.HolidayParticipantList{}, errors.New("enrollment: the export format is required")
	}
	return h.read(ctx, host, weekID, format)
}

func (h *HolidayHostParticipants) read(ctx context.Context, host enrollment.HolidayHost, weekID int64, format string) (enrollment.HolidayParticipantList, error) {
	if host.SchoolID <= 0 || host.AccountID <= 0 {
		return enrollment.HolidayParticipantList{}, errors.New("enrollment: the hosting school and the account are required")
	}
	if weekID <= 0 {
		return enrollment.HolidayParticipantList{}, enrollment.ErrHostedHolidayWeekNotFound
	}
	week, models, participants, found, err := h.store.ReadHostedHolidayWeek(ctx, host.SchoolID, weekID, h.today())
	if err != nil {
		return enrollment.HolidayParticipantList{}, err
	}
	if !found {
		return enrollment.HolidayParticipantList{}, enrollment.ErrHostedHolidayWeekNotFound
	}
	list := holidayParticipantList(week, models, participants)
	// The read is recorded once the week is known to be hosted and open;
	// the entry commits with the answer in the same transaction.
	if err := h.log.RecordHolidayParticipantAccess(ctx, enrollment.HolidayParticipantAccess{
		ActorAccountID: host.AccountID, ActorRole: host.Role, WeekID: week.ID, StartsOn: week.StartsOn, EndsOn: week.EndsOn,
		Participants: len(list.Participants), Format: format,
	}); err != nil {
		return enrollment.HolidayParticipantList{}, fmt.Errorf("record holiday participant access: %w", err)
	}
	return list, nil
}

// holidayParticipantList counts the children per time model, in the order
// of the week's time models, and per home school, the holiday guests last,
// and names the shared questions of its children.
func holidayParticipantList(week enrollment.HostedHolidayWeek, models []enrollment.HolidayTimeModelCount, participants []enrollment.HolidayParticipant) enrollment.HolidayParticipantList {
	if participants == nil {
		participants = []enrollment.HolidayParticipant{}
	}
	for i := range participants {
		if participants[i].Answers == nil {
			participants[i].Answers = []enrollment.HolidayHostAnswer{}
		}
	}
	week.Participants = len(participants)
	perModel := map[int64]int{}
	perSchool := map[int64]int{}
	guests := 0
	for _, participant := range participants {
		perModel[participant.TimeModelID]++
		if participant.HomeSchoolID == nil {
			guests++
			continue
		}
		perSchool[*participant.HomeSchoolID]++
	}
	counted := make([]enrollment.HolidayTimeModelCount, len(models))
	for i, model := range models {
		model.Count = perModel[model.ID]
		counted[i] = model
	}
	schools := make([]enrollment.HolidayHomeSchoolCount, 0, len(perSchool)+1)
	for id, count := range perSchool {
		schools = append(schools, enrollment.HolidayHomeSchoolCount{SchoolID: &id, Count: count})
	}
	slices.SortFunc(schools, func(a, b enrollment.HolidayHomeSchoolCount) int {
		switch {
		case *a.SchoolID < *b.SchoolID:
			return -1
		case *a.SchoolID > *b.SchoolID:
			return 1
		}
		return 0
	})
	if guests > 0 {
		schools = append(schools, enrollment.HolidayHomeSchoolCount{Count: guests})
	}
	return enrollment.HolidayParticipantList{
		Week: week, TimeModels: counted, HomeSchools: schools, Questions: hostQuestions(participants), Participants: participants,
	}
}

// hostQuestions lists each shared question once, in the order the children
// bring them; a later form version adds its new questions after the
// earlier ones, and the first label of a key names its column.
func hostQuestions(participants []enrollment.HolidayParticipant) []enrollment.HolidayHostQuestion {
	questions := []enrollment.HolidayHostQuestion{}
	seen := map[string]bool{}
	for _, participant := range participants {
		for _, answer := range participant.Answers {
			if seen[answer.Key] {
				continue
			}
			seen[answer.Key] = true
			questions = append(questions, enrollment.HolidayHostQuestion{Key: answer.Key, Label: answer.Label, Type: answer.Type})
		}
	}
	return questions
}
