package compose

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Nachweise (#4808): kinds, statuses, missing mandatory kinds and the
// reminders, against the real tables.

type qualificationStaffFake struct {
	members  []workforce.QualificationStaffMember
	managers []workforce.QualificationStaffMember
}

func (f qualificationStaffFake) ListStaffMembers(context.Context) ([]workforce.QualificationStaffMember, error) {
	return f.members, nil
}

func (f qualificationStaffFake) StaffContact(_ context.Context, staffID int64) (workforce.QualificationStaffMember, error) {
	for _, member := range f.members {
		if member.StaffID == staffID {
			return member, nil
		}
	}
	return workforce.QualificationStaffMember{StaffID: staffID}, nil
}

func (f qualificationStaffFake) StammdatenManagers(context.Context) ([]workforce.QualificationStaffMember, error) {
	return f.managers, nil
}

type qualificationSchoolFake struct{}

func (qualificationSchoolFake) School(context.Context, int64) (workforce.PayrollSchool, error) {
	return workforce.PayrollSchool{Name: "OGS Am Park", Subdomain: "park"}, nil
}

type qualificationDeliveries struct {
	mu      sync.Mutex
	notices []workforce.QualificationNotice
	mails   []workforce.QualificationMailMessage
}

func (d *qualificationDeliveries) notify(_ context.Context, notice workforce.QualificationNotice) (bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.notices = append(d.notices, notice)
	return true, nil
}

func (d *qualificationDeliveries) Enqueue(_ context.Context, mail workforce.QualificationMailMessage) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.mails = append(d.mails, mail)
	return nil
}

func (d *qualificationDeliveries) counts() (int, int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.notices), len(d.mails)
}

// within runs fn in the tenant transaction a route or the Worker supplies.
func within[T any](t *testing.T, ctx context.Context, fn func(context.Context) (T, error)) (T, error) {
	t.Helper()
	var result T
	err := testpkg.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		var err error
		result, err = fn(txCtx)
		return err
	})
	return result, err
}

func runWithin(t *testing.T, ctx context.Context, fn func(context.Context) error) error {
	t.Helper()
	return testpkg.WithinCurrentTenant(ctx, fn)
}

func withinCreate(t *testing.T, ctx context.Context, types workforce.StaffQualificationTypes, input workforce.QualificationTypeInput) (workforce.QualificationType, error) {
	t.Helper()
	return within(t, ctx, func(c context.Context) (workforce.QualificationType, error) {
		return types.CreateQualificationType(c, input)
	})
}

func withinUpdate(t *testing.T, ctx context.Context, types workforce.StaffQualificationTypes, id int64, input workforce.QualificationTypeInput) (workforce.QualificationType, error) {
	t.Helper()
	return within(t, ctx, func(c context.Context) (workforce.QualificationType, error) {
		return types.UpdateQualificationType(c, id, input)
	})
}

func standardType(t *testing.T, types []workforce.QualificationType, key string) workforce.QualificationType {
	t.Helper()
	for _, kind := range types {
		if kind.StandardKey == key {
			return kind
		}
	}
	t.Fatalf("standard kind %s missing", key)
	return workforce.QualificationType{}
}

func TestQualificationTypesStartWithTheStandardListPerSchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	types, err := NewQualifications(db, nil, workforce.QualificationReminderPorts{}, nil)
	require.NoError(t, err)

	listed, err := within(t, ctx, types.ListQualificationTypes)
	require.NoError(t, err)
	require.Len(t, listed, 3)
	assert.Equal(t, "Erste Hilfe am Kind", listed[0].Name)
	assert.Equal(t, 8, listed[0].ReminderLeadWeeks)
	again, err := within(t, ctx, types.ListQualificationTypes)
	require.NoError(t, err)
	assert.Len(t, again, 3, "the standard list is added once")

	created, err := withinCreate(t, ctx, types, workforce.QualificationTypeInput{Name: "Hygieneschulung", ReminderLeadWeeks: 4})
	require.NoError(t, err)
	_, err = withinCreate(t, ctx, types, workforce.QualificationTypeInput{Name: "hygieneschulung", ReminderLeadWeeks: 4})
	require.ErrorIs(t, err, workforce.ErrQualificationTypeNameTaken)

	archived, err := withinUpdate(t, ctx, types, created.ID, workforce.QualificationTypeInput{Name: "Hygieneschulung", ReminderLeadWeeks: 4, Archived: true})
	require.NoError(t, err)
	assert.True(t, archived.Archived)

	foreignTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreignTenant)
	foreignCtx := testpkg.TenantContext(foreignTenant)
	_, err = withinUpdate(t, foreignCtx, types, created.ID, workforce.QualificationTypeInput{Name: "Übernommen", ReminderLeadWeeks: 4})
	require.ErrorIs(t, err, workforce.ErrQualificationTypeNotFound, "a kind of another school is invisible")
	foreignList, err := within(t, foreignCtx, types.ListQualificationTypes)
	require.NoError(t, err)
	assert.Len(t, foreignList, 3, "the other school sees only its own standard list")
}

