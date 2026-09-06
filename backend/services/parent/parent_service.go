// Package parent holds the cross-tenant guardian-portal services.
//
// All methods take an account ID rather than reading tenant from
// context because parent-scope JWTs intentionally carry tenant_id=0.
// Per-action tenant context is decided by the picked child on the
// frontend, then validated server-side via the auth.account_tenants
// membership check that already lives in the underlying repos.
package parent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/localization"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	notificationsSvc "github.com/moto-nrw/project-phoenix/modules/delivery/application/notifications"
	mealplanModule "github.com/moto-nrw/project-phoenix/modules/mealplan"
	"github.com/moto-nrw/project-phoenix/realtime"
	absenceSvc "github.com/moto-nrw/project-phoenix/services/absence"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
	configService "github.com/moto-nrw/project-phoenix/services/config"
	enrollmentSvc "github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/moto-nrw/project-phoenix/services/parentmessaging"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// MealPlan is the narrow provider capability used by the parents workflow.
type MealPlan interface {
	Available(context.Context) (bool, error)
	Week(context.Context, mealplanModule.Date) ([]mealplanModule.Entry, error)
	RegistrationAvailable(context.Context) (bool, error)
	Participation(context.Context, int64, mealplanModule.Date, mealplanModule.Date) (mealplanModule.ParticipationPlan, error)
	ReplaceParticipationSchedule(context.Context, mealplanModule.ReplaceParticipationSchedule) (mealplanModule.Date, error)
	SetParticipationForDay(context.Context, mealplanModule.SetParticipationDay) error
	ClearParticipationForDay(context.Context, mealplanModule.SetParticipationDay) error
}

// MealPlanEntry is the parent-portal view of one planned dish. Keeping this
// transport-neutral view in the parent service prevents HTTP handlers from
// depending on the meal-plan module's public API.
type MealPlanEntry struct {
	Date     string
	Position int
	Dish     string
	Note     *string
}

type MealWeekday int

type MealParticipationDay struct {
	Date          string
	Participating bool
	Source        string
	Changeable    bool
}

type MealParticipationPlan struct {
	Weekdays      []MealWeekday
	EffectiveFrom string
	CutoffTime    string
	Days          []MealParticipationDay
}

