// Post-enrollment offering changes (#1665): a guardian requests a different
// set of care offerings / AGs for a child that is already enrolled, effective
// on a chosen date; staff decide it on the central "Änderungsanfragen" page,
// and an approval applies the switch through the same adjustment path an admin
// correction uses — dated, so the groups attended before the switch stay on the
// record.
//
// Why not enrollment.change_requests: those rows are whole-form corrections of
// a submission whose review window is still open, carrying a form snapshot and
// the submission-window rules. A mid-year switch shares neither.
package enrollment

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/parentmessaging"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// OfferingChangeRequestType is the request_type stamped on the chat pills, so
// a localized client can tell this apart from care_schedule / master_data.
const OfferingChangeRequestType = "care_offering"

// offeringChangeMaxNoteLen bounds free text (in runes) from either side so a
// direct API client cannot persist an unbounded payload. Matches the limit the
// care-schedule requests use for the same fields.
const offeringChangeMaxNoteLen = 2000

// German pill texts. The staff portal renders these directly; the parents
// portal localizes from the structured event fields.
const (
	offeringChangeCreatedBody   = "Anfrage: Betreuungsangebote ändern"
	offeringChangeApprovedBody  = "Anfrage bestätigt, Betreuungsangebote werden umgestellt"
	offeringChangeRejectedBody  = "Anfrage abgelehnt"
	offeringChangeWithdrawnBody = "Anfrage zurückgezogen"
)

var (
	// ErrOfferingChangeDisabled means the school has post-enrollment changes
	// switched off.
	ErrOfferingChangeDisabled = errors.New("enrollment: post-enrollment offering changes are disabled")
	// ErrOfferingChangeInvalid marks a request the caller can correct: no
	// selection, an unknown offering, a bad effective date.
	ErrOfferingChangeInvalid = errors.New("enrollment: invalid offering change request")
	// ErrOfferingChangeNoEnrollment means the child has no approved enrollment
	// an offering change could be applied to.
	ErrOfferingChangeNoEnrollment = errors.New("enrollment: child has no approved enrollment")
	// ErrOfferingChangeForbidden means the caller does not own the request.
	ErrOfferingChangeForbidden = errors.New("enrollment: offering change request forbidden")
	// ErrOfferingChangeCapacityFull means an offering in the request has no free
	// slot left. Raised at approval time, when it actually matters.
	ErrOfferingChangeCapacityFull = errors.New("enrollment: care offering is at capacity")
)

// OfferingChangeSelection is one desired offering with its chosen days.
type OfferingChangeSelection struct {
	OfferingID   int64
	SelectedDays []string
}

// OfferingChangeCatalogItem is one offering a guardian can pick, with the
// booking state and the capacity signal the modal shows.
type OfferingChangeCatalogItem struct {
	OfferingID      int64
	Name            string
	Description     string
	DaysOfWeekMode  string
	AvailableDays   []string
	SelectionGroup  string
	SelectionRule   string
	IsRequired      bool
	PriceCents      *int
	IncludesLunch   bool
	IncludesHoliday bool
	// Selected marks the child's current booking, so the modal opens prefilled.
	Selected     bool
	SelectedDays []string
	// IsActive is false only for a held offering that was removed from the
	// catalog. It remains visible to preserve the existing booking.
	IsActive bool
	// Capacity is nil for unlimited; FreeSlots is nil then too. FreeSlots can be
	// zero for an offering the child already holds — losing the slot by editing
	// an unrelated day would be a nasty surprise, so the frontend shows "voll"
	// but keeps an already-selected offering selectable.
	Capacity  *int
	FreeSlots *int
}

// OfferingChangeCatalog is everything the parent modal needs.
type OfferingChangeCatalog struct {
	PhaseID   int64
	PhaseName string
	// SelectionMode is the phase's care_offering_selection_mode, which decides
	// whether picking nothing is allowed.
	SelectionMode string
	// EarliestEffectiveFrom / LatestEffectiveFrom bound the date picker.
	EarliestEffectiveFrom timezone.Date
	LatestEffectiveFrom   timezone.Date
	Items                 []OfferingChangeCatalogItem
}

// OfferingChangeDiffEntry is one "current → requested" line, shared by the
// parent read view and the staff review card.
type OfferingChangeDiffEntry struct {
	Label string
	Old   string
	New   string
}

// OfferingChangeView is a request as both sides see it.
type OfferingChangeView struct {
	Request *enrollmentModels.OfferingChangeRequest
	// StudentName / GroupLabel are filled for the staff queue only; the parents
	// portal already knows whose child it is.
	StudentName string
	Diff        []OfferingChangeDiffEntry
	// LastDecision is the most recent decided request inside the recency window,
	// so a guardian learns the outcome (and a rejection's reason) on the page
	// they submitted from instead of only in the chat.
	LastDecision *OfferingChangeDecision
}

// OfferingChangeDecision is a decided request as the guardian sees it. It
// carries no diff: after an approval was applied the "current → requested"
// comparison is empty, which would read as "nothing changed".
type OfferingChangeDecision struct {
	ID     int64
	Status string
	// DecidedAt is when staff decided it.
	DecidedAt time.Time
	// EffectiveFrom is the date the switch took (or would have taken) effect.
	EffectiveFrom timezone.Date
	Reason        string
	// Requested is what the family asked for, read back from the stored payload.
	// A rejection is only understandable next to the request it refers to, and a
	// live "current → requested" comparison cannot serve here: after an approval
	// was applied it is empty, and after a rejection the payload is the only
	// record of the wish.
	Requested []OfferingChangeRequestedItem
}

// OfferingChangeRequestedItem is one offering of a decided request, resolved to
// its name and the booked days at read time.
type OfferingChangeRequestedItem struct {
	OfferingID int64
	Name       string
	// Days are the requested day abbreviations ("mon"), empty when the offering
	// covers every care day.
	Days []string
}

// offeringDecisionRecencyDays bounds how long a decided request keeps being
// reported. Same window the excused-absence requests use for the same purpose:
// long enough that a guardian sees the outcome on their next visit, short
// enough that the card does not turn into a history list.
const offeringDecisionRecencyDays = 14

// CreateOfferingChangeInput is a guardian's submission.
type CreateOfferingChangeInput struct {
	StudentID     int64
	AccountID     int64
	Selections    []OfferingChangeSelection
	EffectiveFrom timezone.Date
	Note          string
}

// DecideOfferingChangeInput is a staff decision.
type DecideOfferingChangeInput struct {
	RequestID  int64
	Approve    bool
	Reason     string
	ReviewedBy int64
	ActorRole  string
}

