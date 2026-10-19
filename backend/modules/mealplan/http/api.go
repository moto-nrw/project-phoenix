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
	ModuleFailure  func(http.ResponseWriter, *http.Request, error, string)
	// RenderList renders a list export as PDF or XLSX.
	RenderList func(mealplanModule.ListExport, string) (ExportFile, error)
	// CurrentAccountID is the staff account of the request, recorded on a
	// school's change.
	CurrentAccountID func(context.Context) int64
}

type Resource struct {
	module  *mealplanModule.Module
	runtime Runtime
}

func NewResource(module *mealplanModule.Module, runtime Runtime) *Resource {
	if module == nil || runtime.Protected == nil || runtime.Permission == nil || runtime.Success == nil || runtime.InvalidRequest == nil || runtime.ModuleFailure == nil || runtime.RenderList == nil || runtime.CurrentAccountID == nil {
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
		protected.With(rs.runtime.Permission(AccessParticipants), withTx).Get("/participants", rs.getParticipants)
		protected.With(rs.runtime.Permission(AccessParticipants), withTx).Get("/participants/export", rs.exportParticipants)
		protected.With(rs.runtime.Permission(AccessRead), withTx).Get("/orders", rs.getOrders)
		protected.With(rs.runtime.Permission(AccessParticipants), withTx).Get("/orders/month/export", rs.exportMonth)
		protected.With(rs.runtime.Permission(AccessParticipants), withTx).Get("/participation/{studentId}", rs.getChildParticipation)
		protected.With(rs.runtime.Permission(AccessParticipants), withTx).Put("/participation/{studentId}/days/{date}", rs.setChildDay)
		protected.With(rs.runtime.Permission(AccessParticipants), withTx).Delete("/participation/{studentId}/days/{date}", rs.clearChildDay)
		protected.With(rs.runtime.Permission(AccessParticipants), withTx).Put("/participation/{studentId}/schedule", rs.replaceChildSchedule)
	})
	return router
}

type MealPlanEntryResponse struct {
	Date     string  `json:"date"`
	Position int     `json:"position"`
	Dish     string  `json:"dish"`
	Note     *string `json:"note,omitempty"`
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
	// Others are the enrolled children who do not eat that day; only with
	// include=all, for the school's late changes (#3873).
	Others []DailyParticipantResponse `json:"others,omitempty"`
}

type DishRequest struct {
	Dish string  `json:"dish"`
	Note *string `json:"note"`
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
			Date: string(entry.Date), Position: entry.Position, Dish: entry.Dish, Note: entry.Note,
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
		dishes = append(dishes, mealplanModule.Dish{Dish: dish.Dish, Note: dish.Note})
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
	if r.URL.Query().Get("include") == "all" {
		response.Others = participantResponses(list.Others)
	}
	rs.runtime.Success(w, r, http.StatusOK, response, "Meal participation list retrieved successfully")
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
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "pdf"
	}
	if format != "pdf" && format != "xlsx" {
		rs.runtime.InvalidRequest(w, r, fmt.Errorf("unsupported format %q", format))
		return
	}
	export, err := list.Export()
	if err != nil {
		rs.runtime.ModuleFailure(w, r, err, "failed to export meal participation list")
		return
	}
	file, err := rs.runtime.RenderList(export, format)
	if err != nil {
		rs.runtime.ModuleFailure(w, r, err, "failed to export meal participation list")
		return
	}
	writeFile(w, file)
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
