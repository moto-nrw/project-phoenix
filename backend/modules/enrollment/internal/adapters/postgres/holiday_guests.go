package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Holiday guests of the hosting school (#4666, ADR 0120). Every statement
// runs in the tenant transaction of the hosting school (hostDB). The guest
// tables are the school's own; the confirmed bookings and their contacts come
// from the Träger's tables only through the host policies and the narrow
// functions enrollment.holiday_host_participants and
// enrollment.holiday_host_guest_contacts, which answer until the last day of
// a week the school hosts. Each statement names the school once more.

// HolidayGuestStore reads and writes the holiday guests of the hosting
// school.
type HolidayGuestStore struct{}

// NewHolidayGuestStore builds the store.
func NewHolidayGuestStore() *HolidayGuestStore { return &HolidayGuestStore{} }

type holidayHostGroupRow struct {
	OrganizationID      int64 `bun:"organization_id"`
	WeekID              int64 `bun:"holiday_period_week_id"`
	GroupID             int64 `bun:"group_id"`
	AssignedByAccountID int64 `bun:"assigned_by_account_id"`
}

type holidayGuestWeekRow struct {
	ID             int64         `bun:"id"`
	OrganizationID int64         `bun:"organization_id"`
	StartsOn       calendar.Date `bun:"starts_on"`
	EndsOn         calendar.Date `bun:"ends_on"`
}

type holidayGuestRow struct {
	bun.BaseModel   `bun:"table:enrollment.holiday_guests"`
	ID              int64          `bun:"id"`
	OrganizationID  int64          `bun:"organization_id"`
	WeekID          int64          `bun:"holiday_period_week_id"`
	ChildID         int64          `bun:"holiday_registration_child_id"`
	StudentID       int64          `bun:"student_id"`
	StartsOn        calendar.Date  `bun:"starts_on"`
	EndsOn          calendar.Date  `bun:"ends_on"`
	EndedOn         *calendar.Date `bun:"ended_on"`
	HomeSchoolID    *int64         `bun:"home_school_id"`
	GuestSchoolName string         `bun:"guest_school_name"`
	TimeModelName   string         `bun:"time_model_name"`
	TimeModelStarts string         `bun:"time_model_starts_at"`
	TimeModelEnds   string         `bun:"time_model_ends_at"`
}

func (r *holidayGuestRow) guest() enrollment.HolidayGuest {
	return enrollment.HolidayGuest{
		ID: r.ID, OrganizationID: r.OrganizationID, WeekID: r.WeekID, ChildID: r.ChildID, StudentID: r.StudentID,
		StartsOn: r.StartsOn, EndsOn: r.EndsOn, EndedOn: r.EndedOn, HomeSchoolID: r.HomeSchoolID,
		GuestSchoolName: r.GuestSchoolName, TimeModelName: r.TimeModelName, TimeModelStarts: r.TimeModelStarts,
		TimeModelEnds: r.TimeModelEnds,
	}
}

type holidayGuestContactRow struct {
	GuestID      int64  `bun:"holiday_guest_id"`
	ChildID      int64  `bun:"child_id"`
	Kind         string `bun:"kind"`
	Name         string `bun:"name"`
	Phone        string `bun:"phone"`
	Relationship string `bun:"relationship"`
}

const (
	holidayGuestColumns = `id, organization_id, holiday_period_week_id, holiday_registration_child_id, student_id,
		starts_on, ends_on, ended_on, home_school_id, guest_school_name, time_model_name, time_model_starts_at, time_model_ends_at`
	guestWeekQuery = `SELECT week.id, week.organization_id, week.starts_on, week.ends_on
		FROM enrollment.holiday_period_weeks AS week
		JOIN enrollment.holiday_periods AS period
			ON period.organization_id = week.organization_id AND period.id = week.holiday_period_id
		WHERE week.host_school_id = ? AND week.id = ? AND week.ends_on >= ? AND period.status = 'published'`
	// guestSourceQuery reads the minimum fields and the booked time model of
	// the confirmed children of the week who are no students of the school.
	guestSourceQuery = `SELECT participant.child_id, participant.first_name, participant.last_name, participant.birth_date,
			participant.home_school_id, participant.guest_school_name, model.name AS time_model_name,
			to_char(model.starts_at, 'HH24:MI') AS time_model_starts, to_char(model.ends_at, 'HH24:MI') AS time_model_ends
		FROM enrollment.holiday_host_participants(?, ?) AS participant
		JOIN enrollment.holiday_week_time_models AS model
			ON model.organization_id = ? AND model.holiday_period_week_id = ? AND model.id = participant.time_model_id
		WHERE participant.home_school_id IS DISTINCT FROM ?
		ORDER BY participant.child_id`
	guestSourceContactsQuery = `SELECT child_id, kind, name, phone, relationship
		FROM enrollment.holiday_host_guest_contacts(?, ?)
		ORDER BY child_id, kind, position`
	guestContactsQuery = `SELECT holiday_guest_id, kind, name, phone, relationship
		FROM enrollment.holiday_guest_contacts
		WHERE tenant_id = ? AND holiday_guest_id IN (?)
		ORDER BY holiday_guest_id, kind, position`
)

type guestSourceRow struct {
	ChildID         int64         `bun:"child_id"`
	FirstName       string        `bun:"first_name"`
	LastName        string        `bun:"last_name"`
	BirthDate       calendar.Date `bun:"birth_date"`
	HomeSchoolID    *int64        `bun:"home_school_id"`
	GuestSchoolName string        `bun:"guest_school_name"`
	TimeModelName   string        `bun:"time_model_name"`
	TimeModelStarts string        `bun:"time_model_starts"`
	TimeModelEnds   string        `bun:"time_model_ends"`
}

// LockGuests takes the transaction lock of the school's holiday guests.
func (s *HolidayGuestStore) LockGuests(ctx context.Context, schoolID int64) error {
	db, err := hostDB(ctx, schoolID)
	if err != nil {
		return err
	}
	if _, err := db.NewRaw(`SELECT pg_advisory_xact_lock(hashtextextended('enrollment.holiday_guests:' || ?::text, 0))`,
		schoolID).Exec(ctx); err != nil {
		return fmt.Errorf("enrollment postgres: lock holiday guests: %w", err)
	}
	return nil
}

// SetHostGroup stores the group the school picked for the guests of a
// week; a second choice replaces the first.
func (s *HolidayGuestStore) SetHostGroup(ctx context.Context, schoolID int64, group enrollment.HolidayHostGroup) error {
	db, err := hostDB(ctx, schoolID)
	if err != nil {
		return err
	}
	if _, err := db.NewRaw(`INSERT INTO enrollment.holiday_host_groups
			(tenant_id, organization_id, holiday_period_week_id, group_id, assigned_by_account_id)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (tenant_id, holiday_period_week_id) DO UPDATE
			SET group_id = EXCLUDED.group_id, organization_id = EXCLUDED.organization_id,
				assigned_by_account_id = EXCLUDED.assigned_by_account_id, updated_at = CURRENT_TIMESTAMP`,
		schoolID, group.OrganizationID, group.WeekID, group.GroupID, group.AssignedByAccountID).Exec(ctx); err != nil {
		return hostGroupError(fmt.Errorf("enrollment postgres: set holiday host group: %w", err))
	}
	return nil
}

// hostGroupError names a group that is not one of the school's; the foreign
// key decides it, nothing is pre-checked.
func hostGroupError(err error) error {
	var postgresError pgdriver.Error
	if errors.As(err, &postgresError) && postgresError.IntegrityViolation() && postgresError.Field('n') == "fk_holiday_host_groups_group" {
		return fmt.Errorf("%w: %w", enrollment.ErrHolidayGuestGroupInvalid, err)
	}
	return err
}

// ListHostGroups lists the groups the school picked, by week.
func (s *HolidayGuestStore) ListHostGroups(ctx context.Context, schoolID int64) ([]enrollment.HolidayHostGroup, error) {
	db, err := hostDB(ctx, schoolID)
	if err != nil {
		return nil, err
	}
	var groups []holidayHostGroupRow
	if err := db.NewRaw(`SELECT organization_id, holiday_period_week_id, group_id, assigned_by_account_id
		FROM enrollment.holiday_host_groups WHERE tenant_id = ? ORDER BY holiday_period_week_id`, schoolID).Scan(ctx, &groups); err != nil {
		return nil, fmt.Errorf("enrollment postgres: list holiday host groups: %w", err)
	}
	result := make([]enrollment.HolidayHostGroup, len(groups))
	for i, group := range groups {
		result[i] = enrollment.HolidayHostGroup(group)
	}
	return result, nil
}

// ReadGuestWeek reads a week of a published period the school hosts that
// ends on today or later; found is false otherwise.
func (s *HolidayGuestStore) ReadGuestWeek(ctx context.Context, schoolID, weekID int64, today calendar.Date) (enrollment.HolidayGuestWeek, bool, error) {
	db, err := hostDB(ctx, schoolID)
	if err != nil {
		return enrollment.HolidayGuestWeek{}, false, err
	}
	var weeks []holidayGuestWeekRow
	if err := db.NewRaw(guestWeekQuery, schoolID, weekID, today).Scan(ctx, &weeks); err != nil {
		return enrollment.HolidayGuestWeek{}, false, fmt.Errorf("enrollment postgres: read holiday guest week: %w", err)
	}
	if len(weeks) == 0 {
		return enrollment.HolidayGuestWeek{}, false, nil
	}
	return enrollment.HolidayGuestWeek(weeks[0]), true, nil
}

// ListGuestSources reads the confirmed children of a hosted week who are no
// students of the school, with their time model and contacts: two
// statements for any number of children.
func (s *HolidayGuestStore) ListGuestSources(ctx context.Context, schoolID int64, week enrollment.HolidayGuestWeek) ([]enrollment.HolidayGuestSource, error) {
	db, err := hostDB(ctx, schoolID)
	if err != nil {
		return nil, err
	}
	var rows []guestSourceRow
	if err := db.NewRaw(guestSourceQuery, week.OrganizationID, week.ID, week.OrganizationID, week.ID, schoolID).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("enrollment postgres: read holiday guests of the week: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	var contacts []holidayGuestContactRow
	if err := db.NewRaw(guestSourceContactsQuery, week.OrganizationID, week.ID).Scan(ctx, &contacts); err != nil {
		return nil, fmt.Errorf("enrollment postgres: read holiday guest contacts of the week: %w", err)
	}
	byChild := make(map[int64]*enrollment.HolidayGuestSource, len(rows))
	sources := make([]enrollment.HolidayGuestSource, len(rows))
	for i, row := range rows {
		sources[i] = enrollment.HolidayGuestSource{
			ChildID: row.ChildID, FirstName: row.FirstName, LastName: row.LastName, BirthDate: row.BirthDate,
			HomeSchoolID: row.HomeSchoolID, GuestSchoolName: row.GuestSchoolName,
			TimeModelName: row.TimeModelName, TimeModelStarts: row.TimeModelStarts, TimeModelEnds: row.TimeModelEnds,
		}
		byChild[row.ChildID] = &sources[i]
	}
	for _, contact := range contacts {
		if source, ok := byChild[contact.ChildID]; ok {
			appendContact(&source.Emergency, &source.Pickup, contact)
		}
	}
	return sources, nil
}

func appendContact(emergency, pickup *[]enrollment.HolidayGuestContact, row holidayGuestContactRow) {
	contact := enrollment.HolidayGuestContact{Name: row.Name, Phone: row.Phone, Relationship: row.Relationship}
	if row.Kind == "pickup" {
		*pickup = append(*pickup, contact)
		return
	}
	*emergency = append(*emergency, contact)
}

// ListOpenGuests lists the guests of the school whose access has not ended
// early, of one week or, with weekID 0, of every week ending on from or
// later.
func (s *HolidayGuestStore) ListOpenGuests(ctx context.Context, schoolID, weekID int64, from calendar.Date) ([]enrollment.HolidayGuest, error) {
	db, err := hostDB(ctx, schoolID)
	if err != nil {
		return nil, err
	}
	var guests []holidayGuestRow
	query := db.NewSelect().TableExpr("enrollment.holiday_guests").ColumnExpr(holidayGuestColumns).
		Where("tenant_id = ?", schoolID).Where("ended_on IS NULL").Where("ends_on >= ?", from).
		OrderExpr("holiday_period_week_id, holiday_registration_child_id")
	if weekID > 0 {
		query = query.Where("holiday_period_week_id = ?", weekID)
	}
	if err := query.Scan(ctx, &guests); err != nil {
		return nil, fmt.Errorf("enrollment postgres: list holiday guests: %w", err)
	}
	result := make([]enrollment.HolidayGuest, len(guests))
	for i, guest := range guests {
		result[i] = guest.guest()
	}
	return result, nil
}

// InsertGuest stores a new guest with its contacts.
func (s *HolidayGuestStore) InsertGuest(ctx context.Context, schoolID int64, guest enrollment.HolidayGuest) error {
	db, err := hostDB(ctx, schoolID)
	if err != nil {
		return err
	}
	var id int64
	if err := db.NewRaw(`INSERT INTO enrollment.holiday_guests (tenant_id, organization_id, holiday_period_week_id,
			holiday_registration_child_id, student_id, starts_on, ends_on, home_school_id, guest_school_name,
			time_model_name, time_model_starts_at, time_model_ends_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		schoolID, guest.OrganizationID, guest.WeekID, guest.ChildID, guest.StudentID, guest.StartsOn, guest.EndsOn,
		guest.HomeSchoolID, guest.GuestSchoolName, guest.TimeModelName, guest.TimeModelStarts, guest.TimeModelEnds,
	).Scan(ctx, &id); err != nil {
		return fmt.Errorf("enrollment postgres: insert holiday guest: %w", err)
	}
	return insertGuestContacts(ctx, db, schoolID, id, guest)
}

// UpdateGuest replaces the booked care, the home school and the contacts of
// a guest.
func (s *HolidayGuestStore) UpdateGuest(ctx context.Context, schoolID int64, guest enrollment.HolidayGuest) error {
	db, err := hostDB(ctx, schoolID)
	if err != nil {
		return err
	}
	if _, err := db.NewRaw(`UPDATE enrollment.holiday_guests
		SET starts_on = ?, ends_on = ?, home_school_id = ?, guest_school_name = ?, time_model_name = ?,
			time_model_starts_at = ?, time_model_ends_at = ?, updated_at = CURRENT_TIMESTAMP
		WHERE tenant_id = ? AND id = ?`,
		guest.StartsOn, guest.EndsOn, guest.HomeSchoolID, guest.GuestSchoolName, guest.TimeModelName,
		guest.TimeModelStarts, guest.TimeModelEnds, schoolID, guest.ID).Exec(ctx); err != nil {
		return fmt.Errorf("enrollment postgres: update holiday guest: %w", err)
	}
	if _, err := db.NewRaw(`DELETE FROM enrollment.holiday_guest_contacts WHERE tenant_id = ? AND holiday_guest_id = ?`,
		schoolID, guest.ID).Exec(ctx); err != nil {
		return fmt.Errorf("enrollment postgres: replace holiday guest contacts: %w", err)
	}
	return insertGuestContacts(ctx, db, schoolID, guest.ID, guest)
}

func insertGuestContacts(ctx context.Context, db bun.IDB, schoolID, guestID int64, guest enrollment.HolidayGuest) error {
	type contactRow struct {
		bun.BaseModel `bun:"table:enrollment.holiday_guest_contacts"`
		TenantID      int64  `bun:"tenant_id"`
		GuestID       int64  `bun:"holiday_guest_id"`
		Kind          string `bun:"kind"`
		Position      int    `bun:"position"`
		Name          string `bun:"name"`
		Phone         string `bun:"phone"`
		Relationship  string `bun:"relationship"`
	}
	rows := make([]contactRow, 0, len(guest.Emergency)+len(guest.Pickup))
	for kind, contacts := range map[string][]enrollment.HolidayGuestContact{"emergency": guest.Emergency, "pickup": guest.Pickup} {
		for i, contact := range contacts {
			rows = append(rows, contactRow{TenantID: schoolID, GuestID: guestID, Kind: kind, Position: i + 1,
				Name: contact.Name, Phone: contact.Phone, Relationship: contact.Relationship})
		}
	}
	if len(rows) == 0 {
		return nil
	}
	if _, err := db.NewInsert().Model(&rows).Exec(ctx); err != nil {
		return fmt.Errorf("enrollment postgres: insert holiday guest contacts: %w", err)
	}
	return nil
}

// EndGuest records that the access to a guest ended early on endedOn and
// deletes its contacts: a cancelled guest keeps nothing beyond its name.
func (s *HolidayGuestStore) EndGuest(ctx context.Context, schoolID, guestID int64, endedOn calendar.Date) error {
	db, err := hostDB(ctx, schoolID)
	if err != nil {
		return err
	}
	if _, err := db.NewRaw(`UPDATE enrollment.holiday_guests SET ended_on = ?, updated_at = CURRENT_TIMESTAMP
		WHERE tenant_id = ? AND id = ?`, endedOn, schoolID, guestID).Exec(ctx); err != nil {
		return fmt.Errorf("enrollment postgres: end holiday guest: %w", err)
	}
	if _, err := db.NewRaw(`DELETE FROM enrollment.holiday_guest_contacts WHERE tenant_id = ? AND holiday_guest_id = ?`,
		schoolID, guestID).Exec(ctx); err != nil {
		return fmt.Errorf("enrollment postgres: delete contacts of an ended holiday guest: %w", err)
	}
	return nil
}

// PurgeContacts deletes the contacts of every guest whose week ended before
// lastReadableEnd: the read-only days after the week are over.
func (s *HolidayGuestStore) PurgeContacts(ctx context.Context, schoolID int64, lastReadableEnd calendar.Date) error {
	db, err := hostDB(ctx, schoolID)
	if err != nil {
		return err
	}
	if _, err := db.NewRaw(`DELETE FROM enrollment.holiday_guest_contacts AS contact
		USING enrollment.holiday_guests AS guest
		WHERE contact.tenant_id = ? AND guest.tenant_id = contact.tenant_id AND guest.id = contact.holiday_guest_id
			AND guest.ends_on < ?`, schoolID, lastReadableEnd).Exec(ctx); err != nil {
		return fmt.Errorf("enrollment postgres: delete expired holiday guest contacts: %w", err)
	}
	return nil
}

// ReadGuest reads the guest behind a child of the school with its
// contacts, while its access has not ended early and its week ended on
// lastReadableEnd or later; found is false otherwise.
func (s *HolidayGuestStore) ReadGuest(ctx context.Context, schoolID, studentID int64, lastReadableEnd calendar.Date) (enrollment.HolidayGuest, bool, error) {
	db, err := hostDB(ctx, schoolID)
	if err != nil {
		return enrollment.HolidayGuest{}, false, err
	}
	var guests []holidayGuestRow
	if err := db.NewSelect().TableExpr("enrollment.holiday_guests").ColumnExpr(holidayGuestColumns).
		Where("tenant_id = ?", schoolID).Where("student_id = ?", studentID).
		Where("ended_on IS NULL").Where("ends_on >= ?", lastReadableEnd).Scan(ctx, &guests); err != nil {
		return enrollment.HolidayGuest{}, false, fmt.Errorf("enrollment postgres: read holiday guest: %w", err)
	}
	if len(guests) == 0 {
		return enrollment.HolidayGuest{}, false, nil
	}
	guest := guests[0].guest()
	var contacts []holidayGuestContactRow
	if err := db.NewRaw(guestContactsQuery, schoolID, bun.List([]int64{guest.ID})).Scan(ctx, &contacts); err != nil {
		return enrollment.HolidayGuest{}, false, fmt.Errorf("enrollment postgres: read holiday guest contacts: %w", err)
	}
	for _, contact := range contacts {
		appendContact(&guest.Emergency, &guest.Pickup, contact)
	}
	return guest, true, nil
}
