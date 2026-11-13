package parent

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/moto-ogs/project-phoenix/api/common"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/sessiontoken"
	timezone "github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/moto-ogs/project-phoenix/tenant"
	parentService "github.com/moto-ogs/project-phoenix/workflows/parentportal"
)

// ErrHolidayRegistrationLinkNotFound reports a holiday registration link
// without a published period of an active Träger.
var ErrHolidayRegistrationLinkNotFound = errors.New("holiday registration link not found")

// HolidayRegistrationLinks is the consumer-owned port over Enrollment's link
// resolution of the public holiday registration (ADR 0110): the Träger of a
// published period's link, the period with its holiday care wish rule
// (#4703) and the Träger's schools, read in the caller's administrative
// transaction. The composition root binds it; an unknown link, a draft or an
// inactive Träger answers ErrHolidayRegistrationLinkNotFound.
type HolidayRegistrationLinks interface {
	ResolveHolidayRegistrationSchools(ctx context.Context, token string) (parentService.HolidayRegistrationScope, error)
}

// HolidayRegistrationGuardianResponse is the prefill of the parent fields.
// address is the parent's postal address as the school knows it (#6682).
type HolidayRegistrationGuardianResponse struct {
	FirstName string                              `json:"first_name"`
	LastName  string                              `json:"last_name"`
	Email     string                              `json:"email"`
	Phone     string                              `json:"phone"`
	Address   *HolidayRegistrationAddressResponse `json:"address,omitempty"`
}

// HolidayRegistrationAddressResponse is a postal address of the prefill.
type HolidayRegistrationAddressResponse struct {
	Street     string `json:"street"`
	PostalCode string `json:"postal_code"`
	City       string `json:"city"`
}

// HolidayRegistrationContactResponse is another guardian of a child the
// form may offer as the second custodian (full_guardian) or as an emergency
// or pickup contact (#6682). email and phone are absent when the school
// knows none.
type HolidayRegistrationContactResponse struct {
	GuardianProfileID string `json:"guardian_profile_id"`
	FirstName         string `json:"first_name"`
	LastName          string `json:"last_name"`
	Email             string `json:"email,omitempty"`
	Phone             string `json:"phone,omitempty"`
	RelationshipType  string `json:"relationship_type"`
	FullGuardian      bool   `json:"full_guardian"`
	Emergency         bool   `json:"emergency"`
	Pickup            bool   `json:"pickup"`
}

// HolidayRegistrationChildResponse is a child the parent may pick.
// school_id is the school of the Träger the registration names.
// holiday_wish and holiday_wish_proof state the child's holiday care wish
// when the period takes it into account (#4703): "yes", "no" or empty, and
// the sealed statement the form sends back with the child.
type HolidayRegistrationChildResponse struct {
	StudentID        string         `json:"student_id"`
	SchoolID         string         `json:"school_id"`
	FirstName        string         `json:"first_name"`
	LastName         string         `json:"last_name"`
	BirthDate        *timezone.Date `json:"birth_date,omitempty"`
	HolidayWish      string         `json:"holiday_wish,omitempty"`
	HolidayWishProof string         `json:"holiday_wish_proof,omitempty"`
	// Contacts are the child's other guardians (#6682); never health data.
	Contacts []HolidayRegistrationContactResponse `json:"contacts"`
}

// HolidayRegistrationFamilyResponse is the body of GET
// /parent/holiday-registrations/{token}/family. account_proof is the sealed
// proof of the signed-in account the form sends back (ADR 0216); children
// may be empty, the proof is there for every account.
type HolidayRegistrationFamilyResponse struct {
	Guardian     HolidayRegistrationGuardianResponse `json:"guardian"`
	Children     []HolidayRegistrationChildResponse  `json:"children"`
	AccountProof string                              `json:"account_proof"`
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
	scope, err := rs.resolveHolidayRegistrationLink(r.Context(), strings.TrimSpace(chi.URLParam(r, "token")))
	if errors.Is(err, ErrHolidayRegistrationLinkNotFound) {
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeEnrollmentHolidayRegistrationNotFound))
		return
	}
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	family, err := rs.ParentService.HolidayRegistrationFamily(r.Context(), int64(claims.ID), scope)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, holidayRegistrationFamilyResponse(claims, family), "Holiday registration family retrieved")
}