func TestQualificationOverviewShowsExpiredAndMissingMandatoryProofs(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Nachweis", "Person")
	capability := buildWorkforce(t, db)
	types, err := NewQualifications(db, nil, workforce.QualificationReminderPorts{}, nil)
	require.NoError(t, err)
	listed, err := within(t, ctx, types.ListQualificationTypes)
	require.NoError(t, err)
	firstAid := standardType(t, listed, "first_aid_child")
	certificate := standardType(t, listed, "extended_police_certificate")
	_, err = withinUpdate(t, ctx, types, certificate.ID, workforce.QualificationTypeInput{
		Name: certificate.Name, ValidityMonths: certificate.ValidityMonths, ReminderLeadWeeks: 8, Mandatory: true,
	})
	require.NoError(t, err)

	_, err = capability.ReplaceStaffQualifications(ctx, staff.ID, []workforce.StaffQualification{
		{TypeID: &firstAid.ID, AcquiredOn: "2018-01-10", ExpiresOn: "2020-01-10"},
		{Name: "Schwimmschein", ExpiresOn: "2099-06-30"},
	})
	require.NoError(t, err)

	overview, err := NewQualifications(db, qualificationStaffFake{members: []workforce.QualificationStaffMember{
		{StaffID: staff.ID, FirstName: "Nachweis", LastName: "Person"},
	}}, workforce.QualificationReminderPorts{}, nil)
	require.NoError(t, err)
	rows, err := within(t, ctx, overview.QualificationOverview)
	require.NoError(t, err)
	require.Len(t, rows, 3)
	assert.Equal(t, "Erste Hilfe am Kind", rows[0].Name, "a row of a kind shows the kind's name")
	assert.Equal(t, workforce.QualificationStatusExpired, rows[0].Status)
	assert.Equal(t, "Schwimmschein", rows[1].Name)
	assert.Equal(t, workforce.QualificationStatusValid, rows[1].Status)
	assert.Equal(t, "Erweitertes Führungszeugnis", rows[2].Name)
	assert.Equal(t, workforce.QualificationStatusMissing, rows[2].Status)
	assert.True(t, rows[2].Mandatory)
	assert.Nil(t, rows[2].QualificationID)

	own, err := within(t, ctx, func(c context.Context) ([]workforce.QualificationEntry, error) {
		return overview.StaffQualificationStatus(c, staff.ID)
	})
	require.NoError(t, err)
	assert.Len(t, own, 3)
}

func TestQualificationAssignmentKeepsTheFreeTextRow(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Zuordnung", "Person")
	capability := buildWorkforce(t, db)
	types, err := NewQualifications(db, nil, workforce.QualificationReminderPorts{}, nil)
	require.NoError(t, err)
	listed, err := within(t, ctx, types.ListQualificationTypes)
	require.NoError(t, err)
	firstAid := standardType(t, listed, "first_aid_child")

	stored, err := capability.ReplaceStaffQualifications(ctx, staff.ID, []workforce.StaffQualification{{Name: "Erste Hilfe am Kind", ExpiresOn: "2027-03-01"}})
	require.NoError(t, err)
	require.Len(t, stored, 1)
	assert.Nil(t, stored[0].TypeID, "an equal free-text name is not reinterpreted")

	assigned, err := capability.ReplaceStaffQualifications(ctx, staff.ID, []workforce.StaffQualification{{ID: stored[0].ID, TypeID: &firstAid.ID, ExpiresOn: "2027-03-01"}})
	require.NoError(t, err)
	require.Len(t, assigned, 1)
	assert.Equal(t, stored[0].ID, assigned[0].ID, "assigning a kind keeps the row")
	require.NotNil(t, assigned[0].TypeID)
	assert.Equal(t, firstAid.ID, *assigned[0].TypeID)

	foreignTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreignTenant)
	foreignTypes, err := within(t, testpkg.TenantContext(foreignTenant), types.ListQualificationTypes)
	require.NoError(t, err)
	_, err = capability.ReplaceStaffQualifications(ctx, staff.ID, []workforce.StaffQualification{{TypeID: &foreignTypes[0].ID}})
	require.Error(t, err, "a kind of another school cannot be assigned")
}

