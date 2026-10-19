package mealplan_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil/routetest"
	mealplanModule "github.com/moto-nrw/project-phoenix/modules/mealplan"
	mealplanHTTP "github.com/moto-nrw/project-phoenix/modules/mealplan/http"
	"github.com/stretchr/testify/assert"
)

type engine struct {
	available bool
	entries   []mealplanModule.Entry
	// dayChange records the last day change: account, date, participating,
	// by school.
	dayChange *dayChange
}

type dayChange struct {
	accountID     int64
	date          string
	participating bool
	bySchool      bool
}

var moduleFailures = map[error]struct {
	status int
	err    error
}{
	mealplanModule.ErrDisabled:        {status: 403, err: errors.New("feature_disabled")},
	mealplanModule.ErrInvalidMealDate: {status: 400, err: errors.New("meal plan covers weekdays only (Monday-Friday)")},
	mealplanModule.ErrInvalidDishes:   {status: 400, err: mealplanModule.ErrInvalidDishes},
}

func resolveModuleFailure(err error) (int, error) {
	for target, failure := range moduleFailures {
		if errors.Is(err, target) {
			return failure.status, failure.err
		}
	}
	return 500, err
}

func TestStaffRoutesKeepProtectedPermissionBoundaries(t *testing.T) {
	t.Parallel()

	module := mealplanModule.NewModule(engine{available: true})
	protected := false
	var accesses []mealplanHTTP.Access
	resource := mealplanHTTP.NewResource(module, mealplanHTTP.Runtime{
		Protected: routetest.RecordingUnprotectedGroupFunc(&protected),
		Permission: func(access mealplanHTTP.Access) mealplanHTTP.Middleware {
			accesses = append(accesses, access)
			return routetest.IdentityMiddleware
		},
		Success:        routetest.RespondSuccess,
		InvalidRequest: routetest.RespondInvalidRequest,
		ModuleFailure:  routetest.ErrorResponder(resolveModuleFailure),
		RenderList: func(mealplanModule.ListExport, string) (mealplanHTTP.ExportFile, error) {
			return mealplanHTTP.ExportFile{}, nil
		},
		CurrentAccountID: func(context.Context) int64 { return 9 },
		Allowed:          func(context.Context, mealplanHTTP.Access) bool { return true },
	})

	resource.Router()
	if !protected {
		t.Fatal("routes were registered outside the protected tenant boundary")
	}
	// #3873: the school's changes of a child's registration need a
	// permission of their own; seeing the lists with names opens no change.
	want := []mealplanHTTP.Access{mealplanHTTP.AccessRead, mealplanHTTP.AccessWrite, mealplanHTTP.AccessWrite, mealplanHTTP.AccessParticipants, mealplanHTTP.AccessParticipants,
		mealplanHTTP.AccessRead, mealplanHTTP.AccessParticipants,
		mealplanHTTP.AccessRegister, mealplanHTTP.AccessRegister, mealplanHTTP.AccessRegister, mealplanHTTP.AccessRegister}
	if !reflect.DeepEqual(accesses, want) {
		t.Fatalf("permission boundaries = %v, want %v", accesses, want)
	}
}

func (e engine) Available(context.Context) (bool, error) { return e.available, nil }
func (e engine) Week(context.Context, string) ([]mealplanModule.Entry, error) {
	if !e.available {
		return nil, mealplanModule.ErrDisabled
	}
	return e.entries, nil
}
func (e engine) Replace(context.Context, string, []mealplanModule.Dish) error {
	if !e.available {
		return mealplanModule.ErrDisabled
	}
	return nil
}
func (e engine) Clear(context.Context, string) error                 { return nil }
func (e engine) RegistrationAvailable(context.Context) (bool, error) { return e.available, nil }
func (e engine) Participation(context.Context, int64, string, string) (mealplanModule.ParticipationPlan, error) {
	return mealplanModule.ParticipationPlan{}, nil
}
func (e engine) ReplaceParticipationSchedule(context.Context, int64, int64, []mealplanModule.Weekday, string) (mealplanModule.Date, error) {
	return "2026-09-07", nil
}
func (e engine) SetParticipationDay(_ context.Context, _ int64, accountID int64, date string, participating, bySchool bool) error {
	if e.dayChange != nil {
		*e.dayChange = dayChange{accountID: accountID, date: date, participating: participating, bySchool: bySchool}
	}
	return nil
}
func (e engine) SetParticipationWeek(context.Context, int64, int64, string, bool) (mealplanModule.WeekChange, error) {
	return mealplanModule.WeekChange{}, nil
}
func (e engine) ClearParticipationDay(context.Context, int64, int64, string, bool) error {
	return nil
}
func (e engine) DailyParticipants(context.Context, string) (mealplanModule.DailyList, error) {
	return mealplanModule.DailyList{Date: "2026-09-07", CutoffTime: "09:00", Deadline: time.Date(2026, 9, 7, 7, 0, 0, 0, time.UTC), Participants: []mealplanModule.DailyParticipant{{StudentID: 42, FirstName: "Mia", LastName: "Muster", SchoolClass: "2a"}},
		Others: []mealplanModule.DailyParticipant{{StudentID: 43, FirstName: "Ben", LastName: "Bauer", SchoolClass: "3b"}}}, nil
}

