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
	models.ParentMessageRepository
	communication.PortalMessageReads
}
type ParentMessageThreadRepository interface {
	models.ParentMessageThreadRepository
	communication.PortalMessageThreadAccess
}
type ParentMessageReadRepository interface {
	models.ParentMessageReadRepository
	communication.PortalMessageInboxReads
}