// OfferingChangeRequestService is the lifecycle contract. Every method runs
// inside a tenant transaction opened by its caller.
type OfferingChangeRequestService interface {
	// Catalog returns the offerings a guardian may choose from for this child.
	Catalog(ctx context.Context, studentID int64) (*OfferingChangeCatalog, error)

	// GetForStudent returns the child's open request (nil when none) plus its
	// live diff against the current booking.
	GetForStudent(ctx context.Context, studentID int64) (*OfferingChangeView, error)

	// Create stores a pending request after validating it exactly as an
	// approval would, so a request that cannot be applied is never accepted.
	Create(ctx context.Context, input CreateOfferingChangeInput) (*enrollmentModels.OfferingChangeRequest, error)

	// Withdraw flips the submitting guardian's own pending request to withdrawn.
	Withdraw(ctx context.Context, requestID, accountID, studentID int64) error

	// ListPending backs the staff review queue.
	ListPending(ctx context.Context) ([]*OfferingChangeView, error)

	// Decide approves (and applies) or rejects a pending request.
	Decide(ctx context.Context, input DecideOfferingChangeInput) error

	// EarliestEffectiveFrom is the first date a switch may take effect under
	// the school's configured lead time.
	EarliestEffectiveFrom(ctx context.Context) (timezone.Date, error)
}

// OfferingChangeRequestServiceConfig wires the service.
type OfferingChangeRequestServiceConfig struct {
	ChangeRepo               enrollmentModels.OfferingChangeRequestRepository
	RequestChildRepo         enrollmentModels.RequestChildRepository
	RequestRepo              enrollmentModels.RequestRepository
	PhaseRepo                enrollmentModels.PhaseRepository
	CareOfferingRepo         enrollmentModels.CareOfferingRepository
	RequestChildOfferingRepo enrollmentModels.RequestChildOfferingRepository
	StudentRepo              usersModels.StudentRepository
	PersonRepo               usersModels.PersonRepository
	UserContext              authorize.StudentModifyUserContext
	// Applier performs the dated adjustment on approval. It is the decision
	// service, reached through the same narrow interface the change-request
	// service uses.
	Applier  ChangeRequestDecisionApplier
	Settings DecisionSettingsResolver
	Emitter  *parentmessaging.Emitter
	Logger   *slog.Logger
}

type offeringChangeRequestService struct {
	OfferingChangeRequestServiceConfig
}

// NewOfferingChangeRequestService wires a fresh service.
func NewOfferingChangeRequestService(cfg OfferingChangeRequestServiceConfig) OfferingChangeRequestService {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &offeringChangeRequestService{OfferingChangeRequestServiceConfig: cfg}
}

// OfferingChangeLeadDaysDefault mirrors the registry default so a caller
// without a settings service still gets the intended notice period.
const OfferingChangeLeadDaysDefault = 14

// EarliestOfferingChangeDate is the first date a requested switch may take
// effect: tomorrow at the earliest, plus the school's lead time. Exported so
// the parents portal can bound its date picker with the same rule the server
// enforces.
func EarliestOfferingChangeDate(today timezone.Date, leadDays int) timezone.Date {
	if leadDays < 0 {
		leadDays = 0
	}
	return today.AddDays(1 + leadDays)
}

func (s *offeringChangeRequestService) EarliestEffectiveFrom(ctx context.Context) (timezone.Date, error) {
	leadDays := OfferingChangeLeadDaysDefault
	if s.Settings != nil {
		resolved, err := s.resolveLeadDays(ctx)
		if err != nil {
			return timezone.Date{}, err
		}
		leadDays = resolved
	}
	return EarliestOfferingChangeDate(timezone.TodayDate(), leadDays), nil
}

// resolveLeadDays reads the configured notice period. DecisionSettingsResolver
// deliberately exposes only string/bool, so the number is read as a string and
// parsed here rather than widening the interface for one setting.
func (s *offeringChangeRequestService) resolveLeadDays(ctx context.Context) (int, error) {
	raw, err := s.Settings.ResolveString(ctx, configModel.KeyEnrollmentOfferingChangesLeadDays)
	if err != nil {
		return 0, fmt.Errorf("offering change: resolve lead days: %w", err)
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, fmt.Errorf("offering change: lead-days setting is empty")
	}
	days, convErr := strconv.Atoi(raw)
	if convErr != nil || days < 0 {
		return 0, fmt.Errorf("offering change: lead-days setting must be a non-negative integer")
	}
	return days, nil
}

func (s *offeringChangeRequestService) changesEnabled(ctx context.Context) error {
	if s.Settings == nil {
		return ErrOfferingChangeDisabled
	}
	enabled, err := s.Settings.ResolveBool(ctx, configModel.KeyEnrollmentOfferingChangesEnabled)
	if err != nil {
		// Fail closed: a school that never switched this on must not collect
		// requests because a config read blipped.
		s.Logger.Warn("offering change: resolve enabled failed, failing closed",
			slog.String("error", err.Error()),
		)
		return ErrOfferingChangeDisabled
	}
	if !enabled {
		return ErrOfferingChangeDisabled
	}
	careOfferingsEnabled, err := s.Settings.ResolveBool(ctx, configModel.KeyEnrollmentCareOfferingsEnabled)
	if err != nil {
		s.Logger.Warn("offering change: resolve care offerings enabled failed, failing closed",
			slog.String("error", err.Error()),
		)
		return ErrCareOfferingsDisabled
	}
	if !careOfferingsEnabled {
		return ErrCareOfferingsDisabled
	}
	return nil
}

// carePeriodAt resolves the approved enrollment that covers onDate. A request
// may have to take effect in the next approved period when the notice period
// crosses a care-period boundary.
func (s *offeringChangeRequestService) carePeriodAt(
	ctx context.Context,
	studentID int64,
	onDate timezone.Date,
) (*enrollmentModels.StudentCarePeriod, *enrollmentModels.Phase, error) {
	periods, err := s.RequestChildRepo.ListCarePeriodsByStudentID(ctx, studentID)
	if err != nil {
		return nil, nil, fmt.Errorf("offering change: list care periods: %w", err)
	}
	for _, candidate := range periods {
		if !candidate.ServiceStartDate.After(onDate) && !candidate.ServiceEndDate.Before(onDate) {
			phase, phaseErr := s.PhaseRepo.FindByID(ctx, candidate.PhaseID)
			if phaseErr != nil || phase == nil {
				return nil, nil, fmt.Errorf("offering change: load phase: %w", phaseErr)
			}
			return candidate, phase, nil
		}
	}
	return nil, nil, ErrOfferingChangeNoEnrollment
}

// carePeriodAtOrNext resolves the period covering onDate, or the nearest
// approved period beginning after it. The catalog needs the latter so it can
// offer the first valid date when a lead time falls before the next period.
func (s *offeringChangeRequestService) carePeriodAtOrNext(
	ctx context.Context,
	studentID int64,
	onDate timezone.Date,
) (*enrollmentModels.StudentCarePeriod, *enrollmentModels.Phase, error) {
	periods, err := s.RequestChildRepo.ListCarePeriodsByStudentID(ctx, studentID)
	if err != nil {
		return nil, nil, fmt.Errorf("offering change: list care periods: %w", err)
	}
	var next *enrollmentModels.StudentCarePeriod
	for _, candidate := range periods {
		if !candidate.ServiceStartDate.After(onDate) && !candidate.ServiceEndDate.Before(onDate) {
			phase, phaseErr := s.PhaseRepo.FindByID(ctx, candidate.PhaseID)
			if phaseErr != nil || phase == nil {
				return nil, nil, fmt.Errorf("offering change: load phase: %w", phaseErr)
			}
			return candidate, phase, nil
		}
		if candidate.ServiceStartDate.After(onDate) &&
			(next == nil || candidate.ServiceStartDate.Before(next.ServiceStartDate)) {
			next = candidate
		}
	}
	if next == nil {
		return nil, nil, ErrOfferingChangeNoEnrollment
	}
	phase, err := s.PhaseRepo.FindByID(ctx, next.PhaseID)
	if err != nil || phase == nil {
		return nil, nil, fmt.Errorf("offering change: load phase: %w", err)
	}
	return next, phase, nil
}