// Service is the public contract consumed by HTTP handlers.
type Service interface {
	// GuardianAnnouncementTenant reports the school of an announcement whose
	// attachments this account may download, or 0 when it may not (#2890).
	GuardianAnnouncementTenant(ctx context.Context, accountID, announcementID int64) (int64, error)

	// ListChildrenForAccount returns every child linked to any
	// guardian profile owned by the account, across every active
	// tenant mapping. Sorted by school then by name.
	ListChildrenForAccount(ctx context.Context, accountID int64) ([]*parentModels.ChildSummary, error)

	// GetChildTodayStatus liefert den auf Elternsicht reduzierten
	// Betreuungsstatus des laufenden Berliner Kalendertages: eine
	// Ja/Nein-Aussage "in der OGS" plus einen erklaerenden Zustand. Die
	// Antwort enthaelt niemals Raeume, Besuchshistorie, Rohereignisse oder
	// Mitarbeitendennamen.
	GetChildTodayStatus(ctx context.Context, accountID, studentID int64) (*TodayStatus, error)

	// ListEnrollableForAccount returns every (school, open phase)
	// pair the parent could enroll a new child at, with a flag for
	// schools they're already linked to. Sorted with linked schools
	// first.
	ListEnrollableForAccount(ctx context.Context, accountID int64) ([]*parentModels.EnrollablePhase, error)

	// GetEnrollmentSubmitStatus resolves whether the account is linked
	// to the school and whether its guardian relationships grant
	// parent_portal.enrollment.submit there (#1663). Reuses a caller-
	// provided admin transaction when one is in context.
	GetEnrollmentSubmitStatus(ctx context.Context, accountID, schoolID int64) (*parentModels.GuardianSubmitStatus, error)

	// ListEnrollmentsForAccount returns every enrollment.requests row
	// where guardian_account_id matches the calling account, joined
	// to phase + school + child summaries. Used by the dashboard to
	// surface in-progress / decided submissions without the parent
	// having to dig out the email-link status URL.
	ListEnrollmentsForAccount(ctx context.Context, accountID int64) ([]*parentModels.EnrollmentRequestSummary, error)

	// GetChildConsents returns the four consent/acknowledgement states currently
	// stored for the child. Visibility requires parent_portal.access; only the
	// voluntary photo consent can be withdrawn through this interface.
	GetChildConsents(ctx context.Context, accountID, studentID int64) ([]ChildConsent, error)
	WithdrawPhotoConsent(ctx context.Context, accountID, studentID int64) ([]ChildConsent, error)
	GrantPhotoConsent(ctx context.Context, accountID, studentID int64) ([]ChildConsent, error)

	GetProfile(ctx context.Context, accountID int64) (*Profile, error)

	UpdatePortalLocale(ctx context.Context, accountID int64, locale string) (*Profile, error)

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
	SubmitSickNote(ctx context.Context, accountID, studentID int64, dates []timezone.Date, reason, status string, recipientGuardianProfileIDs []int64) (*SickNoteResult, error)

	// ListExcusedRequests returns the child's pending sick and excused absence
	// requests plus any decided in the recent window, newest-first. The method
	// retains its legacy excused-only name. Authorization only.
	ListExcusedRequests(ctx context.Context, accountID, studentID int64) ([]*activeModels.ExcusedAbsenceRequest, error)

	// EditExcusedRequest rewrites the caller's own pending sick or excused
	// absence request instead of withdrawing and refiling it (#2267). The
	// existing share is kept. Authorization: the account must be the
	// submitting guardian.
	EditExcusedRequest(ctx context.Context, accountID, studentID, requestID int64, dates []timezone.Date, note, expectedVersion string) (*activeModels.ExcusedAbsenceRequest, error)

	// EditPickupChangeRequest rewrites the caller's own pending one-day
	// pickup change (#2267).
	EditPickupChangeRequest(ctx context.Context, accountID, studentID, requestID int64, date timezone.Date, pickupTime time.Time, reason, expectedVersion string) (*scheduleModels.CareScheduleChangeRequest, error)

	// EditCareScheduleRequest rewrites the caller's own pending weekly-plan
	// request (#2267).
	EditCareScheduleRequest(ctx context.Context, accountID, studentID, requestID int64, payload map[string]any, expectedVersion string) (*ChildCareSchedule, error)

	// EditMasterDataRequest rewrites the proposed value of the caller's own
	// pending Stammdaten request (#2267). Target and field stay fixed.
	EditMasterDataRequest(ctx context.Context, accountID, studentID, requestID int64, newValue json.RawMessage, expectedVersion string) (*usersModels.StudentDataChangeRequest, error)

	// EditOfferingChangeRequest rewrites the caller's own pending offering
	// change (#2267).
	EditOfferingChangeRequest(ctx context.Context, accountID, studentID, requestID int64, selections []enrollmentSvc.OfferingChangeSelection, effectiveFrom timezone.Date, note string, completeWithdrawalConfirmed bool, expectedVersion string) (*ChildCareOfferings, error)

	// ListRequestEvents returns one request's history for the guardian who
	// submitted it (#2267). Anyone else gets not-found.
	ListRequestEvents(ctx context.Context, accountID, studentID int64, requestType string, requestID int64) ([]ParentRequestEventView, error)

	// ListSickDays returns the child's currently-active parent-facing
	// absences (sick and excused) in the given date range; class-trip days
	// stay excluded as a staff-only status. Authorization only; not gated by
	// the setting so previously-reported days stay visible if a school later
	// disables the feature.
	ListSickDays(ctx context.Context, accountID, studentID int64, from, to timezone.Date) ([]*activeModels.StudentStatusDay, error)

	// ChildFeatures resolves which parent-portal write features are enabled
	// for the child's tenant, so the UI can hide/disable actions the backend
	// would reject with 403. Authorization only.
	ChildFeatures(ctx context.Context, accountID, studentID int64) (ChildFeatureFlags, error)

	// MealPlanWeek returns the Monday-Friday meal plan entries for the school
	// of the given child, for the week containing weekStart. Authorization
	// (parent_portal.access) plus the operations.meal_plan_enabled toggle for
	// the child's tenant: when the feature is off it returns
	// ErrMealPlanDisabled so the portal can hide the section.
	MealPlanWeek(ctx context.Context, accountID, studentID int64, weekStart timezone.Date) ([]MealPlanEntry, error)
	MealParticipation(ctx context.Context, accountID, studentID int64, from, to timezone.Date) (MealParticipationPlan, error)
	// Meal-participation writes require the relationship-scoped
	// parent_portal.meal_participation.manage permission.
	ReplaceMealParticipationSchedule(ctx context.Context, accountID, studentID int64, weekdays []MealWeekday) (string, error)
	SetMealParticipationDay(ctx context.Context, accountID, studentID int64, date timezone.Date, participating bool) error
	ClearMealParticipationDay(ctx context.Context, accountID, studentID int64, date timezone.Date) error

	// SubmitCareExceptionWithReason requires a
	// concrete pickup time and stores the parent's explanation with it. Arrival
	// exceptions remain under staff control.
	SubmitCareExceptionWithReason(ctx context.Context, accountID, studentID int64, date timezone.Date, pickupTime *time.Time, reason string) (*CareException, error)
	SubmitPickupChangeRequest(ctx context.Context, accountID, studentID int64, date timezone.Date, pickupTime time.Time, reason string, recipientGuardianProfileIDs []int64) (*scheduleModels.CareScheduleChangeRequest, error)
	ListPickupChangeRequests(ctx context.Context, accountID, studentID int64) ([]*scheduleModels.CareScheduleChangeRequest, error)

	// ListCareExceptions returns the merged pickup/arrival exceptions for the
	// child in [from, to], including staff-authored ones (flagged via Source)
	// so the portal can show what is already set. Authorization only.
	ListCareExceptions(ctx context.Context, accountID, studentID int64, from, to timezone.Date) ([]*CareException, error)

	// DeleteCareException removes the guardian-authored pickup exception for the
	// given date. Arrival exceptions are left untouched. Authorization only.
	DeleteCareException(ctx context.Context, accountID, studentID int64, date timezone.Date) error

	// ListRelatedAccounts returns every guardian linked to the child with
	// portal-access status. Authorization only.
	ListRelatedAccounts(ctx context.Context, accountID, studentID int64) ([]*RelatedAccount, error)

	// InviteRelatedAccount invites a further guardian to the child by email.
	// Gated by guardians.parent_invite_mode; staff_approval queues the request.
	// confirmRoleUpgrade confirms upgrading an existing restrictive contact
	// link to full access (#2172).
	InviteRelatedAccount(ctx context.Context, accountID, studentID int64, email, firstName, lastName string, confirmRoleUpgrade bool) (*InviteRelatedAccountResult, error)

	// RemoveRelatedAccount removes another account's access to the child.
	// Gated by guardians.parent_can_remove; the primary guardian is protected.
	RemoveRelatedAccount(ctx context.Context, accountID, studentID, guardianProfileID int64) error

	// GetChildMasterData returns the structured Stammdaten view for the child:
	// the child's person + student fields and the calling guardian's own
	// contact data, plus any pending Track B change requests. Authorization
	// only (GuardianPermissionPortalAccess).
	GetChildMasterData(ctx context.Context, accountID, studentID int64) (*ChildMasterData, error)

	// UpdateMasterDataField applies a Track A direct edit to a single field and
	// returns the refreshed Stammdaten view. The edit is written to the live
	// record immediately and recorded as an auto_applied audit row. Gated by
	// operations.parent_master_data_edit_enabled and
	// GuardianPermissionMasterDataEdit.
	UpdateMasterDataField(ctx context.Context, accountID, studentID int64, target, fieldKey string, value json.RawMessage) (*ChildMasterData, error)

	// SubmitMasterDataChangeRequest records pending Track B change requests
	// (name, birthday, school class, permanent Gehzeit) for staff approval. Unchanged or
	// already-pending fields are skipped/rejected. Gated by
	// operations.parent_master_data_request_enabled and
	// GuardianPermissionMasterDataRequest.
	SubmitMasterDataChangeRequest(ctx context.Context, accountID, studentID int64, changes []MasterDataFieldChange, recipientGuardianProfileIDs []int64) ([]*usersModels.StudentDataChangeRequest, error)

	// ListMyMasterDataRequests returns the child's change requests (any status),
	// newest-first. Authorization only.
	ListMyMasterDataRequests(ctx context.Context, accountID, studentID int64) ([]*usersModels.StudentDataChangeRequest, error)

	// ListChildGuardians returns every guardian linked to the child with
	// contact + pickup detail and the caller's per-guardian edit capabilities.
	// Authorization only (parent_portal.access).
	ListChildGuardians(ctx context.Context, accountID, studentID int64) ([]*ChildGuardian, error)

	// CreateGuardianContact adds an accountless contact to the child. Contact
	// data requires parent_portal.guardian.edit; pickup and emergency flags also
	// require parent_portal.pickup.manage.
	CreateGuardianContact(ctx context.Context, accountID, studentID int64, input CreateGuardianContactInput) (*ChildGuardian, error)

	// UpdateGuardianContact edits a contact-only guardian's contact data (or the
	// caller's own profile): name, email, address, phone list. Requires
	// parent_portal.guardian.edit. A guardian with their own portal account is
	// rejected unless it is the caller.
	UpdateGuardianContact(ctx context.Context, accountID, studentID, guardianProfileID int64, input GuardianContactInput) (*ChildGuardian, error)

	// UpdateGuardianRelationship edits the per-child pickup/relationship fields.
	// PickupNotes requires parent_portal.guardian.edit; the
	// can_pickup / is_emergency_contact flags additionally require
	// parent_portal.pickup.manage.
	UpdateGuardianRelationship(ctx context.Context, accountID, studentID, guardianProfileID int64, input GuardianRelationshipInput) (*ChildGuardian, error)

	// ListMessageThreads returns every conversation the guardian owns across
	// all their children's schools (newest activity first), with the unread
	// staff-message count. Cross-tenant.
	ListMessageThreads(ctx context.Context, accountID int64) ([]*usersModels.InboxThread, error)

	// ListChildThreads returns the guardian's conversation(s) about ONE owned
	// child (at most one per the chat model), newest activity first, with the
	// unread staff-message count. Does NOT mark the thread read. Authorization
	// resolves ownership + tenant first.
	ListChildThreads(ctx context.Context, accountID, studentID int64) ([]*usersModels.InboxThread, error)

	// UnreadMessageCount returns the guardian's total count of conversations
	// with unread staff-side activity across all their children's schools — the
	// parent-portal sidebar badge. Cross-tenant; a light COUNT, not a projection.
	UnreadMessageCount(ctx context.Context, accountID int64) (int, error)

	// GetChildConversation returns the guardian's conversation about one owned
	// child (oldest-first) and marks it read. Returns an empty view (ThreadID
	// 0) when no conversation exists yet. Authorization only.
	GetChildConversation(ctx context.Context, accountID, studentID int64) (*MessageThreadView, error)

	// PostChildMessage appends a guardian message to the child's conversation,
	// creating it on the first message, and notifies the OGS. Gated by
	// operations.parent_notes_enabled.
	PostChildMessage(ctx context.Context, accountID, studentID int64, body string) (*MessageThreadView, error)

	// GetChildCareSchedule returns the child's permanent weekly care plan
	// (arrival/pickup times + departure modes per weekday) with any pending
	// change request — the Stammdaten read view. Authorization only.
	GetChildCareSchedule(ctx context.Context, accountID, studentID int64) (*ChildCareSchedule, error)

	// CreateCareScheduleRequest stores a pending care-schedule change request
	// for staff review on the Änderungsanfragen page. Requires
	// parent_portal.request.submit; gated by operations.parent_notes_enabled.
	CreateCareScheduleRequest(ctx context.Context, accountID, studentID int64, payload map[string]any) (*ChildCareSchedule, error)

	// GetChildOfferingCatalog returns the offerings the guardian may pick from
	// for a change request, prefilled with the current booking. Requires
	// parent_portal.request.submit.
	GetChildOfferingCatalog(ctx context.Context, accountID, studentID int64) (*enrollmentSvc.OfferingChangeCatalog, error)
	GetChildOfferingCatalogAt(ctx context.Context, accountID, studentID int64, effectiveFrom timezone.Date) (*enrollmentSvc.OfferingChangeCatalog, error)

	// CreateOfferingChangeRequest stores a pending post-enrollment offering
	// change for staff review. Requires parent_portal.request.submit.
	CreateOfferingChangeRequest(ctx context.Context, accountID, studentID int64, selections []enrollmentSvc.OfferingChangeSelection, effectiveFrom timezone.Date, note string, completeWithdrawalConfirmed bool, recipientGuardianProfileIDs []int64) (*ChildCareOfferings, error)

	// GetChildCareOfferings returns the care offerings the child is booked into,
	// plus whether the guardian may request a change
	// (#1665). Authorization only — seeing the booking does not depend on the
	// change feature being switched on.
	GetChildCareOfferings(ctx context.Context, accountID, studentID int64) (*ChildCareOfferings, error)

	// ListAnnouncements returns the guardian's parent-news feed across all their
	// (news-enabled) children's schools, newest-published first, each with the
	// guardian's read/ack state. Cross-tenant; broadcast (#1669).
	ListAnnouncements(ctx context.Context, accountID int64) ([]*usersModels.AnnouncementFeedItem, error)

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
}

