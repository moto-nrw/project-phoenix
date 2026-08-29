package students

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	userService "github.com/moto-nrw/project-phoenix/services/users"
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
	if rs.FamilyProtectionService == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("family protection service not configured")))
		return
	}
	studentID, ok := common.ParsePositiveInt64IDWithError(w, r, "id", "invalid student id")
	if !ok {
		return
	}
	current, err := rs.FamilyProtectionService.Current(r.Context(), []int64{studentID})
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}
	event := current[studentID]
	response := familyProtectionResponse{StudentID: strconv.FormatInt(studentID, 10)}
	if event != nil {
		response.Enabled = event.Enabled
	}
	common.Respond(w, r, http.StatusOK, response, "Family protection retrieved")
}

func (rs *Resource) setFamilyProtection(w http.ResponseWriter, r *http.Request) {
	if rs.FamilyProtectionService == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("family protection service not configured")))
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
	claims := jwt.ClaimsFromCtx(r.Context())
	event, err := rs.FamilyProtectionService.Set(r.Context(), userService.SetFamilyProtectionInput{
		StudentID: studentID, Enabled: *body.Enabled, Reason: body.Reason, ActorAccountID: int64(claims.ID),
	})
	unchanged := errors.Is(err, userService.ErrFamilyProtectionUnchanged)
	if err != nil && !unchanged {
		renderError(w, r, familyProtectionErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, familyProtectionResponse{
		StudentID: strconv.FormatInt(event.StudentID, 10), Enabled: event.Enabled, Unchanged: unchanged,
	}, "Family protection updated")
}

var familyProtectionErrorRenderer = common.RulesRenderer([]common.ErrorRule{
	{Target: userService.ErrFamilyProtectionForbidden, Render: common.ErrorForbidden},
	{Target: userService.ErrFamilyProtectionInvalid, Render: common.ErrorInvalidRequest},
	{Target: userService.ErrFamilyProtectionNotFound, Render: common.ErrorNotFound},
}, common.ErrorInternalServer)
