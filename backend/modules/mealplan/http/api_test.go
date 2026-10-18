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
		ExportDailyList: func(mealplanModule.DailyList, string) (mealplanHTTP.ExportFile, error) {
			return mealplanHTTP.ExportFile{}, nil
		},
		CurrentAccountID: func(context.Context) int64 { return 9 },
	})

	resource.Router()
	if !protected {
		t.Fatal("routes were registered outside the protected tenant boundary")
	}
	// #3873: the school's changes need the permission of the lists with
	// names, like the lists themselves.
	want := []mealplanHTTP.Access{mealplanHTTP.AccessRead, mealplanHTTP.AccessWrite, mealplanHTTP.AccessWrite, mealplanHTTP.AccessParticipants, mealplanHTTP.AccessParticipants,
		mealplanHTTP.AccessParticipants, mealplanHTTP.AccessParticipants, mealplanHTTP.AccessParticipants, mealplanHTTP.AccessParticipants}
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
		ExportDailyList: func(mealplanModule.DailyList, string) (mealplanHTTP.ExportFile, error) {
			return mealplanHTTP.ExportFile{}, nil
		},
		CurrentAccountID: func(context.Context) int64 { return 9 },
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
		ExportDailyList: func(mealplanModule.DailyList, string) (mealplanHTTP.ExportFile, error) {
			return mealplanHTTP.ExportFile{}, nil
		},
		CurrentAccountID: func(context.Context) int64 { return 9 },
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
