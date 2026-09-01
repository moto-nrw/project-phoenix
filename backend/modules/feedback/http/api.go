// Package feedback exposes the staff Feedback HTTP adapter.
package feedback

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	feedbackModule "github.com/moto-nrw/project-phoenix/modules/feedback"
)

type Middleware = func(http.Handler) http.Handler

type Failure struct {
	Status         int
	Classification string
	Err            error
}

type Runtime struct {
	Protected       func(chi.Router, func(chi.Router, Middleware))
	Permission      func(string) Middleware
	Success         func(http.ResponseWriter, *http.Request, int, any, string)
	Failure         func(http.ResponseWriter, *http.Request, Failure)
	ObserveResponse func(int, string)
}

type Resource struct {
	module  *feedbackModule.Module
	runtime Runtime
}

func NewResource(module *feedbackModule.Module, runtime Runtime) *Resource {
	if module == nil || runtime.Protected == nil || runtime.Permission == nil || runtime.Success == nil || runtime.Failure == nil || runtime.ObserveResponse == nil {
		panic("feedback HTTP: all dependencies are required")
	}
	return &Resource{module: module, runtime: runtime}
}

func (rs *Resource) Router() chi.Router {
	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	rs.runtime.Protected(router, func(protected chi.Router, withTx Middleware) {
		protected.With(rs.runtime.Permission(permissions.FeedbackRead), withTx).Get("/", rs.listFeedback)
		protected.With(rs.runtime.Permission(permissions.FeedbackRead), withTx).Get("/{id}", rs.getFeedback)
		protected.With(rs.runtime.Permission(permissions.FeedbackRead), withTx).Get("/student/{id}", rs.getStudentFeedback)
		protected.With(rs.runtime.Permission(permissions.FeedbackRead), withTx).Get("/date/{date}", rs.getDateFeedback)
		protected.With(rs.runtime.Permission(permissions.FeedbackRead), withTx).Get("/mensa", rs.getMensaFeedback)
		protected.With(rs.runtime.Permission(permissions.FeedbackRead), withTx).Get("/date-range", rs.getDateRangeFeedback)
		protected.With(rs.runtime.Permission(permissions.FeedbackCreate), withTx).Post("/", rs.createFeedback)
		protected.With(rs.runtime.Permission(permissions.FeedbackCreate), withTx).Post("/batch", rs.createBatchFeedback)
		protected.With(rs.runtime.Permission(permissions.FeedbackDelete), withTx).Delete("/{id}", rs.deleteFeedback)
	})
	return router
}

type FeedbackResponse struct {
	ID              int64            `json:"id"`
	Value           string           `json:"value"`
	Day             string           `json:"day"`
	Time            string           `json:"time"`
	StudentID       int64            `json:"student_id"`
	IsMensaFeedback bool             `json:"is_mensa_feedback"`
	CreatedAt       time.Time        `json:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at"`
	Student         *StudentResponse `json:"student,omitempty"`
}

type StudentResponse struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

type FeedbackRequest struct {
	Value           string `json:"value"`
	Day             string `json:"day"`
	Time            string `json:"time"`
	StudentID       int64  `json:"student_id"`
	IsMensaFeedback bool   `json:"is_mensa_feedback"`
}

func (request *FeedbackRequest) Bind(*http.Request) error {
	if request.Value == "" {
		return errors.New("feedback value is required")
	}
	if request.StudentID <= 0 {
		return errors.New("student ID is required")
	}
	if request.Day == "" {
		return errors.New("day is required")
	}
	if request.Time == "" {
		return errors.New("time is required")
	}
	if _, err := feedbackModule.ParseDate(request.Day); err != nil {
		return errors.New("day must be in YYYY-MM-DD format")
	}
	if _, err := time.Parse("15:04:05", request.Time); err != nil {
		return errors.New("time must be in HH:MM:SS format")
	}
	return nil
}

type BatchFeedbackRequest struct {
	Entries []FeedbackRequest `json:"entries"`
}

func (request *BatchFeedbackRequest) Bind(r *http.Request) error {
	if len(request.Entries) == 0 {
		return errors.New("at least one feedback entry is required")
	}
	for index := range request.Entries {
		if err := request.Entries[index].Bind(r); err != nil {
			return errors.New("invalid entry at index " + strconv.Itoa(index) + ": " + err.Error())
		}
	}
	return nil
}

