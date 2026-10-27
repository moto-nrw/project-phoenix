// The notice about children missing without a report (#4791) and the
// default-on decision it relies on, against fakes: only the Schulverwaltung
// is offered the type, it is on until switched off, and the notice names a
// number, never a child.
package notifications_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/notifications"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// In-memory identities; none of these are database rows.
const (
	unreportedTenant  int64 = 77
	unreportedOffice  int64 = 301
	unreportedOffice2 int64 = 302
	unreportedTeacher int64 = 303
)

type fakeSchoolOffice struct {
	ids []int64
	err error
}

func (f fakeSchoolOffice) SchoolOfficeAccountIDs(context.Context) ([]int64, error) {
	return f.ids, f.err
}

// fakeAccountPermissions answers the guardian schools (none) and the
// account permissions, as Identity & Access does.
type fakeAccountPermissions map[int64][]string

func (fakeAccountPermissions) ListGuardianSchoolIDs(context.Context, int64) ([]int64, error) {
	return nil, nil
}

func (f fakeAccountPermissions) FindEffectivePermissionNamesByAccountIDsForTenant(_ context.Context, ids []int64, _ int64) (map[int64][]string, error) {
	out := make(map[int64][]string, len(ids))
	for _, id := range ids {
		out[id] = f[id]
	}
	return out, nil
}

func unreportedReport(studentIDs ...int64) notifications.UnreportedAbsenceReport {
	return notifications.UnreportedAbsenceReport{
		TenantID: unreportedTenant, Date: calendar.NewDate(2099, 3, 2), StudentIDs: studentIDs, ActorAccountID: unreportedTeacher,
	}
}

// unreportedProducer is the absence notifier with the school recipients and
// the office; the consent is the default-on preference service.
func unreportedProducer(capture *captureNotifier, office fakeSchoolOffice, repo *fakePreferenceRepo) notifications.UnreportedAbsenceNotifier {
	consent := notifications.NewPreferenceService(repo, allSettingsOn(), nil, nil, securityruntime.HasPermission)
	recipients := notifications.NewStaffRecipientResolver(
		&stubConsent{}, &fakeStudentReader{}, &fakeGroupReader{}, &fakeStaffAccountReader{},
		&fakeAdminReader{}, &fakeOnDutySetting{}, &fakeDutyReader{})
	producer := notifications.NewAbsenceNotifier(capture, recipients, nil, nil,
		notifications.WithSchoolClassRecipients(contract.SchoolClassRecipients{
			Reach: &fakeSchoolClassReach{}, Consent: consent, Office: office,
		}))
	return producer.(notifications.UnreportedAbsenceNotifier)
}

func TestUnreportedAbsenceNoticeReachesTheOffice(t *testing.T) {
	t.Parallel()
	capture := &captureNotifier{}
	repo := &fakePreferenceRepo{optedOut: map[string][]int64{notifications.TypeSchoolUnreportedAbsence: {unreportedOffice2}}}
	producer := unreportedProducer(capture, fakeSchoolOffice{ids: []int64{unreportedOffice, unreportedOffice2, unreportedTeacher}}, repo)

	require.NoError(t, producer.NotifyUnreportedAbsences(context.Background(), unreportedReport(5, 6)))

	require.Len(t, capture.events, 1)
	event := capture.events[0]
	assert.Equal(t, notifications.TypeSchoolUnreportedAbsence, event.Type)
	assert.Equal(t, []int64{unreportedOffice}, event.Audience.StaffAccountIDs,
		"on by default, minus who switched it off and who marked the children")
	assert.Equal(t, "Fehlt ohne Meldung", event.Title)
	assert.Equal(t, "2 Kinder fehlen ohne Meldung", event.Body, "a number, never a name")
	assert.Equal(t, "/school", event.SchoolDeepLink)
	assert.Empty(t, event.DeepLink, "the notice has no place in the OGS portal")

	require.NoError(t, producer.NotifyUnreportedAbsences(context.Background(), unreportedReport(6, 5)))
	require.Len(t, capture.events, 2)
	assert.Equal(t, event.IdempotencyKey, capture.events[1].IdempotencyKey, "the same children on the same day are one notice")

	require.NoError(t, producer.NotifyUnreportedAbsences(context.Background(), unreportedReport(5)))
	assert.Equal(t, "1 Kind fehlt ohne Meldung", capture.events[2].Body)
	assert.NotEqual(t, event.IdempotencyKey, capture.events[2].IdempotencyKey)
}

