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
