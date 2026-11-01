package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Participant lists of the hosting school (#4665, ADR 0116). Every statement
// runs in the tenant transaction of the hosting school. The host policies
// admit only the school's weeks of published periods until their last day
// and only bookings of confirmed registrations; the children come only
// through enrollment.holiday_host_participants with their minimum fields.
// Each statement names the school, the end of the week by the service's
// calendar day and the confirmed booking once more.

// HolidayHostStore reads the hosted weeks in the hosting school's tenant
// transaction.
type HolidayHostStore struct{}

// NewHolidayHostStore builds the store.
func NewHolidayHostStore() *HolidayHostStore { return &HolidayHostStore{} }

// hostDB returns the tenant transaction of schoolID. An administrative,
// Träger or foreign school transaction is refused.
func hostDB(ctx context.Context, schoolID int64) (bun.IDB, error) {
	if schoolID <= 0 || tenant.IsAdminTx(ctx) || tenant.TraegerTransactionFromContext(ctx) != 0 {
		return nil, errors.New("enrollment postgres: the tenant transaction of the hosting school is required")
	}
	tx, current, err := tenant.CurrentTenantTransaction[bun.Tx](ctx)
	if err != nil {
		return nil, fmt.Errorf("enrollment postgres: the tenant transaction of the hosting school is required: %w", err)
	}
	if current != schoolID {
		return nil, errors.New("enrollment postgres: the tenant transaction belongs to another school")
	}
	return tx, nil
}

type hostedWeekRow struct {
	ID             int64         `bun:"id"`
	OrganizationID int64         `bun:"organization_id"`
	PeriodName     string        `bun:"period_name"`
	StartsOn       calendar.Date `bun:"starts_on"`
	EndsOn         calendar.Date `bun:"ends_on"`
	Participants   int           `bun:"participants"`
	GuestGroupID   *int64        `bun:"guest_group_id"`
	Guests         int           `bun:"guests"`
}

type hostedTimeModelRow struct {
	ID       int64  `bun:"id"`
	Name     string `bun:"name"`
	StartsAt string `bun:"starts_at"`
	EndsAt   string `bun:"ends_at"`
}

type hostedParticipantRow struct {
	ChildID             int64           `bun:"child_id"`
	FirstName           string          `bun:"first_name"`
	LastName            string          `bun:"last_name"`
	BirthDate           calendar.Date   `bun:"birth_date"`
	HomeSchoolID        *int64          `bun:"home_school_id"`
	GuestSchoolName     string          `bun:"guest_school_name"`
	TimeModelID         int64           `bun:"time_model_id"`
	SecondGuardianName  *string         `bun:"second_guardian_name"`
	SecondGuardianPhone *string         `bun:"second_guardian_phone"`
	Answers             json.RawMessage `bun:"answers,type:jsonb"`
	hostedHealthColumns
}

// hostedHealthColumns are the sealed health information of a participant
// as enrollment.holiday_host_participants returns it (#4722); all NULL
// without the parents' consent of version 2.
type hostedHealthColumns struct {
	RegistrationID   int64           `bun:"registration_id"`
	ChildPosition    int             `bun:"child_position"`
	HealthData       []byte          `bun:"health_data"`
	FamilyHealthData []byte          `bun:"family_health_data"`
	HealthFields     json.RawMessage `bun:"health_fields,type:jsonb"`
}

// sealed carries the sealed values to the service; nil when the function
// shares no health information for the child.
func (c hostedHealthColumns) sealed(organizationID int64) (*enrollment.HolidaySealedHostHealth, error) {
	if len(c.HealthFields) == 0 || string(c.HealthFields) == "null" {
		return nil, nil
	}
	var fields []enrollment.HolidayFormField
	if err := json.Unmarshal(c.HealthFields, &fields); err != nil {
		return nil, fmt.Errorf("enrollment postgres: decode shared holiday health questions: %w", err)
	}
	return &enrollment.HolidaySealedHostHealth{
		OrganizationID: organizationID, RegistrationID: c.RegistrationID, Position: c.ChildPosition,
		Child: c.HealthData, Family: c.FamilyHealthData, Fields: fields,
	}, nil
}

func (row hostedParticipantRow) participant(organizationID int64) (enrollment.HolidayParticipant, error) {
	answers, err := decodeHostAnswers(row.Answers)
	if err != nil {
		return enrollment.HolidayParticipant{}, err
	}
	sealed, err := row.sealed(organizationID)
	if err != nil {
		return enrollment.HolidayParticipant{}, err
	}
	participant := enrollment.HolidayParticipant{
		ChildID: row.ChildID, FirstName: row.FirstName, LastName: row.LastName, BirthDate: row.BirthDate,
		HomeSchoolID: row.HomeSchoolID, GuestSchoolName: row.GuestSchoolName, TimeModelID: row.TimeModelID, Answers: answers,
		Sealed: sealed,
	}
	if row.SecondGuardianName != nil {
		participant.SecondGuardian = &enrollment.HolidayHostGuardian{Name: *row.SecondGuardianName}
		if row.SecondGuardianPhone != nil {
			participant.SecondGuardian.Phone = *row.SecondGuardianPhone
		}
	}
	return participant, nil
}

// decodeHostAnswers reads the shared answers the database's narrow
// function built, or a guest copy stored; empty JSON reads as none.
func decodeHostAnswers(raw json.RawMessage) ([]enrollment.HolidayHostAnswer, error) {
	answers := []enrollment.HolidayHostAnswer{}
	if len(raw) == 0 {
		return answers, nil
	}
	if err := json.Unmarshal(raw, &answers); err != nil {
		return nil, fmt.Errorf("enrollment postgres: decode shared holiday answers: %w", err)
	}
	return answers, nil
}

const (
	hostedWeekColumns = `SELECT week.id, week.organization_id, period.name AS period_name, week.starts_on, week.ends_on,
			(SELECT COUNT(*) FROM enrollment.holiday_registration_bookings AS booking
				WHERE booking.organization_id = week.organization_id AND booking.holiday_period_week_id = week.id
					AND enrollment.holiday_host_booking(booking.organization_id, booking.holiday_period_week_id,
						booking.holiday_registration_child_id)) AS participants,
			host_group.group_id AS guest_group_id,
			(SELECT COUNT(*) FROM enrollment.holiday_guests AS guest
				WHERE guest.tenant_id = week.host_school_id AND guest.holiday_period_week_id = week.id
					AND guest.ended_on IS NULL) AS guests
		FROM enrollment.holiday_period_weeks AS week
		JOIN enrollment.holiday_periods AS period
			ON period.organization_id = week.organization_id AND period.id = week.holiday_period_id
		LEFT JOIN enrollment.holiday_host_groups AS host_group
			ON host_group.tenant_id = week.host_school_id AND host_group.holiday_period_week_id = week.id
		WHERE week.host_school_id = ? AND week.ends_on >= ? AND period.status = 'published'`
	listHostedWeeksQuery  = hostedWeekColumns + ` ORDER BY week.starts_on, week.id`
	findHostedWeekQuery   = hostedWeekColumns + ` AND week.id = ?`
	hostedTimeModelsQuery = `SELECT id, name, to_char(starts_at, 'HH24:MI') AS starts_at, to_char(ends_at, 'HH24:MI') AS ends_at
		FROM enrollment.holiday_week_time_models
		WHERE organization_id = ? AND holiday_period_week_id = ?
		ORDER BY starts_at, ends_at, id`
	// hostedParticipantsQuery reads the minimum fields of the confirmed
	// children, the second guardian, the shared answers and the sealed
	// health information through the database's own narrow function: the
	// school has no row access to the children, so no column beyond these
	// can leave, and health information leaves only sealed and only with
	// the parents' consent of version 2 (#5202, #4722).
	hostedParticipantsQuery = `SELECT child_id, first_name, last_name, birth_date, home_school_id, guest_school_name, time_model_id,
			second_guardian_name, second_guardian_phone, answers,
			registration_id, child_position, health_data, family_health_data, health_fields
		FROM enrollment.holiday_host_participants(?, ?)
		ORDER BY lower(last_name), lower(first_name), child_id`
)

// ListHostedHolidayWeeks lists the weeks of published periods the school
// hosts that end on today or later, with their confirmed children counted.
func (s *HolidayHostStore) ListHostedHolidayWeeks(ctx context.Context, schoolID int64, today calendar.Date) ([]enrollment.HostedHolidayWeek, error) {
	db, err := hostDB(ctx, schoolID)
	if err != nil {
		return nil, err
	}
	var rows []hostedWeekRow
	if err := db.NewRaw(listHostedWeeksQuery, schoolID, today).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("enrollment postgres: list hosted holiday weeks: %w", err)
	}
	weeks := make([]enrollment.HostedHolidayWeek, len(rows))
	for i, row := range rows {
		weeks[i] = hostedWeek(row)
	}
	return weeks, nil
}

