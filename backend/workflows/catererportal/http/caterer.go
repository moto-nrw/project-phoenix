package catererportalhttp

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/mealplan"
	"github.com/moto-nrw/project-phoenix/workflows/catererportal"
)

// TokenResponse is the session a caterer's sign-in starts.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// PreviewResponse is what the acceptance page shows.
type PreviewResponse struct {
	Email             string    `json:"email"`
	CatererName       string    `json:"caterer_name"`
	SchoolName        string    `json:"school_name"`
	ExpiresAt         time.Time `json:"expires_at"`
	HasCatererAccount bool      `json:"has_caterer_account"`
	AddressTaken      bool      `json:"address_taken"`
}

// SchoolResponse is a school the caterer supplies.
type SchoolResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// SchoolsResponse is the caterer and its schools.
type SchoolsResponse struct {
	CatererName string           `json:"caterer_name"`
	Schools     []SchoolResponse `json:"schools"`
}

// ChildResponse is a child in an order; names reach the caterer only while
// the school shows them.
type ChildResponse struct {
	FirstName   string `json:"first_name,omitempty"`
	LastName    string `json:"last_name,omitempty"`
	SchoolClass string `json:"school_class,omitempty"`
}

// ChangeResponse is a numbered change after the deadline.
type ChangeResponse struct {
	Sequence   int           `json:"sequence"`
	Child      ChildResponse `json:"child"`
	Delta      int           `json:"delta"`
	DetectedAt time.Time     `json:"detected_at"`
}

// ClassCountResponse is the number of meals of a class.
type ClassCountResponse struct {
	SchoolClass string `json:"school_class"`
	Count       int    `json:"count"`
}

// DayResponse is one weekday of a school's orders.
type DayResponse struct {
	Date        string               `json:"date"`
	Closed      bool                 `json:"closed"`
	Deadline    time.Time            `json:"deadline"`
	Status      string               `json:"status"`
	OrderID     string               `json:"order_id,omitempty"`
	ConfirmedAt *time.Time           `json:"confirmed_at,omitempty"`
	Total       int                  `json:"total"`
	Classes     []ClassCountResponse `json:"classes"`
	Children    []ChildResponse      `json:"children"`
	Changes     []ChangeResponse     `json:"changes"`
}

// SchoolOrdersResponse is a school's orders as its caterer reads them.
type SchoolOrdersResponse struct {
	School          SchoolResponse `json:"school"`
	DeliveryAddress string         `json:"delivery_address"`
	FormatVersion   int            `json:"format_version"`
	Rhythm          string         `json:"rhythm"`
	Detail          string         `json:"detail"`
	Days            []DayResponse  `json:"days"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type acceptRequest struct {
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirm_password"`
}

var errLoginFieldsRequired = errors.New("email and password are required")

func schoolResponse(school catererportal.School) SchoolResponse {
	return SchoolResponse{ID: strconv.FormatInt(school.ID, 10), Name: school.Name}
}

func childResponse(child mealplan.OrderChild) ChildResponse {
	return ChildResponse{FirstName: child.FirstName, LastName: child.LastName, SchoolClass: child.SchoolClass}
}

func dayResponse(day mealplan.OrderDay) DayResponse {
	response := DayResponse{
		Date: string(day.Date), Closed: day.Closed, Deadline: day.Deadline, Status: string(day.Status),
		ConfirmedAt: day.ConfirmedAt, Total: day.Total, Classes: make([]ClassCountResponse, 0, len(day.Classes)),
		Children: make([]ChildResponse, 0, len(day.Children)), Changes: make([]ChangeResponse, 0, len(day.Changes)),
	}
	if day.OrderID > 0 {
		response.OrderID = strconv.FormatInt(day.OrderID, 10)
	}
	for _, count := range day.Classes {
		response.Classes = append(response.Classes, ClassCountResponse(count))
	}
	for _, child := range day.Children {
		response.Children = append(response.Children, childResponse(child))
	}
	for _, change := range day.Changes {
		response.Changes = append(response.Changes, ChangeResponse{Sequence: change.Sequence, Child: childResponse(change.Child), Delta: change.Delta, DetectedAt: change.DetectedAt})
	}
	return response
}

