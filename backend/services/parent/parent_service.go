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
	"fmt"
	"log/slog"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	configService "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// ParentNoteDisplayLimit is how many of the newest parent notes the
// portal and the staff view surface by default.
const ParentNoteDisplayLimit = 3

// Service is the public contract consumed by HTTP handlers.
type Service interface {
	// ListChildrenForAccount returns every child linked to any
	// guardian profile owned by the account, across every active
	// tenant mapping. Sorted by school then by name.
	ListChildrenForAccount(ctx context.Context, accountID int64) ([]*parentModels.ChildSummary, error)

	// ListEnrollableForAccount returns every (school, open phase)
	// pair the parent could enroll a new child at, with a flag for
	// schools they're already linked to. Sorted with linked schools
	// first.
	ListEnrollableForAccount(ctx context.Context, accountID int64) ([]*parentModels.EnrollablePhase, error)

	// ListEnrollmentsForAccount returns every enrollment.requests row
	// where guardian_account_id matches the calling account, joined
	// to phase + school + child summaries. Used by the dashboard to
	// surface in-progress / decided submissions without the parent
	// having to dig out the email-link status URL.
	ListEnrollmentsForAccount(ctx context.Context, accountID int64) ([]*parentModels.EnrollmentRequestSummary, error)

	// SubmitSickNote reports the parent's child sick for one or more
	// dates. Authorization: the account must be a guardian of the
	// student. The write mirrors the staff sick-note path — it clears any
	// opposing "excused" days, upserts a sick status-day per date with
	// source=parent, and flips the live sick flag when today is included.
	// Gated by operations.parent_sick_note_enabled for the child's tenant.
	SubmitSickNote(ctx context.Context, accountID, studentID int64, dates []timezone.Date, reason string) ([]*activeModels.StudentStatusDay, error)

	// ListSickDays returns the child's currently-active sick days in the
	// given date range (excused days are filtered out — parents only
	// manage sick notes). Authorization only; not gated by the setting so
	// previously-reported days stay visible if a school later disables the
	// feature.
	ListSickDays(ctx context.Context, accountID, studentID int64, from, to timezone.Date) ([]*activeModels.StudentStatusDay, error)

	// AddParentNote appends a free-text note the parent left for the
	// team and returns the newest ParentNoteDisplayLimit notes. Gated by
	// operations.parent_notes_enabled for the child's tenant.
	AddParentNote(ctx context.Context, accountID, studentID int64, body string) ([]*usersModels.StudentParentNote, error)

	// ListParentNotes returns the newest notes for the child (limit <= 0
	// uses ParentNoteDisplayLimit). Authorization only — reads are not
	// gated by the setting so previously-left notes stay visible.
	ListParentNotes(ctx context.Context, accountID, studentID int64, limit int) ([]*usersModels.StudentParentNote, error)

	// ChildFeatures resolves which parent-portal write features are enabled
	// for the child's tenant, so the UI can hide/disable actions the backend
	// would reject with 403. Authorization only.
	ChildFeatures(ctx context.Context, accountID, studentID int64) (ChildFeatureFlags, error)
}

// ChildFeatureFlags reports the resolved per-tenant parent-portal feature
// toggles for a single child.
type ChildFeatureFlags struct {
	SickNoteEnabled bool
	NotesEnabled    bool
}

// ServiceConfig is the dependency-injection bundle.
type ServiceConfig struct {
	ChildRepo             parentModels.ChildRepository
	EnrollablePhaseRepo   parentModels.EnrollablePhaseRepository
	EnrollmentRequestRepo parentModels.EnrollmentRequestRepository

	// Per-child write features (sick notes + parent notes).
	StatusDayRepo activeModels.StudentStatusDayRepository
	StudentRepo   usersModels.StudentRepository
	NoteRepo      usersModels.StudentParentNoteRepository
	Settings      configService.SettingsService
	Broadcaster   realtime.Broadcaster

	DB     *bun.DB
	Logger *slog.Logger
}

type service struct {
	childRepo             parentModels.ChildRepository
	enrollablePhaseRepo   parentModels.EnrollablePhaseRepository
	enrollmentRequestRepo parentModels.EnrollmentRequestRepository

	statusDayRepo activeModels.StudentStatusDayRepository
	studentRepo   usersModels.StudentRepository
	noteRepo      usersModels.StudentParentNoteRepository
	settings      configService.SettingsService
	broadcaster   realtime.Broadcaster

	db     *bun.DB
	logger *slog.Logger
}

// NewService wires a parent-portal service.
func NewService(cfg ServiceConfig) Service {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &service{
		childRepo:             cfg.ChildRepo,
		enrollablePhaseRepo:   cfg.EnrollablePhaseRepo,
		enrollmentRequestRepo: cfg.EnrollmentRequestRepo,
		statusDayRepo:         cfg.StatusDayRepo,
		studentRepo:           cfg.StudentRepo,
		noteRepo:              cfg.NoteRepo,
		settings:              cfg.Settings,
		broadcaster:           cfg.Broadcaster,
		db:                    cfg.DB,
		logger:                logger,
	}
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
	err := tenant.WithAdminTx(ctx, s.db, func(adminCtx context.Context, _ bun.Tx) error {
		list, listErr := s.childRepo.ListByAccount(adminCtx, accountID)
		if listErr != nil {
			return listErr
		}
		children = list
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("parent: list children: %w", err)
	}

	s.logger.Debug("parent: listed children",
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
	err := tenant.WithAdminTx(ctx, s.db, func(adminCtx context.Context, _ bun.Tx) error {
		list, listErr := s.enrollablePhaseRepo.ListEnrollable(adminCtx, accountID)
		if listErr != nil {
			return listErr
		}
		phases = list
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("parent: list enrollable: %w", err)
	}

	s.logger.Debug("parent: listed enrollable phases",
		slog.Int64("account_id", accountID),
		slog.Int("count", len(phases)),
	)
	return phases, nil
}

// ListEnrollmentsForAccount returns the parent's enrollment.requests
// rows joined to phase + school + child summaries. Same admin-tx wrap
// as the other parent queries — this crosses tenant_id boundaries.
func (s *service) ListEnrollmentsForAccount(ctx context.Context, accountID int64) ([]*parentModels.EnrollmentRequestSummary, error) {
	if accountID <= 0 {
		return nil, fmt.Errorf("parent: account_id must be positive")
	}
	if s.enrollmentRequestRepo == nil {
		return nil, fmt.Errorf("parent: enrollment request repo not wired")
	}

	var requests []*parentModels.EnrollmentRequestSummary
	err := tenant.WithAdminTx(ctx, s.db, func(adminCtx context.Context, _ bun.Tx) error {
		list, listErr := s.enrollmentRequestRepo.ListByAccount(adminCtx, accountID)
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

	s.logger.Debug("parent: listed enrollment requests",
		slog.Int64("account_id", accountID),
		slog.Int("count", len(requests)),
	)
	return requests, nil
}