func (rs *Resource) listFeedback(w http.ResponseWriter, r *http.Request) {
	filter := feedbackModule.Filter{}
	if value := r.URL.Query().Get("student_id"); value != "" {
		if id, err := strconv.ParseInt(value, 10, 64); err == nil && id > 0 {
			filter.StudentID = &id
		}
	}
	if value := r.URL.Query().Get("date"); value != "" {
		if day, err := feedbackModule.ParseDate(value); err == nil {
			filter.Day = &day
		}
	}
	if value := r.URL.Query().Get("is_mensa"); value != "" {
		isMensa := value == "true" || value == "1"
		filter.IsMensaFeedback = &isMensa
	}
	entries, err := rs.module.FindEntries(r.Context(), filter)
	if err != nil {
		rs.moduleFailure(w, r, err)
		return
	}
	rs.successEntries(w, r, entries, "Feedback entries retrieved successfully")
}

func (rs *Resource) getFeedback(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		rs.invalid(w, r, errors.New("invalid feedback ID"))
		return
	}
	entry, err := rs.module.LookupEntry(r.Context(), id)
	if err != nil {
		rs.moduleFailure(w, r, err)
		return
	}
	rs.success(w, r, http.StatusOK, newFeedbackResponse(entry), "Feedback entry retrieved successfully")
}

func (rs *Resource) getStudentFeedback(w http.ResponseWriter, r *http.Request) {
	enabled, err := rs.module.Available(r.Context())
	if err != nil {
		rs.moduleFailure(w, r, err)
		return
	}
	if !enabled {
		rs.failure(w, r, Failure{Status: http.StatusForbidden, Classification: "Forbidden", Err: errors.New("feature_disabled")}, "feature_disabled")
		return
	}
	studentID, err := parseID(r)
	if err != nil {
		rs.invalid(w, r, errors.New("invalid student ID"))
		return
	}
	entries, err := rs.module.FindEntries(r.Context(), feedbackModule.Filter{StudentID: &studentID})
	if err != nil {
		rs.moduleFailure(w, r, err)
		return
	}
	rs.successEntries(w, r, entries, "Student feedback entries retrieved successfully")
}

func (rs *Resource) getDateFeedback(w http.ResponseWriter, r *http.Request) {
	day, err := feedbackModule.ParseDate(chi.URLParam(r, "date"))
	if err != nil {
		rs.invalid(w, r, errors.New("invalid date format, expected YYYY-MM-DD"))
		return
	}
	entries, err := rs.module.FindEntries(r.Context(), feedbackModule.Filter{Day: &day})
	if err != nil {
		rs.moduleFailure(w, r, err)
		return
	}
	rs.successEntries(w, r, entries, "Date feedback entries retrieved successfully")
}

func (rs *Resource) getMensaFeedback(w http.ResponseWriter, r *http.Request) {
	value := r.URL.Query().Get("is_mensa")
	isMensa := value != "false" && value != "0"
	entries, err := rs.module.FindEntries(r.Context(), feedbackModule.Filter{IsMensaFeedback: &isMensa})
	if err != nil {
		rs.moduleFailure(w, r, err)
		return
	}
	rs.successEntries(w, r, entries, "Mensa feedback entries retrieved successfully")
}

func (rs *Resource) getDateRangeFeedback(w http.ResponseWriter, r *http.Request) {
	start, err := feedbackModule.ParseDate(r.URL.Query().Get("start_date"))
	if err != nil {
		rs.invalid(w, r, errors.New("invalid start date format, expected YYYY-MM-DD"))
		return
	}
	end, err := feedbackModule.ParseDate(r.URL.Query().Get("end_date"))
	if err != nil {
		rs.invalid(w, r, errors.New("invalid end date format, expected YYYY-MM-DD"))
		return
	}
	filter := feedbackModule.Filter{DayFrom: &start, DayTo: &end}
	if value := r.URL.Query().Get("student_id"); value != "" {
		studentID, parseErr := strconv.ParseInt(value, 10, 64)
		if parseErr != nil {
			rs.invalid(w, r, errors.New("invalid student ID"))
			return
		}
		filter.StudentID = &studentID
	}
	entries, err := rs.module.FindEntries(r.Context(), filter)
	if err != nil {
		rs.moduleFailure(w, r, err)
		return
	}
	rs.successEntries(w, r, entries, "Date range feedback entries retrieved successfully")
}

func (rs *Resource) createFeedback(w http.ResponseWriter, r *http.Request) {
	request := &FeedbackRequest{}
	if err := render.Bind(r, request); err != nil {
		rs.invalid(w, r, err)
		return
	}
	entry, err := rs.module.Submit(r.Context(), request.toInput())
	if err != nil {
		rs.moduleFailure(w, r, err)
		return
	}
	rs.success(w, r, http.StatusCreated, newFeedbackResponse(entry), "Feedback entry created successfully")
}

