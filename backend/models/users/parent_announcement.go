package users

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// ErrAnnouncementPublished is returned by the repository Update when the target
// row is no longer an editable draft — it was published (or deleted) between
// the service loading it and the write. The staff service maps it to its
// published-immutable error so a lost race surfaces the same conflict as the
// load-time check. Keeping it here (not in the service) lets the repository
// signal the condition without importing the service package.
var ErrAnnouncementPublished = errors.New("users: parent announcement is published (not editable)")

const ()

// Announcement priority levels (mirrors the chk_parent_announcements_priority
// constraint). "important" only changes presentation (a highlighted badge); it
// has no effect on targeting or delivery.
const (
	ParentAnnouncementPriorityInfo      = "info"
	ParentAnnouncementPriorityImportant = "important"
)

// Audience target types (mirrors the chk_parent_announcement_targets_type
// constraint). An announcement's reach is the OR-union of its target rows.
const (
	AnnouncementTargetSchoolAll         = "school_all"
	AnnouncementTargetClass             = "class"
	AnnouncementTargetGroup             = "group"
	AnnouncementTargetActivityGroup     = "activity_group"
	AnnouncementTargetStudent           = "student"
	AnnouncementTargetPendingEnrollment = "pending_enrollment"
)

// ValidAnnouncementPriority reports whether p is a known priority.
func ValidAnnouncementPriority(p string) bool {
	return p == ParentAnnouncementPriorityInfo || p == ParentAnnouncementPriorityImportant
}

// ValidAnnouncementTargetType reports whether t is a known target type.
func ValidAnnouncementTargetType(t string) bool {
	switch t {
	case AnnouncementTargetSchoolAll, AnnouncementTargetClass, AnnouncementTargetGroup,
		AnnouncementTargetActivityGroup, AnnouncementTargetStudent, AnnouncementTargetPendingEnrollment:
		return true
	default:
		return false
	}
}

// ParentAnnouncement is a tenant-authored broadcast to guardians. It is a DRAFT
// while PublishedAt is nil, becomes visible to its targeted guardians once
// PublishedAt is set, and stays visible until ExpiresAt (nil = never) or
// Active is false. PublishedAt/ExpiresAt are instants (TIMESTAMPTZ), not
// calendar dates.
type ParentAnnouncement struct {
	base.Model `bun:"schema:users,table:parent_announcements"`
	base.TenantModel
	Title                   string     `bun:"title,notnull" json:"title"`
	Body                    string     `bun:"body,notnull" json:"body"`
	Priority                string     `bun:"priority,notnull" json:"priority"`
	LinkURL                 *string    `bun:"link_url" json:"link_url,omitempty"`
	RequiresAcknowledgement bool       `bun:"requires_acknowledgement,notnull" json:"requires_acknowledgement"`
	SendEmail               bool       `bun:"send_email,notnull" json:"send_email"`
	PublishedAt             *time.Time `bun:"published_at" json:"published_at,omitempty"`
	ExpiresAt               *time.Time `bun:"expires_at" json:"expires_at,omitempty"`
	Active                  bool       `bun:"active,notnull" json:"active"`
	CreatedBy               int64      `bun:"created_by,notnull" json:"created_by"`

	// Targets is attached by the repository (ListTargets), never a bun relation:
	// the parent feed resolves audience in a cross-tenant admin tx where explicit
	// joins are clearer than relation magic. bun:"-" so it is never persisted.
	Targets []*ParentAnnouncementTarget `bun:"-" json:"targets,omitempty"`
}

// IsPublished reports whether the announcement has been published (vs draft).
func (a *ParentAnnouncement) IsPublished() bool {
	return a.PublishedAt != nil
}

// ParentAnnouncementTarget is one audience selector on an announcement. The ref
// columns are type-specific (enforced by chk_parent_announcement_targets_ref):
// class -> TargetRefText (the school_class string); group/activity_group/student
// -> TargetRefID; school_all/pending_enrollment -> neither. The table has no
// updated_at (targets are insert/delete only, replaced wholesale on edit), so
// it embeds bun.BaseModel rather than base.Model.
type ParentAnnouncementTarget struct {
	bun.BaseModel `bun:"table:users.parent_announcement_targets,alias:pat"`
	base.TenantModel
	ID             int64     `bun:"id,pk,autoincrement" json:"id"`
	AnnouncementID int64     `bun:"announcement_id,notnull" json:"announcement_id"`
	TargetType     string    `bun:"target_type,notnull" json:"target_type"`
	TargetRefID    *int64    `bun:"target_ref_id" json:"target_ref_id,omitempty"`
	TargetRefText  *string   `bun:"target_ref_text" json:"target_ref_text,omitempty"`
	CreatedAt      time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp" json:"created_at"`
}

