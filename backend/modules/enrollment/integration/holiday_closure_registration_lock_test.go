package integration

import (
	"context"
	"testing"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
	"github.com/moto-ogs/project-phoenix/modules/enrollment/internal/adapters/postgres"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/moto-ogs/project-phoenix/tenant"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Submission and rebooking both lock the period before validating the calendar.
// Preparation must not pass that boundary and miss their later commit.
func TestBookingValidationSerializesWithClosurePreparation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	w := newPlacesWeek(t, db, "Träger Anmeldung während Schließtag")
	mail := &capturedMails{}
	closures := newClosures(mail, closureCalendar{{Date: calendar.NewDate(2027, 7, 7), Kind: enrollment.HolidayNoCareClosingDay}})
	operation := closureOperation(hostSchoolOf(t, db, w.week))
	base := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	var before enrollment.HolidayClosureImpact
	require.NoError(t, testpkg.WithinTraeger(base, w.organization, func(ctx context.Context) error {
		var err error
		before, err = planOne(ctx, closures, w.organization, operation)
		return err
	}))
	require.NoError(t, testpkg.WithinTraeger(base, w.organization, func(ctx context.Context) error {
		_, found, err := postgres.NewHolidayPeriodStore().FindHolidayPeriod(ctx, w.organization, w.period, true)
		require.NoError(t, err)
		require.True(t, found)
		err = testpkg.WithinTraeger(base, w.organization, func(other context.Context) error {
			raw, _ := tenant.TransactionFromContext(other)
			_, err := raw.(bun.Tx).ExecContext(other, `SET LOCAL lock_timeout = '100ms'`)
			if err != nil {
				return err
			}
			operation.Token = before.Token
			_, err = closures.NotifyHolidayClosureOperation(other, w.organization, operation)
			return err
		})
		require.ErrorContains(t, err, "lock timeout", "preparation must wait for the validated booking transaction")
		assert.Empty(t, mail.mails)
		// Commit an additional family's booking after its pre-closure validation.
		raw, _ := tenant.TransactionFromContext(ctx)
		tx := raw.(bun.Tx)
		var registration, child int64
		require.NoError(t, tx.NewRaw(`INSERT INTO enrollment.holiday_registrations (organization_id, holiday_period_id,
   guardian_first_name,guardian_last_name,guardian_email,guardian_phone,privacy_consent_at)
   VALUES (?,?,'Eva','Neu','spaeter@example.org','0123',NOW()) RETURNING id`, w.organization, w.period).Scan(ctx, &registration))
		require.NoError(t, tx.NewRaw(`INSERT INTO enrollment.holiday_registration_children (organization_id,holiday_period_id,holiday_registration_id,position,first_name,last_name,birth_date)
   VALUES (?,?,?,1,'Mia','Neu','2019-03-04') RETURNING id`, w.organization, w.period, registration).Scan(ctx, &child))
		_, err = tx.ExecContext(ctx, `INSERT INTO enrollment.holiday_registration_bookings (organization_id,holiday_period_id,holiday_registration_child_id,holiday_period_week_id,holiday_week_time_model_id)
   SELECT organization_id,holiday_period_id,?,holiday_period_week_id,holiday_week_time_model_id
   FROM enrollment.holiday_registration_bookings WHERE organization_id = ? AND holiday_period_week_id = ? LIMIT 1`, child, w.organization, w.week)
		return err
	}))
	require.NoError(t, testpkg.WithinTraeger(base, w.organization, func(ctx context.Context) error {
		latest, err := planOne(ctx, closures, w.organization, operation)
		require.NoError(t, err)
		assert.Equal(t, before.Families+1, latest.Families, "the late commit remains eligible")
		operation.Token = latest.Token
		sent, err := closures.NotifyHolidayClosureOperation(ctx, w.organization, operation)
		require.NoError(t, err)
		assert.Equal(t, latest.Families, sent.Mails)
		return nil
	}))
}