func resource(available bool) *mealplanHTTP.Resource {
	return resourceAllowing(available, true)
}

// resourceAllowing builds the resource for a caller who passes every access
// level (registering) or only those below a school's change.
func resourceAllowing(available, registering bool) *mealplanHTTP.Resource {
	date, _ := mealplanModule.ParseDate("2026-09-07")
	module := mealplanModule.NewModule(engine{available: available, entries: []mealplanModule.Entry{{
		Date: date, Position: 0, Dish: "Nudeln",
	}}})
	return mealplanHTTP.NewResource(module, mealplanHTTP.Runtime{
		Protected: routetest.UnprotectedGroupFunc(),
		Permission: func(mealplanHTTP.Access) mealplanHTTP.Middleware {
			return routetest.IdentityMiddleware
		},
		Success:        routetest.RespondSuccess,
		InvalidRequest: routetest.RespondInvalidRequest,
		ModuleFailure:  routetest.ErrorResponder(resolveModuleFailure),
		RenderList: func(mealplanModule.ListExport, string) (mealplanHTTP.ExportFile, error) {
			return mealplanHTTP.ExportFile{}, nil
		},
		CurrentAccountID: func(context.Context) int64 { return 9 },
		Allowed: func(_ context.Context, access mealplanHTTP.Access) bool {
			return registering || access != mealplanHTTP.AccessRegister
		},
	})
}

