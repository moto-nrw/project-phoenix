package students

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

type setFamilyProtectionBody struct {
	Enabled *bool  `json:"enabled"`
	Reason  string `json:"reason"`
}

type familyProtectionResponse struct {
	StudentID string `json:"student_id"`
	Enabled   bool   `json:"enabled"`
	// Unchanged says the child was already in the requested state, so no new
	// ledger entry was written. The request still succeeded — repeating a
	// switch is not an error the user has to fix.
	Unchanged bool `json:"unchanged,omitempty"`
}

func (rs *Resource) getFamilyProtection(w http.ResponseWriter, r *http.Request) {
	if rs.FamilyProtection == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("family protection capability is not configured")))
		return
	}
	studentID, ok := common.ParsePositiveInt64IDWithError(w, r, "id", "invalid student id")
	if !ok {
		return
	}
	current, err := rs.FamilyProtection.CurrentFamilyProtection(r.Context(), []int64{studentID})
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, familyProtectionResponse{
		StudentID: strconv.FormatInt(studentID, 10), Enabled: current[studentID],
	}, "Family protection retrieved")
}

func (rs *Resource) setFamilyProtection(w http.ResponseWriter, r *http.Request) {
	if rs.FamilyProtection == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("family protection capability is not configured")))
		return
	}
	studentID, ok := common.ParsePositiveInt64IDWithError(w, r, "id", "invalid student id")
	if !ok {
		return
	}
	var body setFamilyProtectionBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Enabled == nil || strings.TrimSpace(body.Reason) == "" {
		renderError(w, r, common.ErrorInvalidRequest(errors.New("enabled and reason are required")))
		return
	}
	// Re-check the route's own gate inside the handler: the privacy ledger is
	// an admin decision, and the owner capability deliberately does not decide
	// who may ask.
	if !authorize.HasPermission(permissions.ConfigManage, jwt.PermissionsFromCtx(r.Context())) {
		renderError(w, r, common.ErrorForbidden(errors.New("family protection requires configuration permission")))
		return
	}
	claims := jwt.ClaimsFromCtx(r.Context())
	enabled, err := rs.FamilyProtection.SetFamilyProtection(r.Context(), peopleModule.SetFamilyProtection{
		StudentID: studentID, Enabled: *body.Enabled, Reason: body.Reason, ActorAccountID: int64(claims.ID),
	})
	unchanged := errors.Is(err, peopleModule.ErrFamilyProtectionUnchanged)
	if err != nil && !unchanged {
		renderError(w, r, familyProtectionErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, familyProtectionResponse{
		StudentID: strconv.FormatInt(studentID, 10), Enabled: enabled, Unchanged: unchanged,
	}, "Family protection updated")
}

var familyProtectionErrorRenderer = common.RulesRenderer([]common.ErrorRule{
	{Target: peopleModule.ErrFamilyProtectionInvalid, Render: common.ErrorInvalidRequest},
	{Target: peopleModule.ErrStudentNotFound, Render: common.ErrorNotFound},
}, common.ErrorInternalServer)
