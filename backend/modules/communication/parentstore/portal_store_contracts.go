package parentstore

import (
	models "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/communication"
)

// The existing bound stores advertise native public facets beside retained callers.
type ParentAnnouncementRepository interface {
	models.ParentAnnouncementRepository
	communication.PortalAnnouncementAccess
}
type ParentMessageRepository interface {
	communication.ParentMessageRecords
	models.ParentMessageRepository
	communication.PortalMessageReads
}
type ParentMessageThreadRepository interface {
	communication.ParentThreadRecords
	models.ParentMessageThreadRepository
	communication.PortalMessageThreadAccess
}
type ParentMessageReadRepository interface {
	communication.ParentReadRecords
	models.ParentMessageReadRepository
	communication.PortalMessageInboxReads
}
