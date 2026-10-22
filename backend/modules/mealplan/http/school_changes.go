package mealplan

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	mealplanModule "github.com/moto-nrw/project-phoenix/modules/mealplan"
)

// ChildDayResponse is one meal day of a child as the school sees it.
type ChildDayResponse struct {
	Date          string    `json:"date"`
	Participating bool      `json:"participating"`
	Regular       bool      `json:"regular_participating"`
	Source        string    `json:"source"`
	CancelUntil   time.Time `json:"cancel_until"`
}

// ChildParticipationResponse is a child's registration for the school: the
// regular days and the days of the window.
type ChildParticipationResponse struct {
	Weekdays      []int              `json:"weekdays"`
	EffectiveFrom string             `json:"effective_from,omitempty"`
	Days          []ChildDayResponse `json:"days"`
}

// ChildDayRequest registers (true) or cancels (false) a day.
type ChildDayRequest struct {
	Participating *bool `json:"participating"`
}

func (request *ChildDayRequest) Bind(*http.Request) error {
	if request.Participating == nil {
		return errors.New("participating is required")
	}
	return nil
}

// ChildScheduleRequest sets the regular days from a day on.
type ChildScheduleRequest struct {
	Weekdays      []int  `json:"weekdays"`
	EffectiveFrom string `json:"effective_from"`
}

func (request *ChildScheduleRequest) Bind(*http.Request) error {
	if request.Weekdays == nil || request.EffectiveFrom == "" {
		return errors.New("weekdays and effective_from are required")
	}
	return nil
}

// getChildParticipation shows a child's registration to the school (#3873).
func (rs *Resource) getChildParticipation(w http.ResponseWriter, r *http.Request) {
	studentID, ok := rs.studentID(w, r)
	if !ok {
		return
	}
	from, fromErr := mealplanModule.ParseDate(r.URL.Query().Get("from"))
	to, toErr := mealplanModule.ParseDate(r.URL.Query().Get("to"))
	if fromErr != nil || toErr != nil {
		rs.runtime.InvalidRequest(w, r, errors.New("from and to must be in YYYY-MM-DD format"))
		return
	}
	plan, err := rs.module.Participation(r.Context(), studentID, from, to)
	if err != nil {
		rs.runtime.ModuleFailure(w, r, err, "failed to load meal participation")
		return
	}
	response := ChildParticipationResponse{EffectiveFrom: string(plan.EffectiveFrom), Weekdays: make([]int, 0, len(plan.Weekdays)), Days: make([]ChildDayResponse, 0, len(plan.Days))}
	for _, weekday := range plan.Weekdays {
		response.Weekdays = append(response.Weekdays, int(weekday))
	}
	for _, day := range plan.Days {
		response.Days = append(response.Days, ChildDayResponse{Date: string(day.Date), Participating: day.Participating, Regular: day.Regular, Source: string(day.Source), CancelUntil: day.CancelUntil})
	}
	rs.runtime.Success(w, r, http.StatusOK, response, "Meal participation retrieved successfully")
}

// setChildDay registers or cancels a child's day for the school, also after
// the deadline.
func (rs *Resource) setChildDay(w http.ResponseWriter, r *http.Request) {
	change, ok := rs.schoolChange(w, r)
	if !ok {
		return
	}
	request := &ChildDayRequest{}
	if err := render.Bind(r, request); err != nil {
		rs.runtime.InvalidRequest(w, r, err)
		return
	}
	change.Participating = *request.Participating
	if err := rs.module.SetParticipationBySchool(r.Context(), change); err != nil {
		rs.runtime.ModuleFailure(w, r, err, "failed to change meal participation")
		return
	}
	rs.runtime.Success(w, r, http.StatusOK, nil, "Meal participation changed")
}

// clearChildDay returns a child's day to the regular days.
func (rs *Resource) clearChildDay(w http.ResponseWriter, r *http.Request) {
	change, ok := rs.schoolChange(w, r)
	if !ok {
		return
	}
	if err := rs.module.ClearParticipationBySchool(r.Context(), change); err != nil {
		rs.runtime.ModuleFailure(w, r, err, "failed to change meal participation")
		return
	}
	rs.runtime.Success(w, r, http.StatusOK, nil, "Meal participation changed")
}

// replaceChildSchedule sets a child's regular days for the school.
func (rs *Resource) replaceChildSchedule(w http.ResponseWriter, r *http.Request) {
	studentID, ok := rs.studentID(w, r)
	if !ok {
		return
	}
	request := &ChildScheduleRequest{}
	if err := render.Bind(r, request); err != nil {
		rs.runtime.InvalidRequest(w, r, err)
		return
	}
	weekdays := make([]mealplanModule.Weekday, 0, len(request.Weekdays))
	for _, weekday := range request.Weekdays {
		weekdays = append(weekdays, mealplanModule.Weekday(weekday))
	}
	effectiveFrom, err := rs.module.ReplaceScheduleBySchool(r.Context(), mealplanModule.SchoolSchedule{
		StudentID: studentID, StaffAccountID: rs.runtime.CurrentAccountID(r.Context()), Weekdays: weekdays, EffectiveFrom: mealplanModule.Date(request.EffectiveFrom),
	})
	if err != nil {
		rs.runtime.ModuleFailure(w, r, err, "failed to change the regular meal days")
		return
	}
	rs.runtime.Success(w, r, http.StatusOK, map[string]string{"effective_from": string(effectiveFrom)}, "Regular meal days changed")
}

func (rs *Resource) schoolChange(w http.ResponseWriter, r *http.Request) (mealplanModule.SchoolChange, bool) {
	studentID, ok := rs.studentID(w, r)
	if !ok {
		return mealplanModule.SchoolChange{}, false
	}
	date, err := mealplanModule.ParseDate(chi.URLParam(r, "date"))
	if err != nil {
		rs.runtime.InvalidRequest(w, r, errors.New("date must be in YYYY-MM-DD format"))
		return mealplanModule.SchoolChange{}, false
	}
	return mealplanModule.SchoolChange{StudentID: studentID, StaffAccountID: rs.runtime.CurrentAccountID(r.Context()), Date: date}, true
}

func (rs *Resource) studentID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	studentID, err := rs.runtime.ParseIDParam(r, "studentId")
	if err != nil || studentID <= 0 {
		rs.runtime.InvalidRequest(w, r, errors.New("invalid student id"))
		return 0, false
	}
	return studentID, true
}
