// Package timetable — instance participants read endpoint (#2283).
//
//	GET /api/timetable/instances/{id}/participants
//
// The Leseansicht name source: schedules:read holders (every staff role) get
// the display names of the children enrolled in one instance without the
// users:read-gated tenant-wide roster. Names are filtered per student through
// authorize.CanReadStudent, so only verified staff (or admins) receive them.
// Filtered-out children are omitted silently — the planner shows counts from
// the instances payload either way.
package timetable

import (
	"context"
	"errors"
	"net/http"
	"sort"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/base"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModel "github.com/moto-nrw/project-phoenix/models/users"
)

// InstanceParticipantResponse is one visible child in an instance.
type InstanceParticipantResponse struct {
	StudentID   int64  `json:"student_id"`
	DisplayName string `json:"display_name"`
}

// InstanceStaffNameResponse is one assigned staff member's display name.
type InstanceStaffNameResponse struct {
	StaffID     int64  `json:"staff_id"`
	DisplayName string `json:"display_name"`
}

// InstanceParticipantsResponse is the wire shape of the participants list.
// Staff names are included unfiltered: within a team, who supervises which
// block is exactly the overview the Leseansicht exists for; only child names
// run through the CanReadStudent scope filter.
type InstanceParticipantsResponse struct {
	InstanceID   int64                         `json:"instance_id"`
	Participants []InstanceParticipantResponse `json:"participants"`
	Staff        []InstanceStaffNameResponse   `json:"staff"`
}

// getInstanceParticipants handles GET /instances/{id}/participants.
func (rs *Resource) getInstanceParticipants(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	instanceID, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid instance id")))
		return
	}
	if rs.TimetableData == nil || rs.PersonService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("timetable resource not fully wired")))
		return
	}

	if _, err := rs.TimetableData.GetActivityInstance(ctx, instanceID); err != nil {
		if base.IsNoRows(err) {
			common.RenderError(w, r, common.ErrorNotFound(errors.New("instance not found")))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServerWrap("load instance failed", err))
		return
	}

	rows, err := rs.TimetableData.GetInstanceStudents(ctx, instanceID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("load instance students failed", err))
		return
	}

	participants, err := rs.visibleParticipants(r, rows)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("load participants failed", err))
		return
	}

	staffNames, err := rs.instanceStaffNames(ctx, instanceID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("load staff names failed", err))
		return
	}

	resp := InstanceParticipantsResponse{
		InstanceID:   instanceID,
		Participants: participants,
		Staff:        staffNames,
	}
	common.Respond(w, r, http.StatusOK, resp, "Instance participants retrieved")
}

// instanceStaffNames resolves the display names of the staff assigned to the
// instance. Deliberately unfiltered (see InstanceParticipantsResponse).
func (rs *Resource) instanceStaffNames(ctx context.Context, instanceID int64) ([]InstanceStaffNameResponse, error) {
	staffRows, err := rs.TimetableData.GetInstanceStaff(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	staffIDs := make([]int64, 0, len(staffRows))
	for _, row := range staffRows {
		staffIDs = append(staffIDs, row.StaffID)
	}
	staffByID, err := rs.PersonService.GetStaffWithPersonByIDs(ctx, staffIDs)
	if err != nil {
		return nil, err
	}
	names := make([]InstanceStaffNameResponse, 0, len(staffIDs))
	for _, id := range staffIDs {
		staff := staffByID[id]
		if staff == nil || staff.Person == nil {
			continue
		}
		names = append(names, InstanceStaffNameResponse{
			StaffID:     id,
			DisplayName: staff.Person.GetFullName(),
		})
	}
	sort.Slice(names, func(i, j int) bool {
		return names[i].DisplayName < names[j].DisplayName
	})
	return names, nil
}

// visibleParticipants maps enrolled-student rows to named entries, keeping
// only students the caller may read. Alumni are excluded like every other
// staff read (see resolveStudentForRead).
func (rs *Resource) visibleParticipants(r *http.Request, rows []*scheduleModel.InstanceStudent) ([]InstanceParticipantResponse, error) {
	ctx := r.Context()

	studentIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		studentIDs = append(studentIDs, row.StudentID)
	}
	students, err := rs.PersonService.GetStudentsByIDs(ctx, studentIDs)
	if err != nil {
		return nil, err
	}

	perms := jwt.PermissionsFromCtx(ctx)
	visible := make([]*usersModel.Student, 0, len(students))
	personIDs := make([]int64, 0, len(students))
	for _, id := range studentIDs {
		student := students[id]
		if student == nil || student.Status == usersModel.StudentStatusAlumnus {
			continue
		}
		if !authorize.CanReadStudent(ctx, perms, student, rs.UserContextService) {
			continue
		}
		visible = append(visible, student)
		personIDs = append(personIDs, student.PersonID)
	}

	persons, err := rs.PersonService.GetByIDs(ctx, personIDs)
	if err != nil {
		return nil, err
	}

	participants := make([]InstanceParticipantResponse, 0, len(visible))
	for _, student := range visible {
		person := persons[student.PersonID]
		if person == nil {
			continue
		}
		participants = append(participants, InstanceParticipantResponse{
			StudentID:   student.ID,
			DisplayName: person.GetFullName(),
		})
	}
	sort.Slice(participants, func(i, j int) bool {
		return participants[i].DisplayName < participants[j].DisplayName
	})
	return participants, nil
}
