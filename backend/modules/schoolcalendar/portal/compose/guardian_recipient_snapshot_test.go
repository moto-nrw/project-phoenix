package compose_test

import (
	"context"
	"slices"
	"testing"

	"github.com/moto-ogs/project-phoenix/modules/appointments"
	"github.com/moto-ogs/project-phoenix/roots/owners"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestGuardianNotificationAudiencesShareRecipientLinkSnapshot(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	cfg := calendarTestConfig(t, db)
	ctx, ids, family := recipientReadAppointments(t, db, cfg, 1)
	fired := false
	cfg.GuardianProfileRepo = recipientContactReader{GuardianDirectory: cfg.GuardianProfileRepo, after: func() {
		if !fired {
			fired = true
			_, err := db.NewRaw(`DELETE FROM calendar.appointment_recipient_students WHERE recipient_id IN
				(SELECT id FROM calendar.appointment_recipients WHERE appointment_id = ?)`, ids[0]).Exec(ctx)
			require.NoError(t, err)
		}
	}}
	service := owners.NewCalendarPortal(cfg)
	audiences, err := service.GuardianNotificationAudiences(ctx, ids)
	require.NoError(t, err)
	require.True(t, fired, "delete commits between the recipient and former link reads")
	require.Equal(t, []int64{family.StudentID}, audiences[ids[0]].StudentsByGuardian[family.GuardianProfileID], "E2: links come from the recipient snapshot")
	fresh, err := service.GuardianNotificationAudiences(ctx, ids)
	require.NoError(t, err)
	require.Empty(t, fresh[ids[0]].GuardianIDs, "the next resolution observes the committed deletion")
}

// Keep the real read and inject only a malformed recipient kind that the DB
// constraint normally excludes. The old recipientIDs filter excluded it too.
type recipientProjectionProbe struct {
	appointments.Capability
	rows     []*appointments.AppointmentRecipient
	students [][]int64
}

func (p *recipientProjectionProbe) FindAppointmentRecipientsByAppointmentIDs(ctx context.Context, ids []int64) ([]*appointments.AppointmentRecipient, error) {
	rows, err := p.Capability.FindAppointmentRecipientsByAppointmentIDs(ctx, ids)
	if err == nil && len(rows) > 0 {
		rows[0].RecipientType = appointments.RecipientTypeStaff
	}
	p.rows = rows
	p.students = nil
	for _, row := range rows {
		p.students = append(p.students, slices.Clone(row.StudentIDs))
	}
	return rows, err
}

func TestGuardianNotificationAudiencesIgnoreNonGuardianStudentProjection(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	cfg := calendarTestConfig(t, db)
	ctx, ids, family := recipientReadAppointments(t, db, cfg, 2)
	probe := &recipientProjectionProbe{Capability: cfg.Appointments}
	cfg.Appointments = probe
	audiences, err := owners.NewCalendarPortal(cfg).GuardianNotificationAudiences(ctx, ids)
	require.NoError(t, err)
	require.Empty(t, audiences[ids[0]].GuardianIDs, "non-guardian links must not be indexed, even with a profile ID")
	require.Equal(t, []int64{family.GuardianProfileID}, audiences[ids[1]].GuardianIDs)
	for i, row := range probe.rows {
		require.Equal(t, probe.students[i], row.StudentIDs, "the consumer must not mutate the supplied slices")
	}
	cfg.GuardianProfileRepo = recipientContactReader{GuardianDirectory: cfg.GuardianProfileRepo, after: func() {
		t.Fatal("empty and staff-only audiences must not read guardian contacts")
	}}
	service := owners.NewCalendarPortal(cfg)
	staffOnly, err := service.GuardianNotificationAudiences(ctx, ids[:1])
	require.NoError(t, err)
	require.Empty(t, staffOnly[ids[0]].GuardianIDs)
	empty, err := service.GuardianNotificationAudiences(ctx, nil)
	require.NoError(t, err)
	require.Empty(t, empty)
}