// ChildFeatureFlags reports the resolved per-tenant parent-portal feature
// toggles for a single child.
type ChildFeatureFlags struct {
	// CareEnded says the child has left the OGS (#2487). It is STATE, not a
	// capability: when it is true every write flag below is false, and the
	// portal shows a read-only profile with one sentence explaining why
	// instead of buttons that would all fail the same way.
	CareEnded       bool
	SickNoteEnabled bool
	// SickRequiresApproval is true when a Krankmeldung stays pending until the
	// OGS confirms it (operations.parent_sick_requires_approval, #2449).
	SickRequiresApproval bool
	// ExcusedRequiresApproval is true when an "excused" report must be confirmed
	// by the office before it takes effect (operations.parent_excused_requires_approval,
	// #1845). The parent UI uses it to explain that the absence will be pending
	// and to keep the mandatory-note requirement visible. Only meaningful while
	// SickNoteEnabled is true.
	ExcusedRequiresApproval bool
	NotesEnabled            bool
	// RequestSubmitEnabled is true when messaging is on AND the guardian holds
	// parent_portal.request.submit for this child — gates the change-request
	// quick actions (care schedule / master data) in the parent UI.
	RequestSubmitEnabled bool
	PickupChangeEnabled  bool
	PickupManageAllowed  bool
	// GuardianContactManageAllowed is true when the caller may create and edit
	// accountless contacts for this child.
	GuardianContactManageAllowed bool
	// RelatedAccountsInviteEnabled is true when parents may invite further
	// guardians (guardians.parent_invite_mode != disabled).
	RelatedAccountsInviteEnabled bool
	// RelatedAccountsRemoveEnabled is true when parents may remove another
	// account's access (guardians.parent_can_remove).
	RelatedAccountsRemoveEnabled bool
	// MasterDataEditEnabled is true when parents may directly edit the
	// non-guardian Track A Stammdaten fields (setting AND the relationship
	// permission).
	MasterDataEditEnabled bool
	// MasterDataContactEditEnabled is true when parents may directly edit their
	// guardian profile/phone contact fields. These writes require both the
	// master-data edit setting and guardian-management setting.
	MasterDataContactEditEnabled bool
	// MasterDataRequestEnabled is true when parents may submit Track B
	// change requests for approval (setting AND the relationship permission).
	MasterDataRequestEnabled bool
	// MealPlanEnabled is true when the school maintains a meal plan
	// (operations.meal_plan_enabled), so the portal can show the read-only
	// Essensplan section for this child's school.
	MealPlanEnabled         bool
	MealRegistrationEnabled bool
	// HasOpenChangeRequest is STATE, not a capability: true when the child has at
	// least one pending change request (master data OR care schedule) awaiting an
	// OGS decision. It rides along on the features fetch (the one call the child
	// overview already makes) so the overview can badge the Stammdaten entry
	// without pulling the full master-data + care-schedule payloads. The details
	// still live on the Stammdaten page. Defaults false, so a fetch failure never
	// shows a phantom badge.
	HasOpenChangeRequest bool
	// NewsEnabled is true when the school broadcasts parent announcements
	// (operations.parent_news_enabled), so the portal can advertise the
	// Neuigkeiten feed for this child's school. When every linked school has
	// it off, the feed/unread endpoints return nothing and the nav/panel
	// entries must stay hidden rather than dead-end on an empty page.
	NewsEnabled bool
	// ReasonRequired is true when this school makes the family state a reason
	// for a request (operations.parent_request_reason_policy is "guardians" or
	// "both", #2267). The portal marks the note field as required instead of
	// letting the server reject the submission afterwards.
	ReasonRequired bool
}

