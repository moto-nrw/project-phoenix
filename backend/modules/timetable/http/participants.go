// Package timetablehttp — instance participants read endpoint (#2283).
//
//	GET /api/timetable/instances/{id}/participants
//
// The Leseansicht name source: schedules:read holders (every staff role) get
// the display names of the children enrolled in one instance without the
// users:read-gated tenant-wide roster. Names are filtered per student through
// securityruntime.CanReadStudent, so only verified staff (or admins) receive them.
// Filtered-out children are omitted silently — the planner shows counts from
// the instances payload either way.
package timetablehttp

import (
	"context"
	"errors"
	"net/http"
	"sort"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
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
	if rs.TimetableData == nil || rs.People == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("timetable resource not fully wired")))
		return
	}

	if _, err := rs.TimetableData.FindScheduledInstance(ctx, instanceID); err != nil {
		if errors.Is(err, timetable.ErrActivityInstanceNotFound) {
			common.RenderError(w, r, common.ErrorNotFound(errors.New("instance not found")))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServerWrap("load instance failed", err))
		return
	}

	rows, err := rs.TimetableData.ListBlockParticipants(ctx, instanceID)
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
	staffRows, err := rs.TimetableData.ListBlockStaff(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	staffIDs := make([]int64, 0, len(staffRows))
	for _, row := range staffRows {
		staffIDs = append(staffIDs, row.StaffID)
	}
	staffNames, err := rs.People.StaffNames(ctx, staffIDs)
	if err != nil {
		return nil, err
	}
	names := make([]InstanceStaffNameResponse, 0, len(staffIDs))
	for _, id := range staffIDs {
		name, ok := staffNames[id]
		if !ok {
			continue
		}
		names = append(names, InstanceStaffNameResponse{
			StaffID:     id,
			DisplayName: name,
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
func (rs *Resource) visibleParticipants(r *http.Request, rows []timetable.ScheduledParticipant) ([]InstanceParticipantResponse, error) {
	studentIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		studentIDs = append(studentIDs, row.StudentID)
	}
	names, err := rs.readableStudentNames(r, studentIDs)
	if err != nil {
		return nil, err
	}

	participants := make([]InstanceParticipantResponse, 0, len(names))
	for studentID, name := range names {
		participants = append(participants, InstanceParticipantResponse{
			StudentID:   studentID,
			DisplayName: name,
		})
	}
	sort.Slice(participants, func(i, j int) bool {
		if participants[i].DisplayName != participants[j].DisplayName {
			return participants[i].DisplayName < participants[j].DisplayName
		}
		return participants[i].StudentID < participants[j].StudentID
	})
	return participants, nil
}

// readableStudentNames returns the display names of the children the caller
// may read. Graduated children and children whose care has ended drop out:
// they are not selectable participants any more (#2487).
func (rs *Resource) readableStudentNames(r *http.Request, studentIDs []int64) (map[int64]string, error) {
	result := make(map[int64]string, len(studentIDs))
	if len(studentIDs) == 0 {
		return result, nil
	}
	ctx := r.Context()
	attending, err := rs.People.AttendingStudentPersons(ctx, studentIDs, rs.todayDate())
	if err != nil {
		return nil, err
	}
	perms := jwt.PermissionsFromCtx(ctx)
	visible := make([]int64, 0, len(attending))
	personIDs := make([]int64, 0, len(attending))
	for _, id := range studentIDs {
		personID, ok := attending[id]
		if !ok {
			continue
		}
		if !securityruntime.CanReadStudent(ctx, perms, readableStudent{}, rs.UserContextService) {
			continue
		}
		visible = append(visible, id)
		personIDs = append(personIDs, personID)
	}
	names, err := rs.People.PersonNames(ctx, personIDs)
	if err != nil {
		return nil, err
	}
	for _, studentID := range visible {
		if name, ok := names[attending[studentID]]; ok {
			result[studentID] = name
		}
	}
	return result, nil
}
