// Router-level tests of the places of the holiday weeks (#4699, decision
// E-e): the capacity of a week, places in the order the registrations came
// in, the waiting list per booking, moving up by a cancellation, a rebooking
// or a higher capacity with a mail to the parents, the contribution and the
// host school's participant list without waiting bookings, and the lock that
// keeps two registrations at the same time from both taking the last place.
package api

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

type placementBody struct {
	ChildPosition int    `json:"child_position"`
	WeekID        int64  `json:"week_id"`
	Placement     string `json:"placement"`
}

type receiptBody struct {
	ID         int64           `json:"id"`
	Placements []placementBody `json:"placements"`
}

type weekPlacesBody struct {
	WeekID   int64 `json:"week_id"`
	Capacity *int  `json:"capacity"`
	Placed   int   `json:"placed"`
	Waitlist []struct {
		Position       int    `json:"position"`
		RegistrationID int64  `json:"registration_id"`
		ChildFirstName string `json:"child_first_name"`
	} `json:"waitlist"`
}

type placeChangeBody struct {
	MovedUp []int64 `json:"moved_up"`
}

func holidayPlacesPath(periodID int64) string {
	return fmt.Sprintf("%s/places", holidayPeriodPath(periodID))
}

func holidayCapacityPath(periodID, weekID int64) string {
	return fmt.Sprintf("/traeger/holiday-contributions/%d/weeks/%d/capacity", periodID, weekID)
}

// placesRegistration is a registration of the places tests: each named
// child books the first time model of the weeks it names.
func placesRegistration(f reviewFixture, email string, children map[string][]int) map[string]any {
	body := siblingsRegistration(f.offer, email)
	list := []map[string]any{}
	for _, name := range []string{"Tom", "Ida", "Max", "Ole"} {
		weeks, ok := children[name]
		if !ok {
			continue
		}
		bookings := []map[string]any{}
		for _, index := range weeks {
			bookings = append(bookings, map[string]any{"week_id": f.offer.Weeks[index].ID, "time_model_id": f.offer.Weeks[index].TimeModels[0].ID})
		}
		list = append(list, map[string]any{
			"first_name": name, "last_name": "Platz", "birth_date": "2017-05-04", "home_school_id": f.offer.Schools[0].ID, "bookings": bookings,
		})
	}
	body["children"] = list
	return body
}

func (f reviewFixture) submitPlaces(t *testing.T, email string, children map[string][]int) receiptBody {
	t.Helper()
	rec := publicJSON(t, f.public, http.MethodPost, registrationPath(f.period.PublicToken), placesRegistration(f, email, children))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[receiptBody](t, rec.Body.Bytes())
}

func (f reviewFixture) setCapacity(t *testing.T, weekID int64, capacity any) placeChangeBody {
	t.Helper()
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, holidayCapacityPath(f.period.ID, weekID), map[string]any{"capacity": capacity})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[placeChangeBody](t, rec.Body.Bytes())
}