// resolveHolidayRegistrationLink resolves the link in the administrative
// transaction, like the school slug of the embedded enrollment
// (ADR 0113): it reads only the Träger of the link and its schools.
func (rs *Resource) resolveHolidayRegistrationLink(ctx context.Context, token string) (parentService.HolidayRegistrationScope, error) {
	var scope parentService.HolidayRegistrationScope
	err := tenant.WithinAdmin(ctx, func(adminCtx context.Context) error {
		var resolveErr error
		scope, resolveErr = rs.HolidayLinks.ResolveHolidayRegistrationSchools(adminCtx, token)
		return resolveErr
	})
	return scope, err
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
		if address := profile.Address; address != nil {
			guardian.Address = &HolidayRegistrationAddressResponse{Street: address.Street, PostalCode: address.PostalCode, City: address.City}
		}
	}
	children := make([]HolidayRegistrationChildResponse, 0, len(family.Children))
	for _, child := range family.Children {
		children = append(children, HolidayRegistrationChildResponse{
			StudentID: strconv.FormatInt(child.StudentID, 10), SchoolID: strconv.FormatInt(child.SchoolID, 10),
			FirstName: child.FirstName, LastName: child.LastName, BirthDate: child.BirthDate,
			HolidayWish: child.HolidayWish, HolidayWishProof: child.HolidayWishProof,
			Contacts: holidayRegistrationContactsResponse(child.Contacts),
		})
	}
	return HolidayRegistrationFamilyResponse{Guardian: guardian, Children: children, AccountProof: family.AccountProof}
}

func holidayRegistrationContactsResponse(contacts []parentService.HolidayRegistrationContact) []HolidayRegistrationContactResponse {
	out := make([]HolidayRegistrationContactResponse, 0, len(contacts))
	for _, contact := range contacts {
		out = append(out, HolidayRegistrationContactResponse{
			GuardianProfileID: strconv.FormatInt(contact.GuardianProfileID, 10),
			FirstName:         contact.FirstName, LastName: contact.LastName, Email: contact.Email, Phone: contact.Phone,
			RelationshipType: contact.RelationshipType, FullGuardian: contact.FullGuardian,
			Emergency: contact.Emergency, Pickup: contact.Pickup,
		})
	}
	return out
}

func firstNonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

// HolidayRegistrationOfferChildResponse names a child of the account an
// offer is for, by what the account's child list already shows.
type HolidayRegistrationOfferChildResponse struct {
	StudentID string `json:"student_id"`
	FirstName string `json:"first_name"`
}

// HolidayRegistrationOfferResponse is a holiday period on the start page of
// the parents app (#6671). token opens the registration form; open is false
// before registration_opens_on. first_day and last_day span the care weeks
// and are absent while the period has none.
type HolidayRegistrationOfferResponse struct {
	Token                string                                  `json:"token"`
	Name                 string                                  `json:"name"`
	TraegerName          string                                  `json:"traeger_name"`
	RegistrationOpensOn  timezone.Date                           `json:"registration_opens_on"`
	RegistrationClosesOn timezone.Date                           `json:"registration_closes_on"`
	FirstDay             *timezone.Date                          `json:"first_day,omitempty"`
	LastDay              *timezone.Date                          `json:"last_day,omitempty"`
	Open                 bool                                    `json:"open"`
	Children             []HolidayRegistrationOfferChildResponse `json:"children"`
}

// listHolidayRegistrationOffers serves GET /parent/me/holiday-registrations
// (#6671): the published holiday periods of the Träger of the account's
// children whose registration has not closed. The account comes from the
// claims; the workflow offers a period only for children the account may
// register.
func (rs *Resource) listHolidayRegistrationOffers(w http.ResponseWriter, r *http.Request) {
	if rs.ParentService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("parent holiday registration offers not configured")))
		return
	}
	claims := sessiontoken.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(sessiontoken.ErrTokenUnauthorized))
		return
	}
	offers, err := rs.ParentService.HolidayRegistrationOffers(r.Context(), int64(claims.ID))
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, holidayRegistrationOfferResponses(offers), "Holiday registration offers retrieved")
}

