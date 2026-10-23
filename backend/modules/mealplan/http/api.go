// Package mealplan exposes the staff Meal Plan HTTP adapter.
package mealplan

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

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
	Protected        func(chi.Router, func(chi.Router, Middleware))
	Permission       func(Access) Middleware
	Success          func(http.ResponseWriter, *http.Request, int, any, string)
	InvalidRequest   func(http.ResponseWriter, *http.Request, error)
	ModuleFailure    func(http.ResponseWriter, *http.Request, error, string)
	ExportDailyList  func(mealplanModule.DailyList, string) (ExportFile, error)
	ExportWeeklyList func(mealplanModule.WeeklyList, string) (ExportFile, error)
}

type Resource struct {
	module  mealplanModule.Capability
	runtime Runtime
}

func NewResource(module mealplanModule.Capability, runtime Runtime) *Resource {
	if module == nil || runtime.Protected == nil || runtime.Permission == nil || runtime.Success == nil || runtime.InvalidRequest == nil || runtime.ModuleFailure == nil || runtime.ExportDailyList == nil || runtime.ExportWeeklyList == nil {
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
		protected.With(rs.runtime.Permission(AccessParticipants), withTx).Get("/participants/week", rs.getWeeklyParticipants)
		protected.With(rs.runtime.Permission(AccessParticipants), withTx).Get("/participants/week/export", rs.exportWeeklyParticipants)
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
	Date           string                     `json:"date"`
	CutoffTime     string                     `json:"cutoff_time"`
	CutoffLeadDays int                        `json:"cutoff_lead_days"`
	Participants   []DailyParticipantResponse `json:"participants"`
}

type WeeklyDayResponse struct {
	Date         string                     `json:"date"`
	Participants []DailyParticipantResponse `json:"participants"`
}

type WeeklyListResponse struct {
	WeekStart      string              `json:"week_start"`
	CutoffTime     string              `json:"cutoff_time"`
	CutoffLeadDays int                 `json:"cutoff_lead_days"`
	Days           []WeeklyDayResponse `json:"days"`
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
	rs.runtime.Success(w, r, http.StatusOK, DailyListResponse{Date: string(list.Date), CutoffTime: list.CutoffTime, CutoffLeadDays: list.CutoffLeadDays, Participants: participantResponses(list.Participants)}, "Meal participation list retrieved successfully")
}

func (rs *Resource) getWeeklyParticipants(w http.ResponseWriter, r *http.Request) {
	week, ok := rs.weeklyList(w, r)
	if !ok {
		return
	}
	days := make([]WeeklyDayResponse, 0, len(week.Days))
	for _, day := range week.Days {
		days = append(days, WeeklyDayResponse{Date: string(day.Date), Participants: participantResponses(day.Participants)})
	}
	rs.runtime.Success(w, r, http.StatusOK, WeeklyListResponse{WeekStart: string(week.WeekStart), CutoffTime: week.CutoffTime, CutoffLeadDays: week.CutoffLeadDays, Days: days}, "Weekly meal participation list retrieved successfully")
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
	file, err := rs.runtime.ExportDailyList(list, format)
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
	file, err := rs.runtime.ExportWeeklyList(week, format)
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
	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, file.Filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(file.Data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(file.Data)
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
