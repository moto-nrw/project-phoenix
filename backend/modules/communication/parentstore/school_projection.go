package parentstore

import (
	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/adapters/parentpostgres"
	recordports "github.com/moto-nrw/project-phoenix/modules/communication/ports"
)

// MessagingRepositories are the parent-messaging and announcement
// repositories a composition root hands to the retained services, next to
// the staff-messaging read cursors.
type MessagingRepositories struct {
	ParentAnnouncement  recordports.ParentAnnouncementRepository
	ParentMessageThread recordports.ParentMessageThreadRepository
	ParentMessageRead   recordports.ParentMessageReadRepository
	StaffMessageRead    communication.StaffReadRecords
}

// WithSchoolNames returns the repositories with the announcement and inbox
// school names served through schools.
func (r MessagingRepositories) WithSchoolNames(schools recordports.SchoolDirectory) MessagingRepositories {
	return MessagingRepositories{
		ParentAnnouncement:  WithAnnouncementSchoolNames(r.ParentAnnouncement, schools),
		ParentMessageThread: r.ParentMessageThread,
		ParentMessageRead:   WithInboxSchoolNames(r.ParentMessageRead, schools),
		StaffMessageRead:    r.StaffMessageRead,
	}
}
func WithInboxSchoolNames(reads recordports.ParentMessageReadRepository, schools recordports.SchoolDirectory) recordports.ParentMessageReadRepository {
	return parentpostgres.WithInboxSchoolNames(reads, schools)
}
func WithAnnouncementSchoolNames(records recordports.ParentAnnouncementRepository, schools recordports.SchoolDirectory) recordports.ParentAnnouncementRepository {
	return parentpostgres.WithAnnouncementSchoolNames(records, schools)
}