func (s *offeringChangeRequestService) carePeriodForEarliestEffectiveDate(
	ctx context.Context,
	studentID int64,
) (*enrollmentModels.StudentCarePeriod, *enrollmentModels.Phase, timezone.Date, error) {
	earliest, err := s.EarliestEffectiveFrom(ctx)
	if err != nil {
		return nil, nil, timezone.Date{}, err
	}
	period, phase, err := s.carePeriodAtOrNext(ctx, studentID, earliest)
	if err != nil {
		return nil, nil, timezone.Date{}, err
	}
	if earliest.Before(phase.ServiceStartDate) {
		earliest = phase.ServiceStartDate
	}
	return period, phase, earliest, nil
}

func (s *offeringChangeRequestService) Catalog(
	ctx context.Context,
	studentID int64,
) (*OfferingChangeCatalog, error) {
	if err := s.changesEnabled(ctx); err != nil {
		return nil, err
	}
	period, phase, earliest, err := s.carePeriodForEarliestEffectiveDate(ctx, studentID)
	if err != nil {
		return nil, err
	}
	active, err := s.CareOfferingRepo.ListActiveByPhase(ctx, phase.ID)
	if err != nil {
		return nil, fmt.Errorf("offering change: list active offerings: %w", err)
	}
	current, err := s.RequestChildOfferingRepo.ListByRequestChildIDAtDate(ctx, period.RequestChildID, earliest)
	if err != nil {
		return nil, fmt.Errorf("offering change: list current offerings: %w", err)
	}
	currentByID := make(map[int64]*enrollmentModels.RequestChildOffering, len(current))
	for _, link := range current {
		if link != nil {
			currentByID[link.CareOfferingID] = link
		}
	}
	activeByID := offeringsByID(active)
	allowed := offeringsByID(active)
	if err := s.addHeldOfferingsAtDate(ctx, period.RequestChildID, earliest, allowed); err != nil {
		return nil, err
	}
	catalog := &OfferingChangeCatalog{
		PhaseID:               phase.ID,
		PhaseName:             phase.Name,
		SelectionMode:         phase.CareOfferingSelectionMode,
		EarliestEffectiveFrom: earliest,
		LatestEffectiveFrom:   phase.ServiceEndDate,
		Items:                 make([]OfferingChangeCatalogItem, 0, len(allowed)),
	}
	for _, offering := range allowed {
		if offering == nil {
			continue
		}
		item, itemErr := s.catalogItem(ctx, offering, currentByID[offering.ID], earliest)
		if itemErr != nil {
			return nil, itemErr
		}
		item.IsActive = activeByID[offering.ID] != nil
		catalog.Items = append(catalog.Items, item)
	}
	sort.SliceStable(catalog.Items, func(i, j int) bool {
		return catalog.Items[i].OfferingID < catalog.Items[j].OfferingID
	})
	return catalog, nil
}

func (s *offeringChangeRequestService) catalogItem(
	ctx context.Context,
	offering *enrollmentModels.CareOffering,
	current *enrollmentModels.RequestChildOffering,
	onDate timezone.Date,
) (OfferingChangeCatalogItem, error) {
	item := OfferingChangeCatalogItem{
		OfferingID:      offering.ID,
		Name:            offering.Name,
		DaysOfWeekMode:  offering.DaysOfWeekMode,
		AvailableDays:   append([]string(nil), offering.AvailableDays...),
		SelectionGroup:  offering.SelectionGroup,
		SelectionRule:   offering.SelectionRule,
		IsRequired:      offering.IsRequired,
		PriceCents:      offering.PriceCents,
		IncludesLunch:   offering.IncludesLunch,
		IncludesHoliday: offering.IncludesHolidayCare,
	}
	if offering.Description != nil {
		item.Description = *offering.Description
	}
	if current != nil {
		item.Selected = true
		item.SelectedDays = append([]string(nil), current.SelectedDays...)
	}
	if offering.Capacity == nil {
		return item, nil
	}
	capacity := *offering.Capacity
	item.Capacity = &capacity
	taken, err := s.RequestChildOfferingRepo.CountActiveByCareOfferingOnDate(ctx, offering.ID, onDate)
	if err != nil {
		return OfferingChangeCatalogItem{}, fmt.Errorf("offering change: count offering occupancy: %w", err)
	}
	free := max(capacity-taken, 0)
	item.FreeSlots = &free
	return item, nil
}

func (s *offeringChangeRequestService) GetForStudent(
	ctx context.Context,
	studentID int64,
) (*OfferingChangeView, error) {
	pending, err := s.ChangeRepo.GetPendingForStudent(ctx, studentID)
	if err != nil {
		return nil, fmt.Errorf("offering change: get pending: %w", err)
	}
	decision, err := s.lastDecisionForStudent(ctx, studentID)
	if err != nil {
		return nil, err
	}
	if pending == nil && decision == nil {
		return nil, nil
	}
	view := &OfferingChangeView{Request: pending, LastDecision: decision}
	if pending == nil {
		return view, nil
	}
	diff, diffErr := s.diffForRequest(ctx, pending)
	if diffErr != nil {
		// A diff we cannot build must not hide the request itself: the guardian
		// still needs to see that one is open (and be able to withdraw it).
		s.Logger.Warn("offering change: build diff failed",
			slog.Int64("request_id", pending.ID),
			slog.String("error", diffErr.Error()),
		)
		return view, nil
	}
	view.Diff = diff
	return view, nil
}

// lastDecisionForStudent returns the newest approved/rejected request decided
// within the recency window. Withdrawals are skipped: the guardian withdrew it
// themselves and does not need to be told about it.
func (s *offeringChangeRequestService) lastDecisionForStudent(
	ctx context.Context,
	studentID int64,
) (*OfferingChangeDecision, error) {
	rows, err := s.ChangeRepo.ListByStudent(ctx, studentID)
	if err != nil {
		return nil, fmt.Errorf("offering change: list requests: %w", err)
	}
	cutoff := time.Now().AddDate(0, 0, -offeringDecisionRecencyDays)
	for _, row := range rows {
		if row == nil || row.ReviewedAt == nil {
			continue
		}
		if row.Status != enrollmentModels.OfferingChangeStatusApproved &&
			row.Status != enrollmentModels.OfferingChangeStatusRejected {
			continue
		}
		if row.ReviewedAt.Before(cutoff) {
			// Rows are newest-first, so everything below is older too.
			return nil, nil
		}
		decision := &OfferingChangeDecision{
			ID:            row.ID,
			Status:        row.Status,
			DecidedAt:     *row.ReviewedAt,
			EffectiveFrom: row.EffectiveFrom,
		}
		if row.DecisionReason != nil {
			decision.Reason = *row.DecisionReason
		}
		requested, reqErr := s.requestedItems(ctx, row)
		if reqErr != nil {
			// The outcome matters more than the recap: report the decision even
			// when the payload cannot be resolved to names.
			s.Logger.Warn("offering change: resolve requested offerings failed",
				slog.Int64("request_id", row.ID),
				slog.String("error", reqErr.Error()),
			)
		} else {
			decision.Requested = requested
		}
		return decision, nil
	}
	return nil, nil
}