// ParentAnnouncementRead is a guardian account's read/ack state for one
// announcement. Read state is account-global (one row per account+announcement),
// not per child: the feed is a cross-child, cross-tenant list, so "gelesen" is
// the guardian's, not a child's. AcknowledgedAt is nil until the guardian
// explicitly confirms an announcement that RequiresAcknowledgement.
type ParentAnnouncementRead struct {
	bun.BaseModel `bun:"table:users.parent_announcement_reads,alias:par"`
	base.TenantModel
	AnnouncementID int64      `bun:"announcement_id,pk" json:"announcement_id"`
	AccountID      int64      `bun:"account_id,pk" json:"account_id"`
	ReadAt         time.Time  `bun:"read_at,notnull" json:"read_at"`
	AcknowledgedAt *time.Time `bun:"acknowledged_at" json:"acknowledged_at,omitempty"`
}

// AnnouncementFeedItem is the parent-facing list/detail projection: the
// announcement core fields plus the authoring school's name (the feed is
// cross-tenant, so each row is labelled with its school) and the guardian's
// read/ack state. Built by a join in the repository; json tags carry the
// parent-facing payload.
type AnnouncementFeedItem struct {
	ID                      int64      `bun:"id" json:"id"`
	TenantID                int64      `bun:"tenant_id" json:"tenant_id"`
	Title                   string     `bun:"title" json:"title"`
	Body                    string     `bun:"body" json:"body"`
	Priority                string     `bun:"priority" json:"priority"`
	LinkURL                 *string    `bun:"link_url" json:"link_url,omitempty"`
	RequiresAcknowledgement bool       `bun:"requires_acknowledgement" json:"requires_acknowledgement"`
	PublishedAt             *time.Time `bun:"published_at" json:"published_at,omitempty"`
	ExpiresAt               *time.Time `bun:"expires_at" json:"expires_at,omitempty"`
	SchoolName              string     `bun:"school_name" json:"school_name"`
	ReadAt                  *time.Time `bun:"read_at" json:"read_at,omitempty"`
	AcknowledgedAt          *time.Time `bun:"acknowledged_at" json:"acknowledged_at,omitempty"`
}

// AnnouncementRecipient is one guardian to e-mail when an announcement that
// opted into e-mail is published: their address and name (for the greeting).
type AnnouncementRecipient struct {
	AccountID int64  `bun:"account_id"`
	Email     string `bun:"email"`
	FirstName string `bun:"first_name"`
	LastName  string `bun:"last_name"`
}

// AnnouncementRecipientStatus is one guardian account in an announcement's
// live audience together with that account's read/ack state — the staff-facing
// "wer hat gelesen/bestätigt" list behind the reach counts. Resolved live, so
// the list always reflects the current relationships.
type AnnouncementRecipientStatus struct {
	AccountID      int64      `bun:"account_id" json:"account_id"`
	FirstName      string     `bun:"first_name" json:"first_name"`
	LastName       string     `bun:"last_name" json:"last_name"`
	ReadAt         *time.Time `bun:"read_at" json:"read_at,omitempty"`
	AcknowledgedAt *time.Time `bun:"acknowledged_at" json:"acknowledged_at,omitempty"`
}

// AnnouncementStats is the staff-facing reach/engagement summary for one
// announcement: how many guardian accounts it currently targets, how many have
// read it, and how many have acknowledged it. Reach is resolved live against
// the current student/guardian relationships, so the counts reflect "right now"
// rather than a publish-time snapshot.
type AnnouncementStats struct {
	TargetCount       int `json:"target_count"`
	ReadCount         int `json:"read_count"`
	AcknowledgedCount int `json:"acknowledged_count"`
}