func (rs *Resource) createBatchFeedback(w http.ResponseWriter, r *http.Request) {
	request := &BatchFeedbackRequest{}
	if err := render.Bind(r, request); err != nil {
		rs.invalid(w, r, err)
		return
	}
	inputs := make([]feedbackModule.CreateEntry, 0, len(request.Entries))
	for index := range request.Entries {
		inputs = append(inputs, request.Entries[index].toInput())
	}
	entries, err := rs.module.SubmitBatch(r.Context(), inputs)
	if err != nil {
		var batchErr *feedbackModule.BatchOperationError
		if errors.As(err, &batchErr) {
			messages := make([]string, 0, len(batchErr.Errors))
			for _, failure := range batchErr.Errors {
				messages = append(messages, failure.Error())
			}
			rs.successWithCode(w, r, http.StatusPartialContent, map[string]any{"errors": messages}, "Some feedback entries could not be created", "batch_partial")
			return
		}
		rs.moduleFailure(w, r, err)
		return
	}
	rs.success(w, r, http.StatusCreated, map[string]any{"count": len(entries)}, "Feedback entries created successfully")
}

func (rs *Resource) deleteFeedback(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		rs.invalid(w, r, errors.New("invalid feedback ID"))
		return
	}
	if err := rs.module.EraseEntry(r.Context(), id); err != nil {
		rs.moduleFailure(w, r, err)
		return
	}
	rs.success(w, r, http.StatusOK, nil, "Feedback entry deleted successfully")
}

func (request FeedbackRequest) toInput() feedbackModule.CreateEntry {
	return feedbackModule.CreateEntry{
		Value: request.Value, Day: feedbackModule.Date(request.Day), Time: request.Time,
		StudentID: request.StudentID, IsMensaFeedback: request.IsMensaFeedback,
	}
}

func parseID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("invalid ID")
	}
	return id, nil
}

func newFeedbackResponse(entry feedbackModule.Entry) FeedbackResponse {
	response := FeedbackResponse{
		ID: entry.ID, Value: entry.Value, Day: string(entry.Day), Time: entry.Time, StudentID: entry.StudentID,
		IsMensaFeedback: entry.IsMensaFeedback, CreatedAt: entry.CreatedAt, UpdatedAt: entry.UpdatedAt,
	}
	if entry.Student != nil {
		response.Student = &StudentResponse{ID: entry.Student.ID, FirstName: entry.Student.FirstName, LastName: entry.Student.LastName}
	}
	return response
}

func (rs *Resource) successEntries(w http.ResponseWriter, r *http.Request, entries []feedbackModule.Entry, message string) {
	responses := make([]FeedbackResponse, 0, len(entries))
	for _, entry := range entries {
		responses = append(responses, newFeedbackResponse(entry))
	}
	rs.success(w, r, http.StatusOK, responses, message)
}

func (rs *Resource) invalid(w http.ResponseWriter, r *http.Request, err error) {
	rs.failure(w, r, Failure{Status: http.StatusBadRequest, Classification: "Invalid Request", Err: err}, "invalid_parameters")
}

func (rs *Resource) moduleFailure(w http.ResponseWriter, r *http.Request, err error) {
	status, classification := classifyModuleError(err)
	rs.failure(w, r, Failure{Status: status, Classification: classification, Err: err}, feedbackModule.ErrorCode(err))
}

func (rs *Resource) success(w http.ResponseWriter, r *http.Request, status int, data any, message string) {
	rs.successWithCode(w, r, status, data, message, "none")
}

func (rs *Resource) successWithCode(w http.ResponseWriter, r *http.Request, status int, data any, message, code string) {
	rs.runtime.Success(w, r, status, data, message)
	rs.runtime.ObserveResponse(status, code)
}

func (rs *Resource) failure(w http.ResponseWriter, r *http.Request, failure Failure, code string) {
	rs.runtime.Failure(w, r, failure)
	rs.runtime.ObserveResponse(failure.Status, code)
}

func classifyModuleError(err error) (int, string) {
	switch {
	case errors.Is(err, feedbackModule.ErrEntryNotFound):
		return http.StatusNotFound, "Resource Not Found"
	case errors.Is(err, feedbackModule.ErrInvalidEntryData):
		return http.StatusBadRequest, "Invalid Feedback Data"
	case errors.Is(err, feedbackModule.ErrInvalidDateRange):
		return http.StatusBadRequest, "Invalid Date Range"
	case errors.Is(err, feedbackModule.ErrStudentNotFound):
		return http.StatusNotFound, "Student Not Found"
	default:
		return http.StatusInternalServerError, "Internal Server Error"
	}
}
