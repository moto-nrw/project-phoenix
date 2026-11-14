package compose_test

import (
	"context"
	"testing"

	"github.com/moto-ogs/project-phoenix/modules/appointments"
	calendarCompose "github.com/moto-ogs/project-phoenix/modules/schoolcalendar/portal/compose"
	"github.com/moto-ogs/project-phoenix/roots/owners"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// The reference deliberately uses the retained, separate link reader. Compare
// the entire exported projection, not just the guardian count or ID set.
func legacyRecipientAudiences(t *testing.T, ctx context.Context, cfg calendarCompose.Config, ids []int64) map[int64]calendarCompose.GuardianNotificationAudience {
	t.Helper()
	rows, err := cfg.Appointments.FindAppointmentRecipientsByAppointmentIDs(ctx, ids)
	require.NoError(t, err)
	var guardians, recipients []int64
	for _, row := range rows {
		if row.RecipientType == appointments.RecipientTypeGuardianProfile && row.GuardianProfileID != nil {
			guardians = append(guardians, *row.GuardianProfileID)
			recipients = append(recipients, row.ID)
		}
	}
	contacts, err := cfg.GuardianProfileRepo.ListPortalContacts(ctx, guardians, nil)
	require.NoError(t, err)
	links, err := cfg.Appointments.FindAppointmentRecipientStudents(ctx, recipients)
	require.NoError(t, err)
	profiles := map[int64]*calendarCompose.GuardianNotificationProfile{}
	allowed := map[[2]int64]bool{}
	for _, contact := range contacts {
		p := contact.Profile
		profiles[p.ID] = &calendarCompose.GuardianNotificationProfile{
			ID: p.ID, AccountID: p.AccountID, Email: p.Email, FirstName: p.FirstName, LastName: p.LastName, PortalLocale: p.PortalLocale,
		}
		if contact.Relationship != nil && contact.Relationship.PortalAccess {
			allowed[[2]int64{p.ID, contact.Relationship.StudentID}] = true
		}
	}
	result := map[int64]calendarCompose.GuardianNotificationAudience{}
	for _, id := range ids {
		result[id] = calendarCompose.GuardianNotificationAudience{Profiles: profiles, StudentsByGuardian: map[int64][]int64{}, Answers: map[int64]calendarCompose.GuardianNotificationAnswer{}}
	}
	for _, row := range rows {
		if row.GuardianProfileID == nil {
			continue
		}
		guardian := *row.GuardianProfileID
		profile := profiles[guardian]
		if profile == nil || profile.AccountID == nil || *profile.AccountID <= 0 {
			continue
		}
		var children []int64
		for _, link := range links {
			if link.RecipientID == row.ID && allowed[[2]int64{guardian, link.StudentID}] {
				children = append(children, link.StudentID)
			}
		}
		if len(children) > 0 {
			audience := result[row.AppointmentID]
			audience.GuardianIDs = append(audience.GuardianIDs, guardian)
			audience.StudentsByGuardian[guardian] = append(audience.StudentsByGuardian[guardian], children...)
			audience.Answers[guardian] = calendarCompose.GuardianNotificationAnswer{Status: row.Status, OccurrenceResponses: row.OccurrenceResponses}
			result[row.AppointmentID] = audience
		}
	}
	return result
}

func TestGuardianNotificationAudiencesMatchLegacyLinks(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	cfg := calendarTestConfig(t, db)
	ctx, ids, family := recipientReadAppointments(t, db, cfg, 2)
	sibling := testpkg.CreateTestStudent(t, db, "Sibling", "Recipient", "1a")
	link := testpkg.CreateTestStudentGuardianLink(t, db, sibling.ID, family.GuardianProfileID, "primary_guardian")
	var recipientID int64
	require.NoError(t, db.NewRaw(`SELECT id FROM calendar.appointment_recipients WHERE appointment_id = ?`, ids[0]).Scan(ctx, &recipientID))
	// Insert in reverse order of student ID: both readers must sort, not retain insertion order.
	_, err := db.NewRaw(`DELETE FROM calendar.appointment_recipient_students WHERE recipient_id = ?`, recipientID).Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewRaw(`INSERT INTO calendar.appointment_recipient_students (tenant_id, recipient_id, student_id) VALUES (?, ?, ?), (?, ?, ?)`, testpkg.Tenant(t), recipientID, sibling.ID, testpkg.Tenant(t), recipientID, family.StudentID).Exec(ctx)
	require.NoError(t, err)
	foreignTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreignTenant)
	foreignStudent := testpkg.CreateTestStudentForTenant(t, db, foreignTenant, "Foreign", "Recipient", "1a")
	foreignStaff, _ := testpkg.CreateTestStaffWithAccountForTenant(t, db, foreignTenant, "Foreign", "Organizer")
	foreignGuardian := testpkg.CreateTestGuardianProfileForTenant(t, db, foreignTenant, "Foreign", "Guardian", "foreign@example.test")
	foreignCtx := testpkg.ContextForTenant(testpkg.ContextWithTenantRuntime(ctx, testpkg.TenantRuntime(t, db)), foreignTenant)
	foreignAppointment, _, err := cfg.Appointments.CreateAppointment(foreignCtx, appointments.CreateAppointment{AppointmentFields: appointments.AppointmentFields{
		OrganizerStaffID: foreignStaff.ID, Title: "Other school", StartDate: portalDate(2026, 10, 12), EndDate: portalDate(2026, 10, 12),
		StartTime: wallClock(14, 0), EndTime: wallClock(15, 0), DeliveryMode: appointments.DeliveryModeInformational,
	}})
	require.NoError(t, err)
	_, _, err = cfg.Appointments.CreateAppointmentRecipients(foreignCtx, foreignAppointment.ID, []appointments.AppointmentRecipientFields{{
		RecipientType: appointments.RecipientTypeGuardianProfile, GuardianProfileID: &foreignGuardian.ID,
		Status: appointments.ResponseStatusPending, StudentIDs: []int64{foreignStudent.ID},
	}})
	require.NoError(t, err)
	foreignID, foreignStudentID := foreignAppointment.ID, foreignStudent.ID
	ids = append(ids, foreignID)
	service := owners.NewCalendarPortal(cfg)
	check := func(want []int64) {
		t.Helper()
		require.NoError(t, testpkg.WithinTenantContext(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context) error {
			rows, err := cfg.Appointments.FindAppointmentRecipientsByAppointmentIDs(txCtx, ids)
			require.NoError(t, err)
			for _, row := range rows {
				links, err := cfg.Appointments.FindAppointmentRecipientStudents(txCtx, []int64{row.ID})
				require.NoError(t, err)
				var students []int64
				for _, value := range links {
					students = append(students, value.StudentID)
				}
				require.Equal(t, students, row.StudentIDs)
				require.NotContains(t, row.StudentIDs, foreignStudentID)
			}
			before := legacyRecipientAudiences(t, txCtx, cfg, ids)
			after, err := service.GuardianNotificationAudiences(txCtx, ids)
			require.NoError(t, err)
			require.Equal(t, before, after)
			require.Empty(t, after[foreignID].GuardianIDs)
			require.Equal(t, want, after[ids[0]].StudentsByGuardian[family.GuardianProfileID])
			return nil
		}))
	}
	check([]int64{family.StudentID, sibling.ID})
	deactivateAccountTenant(t, db, family.AccountID, testpkg.Tenant(t))
	check(nil)
	_, err = db.NewRaw(`UPDATE auth.account_tenants SET status = 'active', deactivated_at = NULL WHERE account_id = ? AND tenant_id = ?`, family.AccountID, testpkg.Tenant(t)).Exec(ctx)
	require.NoError(t, err)
	check([]int64{family.StudentID, sibling.ID})
	require.NoError(t, testpkg.SetTestStudentGuardianPermissions(ctx, db, link.ID, map[string]interface{}{"parent_portal.access": false}))
	check([]int64{family.StudentID})
	_, err = db.NewRaw(`DELETE FROM users.student_guardian_relationships WHERE id = ?`, link.ID).Exec(ctx)
	require.NoError(t, err)
	check([]int64{family.StudentID})
	_, err = db.NewRaw(`DELETE FROM calendar.appointment_recipient_students WHERE recipient_id = ?`, recipientID).Exec(ctx)
	require.NoError(t, err)
	check(nil)
	_, err = db.NewRaw(`UPDATE users.guardian_profiles SET account_id = NULL, has_account = false WHERE id = ?`, family.GuardianProfileID).Exec(ctx)
	require.NoError(t, err)
	check(nil)
}