// CareException is the parent-facing projection of a single day's pickup and/or
// arrival override. PickupTime/ArrivalTime carry the wall-clock instant anchored
// to the reference date (TIME column); a nil field means that leg has no
// override for the day. Source is "guardian" for parent-authored entries and
// "staff" for ones the team set.
type CareException struct {
	Date         timezone.Date
	PickupTime   *time.Time
	ArrivalTime  *time.Time
	Reason       *string
	Source       string
	PickupSource string
	UpdatedAt    time.Time
	// PickupAbsent is true when a pickup-exception row exists for the day but
	// carries no time (StudentPickupException.IsAbsent) — staff's "not coming
	// today" marker. It is distinct from a nil PickupTime meaning "this day has
	// no pickup override at all": the former must resolve to an absence, the
	// latter falls back to the base plan. Such a row creates no status day, so
	// the care-schedule today_absent signal misses it (#1725 review).
	PickupAbsent bool
	// ArrivalAbsent is the arrival-leg counterpart: true when an arrival-exception
	// row exists for the day with no expected arrival (StudentArrivalException.
	// IsAbsent) — the child is not coming. Like PickupAbsent it creates no status
	// day, so today_absent misses it; either leg being absent must resolve the
	// tile to an absence rather than falling back to the base-plan pickup, so a
	// guardian is never told to expect a pickup for a child who is not coming
	// (#1725 review).
	ArrivalAbsent bool
}

