// Package mealplan exposes the staff Meal Plan HTTP adapter.
package mealplan

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"context"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	mealplanModule "github.com/moto-nrw/project-phoenix/modules/mealplan"
)

type Middleware = func(http.Handler) http.Handler

type Access uint8

const (
	AccessRead Access = iota + 1
	AccessWrite
	AccessParticipants
	// AccessRegister changes a child's registration for the school (#3873).
	AccessRegister
	// AccessDailyParticipants reads the child list needed to change registrations.
	AccessDailyParticipants
)

type ExportFile struct {
	Data        []byte
	ContentType string
	Filename    string
}

type Runtime struct {
	Protected      func(chi.Router, func(chi.Router, Middleware))
	Permission     func(Access) Middleware
	Success        func(http.ResponseWriter, *http.Request, int, any, string)
	InvalidRequest func(http.ResponseWriter, *http.Request, error)
	// ParseIDParam reads an int64 path parameter: api/common.ParseIDParam.
	ParseIDParam  func(*http.Request, string) (int64, error)
	ModuleFailure func(http.ResponseWriter, *http.Request, error, string)
	// RenderList renders a list export as PDF or XLSX.
	RenderList func(mealplanModule.ListExport, string) (ExportFile, error)
	// CurrentAccountID is the staff account of the request, recorded on a
	// school's change.
	CurrentAccountID func(context.Context) int64
	// Allowed reports whether the caller passes an access level, for a
	// response part only some readers see.
	Allowed func(context.Context, Access) bool
}

type Resource struct {
	module  mealplanModule.Capability
	runtime Runtime
}

func NewResource(module mealplanModule.Capability, runtime Runtime) *Resource {
	if module == nil || runtime.Protected == nil || runtime.Permission == nil || runtime.Success == nil || runtime.InvalidRequest == nil || runtime.ModuleFailure == nil || runtime.RenderList == nil || runtime.CurrentAccountID == nil || runtime.Allowed == nil {
		panic("meal plan HTTP: all dependencies are required")
	}
	return &Resource{module: module, runtime: runtime}
}

func (rs *Resource) Router() chi.Router {
	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	rs.runtime.Protected(router, func(protected chi.Router, withTx Middleware) {
		protected.With(rs.runtime.Permission(AccessRead), withTx).Get("/", rs.getWeek)
		protected.With(rs.runtime.Permission(AccessWrite), withTx).Put("/{date}", rs.setDay)
		protected.With(rs.runtime.Permission(AccessWrite), withTx).Delete("/{date}", rs.deleteDay)
		protected.With(rs.runtime.Permission(AccessDailyParticipants), withTx).Get("/participants", rs.getParticipants)
		protected.With(rs.runtime.Permission(AccessParticipants), withTx).Get("/participants/export", rs.exportParticipants)
		protected.With(rs.runtime.Permission(AccessParticipants), withTx).Get("/participants/week", rs.getWeeklyParticipants)
		protected.With(rs.runtime.Permission(AccessParticipants), withTx).Get("/participants/week/export", rs.exportWeeklyParticipants)
		protected.With(rs.runtime.Permission(AccessRead), withTx).Get("/orders", rs.getOrders)
		protected.With(rs.runtime.Permission(AccessParticipants), withTx).Get("/orders/month/export", rs.exportMonth)
		protected.With(rs.runtime.Permission(AccessRegister), withTx).Get("/participation/{studentId}", rs.getChildParticipation)
		protected.With(rs.runtime.Permission(AccessRegister), withTx).Put("/participation/{studentId}/days/{date}", rs.setChildDay)
		protected.With(rs.runtime.Permission(AccessRegister), withTx).Delete("/participation/{studentId}/days/{date}", rs.clearChildDay)
		protected.With(rs.runtime.Permission(AccessRegister), withTx).Put("/participation/{studentId}/schedule", rs.replaceChildSchedule)
	})
	return router
}

type MealPlanEntryResponse struct {
	Date     string  `json:"date"`
	Position int     `json:"position"`
	Dish     string  `json:"dish"`
	Note     *string `json:"note,omitempty"`
	// Allergens are the codes of the main allergens; empty when unmarked.
	Allergens []string `json:"allergens"`
}

