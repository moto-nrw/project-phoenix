package parent

import (
	"net/http"
	"strconv"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
)

// ChildResponse is the wire shape for one row in the /me/children
// list. Mirrors models/parent.ChildSummary but stringifies int64 IDs
// per the project's frontend convention (Rule 4 in CLAUDE.md).
type ChildResponse struct {
	StudentID     string     `json:"student_id"`
	TenantID      string     `json:"tenant_id"`
	FirstName     string     `json:"first_name"`
	LastName      string     `json:"last_name"`
	SchoolClass   string     `json:"school_class,omitempty"`
	Status        string     `json:"status"`
	EnrolledFrom  *time.Time `json:"enrolled_from,omitempty"`
	EnrolledUntil *time.Time `json:"enrolled_until,omitempty"`
	SchoolName    string     `json:"school_name"`
	SchoolSlug    string     `json:"school_slug"`
}

func toChildResponse(c *parentModels.ChildSummary) ChildResponse {
	return ChildResponse{
		StudentID:     strconv.FormatInt(c.StudentID, 10),
		TenantID:      strconv.FormatInt(c.TenantID, 10),
		FirstName:     c.FirstName,
		LastName:      c.LastName,
		SchoolClass:   c.SchoolClass,
		Status:        c.Status,
		EnrolledFrom:  c.EnrolledFrom,
		EnrolledUntil: c.EnrolledUntil,
		SchoolName:    c.SchoolName,
		SchoolSlug:    c.SchoolSlug,
	}
}

// listMyChildren returns every student linked to the calling parent's
// account, across every active tenant mapping. Auth: parent-scope JWT
// (enforced by jwt.ParentMiddleware on the route group). The handler
// reads claims.ID for the account id — never trusts a query param or
// body field for it.
func (rs *Resource) listMyChildren(w http.ResponseWriter, r *http.Request) {
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(jwt.ErrTokenUnauthorized))
		return
	}

	children, err := rs.ParentService.ListChildrenForAccount(r.Context(), int64(claims.ID))
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	out := make([]ChildResponse, 0, len(children))
	for _, c := range children {
		out = append(out, toChildResponse(c))
	}

	common.Respond(w, r, http.StatusOK, out, "Children retrieved")
}

// EnrollablePhaseResponse is the wire shape for /me/enrollable-schools.
// Mirrors models/parent.EnrollablePhase but stringifies int64 ids.
type EnrollablePhaseResponse struct {
	SchoolID          string     `json:"school_id"`
	SchoolName        string     `json:"school_name"`
	SchoolSlug        string     `json:"school_slug"`
	PhaseID           string     `json:"phase_id"`
	PhaseName         string     `json:"phase_name"`
	PhaseKind         string     `json:"phase_kind"`
	ServiceStartDate  time.Time  `json:"service_start_date"`
	ServiceEndDate    time.Time  `json:"service_end_date"`
	EnrollmentOpenAt  *time.Time `json:"enrollment_open_at,omitempty"`
	EnrollmentCloseAt *time.Time `json:"enrollment_close_at,omitempty"`
	AlreadyLinked     bool       `json:"already_linked"`
}

// listEnrollableSchools returns every (school, open phase) pair the
// parent can submit a new enrollment to, with a flag indicating
// whether they already have a child at that school. Auth: parent
// scope, claims.ID for account_id.
func (rs *Resource) listEnrollableSchools(w http.ResponseWriter, r *http.Request) {
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(jwt.ErrTokenUnauthorized))
		return
	}

	phases, err := rs.ParentService.ListEnrollableForAccount(r.Context(), int64(claims.ID))
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	out := make([]EnrollablePhaseResponse, 0, len(phases))
	for _, p := range phases {
		out = append(out, EnrollablePhaseResponse{
			SchoolID:          strconv.FormatInt(p.SchoolID, 10),
			SchoolName:        p.SchoolName,
			SchoolSlug:        p.SchoolSlug,
			PhaseID:           strconv.FormatInt(p.PhaseID, 10),
			PhaseName:         p.PhaseName,
			PhaseKind:         p.PhaseKind,
			ServiceStartDate:  p.ServiceStartDate,
			ServiceEndDate:    p.ServiceEndDate,
			EnrollmentOpenAt:  p.EnrollmentOpenAt,
			EnrollmentCloseAt: p.EnrollmentCloseAt,
			AlreadyLinked:     p.AlreadyLinked,
		})
	}

	common.Respond(w, r, http.StatusOK, out, "Enrollable phases retrieved")
}