// ReadHostedHolidayWeek reads one hosted week, its time models and its
// confirmed children: three statements for any number of children.
func (s *HolidayHostStore) ReadHostedHolidayWeek(ctx context.Context, schoolID, weekID int64, today calendar.Date) (enrollment.HostedHolidayWeek, []enrollment.HolidayTimeModelCount, []enrollment.HolidayParticipant, bool, error) {
	db, err := hostDB(ctx, schoolID)
	if err != nil {
		return enrollment.HostedHolidayWeek{}, nil, nil, false, err
	}
	var weeks []hostedWeekRow
	if err := db.NewRaw(findHostedWeekQuery, schoolID, today, weekID).Scan(ctx, &weeks); err != nil {
		return enrollment.HostedHolidayWeek{}, nil, nil, false, fmt.Errorf("enrollment postgres: read hosted holiday week: %w", err)
	}
	if len(weeks) == 0 {
		return enrollment.HostedHolidayWeek{}, nil, nil, false, nil
	}
	week := weeks[0]
	var models []hostedTimeModelRow
	if err := db.NewRaw(hostedTimeModelsQuery, week.OrganizationID, week.ID).Scan(ctx, &models); err != nil {
		return enrollment.HostedHolidayWeek{}, nil, nil, false, fmt.Errorf("enrollment postgres: read hosted holiday time models: %w", err)
	}
	var children []hostedParticipantRow
	if err := db.NewRaw(hostedParticipantsQuery, week.OrganizationID, week.ID).Scan(ctx, &children); err != nil {
		return enrollment.HostedHolidayWeek{}, nil, nil, false, fmt.Errorf("enrollment postgres: read holiday participants: %w", err)
	}
	counted := make([]enrollment.HolidayTimeModelCount, len(models))
	for i, model := range models {
		counted[i] = enrollment.HolidayTimeModelCount{ID: model.ID, Name: model.Name, StartsAt: model.StartsAt, EndsAt: model.EndsAt}
	}
	participants := make([]enrollment.HolidayParticipant, len(children))
	for i, child := range children {
		if participants[i], err = child.participant(week.OrganizationID); err != nil {
			return enrollment.HostedHolidayWeek{}, nil, nil, false, err
		}
	}
	return hostedWeek(week), counted, participants, true, nil
}

