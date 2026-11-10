package api

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func TestHolidayGuestPlansOnlyBookedDaysAndReconcilesDateChanges(t *testing.T) {
	t.Parallel()
	f := newHolidayGuestFixture(t, "Träger Tagesgäste")
	week := f.offer.Weeks[0]
	// Turn the still unconfirmed fixture into a day offer before placement.
	_, err := f.db.ExecContext(t.Context(), `UPDATE enrollment.holiday_period_weeks SET booking_unit = 'day'
		WHERE organization_id = ? AND id = ?`, f.office.organization, week.ID)
	require.NoError(t, err)
	_, err = f.db.ExecContext(t.Context(), `UPDATE enrollment.holiday_registration_bookings SET days = ARRAY['2099-07-06'::date]
		WHERE organization_id = ? AND holiday_period_week_id = ?`, f.office.organization, week.ID)
	require.NoError(t, err)
	f.confirm(t, f.siblings, "confirm")
	lead := f.staffOf(t, f.nord, permissions.ConfigRead, permissions.UsersCreate, permissions.UsersRead)
	rec := f.assign(t, lead, week.ID, f.group)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var guest struct {
		StudentID int64
		ChildID   int64
	}
	require.NoError(t, f.db.NewRaw(`SELECT student_id, holiday_registration_child_id AS child_id FROM enrollment.holiday_guests
		WHERE tenant_id = ? AND holiday_period_week_id = ?`, f.nord, week.ID).Scan(t.Context(), &guest))
	assert.Equal(t, []string{"2099-07-06 08:00-13:00"}, f.careDays(t, guest.StudentID))
	f.reconcile(t)
	assert.Equal(t, []string{"2099-07-06 08:00-13:00"}, f.careDays(t, guest.StudentID), "unchanged dates keep the plan")

	path := holidayRebookPath(f.periodID(t), f.siblings, guest.ChildID)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, path, map[string]any{"bookings": []map[string]any{
		{"week_id": week.ID, "time_model_id": week.TimeModels[0].ID, "days": []string{"2099-07-07"}},
	}})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	f.reconcile(t)
	assert.Equal(t, []string{"2099-07-07 08:00-13:00"}, f.careDays(t, guest.StudentID), "date-only rebooking removes the old day")
	f.today.Store(calendar.NewDate(2099, 7, 6))
	f.reconcile(t)
	assert.Equal(t, []string{"2099-07-07 08:00-13:00"}, f.careDays(t, guest.StudentID))
}