// Profile carries the parent's explicit parents-portal locale choice.
// Explicit is false when the guardian has never picked a language in the
// portal (portal_locale IS NULL); the frontend then keeps the anonymous
// cookie/Accept-Language locale rather than snapping to the default.
type Profile struct {
	FirstName string
	LastName  string
	Locale    string
	Explicit  bool
}

// ServiceConfig is the dependency-injection bundle.
type ServiceConfig struct {
	ChildRepo             parentModels.ChildRepository
	EnrollablePhaseRepo   parentModels.EnrollablePhaseRepository
	EnrollmentSettings    enrollmentSettingsQueries
	EnrollmentRequestRepo parentModels.EnrollmentRequestRepository
	GuardianProfileRepo   usersModels.GuardianProfileRepository

	// AttendanceRepo liefert die schulweite Anwesenheit des Kindes. Sie ist
	// die einzige Praesenzquelle fuer Eltern; active.visits wird nie gelesen,
	// damit kein Raumbezug nach aussen gelangt. Gefuellt wird die Tabelle
	// sowohl vom Kiosk-Scan als auch von der manuellen Erfassung im
	// Personal-Portal, der Tagesstatus funktioniert also mit und ohne NFC.
	AttendanceRepo activeModels.AttendanceRepository

	// Per-child write features (sick notes + care exceptions).
	StatusDayRepo        activeModels.StudentStatusDayRepository
	StudentRepo          usersModels.StudentRepository
	PickupExceptionRepo  scheduleModels.StudentPickupExceptionRepository
	ArrivalExceptionRepo scheduleModels.StudentArrivalExceptionRepository
	// PickupAutoExcusal couples pulled-forward day pickup times with the
	// per-block partial-absence mechanics (#2360). Optional in tests; nil
	// skips the coupling.
	PickupAutoExcusal *scheduleSvc.PickupAutoExcusalSyncer
	Settings          configService.SettingsService
	Broadcaster       realtime.Broadcaster

	// Weekly care plan read view + change requests (#1803).
	ArrivalSchedules scheduleSvc.ArrivalScheduleService
	PickupSchedules  scheduleSvc.PickupScheduleService
	CareRequests     scheduleSvc.CareScheduleRequestService
	// ExcusedRequests is the legacy-named office-approval store for parent sick
	// and excused absences. When the matching setting is on, a submission becomes
	// a pending request here instead of a direct status day.
	ExcusedRequests absenceSvc.ExcusedAbsenceRequestService

	// AbsenceNotifier informs the child's group and the office that an absence
	// was reported. Optional and best-effort, after-commit only.
	AbsenceNotifier notificationsSvc.AbsenceNotifier
	// StaffParentMessageNotifier informs responsible staff after a guardian
	// message commits. Optional and best-effort, after-commit only.
	ParentMessageNotifier notificationsSvc.StaffParentMessageNotifier

	// Emitter posts notification pills into the child's parent-OGS thread for
	// self-service actions (sick note, one-day pickup change) and master-data
	// request submissions. Best-effort, after-commit only.
	Emitter *parentmessaging.Emitter

	// Meal plan (Essensplan) public capability for the child's school.
	MealPlan MealPlan

	// Booked care offerings read view (#1665). The offering side is reached
	// through the approved enrollment behind the child.
	CarePeriods      enrollmentSvc.StudentCarePeriodReader
	OfferingHistory  enrollmentSvc.OfferingHistoryReader
	CareOfferingRepo enrollmentModels.CareOfferingRepository
	// OfferingChanges owns the post-enrollment change-request lifecycle; this
	// service only authorizes the guardian and hands over.
	OfferingChanges enrollmentSvc.OfferingChangeRequestService

	// Parent-OGS messaging.
	MessageThreadRepo usersModels.ParentMessageThreadRepository
	MessageRepo       usersModels.ParentMessageRepository
	MessageReadRepo   usersModels.ParentMessageReadRepository

	// Parent announcements (broadcast news feed).
	AnnouncementRepo usersModels.ParentAnnouncementRepository

	// Related-accounts management (invite/remove further guardians from the
	// parents portal). The invitation service runs the shared resolve logic.
	GuardianInvites     authService.GuardianInvitationService
	GuardianInviteRepo  authModels.GuardianInvitationRepository
	StudentGuardianRepo usersModels.StudentGuardianRepository

	// Stammdaten view + change flow (Track A direct edit, Track B requests).
	PersonRepo                usersModels.PersonRepository
	ChangeRequestRepo         usersModels.StudentDataChangeRequestRepository
	CareRequestRepo           scheduleModels.CareScheduleChangeRequestRepository
	ExcusedRequestRepo        activeModels.ExcusedAbsenceRequestRepository
	OfferingChangeRequestRepo enrollmentModels.OfferingChangeRequestRepository
	FamilyProtectionEvents    usersModels.FamilyProtectionEventRepository
	ParentRequestShares       usersModels.ParentRequestShareEventRepository
	StudentAudit              usersSvc.StudentChangeRecorder
	// ParentRequestEvents appends to the append-only parent-request ledger
	// inside the ambient transaction. Nil skips recording (tests).
	ParentRequestEvents usersSvc.ParentRequestEventRecorder

	// Guardian contact + pickup editing (#1667). The phone repo backs both the
	// caller's primary-phone master-data edit and the wholesale phone-list replace
	// on a contact edit; the audit repo records every contact-field and
	// pickup/emergency flag change (append-only).
	GuardianPhoneRepo       usersModels.GuardianPhoneNumberRepository
	GuardianChangeAuditRepo auditModels.GuardianChangeRepository
	StudentConsents         usersSvc.StudentConsentService
	StudentPhotos           usersSvc.StudentPhotoService

	DB     *bun.DB
	Logger *slog.Logger
	Now    func() time.Time
}