func (f reviewFixture) places(t *testing.T) map[int64]weekPlacesBody {
	t.Helper()
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodGet, holidayPlacesPath(f.period.ID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	weeks := map[int64]weekPlacesBody{}
	for _, week := range decodeSchoolPortalData[[]weekPlacesBody](t, rec.Body.Bytes()) {
		weeks[week.WeekID] = week
	}
	return weeks
}

func (f reviewFixture) contributionTotal(t *testing.T, registrationID int64) int64 {
	t.Helper()
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodGet, holidayContributionPath(f.period.ID, registrationID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[contributionBody](t, rec.Body.Bytes()).TotalCents
}

func (f reviewFixture) mails(t *testing.T, kind string, registrationID int64) int {
	t.Helper()
	var count int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM platform.email_outbox
		WHERE organization_id = ? AND kind = ? AND related_entity_id = ?`, f.office.organization, kind, registrationID).
		Scan(context.Background(), &count))
	return count
}

// hostParticipants counts the children the host school of the week sees in
// its participant list, as the host school's session would.
func (f reviewFixture) hostParticipants(t *testing.T, hostSchoolID, weekID int64) int {
	t.Helper()
	ctx := context.Background()
	tx, err := f.db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, tx.Rollback()) }()
	_, err = tx.ExecContext(ctx, `SELECT set_config('app.current_tenant_id', ?, true)`, fmt.Sprint(hostSchoolID))
	require.NoError(t, err)
	var count int
	require.NoError(t, tx.NewRaw(`SELECT count(*) FROM enrollment.holiday_host_participants(?, ?)`, f.office.organization, weekID).Scan(ctx, &count))
	return count
}

// TestHolidayPlacesGoInOrderOfArrivalAndMoveUpOnCancellation pins the
// waiting list end to end. Each guard goes red without its protection: the
// placement at sending (waitlisted bookings), the order of the waiting list,
// the contribution without waiting bookings (chain filter), the host
// school's list without them (the placement condition of migration
// 1.15.855) and the move up with its mail and new contribution.
func TestHolidayPlacesGoInOrderOfArrivalAndMoveUpOnCancellation(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Plätze Reihenfolge")
	first, second := f.offer.Weeks[0], f.offer.Weeks[1]
	// Lia and Ben of the fixture hold both places of the first week.
	change := f.setCapacity(t, first.ID, 2)
	assert.Empty(t, change.MovedUp)

	tom := f.submitPlaces(t, "tom.platz@example.org", map[string][]int{"Tom": {0}})
	assert.Equal(t, []placementBody{{ChildPosition: 1, WeekID: first.ID, Placement: "waitlisted"}}, tom.Placements)
	// Siblings are placed apart: Max gets his week, Ida waits behind Tom.
	family := f.submitPlaces(t, "ida.platz@example.org", map[string][]int{"Ida": {0}, "Max": {1}})
	assert.ElementsMatch(t, []placementBody{
		{ChildPosition: 1, WeekID: first.ID, Placement: "waitlisted"},
		{ChildPosition: 2, WeekID: second.ID, Placement: "placed"},
	}, family.Placements)

	weeks := f.places(t)
	require.NotNil(t, weeks[first.ID].Capacity)
	assert.Equal(t, 2, weeks[first.ID].Placed)
	require.Len(t, weeks[first.ID].Waitlist, 2)
	assert.Equal(t, tom.ID, weeks[first.ID].Waitlist[0].RegistrationID, "the waiting list keeps the order of arrival")
	assert.Equal(t, 2, weeks[first.ID].Waitlist[1].Position)
	assert.Nil(t, weeks[second.ID].Capacity, "a week without a capacity is unlimited")
	assert.Empty(t, weeks[second.ID].Waitlist)

	var receipt string
	require.NoError(t, f.db.NewRaw(`SELECT payload::text FROM platform.email_outbox
		WHERE organization_id = ? AND kind = 'holiday_registration_received' AND related_entity_id = ?`, f.office.organization, tom.ID).
		Scan(context.Background(), &receipt))
	assert.Contains(t, receipt, `"waitlist_weeks"`, "the receipt names the weeks on the waiting list")

	// The family is confirmed with Ida still waiting: only Max's week is billed.
	f.detail(t, f.act(t, f.siblings, "confirm"))
	confirmed := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, holidayConfirmPath(f.period.ID, family.ID), map[string]any{})
	require.Equal(t, http.StatusOK, confirmed.Code, confirmed.Body.String())
	assert.Equal(t, second.TimeModels[0].PriceCents, f.contributionTotal(t, family.ID), "a waiting booking is never billed")
	detail := f.detail(t, traegerJSON(t, f.traeger, f.office.token, http.MethodGet, f.registrationPath(f.siblings, ""), nil))
	host := detail.Children[0].Bookings[0].HostSchoolID
	assert.Equal(t, 2, f.hostParticipants(t, host, first.ID), "the host school sees Lia and Ben, not the waiting Ida")

	// Cancelling Lia and Ben frees both places: Tom and Ida move up in order.
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, holidayCancelPath(f.period.ID, f.siblings), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, []int64{tom.ID, family.ID}, decodeSchoolPortalData[placeChangeBody](t, rec.Body.Bytes()).MovedUp)
	weeks = f.places(t)
	assert.Equal(t, 2, weeks[first.ID].Placed)
	assert.Empty(t, weeks[first.ID].Waitlist)
	assert.Equal(t, 1, f.mails(t, "holiday_place_granted", tom.ID), "Tom's parents hear of the place")
	assert.Equal(t, 1, f.mails(t, "holiday_place_granted", family.ID))
	assert.Equal(t, first.TimeModels[0].PriceCents+second.TimeModels[0].PriceCents, f.contributionTotal(t, family.ID),
		"the contribution of the confirmed family is computed again with Ida's week")
	assert.Equal(t, 1, f.hostParticipants(t, host, first.ID), "Ida, confirmed and placed, is now on the host school's list")

	history := f.detail(t, traegerJSON(t, f.traeger, f.office.token, http.MethodGet, f.registrationPath(tom.ID, ""), nil)).History
	assert.Equal(t, "promote", history[len(history)-1].Action)
}

// TestHolidayPlacesMoveUpOnRebookingAndCapacity frees a place by a
// rebooking and by a higher capacity; a lower capacity takes no place away.
// The routes refuse a capacity out of range, a week of another period and
// another Träger.
func TestHolidayPlacesMoveUpOnRebookingAndCapacity(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Plätze Umbuchung")
	first, second := f.offer.Weeks[0], f.offer.Weeks[1]
	f.setCapacity(t, first.ID, 2)
	tom := f.submitPlaces(t, "tom.umbuchung@example.org", map[string][]int{"Tom": {0}})
	require.Equal(t, "waitlisted", tom.Placements[0].Placement)

	// Ben leaves the first week: Tom gets his place.
	ben := f.list(t, "").Registrations[0].Children[1]
	rebook := map[string]any{"bookings": []map[string]any{{"week_id": second.ID, "time_model_id": second.TimeModels[0].ID}}}
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, holidayRebookPath(f.period.ID, f.siblings, ben.ID), rebook)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Empty(t, f.places(t)[first.ID].Waitlist)
	assert.Equal(t, 1, f.mails(t, "holiday_place_granted", tom.ID))

	ole := f.submitPlaces(t, "ole.kapazitaet@example.org", map[string][]int{"Ole": {0}})
	require.Equal(t, "waitlisted", ole.Placements[0].Placement)
	assert.Equal(t, []int64{ole.ID}, f.setCapacity(t, first.ID, 3).MovedUp, "a higher capacity moves the waiting list up")
	assert.Empty(t, f.setCapacity(t, first.ID, 1).MovedUp)
	assert.Equal(t, 3, f.places(t)[first.ID].Placed, "a lower capacity takes no place away")
	assert.Empty(t, f.setCapacity(t, first.ID, nil).MovedUp)
	assert.Nil(t, f.places(t)[first.ID].Capacity)

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, holidayCapacityPath(f.period.ID, first.ID), map[string]any{"capacity": -1})
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	code, fields := registrationRefusalOf(t, rec)
	assert.Equal(t, "enrollment.holiday_registration_invalid", code)
	assert.Equal(t, []string{"capacity"}, fields)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, holidayCapacityPath(f.period.ID, first.ID+second.ID+1000), map[string]any{"capacity": 1})
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	code, _ = registrationRefusalOf(t, rec)
	assert.Equal(t, "enrollment.holiday_week_not_found", code)

	other := newTraegerOffice(t, f.db, f.traeger, "Träger Plätze Fremd", identityaccess.TraegerRoleAdministration)
	rec = traegerJSON(t, f.traeger, other.token, http.MethodGet, holidayPlacesPath(f.period.ID), nil)
	assert.Equal(t, http.StatusNotFound, rec.Code, "another Träger reads no places: %s", rec.Body.String())
	rec = traegerJSON(t, f.traeger, other.token, http.MethodPut, holidayCapacityPath(f.period.ID, first.ID), map[string]any{"capacity": 9})
	assert.Equal(t, http.StatusNotFound, rec.Code, "nor changes them: %s", rec.Body.String())
}

// TestHolidayPlacesHoldTheLastPlaceAgainstAConcurrentRegistration runs two
// transactions at once for the last place of a week. The first takes it
// while it holds the week's row; the registration sent meanwhile waits for
// that lock and then counts the place as taken. Without SELECT ... FOR
// UPDATE on the week, the second counts in its own snapshot, sees the place
// free and overbooks the week; under REPEATABLE READ it would count in the
// snapshot of its first statement and overbook too.
func TestHolidayPlacesHoldTheLastPlaceAgainstAConcurrentRegistration(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Plätze Gleichzeitig")
	first := f.offer.Weeks[0]
	f.setCapacity(t, first.ID, 3)
	ctx := context.Background()

	holder, err := f.db.BeginTx(ctx, nil)
	require.NoError(t, err)
	committed := false
	defer func() {
		if !committed {
			require.NoError(t, holder.Rollback())
		}
	}()
	var pid int
	require.NoError(t, holder.NewRaw(`SELECT pg_backend_pid()`).Scan(ctx, &pid))
	_, err = holder.ExecContext(ctx, `SELECT id FROM enrollment.holiday_period_weeks WHERE id = ? FOR UPDATE`, first.ID)
	require.NoError(t, err)
	var registration, child int64
	require.NoError(t, holder.NewRaw(`INSERT INTO enrollment.holiday_registrations (organization_id, holiday_period_id,
			guardian_first_name, guardian_last_name, guardian_email, guardian_phone, privacy_consent_at)
		VALUES (?, ?, 'Erika', 'Erste', 'erika.erste@example.org', '0123', NOW()) RETURNING id`,
		f.office.organization, f.period.ID).Scan(ctx, &registration))
	require.NoError(t, holder.NewRaw(`INSERT INTO enrollment.holiday_registration_children (organization_id, holiday_period_id,
			holiday_registration_id, position, first_name, last_name, birth_date)
		VALUES (?, ?, ?, 1, 'Ella', 'Erste', '2017-01-02') RETURNING id`, f.office.organization, f.period.ID, registration).Scan(ctx, &child))
	_, err = holder.ExecContext(ctx, `INSERT INTO enrollment.holiday_registration_bookings (organization_id, holiday_period_id,
			holiday_registration_child_id, holiday_period_week_id, holiday_week_time_model_id, placement) VALUES (?, ?, ?, ?, ?, 'placed')`,
		f.office.organization, f.period.ID, child, first.ID, first.TimeModels[0].ID)
	require.NoError(t, err)

	sent := make(chan receiptBody, 1)
	go func() {
		rec := publicJSON(t, f.public, http.MethodPost, registrationPath(f.period.PublicToken),
			placesRegistration(f, "tom.gleichzeitig@example.org", map[string][]int{"Tom": {0}}))
		if rec.Code != http.StatusCreated {
			t.Errorf("the second registration was refused: %d %s", rec.Code, rec.Body.String())
		}
		sent <- decodeSchoolPortalData[receiptBody](t, rec.Body.Bytes())
	}()
	// Wait until the second registration waits for the week's lock, or has
	// already finished, which only happens without the lock.
	var receipt *receiptBody
	deadline := time.Now().Add(20 * time.Second)
	for receipt == nil && time.Now().Before(deadline) {
		var blocked int
		require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM pg_stat_activity WHERE ? = ANY(pg_blocking_pids(pid))`, pid).Scan(ctx, &blocked))
		if blocked > 0 {
			break
		}
		select {
		case done := <-sent:
			receipt = &done
		case <-time.After(20 * time.Millisecond):
		}
	}
	require.NoError(t, holder.Commit())
	committed = true
	if receipt == nil {
		done := <-sent
		receipt = &done
	}

	require.Len(t, receipt.Placements, 1)
	assert.Equal(t, "waitlisted", receipt.Placements[0].Placement, "the second registration finds the last place taken")
	var placed int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM enrollment.holiday_registration_bookings AS booking
		JOIN enrollment.holiday_registration_children AS child ON child.id = booking.holiday_registration_child_id
		JOIN enrollment.holiday_registrations AS registration ON registration.id = child.holiday_registration_id
		WHERE booking.holiday_period_week_id = ? AND booking.placement = 'placed' AND registration.status IN ('received', 'confirmed')`,
		first.ID).Scan(ctx, &placed))
	assert.Equal(t, 3, placed, "the week is not overbooked")
}
