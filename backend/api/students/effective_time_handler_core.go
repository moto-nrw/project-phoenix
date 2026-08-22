package students

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/models/users"
)

const dateFormatISO = "2006-01-02"

func parseTimeOnly(timeString string) (time.Time, error) {
	return time.Parse("2006-01-02 15:04", "2000-01-01 "+timeString)
}

type careScheduleItem struct {
	// TimeOptional allows an empty Time, meaning "take it from the class
	// timetable" (#2414). Only the arrival side sets it.
	TimeOptional bool
	Weekday      int
	Time         string
	Notes        *string
}

func validateCareScheduleItem(item careScheduleItem, timeField string) error {
	if item.Weekday < schedule.WeekdayMonday || item.Weekday > schedule.WeekdayFriday {
		return errors.New("weekday must be between 1 (Monday) and 5 (Friday)")
	}
	if item.Notes != nil && len(*item.Notes) > 500 {
		return errors.New("notes cannot exceed 500 characters")
	}
	if item.Time == "" {
		// An arrival row may omit the time: it then marks the care day and
		// takes the time from the child's class timetable (#2414). Pickup
		// rows keep requiring one.
		if item.TimeOptional {
			return nil
		}
		return fmt.Errorf("%s is required", timeField)
	}
	if _, err := time.Parse("15:04", item.Time); err != nil {
		return fmt.Errorf("invalid %s format, expected HH:MM", timeField)
	}
	return nil
}

func validateCareScheduleItems[T any](
	items []T,
	timeField string,
	mapper func(T) careScheduleItem,
) error {
	seenWeekdays := make(map[int]bool)
	for index, raw := range items {
		item := mapper(raw)
		if item.Weekday < schedule.WeekdayMonday || item.Weekday > schedule.WeekdayFriday {
			return fmt.Errorf(
				"schedule %d: weekday must be between 1 (Monday) and 5 (Friday)",
				index,
			)
		}
		if seenWeekdays[item.Weekday] {
			return fmt.Errorf("schedule %d: duplicate weekday %d", index, item.Weekday)
		}
		seenWeekdays[item.Weekday] = true
		// Delegate the per-item rules instead of keeping a second copy of
		// them: the copy silently ignored TimeOptional and rejected every
		// care day without an own time (#2414).
		if err := validateCareScheduleItem(item, timeField); err != nil {
			return fmt.Errorf("schedule %d: %w", index, err)
		}
	}
	return nil
}

func validateCareExceptionRequest(
	exceptionDate string,
	value *string,
	reason *string,
	timeField string,
) error {
	if exceptionDate == "" {
		return errors.New("exception_date is required")
	}
	if _, err := time.Parse(dateFormatISO, exceptionDate); err != nil {
		return errors.New("invalid exception_date format, expected YYYY-MM-DD")
	}
	if value != nil && *value != "" {
		if _, err := time.Parse("15:04", *value); err != nil {
			return fmt.Errorf("invalid %s format, expected HH:MM", timeField)
		}
	}
	if reason != nil && len(*reason) > 255 {
		return errors.New("reason cannot exceed 255 characters")
	}
	return nil
}

func validateCareNoteRequest(noteDate string, content string) error {
	if noteDate == "" {
		return errors.New("note_date is required")
	}
	if _, err := time.Parse(dateFormatISO, noteDate); err != nil {
		return errors.New("invalid note_date format, expected YYYY-MM-DD")
	}
	if content == "" {
		return errors.New("content is required")
	}
	if len(content) > 500 {
		return errors.New("content cannot exceed 500 characters")
	}
	return nil
}

func (rs *Resource) requireCareReadAccess(
	w http.ResponseWriter,
	r *http.Request,
	domain string,
) *users.Student {
	student, ok := rs.parseAndGetStudent(w, r)
	if !ok {
		return nil
	}
	if !rs.checkStudentReadAccess(r, student) {
		renderError(
			w,
			r,
			common.ErrorForbidden(fmt.Errorf("read access required to view %s schedules", domain)),
		)
		return nil
	}
	return student
}

func (rs *Resource) requireCareWriteAccess(
	w http.ResponseWriter,
	r *http.Request,
	action string,
) *users.Student {
	student, ok := rs.parseAndGetStudent(w, r)
	if !ok {
		return nil
	}
	if !rs.checkStudentFullAccess(r, student) {
		renderError(w, r, common.ErrorForbidden(fmt.Errorf("full access required to %s", action)))
		return nil
	}
	return student
}

func verifyCareOwnership[T any](
	w http.ResponseWriter,
	r *http.Request,
	entityID int64,
	studentID int64,
	notFoundMessage string,
	wrongOwnerMessage string,
	get func(context.Context, int64) (T, error),
	owner func(T) int64,
) T {
	entity, err := get(r.Context(), entityID)
	if err != nil || reflect.ValueOf(entity).IsZero() {
		renderError(w, r, common.ErrorNotFound(errors.New(notFoundMessage)))
		var zero T
		return zero
	}
	if owner(entity) != studentID {
		renderError(
			w,
			r,
			common.ErrorForbidden(errors.New(wrongOwnerMessage)),
		)
		var zero T
		return zero
	}
	return entity
}

type BulkEffectiveTimeRequest struct {
	StudentIDs []int64 `json:"student_ids"`
	Date       *string `json:"date,omitempty"`
}

func (r *BulkEffectiveTimeRequest) Bind(_ *http.Request) error {
	if len(r.StudentIDs) == 0 {
		return errors.New("student_ids array cannot be empty")
	}
	if len(r.StudentIDs) > 500 {
		return errors.New("student_ids array cannot exceed 500 items")
	}
	if r.Date != nil && *r.Date != "" {
		if _, err := time.Parse(dateFormatISO, *r.Date); err != nil {
			return errors.New("invalid date format, expected YYYY-MM-DD")
		}
	}
	return nil
}

func handleBulkEffectiveTimes[T, R any](
	rs *Resource,
	w http.ResponseWriter,
	r *http.Request,
	successMessage string,
	fetch func(context.Context, []int64, timezone.Date) (map[int64]*T, error),
	mapper func(int64, *T) R,
) {
	req := &BulkEffectiveTimeRequest{}
	if err := render.Bind(r, req); err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	authorizedIDs, err := rs.filterAuthorizedStudentIDs(r, req.StudentIDs)
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}
	if len(authorizedIDs) == 0 {
		common.Respond(w, r, http.StatusOK, []R{}, successMessage)
		return
	}

	date := timezone.TodayDate()
	if req.Date != nil && *req.Date != "" {
		date, _ = timezone.ParseDate(*req.Date)
	}

	values, err := fetch(r.Context(), authorizedIDs, date)
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}

	response := make([]R, 0, len(values))
	for studentID, value := range values {
		response = append(response, mapper(studentID, value))
	}
	common.Respond(w, r, http.StatusOK, response, successMessage)
}
