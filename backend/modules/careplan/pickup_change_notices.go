package careplan

import (
	"context"
	"time"

	calendar "github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// Telling the child's team about a changed pickup time (#6855). A family asks
// for another pickup time on one day; once the OGS approves it, the staff
// responsible for the child get an unread marker that links to the child,
// and a device notification. A correction that takes the approval back marks
// it unread again. Who the team is and how it hears about it on its devices
// is the root's to bind: both come from owners built after Care Plan.

// PickupChangeNotice is a one-day pickup time a family asked for that became
// current on approval, or that a corrected decision took back.
type PickupChangeNotice struct {
	TenantID   int64
	StudentID  int64
	Date       calendar.Date
	PickupTime string // "15:04"
	// Withdrawn reports that the approved time no longer applies.
	Withdrawn bool
	// ReviewerAccountID decided it and needs no notice.
	ReviewerAccountID int64
	// RequestID is the family's request the marker points at.
	RequestID int64
}

// PickupChangeNotifier tells the staff responsible for the child, inside the
// decision's transaction, that its pickup time of one day changed.
type PickupChangeNotifier interface {
	NotifyPickupChangeApproved(ctx context.Context, notice PickupChangeNotice) error
}

// PickupChangeTeamReach names the staff responsible for a child and tells
// them on their devices. The root binds it over Delivery's responsibility
// rule and its shared staff push.
type PickupChangeTeamReach interface {
	// ResponsibleAccountIDs lists the active staff accounts of the request's
	// school that look after the child and may read its pickup times, without
	// excludedAccountID.
	ResponsibleAccountIDs(ctx context.Context, studentID, excludedAccountID int64) ([]int64, error)
	// NotifyPickupChanged enqueues the device notification in the caller's
	// transaction. A school without notifications, or a time outside its
	// notification window, is no error: the unread marker still reaches
	// everyone.
	NotifyPickupChanged(ctx context.Context, push PickupChangePush) error
}

// PickupChangePush is the device notification of one changed pickup time. It
// names no child: the text shows on a locked screen or a shared tablet, the
// link opens the child.
type PickupChangePush struct {
	StudentID           int64
	RequestID           int64
	Withdrawn           bool
	RecipientAccountIDs []int64
	Title               string
	Body                string
	DeepLink            string
}

// PickupChangeTeamNotice is one unread marker of the reader: a changed pickup
// time of a child the reader looks after, on a day still ahead.
type PickupChangeTeamNotice struct {
	RequestID        int64
	StudentID        int64
	StudentFirstName string
	Date             calendar.Date
	// PickupTime is the approved time ("15:04"); empty when Withdrawn.
	PickupTime string
	// Withdrawn reports that a corrected decision took the time back.
	Withdrawn  bool
	InformedAt time.Time
}

// PickupChangeNoticeRevision identifies a marker as the reader loaded it. A
// corrected decision told after the load is another revision.
type PickupChangeNoticeRevision struct {
	RequestID  int64
	InformedAt time.Time
}

// PickupChangeTeam tells the child's team about a changed one-day pickup
// time and serves its unread markers: the care request decisions call it,
// the student routes read it.
type PickupChangeTeam interface {
	PickupChangeNotifier
	PickupChangeTeamNotices
}

// PickupChangeTeamNotices is the read side of the markers.
type PickupChangeTeamNotices interface {
	// ListUnreadPickupChangeNotices returns the reader's unread markers on
	// days from today on, newest first.
	ListUnreadPickupChangeNotices(ctx context.Context, readerAccountID int64) ([]PickupChangeTeamNotice, error)
	// MarkPickupChangeNoticesRead clears the observed markers of one child
	// (the reader opened the child) and returns the markers still unread. A
	// marker told again while the child loaded stays unread.
	MarkPickupChangeNoticesRead(ctx context.Context, readerAccountID, studentID int64, observed []PickupChangeNoticeRevision) ([]PickupChangeTeamNotice, error)
}