type enrollmentSettingsQueries interface {
	EnrollmentEnabledForTenants(ctx context.Context, tenantIDs []int64) (map[int64]bool, error)
}

type service struct {
	ServiceConfig
}

// NewService wires a parent-portal service.
func NewService(cfg ServiceConfig) Service {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Now == nil {
		cfg.Now = timezone.Now
	}
	return &service{ServiceConfig: cfg}
}

func (s *service) now() time.Time {
	if s.Now == nil {
		return timezone.Now()
	}
	return s.Now()
}

func (s *service) todayDate() timezone.Date {
	return timezone.DateFromTime(s.now())
}

// AbsenceNotifierSetter injects the sick/excused producer after construction.
//
// Deliberately a standalone interface rather than a method on Service: the
// notification stack is wired later than this service, and widening Service
// would force every test double to grow a method none of them call.
type AbsenceNotifierSetter interface {
	SetAbsenceNotifier(notifier notificationsSvc.AbsenceNotifier)
}

// StudentPhotoSetter injects the photo lifecycle after API bootstrap. The
// unlinker lives in the API layer, so the parent service cannot receive this
// dependency during the main service-factory construction.
type StudentPhotoSetter interface {
	SetStudentPhotos(photos usersSvc.StudentPhotoService)
}

// SetStudentPhotos implements StudentPhotoSetter.
func (s *service) SetStudentPhotos(photos usersSvc.StudentPhotoService) {
	s.StudentPhotos = photos
}

