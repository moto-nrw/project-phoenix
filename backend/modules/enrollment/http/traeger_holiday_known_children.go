package enrollmenthttp

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/moto-ogs/project-phoenix/api/common"
	capability "github.com/moto-ogs/project-phoenix/modules/enrollment"
)

// The hint on a known child (#6683, ADR 0214):
// GET .../registrations/known-children names the registered children of the
// period, or of the one registration ?registration_id= names, that a school of
// the Träger knows. The Träger projection compares them in its
// administrative transaction before the route opens the Träger transaction;
// the route then records the comparison in the registrations' trail and
// answers only after that entry.

type holidayKnownChildrenKey struct{}

// knownChildrenResponse is the answer: the known children, each with its
// registration and its school.
type knownChildrenResponse struct {
	Children []capability.HolidayKnownChild `json:"children"`
}

var (
	errKnownChildrenRegistration = errors.New("registration_id must be a positive ID")
	errKnownChildrenMissing      = errors.New("enrollment http: the known children were not compared before the Träger transaction")
)

// knownChildrenRegistration reads the optional registration_id; 0 asks for
// the whole period.
func knownChildrenRegistration(r *http.Request) (int64, error) {
	raw := strings.TrimSpace(r.URL.Query().Get("registration_id"))
	if raw == "" {
		return 0, nil
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, errKnownChildrenRegistration
	}
	return id, nil
}

// withKnownChildren compares the registered children with the children of
// the Träger's schools before the Träger transaction opens.
func (rs *HolidayPeriodResource) withKnownChildren(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		organizationID, ok := traegerOf(w, r)
		if !ok {
			return
		}
		periodID, err := common.ParseIDParam(r, "id")
		if err != nil {
			common.RenderError(w, r, common.ErrorInvalidRequest(err))
			return
		}
		registrationID, err := knownChildrenRegistration(r)
		if err != nil {
			common.RenderError(w, r, common.ErrorInvalidRequest(err))
			return
		}
		known, err := rs.known.KnownHolidayChildren(r.Context(), organizationID, periodID, registrationID)
		if err != nil {
			common.RenderError(w, r, common.ErrorInternalServer(err))
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), holidayKnownChildrenKey{}, known)))
	})
}

// listKnownChildren records the comparison and answers the known children.
func (rs *HolidayPeriodResource) listKnownChildren(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	periodID, err := common.ParseIDParam(r, "id")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	registrationID, err := knownChildrenRegistration(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	found, ok := r.Context().Value(holidayKnownChildrenKey{}).([]capability.HolidayKnownChild)
	if !ok {
		// The comparison did not run: the route lacks withKnownChildren.
		// Recording an empty comparison would claim a read that never happened.
		common.RenderError(w, r, common.ErrorInternalServer(errKnownChildrenMissing))
		return
	}
	known, err := rs.knownTrail.RecordKnownHolidayChildren(r.Context(), reviewer, periodID, registrationID, found)
	if err != nil {
		renderHolidayReviewError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, knownChildrenResponse{Children: known}, "Known holiday children retrieved successfully")
}
