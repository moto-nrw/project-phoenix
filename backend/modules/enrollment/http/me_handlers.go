package enrollmenthttp

import (
	"errors"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// getMyProfile assembles the autofill payload for the JWT-bearing
// guardian under the tenant in context. Non-guardian sessions get the
// auth claims as guardian fields and an empty children list so the
// form still works.
func (rs *Resource) getMyProfile(w http.ResponseWriter, r *http.Request) {
	if rs.GuardianProfiles == nil {
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

	profile, err := rs.GuardianProfiles.LoadForTenant(r.Context(), accountID, tenantID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, common.BuildGuardianProfileResponse(claims, guardianProfileData(profile)), "Profile retrieved")
}

// guardianProfileData maps the already-authorized profile to the shared
// autofill values.
func guardianProfileData(profile *capability.GuardianAutofill) *common.GuardianProfileData {
	if profile == nil {
		return nil
	}
	data := &common.GuardianProfileData{
		Present: true, FirstName: profile.FirstName, LastName: profile.LastName,
		Email: profile.Email, PrimaryPhone: profile.PrimaryPhone,
		Children: make([]common.GuardianProfileChildData, 0, len(profile.Children)),
	}
	for _, child := range profile.Children {
		data.Children = append(data.Children, common.GuardianProfileChildData{
			StudentID: child.StudentID, FirstName: child.FirstName, LastName: child.LastName,
			SchoolClass: child.SchoolClass, EnrollmentSubmit: child.EnrollmentSubmit, Status: child.Status,
		})
	}
	return data
}
