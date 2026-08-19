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

// Response types (mirrors the chk_parent_announcements_response_type
// constraint). "none" is a plain Mitteilung; the two choice types make the
// announcement a poll (Umfrage, #1371) that guardians answer PER CHILD.
const (
	ParentAnnouncementResponseNone         = "none"
	ParentAnnouncementResponseSingleChoice = "single_choice"
	ParentAnnouncementResponseMultiChoice  = "multi_choice"
)

// ValidAnnouncementResponseType reports whether t is a known response type.
func ValidAnnouncementResponseType(t string) bool {
	switch t {
	case ParentAnnouncementResponseNone,
		ParentAnnouncementResponseSingleChoice,
		ParentAnnouncementResponseMultiChoice:
		return true
	default:
		return false
	}
}

// Delivery modes (mirrors the chk_parent_announcements_delivery_mode
// constraint). "standard" is the Mitteilung every pre-#2384 row is: e-mail and
// acknowledgement are independent opt-ins and the mail carries only a title and
// a portal link. "letter" is the Elternbrief: both channels are mandatory (the
// chk_parent_announcements_letter_channels CHECK enforces it in the database, not
// just in the service) and the mail carries the full body.
const (
	ParentAnnouncementDeliveryStandard = "standard"
	ParentAnnouncementDeliveryLetter   = "letter"
)

// E-mail audiences (mirrors the chk_parent_announcements_email_audience
// constraint). This is deliberately a SEPARATE axis from the portal audience:
// a letter may be a general notice everyone should receive, or it may carry
// details only guardians with portal access are meant to see (#2384).
//
//	EmailAudiencePortalOnly  — only guardians with parent_portal.access, i.e. the
//	                           same people who see the announcement in the portal
//	EmailAudienceAllContacts — additionally every guardian of a reached child that
//	                           has an address but no portal access; they receive
//	                           the mail and can never acknowledge
//
// PortalOnly is the DEFAULT on purpose: an omission must fall towards not
// disclosing the body to someone deliberately excluded from the portal.
const (
	EmailAudiencePortalOnly  = "portal_only"
	EmailAudienceAllContacts = "all_contacts"
)

// ValidAnnouncementDeliveryMode reports whether m is a known delivery mode.
func ValidAnnouncementDeliveryMode(m string) bool {
	return m == ParentAnnouncementDeliveryStandard || m == ParentAnnouncementDeliveryLetter
}

// ValidAnnouncementEmailAudience reports whether a is a known e-mail audience.
func ValidAnnouncementEmailAudience(a string) bool {
	return a == EmailAudiencePortalOnly || a == EmailAudienceAllContacts
}

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

	// ResponseType turns the announcement into a poll (Umfrage): "none" is a plain
	// Mitteilung, single_choice/multi_choice carry Options guardians answer per
	// child. ResponseDeadline is the answer cut-off and is deliberately NOT
	// ExpiresAt: a closed poll stays readable in the feed, it just stops accepting
	// answers.
	// nullzero+default: an unset ResponseType must fall through to the column
	// default ('none'), not insert an empty string that trips
	// chk_parent_announcements_response_type. Any caller constructing a plain
	// Mitteilung can therefore leave the field alone.
	ResponseType     string     `bun:"response_type,notnull,nullzero,default:'none'" json:"response_type"`
	ResponseDeadline *time.Time `bun:"response_deadline" json:"response_deadline,omitempty"`

	// DeliveryMode turns the announcement into an Elternbrief (#2384) and
	// EmailAudience widens who receives the mail. Both use nullzero+default for
	// the same reason ResponseType does: a caller that leaves the field alone
	// must fall through to the column default rather than insert an empty string
	// that trips the CHECK constraint.
	DeliveryMode  string `bun:"delivery_mode,notnull,nullzero,default:'standard'" json:"delivery_mode"`
	EmailAudience string `bun:"email_audience,notnull,nullzero,default:'portal_only'" json:"email_audience"`

	// Targets is attached by the repository (ListTargets), never a bun relation:
	// the parent feed resolves audience in a cross-tenant admin tx where explicit
	// joins are clearer than relation magic. bun:"-" so it is never persisted.
	Targets []*ParentAnnouncementTarget `bun:"-" json:"targets,omitempty"`
	// Options is attached the same way (ListOptions) and is empty for a plain
	// Mitteilung.
	Options []*ParentAnnouncementOption `bun:"-" json:"options,omitempty"`
}

// IsPublished reports whether the announcement has been published (vs draft).
func (a *ParentAnnouncement) IsPublished() bool {
	return a.PublishedAt != nil
}