func hostedWeek(row hostedWeekRow) enrollment.HostedHolidayWeek {
	return enrollment.HostedHolidayWeek{
		ID: row.ID, PeriodName: row.PeriodName, StartsOn: row.StartsOn, EndsOn: row.EndsOn, Participants: row.Participants,
		GuestGroupID: row.GuestGroupID, Guests: row.Guests,
	}
}

// studentHealthQuery matches a child of the hosting school to the confirmed
// children of the weeks it hosts: the registration names the school as the
// home school and the same names and date of birth. Only rows whose parents
// agreed under consent texts of version 2 carry health information; the
// function decides that, the end of the week and the hosting school (#4722).
const studentHealthQuery = `SELECT week.id AS week_id, week.organization_id, period.name AS period_name,
		week.starts_on, week.ends_on, participant.registration_id, participant.child_position,
		participant.health_data, participant.family_health_data, participant.health_fields
	FROM enrollment.holiday_period_weeks AS week
	JOIN enrollment.holiday_periods AS period
		ON period.organization_id = week.organization_id AND period.id = week.holiday_period_id
	CROSS JOIN LATERAL enrollment.holiday_host_participants(week.organization_id, week.id) AS participant
	WHERE week.host_school_id = ? AND week.ends_on >= ? AND period.status = 'published'
		AND participant.home_school_id = ? AND participant.birth_date = ?
		AND lower(btrim(participant.first_name)) = lower(btrim(?))
		AND lower(btrim(participant.last_name)) = lower(btrim(?))
		AND participant.health_fields IS NOT NULL
	ORDER BY week.starts_on, week.id, participant.registration_id`

type studentHealthRow struct {
	WeekID         int64         `bun:"week_id"`
	OrganizationID int64         `bun:"organization_id"`
	PeriodName     string        `bun:"period_name"`
	StartsOn       calendar.Date `bun:"starts_on"`
	EndsOn         calendar.Date `bun:"ends_on"`
	hostedHealthColumns
}

// ReadStudentHealth reads the sealed health information the confirmed
// registrations give for a child of the school, per hosted week that ends
// on today or later: one statement.
func (s *HolidayHostStore) ReadStudentHealth(ctx context.Context, schoolID int64, student enrollment.HolidayStudentIdentity, today calendar.Date) ([]enrollment.HolidayStudentHealth, error) {
	db, err := hostDB(ctx, schoolID)
	if err != nil {
		return nil, err
	}
	var rows []studentHealthRow
	if err := db.NewRaw(studentHealthQuery, schoolID, today, schoolID, student.BirthDate, student.FirstName, student.LastName).
		Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("enrollment postgres: read holiday student health: %w", err)
	}
	entries := make([]enrollment.HolidayStudentHealth, 0, len(rows))
	for _, row := range rows {
		sealed, err := row.sealed(row.OrganizationID)
		if err != nil {
			return nil, err
		}
		entries = append(entries, enrollment.HolidayStudentHealth{
			WeekID: row.WeekID, PeriodName: row.PeriodName, StartsOn: row.StartsOn, EndsOn: row.EndsOn, Sealed: sealed,
		})
	}
	return entries, nil
}