// SetAbsenceNotifier implements AbsenceNotifierSetter.
func (s *service) SetAbsenceNotifier(notifier notificationsSvc.AbsenceNotifier) {
	s.AbsenceNotifier = notifier
}

func (s *service) GetProfile(ctx context.Context, accountID int64) (*Profile, error) {
	if accountID <= 0 {
		return nil, fmt.Errorf("parent: account_id must be positive")
	}
	if s.GuardianProfileRepo == nil {
		return nil, fmt.Errorf("parent: guardian profile repo not wired")
	}
	profile := &Profile{Locale: localization.DefaultLocale(), Explicit: false}
	err := tenant.WithAdminTx(ctx, s.DB, func(adminCtx context.Context, _ bun.Tx) error {
		row, err := s.GuardianProfileRepo.FindByAccountID(adminCtx, accountID)
		if err != nil {
			return err
		}
		if row != nil {
			profile.FirstName = row.FirstName
			profile.LastName = row.LastName
		}
		// portal_locale IS NULL → the guardian has never chosen a portal
		// language. Leave Explicit=false so the caller keeps the anonymous
		// locale instead of forcing the default.
		if row != nil && row.PortalLocale != nil && *row.PortalLocale != "" {
			profile.Locale = localization.NormalizeLocale(*row.PortalLocale)
			profile.Explicit = true
		}
		return nil
	})
	if err != nil {
		// No guardian_profiles row for an authenticated parent is a
		// data-integrity fault, not a normal state: the guardian role and the
		// profile link are written in the same transaction (auth
		// guardianInvitationService.linkProfileToAccount / enrollment
		// decisionService.ensureGuardianRoleForTenant), and these endpoints sit
		// behind ParentMiddleware, which login only reaches once the account
		// has that role. Log it so the corruption is observable, then degrade
		// to the anonymous locale rather than failing a read just to render a
		// language. Any other error is a real fault and must surface unmasked.
		if errors.Is(err, usersModels.ErrGuardianProfileNotFound) {
			s.Logger.Warn("parent: guardian profile missing for authenticated account",
				slog.Int64("account_id", accountID),
			)
			return &Profile{Locale: localization.DefaultLocale(), Explicit: false}, nil
		}
		return nil, fmt.Errorf("parent: get profile: %w", err)
	}
	return profile, nil
}

func (s *service) UpdatePortalLocale(ctx context.Context, accountID int64, locale string) (*Profile, error) {
	if accountID <= 0 {
		return nil, fmt.Errorf("parent: account_id must be positive")
	}
	if s.GuardianProfileRepo == nil {
		return nil, fmt.Errorf("parent: guardian profile repo not wired")
	}
	// Reject unknown locales rather than letting NormalizeLocale silently coerce
	// them to the default — persisting a bad value as 'de' is exactly the kind
	// of invisible fallback this codebase forbids. The handler validates too,
	// but the service is independently reachable, so it must not trust callers.
	if !localization.IsSupported(locale) {
		return nil, fmt.Errorf("parent: unsupported locale %q", locale)
	}
	normalized := localization.NormalizeLocale(locale)
	profile := &Profile{Locale: normalized, Explicit: true}
	err := tenant.WithAdminTx(ctx, s.DB, func(adminCtx context.Context, _ bun.Tx) error {
		if err := s.GuardianProfileRepo.UpdatePortalLocaleByAccountID(adminCtx, accountID, normalized); err != nil {
			return err
		}
		row, err := s.GuardianProfileRepo.FindByAccountID(adminCtx, accountID)
		if err != nil {
			return err
		}
		if row != nil {
			profile.FirstName = row.FirstName
			profile.LastName = row.LastName
		}
		return nil
	})
	if err != nil {
		// Same invariant as GetProfile: an authenticated parent always has a
		// linked guardian_profiles row, so zero rows updated is data corruption,
		// not an expected empty state. Log it and keep the sentinel wrapped so
		// the handler can map it to 409 (a permanent state conflict) instead of
		// a generic 500. Never swallow it into a fake success — a "saved"
		// preference that never hit the DB must not masquerade as persisted.
		if errors.Is(err, usersModels.ErrGuardianProfileNotFound) {
			s.Logger.Warn("parent: cannot persist portal locale, no guardian profile for account",
				slog.Int64("account_id", accountID),
			)
		}
		return nil, fmt.Errorf("parent: update portal locale: %w", err)
	}
	return profile, nil
}

