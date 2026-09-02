package staff

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

func (rs *Resource) respond(w http.ResponseWriter, r *http.Request, status int, data any, message string) {
	rs.runtime.Success(w, r, status, data, message)
	rs.runtime.ObserveResponse(status, "none")
}

func (rs *Resource) failure(w http.ResponseWriter, r *http.Request, kind FailureKind, err error, code string) {
	rs.runtime.Failure(w, r, kind, err)
	rs.runtime.ObserveResponse(statusOf(kind), code)
}

func statusOf(kind FailureKind) int {
	switch kind {
	case FailureInvalidRequest:
		return http.StatusBadRequest
	case FailureUnauthorized:
		return http.StatusUnauthorized
	case FailureForbidden:
		return http.StatusForbidden
	case FailureNotFound:
		return http.StatusNotFound
	case FailureConflict:
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

// parseStaffID reads the {id} path parameter.
func (rs *Resource) parseStaffID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		rs.failure(w, r, FailureInvalidRequest, errors.New(msgInvalidStaffID), "invalid_parameters")
		return 0, false
	}
	return id, true
}

// findStaff answers 404 for every lookup failure, exactly like the handlers
// this adapter replaces: the staff screens treat "staff member not found" as
// the single negative outcome of a profile read.
func (rs *Resource) findStaff(w http.ResponseWriter, r *http.Request, id int64) (schoolmembership.Staff, bool) {
	staff, err := rs.membership.FindStaff(r.Context(), id)
	if err != nil {
		rs.failure(w, r, FailureNotFound, errors.New(msgStaffNotFound), schoolmembership.ErrorCode(err))
		return schoolmembership.Staff{}, false
	}
	return staff, true
}

// parseAndFindStaff is the parse + lookup pair every /{id} handler starts with.
func (rs *Resource) parseAndFindStaff(w http.ResponseWriter, r *http.Request) (schoolmembership.Staff, bool) {
	id, ok := rs.parseStaffID(w, r)
	if !ok {
		return schoolmembership.Staff{}, false
	}
	return rs.findStaff(w, r, id)
}

func (rs *Resource) internal(w http.ResponseWriter, r *http.Request, err error) {
	rs.failure(w, r, FailureInternal, err, schoolmembership.ErrorCode(err))
}

// teacherByStaff resolves the optional teacher profile. A missing profile and
// a failing lookup both mean "not a teacher" — the legacy handlers swallowed
// the error the same way.
func (rs *Resource) teacherByStaff(ctx context.Context, staffID int64) *schoolmembership.Teacher {
	teacher, err := rs.membership.FindTeacherByStaff(ctx, staffID)
	if err != nil {
		return nil
	}
	return &teacher
}

// personFor loads the linked person. Best effort: a failure is logged, and
// the response goes out without the person block, as before.
func (rs *Resource) personFor(ctx context.Context, staff schoolmembership.Staff) *Person {
	if staff.PersonID <= 0 {
		return nil
	}
	person, err := rs.runtime.Person(ctx, staff.PersonID)
	if err != nil {
		rs.runtime.Log.Warn("failed to get person data for staff member",
			slog.Int64("staff_id", staff.ID),
			slog.String("error", err.Error()))
		return nil
	}
	return &person
}

