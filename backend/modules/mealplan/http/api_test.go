package mealplan_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	mealplanModule "github.com/moto-nrw/project-phoenix/modules/mealplan"
	mealplanHTTP "github.com/moto-nrw/project-phoenix/modules/mealplan/http"
	"github.com/stretchr/testify/assert"
)

type engine struct {
	available bool
	entries   []mealplanModule.Entry
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
		Protected: testutil.RecordingUnprotectedGroupFunc(&protected),
		Permission: func(access mealplanHTTP.Access) mealplanHTTP.Middleware {
			accesses = append(accesses, access)
			return testutil.IdentityMiddleware
		},
		Success:        testutil.RespondSuccess,
		InvalidRequest: testutil.RespondInvalidRequest,
		ModuleFailure:  testutil.ErrorResponder(resolveModuleFailure),
		ExportDailyList: func(mealplanModule.DailyList, string) (mealplanHTTP.ExportFile, error) {
			return mealplanHTTP.ExportFile{}, nil
		},
	})

	resource.Router()
	if !protected {
		t.Fatal("routes were registered outside the protected tenant boundary")
	}
	want := []mealplanHTTP.Access{mealplanHTTP.AccessRead, mealplanHTTP.AccessWrite, mealplanHTTP.AccessWrite, mealplanHTTP.AccessParticipants, mealplanHTTP.AccessParticipants}
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
func (e engine) ReplaceParticipationSchedule(context.Context, int64, int64, []mealplanModule.Weekday) (mealplanModule.Date, error) {
	return "2026-09-07", nil
}
func (e engine) SetParticipationDay(context.Context, int64, int64, string, bool) error {
	return nil
}
func (e engine) ClearParticipationDay(context.Context, int64, int64, string) error {
	return nil
}
func (e engine) DailyParticipants(context.Context, string) (mealplanModule.DailyList, error) {
	return mealplanModule.DailyList{Date: "2026-09-07", CutoffTime: "09:00", Participants: []mealplanModule.DailyParticipant{{StudentID: 42, FirstName: "Mia", LastName: "Muster", SchoolClass: "2a"}}}, nil
}

func resource(available bool) *mealplanHTTP.Resource {
	date, _ := mealplanModule.ParseDate("2026-09-07")
	module := mealplanModule.NewModule(engine{available: available, entries: []mealplanModule.Entry{{
		Date: date, Position: 0, Dish: "Nudeln",
	}}})
	return mealplanHTTP.NewResource(module, mealplanHTTP.Runtime{
		Protected: testutil.UnprotectedGroupFunc(),
		Permission: func(mealplanHTTP.Access) mealplanHTTP.Middleware {
			return testutil.IdentityMiddleware
		},
		Success:        testutil.RespondSuccess,
		InvalidRequest: testutil.RespondInvalidRequest,
		ModuleFailure:  testutil.ErrorResponder(resolveModuleFailure),
		ExportDailyList: func(mealplanModule.DailyList, string) (mealplanHTTP.ExportFile, error) {
			return mealplanHTTP.ExportFile{}, nil
		},
	})
}

func TestStaffDailyListResponseContract(t *testing.T) {
	t.Parallel()
	response := testutil.ExecuteRequest(resource(true).Router(), testutil.NewRequest("GET", "/participants?date=2026-09-07", nil))
	if response.Code != 200 {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	assert.JSONEq(t, `{"status":"success","data":{"date":"2026-09-07","cutoff_time":"09:00","participants":[{"student_id":42,"first_name":"Mia","last_name":"Muster","school_class":"2a"}]},"message":"Meal participation list retrieved successfully"}`, response.Body.String())
}

func TestStaffWeekResponseContract(t *testing.T) {
	t.Parallel()
	request := testutil.NewRequest("GET", "/?week_start=2026-09-07", nil)
	response := testutil.ExecuteRequest(resource(true).Router(), request)

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

	disabled := testutil.ExecuteRequest(resource(false).Router(), testutil.NewRequest("GET", "/?week_start=2026-09-07", nil))
	if disabled.Code != 403 {
		t.Fatalf("disabled status = %d, body = %s", disabled.Code, disabled.Body.String())
	}

	malformed := testutil.ExecuteRequest(resource(true).Router(), testutil.NewRequest("PUT", "/2026-09-07", strings.NewReader(`{}`)))
	if malformed.Code != 400 {
		t.Fatalf("malformed status = %d, body = %s", malformed.Code, malformed.Body.String())
	}
}

func TestStaffDeleteKeepsWeekendNoOpContract(t *testing.T) {
	t.Parallel()

	response := testutil.ExecuteRequest(resource(true).Router(), testutil.NewRequest("DELETE", "/2026-09-05", nil))
	if response.Code != 200 {
		t.Fatalf("weekend delete status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestStaffWeekendWriteKeepsLegacyErrorContract(t *testing.T) {
	t.Parallel()

	response := testutil.ExecuteRequest(resource(true).Router(), testutil.NewRequest(
		"PUT", "/2026-09-05", strings.NewReader(`{"dishes":[{"dish":"Suppe"}]}`),
	))
	if response.Code != 400 {
		t.Fatalf("weekend write status = %d, body = %s", response.Code, response.Body.String())
	}
	want := `{"status":"error","error":"meal plan covers weekdays only (Monday-Friday)"}` + "\n"
	if response.Body.String() != want {
		t.Fatalf("weekend write body = %q, want %q", response.Body.String(), want)
	}
}