func (s *service) ListChildrenForAccount(ctx context.Context, accountID int64) ([]*parentModels.ChildSummary, error) {
	if accountID <= 0 {
		return nil, fmt.Errorf("parent: account_id must be positive")
	}

	// Wrap in admin tx — the cross-tenant JOIN in the repo can't run
	// under phoenix_tenant or phoenix_auth because RLS on every joined
	// table requires app.current_tenant_id, and the parent-scope JWT
	// has none. Admin role with BYPASSRLS sees every tenant; the
	// account_tenants WHERE clause in the query keeps the result set
	// scoped to the caller's own membership.
	var children []*parentModels.ChildSummary
	err := tenant.WithAdminTx(ctx, s.DB, func(adminCtx context.Context, _ bun.Tx) error {
		list, listErr := s.ChildRepo.ListByAccount(adminCtx, accountID)
		if listErr != nil {
			return listErr
		}
		children = list
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("parent: list children: %w", err)
	}

	s.Logger.Debug("parent: listed children",
		slog.Int64("account_id", accountID),
		slog.Int("count", len(children)),
	)
	return children, nil
}

// ListEnrollableForAccount returns the (school, open phase) pairs the
// parent can enroll a new child at. Same admin-tx wrap as the children
// query — the join crosses tenant_id boundaries scoped by
// auth.account_tenants membership for the AlreadyLinked flag.
func (s *service) ListEnrollableForAccount(ctx context.Context, accountID int64) ([]*parentModels.EnrollablePhase, error) {
	if accountID <= 0 {
		return nil, fmt.Errorf("parent: account_id must be positive")
	}

	var phases []*parentModels.EnrollablePhase
	err := tenant.WithAdminTx(ctx, s.DB, func(adminCtx context.Context, _ bun.Tx) error {
		list, listErr := s.EnrollablePhaseRepo.ListEnrollable(adminCtx, accountID)
		if listErr != nil {
			return listErr
		}
		phases = list
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("parent: list enrollable: %w", err)
	}
	if len(phases) > 0 {
		if s.EnrollmentSettings == nil {
			return nil, errors.New("parent: enrollment settings queries not wired")
		}
		tenantIDs := make([]int64, 0, len(phases))
		seen := make(map[int64]struct{}, len(phases))
		for _, phase := range phases {
			if _, exists := seen[phase.SchoolID]; exists {
				continue
			}
			seen[phase.SchoolID] = struct{}{}
			tenantIDs = append(tenantIDs, phase.SchoolID)
		}
		enabled, settingsErr := s.EnrollmentSettings.EnrollmentEnabledForTenants(ctx, tenantIDs)
		if settingsErr != nil {
			return nil, fmt.Errorf("parent: resolve enrollment settings: %w", settingsErr)
		}
		filtered := phases[:0]
		for _, phase := range phases {
			if enabled[phase.SchoolID] {
				filtered = append(filtered, phase)
			}
		}
		phases = filtered
	}

	s.Logger.Debug("parent: listed enrollable phases",
		slog.Int64("account_id", accountID),
		slog.Int("count", len(phases)),
	)
	return phases, nil
}

// GetEnrollmentSubmitStatus resolves the (account, school) submit
// facts for the parent enrollment path (#1663). Uses
// WithAdminTxOrDirect so the parent submit handler's existing admin
// transaction is reused instead of opening a nested one.
func (s *service) GetEnrollmentSubmitStatus(ctx context.Context, accountID, schoolID int64) (*parentModels.GuardianSubmitStatus, error) {
	if accountID <= 0 || schoolID <= 0 {
		return nil, fmt.Errorf("parent: account_id and school_id must be positive")
	}

	var status *parentModels.GuardianSubmitStatus
	err := tenant.WithAdminTxOrDirect(ctx, s.DB, func(adminCtx context.Context) error {
		st, stErr := s.EnrollablePhaseRepo.GuardianSubmitStatus(adminCtx, accountID, schoolID)
		if stErr != nil {
			return stErr
		}
		status = st
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("parent: enrollment submit status: %w", err)
	}
	return status, nil
}

// ListEnrollmentsForAccount returns the parent's enrollment.requests
// rows joined to phase + school + child summaries. Same admin-tx wrap
// as the other parent queries — this crosses tenant_id boundaries.
func (s *service) ListEnrollmentsForAccount(ctx context.Context, accountID int64) ([]*parentModels.EnrollmentRequestSummary, error) {
	if accountID <= 0 {
		return nil, fmt.Errorf("parent: account_id must be positive")
	}
	if s.EnrollmentRequestRepo == nil {
		return nil, fmt.Errorf("parent: enrollment request repo not wired")
	}

	var requests []*parentModels.EnrollmentRequestSummary
	err := tenant.WithAdminTx(ctx, s.DB, func(adminCtx context.Context, _ bun.Tx) error {
		list, listErr := s.EnrollmentRequestRepo.ListByAccount(adminCtx, accountID)
		if listErr != nil {
			return listErr
		}
		requests = list
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("parent: list enrollments: %w", err)
	}

	// Redact admin-internal status reasons unless the owning phase opts
	// in via show_status_reason_to_parent. Same gate the public status
	// page and the decision email apply — keeps internal rejection /
	// waitlist notes out of the parent dashboard payload.
	for _, req := range requests {
		if req.ShowStatusReasonToParent {
			continue
		}
		for i := range req.Children {
			req.Children[i].StatusReason = nil
		}
	}

	s.Logger.Debug("parent: listed enrollment requests",
		slog.Int64("account_id", accountID),
		slog.Int("count", len(requests)),
	)
	return requests, nil
}
