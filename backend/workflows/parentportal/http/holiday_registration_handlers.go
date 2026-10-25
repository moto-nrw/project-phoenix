package parent

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// ErrHolidayRegistrationLinkNotFound reports a holiday registration link
// without a published period of an active Träger.
var ErrHolidayRegistrationLinkNotFound = errors.New("holiday registration link not found")

// HolidayRegistrationLinks is the consumer-owned port over Enrollment's link
// resolution of the public holiday registration (ADR 0110): the Träger of a
// published period's link and the Träger's schools, read in the caller's
// administrative transaction. The composition root binds it; an unknown link,
// a draft or an inactive Träger answers ErrHolidayRegistrationLinkNotFound.
type HolidayRegistrationLinks interface {
	ResolveHolidayRegistrationSchools(ctx context.Context, token string) (organizationID int64, schoolIDs []int64, err error)
}

// HolidayRegistrationGuardianResponse is the prefill of the parent fields.
type HolidayRegistrationGuardianResponse struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
}

// HolidayRegistrationChildResponse is a child the parent may pick.
// school_id is the school of the Träger the registration names.
type HolidayRegistrationChildResponse struct {
	StudentID string         `json:"student_id"`
	SchoolID  string         `json:"school_id"`
	FirstName string         `json:"first_name"`
	LastName  string         `json:"last_name"`
	BirthDate *timezone.Date `json:"birth_date,omitempty"`
}

// HolidayRegistrationFamilyResponse is the body of GET
// /parent/holiday-registrations/{token}/family.
type HolidayRegistrationFamilyResponse struct {
	Guardian HolidayRegistrationGuardianResponse `json:"guardian"`
	Children []HolidayRegistrationChildResponse  `json:"children"`
}

// getHolidayRegistrationFamily serves the prefill of the holiday registration
// for a parent with an account (#4662): the children of the account at the
// schools of the link's Träger whose relationship grants the registration,
// and the guardian's contact data. The account comes from the claims, the
// Träger only from the resolved link. The registration itself stays the
// public submit of the link.
func (rs *Resource) getHolidayRegistrationFamily(w http.ResponseWriter, r *http.Request) {
	if rs.HolidayLinks == nil || rs.ParentService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("parent holiday registration family not configured")))
		return
	}
	claims := sessiontoken.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(sessiontoken.ErrTokenUnauthorized))
		return
	}
	organizationID, schoolIDs, err := rs.resolveHolidayRegistrationLink(r.Context(), strings.TrimSpace(chi.URLParam(r, "token")))
	if errors.Is(err, ErrHolidayRegistrationLinkNotFound) {
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeEnrollmentHolidayRegistrationNotFound))
		return
	}
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	family, err := rs.ParentService.HolidayRegistrationFamily(r.Context(), int64(claims.ID), organizationID, schoolIDs)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, holidayRegistrationFamilyResponse(claims, family), "Holiday registration family retrieved")
}

// resolveHolidayRegistrationLink resolves the link in the administrative
// transaction, like the school slug of the embedded enrollment
// (ADR 0113): it reads only the Träger of the link and its schools.
func (rs *Resource) resolveHolidayRegistrationLink(ctx context.Context, token string) (int64, []int64, error) {
	var organizationID int64
	var schoolIDs []int64
	err := tenant.WithinAdmin(ctx, func(adminCtx context.Context) error {
		var resolveErr error
		organizationID, schoolIDs, resolveErr = rs.HolidayLinks.ResolveHolidayRegistrationSchools(adminCtx, token)
		return resolveErr
	})
	return organizationID, schoolIDs, err
}

// holidayRegistrationFamilyResponse fills the parent fields from the
// guardian's profile and falls back to the account's claims where the
// profile has no value.
func holidayRegistrationFamilyResponse(claims sessiontoken.AppClaims, family *parentService.HolidayRegistrationFamily) HolidayRegistrationFamilyResponse {
	guardian := HolidayRegistrationGuardianResponse{FirstName: claims.FirstName, LastName: claims.LastName, Email: claims.Sub}
	if profile := family.Guardian; profile != nil {
		guardian = HolidayRegistrationGuardianResponse{
			FirstName: firstNonEmpty(profile.FirstName, guardian.FirstName), LastName: firstNonEmpty(profile.LastName, guardian.LastName),
			Email: firstNonEmpty(profile.Email, guardian.Email), Phone: profile.Phone,
		}
	}
	children := make([]HolidayRegistrationChildResponse, 0, len(family.Children))
	for _, child := range family.Children {
		children = append(children, HolidayRegistrationChildResponse{
			StudentID: strconv.FormatInt(child.StudentID, 10), SchoolID: strconv.FormatInt(child.SchoolID, 10),
			FirstName: child.FirstName, LastName: child.LastName, BirthDate: child.BirthDate,
		})
	}
	return HolidayRegistrationFamilyResponse{Guardian: guardian, Children: children}
}

func firstNonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}