func TestUnreportedAbsenceNoticeSilentCases(t *testing.T) {
	t.Parallel()
	capture := &captureNotifier{}
	producer := unreportedProducer(capture, fakeSchoolOffice{ids: []int64{unreportedTeacher}}, &fakePreferenceRepo{})
	require.NoError(t, producer.NotifyUnreportedAbsences(context.Background(), unreportedReport(5)))
	require.NoError(t, producer.NotifyUnreportedAbsences(context.Background(), unreportedReport()))
	assert.Empty(t, capture.events, "nobody but the actor, or nobody missing")

	failing := unreportedProducer(capture, fakeSchoolOffice{err: errors.New("down")}, &fakePreferenceRepo{})
	require.Error(t, failing.NotifyUnreportedAbsences(context.Background(), unreportedReport(5)))

	plain := notifications.NewAbsenceNotifier(capture, nil, nil, nil).(notifications.UnreportedAbsenceNotifier)
	require.NoError(t, plain.NotifyUnreportedAbsences(context.Background(), unreportedReport(5)))
	assert.Empty(t, capture.events, "without an office the notifier sends nothing")
}

func TestDefaultOnTypeIsOfferedToTheOfficeOnly(t *testing.T) {
	t.Parallel()
	ctx := tenant.WithTenantID(context.Background(), unreportedTenant)
	repo := &fakePreferenceRepo{}
	svc := notifications.NewPreferenceService(repo, allSettingsOn(), nil, fakeAccountPermissions{
		unreportedOffice:  {permissions.ClassDayRead, permissions.ClassDayAllClasses, permissions.ClassDayAbsenceReportWrite},
		unreportedTeacher: {permissions.ClassDayRead},
	}, securityruntime.HasPermission)

	enabled := func(accountID int64) (bool, bool) {
		overview, err := svc.GetForAccount(ctx, accountID, notifications.PortalSchool)
		require.NoError(t, err)
		for _, state := range overview.Types {
			if state.Key == notifications.TypeSchoolUnreportedAbsence {
				return true, state.Enabled
			}
		}
		return false, false
	}

	offered, on := enabled(unreportedOffice)
	assert.True(t, offered)
	assert.True(t, on, "on until the Schulverwaltung switches it off")
	offered, _ = enabled(unreportedTeacher)
	assert.False(t, offered, "a Lehrkraft is not offered the school-wide notice")

	require.ErrorIs(t, svc.SetForPortalAccount(ctx, unreportedTeacher, notifications.PortalSchool, notifications.TypeSchoolUnreportedAbsence, true),
		notifications.ErrUnknownNotificationType)
	require.NoError(t, svc.SetForPortalAccount(ctx, unreportedOffice, notifications.PortalSchool, notifications.TypeSchoolUnreportedAbsence, false))

	repo.stored = map[string]bool{notifications.TypeSchoolUnreportedAbsence: false}
	_, on = enabled(unreportedOffice)
	assert.False(t, on, "a stored decision wins over the default")

	repo.upserted = nil
	require.NoError(t, svc.DisableAllForPortalAccount(ctx, unreportedOffice, notifications.PortalSchool))
	require.Len(t, repo.upserted, 1, "switching everything off stores an explicit off for the default-on type")
	assert.Equal(t, notifications.TypeSchoolUnreportedAbsence, repo.upserted[0].NotificationType)
	assert.False(t, repo.upserted[0].Enabled)
}

// The notice goes to whoever may enter the missing report (#4791): offered
// and default-on for holders of class_day:absence_report_write, not for every
// role that sees all classes.
func TestUnreportedAbsenceNoticeFollowsTheRight(t *testing.T) {
	t.Parallel()
	def, ok := notifications.GetType(notifications.TypeSchoolUnreportedAbsence)
	require.True(t, ok)
	assert.Equal(t, permissions.ClassDayAbsenceReportWrite, def.Permission)
	assert.True(t, def.DefaultOn)

	ctx := tenant.WithTenantID(context.Background(), unreportedTenant)
	svc := notifications.NewPreferenceService(&fakePreferenceRepo{}, allSettingsOn(), nil, fakeAccountPermissions{
		unreportedOffice2: {permissions.ClassDayRead, permissions.ClassDayAllClasses},
	}, securityruntime.HasPermission)
	overview, err := svc.GetForAccount(ctx, unreportedOffice2, notifications.PortalSchool)
	require.NoError(t, err)
	require.NotEmpty(t, overview.Types)
	for _, state := range overview.Types {
		assert.NotEqual(t, notifications.TypeSchoolUnreportedAbsence, state.Key,
			"seeing all classes alone does not make the notice someone's task")
	}
}