// ParentAnnouncementRepository is the tenant-scoped data-access contract for
// parent announcements. Staff-facing methods run inside a tenant transaction;
// the parent feed / audience methods accept explicit tenant scoping so they can
// run inside a cross-tenant admin transaction (a guardian's children span
// schools).
type ParentAnnouncementRepository interface {
	// --- staff authoring (tenant tx) ---
	Create(ctx context.Context, a *ParentAnnouncement) error
	Update(ctx context.Context, a *ParentAnnouncement) error
	FindByID(ctx context.Context, id int64) (*ParentAnnouncement, error)
	Delete(ctx context.Context, id int64) error
	// ListForTenant returns the tenant's announcements newest-first.
	// includeInactive controls whether soft-disabled (active=false) rows appear.
	ListForTenant(ctx context.Context, includeInactive bool) ([]*ParentAnnouncement, error)
	SetPublished(ctx context.Context, id int64, publishedAt *time.Time) error
	// PublishIfDraft atomically flips a draft (published_at IS NULL) to
	// published and reports whether this call made the change — false when the
	// row was already published (a concurrent publish won the race), so the
	// caller enqueues the opt-in e-mails only once.
	PublishIfDraft(ctx context.Context, id int64, publishedAt time.Time) (bool, error)

	// --- targets (tenant tx) ---
	ReplaceTargets(ctx context.Context, tenantID, announcementID int64, targets []*ParentAnnouncementTarget) error
	ListTargets(ctx context.Context, announcementID int64) ([]*ParentAnnouncementTarget, error)

	// --- audience resolution (admin or tenant tx; explicit tenant filter) ---
	// CountAudience returns the number of distinct guardian accounts an
	// announcement's targets currently reach within its tenant.
	CountAudience(ctx context.Context, tenantID, announcementID int64) (int, error)
	// AccountMatchesAnnouncement reports whether a guardian account is in an
	// announcement's audience right now (used to authorize a read/ack).
	AccountMatchesAnnouncement(ctx context.Context, tenantID, announcementID, accountID int64) (bool, error)
	// ResolveAudienceEmails returns the distinct guardian recipients (with a
	// non-empty e-mail) an announcement currently reaches, for the publish-time
	// e-mail notification.
	ResolveAudienceEmails(ctx context.Context, tenantID, announcementID int64) ([]*AnnouncementRecipient, error)
	// SchoolName returns the tenant's school name (for e-mail greetings/subject).
	SchoolName(ctx context.Context, tenantID int64) (string, error)
	// AudienceRecipients returns the guardian accounts an announcement currently
	// reaches, each with name and read/ack state (nil = not read / not
	// acknowledged), ordered by name.
	AudienceRecipients(ctx context.Context, tenantID, announcementID int64) ([]*AnnouncementRecipientStatus, error)

	// --- parent feed (admin tx, cross-tenant) ---
	// ListFeedForAccount returns published, active, unexpired announcements the
	// account is targeted by across the given tenants, newest-published first,
	// each with the account's read/ack state.
	ListFeedForAccount(ctx context.Context, accountID int64, tenantIDs []int64) ([]*AnnouncementFeedItem, error)
	// CountUnreadForAccount returns how many of those the account has not read.
	CountUnreadForAccount(ctx context.Context, accountID int64, tenantIDs []int64) (int, error)

	// --- read / ack state ---
	// MarkRead / MarkAcknowledged stamp the guardian's read/ack row only while
	// the announcement is still live at expectedPublishedAt. The version guard
	// closes the window between the service's resolve/authorize phase and this
	// write: a correction (unpublish -> edit -> republish) re-stamps published_at
	// and clears reads, so a stale request must not record a read/ack against the
	// corrected wording the guardian never saw. They return true when the
	// announcement was still live at that version (the write applied, or a prior
	// read/ack already existed) and false when the guard missed — the service maps
	// false to ErrAnnouncementStale so the client gets a 409 and refetches rather
	// than a silent 200 that diverges from the persisted state.
	MarkRead(ctx context.Context, tenantID, announcementID, accountID int64, expectedPublishedAt time.Time) (bool, error)
	MarkAcknowledged(ctx context.Context, tenantID, announcementID, accountID int64, expectedPublishedAt time.Time) (bool, error)
	Stats(ctx context.Context, tenantID, announcementID int64) (*AnnouncementStats, error)
}