// requestedItems reads the stored payload back into named offerings, sorted by
// catalog order so the recap matches the booking list above it. An offering
// deleted from the catalog since is still listed, by id, rather than dropped:
// the family asked for it, and hiding it would make the decision unreadable.
func (s *offeringChangeRequestService) requestedItems(
	ctx context.Context,
	row *enrollmentModels.OfferingChangeRequest,
) ([]OfferingChangeRequestedItem, error) {
	selections, err := selectionsFromPayload(row.Payload)
	if err != nil {
		return nil, err
	}
	if len(selections) == 0 {
		return []OfferingChangeRequestedItem{}, nil
	}
	ids := make([]int64, 0, len(selections))
	for _, selection := range selections {
		ids = append(ids, selection.OfferingID)
	}
	offerings, err := s.CareOfferingRepo.ListByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("list requested offerings: %w", err)
	}
	nameByID := make(map[int64]string, len(offerings))
	sortByID := make(map[int64]int, len(offerings))
	for _, offering := range offerings {
		if offering != nil {
			nameByID[offering.ID] = offering.Name
			sortByID[offering.ID] = offering.SortOrder
		}
	}
	items := make([]OfferingChangeRequestedItem, 0, len(selections))
	for _, selection := range selections {
		name := nameByID[selection.OfferingID]
		if name == "" {
			name = fmt.Sprintf("Angebot %d", selection.OfferingID)
		}
		items = append(items, OfferingChangeRequestedItem{
			OfferingID: selection.OfferingID,
			Name:       name,
			Days:       append([]string(nil), selection.SelectedDays...),
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		left, right := sortByID[items[i].OfferingID], sortByID[items[j].OfferingID]
		if left == right {
			return items[i].Name < items[j].Name
		}
		return left < right
	})
	return items, nil
}

func (s *offeringChangeRequestService) Create(
	ctx context.Context,
	input CreateOfferingChangeInput,
) (*enrollmentModels.OfferingChangeRequest, error) {
	if err := s.changesEnabled(ctx); err != nil {
		return nil, err
	}
	if input.StudentID <= 0 || input.AccountID <= 0 {
		return nil, fmt.Errorf("%w: student and account are required", ErrOfferingChangeInvalid)
	}
	note := strings.TrimSpace(input.Note)
	if utf8.RuneCountInString(note) > offeringChangeMaxNoteLen {
		return nil, fmt.Errorf("%w: note is too long", ErrOfferingChangeInvalid)
	}
	earliest, err := s.EarliestEffectiveFrom(ctx)
	if err != nil {
		return nil, err
	}
	if input.EffectiveFrom.Before(earliest) {
		return nil, fmt.Errorf("%w: effective date is earlier than the school's notice period", ErrOfferingChangeInvalid)
	}
	period, phase, err := s.carePeriodAt(ctx, input.StudentID, input.EffectiveFrom)
	if err != nil {
		return nil, err
	}
	if input.EffectiveFrom.Before(phase.ServiceStartDate) {
		return nil, fmt.Errorf("%w: effective date is before the care period starts", ErrOfferingChangeInvalid)
	}
	if input.EffectiveFrom.After(phase.ServiceEndDate) {
		return nil, fmt.Errorf("%w: effective date is after the care period ends", ErrOfferingChangeInvalid)
	}
	selections, err := s.validateSelections(ctx, phase, period.RequestChildID, input.EffectiveFrom, input.Selections)
	if err != nil {
		return nil, err
	}
	current, err := s.RequestChildOfferingRepo.ListByRequestChildIDAtDate(ctx, period.RequestChildID, input.EffectiveFrom)
	if err != nil {
		return nil, fmt.Errorf("offering change: list current offerings: %w", err)
	}
	if sameOfferingSelections(current, selections) {
		return nil, fmt.Errorf("%w: offerings are unchanged", ErrOfferingChangeInvalid)
	}
	existing, err := s.ChangeRepo.GetPendingForStudent(ctx, input.StudentID)
	if err != nil {
		return nil, fmt.Errorf("offering change: check pending: %w", err)
	}
	if existing != nil {
		return nil, enrollmentModels.ErrOfferingChangeAlreadyPending
	}
	row := &enrollmentModels.OfferingChangeRequest{
		StudentID:      input.StudentID,
		RequestChildID: period.RequestChildID,
		SubmittedBy:    input.AccountID,
		Payload:        payloadFromSelections(selections),
		EffectiveFrom:  input.EffectiveFrom,
		Status:         enrollmentModels.OfferingChangeStatusPending,
	}
	if note != "" {
		row.ParentNote = &note
	}
	if err := s.ChangeRepo.Create(ctx, row); err != nil {
		if isPendingOfferingChangeConflict(err) {
			return nil, enrollmentModels.ErrOfferingChangeAlreadyPending
		}
		return nil, fmt.Errorf("offering change: create: %w", err)
	}
	s.emitPillAfterCommit(ctx, row, parentmessaging.ChildEvent{
		EventType:      usersModels.ParentMessageEventRequestCreated,
		ActorKind:      usersModels.ParentMessageSenderGuardian,
		ActorAccountID: input.AccountID,
		Body:           offeringChangeCreatedBody,
		RequestType:    OfferingChangeRequestType,
		RequestStatus:  usersModels.ParentMessageRequestStatusOpen,
	})
	s.Logger.Info("offering change request created",
		slog.Int64("student_id", input.StudentID),
		slog.Int64("request_child_id", period.RequestChildID),
		slog.String("effective_from", input.EffectiveFrom.String()),
		slog.Int("offering_count", len(selections)),
	)
	return row, nil
}

func (s *offeringChangeRequestService) Withdraw(ctx context.Context, requestID, accountID, studentID int64) error {
	row, err := s.ChangeRepo.FindByIDForUpdate(ctx, requestID)
	if err != nil {
		return err
	}
	// Ownership first, so a foreign request's id cannot be probed for status.
	if row.StudentID != studentID || row.SubmittedBy != accountID {
		return ErrOfferingChangeForbidden
	}
	if row.IsTerminal() {
		return enrollmentModels.ErrOfferingChangeNotPending
	}
	if err := s.ChangeRepo.Decide(ctx, requestID, enrollmentModels.OfferingChangeStatusWithdrawn, nil, nil, false); err != nil {
		return err
	}
	s.emitPillAfterCommit(ctx, row, parentmessaging.ChildEvent{
		EventType:      usersModels.ParentMessageEventRequestStatus,
		ActorKind:      usersModels.ParentMessageSenderGuardian,
		ActorAccountID: accountID,
		Body:           offeringChangeWithdrawnBody,
		RequestType:    OfferingChangeRequestType,
		RequestStatus:  usersModels.ParentMessageRequestStatusWithdrawn,
	})
	return nil
}