type DailyParticipantResponse struct {
	StudentID   string `json:"student_id"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	SchoolClass string `json:"school_class"`
}

type DailyListResponse struct {
	Date         string                     `json:"date"`
	CutoffTime   string                     `json:"cutoff_time"`
	Deadline     time.Time                  `json:"deadline"`
	Closed       bool                       `json:"closed"`
	Participants []DailyParticipantResponse `json:"participants"`
	Others       []DailyParticipantResponse `json:"others,omitempty"`
}
type WeeklyDayResponse struct {
	Date         string                     `json:"date"`
	Deadline     time.Time                  `json:"deadline"`
	Closed       bool                       `json:"closed"`
	Participants []DailyParticipantResponse `json:"participants"`
}
type WeeklyListResponse struct {
	WeekStart string              `json:"week_start"`
	Days      []WeeklyDayResponse `json:"days"`
}

type DishRequest struct {
	Dish string  `json:"dish"`
	Note *string `json:"note"`
	// Allergens are codes of the main allergens; omitted means none.
	Allergens []string `json:"allergens"`
}

type SetDayRequest struct {
	Dishes *[]DishRequest `json:"dishes"`
}

func (request *SetDayRequest) Bind(*http.Request) error {
	if request.Dishes == nil {
		return errors.New("dishes is required (use an empty array to clear the day)")
	}
	return nil
}

func (rs *Resource) getWeek(w http.ResponseWriter, r *http.Request) {
	weekStart := r.URL.Query().Get("week_start")
	parsed, err := mealplanModule.ParseDate(weekStart)
	if err != nil {
		rs.runtime.InvalidRequest(w, r, errors.New("week_start must be in YYYY-MM-DD format"))
		return
	}
	entries, err := rs.module.Week(r.Context(), parsed)
	if err != nil {
		rs.runtime.ModuleFailure(w, r, err, "failed to load meal plan")
		return
	}
	responses := make([]MealPlanEntryResponse, 0, len(entries))
	for _, entry := range entries {
		responses = append(responses, MealPlanEntryResponse{
			Date: string(entry.Date), Position: entry.Position, Dish: entry.Dish, Note: entry.Note, Allergens: entry.AllergenCodes(),
		})
	}
	rs.runtime.Success(w, r, http.StatusOK, responses, "Meal plan retrieved successfully")
}

func (rs *Resource) setDay(w http.ResponseWriter, r *http.Request) {
	date := chi.URLParam(r, "date")
	parsed, err := mealplanModule.ParseDate(date)
	if err != nil {
		rs.runtime.InvalidRequest(w, r, errors.New("date must be in YYYY-MM-DD format"))
		return
	}
	request := &SetDayRequest{}
	if err := render.Bind(r, request); err != nil {
		rs.runtime.InvalidRequest(w, r, err)
		return
	}
	dishes := make([]mealplanModule.Dish, 0, len(*request.Dishes))
	for _, dish := range *request.Dishes {
		allergens := make([]mealplanModule.Allergen, 0, len(dish.Allergens))
		for _, code := range dish.Allergens {
			allergens = append(allergens, mealplanModule.Allergen(code))
		}
		dishes = append(dishes, mealplanModule.Dish{Dish: dish.Dish, Note: dish.Note, Allergens: allergens})
	}
	if err := rs.module.ReplaceDay(r.Context(), mealplanModule.ReplaceDay{Date: parsed, Dishes: dishes}); err != nil {
		rs.runtime.ModuleFailure(w, r, err, "failed to save meal")
		return
	}
	rs.runtime.Success(w, r, http.StatusOK, nil, "Meal plan day saved successfully")
}

func (rs *Resource) deleteDay(w http.ResponseWriter, r *http.Request) {
	date := chi.URLParam(r, "date")
	parsed, err := mealplanModule.ParseDate(date)
	if err != nil {
		rs.runtime.InvalidRequest(w, r, errors.New("date must be in YYYY-MM-DD format"))
		return
	}
	if err := rs.module.ClearDay(r.Context(), parsed); err != nil {
		rs.runtime.ModuleFailure(w, r, err, "failed to delete meal")
		return
	}
	rs.runtime.Success(w, r, http.StatusOK, nil, "Meal deleted successfully")
}

func (rs *Resource) getParticipants(w http.ResponseWriter, r *http.Request) {
	list, ok := rs.dailyList(w, r)
	if !ok {
		return
	}
	response := DailyListResponse{Date: string(list.Date), CutoffTime: list.CutoffTime, Deadline: list.Deadline, Closed: list.Closed, Participants: participantResponses(list.Participants)}
	// The children who do not eat are there to register them, so only a
	// caller who may change a registration sees them.
	if r.URL.Query().Get("include") == "all" && rs.runtime.Allowed(r.Context(), AccessRegister) {
		response.Others = participantResponses(list.Others)
	}
	rs.runtime.Success(w, r, http.StatusOK, response, "Meal participation list retrieved successfully")
}

func (rs *Resource) getWeeklyParticipants(w http.ResponseWriter, r *http.Request) {
	week, ok := rs.weeklyList(w, r)
	if !ok {
		return
	}
	days := make([]WeeklyDayResponse, 0, len(week.Days))
	for _, day := range week.Days {
		days = append(days, WeeklyDayResponse{Date: string(day.Date), Deadline: day.Deadline, Closed: day.Closed, Participants: participantResponses(day.Participants)})
	}
	rs.runtime.Success(w, r, http.StatusOK, WeeklyListResponse{WeekStart: string(week.WeekStart), Days: days}, "Weekly meal participation list retrieved successfully")
}

func participantResponses(participants []mealplanModule.DailyParticipant) []DailyParticipantResponse {
	responses := make([]DailyParticipantResponse, 0, len(participants))
	for _, participant := range participants {
		responses = append(responses, DailyParticipantResponse{StudentID: strconv.FormatInt(participant.StudentID, 10), FirstName: participant.FirstName, LastName: participant.LastName, SchoolClass: participant.SchoolClass})
	}
	return responses
}

func (rs *Resource) exportParticipants(w http.ResponseWriter, r *http.Request) {
	list, ok := rs.dailyList(w, r)
	if !ok {
		return
	}
	format, ok := rs.exportFormat(w, r)
	if !ok {
		return
	}
	export, err := list.Export()
	if err != nil {
		rs.writeExport(w, r, ExportFile{}, err)
		return
	}
	file, err := rs.runtime.RenderList(export, format)
	rs.writeExport(w, r, file, err)
}

func (rs *Resource) exportWeeklyParticipants(w http.ResponseWriter, r *http.Request) {
	week, ok := rs.weeklyList(w, r)
	if !ok {
		return
	}
	format, ok := rs.exportFormat(w, r)
	if !ok {
		return
	}
	export, err := week.Export()
	if err != nil {
		rs.writeExport(w, r, ExportFile{}, err)
		return
	}
	file, err := rs.runtime.RenderList(export, format)
	rs.writeExport(w, r, file, err)
}

func (rs *Resource) exportFormat(w http.ResponseWriter, r *http.Request) (string, bool) {
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "pdf"
	}
	if format != "pdf" && format != "xlsx" {
		rs.runtime.InvalidRequest(w, r, fmt.Errorf("unsupported format %q", format))
		return "", false
	}
	return format, true
}

func (rs *Resource) writeExport(w http.ResponseWriter, r *http.Request, file ExportFile, err error) {
	if err != nil {
		rs.runtime.ModuleFailure(w, r, err, "failed to export meal participation list")
		return
	}
	writeFile(w, file)
}

func (rs *Resource) weeklyList(w http.ResponseWriter, r *http.Request) (mealplanModule.WeeklyList, bool) {
	weekStart, err := mealplanModule.ParseDate(r.URL.Query().Get("week_start"))
	if err != nil {
		rs.runtime.InvalidRequest(w, r, errors.New("week_start must be in YYYY-MM-DD format"))
		return mealplanModule.WeeklyList{}, false
	}
	week, err := rs.module.WeeklyList(r.Context(), weekStart)
	if err != nil {
		rs.runtime.ModuleFailure(w, r, err, "failed to load meal participation list")
		return mealplanModule.WeeklyList{}, false
	}
	return week, true
}

func (rs *Resource) dailyList(w http.ResponseWriter, r *http.Request) (mealplanModule.DailyList, bool) {
	date, err := mealplanModule.ParseDate(r.URL.Query().Get("date"))
	if err != nil {
		rs.runtime.InvalidRequest(w, r, errors.New("date must be in YYYY-MM-DD format"))
		return mealplanModule.DailyList{}, false
	}
	list, err := rs.module.DailyList(r.Context(), date)
	if err != nil {
		rs.runtime.ModuleFailure(w, r, err, "failed to load meal participation list")
		return mealplanModule.DailyList{}, false
	}
	return list, true
}
