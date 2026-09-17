package api

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	apiCommon "github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/api/students"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	schoolMembershipModule "github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	classListHTTP "github.com/moto-nrw/project-phoenix/modules/schoolmembership/http/classlistentries"
	"github.com/moto-nrw/project-phoenix/observability"
	"github.com/uptrace/bun"
)

// The class-list entries under /api/class-list-entries are served by the
// School Membership class-list adapter (#2668, #3355). Reads, the audited
// write flows, the student-match hint and the display order all go through
// the owner capability; this root only supplies rendering, auth and the JWT
// identity.

// newClassListEntriesResource binds the adapter to the shared renderer and
// the JWT identity.
func newClassListEntriesResource(entries schoolMembershipModule.ClassListEntries, db *bun.DB, logger *slog.Logger) *classListHTTP.Resource {
	return classListHTTP.NewResource(entries, classListHTTP.Runtime{
		Protected: func(router chi.Router, register func(chi.Router, classListHTTP.Middleware)) {
			apiCommon.ProtectedTenantGroup(router, db, func(protected chi.Router, withTx apiCommon.Middleware) {
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
		apiCommon.RenderError(w, r, apiCommon.ErrorInvalidRequest(err))
	case classListHTTP.FailureNotFound:
		apiCommon.RenderError(w, r, apiCommon.ErrorNotFound(err))
	default:
		apiCommon.RenderError(w, r, apiCommon.ErrorInternalServer(err))
	}
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