func (s *offeringChangeRequestService) ListPending(ctx context.Context) ([]*OfferingChangeView, error) {
	rows, err := s.ChangeRepo.ListPendingForTenant(ctx)
	if err != nil {
		return nil, fmt.Errorf("offering change: list pending: %w", err)
	}
	studentIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		if row != nil {
			studentIDs = append(studentIDs, row.StudentID)
		}
	}
	students, err := s.StudentRepo.FindByIDs(ctx, studentIDs)
	if err != nil {
		return nil, fmt.Errorf("offering change: load students: %w", err)
	}
	writable := authorize.WritableStudentFilter(ctx, jwt.PermissionsFromCtx(ctx), s.UserContext)
	views := make([]*OfferingChangeView, 0, len(rows))
	for _, row := range rows {
		student := students[row.StudentID]
		if !writable(student) || student.IsAlumnus() {
			continue
		}
		view := &OfferingChangeView{Request: row, StudentName: s.studentName(ctx, row.StudentID)}
		if diff, diffErr := s.diffForRequest(ctx, row); diffErr == nil {
			view.Diff = diff
		} else {
			s.Logger.Warn("offering change: build diff failed",
				slog.Int64("request_id", row.ID),
				slog.String("error", diffErr.Error()),
			)
		}
		views = append(views, view)
	}
	return views, nil
}

func (s *offeringChangeRequestService) Decide(ctx context.Context, input DecideOfferingChangeInput) error {
	if input.RequestID <= 0 || input.ReviewedBy <= 0 {
		return fmt.Errorf("%w: request and reviewer are required", ErrOfferingChangeInvalid)
	}
	reason := strings.TrimSpace(input.Reason)
	if utf8.RuneCountInString(reason) > offeringChangeMaxNoteLen {
		return fmt.Errorf("%w: reason is too long", ErrOfferingChangeInvalid)
	}
	if !input.Approve && reason == "" {
		// A rejection the family cannot understand generates the phone call the
		// whole feature exists to avoid.
		return fmt.Errorf("%w: a rejection needs a reason", ErrOfferingChangeInvalid)
	}
	row, err := s.ChangeRepo.FindByIDForUpdate(ctx, input.RequestID)
	if err != nil {
		return err
	}
	if row.IsTerminal() {
		return enrollmentModels.ErrOfferingChangeNotPending
	}
	student, err := s.StudentRepo.FindByIDForUpdate(ctx, row.StudentID)
	if err != nil {
		return fmt.Errorf("offering change: load student for decision: %w", err)
	}
	if student.IsAlumnus() {
		return enrollmentModels.ErrOfferingChangeNotFound
	}
	if ok, _ := authorize.CanUpdateStudent(ctx, jwt.PermissionsFromCtx(ctx), student, s.UserContext); !ok {
		return ErrOfferingChangeForbidden
	}
	if !input.Approve {
		if err := s.ChangeRepo.Decide(
			ctx, row.ID, enrollmentModels.OfferingChangeStatusRejected, &reason, &input.ReviewedBy, false,
		); err != nil {
			return err
		}
		s.emitDecisionPill(ctx, row, input.ReviewedBy, offeringChangeRejectedBody,
			usersModels.ParentMessageRequestStatusRejected, reason)
		return nil
	}
	if err := s.applyApproved(ctx, row, input); err != nil {
		return err
	}
	if err := s.ChangeRepo.Decide(
		ctx, row.ID, enrollmentModels.OfferingChangeStatusApproved, optionalString(reason), &input.ReviewedBy, true,
	); err != nil {
		return err
	}
	s.emitDecisionPill(ctx, row, input.ReviewedBy, offeringChangeApprovedBody,
		usersModels.ParentMessageRequestStatusDone, reason)
	s.Logger.Info("offering change request approved",
		slog.Int64("request_id", row.ID),
		slog.Int64("student_id", row.StudentID),
		slog.String("effective_from", row.EffectiveFrom.String()),
	)
	return nil
}

// applyApproved re-validates the request against today's catalog and capacity
// and then performs the dated switch. Re-validation is the point: the request
// may be weeks old, and an offering can have been deactivated or filled up in
// the meantime. A failure here leaves the row pending, so the office sees an
// actionable error instead of a request marked done that never applied.
func (s *offeringChangeRequestService) applyApproved(
	ctx context.Context,
	row *enrollmentModels.OfferingChangeRequest,
	input DecideOfferingChangeInput,
) error {
	if s.Settings == nil {
		return ErrCareOfferingsDisabled
	}
	careOfferingsEnabled, err := s.Settings.ResolveBool(ctx, configModel.KeyEnrollmentCareOfferingsEnabled)
	if err != nil {
		return fmt.Errorf("offering change: resolve care offerings setting: %w", err)
	}
	if !careOfferingsEnabled {
		return ErrCareOfferingsDisabled
	}
	child, err := s.RequestChildRepo.FindByID(ctx, row.RequestChildID)
	if err != nil || child == nil {
		return fmt.Errorf("offering change: load request child: %w", err)
	}
	request, err := s.RequestRepo.FindByID(ctx, child.RequestID)
	if err != nil || request == nil {
		return fmt.Errorf("offering change: load request: %w", err)
	}
	phase, err := s.PhaseRepo.FindByID(ctx, request.PhaseID)
	if err != nil || phase == nil {
		return fmt.Errorf("offering change: load phase: %w", err)
	}
	selections, err := selectionsFromPayload(row.Payload)
	if err != nil {
		return err
	}
	effectiveFrom := row.EffectiveFrom
	// A request whose date has passed while it waited applies as soon as
	// possible instead of being refused: the family asked for a change, the
	// office agreed, and a date in the past would be rejected by the adjustment
	// validator.
	if today := timezone.TodayDate(); effectiveFrom.Before(today) {
		effectiveFrom = today
	}
	if effectiveFrom.After(phase.ServiceEndDate) {
		return fmt.Errorf("%w: the care period ended before this request was decided", ErrOfferingChangeInvalid)
	}
	if _, err := s.validateSelections(ctx, phase, row.RequestChildID, effectiveFrom, selections); err != nil {
		return err
	}
	if err := s.assertCapacityAvailable(ctx, phase, row.RequestChildID, effectiveFrom, selections); err != nil {
		return err
	}
	// Keep the actual date in memory for the adjustment audit. Persist it only
	// after the switch succeeds, so a client error leaves the pending row intact.
	row.EffectiveFrom = effectiveFrom
	adjustment := UpdateChildOfferingsInput{
		RequestID:      request.ID,
		ChildID:        row.RequestChildID,
		Offerings:      adjustmentSelections(selections),
		Reason:         offeringChangeAdjustmentReason(row, input.Reason),
		ActorAccountID: input.ReviewedBy,
		ActorRole:      cmpOr(strings.TrimSpace(input.ActorRole), "admin"),
		EffectiveFrom:  &effectiveFrom,
	}
	if _, err := s.Applier.applyApprovedChangeRequestOfferings(ctx, adjustment); err != nil {
		return fmt.Errorf("offering change: apply: %w", err)
	}
	if err := s.ChangeRepo.UpdateEffectiveFrom(ctx, row.ID, effectiveFrom); err != nil {
		return fmt.Errorf("offering change: update applied effective date: %w", err)
	}
	return nil
}

