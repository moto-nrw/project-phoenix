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
	// ReadStudentHealth reads, per hosted week ending on today or later,
	// the sealed health information of the confirmed child whose
	// registration names the school as its home school and the child's
	// names and date of birth (#4722).
	ReadStudentHealth(ctx context.Context, schoolID int64, student enrollment.HolidayStudentIdentity, today calendar.Date) ([]enrollment.HolidayStudentHealth, error)
}

// HolidayHostParticipants serves enrollment.HolidayHostParticipants.
type HolidayHostParticipants struct {
	store    HolidayHostStore
	log      enrollment.HolidayParticipantAccessLog
	health   enrollment.HolidayHealthCipher
	students enrollment.HolidayStudentIdentities
	today    func() calendar.Date
}

var _ enrollment.HolidayHostParticipants = (*HolidayHostParticipants)(nil)

// NewHolidayHostParticipants builds the participant lists; health opens the
// health information of the children and students names the school's own
// children (#4722). A nil today uses the Berlin calendar day.
func NewHolidayHostParticipants(store HolidayHostStore, log enrollment.HolidayParticipantAccessLog, health enrollment.HolidayHealthCipher, students enrollment.HolidayStudentIdentities, today func() calendar.Date) *HolidayHostParticipants {
	if store == nil || log == nil || students == nil {
		panic("enrollment: the holiday host store, the access log and the student identities are required")
	}
	if health == nil {
		panic(errHolidayHealthCipherRequired)
	}
	if today == nil {
		today = calendar.TodayDate
	}
	return &HolidayHostParticipants{store: store, log: log, health: health, students: students, today: today}
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
	healthParticipants := 0
	for i := range participants {
		// An export carries no health column, and an account without the
		// right to read health information gets none (#4722).
		if format == "" && host.ReadsHealth {
			if participants[i].Health, err = openHolidayHostHealth(h.health, participants[i].Sealed); err != nil {
				return enrollment.HolidayParticipantList{}, err
			}
		}
		participants[i].Sealed = nil
		if participants[i].Health != nil {
			healthParticipants++
		}
	}
	list := holidayParticipantList(week, models, participants)
	// The read is recorded once the week is known to be hosted and open;
	// the entry commits with the answer in the same transaction.
	if err := h.log.RecordHolidayParticipantAccess(ctx, enrollment.HolidayParticipantAccess{
		ActorAccountID: host.AccountID, ActorRole: host.Role, WeekID: week.ID, StartsOn: week.StartsOn, EndsOn: week.EndsOn,
		Participants: len(list.Participants), HealthParticipants: healthParticipants, Format: format,
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

// HolidayStudentHealth returns the health information the confirmed
// registrations give for a child of the hosting school, per hosted week
// until its last day, and records the read in the same transaction when
// there is any. The school's own health field stays as it is.
func (h *HolidayHostParticipants) HolidayStudentHealth(ctx context.Context, host enrollment.HolidayHost, studentID int64) ([]enrollment.HolidayStudentHealth, error) {
	if host.SchoolID <= 0 || host.AccountID <= 0 {
		return nil, errors.New("enrollment: the hosting school and the account are required")
	}
	if studentID <= 0 {
		return nil, enrollment.ErrHolidayStudentNotFound
	}
	student, err := h.students.HolidayStudentIdentity(ctx, studentID)
	if err != nil {
		return nil, err
	}
	if student.BirthDate == "" {
		// A child without a date of birth matches no registration.
		return []enrollment.HolidayStudentHealth{}, nil
	}
	found, err := h.store.ReadStudentHealth(ctx, host.SchoolID, student, h.today())
	if err != nil {
		return nil, err
	}
	entries, err := h.openStudentHealth(found)
	if err != nil || len(entries) == 0 {
		return entries, err
	}
	if err := h.log.RecordHolidayStudentHealthAccess(ctx, holidayStudentHealthAccess(host, studentID, entries)); err != nil {
		return nil, fmt.Errorf("record holiday student health access: %w", err)
	}
	return entries, nil
}

// openStudentHealth opens the sealed entries and keeps those with health
// information.
func (h *HolidayHostParticipants) openStudentHealth(found []enrollment.HolidayStudentHealth) ([]enrollment.HolidayStudentHealth, error) {
	entries := make([]enrollment.HolidayStudentHealth, 0, len(found))
	for _, entry := range found {
		health, err := openHolidayHostHealth(h.health, entry.Sealed)
		if err != nil {
			return nil, err
		}
		entry.Sealed = nil
		if health != nil {
			entry.Health = *health
			entries = append(entries, entry)
		}
	}
	return entries, nil
}

// holidayStudentHealthAccess names the weeks and the span the read covered.
func holidayStudentHealthAccess(host enrollment.HolidayHost, studentID int64, entries []enrollment.HolidayStudentHealth) enrollment.HolidayStudentHealthAccess {
	access := enrollment.HolidayStudentHealthAccess{ActorAccountID: host.AccountID, ActorRole: host.Role, StudentID: studentID}
	for _, entry := range entries {
		access.WeekIDs = append(access.WeekIDs, entry.WeekID)
		if access.StartsOn == "" || entry.StartsOn.Before(access.StartsOn) {
			access.StartsOn = entry.StartsOn
		}
		if access.EndsOn == "" || entry.EndsOn.After(access.EndsOn) {
			access.EndsOn = entry.EndsOn
		}
	}
	return access
}
