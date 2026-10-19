package owners

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	recordports "github.com/moto-nrw/project-phoenix/modules/communication/ports"

	parentStore "github.com/moto-nrw/project-phoenix/modules/communication/parentstore"
	staffStore "github.com/moto-nrw/project-phoenix/modules/communication/staffstore"
	guardianLinkView "github.com/moto-nrw/project-phoenix/modules/guardianlinkview/compose"
	parentAudience "github.com/moto-nrw/project-phoenix/modules/parentannouncementaudience"
	audienceRows "github.com/moto-nrw/project-phoenix/modules/parentannouncementaudience/contract"
	parentInbox "github.com/moto-nrw/project-phoenix/modules/parentmessageinbox"
	staffInbox "github.com/moto-nrw/project-phoenix/modules/staffmessageinbox"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

type announcementApplicants struct {
	source recordports.PendingApplicantSource
}

func (a announcementApplicants) PendingAnnouncementApplicants(ctx context.Context) ([]audienceRows.PendingAnnouncementApplicant, error) {
	rows, err := a.source.PendingAnnouncementApplicants(ctx)
	return applicantRows(rows), err
}

func (a announcementApplicants) PendingAnnouncementApplicantsForSchools(ctx context.Context, ids []int64) ([]audienceRows.PendingAnnouncementApplicant, error) {
	rows, err := a.source.PendingAnnouncementApplicantsForSchools(ctx, ids)
	return applicantRows(rows), err
}

func applicantRows(rows []recordports.PendingAnnouncementApplicant) []audienceRows.PendingAnnouncementApplicant {
	if rows == nil {
		return nil
	}
	values := make([]audienceRows.PendingAnnouncementApplicant, len(rows))
	for i, row := range rows {
		values[i] = audienceRows.PendingAnnouncementApplicant(row)
	}
	return values
}

func NewParentAnnouncementAudience(db transactionruntime.Database, applicants recordports.PendingApplicantSource, clocks ...func() time.Time) recordports.AnnouncementAudienceReads {
	return communicationAnnouncementAudience{source: parentAudience.New(parentStore.ProjectionDatabase(db.Pool()), announcementApplicants{source: applicants}, guardianLinkView.NewReader(), clocks...)}
}

func NewParentMessageInbox(db transactionruntime.Database) recordports.ParentInboxReads {
	return communicationParentInbox{source: parentInbox.New(parentStore.ProjectionDatabase(db.Pool()), guardianLinkView.NewReader())}
}

func NewStaffMessageInbox(db transactionruntime.Database) communication.StaffInboxReads {
	return communicationStaffInbox{source: staffInbox.New(staffStore.ProjectionDatabase(db.Pool()))}
}