// assertCapacityAvailable refuses an approval that would overbook an offering
// the child does not already hold. Offerings the child keeps are exempt: their
// slot is already counted, and re-checking them would reject a request that
// only changes days.
func (s *offeringChangeRequestService) assertCapacityAvailable(
	ctx context.Context,
	phase *enrollmentModels.Phase,
	requestChildID int64,
	effectiveFrom timezone.Date,
	selections []OfferingChangeSelection,
) error {
	current, err := s.RequestChildOfferingRepo.ListByRequestChildIDAtDate(ctx, requestChildID, effectiveFrom)
	if err != nil {
		return fmt.Errorf("offering change: list current offerings: %w", err)
	}
	held := heldOfferingIDs(current)
	allOfferingIDs, err := s.materializedOfferingIDs(ctx, phase, requestChildID, effectiveFrom, selections)
	if err != nil {
		return err
	}
	needed := make([]int64, 0, len(allOfferingIDs))
	for _, offeringID := range allOfferingIDs {
		if held[offeringID] {
			continue
		}
		needed = append(needed, offeringID)
	}
	sort.Slice(needed, func(i, j int) bool { return needed[i] < needed[j] })
	locked, err := s.CareOfferingRepo.ListByIDsForUpdate(ctx, needed)
	if err != nil {
		return fmt.Errorf("offering change: lock offering capacity: %w", err)
	}
	lockedByID := offeringsByID(locked)
	for _, offeringID := range needed {
		offering := lockedByID[offeringID]
		if offering == nil {
			return fmt.Errorf("%w: care offering %d is unavailable", ErrOfferingChangeInvalid, offeringID)
		}
		if offering.Capacity == nil {
			continue
		}
		taken, countErr := s.RequestChildOfferingRepo.CountActiveByCareOfferingOnDate(ctx, offering.ID, effectiveFrom)
		if countErr != nil {
			return fmt.Errorf("offering change: count offering occupancy: %w", countErr)
		}
		if taken >= *offering.Capacity {
			return fmt.Errorf("%w: %s", ErrOfferingChangeCapacityFull, offering.Name)
		}
	}
	return nil
}

// materializedOfferingIDs includes required and auto-added offerings, which
// are not present in a guardian's raw selection but consume capacity too.
func (s *offeringChangeRequestService) materializedOfferingIDs(
	ctx context.Context,
	phase *enrollmentModels.Phase,
	requestChildID int64,
	effectiveFrom timezone.Date,
	selections []OfferingChangeSelection,
) ([]int64, error) {
	active, err := s.CareOfferingRepo.ListActiveByPhase(ctx, phase.ID)
	if err != nil {
		return nil, fmt.Errorf("offering change: list active offerings: %w", err)
	}
	allowed := offeringsByID(active)
	if err := s.addHeldOfferingsAtDate(ctx, requestChildID, effectiveFrom, allowed); err != nil {
		return nil, err
	}
	_, submit, err := normalizeOfferingSelections(selections, allowed)
	if err != nil {
		return nil, err
	}
	child, err := s.RequestChildRepo.FindByID(ctx, requestChildID)
	if err != nil || child == nil {
		return nil, fmt.Errorf("offering change: load request child: %w", err)
	}
	submit.TargetGradeLevel = child.TargetGradeLevel
	materialized, err := materializeAndValidateChildrenOfferingSelections(
		[]SubmitChild{submit}, allowed, phase.CareOfferingSelectionMode,
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrOfferingChangeInvalid, err)
	}
	ids := make([]int64, 0, len(materialized[0]))
	for _, selection := range materialized[0] {
		ids = append(ids, selection.OfferingID)
	}
	return ids, nil
}

func heldOfferingIDs(links []*enrollmentModels.RequestChildOffering) map[int64]bool {
	held := make(map[int64]bool, len(links))
	for _, link := range links {
		if link != nil {
			held[link.CareOfferingID] = true
		}
	}
	return held
}

func sameOfferingSelections(current []*enrollmentModels.RequestChildOffering, selections []OfferingChangeSelection) bool {
	if len(current) != len(selections) {
		return false
	}
	byID := make(map[int64]*enrollmentModels.RequestChildOffering, len(current))
	for _, link := range current {
		if link == nil {
			return false
		}
		byID[link.CareOfferingID] = link
	}
	for _, selection := range selections {
		link := byID[selection.OfferingID]
		if link == nil || !slices.Equal(canonicalDays(link.SelectedDays), canonicalDays(selection.SelectedDays)) {
			return false
		}
	}
	return true
}

// validateSelections checks the desired selection against the phase catalog and
// its selection rules using the very same materializer the enrollment form and
// the admin adjustment use, so a request cannot be accepted that an approval
// would then refuse.
func (s *offeringChangeRequestService) validateSelections(
	ctx context.Context,
	phase *enrollmentModels.Phase,
	requestChildID int64,
	effectiveFrom timezone.Date,
	selections []OfferingChangeSelection,
) ([]OfferingChangeSelection, error) {
	active, err := s.CareOfferingRepo.ListActiveByPhase(ctx, phase.ID)
	if err != nil {
		return nil, fmt.Errorf("offering change: list active offerings: %w", err)
	}
	allowed := offeringsByID(active)
	// Offerings the child already holds stay selectable even when the admin
	// deactivated them in the catalog: keeping the status quo must never be
	// blocked by a catalog edit.
	if err := s.addHeldOfferingsAtDate(ctx, requestChildID, effectiveFrom, allowed); err != nil {
		return nil, err
	}
	normalized, submit, err := normalizeOfferingSelections(selections, allowed)
	if err != nil {
		return nil, err
	}
	child, err := s.RequestChildRepo.FindByID(ctx, requestChildID)
	if err != nil || child == nil {
		return nil, fmt.Errorf("offering change: load request child: %w", err)
	}
	submit.TargetGradeLevel = child.TargetGradeLevel
	_, err = materializeAndValidateChildrenOfferingSelections(
		[]SubmitChild{submit}, allowed, phase.CareOfferingSelectionMode,
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrOfferingChangeInvalid, err)
	}
	// Keep the guardian's explicit selection in the request payload. The
	// materializer adds dependent offerings for validation and capacity, but
	// storing those additions would turn them into manual bookings on approval.
	sort.SliceStable(normalized, func(i, j int) bool {
		return normalized[i].OfferingID < normalized[j].OfferingID
	})
	return normalized, nil
}

