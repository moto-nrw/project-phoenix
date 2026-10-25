package parent

import (
	"context"
	"encoding/json"
	"time"

	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// PortalService is the guardian portal as these handlers consume it: the
// workflow's child, conversation, announcement and request-sharing flows. The
// composition root binds it to the composed parentportal.Portal; handler tests
// bind fakes. Status codes and error strings follow the sentinel errors the
// workflow declares.
type PortalService interface {
	// GuardianAnnouncementTenant reports the school of an announcement whose
	// attachments this account may download, or 0 when it may not (#2890).
	GuardianAnnouncementTenant(ctx context.Context, accountID, announcementID int64) (int64, error)

	// ListChildrenForAccount returns every child linked to any
	// guardian profile owned by the account, across every active
	// tenant mapping. Sorted by school then by name.
	ListChildrenForAccount(ctx context.Context, accountID int64) ([]*parentService.ChildSummary, error)

	// GetChildTodayStatus liefert den auf Elternsicht reduzierten
	// Betreuungsstatus des laufenden Berliner Kalendertages: eine
	// Ja/Nein-Aussage "in der OGS" plus einen erklaerenden Zustand. Die
	// Antwort enthaelt niemals Raeume, Besuchshistorie, Rohereignisse oder
	// Mitarbeitendennamen.
	GetChildTodayStatus(ctx context.Context, accountID, studentID int64) (*parentService.TodayStatus, error)

	// ListEnrollableForAccount returns every (school, open phase)
	// pair the parent could enroll a new child at, with a flag for
	// schools they're already linked to. Sorted with linked schools
	// first.
	ListEnrollableForAccount(ctx context.Context, accountID int64) ([]*parentService.EnrollablePhase, error)

	// GetEnrollmentSubmitStatus resolves whether the account is linked
	// to the school and whether its guardian relationships grant
	// parent_portal.enrollment.submit there (#1663). Reuses a caller-
	// provided admin transaction when one is in context.
	GetEnrollmentSubmitStatus(ctx context.Context, accountID, schoolID int64) (*parentService.GuardianSubmitStatus, error)

	// ListEnrollmentsForAccount returns every enrollment.requests row
	// where guardian_account_id matches the calling account, joined
	// to phase + school + child summaries. Used by the dashboard to
	// surface in-progress / decided submissions without the parent
	// having to dig out the email-link status URL.
	ListEnrollmentsForAccount(ctx context.Context, accountID int64) ([]*parentService.EnrollmentRequestSummary, error)

	// GetChildConsents returns the four consent/acknowledgement states currently
	// stored for the child. Visibility requires parent_portal.access; only the
	// voluntary photo consent can be withdrawn through this interface.
	GetChildConsents(ctx context.Context, accountID, studentID int64) ([]parentService.ChildConsent, error)
	WithdrawPhotoConsent(ctx context.Context, accountID, studentID int64) ([]parentService.ChildConsent, error)
	GrantPhotoConsent(ctx context.Context, accountID, studentID int64) ([]parentService.ChildConsent, error)

	GetProfile(ctx context.Context, accountID int64) (*parentService.Profile, error)

	UpdatePortalLocale(ctx context.Context, accountID int64, locale string) (*parentService.Profile, error)

	// SubmitSickNote reports the parent's child sick or excused for one or more
	// dates. Authorization: the account must be a guardian of the student. The
	// status must be StudentStatusDaySick or StudentStatusDayExcused. Gated by
	// operations.parent_sick_note_enabled for the child's tenant.
	//
	// By default, both statuses become PENDING requests. The independent sick
	// and excused approval settings can opt a school into direct status writes.
	// A note is mandatory for both absence types.
	// recipientGuardianProfileIDs are the co-guardians the family picked for
	// this request; they are stored in the SAME transaction as the request row
	// (#2267). Empty shares with nobody.
	SubmitSickNote(ctx context.Context, accountID, studentID int64, dates []timezone.Date, reason, status string, recipientGuardianProfileIDs []int64) (*parentService.SickNoteResult, error)

	// ListExcusedRequests returns the child's pending sick and excused absence
	// requests plus any decided in the recent window, newest-first. The method
	// retains its legacy excused-only name. Authorization only.
	ListExcusedRequests(ctx context.Context, accountID, studentID int64) ([]*parentService.ExcusedAbsenceRequestSnapshot, error)

	// EditExcusedRequest rewrites the caller's own pending sick or excused
	// absence request instead of withdrawing and refiling it (#2267). The
	// existing share is kept. Authorization: the account must be the
	// submitting guardian.
	EditExcusedRequest(ctx context.Context, accountID, studentID, requestID int64, dates []timezone.Date, note, expectedVersion string) (*parentService.ExcusedAbsenceRequestSnapshot, error)

	// EditPickupChangeRequest rewrites the caller's own pending one-day
	// pickup change (#2267).
	EditPickupChangeRequest(ctx context.Context, accountID, studentID, requestID int64, date timezone.Date, pickupTime time.Time, departureMode, reason, expectedVersion string) (*parentService.PickupChangeRequest, error)

	// EditCareScheduleRequest rewrites the caller's own pending weekly-plan
	// request (#2267).
	EditCareScheduleRequest(ctx context.Context, accountID, studentID, requestID int64, payload map[string]any, expectedVersion string) (*parentService.ChildCareSchedule, error)

	// EditMasterDataRequest rewrites the proposed value of the caller's own
	// pending Stammdaten request (#2267). Target and field stay fixed.
	EditMasterDataRequest(ctx context.Context, accountID, studentID, requestID int64, newValue json.RawMessage, expectedVersion string) (*parentService.StudentDataChangeRequest, error)

	// EditOfferingChangeRequest rewrites the caller's own pending offering
	// change (#2267).
	EditOfferingChangeRequest(ctx context.Context, accountID, studentID, requestID int64, selections []parentService.OfferingChangeSelectionSnapshot, effectiveFrom timezone.Date, note string, completeWithdrawalConfirmed bool, expectedVersion string) (*parentService.ChildCareOfferings, error)

	// ListRequestEvents returns one request's history for the guardian who
	// submitted it (#2267). Anyone else gets not-found.
	ListRequestEvents(ctx context.Context, accountID, studentID int64, requestType string, requestID int64) ([]parentService.ParentRequestEventView, error)

	// ListSickDays returns the child's currently-active parent-facing
	// absences (sick and excused) in the given date range; class-trip days
	// stay excluded as a staff-only status. Authorization only; not gated by
	// the setting so previously-reported days stay visible if a school later
	// disables the feature.
	ListSickDays(ctx context.Context, accountID, studentID int64, from, to timezone.Date) ([]*parentService.StudentStatusDaySnapshot, error)

	// ChildFeatures resolves which parent-portal write features are enabled
	// for the child's tenant, so the UI can hide/disable actions the backend
	// would reject with 403. Authorization only.
	ChildFeatures(ctx context.Context, accountID, studentID int64) (parentService.ChildFeatureFlags, error)

	// MealPlanWeek returns the Monday-Friday meal plan entries for the school
	// of the given child, for the week containing weekStart. Authorization
	// (parent_portal.access) plus the operations.meal_plan_enabled toggle for
	// the child's tenant: when the feature is off it returns
	// ErrMealPlanDisabled so the portal can hide the section.
	MealPlanWeek(ctx context.Context, accountID, studentID int64, weekStart timezone.Date) ([]parentService.MealPlanEntry, error)
	MealParticipation(ctx context.Context, accountID, studentID int64, from, to timezone.Date) (parentService.MealParticipationPlan, error)
	// Meal-participation writes require the relationship-scoped
	// parent_portal.meal_participation.manage permission.
	ReplaceMealParticipationSchedule(ctx context.Context, accountID, studentID int64, weekdays []parentService.MealWeekday) (string, error)
	SetMealParticipationDay(ctx context.Context, accountID, studentID int64, date timezone.Date, participating bool) error
	ClearMealParticipationDay(ctx context.Context, accountID, studentID int64, date timezone.Date) error

	// SubmitCareExceptionWithReason requires a
	// concrete pickup time and stores the parent's explanation with it. Arrival
	// exceptions remain under staff control.
	SubmitCareExceptionWithReason(ctx context.Context, accountID, studentID int64, date timezone.Date, pickupTime *time.Time, reason string) (*parentService.CareException, error)
	SubmitPickupChangeRequest(ctx context.Context, accountID, studentID int64, date timezone.Date, pickupTime time.Time, departureMode, reason string, recipientGuardianProfileIDs []int64) (*parentService.PickupChangeRequest, error)
	ListPickupChangeRequests(ctx context.Context, accountID, studentID int64) ([]parentService.PickupChangeRequest, error)
	// SubmitPickupChange and EditPickupChange carry the one-day collector and
	// the signature a collector or walking home alone needs (#4780).
	SubmitPickupChange(ctx context.Context, accountID, studentID int64, submission parentService.PickupChangeSubmission) (*parentService.PickupChangeRequest, error)
	EditPickupChange(ctx context.Context, accountID, studentID, requestID int64, edit parentService.PickupChangeEdit) (*parentService.PickupChangeRequest, error)
	// CreateSignedCareScheduleRequest and EditSignedCareScheduleRequest carry
	// the signature a weekday the child walks home alone needs (#4780).
	CreateSignedCareScheduleRequest(ctx context.Context, accountID, studentID int64, payload map[string]any, signature *parentService.PickupSignatureInput) (*parentService.ChildCareSchedule, error)
	EditSignedCareScheduleRequest(ctx context.Context, accountID, studentID, requestID int64, payload map[string]any, expectedVersion string, signature *parentService.PickupSignatureInput) (*parentService.ChildCareSchedule, error)
	// Pickup permission requests and the proof of a signed request (#4780).
	PickupSignatureStatement(ctx context.Context, accountID, studentID int64, input parentService.PickupSignatureStatementInput) (string, error)
	SubmitPickupPermissionRequest(ctx context.Context, accountID, studentID, guardianProfileID int64, input parentService.PickupPermissionInput) (*parentService.PickupChangeRequest, error)
	SubmitPickupPermissionRequestWithRelationship(ctx context.Context, accountID, studentID, guardianProfileID int64, input parentService.PickupPermissionInput, relationship parentService.GuardianRelationshipInput) (*parentService.PickupChangeRequest, error)
	ListPickupPermissionRequests(ctx context.Context, accountID, studentID int64) ([]parentService.PickupChangeRequest, error)
	PickupRequestProof(ctx context.Context, accountID, studentID, requestID int64) (*parentService.PickupProof, error)

	// ListCareExceptions returns the merged pickup/arrival exceptions for the
	// child in [from, to], including staff-authored ones (flagged via Source)
	// so the portal can show what is already set. Authorization only.
	ListCareExceptions(ctx context.Context, accountID, studentID int64, from, to timezone.Date) ([]*parentService.CareException, error)

	// DeleteCareException removes the guardian-authored pickup exception for the
	// given date. Arrival exceptions are left untouched. Authorization only.
	DeleteCareException(ctx context.Context, accountID, studentID int64, date timezone.Date) error

	// ListRelatedAccounts returns every guardian linked to the child with
	// portal-access status. Authorization only.
	ListRelatedAccounts(ctx context.Context, accountID, studentID int64) ([]*parentService.RelatedAccount, error)

	// InviteRelatedAccount invites a further guardian to the child by email.
	// Gated by guardians.parent_invite_mode; staff_approval queues the request.
	// confirmRoleUpgrade confirms upgrading an existing restrictive contact
	// link to full access (#2172).
	InviteRelatedAccount(ctx context.Context, accountID, studentID int64, email, firstName, lastName string, confirmRoleUpgrade bool) (*parentService.InviteRelatedAccountResult, error)

	// RemoveRelatedAccount removes another account's access to the child.
	// Gated by guardians.parent_can_remove; the primary guardian is protected.
	RemoveRelatedAccount(ctx context.Context, accountID, studentID, guardianProfileID int64) error

	// GetChildMasterData returns the structured Stammdaten view for the child:
	// the child's person + student fields and the calling guardian's own
	// contact data, plus any pending Track B change requests. Authorization
	// only (GuardianPermissionPortalAccess).
	GetChildMasterData(ctx context.Context, accountID, studentID int64) (*parentService.ChildMasterData, error)

	// UpdateMasterDataField applies a Track A direct edit to a single field and
	// returns the refreshed Stammdaten view. The edit is written to the live
	// record immediately and recorded as an auto_applied audit row. Gated by
	// operations.parent_master_data_edit_enabled and
	// GuardianPermissionMasterDataEdit.
	UpdateMasterDataField(ctx context.Context, accountID, studentID int64, target, fieldKey string, value json.RawMessage) (*parentService.ChildMasterData, error)

	// SubmitMasterDataChangeRequest records pending Track B change requests
	// (name, birthday, school class, permanent Gehzeit) for staff approval. Unchanged or
	// already-pending fields are skipped/rejected. Gated by
	// operations.parent_master_data_request_enabled and
	// GuardianPermissionMasterDataRequest.
	SubmitMasterDataChangeRequest(ctx context.Context, accountID, studentID int64, changes []parentService.MasterDataFieldChange, recipientGuardianProfileIDs []int64) ([]*parentService.StudentDataChangeRequest, error)

	// ListMyMasterDataRequests returns the child's change requests (any status),
	// newest-first. Authorization only.
	ListMyMasterDataRequests(ctx context.Context, accountID, studentID int64) ([]*parentService.StudentDataChangeRequest, error)

	// ListChildGuardians returns every guardian linked to the child with
	// contact + pickup detail and the caller's per-guardian edit capabilities.
	// Authorization only (parent_portal.access).
	ListChildGuardians(ctx context.Context, accountID, studentID int64) ([]*parentService.ChildGuardian, error)

	// CreateGuardianContact adds an accountless contact to the child. Contact
	// data requires parent_portal.guardian.edit; pickup and emergency flags also
	// require parent_portal.pickup.manage.
	CreateGuardianContact(ctx context.Context, accountID, studentID int64, input parentService.CreateGuardianContactInput) (*parentService.ChildGuardian, error)

	// UpdateGuardianContact edits a contact-only guardian's contact data (or the
	// caller's own profile): name, email, address, phone list. Requires
	// parent_portal.guardian.edit. A guardian with their own portal account is
	// rejected unless it is the caller.
	UpdateGuardianContact(ctx context.Context, accountID, studentID, guardianProfileID int64, input parentService.GuardianContactInput) (*parentService.ChildGuardian, error)

	// UpdateGuardianRelationship edits the per-child pickup/relationship fields.
	// PickupNotes requires parent_portal.guardian.edit; the
	// can_pickup / is_emergency_contact flags additionally require
	// parent_portal.pickup.manage.
	UpdateGuardianRelationship(ctx context.Context, accountID, studentID, guardianProfileID int64, input parentService.GuardianRelationshipInput) (*parentService.ChildGuardian, error)

	// ListMessageThreads returns every conversation the guardian owns across
	// all their children's schools (newest activity first), with the unread
	// staff-message count. Cross-tenant.
	ListMessageThreads(ctx context.Context, accountID int64) ([]*parentService.InboxThread, error)

	// ListChildThreads returns the guardian's conversation(s) about ONE owned
	// child (at most one per the chat model), newest activity first, with the
	// unread staff-message count. Does NOT mark the thread read. Authorization
	// resolves ownership + tenant first.
	ListChildThreads(ctx context.Context, accountID, studentID int64) ([]*parentService.InboxThread, error)

	// UnreadMessageCount returns the guardian's total count of conversations
	// with unread staff-side activity across all their children's schools — the
	// parent-portal sidebar badge. Cross-tenant; a light COUNT, not a projection.
	UnreadMessageCount(ctx context.Context, accountID int64) (int, error)

	// GetChildConversation returns the guardian's conversation about one owned
	// child (oldest-first) and marks it read. Returns an empty view (ThreadID
	// 0) when no conversation exists yet. Authorization only.
	GetChildConversation(ctx context.Context, accountID, studentID int64) (*parentService.MessageThreadView, error)

	// PostChildReply appends a guardian message to the child's conversation,
	// creating it on the first message, and notifies the OGS. Gated by
	// operations.parent_notes_enabled. A positive groupThreadID marks the
	// message as the answer to that group thread (#4784); it must have reached
	// the guardian for this child and still be open.
	PostChildReply(ctx context.Context, accountID, studentID int64, body string, groupThreadID int64) (*parentService.MessageThreadView, error)

	// ListGroupThreads returns the group threads that reached the guardian,
	// newest first (#4784). Cross-tenant.
	ListGroupThreads(ctx context.Context, accountID int64) ([]*parentService.GroupThreadListItem, error)

	// GetGroupThread opens one group thread with the messages that reached the
	// guardian and marks them read.
	GetGroupThread(ctx context.Context, accountID, threadID int64) (*parentService.GroupThreadDetail, error)

	// GetChildCareSchedule returns the child's permanent weekly care plan
	// (arrival/pickup times + departure modes per weekday) with any pending
	// change request — the Stammdaten read view. Authorization only.
	GetChildCareSchedule(ctx context.Context, accountID, studentID int64) (*parentService.ChildCareSchedule, error)

	// CreateCareScheduleRequest stores a pending care-schedule change request
	// for staff review on the Änderungsanfragen page. Requires
	// parent_portal.request.submit; gated by operations.parent_notes_enabled.
	CreateCareScheduleRequest(ctx context.Context, accountID, studentID int64, payload map[string]any) (*parentService.ChildCareSchedule, error)

	// GetChildOfferingCatalog returns the offerings the guardian may pick from
	// for a change request, prefilled with the current booking. Requires
	// parent_portal.request.submit.
	GetChildOfferingCatalog(ctx context.Context, accountID, studentID int64) (*parentService.OfferingChangeCatalogSnapshot, error)
	GetChildOfferingCatalogAt(ctx context.Context, accountID, studentID int64, effectiveFrom timezone.Date) (*parentService.OfferingChangeCatalogSnapshot, error)

	// CreateOfferingChangeRequest stores a pending post-enrollment offering
	// change for staff review. Requires parent_portal.request.submit.
	CreateOfferingChangeRequest(ctx context.Context, accountID, studentID int64, selections []parentService.OfferingChangeSelectionSnapshot, effectiveFrom timezone.Date, note string, completeWithdrawalConfirmed bool, recipientGuardianProfileIDs []int64) (*parentService.ChildCareOfferings, error)

	// GetChildCareOfferings returns the care offerings the child is booked into,
	// plus whether the guardian may request a change
	// (#1665). Authorization only — seeing the booking does not depend on the
	// change feature being switched on.
	GetChildCareOfferings(ctx context.Context, accountID, studentID int64) (*parentService.ChildCareOfferings, error)

	// GetChildCourses lists the school's courses (AGs reached through a care
	// offering) with the child's state, and RequestChildCourse /
	// WithdrawChildCourseRequest are the family's two actions on them (#3075).
	// Reading needs parent_portal.enrollments.view and reports a missing
	// permission as a named reason. The two actions additionally need
	// parent_portal.enrollment.submit.
	GetChildCourses(ctx context.Context, accountID, studentID int64) (*parentService.CourseCatalogSnapshot, error)
	RequestChildCourse(ctx context.Context, accountID, studentID, offeringID int64, note string) (*parentService.CourseCatalogSnapshot, error)
	WithdrawChildCourseRequest(ctx context.Context, accountID, studentID, requestID int64) (*parentService.CourseCatalogSnapshot, error)

	// ListAnnouncements returns the guardian's parent-news feed across all their
	// (news-enabled) children's schools, newest-published first, each with the
	// guardian's read/ack state. Cross-tenant; broadcast (#1669).
	ListAnnouncements(ctx context.Context, accountID int64) ([]*parentService.AnnouncementFeedItem, error)

	// UnreadAnnouncementCount returns how many feed announcements the guardian
	// has not read — the parent-portal Neuigkeiten badge. Cross-tenant.
	UnreadAnnouncementCount(ctx context.Context, accountID int64) (int, error)

	// MarkAnnouncementRead records that the guardian opened an announcement.
	// Refuses one that is not live or outside the guardian's audience, and
	// rejects a stale request whose expectedPublishedAt no longer matches the
	// live announcement (ErrAnnouncementStale) so a corrected announcement is
	// not marked read against the retracted wording.
	MarkAnnouncementRead(ctx context.Context, accountID, announcementID int64, expectedPublishedAt time.Time) error

	// AcknowledgeAnnouncement records an explicit "gelesen und bestätigt"; valid
	// only for an announcement that requires acknowledgement. Same audience and
	// stale-version guards as MarkAnnouncementRead.
	AcknowledgeAnnouncement(ctx context.Context, accountID, announcementID int64, expectedPublishedAt time.Time) error

	// RespondToAnnouncement records the guardian's answer to a poll (Umfrage) for
	// ONE child: optionIDs replaces whatever was selected for that child, an empty
	// slice withdraws the answer. Authorized per child (the account must be a
	// portal-enabled guardian of a child the poll reaches), with the same
	// stale-version guard as MarkAnnouncementRead plus the answer deadline (#1371).
	RespondToAnnouncement(ctx context.Context, accountID, announcementID, studentID int64, optionIDs []int64, expectedPublishedAt time.Time) error
	// SubmitDeclaration records one Erklärung for one child (#3430); false =
	// the same action was already the latest and nothing new was stored.
	SubmitDeclaration(ctx context.Context, accountID, announcementID int64, in parentService.DeclarationInput, confirm parentService.PasswordConfirmer) (*parentService.DeclarationSubmission, bool, error)
	// DeclarationProof returns the account's own proof for one child.
	DeclarationProof(ctx context.Context, accountID, announcementID, studentID int64) (*parentService.DeclarationProof, error)
}
