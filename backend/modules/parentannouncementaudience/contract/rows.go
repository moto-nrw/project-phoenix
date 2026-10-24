package contract

import "time"

// ParentAnnouncementFeedScope names the schools a guardian's feed draws from:
// schools with parent news on contribute every live announcement, schools with
// only the cancellation notice on contribute system-authored rows.
type ParentAnnouncementFeedScope struct {
	TenantIDs           []int64
	SystemOnlyTenantIDs []int64
}

// IsEmpty reports whether no school contributes anything.
func (s ParentAnnouncementFeedScope) IsEmpty() bool {
	return len(s.TenantIDs) == 0 && len(s.SystemOnlyTenantIDs) == 0
}

// ParentAnnouncementFeedItem is one row of the guardian-facing feed with the
// reading account's read/acknowledgement state.
type ParentAnnouncementFeedItem struct {
	ID                      int64
	TenantID                int64
	Title                   string
	Body                    string
	Priority                string
	LinkURL                 *string
	RequiresAcknowledgement bool
	PublishedAt             *time.Time
	ExpiresAt               *time.Time
	ResponseType            string
	ResponseDeadline        *time.Time
	DeliveryMode            string
	SystemKind              *string
	ReadAt                  *time.Time
	AcknowledgedAt          *time.Time
	ReminderSentAt          *time.Time
	ReminderText            *string
}

// ParentAnnouncementPollChild is one child a guardian may answer a poll for,
// with the option ids currently selected for that child.
type ParentAnnouncementPollChild struct {
	AnnouncementID  int64
	StudentID       int64
	FirstName       string
	LastName        string
	SelectedOptions []int64
}

// ParentAnnouncementPollOptionResult is one option's tally.
type ParentAnnouncementPollOptionResult struct {
	OptionID int64
	Label    string
	Position int
	Count    int
}

// ParentAnnouncementPollResults is the staff-facing evaluation of a poll.
type ParentAnnouncementPollResults struct {
	ChildCount       int
	TargetChildCount int
	AnsweredCount    int
	Options          []*ParentAnnouncementPollOptionResult
}

// ParentAnnouncementPollChildStatus is one reached child in the staff-facing
// answer list.
type ParentAnnouncementPollChildStatus struct {
	StudentID    int64
	FirstName    string
	LastName     string
	SchoolClass  string
	AnswerLabels []string
	RespondedAt  *time.Time
	CanAnswer    bool
}

// ParentAnnouncementReminderRecipient is one guardian to remind, with the
// address and locale the mail needs.
type ParentAnnouncementReminderRecipient struct {
	AccountID    int64
	Email        string
	FirstName    string
	LastName     string
	PortalLocale string
}

// ParentAnnouncementRecipient is one guardian to e-mail on publication.
type ParentAnnouncementRecipient struct {
	AccountID int64
	Email     string
	FirstName string
	LastName  string
}

// ParentAnnouncementDeliveryRecipient is one guardian linked to a reached
// child, resolved without the portal-access filter so the recipient matrix can
// show who receives nothing and why.
type ParentAnnouncementDeliveryRecipient struct {
	GuardianProfileID int64
	AccountID         *int64
	FirstName         string
	LastName          string
	Email             string
	HasPortalAccess   bool
	PortalLocale      string
}

// ParentAnnouncementLetterChildStatus is one reached child of an Elternbrief
// with its derived fulfilment state.
type ParentAnnouncementLetterChildStatus struct {
	StudentID      int64
	FirstName      string
	LastName       string
	SchoolClass    string
	CanConfirm     bool
	AcknowledgedAt *time.Time
	AckFirstName   string
	AckLastName    string
}

// ParentAnnouncementRecipientStatus is one guardian account in the live
// audience with that account's read/acknowledgement state.
type ParentAnnouncementRecipientStatus struct {
	AccountID      int64
	FirstName      string
	LastName       string
	PortalLocale   string
	ReadAt         *time.Time
	AcknowledgedAt *time.Time
}

// ParentAnnouncementStats is the staff-facing reach/engagement summary.
type ParentAnnouncementStats struct {
	TargetCount       int
	ReadCount         int
	AcknowledgedCount int
}

// PendingAnnouncementApplicant identifies an account or pre-account e-mail
// with an undecided enrollment application. Enrollment owns those rows; the
// audience projection receives them as values and never joins its tables.
//
// The json tags are load-bearing: the projection binds the slice as a jsonb
// record set, so the tag names are the SQL column names.
type PendingAnnouncementApplicant struct {
	TenantID          int64  `json:"tenant_id"`
	GuardianFirstName string `json:"guardian_first_name"`
	GuardianLastName  string `json:"guardian_last_name"`
	GuardianAccountID *int64 `json:"guardian_account_id"`
	GuardianEmail     string `json:"guardian_email"`
}

// DeclarationHistoryLink identifies the child a declaration submission keeps
// reachable after an activity enrollment ends. Communication's store owns the
// submission lookup; audience projections only receive these scoped values.
type DeclarationHistoryLink struct {
	TenantID       int64 `json:"tenant_id"`
	AnnouncementID int64 `json:"announcement_id"`
	StudentID      int64 `json:"student_id"`
}

// DeclarationChild is one child a declaration reaches.
type DeclarationChild struct {
	AnnouncementID int64
	StudentID      int64
	FirstName      string
	LastName       string
	SchoolClass    string
}

// DeclarationSigner is one guardian who may declare for a reached child.
type DeclarationSigner struct {
	StudentID         int64
	GuardianProfileID int64
	AccountID         int64
	FirstName         string
	LastName          string
	Email             string
	PortalLocale      string
	GuardianRole      string
}

// DeclarationSignerContext is the relationship a submission is made under.
type DeclarationSignerContext struct {
	GuardianProfileID int64
	FirstName         string
	LastName          string
	GuardianRole      string
}

// GroupAudienceTarget is the group, class or AG a parent group thread
// addresses (#4784). A class carries its name, a group and an AG their id.
type GroupAudienceTarget struct {
	Type    string
	RefID   *int64
	RefText *string
}

// GroupAudienceChild is one child a group thread target reaches right now,
// with the guardian accounts that may read it: a linked account with
// parent_portal.access and an active membership (ADR 0079). An empty list
// marks a child without reachable parents.
type GroupAudienceChild struct {
	StudentID          int64
	FirstName          string
	LastName           string
	SchoolClass        string
	GuardianAccountIDs []int64
}

// GroupStudent names one child of a group thread's read list.
type GroupStudent struct {
	StudentID   int64
	FirstName   string
	LastName    string
	SchoolClass string
}