func offeringsByID(offerings []*enrollmentModels.CareOffering) map[int64]*enrollmentModels.CareOffering {
	byID := make(map[int64]*enrollmentModels.CareOffering, len(offerings))
	for _, offering := range offerings {
		if offering != nil {
			byID[offering.ID] = offering
		}
	}
	return byID
}

func (s *offeringChangeRequestService) addHeldOfferingsAtDate(
	ctx context.Context,
	requestChildID int64,
	onDate timezone.Date,
	allowed map[int64]*enrollmentModels.CareOffering,
) error {
	if requestChildID <= 0 {
		return nil
	}
	current, err := s.RequestChildOfferingRepo.ListByRequestChildIDAtDate(ctx, requestChildID, onDate)
	if err != nil {
		return fmt.Errorf("offering change: list current offerings: %w", err)
	}
	heldIDs := make([]int64, 0, len(current))
	for _, link := range current {
		if link != nil && allowed[link.CareOfferingID] == nil {
			heldIDs = append(heldIDs, link.CareOfferingID)
		}
	}
	if len(heldIDs) == 0 {
		return nil
	}
	held, err := s.CareOfferingRepo.ListByIDs(ctx, heldIDs)
	if err != nil {
		return fmt.Errorf("offering change: list held offerings: %w", err)
	}
	for id, offering := range offeringsByID(held) {
		allowed[id] = offering
	}
	return nil
}

func normalizeOfferingSelections(
	selections []OfferingChangeSelection,
	allowed map[int64]*enrollmentModels.CareOffering,
) ([]OfferingChangeSelection, SubmitChild, error) {
	normalized := make([]OfferingChangeSelection, 0, len(selections))
	submit := SubmitChild{
		OfferingIDs:  make([]int64, 0, len(selections)),
		OfferingDays: make([]SubmitOfferingDays, 0, len(selections)),
	}
	seen := make(map[int64]bool, len(selections))
	for _, selection := range selections {
		if selection.OfferingID <= 0 {
			return nil, SubmitChild{}, fmt.Errorf("%w: offering id is required", ErrOfferingChangeInvalid)
		}
		if seen[selection.OfferingID] {
			continue
		}
		if allowed[selection.OfferingID] == nil {
			return nil, SubmitChild{}, fmt.Errorf("%w: care offering %d cannot be booked for this child", ErrOfferingChangeInvalid, selection.OfferingID)
		}
		seen[selection.OfferingID] = true
		for _, day := range selection.SelectedDays {
			normalizedDay := strings.ToLower(strings.TrimSpace(day))
			if normalizedDay == "" {
				continue
			}
			if _, ok := enrollmentModels.CanonicalDayToISOWeekday(normalizedDay); !ok {
				return nil, SubmitChild{}, fmt.Errorf("%w: unknown selected day %q", ErrOfferingChangeInvalid, day)
			}
		}
		days := canonicalDays(selection.SelectedDays)
		normalized = append(normalized, OfferingChangeSelection{
			OfferingID:   selection.OfferingID,
			SelectedDays: days,
		})
		submit.OfferingIDs = append(submit.OfferingIDs, selection.OfferingID)
		if len(days) > 0 {
			submit.OfferingDays = append(submit.OfferingDays, SubmitOfferingDays{
				OfferingID:   selection.OfferingID,
				SelectedDays: days,
			})
		}
	}
	return normalized, submit, nil
}

// diffForRequest renders "current → requested" per offering, including the ones
// that are dropped, so a reviewer never has to open two screens.
func (s *offeringChangeRequestService) diffForRequest(
	ctx context.Context,
	row *enrollmentModels.OfferingChangeRequest,
) ([]OfferingChangeDiffEntry, error) {
	requested, err := selectionsFromPayload(row.Payload)
	if err != nil {
		return nil, err
	}
	current, err := s.RequestChildOfferingRepo.ListByRequestChildIDAtDate(ctx, row.RequestChildID, row.EffectiveFrom)
	if err != nil {
		return nil, fmt.Errorf("offering change: list current offerings: %w", err)
	}
	ids, currentByID, requestedByID := offeringChangeSides(current, requested)
	offerings, err := s.CareOfferingRepo.ListByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("offering change: list offerings for diff: %w", err)
	}
	nameByID := offeringNamesByID(offerings)
	entries := make([]OfferingChangeDiffEntry, 0, len(ids))
	for _, id := range ids {
		entry, changed := offeringDiffEntry(id, nameByID[id], currentByID[id], requestedByID)
		if !changed {
			continue
		}
		entries = append(entries, entry)
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].Label < entries[j].Label })
	return entries, nil
}

