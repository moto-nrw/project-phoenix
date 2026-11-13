package api

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-ogs/project-phoenix/auth/authorize/permissions"
	deliveryContract "github.com/moto-ogs/project-phoenix/modules/delivery/contract"
	peopleValues "github.com/moto-ogs/project-phoenix/modules/peopledirectory"
	studentsAPI "github.com/moto-ogs/project-phoenix/modules/peopledirectory/inbound/students"
	"github.com/moto-ogs/project-phoenix/roots/owners"
	"github.com/moto-ogs/project-phoenix/tenant"
)

// studentNoteTeamReach binds "Speichern und Team informieren" (#6854) to the
// school's staff and Delivery: the colleagues are the live staff whose active
// account may read children's notes (users:read, the right of the notes
// routes), and they hear about it through the shared staff push.
type studentNoteTeamReach struct {
	staffPusher
	staff studentNoteReaders
	now   func() time.Time
}

var _ studentsAPI.StudentNoteTeamReach = studentNoteTeamReach{}

// studentNoteReaders lists the live staff of the request's school whose
// active account holds a permission.
type studentNoteReaders interface {
	ListStaffWithPermission(ctx context.Context, permissionName string) ([]*peopleValues.StaffWithRoleInfo, error)
}

// staffPusher is the root's shared staff push (owners.StaffPushNotifier).
type staffPusher interface {
	Notify(ctx context.Context, push owners.StaffPush) error
}

// newStudentNoteTeamReach binds the reach over the staff permission facts
// and the root's notifications.
func newStudentNoteTeamReach(staff studentNoteReaders, notifications owners.DeliveryNotifications) studentNoteTeamReach {
	return studentNoteTeamReach{staffPusher: owners.NewStaffPushNotifier(notifications), staff: staff, now: time.Now}
}

func (r studentNoteTeamReach) NoteReaderAccountIDs(ctx context.Context) ([]int64, error) {
	if r.staff == nil {
		return nil, errors.New("student note team: staff directory is not configured")
	}
	rows, err := r.staff.ListStaffWithPermission(ctx, permissions.UsersRead)
	if err != nil {
		return nil, fmt.Errorf("student note team: list note readers: %w", err)
	}
	accounts := make([]int64, 0, len(rows))
	for _, row := range rows {
		if row != nil && row.AccountID > 0 {
			accounts = append(accounts, row.AccountID)
		}
	}
	return accounts, nil
}

// NotifyStudentNoteShared sends one push per announcement. Telling the team
// again about a corrected note is a new announcement, so the key carries the
// moment, not only the note.
func (r studentNoteTeamReach) NotifyStudentNoteShared(ctx context.Context, push studentsAPI.StudentNoteSharedPush) error {
	return r.Notify(ctx, owners.StaffPush{
		TenantID: tenant.FromContext(ctx), Type: deliveryContract.NotificationStaffStudentNoteShared,
		IdempotencyKey: fmt.Sprintf("student_note_shared:%d:%d", push.NoteID, r.now().UnixNano()),
		RelatedType:    "student_note", RelatedID: push.NoteID,
		Title: push.Title, Body: push.Body, DeepLink: push.DeepLink, AccountIDs: push.RecipientAccountIDs,
	})
}
