package mealplan_test

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	mealplanModule "github.com/moto-nrw/project-phoenix/modules/mealplan"
	mealplanHTTP "github.com/moto-nrw/project-phoenix/modules/mealplan/http"
)

type engine struct {
	available bool
	entries   []mealplanModule.Entry
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
		Success: testutil.RespondSuccess,
		Failure: testutil.RespondError,
	})

	resource.Router()
	if !protected {
		t.Fatal("routes were registered outside the protected tenant boundary")
	}
	want := []mealplanHTTP.Access{mealplanHTTP.AccessRead, mealplanHTTP.AccessWrite, mealplanHTTP.AccessWrite}
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
func (e engine) Clear(context.Context, string) error { return nil }

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
		Success: testutil.RespondSuccess,
		Failure: testutil.RespondError,
	})
}

func TestStaffWeekResponseContract(t *testing.T) {
	t.Parallel()
	request := testutil.NewRequest("GET", "/?week_start=2026-09-07", nil)
	response := testutil.ExecuteRequest(resource(true).Router(), request)

	if response.Code != 200 {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if body := response.Body.String(); !strings.Contains(body, `"date":"2026-09-07"`) || !strings.Contains(body, `"dish":"Nudeln"`) {
		t.Fatalf("response body = %s", body)
	}
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