func offeringChangeSides(
	current []*enrollmentModels.RequestChildOffering,
	requested []OfferingChangeSelection,
) (
	[]int64,
	map[int64]*enrollmentModels.RequestChildOffering,
	map[int64]OfferingChangeSelection,
) {
	ids := make([]int64, 0, len(current)+len(requested))
	seen := make(map[int64]bool, len(current)+len(requested))
	addID := func(id int64) {
		if id > 0 && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	currentByID := make(map[int64]*enrollmentModels.RequestChildOffering, len(current))
	for _, link := range current {
		if link != nil {
			currentByID[link.CareOfferingID] = link
			addID(link.CareOfferingID)
		}
	}
	requestedByID := make(map[int64]OfferingChangeSelection, len(requested))
	for _, selection := range requested {
		requestedByID[selection.OfferingID] = selection
		addID(selection.OfferingID)
	}
	return ids, currentByID, requestedByID
}

func offeringNamesByID(offerings []*enrollmentModels.CareOffering) map[int64]string {
	names := make(map[int64]string, len(offerings))
	for _, offering := range offerings {
		if offering != nil {
			names[offering.ID] = offering.Name
		}
	}
	return names
}

func offeringDiffEntry(
	id int64,
	name string,
	current *enrollmentModels.RequestChildOffering,
	requestedByID map[int64]OfferingChangeSelection,
) (OfferingChangeDiffEntry, bool) {
	if name == "" {
		name = fmt.Sprintf("Angebot %d", id)
	}
	oldLabel := "nicht gebucht"
	if current != nil {
		oldLabel = germanDayLabel(current.SelectedDays)
	}
	newLabel := "abgemeldet"
	if requested, wanted := requestedByID[id]; wanted {
		newLabel = germanDayLabel(requested.SelectedDays)
	}
	return OfferingChangeDiffEntry{Label: name, Old: oldLabel, New: newLabel}, oldLabel != newLabel
}

func (s *offeringChangeRequestService) studentName(ctx context.Context, studentID int64) string {
	if s.StudentRepo == nil || s.PersonRepo == nil {
		return ""
	}
	student, err := s.StudentRepo.FindByID(ctx, studentID)
	if err != nil || student == nil {
		return ""
	}
	person, err := s.PersonRepo.FindByID(ctx, student.PersonID)
	if err != nil || person == nil {
		return ""
	}
	return strings.TrimSpace(person.GetFullName())
}

func (s *offeringChangeRequestService) emitDecisionPill(
	ctx context.Context,
	row *enrollmentModels.OfferingChangeRequest,
	reviewedBy int64,
	body, status, reason string,
) {
	s.emitPillAfterCommit(ctx, row, parentmessaging.ChildEvent{
		EventType:      usersModels.ParentMessageEventRequestStatus,
		ActorKind:      usersModels.ParentMessageSenderStaff,
		ActorAccountID: reviewedBy,
		Body:           body,
		RequestType:    OfferingChangeRequestType,
		RequestStatus:  status,
		DecisionReason: reason,
	})
}

// emitPillAfterCommit posts the notification pill into the child's parent-OGS
// thread once the surrounding transaction committed. Best-effort: a lost pill
// costs a notification, never the decision.
func (s *offeringChangeRequestService) emitPillAfterCommit(
	ctx context.Context,
	row *enrollmentModels.OfferingChangeRequest,
	ev parentmessaging.ChildEvent,
) {
	if s.Emitter == nil {
		return
	}
	tenantID := row.TenantID
	if tenantID <= 0 {
		tenantID = tenant.FromContext(ctx)
	}
	refID := row.ID
	ev.RefTable = "enrollment.offering_change_requests"
	ev.RefID = &refID
	studentID := row.StudentID
	guardianAccountID := row.SubmittedBy
	tenant.RegisterAfterCommit(ctx, func() {
		s.Emitter.EmitChildEvent(tenantID, studentID, guardianAccountID, ev)
	})
}

func offeringChangeAdjustmentReason(row *enrollmentModels.OfferingChangeRequest, staffReason string) string {
	reason := fmt.Sprintf("Elternanfrage #%d freigegeben (gültig ab %s)",
		row.ID, row.EffectiveFrom.Format("02.01.2006"))
	if trimmed := strings.TrimSpace(staffReason); trimmed != "" {
		reason += ": " + trimmed
	}
	return reason
}

func adjustmentSelections(selections []OfferingChangeSelection) []OfferingAdjustmentSelection {
	out := make([]OfferingAdjustmentSelection, 0, len(selections))
	for _, selection := range selections {
		out = append(out, OfferingAdjustmentSelection{
			OfferingID:   selection.OfferingID,
			SelectedDays: append([]string(nil), selection.SelectedDays...),
		})
	}
	return out
}

func payloadFromSelections(selections []OfferingChangeSelection) map[string]any {
	rows := make([]any, 0, len(selections))
	for _, selection := range selections {
		row := map[string]any{"offering_id": selection.OfferingID}
		if len(selection.SelectedDays) > 0 {
			days := make([]any, 0, len(selection.SelectedDays))
			for _, day := range selection.SelectedDays {
				days = append(days, day)
			}
			row["selected_days"] = days
		}
		rows = append(rows, row)
	}
	return map[string]any{"offerings": rows}
}

// selectionsFromPayload parses the stored payload. It is strict: a payload we
// cannot read must not be silently applied as "no offerings", which would
// unbook the child entirely.
func selectionsFromPayload(payload map[string]any) ([]OfferingChangeSelection, error) {
	raw, ok := payload["offerings"]
	if !ok {
		return nil, fmt.Errorf("%w: payload has no offerings", ErrOfferingChangeInvalid)
	}
	list, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("%w: payload offerings must be a list", ErrOfferingChangeInvalid)
	}
	selections := make([]OfferingChangeSelection, 0, len(list))
	for _, entry := range list {
		selection, err := selectionFromPayloadEntry(entry)
		if err != nil {
			return nil, err
		}
		selections = append(selections, selection)
	}
	return selections, nil
}

func selectionFromPayloadEntry(entry any) (OfferingChangeSelection, error) {
	row, ok := entry.(map[string]any)
	if !ok {
		return OfferingChangeSelection{}, fmt.Errorf("%w: payload offering entry is not an object", ErrOfferingChangeInvalid)
	}
	id, err := payloadInt64(row["offering_id"])
	if err != nil {
		return OfferingChangeSelection{}, err
	}
	selection := OfferingChangeSelection{OfferingID: id}
	daysRaw, hasDays := row["selected_days"]
	if !hasDays || daysRaw == nil {
		return selection, nil
	}
	days, ok := daysRaw.([]any)
	if !ok {
		return OfferingChangeSelection{}, fmt.Errorf("%w: payload selected_days must be a list", ErrOfferingChangeInvalid)
	}
	values, err := payloadStrings(days)
	if err != nil {
		return OfferingChangeSelection{}, err
	}
	selection.SelectedDays = canonicalDays(values)
	return selection, nil
}

func payloadStrings(values []any) ([]string, error) {
	strings := make([]string, 0, len(values))
	for _, value := range values {
		text, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("%w: payload selected_days must contain strings", ErrOfferingChangeInvalid)
		}
		strings = append(strings, text)
	}
	return strings, nil
}

// payloadInt64 accepts the shapes jsonb round-trips through Go: float64 from
// encoding/json, plus int64/string for hand-written payloads.
func payloadInt64(value any) (int64, error) {
	switch typed := value.(type) {
	case float64:
		return int64(typed), nil
	case int64:
		return typed, nil
	case int:
		return int64(typed), nil
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("%w: offering id %q is not a number", ErrOfferingChangeInvalid, typed)
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("%w: offering id is missing", ErrOfferingChangeInvalid)
	}
}

// canonicalDays lowercases, de-duplicates and week-orders day abbreviations.
func canonicalDays(days []string) []string {
	out := make([]string, 0, len(days))
	seen := make(map[string]bool, len(days))
	for _, day := range days {
		normalized := strings.ToLower(strings.TrimSpace(day))
		if normalized == "" || seen[normalized] {
			continue
		}
		if _, ok := enrollmentModels.CanonicalDayToISOWeekday(normalized); !ok {
			continue
		}
		seen[normalized] = true
		out = append(out, normalized)
	}
	sort.SliceStable(out, func(i, j int) bool {
		left, _ := enrollmentModels.CanonicalDayToISOWeekday(out[i])
		right, _ := enrollmentModels.CanonicalDayToISOWeekday(out[j])
		return left < right
	})
	return out
}

// germanDayLabel renders a day set for the diff ("Mo, Di" / "alle Tage").
func germanDayLabel(days []string) string {
	canonical := canonicalDays(days)
	if len(canonical) == 0 {
		return "alle Betreuungstage"
	}
	labels := map[string]string{
		"mon": "Mo", "tue": "Di", "wed": "Mi", "thu": "Do",
		"fri": "Fr", "sat": "Sa", "sun": "So",
	}
	parts := make([]string, 0, len(canonical))
	for _, day := range canonical {
		parts = append(parts, labels[day])
	}
	return strings.Join(parts, ", ")
}

func optionalString(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}

func cmpOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

// isPendingOfferingChangeConflict recognises the partial unique index violation
// from a concurrent second submit.
func isPendingOfferingChangeConflict(err error) bool {
	return err != nil && strings.Contains(err.Error(), "uq_offering_change_requests_pending")
}