func TestStaffDailyListResponseContract(t *testing.T) {
	t.Parallel()
	response := routetest.ExecuteRequest(resource(true).Router(), routetest.NewRequest("GET", "/participants?date=2026-09-07", nil))
	if response.Code != 200 {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	assert.JSONEq(t, `{"status":"success","data":{"date":"2026-09-07","cutoff_time":"09:00","deadline":"2026-09-07T07:00:00Z","closed":false,"participants":[{"student_id":"42","first_name":"Mia","last_name":"Muster","school_class":"2a"}]},"message":"Meal participation list retrieved successfully"}`, response.Body.String())
}

func TestStaffWeekResponseContract(t *testing.T) {
	t.Parallel()
	request := routetest.NewRequest("GET", "/?week_start=2026-09-07", nil)
	response := routetest.ExecuteRequest(resource(true).Router(), request)

	if response.Code != 200 {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	assert.JSONEq(t, `{
		"status":"success",
		"data":[{"date":"2026-09-07","position":0,"dish":"Nudeln"}],
		"message":"Meal plan retrieved successfully"
	}`, response.Body.String())
}

func TestStaffDisabledAndMalformedContracts(t *testing.T) {
	t.Parallel()

	disabled := routetest.ExecuteRequest(resource(false).Router(), routetest.NewRequest("GET", "/?week_start=2026-09-07", nil))
	if disabled.Code != 403 {
		t.Fatalf("disabled status = %d, body = %s", disabled.Code, disabled.Body.String())
	}

	malformed := routetest.ExecuteRequest(resource(true).Router(), routetest.NewRequest("PUT", "/2026-09-07", strings.NewReader(`{}`)))
	if malformed.Code != 400 {
		t.Fatalf("malformed status = %d, body = %s", malformed.Code, malformed.Body.String())
	}
}

func TestStaffDeleteKeepsWeekendNoOpContract(t *testing.T) {
	t.Parallel()

	response := routetest.ExecuteRequest(resource(true).Router(), routetest.NewRequest("DELETE", "/2026-09-05", nil))
	if response.Code != 200 {
		t.Fatalf("weekend delete status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestStaffWeekendWriteKeepsItsErrorContract(t *testing.T) {
	t.Parallel()

	response := routetest.ExecuteRequest(resource(true).Router(), routetest.NewRequest(
		"PUT", "/2026-09-05", strings.NewReader(`{"dishes":[{"dish":"Suppe"}]}`),
	))
	if response.Code != 400 {
		t.Fatalf("weekend write status = %d, body = %s", response.Code, response.Body.String())
	}
	want := `{"status":400,"detail":"meal plan covers weekdays only (Monday-Friday)","code":"general.input"}` + "\n"
	if response.Body.String() != want {
		t.Fatalf("weekend write body = %q, want %q", response.Body.String(), want)
	}
}

func TestStaffDailyListListsTheOthersOnRequest(t *testing.T) {
	t.Parallel()
	response := routetest.ExecuteRequest(resource(true).Router(), routetest.NewRequest("GET", "/participants?date=2026-09-07&include=all", nil))
	if response.Code != 200 {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	assert.Contains(t, response.Body.String(), `"others":[{"student_id":"43","first_name":"Ben","last_name":"Bauer","school_class":"3b"}]`)

	// Without the right to register a child, the children who do not eat
	// stay out of the list.
	reader := routetest.ExecuteRequest(resourceAllowing(true, false).Router(), routetest.NewRequest("GET", "/participants?date=2026-09-07&include=all", nil))
	assert.Equal(t, 200, reader.Code)
	assert.NotContains(t, reader.Body.String(), "others")
	assert.NotContains(t, reader.Body.String(), "Ben")
}

func TestStaffDayChangeIsTheSchools(t *testing.T) {
	t.Parallel()
	recorded := &dayChange{}
	module := mealplanModule.NewModule(engine{available: true, dayChange: recorded})
	router := mealplanHTTP.NewResource(module, mealplanHTTP.Runtime{
		Protected:  routetest.UnprotectedGroupFunc(),
		Permission: func(mealplanHTTP.Access) mealplanHTTP.Middleware { return routetest.IdentityMiddleware },
		Success:    routetest.RespondSuccess, InvalidRequest: routetest.RespondInvalidRequest,
		ModuleFailure: routetest.ErrorResponder(resolveModuleFailure),
		RenderList: func(mealplanModule.ListExport, string) (mealplanHTTP.ExportFile, error) {
			return mealplanHTTP.ExportFile{}, nil
		},
		CurrentAccountID: func(context.Context) int64 { return 9 },
		Allowed:          func(context.Context, mealplanHTTP.Access) bool { return true },
	}).Router()

	response := routetest.ExecuteRequest(router, routetest.NewRequest("PUT", "/participation/42/days/2026-09-10", strings.NewReader(`{"participating":false}`)))
	if response.Code != 200 {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	assert.Equal(t, dayChange{accountID: 9, date: "2026-09-10", participating: false, bySchool: true}, *recorded)

	missing := routetest.ExecuteRequest(router, routetest.NewRequest("PUT", "/participation/42/days/2026-09-10", strings.NewReader(`{}`)))
	assert.Equal(t, 400, missing.Code)
	badStudent := routetest.ExecuteRequest(router, routetest.NewRequest("PUT", "/participation/abc/days/2026-09-10", strings.NewReader(`{"participating":true}`)))
	assert.Equal(t, 400, badStudent.Code)
}

func (e engine) SettleOrders(context.Context) error { return nil }

func (e engine) Orders(_ context.Context, from, _ string) (mealplanModule.OrderWeek, error) {
	confirmed := time.Date(2026, 9, 7, 9, 30, 0, 0, time.UTC)
	return mealplanModule.OrderWeek{Rhythm: "daily", Detail: "by_class", Days: []mealplanModule.OrderDay{{
		Date: mealplanModule.Date(from), Deadline: time.Date(2026, 9, 7, 7, 0, 0, 0, time.UTC), Status: mealplanModule.OrderConfirmed, OrderID: 12, ConfirmedAt: &confirmed, Total: 1,
		Classes:  []mealplanModule.ClassCount{{SchoolClass: "2a", Count: 1}},
		Children: []mealplanModule.OrderChild{{StudentID: 42, FirstName: "Mia", LastName: "Muster", SchoolClass: "2a"}},
		Changes:  []mealplanModule.OrderChange{{Sequence: 3, Date: mealplanModule.Date(from), Child: mealplanModule.OrderChild{StudentID: 43, FirstName: "Ben", SchoolClass: "3b"}, Delta: -1, DetectedAt: time.Date(2026, 9, 7, 8, 0, 0, 0, time.UTC)}},
	}}}, nil
}

// #3873: the school's order view carries numbers and classes, never a name.
func TestStaffOrdersResponseContract(t *testing.T) {
	t.Parallel()
	response := routetest.ExecuteRequest(resource(true).Router(), routetest.NewRequest("GET", "/orders?from=2026-09-07&to=2026-09-07", nil))
	if response.Code != 200 {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	assert.JSONEq(t, `{"status":"success","data":{"format_version":1,"rhythm":"daily","detail":"by_class","days":[{"date":"2026-09-07","closed":false,"deadline":"2026-09-07T07:00:00Z","status":"confirmed","order_id":"12","confirmed_at":"2026-09-07T09:30:00Z","total":1,"classes":[{"school_class":"2a","count":1}],"changes":[{"sequence":3,"school_class":"3b","delta":-1,"detected_at":"2026-09-07T08:00:00Z"}]}]},"message":"Meal orders retrieved successfully"}`, response.Body.String())
	assert.NotContains(t, response.Body.String(), "Mia")

	bad := routetest.ExecuteRequest(resource(true).Router(), routetest.NewRequest("GET", "/orders?from=7.9.2026&to=2026-09-07", nil))
	assert.Equal(t, 400, bad.Code)
}

func TestStaffMonthExportRefusesBadInput(t *testing.T) {
	t.Parallel()
	for _, query := range []string{"month=2026-13&format=pdf", "month=2026-09&format=docx"} {
		response := routetest.ExecuteRequest(resource(true).Router(), routetest.NewRequest("GET", "/orders/month/export?"+query, nil))
		assert.Equal(t, 400, response.Code, query)
	}
	ok := routetest.ExecuteRequest(resource(true).Router(), routetest.NewRequest("GET", "/orders/month/export?month=2026-09&format=xlsx", nil))
	assert.Equal(t, 200, ok.Code)
}
