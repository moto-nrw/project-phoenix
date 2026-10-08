package enrollment

import (
	"errors"
	"net/http"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	"github.com/moto-nrw/project-phoenix/tenant"
)

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

	common.Respond(w, r, http.StatusOK, common.BuildGuardianProfileResponse(claims, enrollmentProfileData(loaded)), "Profile retrieved")
}

// enrollmentProfileData maps the already-authorized loader result to HTTP values.
func enrollmentProfileData(loaded *usersModels.GuardianProfileWithChildren) *common.GuardianProfileData {
	if loaded == nil || loaded.Profile == nil {
		return nil
	}
	data := &common.GuardianProfileData{
		Present: true, FirstName: loaded.Profile.FirstName, LastName: loaded.Profile.LastName,
		Email: loaded.Profile.Email, PrimaryPhone: loaded.PrimaryPhone,
		Children: make([]common.GuardianProfileChildData, 0, len(loaded.Children)),
	}
	for _, child := range loaded.Children {
		data.Children = append(data.Children, common.GuardianProfileChildData{
			StudentID: child.StudentID, FirstName: child.FirstName, LastName: child.LastName,
			SchoolClass: child.SchoolClass, EnrollmentSubmit: child.EnrollmentSubmit, Status: child.Status,
		})
	}
	return data
}
