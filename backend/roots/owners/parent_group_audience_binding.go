package owners

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	parentStore "github.com/moto-nrw/project-phoenix/modules/communication/parentstore"
	guardianLinkView "github.com/moto-nrw/project-phoenix/modules/guardianlinkview/compose"
	parentAudience "github.com/moto-nrw/project-phoenix/modules/parentannouncementaudience"
	audienceRows "github.com/moto-nrw/project-phoenix/modules/parentannouncementaudience/contract"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// NewParentGroupAudience binds the parent audience projection to the group
// threads (#4784): the same target match and reachable guardians as an
// announcement. A group thread has no pending_enrollment target, so the
// projection never asks for applicants.
func NewParentGroupAudience(db transactionruntime.Database) communication.ParentGroupAudienceReads {
	return communicationGroupAudience{source: parentAudience.New(parentStore.ProjectionDatabase(db.Pool()), noGroupApplicants{}, guardianLinkView.NewReader())}
}

type noGroupApplicants struct{}

func (noGroupApplicants) PendingAnnouncementApplicants(context.Context) ([]audienceRows.PendingAnnouncementApplicant, error) {
	return nil, nil
}

func (noGroupApplicants) PendingAnnouncementApplicantsForSchools(context.Context, []int64) ([]audienceRows.PendingAnnouncementApplicant, error) {
	return nil, nil
}

type groupAudienceSource interface {
	GroupAudience(ctx context.Context, schoolID int64, target audienceRows.GroupAudienceTarget) ([]*audienceRows.GroupAudienceChild, error)
	GroupStudents(ctx context.Context, schoolID int64, studentIDs []int64) ([]*audienceRows.GroupStudent, error)
}

type communicationGroupAudience struct{ source groupAudienceSource }

func (a communicationGroupAudience) GroupAudience(ctx context.Context, schoolID int64, target communication.ParentGroupTarget) ([]communication.ParentGroupAudienceChild, error) {
	rows, err := a.source.GroupAudience(ctx, schoolID, audienceRows.GroupAudienceTarget{Type: target.Type, RefID: target.RefID, RefText: target.RefText})
	if err != nil {
		return nil, err
	}
	out := make([]communication.ParentGroupAudienceChild, 0, len(rows))
	for _, row := range rows {
		out = append(out, communication.ParentGroupAudienceChild{
			StudentID: row.StudentID, FirstName: row.FirstName, LastName: row.LastName,
			SchoolClass: row.SchoolClass, GuardianAccountIDs: row.GuardianAccountIDs,
		})
	}
	return out, nil
}

func (a communicationGroupAudience) GroupStudents(ctx context.Context, schoolID int64, studentIDs []int64) ([]communication.ParentGroupChild, error) {
	rows, err := a.source.GroupStudents(ctx, schoolID, studentIDs)
	if err != nil {
		return nil, err
	}
	out := make([]communication.ParentGroupChild, 0, len(rows))
	for _, row := range rows {
		out = append(out, communication.ParentGroupChild{
			StudentID: row.StudentID, FirstName: row.FirstName, LastName: row.LastName, SchoolClass: row.SchoolClass,
		})
	}
	return out, nil
}