// IsLetter reports whether the announcement is an Elternbrief — the mode in
// which both channels are mandatory and the mail carries the full body.
func (a *ParentAnnouncement) IsLetter() bool {
	return a.DeliveryMode == ParentAnnouncementDeliveryLetter
}

// ReachesContactsWithoutPortal reports whether the e-mail goes beyond the portal
// audience to guardians that have an address but no portal access.
func (a *ParentAnnouncement) ReachesContactsWithoutPortal() bool {
	return a.EmailAudience == EmailAudienceAllContacts
}

// IsPoll reports whether the announcement asks guardians a question.
func (a *ParentAnnouncement) IsPoll() bool {
	return a.ResponseType == ParentAnnouncementResponseSingleChoice ||
		a.ResponseType == ParentAnnouncementResponseMultiChoice
}

// AcceptsResponsesAt reports whether the poll still accepts answers at t: it
// must be a poll and either have no deadline or one that has not passed.
// Visibility (active/published/expires_at) is checked separately — a closed poll
// stays readable.
func (a *ParentAnnouncement) AcceptsResponsesAt(t time.Time) bool {
	if !a.IsPoll() {
		return false
	}
	return a.ResponseDeadline == nil || a.ResponseDeadline.After(t)
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

// ParentAnnouncementOption is one answer option of a poll. Position drives the
// display order and is unique per announcement. Options are insert/delete only
// (replaced wholesale while the announcement is a draft), so the table has no
// updated_at and this embeds bun.BaseModel rather than base.Model.
type ParentAnnouncementOption struct {
	bun.BaseModel `bun:"table:users.parent_announcement_options,alias:pao"`
	base.TenantModel
	ID             int64     `bun:"id,pk,autoincrement" json:"id"`
	AnnouncementID int64     `bun:"announcement_id,notnull" json:"announcement_id"`
	Label          string    `bun:"label,notnull" json:"label"`
	Position       int       `bun:"position,notnull" json:"position"`
	CreatedAt      time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp" json:"created_at"`
}

// ParentAnnouncementResponse is one child's chosen option. The answer is per
// CHILD, not per guardian account: a parent with two children answers once per
// child, and the staff-facing result counts children. AccountID records which
// guardian submitted it (two guardians share a child; the service replaces the
// child's whole row set, so the last writer wins).
type ParentAnnouncementResponse struct {
	bun.BaseModel `bun:"table:users.parent_announcement_responses,alias:par_resp"`
	base.TenantModel
	ID             int64     `bun:"id,pk,autoincrement" json:"id"`
	AnnouncementID int64     `bun:"announcement_id,notnull" json:"announcement_id"`
	OptionID       int64     `bun:"option_id,notnull" json:"option_id"`
	StudentID      int64     `bun:"student_id,notnull" json:"student_id"`
	AccountID      int64     `bun:"account_id,notnull" json:"account_id"`
	RespondedAt    time.Time `bun:"responded_at,nullzero,notnull,default:current_timestamp" json:"responded_at"`
}

// AnnouncementPollChild is one child a guardian may answer for, with the option
// ids currently selected for that child (empty = not answered yet). Used by the
// parent feed so the portal can render one answer row per child.
type AnnouncementPollChild struct {
	AnnouncementID  int64   `bun:"announcement_id" json:"-"`
	StudentID       int64   `bun:"student_id" json:"-"`
	FirstName       string  `bun:"first_name" json:"first_name"`
	LastName        string  `bun:"last_name" json:"last_name"`
	SelectedOptions []int64 `bun:"selected_options,array" json:"-"`
}

// AnnouncementPollOptionResult is one option's tally: how many reached children
// picked it.
type AnnouncementPollOptionResult struct {
	OptionID int64  `bun:"option_id" json:"option_id"`
	Label    string `bun:"label" json:"label"`
	Position int    `bun:"position" json:"position"`
	Count    int    `bun:"count" json:"count"`
}

// AnnouncementPollResults is the staff-facing evaluation of a poll. ChildCount
// is the number of currently reached children with at least one guardian who
// may respond; TargetChildCount is the full portal-visible target audience.
// Reach is resolved live against current relationships, so the numbers reflect
// "right now" rather than a publish-time snapshot.
type AnnouncementPollResults struct {
	ChildCount       int                             `json:"child_count"`
	TargetChildCount int                             `json:"target_child_count"`
	AnsweredCount    int                             `json:"answered_count"`
	Options          []*AnnouncementPollOptionResult `json:"options"`
}

// AnnouncementPollChildStatus is one reached child in the staff-facing answer
// list: who they are and what was answered for them (empty = still open).
type AnnouncementPollChildStatus struct {
	StudentID     int64      `bun:"student_id" json:"student_id"`
	FirstName     string     `bun:"first_name" json:"first_name"`
	LastName      string     `bun:"last_name" json:"last_name"`
	SchoolClass   string     `bun:"school_class" json:"school_class"`
	AnswerLabels  []string   `bun:"answer_labels,array" json:"answer_labels"`
	RespondedAt   *time.Time `bun:"responded_at" json:"responded_at,omitempty"`
	CanAnswer     bool       `bun:"can_answer" json:"can_answer"`
	GuardianCount int        `bun:"guardian_count" json:"-"`
}

// AnnouncementPollReminderRecipient is one guardian to remind about a poll they
// have not answered for a given child: the account, their address and name.
type AnnouncementPollReminderRecipient struct {
	AccountID    int64  `bun:"account_id"`
	Email        string `bun:"email"`
	FirstName    string `bun:"first_name"`
	LastName     string `bun:"last_name"`
	PortalLocale string `bun:"portal_locale"`
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
	ResponseType            string     `bun:"response_type" json:"response_type"`
	// DeliveryMode lets the portal mark an Elternbrief as binding ("Bestätigung
	// erforderlich") instead of making the parent infer it from
	// RequiresAcknowledgement alone.
	DeliveryMode     string     `bun:"delivery_mode" json:"delivery_mode"`
	ResponseDeadline *time.Time `bun:"response_deadline" json:"response_deadline,omitempty"`
	SchoolName       string     `bun:"school_name" json:"school_name"`
	ReadAt           *time.Time `bun:"read_at" json:"read_at,omitempty"`
	AcknowledgedAt   *time.Time `bun:"acknowledged_at" json:"acknowledged_at,omitempty"`

	// Options and Children are attached by the service for poll items (bun:"-":
	// they come from separate batched queries, not this row's columns).
	Options  []*ParentAnnouncementOption `bun:"-" json:"options,omitempty"`
	Children []*AnnouncementPollChild    `bun:"-" json:"children,omitempty"`
}

// IsPoll reports whether the feed item asks the guardian a question.
func (i *AnnouncementFeedItem) IsPoll() bool {
	return i.ResponseType == ParentAnnouncementResponseSingleChoice ||
		i.ResponseType == ParentAnnouncementResponseMultiChoice
}

// AnnouncementRecipient is one guardian to e-mail when an announcement that
// opted into e-mail is published: their address and name (for the greeting).
type AnnouncementRecipient struct {
	AccountID int64  `bun:"account_id"`
	Email     string `bun:"email"`
	FirstName string `bun:"first_name"`
	LastName  string `bun:"last_name"`
}

// AnnouncementDeliveryRecipient is one guardian linked to a reached child,
// resolved WITHOUT the portal-access filter that ResolveAudienceEmails applies.
// It is the input for the per-recipient delivery rows behind the staff matrix
// (#2384), which must also show the people that get nothing — a guardian with
// no address, or one without portal access.
//
// HasPortalAccess is true when the guardian can actually see the announcement in
// moto: a students_guardians link granting parent_portal.access on at least one
// reached child, a linked account, and an active tenant membership. Only those
// people can ever acknowledge.
type AnnouncementDeliveryRecipient struct {
	GuardianProfileID int64  `bun:"guardian_profile_id"`
	AccountID         *int64 `bun:"account_id"`
	FirstName         string `bun:"first_name"`
	LastName          string `bun:"last_name"`
	Email             string `bun:"email"`
	HasPortalAccess   bool   `bun:"has_portal_access"`
	PortalLocale      string `bun:"portal_locale"`
}

// AnnouncementLetterChildStatus is one reached child in the Elternbrief view:
// is the letter fulfilled for this child, and if so, who confirmed it and when.
//
// Fulfilment is DERIVED, never stored (#2384). A child counts as fulfilled when
// any guardian with parent_portal.access on that child has acknowledged the
// announcement. Because acknowledgement is recorded per ACCOUNT, one
// confirmation automatically covers every addressed sibling of that guardian —
// the rule falls out of the existing model instead of needing a second table.
//
// AcknowledgedAt nil means still open. The named person is the FIRST to confirm:
// once the letter is fulfilled, who got there first is the answer to "wer hat
// für dieses Kind bestätigt".
type AnnouncementLetterChildStatus struct {
	StudentID      int64      `bun:"student_id" json:"student_id"`
	FirstName      string     `bun:"first_name" json:"first_name"`
	LastName       string     `bun:"last_name" json:"last_name"`
	SchoolClass    string     `bun:"school_class" json:"school_class"`
	AcknowledgedAt *time.Time `bun:"acknowledged_at" json:"acknowledged_at,omitempty"`
	AckFirstName   string     `bun:"ack_first_name" json:"ack_first_name,omitempty"`
	AckLastName    string     `bun:"ack_last_name" json:"ack_last_name,omitempty"`
}

// Fulfilled reports whether someone has confirmed the letter for this child.
func (c *AnnouncementLetterChildStatus) Fulfilled() bool { return c.AcknowledgedAt != nil }

// AnnouncementRecipientStatus is one guardian account in an announcement's
// live audience together with that account's read/ack state — the staff-facing
// "wer hat gelesen/bestätigt" list behind the reach counts. Resolved live, so
// the list always reflects the current relationships.
type AnnouncementRecipientStatus struct {
	AccountID      int64      `bun:"account_id" json:"account_id"`
	FirstName      string     `bun:"first_name" json:"first_name"`
	LastName       string     `bun:"last_name" json:"last_name"`
	PortalLocale   string     `bun:"portal_locale" json:"-"`
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

	// --- poll options (tenant tx) ---
	// ReplaceOptions swaps a DRAFT poll's answer options wholesale. Like
	// ReplaceTargets it locks the announcement and refuses a published row
	// (ErrAnnouncementPublished), so options can never change under answers that
	// already reference them.
	ReplaceOptions(ctx context.Context, tenantID, announcementID int64, options []*ParentAnnouncementOption) error
	ListOptions(ctx context.Context, announcementID int64) ([]*ParentAnnouncementOption, error)
	// ListOptionsForAnnouncements batches the option lookup for a feed page
	// (admin or tenant tx; explicit tenant scoping via the announcement ids).
	ListOptionsForAnnouncements(ctx context.Context, announcementIDs []int64) ([]*ParentAnnouncementOption, error)

	// --- poll responses ---
	// AnswerableChildren returns the children of accountID that a poll currently
	// reaches, each with the option ids selected for that child so far.
	AnswerableChildren(ctx context.Context, accountID int64, announcementIDs []int64) ([]*AnnouncementPollChild, error)
	// AccountMayAnswerForStudent reports whether the account is a portal-enabled
	// guardian of the student AND the poll's audience currently reaches that
	// child — the authorization for a per-child answer.
	AccountMayAnswerForStudent(ctx context.Context, tenantID, announcementID, accountID, studentID int64) (bool, error)
	// SetResponse replaces a child's selected options for a poll (delete + insert
	// in one statement set). Guarded on the announcement still being live at
	// expectedPublishedAt and still accepting answers, mirroring MarkRead: it
	// returns false when the guard missed so the service can answer 409 instead of
	// a silent 200. An empty optionIDs slice withdraws the answer.
	SetResponse(ctx context.Context, tenantID, announcementID, studentID, accountID int64, optionIDs []int64, expectedPublishedAt time.Time) (bool, error)
	// PollResults returns the per-option tally plus reach/answered child counts.
	PollResults(ctx context.Context, tenantID, announcementID int64) (*AnnouncementPollResults, error)
	// PollChildren returns every reached child with the labels answered for them
	// (empty = open), ordered by name — the per-child view behind PollResults.
	PollChildren(ctx context.Context, tenantID, announcementID int64) ([]*AnnouncementPollChildStatus, error)
	// UnansweredReminderRecipients returns the distinct guardians (with an
	// address) of reached children that have NO answer yet — the audience of the
	// "Erinnerung senden" action.
	UnansweredReminderRecipients(ctx context.Context, tenantID, announcementID int64) ([]*AnnouncementPollReminderRecipient, error)

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
	// UnacknowledgedReminderRecipients returns the guardians of reached children
	// that nobody has confirmed the Elternbrief for yet — one row per person, not
	// per child.
	UnacknowledgedReminderRecipients(ctx context.Context, tenantID, announcementID int64) ([]*AnnouncementPollReminderRecipient, error)
	// LetterChildStatuses returns every child the announcement reaches with the
	// derived fulfilment state: who confirmed the letter for that child and when,
	// or nil while it is still open. The audience is the same portal-visible one
	// the poll view uses, so a child nobody can reach in moto is visible rather
	// than silently missing.
	LetterChildStatuses(ctx context.Context, tenantID, announcementID int64) ([]*AnnouncementLetterChildStatus, error)
	// ResolveDeliveryRecipients returns EVERY guardian linked to a child the
	// announcement's student-based targets reach — including those without an
	// address and those without portal access — so the recipient matrix can show
	// who gets nothing and why. Unlike ResolveAudienceEmails it applies no
	// parent_portal.access filter; the caller decides who is mailed from the
	// announcement's email_audience. The pending_enrollment target is deliberately
	// not covered: those guardians have no student link, and an Elternbrief may
	// not target them at all.
	ResolveDeliveryRecipients(ctx context.Context, tenantID, announcementID int64) ([]*AnnouncementDeliveryRecipient, error)
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
