package students

// Table-driven tests for the three photo error mappers. Each sentinel must
// land on a stable HTTP status — the frontend keys German user-facing
// strings off these codes, and PyrePortal does the same for the IoT-side
// flows that touch students. Drift here = silent breakage in the kiosk UI.

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	userService "github.com/moto-nrw/project-phoenix/services/users"
)

func mapperReq() *http.Request {
	return httptest.NewRequest(http.MethodPost, "/api/students/1/photo", nil)
}

func TestMapPhotoUploadError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
		want int
	}{
		{"feature disabled", userService.ErrPhotoFeatureDisabled, http.StatusForbidden},
		{"feature disabled mid-tx", userService.ErrPhotoFeatureDisabledMid, http.StatusForbidden},
		{"student not found", userService.ErrPhotoStudentNotFound, http.StatusNotFound},
		{"student forbidden", userService.ErrPhotoStudentForbidden, http.StatusForbidden},
		{"student reassigned mid-tx", userService.ErrPhotoStudentReassigned, http.StatusForbidden},
		{"consent required first", userService.ErrPhotoConsentRequired, http.StatusBadRequest},
		{"consent withdrawn mid-tx", userService.ErrPhotoConsentWithdrawn, http.StatusConflict},
		{"no tenant context", userService.ErrPhotoNoTenant, http.StatusBadRequest},
		{"unknown error", errors.New("boom"), http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			mapPhotoUploadError(rec, mapperReq(), tc.err)
			if rec.Code != tc.want {
				t.Fatalf("status mismatch: got %d, want %d (body=%s)", rec.Code, tc.want, rec.Body.String())
			}
		})
	}

	t.Run("consent withdrawn mid-tx body", func(t *testing.T) {
		rec := httptest.NewRecorder()
		mapPhotoUploadError(rec, mapperReq(), userService.ErrPhotoConsentWithdrawn)
		if rec.Code != http.StatusConflict {
			t.Fatalf("withdrawn branch got status %d, want 409", rec.Code)
		}
		if !contains(rec.Body.String(), "widerrufen") {
			t.Fatalf("withdrawn branch did not emit the German conflict message: %s", rec.Body.String())
		}
	})
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

func TestMapPhotoDeleteError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
		want int
	}{
		{"feature disabled", userService.ErrPhotoFeatureDisabled, http.StatusForbidden},
		{"student not found", userService.ErrPhotoStudentNotFound, http.StatusNotFound},
		{"student forbidden", userService.ErrPhotoStudentForbidden, http.StatusForbidden},
		{"student reassigned", userService.ErrPhotoStudentReassigned, http.StatusForbidden},
		{"no tenant context", userService.ErrPhotoNoTenant, http.StatusBadRequest},
		{"unknown error", errors.New("boom"), http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			mapPhotoDeleteError(rec, mapperReq(), tc.err)
			if rec.Code != tc.want {
				t.Fatalf("status mismatch: got %d, want %d (body=%s)", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}

func TestMapPhotoReadError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
		want int
	}{
		{"feature disabled", userService.ErrPhotoFeatureDisabled, http.StatusForbidden},
		{"student not found", userService.ErrPhotoStudentNotFound, http.StatusNotFound},
		{"student forbidden", userService.ErrPhotoStudentForbidden, http.StatusForbidden},
		{"photo not set", userService.ErrPhotoNotSet, http.StatusNotFound},
		{"filename mismatch", userService.ErrPhotoFilenameMismatch, http.StatusForbidden},
		{"no tenant context", userService.ErrPhotoNoTenant, http.StatusBadRequest},
		{"unknown error", errors.New("boom"), http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			mapPhotoReadError(rec, mapperReq(), tc.err)
			if rec.Code != tc.want {
				t.Fatalf("status mismatch: got %d, want %d (body=%s)", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}
