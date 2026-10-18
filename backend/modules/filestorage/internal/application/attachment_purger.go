package application

import (
	"time"

	"github.com/moto-nrw/project-phoenix/modules/filestorage/internal/ports"
)

// AttachmentPurgerDependencies are the seams of the announcement side of the
// attachments: the attachment rows, their cleanup intents and objects.
type AttachmentPurgerDependencies struct {
	Attachments        ports.DocumentStore
	AttachmentCleanups ports.CleanupStore
	AttachmentObjects  ports.Objects
	Tx                 ports.Transaction
	Observe            ports.Observer
	Now                func() time.Time
}

// NewAttachmentPurger wires the service for the announcement side of the
// attachments alone: QueueAttachmentCleanupForAnnouncement, CountAttachments
// and AttachmentDigests. They need neither the announcements nor the parents
// portal, so a root builds the purger before both (#4080). The composition
// hands out only these three methods; every seam they use is required.
func NewAttachmentPurger(deps AttachmentPurgerDependencies) *Service {
	if deps.Now == nil {
		deps.Now = time.Now
	}
	if deps.Attachments == nil || deps.AttachmentCleanups == nil || deps.AttachmentObjects == nil ||
		deps.Tx == nil || deps.Observe == nil || deps.Now == nil {
		panic("file storage application: all dependencies are required")
	}
	return &Service{deps: Dependencies{
		Attachments: deps.Attachments, AttachmentCleanups: deps.AttachmentCleanups, AttachmentObjects: deps.AttachmentObjects,
		Tx: deps.Tx, Observe: deps.Observe, Now: deps.Now,
	}}
}
