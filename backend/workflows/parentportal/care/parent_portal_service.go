// Package legacy holds the cross-tenant guardian-portal services.
//
// All methods take an account ID rather than reading tenant from
// context because parent-scope JWTs intentionally carry tenant_id=0.
// Per-action tenant context is decided by the picked child on the
// frontend, then validated server-side via the auth.account_tenants
// membership check that already lives in the underlying repos.
package care

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
)

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

// ChildFeatureFlags reports the resolved per-tenant parent-portal feature
// toggles for a single child.
type ChildFeatureFlags struct {
	// CareEnded says the child has left the OGS (#2487). It is STATE, not a
	// capability: when it is true every write flag below is false, and the
	// portal shows a read-only profile with one sentence explaining why
	// instead of buttons that would all fail the same way.
	CareEnded          bool
	SickNoteEnabled    bool
	ExcusedNoteEnabled bool
	// SickRequiresApproval is true when a Krankmeldung stays pending until the
	// OGS confirms it (operations.parent_sick_requires_approval, #2449).
	SickRequiresApproval bool
	// ExcusedRequiresApproval is true when an "excused" report must be confirmed
	// by the office before it takes effect (operations.parent_excused_requires_approval,
	// #1845). The parent UI uses it to explain that the absence will be pending
	// and to keep the mandatory-note requirement visible. Only meaningful while
	// ExcusedNoteEnabled is true.
	ExcusedRequiresApproval bool
	NotesEnabled            bool
	// RequestSubmitEnabled is true when messaging is on AND the guardian holds
	// parent_portal.request.submit for this child — gates the change-request
	// quick actions (care schedule / master data) in the parent UI.
	RequestSubmitEnabled bool
	PickupChangeEnabled  bool
	// PickupChangeCutoffTime is the school's same-day cutoff for the one-day
	// pickup change as HH:MM (#3163); empty when there is none or the pickup
	// change is off. PickupChangeTodayClosed says the cutoff has passed, so
	// today is closed for guardians while later days stay open. The portal
	// shows both before anyone types, instead of rebuilding the rule.
	PickupChangeCutoffTime  string
	PickupChangeTodayClosed bool
	PickupManageAllowed     bool
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

// StudentPhotoUnlinker is the part of the photo lifecycle a photo-consent
// withdrawal needs: the stored file goes once the withdrawal commits.
type StudentPhotoUnlinker interface {
	ScheduleUnlinkAfterCommit(ctx context.Context, path string)
}

func (s *Service) GetProfile(ctx context.Context, accountID int64) (*Profile, error) {
	if accountID <= 0 {
		return nil, fmt.Errorf("parent: account_id must be positive")
	}
	if s.GuardianProfileRepo == nil {
		return nil, fmt.Errorf("parent: guardian profile repo not wired")
	}
	profile := &Profile{Locale: s.Locales.Default(), Explicit: false}
	err := tenant.WithinAdmin(ctx, func(adminCtx context.Context) error {
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
			profile.Locale = s.Locales.Normalize(*row.PortalLocale)
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
			return &Profile{Locale: s.Locales.Default(), Explicit: false}, nil
		}
		return nil, fmt.Errorf("parent: get profile: %w", err)
	}
	return profile, nil
}

func (s *Service) UpdatePortalLocale(ctx context.Context, accountID int64, locale string) (*Profile, error) {
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
	if !s.Locales.IsSupported(locale) {
		return nil, fmt.Errorf("parent: unsupported locale %q", locale)
	}
	normalized := s.Locales.Normalize(locale)
	if err := s.writePortalLocale(ctx, accountID, normalized); err != nil {
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
	profile := &Profile{Locale: normalized, Explicit: true}
	err := tenant.WithinAdmin(ctx, func(adminCtx context.Context) error {
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
		return nil, fmt.Errorf("parent: update portal locale: %w", err)
	}
	return profile, nil
}

// writePortalLocale stores the account's portal language on its guardian
// profile in every school it has one, one tenant unit of work per school:
// People Directory writes only inside the school in context. Finding the
// schools is a cross-tenant read; no write runs in the administrative unit of
// work.
func (s *Service) writePortalLocale(ctx context.Context, accountID int64, locale string) error {
	if s.Guardians == nil {
		return errors.New("parent: guardian records are not configured")
	}
	var tenantIDs []int64
	err := tenant.WithinAdmin(ctx, func(adminCtx context.Context) error {
		var readErr error
		tenantIDs, readErr = s.Guardians.ListGuardianProfileTenants(adminCtx, accountID)
		return readErr
	})
	if err != nil {
		return err
	}
	var updated int64
	for _, tenantID := range tenantIDs {
		err := InTenant(ctx, tenantID, func(txCtx context.Context) error {
			rows, writeErr := s.Guardians.SetGuardianPortalLocale(txCtx, accountID, locale)
			updated += rows
			return writeErr
		})
		if err != nil {
			return err
		}
	}
	if updated == 0 {
		return usersModels.ErrGuardianProfileNotFound
	}
	return nil
}

func (s *Service) ListChildrenForAccount(ctx context.Context, accountID int64) ([]*parentModels.ChildSummary, error) {
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
	err := tenant.WithinAdmin(ctx, func(adminCtx context.Context) error {
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
func (s *Service) ListEnrollableForAccount(ctx context.Context, accountID int64) ([]*parentModels.EnrollablePhase, error) {
	if accountID <= 0 {
		return nil, fmt.Errorf("parent: account_id must be positive")
	}

	var phases []*parentModels.EnrollablePhase
	err := tenant.WithinAdmin(ctx, func(adminCtx context.Context) error {
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
		phases, err = s.filterEnrollmentEnabledPhases(ctx, phases)
		if err != nil {
			return nil, err
		}
	}

	s.Logger.Debug("parent: listed enrollable phases",
		slog.Int64("account_id", accountID),
		slog.Int("count", len(phases)),
	)
	return phases, nil
}

// filterEnrollmentEnabledPhases keeps the phases whose school has enrollment
// switched on, resolved in one cross-tenant settings query.
func (s *Service) filterEnrollmentEnabledPhases(ctx context.Context, phases []*parentModels.EnrollablePhase) ([]*parentModels.EnrollablePhase, error) {
	if s.EnrollmentSettings == nil {
		return nil, errors.New("parent: enrollment settings queries not wired")
	}
	enabled, settingsErr := s.EnrollmentSettings.EnrollmentEnabledForTenants(ctx, enrollablePhaseSchoolIDs(phases))
	if settingsErr != nil {
		return nil, fmt.Errorf("parent: resolve enrollment settings: %w", settingsErr)
	}
	filtered := phases[:0]
	for _, phase := range phases {
		if enabled[phase.SchoolID] {
			filtered = append(filtered, phase)
		}
	}
	return filtered, nil
}

// enrollablePhaseSchoolIDs returns the distinct school IDs of the phases in
// first-seen order.
func enrollablePhaseSchoolIDs(phases []*parentModels.EnrollablePhase) []int64 {
	tenantIDs := make([]int64, 0, len(phases))
	seen := make(map[int64]struct{}, len(phases))
	for _, phase := range phases {
		if _, exists := seen[phase.SchoolID]; exists {
			continue
		}
		seen[phase.SchoolID] = struct{}{}
		tenantIDs = append(tenantIDs, phase.SchoolID)
	}
	return tenantIDs
}

// GetEnrollmentSubmitStatus resolves the (account, school) submit
// facts for the parent enrollment path (#1663). Uses
// WithAdminTxOrDirect so the parent submit handler's existing admin
// transaction is reused instead of opening a nested one.
func (s *Service) GetEnrollmentSubmitStatus(ctx context.Context, accountID, schoolID int64) (*parentModels.GuardianSubmitStatus, error) {
	if accountID <= 0 || schoolID <= 0 {
		return nil, fmt.Errorf("parent: account_id and school_id must be positive")
	}

	var status *parentModels.GuardianSubmitStatus
	err := tenant.WithinAdmin(ctx, func(adminCtx context.Context) error {
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
func (s *Service) ListEnrollmentsForAccount(ctx context.Context, accountID int64) ([]*parentModels.EnrollmentRequestSummary, error) {
	if accountID <= 0 {
		return nil, fmt.Errorf("parent: account_id must be positive")
	}
	if s.EnrollmentRequestRepo == nil {
		return nil, fmt.Errorf("parent: enrollment request repo not wired")
	}

	var requests []*parentModels.EnrollmentRequestSummary
	err := tenant.WithinAdmin(ctx, func(adminCtx context.Context) error {
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
