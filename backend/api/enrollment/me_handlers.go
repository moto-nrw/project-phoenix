package enrollment

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// MeProfileResponse is the autofill payload for the parent enrollment
// form. It carries the guardian's account info + their linked students
// so the form can prefill guardian fields and offer one-click child
// reuse. Returns 200 with empty children when the guardian has none
// yet; 401 when no session is in scope.
type MeProfileResponse struct {
	Guardian MeGuardian `json:"guardian"`
	Children []MeChild  `json:"children"`
}

type MeGuardian struct {
	FirstName string  `json:"first_name"`
	LastName  string  `json:"last_name"`
	Email     string  `json:"email"`
	Phone     *string `json:"phone,omitempty"`
}

// MeChild only carries fields the form needs to prefill a child slot.
// Date-of-birth is intentionally absent — users.students has no DOB
// column today; parent fills it manually for the enrollment row.
type MeChild struct {
	ID          string `json:"id"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	SchoolClass string `json:"school_class"`
	GradeLevel  *int   `json:"grade_level,omitempty"`
}

// getMyProfile assembles the autofill payload for the JWT-bearing
// guardian under the tenant in context. Non-guardian sessions get the
// auth claims as guardian fields and an empty children list so the
// form still works.
func (rs *Resource) getMyProfile(w http.ResponseWriter, r *http.Request) {
	if rs.GuardianProfileLoader == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("me/profile not wired")))
		return
	}

	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorForbidden(errors.New("not authenticated")))
		return
	}
	accountID := int64(claims.ID)
	tenantID := tenant.FromContext(r.Context())
	if tenantID == 0 {
		common.RenderError(w, r, common.ErrorForbidden(errors.New("tenant context missing")))
		return
	}

	loaded, err := rs.GuardianProfileLoader.LoadForTenant(r.Context(), accountID, tenantID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, buildMeProfileResponse(claims, loaded), "Profile retrieved")
}

// buildMeProfileResponse merges claims-derived defaults with the
// (possibly nil) loaded profile. Nil loaded → defaults only with
// empty children list.
func buildMeProfileResponse(claims jwt.AppClaims, loaded *usersModels.GuardianProfileWithChildren) MeProfileResponse {
	resp := MeProfileResponse{
		Guardian: MeGuardian{
			FirstName: claims.FirstName,
			LastName:  claims.LastName,
			Email:     claims.Sub,
		},
		Children: []MeChild{},
	}
	if loaded == nil || loaded.Profile == nil {
		return resp
	}
	if loaded.Profile.FirstName != "" {
		resp.Guardian.FirstName = loaded.Profile.FirstName
	}
	if loaded.Profile.LastName != "" {
		resp.Guardian.LastName = loaded.Profile.LastName
	}
	if loaded.Profile.Email != nil && *loaded.Profile.Email != "" {
		resp.Guardian.Email = *loaded.Profile.Email
	}
	if loaded.PrimaryPhone != "" {
		phone := loaded.PrimaryPhone
		resp.Guardian.Phone = &phone
	}
	for _, child := range loaded.Children {
		entry := MeChild{
			ID:          strconv.FormatInt(child.StudentID, 10),
			FirstName:   child.FirstName,
			LastName:    child.LastName,
			SchoolClass: child.SchoolClass,
		}
		if grade := parseGradeLevel(child.SchoolClass); grade > 0 {
			entry.GradeLevel = &grade
		}
		resp.Children = append(resp.Children, entry)
	}
	return resp
}

// parseGradeLevel pulls the leading integer out of strings like "1a",
// "2b", "10" → 1, 2, 10. Returns 0 when nothing parses (e.g.
// "Vorschule"); the form treats that as "no prefill".
func parseGradeLevel(schoolClass string) int {
	digits := ""
	for _, r := range schoolClass {
		if r >= '0' && r <= '9' {
			digits += string(r)
			continue
		}
		break
	}
	if digits == "" {
		return 0
	}
	n, err := strconv.Atoi(digits)
	if err != nil {
		return 0
	}
	return n
}
