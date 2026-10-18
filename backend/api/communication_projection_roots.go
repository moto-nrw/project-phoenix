package api

import (
	"context"
	"time"

	parentStore "github.com/moto-nrw/project-phoenix/modules/communication/parentstore"
	parentAudience "github.com/moto-nrw/project-phoenix/modules/communication/projections/parentannouncementaudience"
	parentInbox "github.com/moto-nrw/project-phoenix/modules/communication/projections/parentmessageinbox"
	staffInbox "github.com/moto-nrw/project-phoenix/modules/communication/projections/staffmessageinbox"
	staffStore "github.com/moto-nrw/project-phoenix/modules/communication/staffstore"
	guardianLinkView "github.com/moto-nrw/project-phoenix/modules/guardianlinkview/compose"
	audienceRows "github.com/moto-nrw/project-phoenix/modules/parentannouncementaudience/contract"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

type announcementApplicants struct {
	source parentStore.PendingApplicantSource
}

func (a announcementApplicants) PendingAnnouncementApplicants(ctx context.Context) ([]audienceRows.PendingAnnouncementApplicant, error) {
	rows, err := a.source.PendingAnnouncementApplicants(ctx)
	return applicantRows(rows), err
}
func (a announcementApplicants) PendingAnnouncementApplicantsForSchools(ctx context.Context, ids []int64) ([]audienceRows.PendingAnnouncementApplicant, error) {
	rows, err := a.source.PendingAnnouncementApplicantsForSchools(ctx, ids)
	return applicantRows(rows), err
}
func applicantRows(rows []parentStore.PendingAnnouncementApplicant) []audienceRows.PendingAnnouncementApplicant {
	if rows == nil {
		return nil
	}
	values := make([]audienceRows.PendingAnnouncementApplicant, len(rows))
	for i, row := range rows {
		values[i] = audienceRows.PendingAnnouncementApplicant(row)
	}
	return values
}
func newParentAnnouncementAudience(db transactionruntime.Database, applicants parentStore.PendingApplicantSource, clocks ...func() time.Time) parentStore.AnnouncementAudienceReads {
	return parentAudience.New(parentStore.ProjectionDatabase(db.Pool()), announcementApplicants{source: applicants}, guardianLinkView.NewReader(), clocks...)
}
func newParentMessageInbox(db transactionruntime.Database) parentStore.ParentInbox {
	return parentInbox.New(parentStore.ProjectionDatabase(db.Pool()), guardianLinkView.NewReader())
}
func newStaffMessageInbox(db transactionruntime.Database) staffStore.StaffInbox {
	return staffInbox.New(staffStore.ProjectionDatabase(db.Pool()))
}