// personIndex batch-loads the persons behind a staff list, keyed by person ID.
//
// A failure is fatal for the caller: the handlers this replaces read staff
// and person in one join, so a broken person lookup answered 500 rather than
// silently shrinking the list.
func (rs *Resource) personIndex(ctx context.Context, members []schoolmembership.Staff) (map[int64]*Person, error) {
	index := make(map[int64]*Person, len(members))
	ids := make([]int64, 0, len(members))
	for _, staff := range members {
		if staff.PersonID > 0 {
			ids = append(ids, staff.PersonID)
		}
	}
	if len(ids) == 0 {
		return index, nil
	}
	persons, err := rs.runtime.Persons(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range persons {
		index[persons[i].ID] = &persons[i]
	}
	return index, nil
}

// teacherIndex batch-loads the teacher profiles of a staff list, keyed by
// staff ID, so the list view stays free of the per-row lookup.
func (rs *Resource) teacherIndex(ctx context.Context, staffIDs []int64) (map[int64]schoolmembership.Teacher, error) {
	index := make(map[int64]schoolmembership.Teacher, len(staffIDs))
	if len(staffIDs) == 0 {
		return index, nil
	}
	teachers, err := rs.membership.ListTeachers(ctx, schoolmembership.TeacherFilter{StaffIDs: staffIDs})
	if err != nil {
		return nil, err
	}
	for _, teacher := range teachers {
		index[teacher.StaffID] = teacher
	}
	return index, nil
}

// stringMap runs a non-critical lookup: an error yields an empty map and a
// warning, never a failed request.
func (rs *Resource) stringMap(ctx context.Context, label string, fetch func(context.Context) (map[int64]string, error)) map[int64]string {
	values, err := fetch(ctx)
	if err != nil {
		rs.runtime.Log.Warn("failed to fetch "+label, slog.String("error", err.Error()))
		return map[int64]string{}
	}
	if values == nil {
		return map[int64]string{}
	}
	return values
}

// accountMap batch-loads one per-account string attribute for a set of staff
// members. Non-critical like stringMap.
func (rs *Resource) accountMap(ctx context.Context, accountIDs []int64, label string, fetch func(context.Context, []int64) (map[int64]string, error)) map[int64]string {
	if len(accountIDs) == 0 {
		return map[int64]string{}
	}
	return rs.stringMap(ctx, "account "+label+" map", func(ctx context.Context) (map[int64]string, error) {
		return fetch(ctx, accountIDs)
	})
}

func (rs *Resource) presentSet(ctx context.Context) map[int64]bool {
	ids, err := rs.runtime.PresentStaffIDs(ctx)
	if err != nil {
		rs.runtime.Log.Warn("failed to fetch present staff IDs", slog.String("error", err.Error()))
		return map[int64]bool{}
	}
	present := make(map[int64]bool, len(ids))
	for _, id := range ids {
		present[id] = true
	}
	return present
}

// directory holds every per-staff decoration for one request, so the list and
// the detail view enrich a row the same way.
type directory struct {
	present     map[int64]bool
	workStatus  map[int64]string
	absence     map[int64]string
	absenceLbl  map[int64]string
	accountRole map[int64]string
	email       map[int64]string
	avatar      map[int64]string
}

func (rs *Resource) loadDirectory(ctx context.Context, accountIDs []int64) directory {
	return directory{
		present:     rs.presentSet(ctx),
		workStatus:  rs.stringMap(ctx, "work status map", rs.runtime.WorkStatusMap),
		absence:     rs.stringMap(ctx, "absence map", rs.runtime.AbsenceMap),
		absenceLbl:  rs.stringMap(ctx, "absence label map", rs.runtime.AbsenceLabelMap),
		accountRole: rs.accountMap(ctx, accountIDs, "role", rs.runtime.AccountRoles),
		email:       rs.accountMap(ctx, accountIDs, "email", rs.runtime.AccountEmails),
		avatar:      rs.accountMap(ctx, accountIDs, "avatar", rs.runtime.AccountAvatars),
	}
}

// enrich builds the decoration of one staff row. The school's own absence
// wording names the reason just as AbsenceType does, so it follows the same
// personnel tier the response constructors apply (#2906).
func (d directory) enrich(staff schoolmembership.Staff, person *Person, personnel bool) enrichment {
	data := enrichment{
		present:     d.present[staff.ID],
		workStatus:  d.workStatus[staff.ID],
		absenceType: d.absence[staff.ID],
	}
	if personnel {
		data.absenceTypeLabel = d.absenceLbl[staff.ID]
	}
	if person != nil && person.AccountID != nil {
		data.accountRole = d.accountRole[*person.AccountID]
		data.email = d.email[*person.AccountID]
		data.avatar = d.avatar[*person.AccountID]
	}
	return data
}

func accountIDsOf(persons map[int64]*Person) []int64 {
	ids := make([]int64, 0, len(persons))
	for _, person := range persons {
		if person != nil && person.AccountID != nil {
			ids = append(ids, *person.AccountID)
		}
	}
	return ids
}

func staffIDsOf(members []schoolmembership.Staff) []int64 {
	ids := make([]int64, 0, len(members))
	for _, staff := range members {
		ids = append(ids, staff.ID)
	}
	return ids
}