func holidayRegistrationOfferResponses(offers []parentService.HolidayRegistrationOffer) []HolidayRegistrationOfferResponse {
	responses := make([]HolidayRegistrationOfferResponse, 0, len(offers))
	for _, offer := range offers {
		response := HolidayRegistrationOfferResponse{
			Token: offer.Token, Name: offer.Name, TraegerName: offer.TraegerName,
			RegistrationOpensOn: offer.RegistrationOpensOn, RegistrationClosesOn: offer.RegistrationClosesOn,
			Open: offer.Open, Children: make([]HolidayRegistrationOfferChildResponse, 0, len(offer.Children)),
		}
		if !offer.FirstDay.IsZero() && !offer.LastDay.IsZero() {
			first, last := offer.FirstDay, offer.LastDay
			response.FirstDay, response.LastDay = &first, &last
		}
		for _, child := range offer.Children {
			response.Children = append(response.Children, HolidayRegistrationOfferChildResponse{
				StudentID: strconv.FormatInt(child.StudentID, 10), FirstName: child.FirstName,
			})
		}
		responses = append(responses, response)
	}
	return responses
}

// HolidayRegistrationEntryWeekResponse is a booked week of a child.
type HolidayRegistrationEntryWeekResponse struct {
	FirstDay      timezone.Date `json:"first_day"`
	LastDay       timezone.Date `json:"last_day"`
	TimeModelName string        `json:"time_model_name"`
	Placement     string        `json:"placement"`
}

// HolidayRegistrationEntryChildResponse is a child of a registration by its
// first name.
type HolidayRegistrationEntryChildResponse struct {
	FirstName string                                 `json:"first_name"`
	Weeks     []HolidayRegistrationEntryWeekResponse `json:"weeks"`
}

// HolidayRegistrationEntryResponse is a holiday registration linked to the
// account. first_day and last_day span the care weeks and are null while
// the period has none.
type HolidayRegistrationEntryResponse struct {
	ID          int64                                   `json:"id"`
	TraegerName string                                  `json:"traeger_name"`
	PeriodName  string                                  `json:"period_name"`
	FirstDay    *timezone.Date                          `json:"first_day"`
	LastDay     *timezone.Date                          `json:"last_day"`
	SubmittedAt time.Time                               `json:"submitted_at"`
	Status      string                                  `json:"status"`
	Children    []HolidayRegistrationEntryChildResponse `json:"children"`
}

// HolidayRegistrationEntriesResponse is the body of GET
// /parent/me/holiday-registration-entries.
type HolidayRegistrationEntriesResponse struct {
	Registrations []HolidayRegistrationEntryResponse `json:"registrations"`
}

// listHolidayRegistrationEntries serves GET
// /parent/me/holiday-registration-entries (ADR 0216 point 9): the holiday
// registrations linked to the account, newest first. The account comes
// from the claims only.
func (rs *Resource) listHolidayRegistrationEntries(w http.ResponseWriter, r *http.Request) {
	if rs.ParentService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("parent holiday registration entries not configured")))
		return
	}
	claims := sessiontoken.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(sessiontoken.ErrTokenUnauthorized))
		return
	}
	entries, err := rs.ParentService.HolidayRegistrationEntries(r.Context(), int64(claims.ID))
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, holidayRegistrationEntriesResponse(entries), "Holiday registration entries retrieved")
}

func holidayRegistrationEntriesResponse(entries []parentService.HolidayRegistrationEntry) HolidayRegistrationEntriesResponse {
	registrations := make([]HolidayRegistrationEntryResponse, 0, len(entries))
	for _, entry := range entries {
		response := HolidayRegistrationEntryResponse{
			ID: entry.ID, TraegerName: entry.TraegerName, PeriodName: entry.PeriodName, SubmittedAt: entry.SubmittedAt,
			Status: entry.Status, Children: make([]HolidayRegistrationEntryChildResponse, 0, len(entry.Children)),
		}
		if !entry.FirstDay.IsZero() && !entry.LastDay.IsZero() {
			first, last := entry.FirstDay, entry.LastDay
			response.FirstDay, response.LastDay = &first, &last
		}
		for _, child := range entry.Children {
			weeks := make([]HolidayRegistrationEntryWeekResponse, 0, len(child.Weeks))
			for _, week := range child.Weeks {
				weeks = append(weeks, HolidayRegistrationEntryWeekResponse(week))
			}
			response.Children = append(response.Children, HolidayRegistrationEntryChildResponse{FirstName: child.FirstName, Weeks: weeks})
		}
		registrations = append(registrations, response)
	}
	return HolidayRegistrationEntriesResponse{Registrations: registrations}
}
