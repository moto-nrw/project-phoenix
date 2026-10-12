package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	apiCommon "github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/inbound/students"
	schoolMembershipModule "github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	classListHTTP "github.com/moto-nrw/project-phoenix/modules/schoolmembership/http/classlistentries"
	"github.com/moto-nrw/project-phoenix/observability"
)

// The class-list entries under /api/class-list-entries are served by the
// School Membership class-list adapter (#2668, #3355). Reads, the audited
// write flows, the student-match hint and the display order all go through
// the owner capability; this root only supplies rendering, auth and the JWT
// identity.

// newClassListEntriesResource binds the adapter to the shared renderer and
// the JWT identity.
func newClassListEntriesResource(entries schoolMembershipModule.ClassListEntries, logger *slog.Logger) *classListHTTP.Resource {
	return classListHTTP.NewResource(entries, classListHTTP.Runtime{
		Protected: func(router chi.Router, register func(chi.Router, classListHTTP.Middleware)) {
			apiCommon.ProtectedTenantRoutes(router, func(protected chi.Router, withTx apiCommon.Middleware) {
				register(protected, withTx)
			})
		},
		Permission:       apiCommon.RequiresPermission,
		Success:          apiCommon.Respond,
		Failure:          renderClassListEntryFailure,
		ObserveResponse:  observability.ObserveSchoolMembershipHTTPResponse,
		CurrentAccountID: func(ctx context.Context) int64 { return int64(jwt.ClaimsFromCtx(ctx).ID) },

		Log: logger,
	})
}

// renderClassListEntryFailure writes the shared error shape for a classified
// failure. It does not observe: the adapter records every response itself.
func renderClassListEntryFailure(w http.ResponseWriter, r *http.Request, kind classListHTTP.FailureKind, err error) {
	switch kind {
	case classListHTTP.FailureInvalidRequest:
		if code := classListEntryRefusalCode(err); code != "" {
			apiCommon.RenderError(w, r, apiCommon.ErrorInvalidRequestWithCode(err, code))
			return
		}
		apiCommon.RenderError(w, r, apiCommon.ErrorInvalidRequest(err))
	case classListHTTP.FailureNotFound:
		apiCommon.RenderError(w, r, apiCommon.ErrorNotFound(err))
	default:
		apiCommon.RenderError(w, r, apiCommon.ErrorInternalServer(err))
	}
}

// classListEntryRefusals names the registered code of each refusal the
// class-list screen words itself (#2517). The status stays the owner's
// classification; a rejected field without its own code keeps the class code.
var classListEntryRefusals = []struct {
	err  error
	code string
}{
	{schoolMembershipModule.ErrClassListEntryDuplicate, apiCommon.CodeStudentsClassListEntryDuplicate},
	{schoolMembershipModule.ErrClassListEntryStudentExists, apiCommon.CodeStudentsClassListEntryStudentExists},
	{schoolMembershipModule.ErrClassListEntryStudentNotFound, apiCommon.CodeStudentsClassListEntryStudentNotFound},
	{schoolMembershipModule.ErrClassListEntryAssignMismatch, apiCommon.CodeStudentsClassListEntryAssignMismatch},
}

func classListEntryRefusalCode(err error) string {
	for _, refusal := range classListEntryRefusals {
		if errors.Is(err, refusal.err) {
			return refusal.code
		}
	}
	return ""
}

// classListEntryStudentsReader adapts the owner capability to the narrow
// reader api/students consumes: the "Klassenliste" export and the class
// dropdown need the entries in display order and nothing else, so the HTTP
// resource stays free of the owner's types.
type classListEntryStudentsReader struct {
	entries schoolMembershipModule.ClassListEntries
}

func (r classListEntryStudentsReader) ListClassListEntriesInDisplayOrder(ctx context.Context) ([]students.ClassListEntry, error) {
	values, err := r.entries.ListClassListEntriesInDisplayOrder(ctx, schoolMembershipModule.ClassListEntryFilter{})
	if err != nil {
		return nil, err
	}
	result := make([]students.ClassListEntry, 0, len(values))
	for _, value := range values {
		result = append(result, students.ClassListEntry{
			ID: value.ID, FirstName: value.FirstName, LastName: value.LastName, SchoolClass: value.SchoolClass,
		})
	}
	return result, nil
}