// login signs a caterer in to the caterer portal.
func (rs *Resource) login(w http.ResponseWriter, r *http.Request) {
	var request loginRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request body")))
		return
	}
	request.Email = strings.ToLower(strings.TrimSpace(request.Email))
	if request.Email == "" || request.Password == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errLoginFieldsRequired))
		return
	}
	access, refresh, err := rs.portal.LoginCatererWithAudit(r.Context(), request.Email, request.Password, common.GetClientIPString(r), r.Header.Get("User-Agent"))
	if err != nil {
		renderFailure(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, TokenResponse{AccessToken: access, RefreshToken: refresh}, "Login successful")
}

func invitationPath(r *http.Request) (int64, string, bool) {
	schoolID, ok := pathID(r, "schoolId")
	token := strings.TrimSpace(chi.URLParam(r, "token"))
	return schoolID, token, ok && token != ""
}

// previewInvitation shows what the link offers, in the school it names.
func (rs *Resource) previewInvitation(w http.ResponseWriter, r *http.Request) {
	schoolID, token, ok := invitationPath(r)
	if !ok {
		renderFailure(w, r, identityaccess.ErrInvitationNotFound)
		return
	}
	preview, err := rs.portal.PreviewInvitation(r.Context(), schoolID, token)
	if err != nil {
		renderFailure(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, PreviewResponse(preview), "Invitation retrieved")
}

// acceptInvitation spends the invitation: a new caterer sets its password,
// an existing one confirms with it.
func (rs *Resource) acceptInvitation(w http.ResponseWriter, r *http.Request) {
	schoolID, token, ok := invitationPath(r)
	if !ok {
		renderFailure(w, r, identityaccess.ErrInvitationNotFound)
		return
	}
	var request acceptRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.Password == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("password is required")))
		return
	}
	err := rs.portal.AcceptInvitation(r.Context(), schoolID, token, identityaccess.CatererRegistration{Password: request.Password, ConfirmPassword: request.ConfirmPassword})
	if err != nil {
		renderFailure(w, r, err)
		return
	}
	common.RespondNoContent(w, r)
}

// getSchools names the caterer and the schools it supplies.
func (rs *Resource) getSchools(w http.ResponseWriter, r *http.Request) {
	schools, err := rs.portal.Schools(r.Context(), rs.currentAccountID(r.Context()))
	if err != nil {
		renderFailure(w, r, err)
		return
	}
	response := SchoolsResponse{CatererName: schools.CatererName, Schools: make([]SchoolResponse, 0, len(schools.Schools))}
	for _, school := range schools.Schools {
		response.Schools = append(response.Schools, schoolResponse(school))
	}
	common.Respond(w, r, http.StatusOK, response, "Schools retrieved")
}

// getSchoolOrders returns one school's orders of [from, to].
func (rs *Resource) getSchoolOrders(w http.ResponseWriter, r *http.Request) {
	schoolID, ok := pathID(r, "schoolId")
	if !ok {
		common.RenderError(w, r, common.ErrorInvalidRequest(errInvalidID))
		return
	}
	from, fromErr := mealplan.ParseDate(r.URL.Query().Get("from"))
	to, toErr := mealplan.ParseDate(r.URL.Query().Get("to"))
	if fromErr != nil || toErr != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("from and to must be in YYYY-MM-DD format")))
		return
	}
	orders, err := rs.portal.SchoolOrders(r.Context(), rs.currentAccountID(r.Context()), schoolID, from, to)
	if err != nil {
		renderFailure(w, r, err)
		return
	}
	response := SchoolOrdersResponse{
		School: schoolResponse(orders.School), DeliveryAddress: orders.DeliveryAddress, FormatVersion: mealplan.OrderFormatVersion,
		Rhythm: orders.Orders.Rhythm, Detail: orders.Orders.Detail, Days: make([]DayResponse, 0, len(orders.Orders.Days)),
	}
	for _, day := range orders.Orders.Days {
		response.Days = append(response.Days, dayResponse(day))
	}
	common.Respond(w, r, http.StatusOK, response, "Orders retrieved")
}

// confirmOrder records that the caterer received a fixed order.
func (rs *Resource) confirmOrder(w http.ResponseWriter, r *http.Request) {
	schoolID, schoolOK := pathID(r, "schoolId")
	orderID, orderOK := pathID(r, "orderId")
	if !schoolOK || !orderOK {
		common.RenderError(w, r, common.ErrorInvalidRequest(errInvalidID))
		return
	}
	if err := rs.portal.ConfirmOrder(r.Context(), rs.currentAccountID(r.Context()), schoolID, orderID); err != nil {
		renderFailure(w, r, err)
		return
	}
	common.RespondNoContent(w, r)
}