func TestQualificationRemindersGoOutOnceOnTheLeadDay(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Erinnerung", "Person")
	capability := buildWorkforce(t, db)
	types, err := NewQualifications(db, nil, workforce.QualificationReminderPorts{}, nil)
	require.NoError(t, err)
	listed, err := within(t, ctx, types.ListQualificationTypes)
	require.NoError(t, err)
	firstAid := standardType(t, listed, "first_aid_child")
	_, err = capability.ReplaceStaffQualifications(ctx, staff.ID, []workforce.StaffQualification{
		{TypeID: &firstAid.ID, ExpiresOn: "2026-12-28"}, // the eight-week lead starts on 2026-11-02
	})
	require.NoError(t, err)

	deliveries := &qualificationDeliveries{}
	today := calendar.NewDate(2026, 11, 1)
	directory := qualificationStaffFake{
		members:  []workforce.QualificationStaffMember{{StaffID: staff.ID, FirstName: "Erinnerung", LastName: "Person", AccountID: 901, Email: "person@example.com"}},
		managers: []workforce.QualificationStaffMember{{StaffID: 1, FirstName: "Lea", LastName: "Leitung", AccountID: 900, Email: "leitung@example.com"}},
	}
	reminders, err := NewQualifications(db, directory, workforce.QualificationReminderPorts{
		Notify: deliveries.notify, Mail: deliveries, Schools: qualificationSchoolFake{},
		FrontendURL: "https://moto.example.com",
	}, func() time.Time { return today.BerlinMidnight().Add(12 * time.Hour) })
	require.NoError(t, err)

	require.NoError(t, runWithin(t, ctx, reminders.RunDue))
	notices, mails := deliveries.counts()
	assert.Zero(t, notices, "nothing the day before the lead starts")
	assert.Zero(t, mails)

	today = calendar.NewDate(2026, 11, 2)
	require.NoError(t, runWithin(t, ctx, reminders.RunDue))
	notices, mails = deliveries.counts()
	require.Equal(t, 2, notices, "the Leitung and the person, once each")
	assert.Equal(t, 2, mails)
	assert.Equal(t, []int64{900}, deliveries.notices[0].RecipientAccountIDs)
	assert.NotContains(t, deliveries.notices[0].Body, "Erinnerung", "the Leitung's push names no person")
	assert.True(t, deliveries.notices[1].Own)
	assert.Equal(t, []int64{901}, deliveries.notices[1].RecipientAccountIDs)
	assert.Equal(t, "https://park.moto.example.com/staff/"+strconv.FormatInt(staff.ID, 10), deliveries.mails[0].Content.LinkURL)

	require.NoError(t, runWithin(t, ctx, reminders.RunDue))
	today = calendar.NewDate(2026, 11, 3)
	require.NoError(t, runWithin(t, ctx, reminders.RunDue))
	notices, mails = deliveries.counts()
	assert.Equal(t, 2, notices, "no second lead reminder")
	assert.Equal(t, 2, mails)

	today = calendar.NewDate(2026, 12, 28)
	require.NoError(t, runWithin(t, ctx, reminders.RunDue))
	notices, _ = deliveries.counts()
	assert.Equal(t, 4, notices, "the expiry day reminds once more")
	assert.Contains(t, deliveries.notices[2].Title, "heute")

	foreignTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreignTenant)
	require.NoError(t, runWithin(t, testpkg.TenantContext(foreignTenant), reminders.RunDue))
	notices, _ = deliveries.counts()
	assert.Equal(t, 4, notices, "another school's run sees none of these rows")
}

func TestQualificationReminderCaughtUpAfterExpirySaysLapsed(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Nachgeholt", "Person")
	capability := buildWorkforce(t, db)
	_, err := capability.ReplaceStaffQualifications(ctx, staff.ID, []workforce.StaffQualification{
		{Name: "Hygieneschulung", ExpiresOn: "2026-11-02"},
	})
	require.NoError(t, err)

	deliveries := &qualificationDeliveries{}
	reminders, err := NewQualifications(db, qualificationStaffFake{
		members:  []workforce.QualificationStaffMember{{StaffID: staff.ID, FirstName: "Nachgeholt", LastName: "Person", AccountID: 902, Email: "person@example.com"}},
		managers: []workforce.QualificationStaffMember{{StaffID: 1, FirstName: "Lea", LastName: "Leitung", AccountID: 900, Email: "leitung@example.com"}},
	}, workforce.QualificationReminderPorts{
		Notify: deliveries.notify, Mail: deliveries, Schools: qualificationSchoolFake{},
		FrontendURL: "https://moto.example.com",
	}, func() time.Time { return calendar.NewDate(2026, 11, 5).BerlinMidnight().Add(12 * time.Hour) })
	require.NoError(t, err)

	require.NoError(t, runWithin(t, ctx, reminders.RunDue))
	require.Len(t, deliveries.notices, 2)
	assert.Equal(t, "Nachweis ist abgelaufen", deliveries.notices[0].Title, "a caught-up reminder does not say 'heute'")
	assert.Equal(t, "Nachweis ist abgelaufen", deliveries.mails[0].Subject)
	assert.True(t, deliveries.mails[0].Content.Lapsed)
}
