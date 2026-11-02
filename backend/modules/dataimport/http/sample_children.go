package importapi

import (
	"context"
	"errors"
	"net"
	"net/http"

	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
)

// Registered codes of a refused sample children request (error-registry/,
// #4658).
const (
	codeImportNotTestTraeger      = "import.not_test_traeger"
	codeImportTestTraegerInactive = "import.test_traeger_inactive"
	codeOrganizationNotFound      = "provisioning.organization_not_found"
	codeOperatorAuditUnavailable  = "operator.audit_unavailable"
)

// OperatorRuntime binds the operator session and the response rendering of
// the operator route. The operator router in front of it authenticates the
// operator and rejects writes of a read-only preview.
type OperatorRuntime struct {
	ParseID    func(*http.Request, string) (int64, error)
	OperatorID func(context.Context) int64
	ClientIP   func(*http.Request) net.IP
	Success    func(http.ResponseWriter, *http.Request, int, any, string)
	Failure    func(http.ResponseWriter, *http.Request, Failure)
}

// SampleChildrenHandler answers POST
// /operator/organizations/{id}/sample-children: it fills the test schools of
// the test Träger with invented children (#4658).
func SampleChildrenHandler(workflow importModels.SampleChildren, runtime OperatorRuntime) http.HandlerFunc {
	if workflow == nil || runtime.ParseID == nil || runtime.OperatorID == nil || runtime.ClientIP == nil || runtime.Success == nil || runtime.Failure == nil {
		panic("data import: the sample children route needs its workflow and operator runtime")
	}
	return func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := runtime.ParseID(r, "id")
		if err != nil || organizationID <= 0 {
			runtime.Failure(w, r, Failure{Status: http.StatusBadRequest, Cause: errors.New("invalid organization ID")})
			return
		}
		result, err := workflow.FillTestTraeger(r.Context(), organizationID, runtime.OperatorID(r.Context()), runtime.ClientIP(r))
		if err != nil {
			runtime.Failure(w, r, sampleChildrenFailure(err))
			return
		}
		runtime.Success(w, r, http.StatusOK, result, "Sample children imported")
	}
}

func sampleChildrenFailure(err error) Failure {
	switch {
	case errors.Is(err, importModels.ErrNotTestTraeger):
		return Failure{Status: http.StatusConflict, Code: codeImportNotTestTraeger, Cause: err}
	case errors.Is(err, importModels.ErrTestTraegerInactive):
		return Failure{Status: http.StatusConflict, Code: codeImportTestTraegerInactive, Cause: err}
	case errors.Is(err, importModels.ErrTestTraegerNotFound):
		return Failure{Status: http.StatusNotFound, Code: codeOrganizationNotFound, Cause: err}
	case errors.Is(err, importModels.ErrSampleAuditUnavailable):
		return Failure{Status: http.StatusServiceUnavailable, Code: codeOperatorAuditUnavailable, Cause: err}
	default:
		return Failure{Status: http.StatusInternalServerError, Cause: err, Message: "Beispieldaten konnten nicht angelegt werden"}
	}
}
