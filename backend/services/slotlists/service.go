package slotlists

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	educationModel "github.com/moto-nrw/project-phoenix/models/education"
	facilitiesModel "github.com/moto-nrw/project-phoenix/models/facilities"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	userModel "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/listexport"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/services/usercontext"
)

// confidentialityNote matches the wording of the other printed exports.
const confidentialityNote = "Vertraulich, nur für berechtigte Personen. Nach Gebrauch sicher vernichten."

const timeLayout = "15:04"

// ErrPickupCohortPastDate is returned when a Ganztag pickup list is requested
// for a date before today. Recurring pickup schedules
// (schedule.student_pickup_schedules) carry no validity interval and no
// historical snapshot, so a past pickup plan cannot be faithfully
// reconstructed: the *current* schedule would be projected onto the past date
// and silently move children between cohorts or show a time that was never
// planned then (#1565 review). Slot-based lists remain available for past dates
// — only pickup cohorts are refused. The handler maps this to HTTP 400.
var ErrPickupCohortPastDate = errors.New("Ganztag-Listen sind nur für heute und künftige Tage verfügbar")

// ErrReconciliationFutureDate is returned when an Abgleich (reconciliation) list
// is requested for a strictly-future date. Reconciliation compares the plan
// against documented presence, and a future day has no presence evidence yet, so
// every planned child would fall through to "Fehlt" — a printable list claiming
// the whole group is missing before the day has even started (#1565 review). Plan
// and Ist lists stay available for future dates; only the merge is refused. The
// handler maps this to HTTP 400.
var ErrReconciliationFutureDate = errors.New("Ein Abgleich ist nur für heute und vergangene Tage möglich, nicht für künftige Tage") //nolint:staticcheck // ST1005: user-facing German message

// ErrListDrifted is returned by RenderList when the freshly rebuilt list no
// longer matches the content signature the client verified in its preview
// (Params.ExpectedSignature). Live attendance, roster or plan data changed
// between the client's preview verification and this export render, so honoring
// the export would hand out a file the user never reviewed. The handler maps
// this to HTTP 409 so the client can refresh the preview and ask the user to
// re-check before exporting again (#1565 review pass 2).
var ErrListDrifted = errors.New("Die Liste hat sich seit der Vorschau geändert. Bitte erneut prüfen und exportieren.") //nolint:staticcheck // ST1005: user-facing German message

// Service builds slot lists for preview (JSON) and export (PDF/XLSX).
type Service interface {
	BuildList(ctx context.Context, params Params) (*Result, error)
	ListOptions(ctx context.Context, date timezone.Date) (*OptionsResult, error)
	RenderList(ctx context.Context, params Params, format listexport.Format) (listexport.File, error)
}

type instanceReader interface {
	FindByTenantAndDate(ctx context.Context, date timezone.Date) ([]*scheduleModel.ActivityInstance, error)
}

type instanceStudentReader interface {
	FindByInstanceID(ctx context.Context, instanceID int64) ([]*scheduleModel.InstanceStudent, error)
	FindByInstanceIDs(ctx context.Context, instanceIDs []int64) ([]*scheduleModel.InstanceStudent, error)
}

type visitReader interface {
	FindByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64) ([]*activeModel.Visit, error)
}

type attendanceReader interface {
	FindForDate(ctx context.Context, date timezone.Date) ([]*activeModel.Attendance, error)
}

// statusDayReader loads the broad day statuses (sick / excused / class trip)
// that sign a child off for a whole date (#1565 review: a registered absence
// must show as "Abgemeldet" on pickup lists, not as unexplained "Fehlt").
type statusDayReader interface {
	// FindSignedOffByStudentIDsAndDate returns active sign-offs plus the
	// scheduler's end-of-day archived rows (source = "end_of_day"), so a child
	// signed off sick/excused all day still renders "Abgemeldet" after the
	// configured status-clear time rather than an unexplained "Fehlt" (#1565
	// review pass 1).
	FindSignedOffByStudentIDsAndDate(ctx context.Context, studentIDs []int64, date timezone.Date) ([]*activeModel.StudentStatusDay, error)
}

// careDayReader resolves the care-plan verdict per child on a date. A
// "cancelled" verdict ("Kommt heute nicht" — a timeless arrival/pickup
// exception) is a registered absence that pickup cohorts must show as
// "Abgemeldet", never as unexplained "Fehlt" (#1565 review).
type careDayReader interface {
	ResolveForDate(ctx context.Context, studentIDs []int64, date timezone.Date) (map[int64]scheduleSvc.CareDayStatus, error)
}

// regularPickupReader returns the recurring weekly pickup rows (no exceptions
// applied). Used to place a cancelled child into a cohort by their normal
// pickup bucket when a same-day exception cleared their effective time.
type regularPickupReader interface {
	FindByStudentIDsAndWeekday(ctx context.Context, studentIDs []int64, weekday int) ([]*scheduleModel.StudentPickupSchedule, error)
}

type studentReader interface {
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*userModel.Student, error)
	List(ctx context.Context, filters map[string]interface{}) ([]*userModel.Student, error)
}

type personReader interface {
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*userModel.Person, error)
}

type educationGroupReader interface {
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*educationModel.Group, error)
}

type roomReader interface {
	FindByID(ctx context.Context, id any) (*facilitiesModel.Room, error)
}

type pickupTimeReader interface {
	GetBulkEffectivePickupTimesForDate(ctx context.Context, studentIDs []int64, date timezone.Date) (map[int64]*scheduleSvc.EffectivePickupTime, error)
}

type arrivalTimeReader interface {
	GetBulkEffectiveArrivalTimesForDate(ctx context.Context, studentIDs []int64, date timezone.Date) (map[int64]*scheduleSvc.EffectiveArrivalTime, error)
}

type settingsReader interface {
	HasTenantOverride(ctx context.Context, key string) (bool, error)
	ResolveString(ctx context.Context, key string) (string, error)
	// LockSlotListCutoffPairShared acquires the SHARED advisory lock guarding the
	// Ganztag pickup-cutoff pair so pickupBuckets observes both cutoffs from one
	// consistent snapshot, never one cutoff from before and the sibling from after
	// a concurrent two-write change (#1565 review pass 12). Shared mode lets
	// concurrent readers (/options, pickup preview, exports) run in parallel while
	// still excluding a cutoff writer (#1565 review).
	LockSlotListCutoffPairShared(ctx context.Context) error
}

// Dependencies wires the narrow read interfaces the service needs.
type Dependencies struct {
	InstanceRepo        instanceReader
	InstanceStudentRepo instanceStudentReader
	VisitRepo           visitReader
	AttendanceRepo      attendanceReader
	StatusDayRepo       statusDayReader
	CareDayService      careDayReader
	PickupScheduleRepo  regularPickupReader
	StudentRepo         studentReader
	PersonRepo          personReader
	EducationGroupRepo  educationGroupReader
	RoomRepo            roomReader
	PickupService       pickupTimeReader
	ArrivalService      arrivalTimeReader
	ListExport          *listexport.RendererService
	Settings            settingsReader
	UserContext         authorize.StudentReadUserContext
	Logger              *slog.Logger
	// Now overrides the service clock. Leave nil in production (defaults to
	// time.Now); tests inject a fixed instant for a deterministic weekday.
	Now func() time.Time
}

type service struct {
	instanceRepo        instanceReader
	instanceStudentRepo instanceStudentReader
	visitRepo           visitReader
	attendanceRepo      attendanceReader
	statusDayRepo       statusDayReader
	careDayService      careDayReader
	pickupScheduleRepo  regularPickupReader
	studentRepo         studentReader
	personRepo          personReader
	educationGroupRepo  educationGroupReader
	roomRepo            roomReader
	pickupService       pickupTimeReader
	arrivalService      arrivalTimeReader
	listExport          *listexport.RendererService
	settings            settingsReader
	userContext         authorize.StudentReadUserContext
	logger              *slog.Logger
	// now is the clock the service reads "today" and "has this slot started
	// yet" from. Production leaves it nil (→ time.Now); tests inject a fixed
	// instant so the pickup suite is not at the mercy of the weekday CI runs on.
	now func() time.Time
}

// NewService creates a slot list service. Settings is a required dependency:
// the Ganztag pickup-cohort cutoffs are tenant configuration resolved from the
// settings registry, so a service wired without it would silently serve
// authoritative cohorts using hardcoded defaults unrelated to the registered
// values (#1565 review pass 1). A nil Settings is a wiring bug, so fail fast at
// construction rather than degrade at request time.
func NewService(deps Dependencies) Service {
	if deps.Settings == nil {
		panic("slotlists.NewService: Settings dependency is required")
	}
	now := deps.Now
	if now == nil {
		now = time.Now
	}
	return &service{
		instanceRepo:        deps.InstanceRepo,
		instanceStudentRepo: deps.InstanceStudentRepo,
		visitRepo:           deps.VisitRepo,
		attendanceRepo:      deps.AttendanceRepo,
		statusDayRepo:       deps.StatusDayRepo,
		careDayService:      deps.CareDayService,
		pickupScheduleRepo:  deps.PickupScheduleRepo,
		studentRepo:         deps.StudentRepo,
		personRepo:          deps.PersonRepo,
		educationGroupRepo:  deps.EducationGroupRepo,
		roomRepo:            deps.RoomRepo,
		pickupService:       deps.PickupService,
		arrivalService:      deps.ArrivalService,
		listExport:          deps.ListExport,
		settings:            deps.Settings,
		userContext:         deps.UserContext,
		logger:              deps.Logger,
		now:                 now,
	}
}

// currentTime returns the service's notion of "now" (real clock in production,
// an injected fixed instant in tests).
func (s *service) currentTime() time.Time { return s.now() }

// todayDate returns the current calendar day in Berlin, derived from the
// service clock so the date guards move with an injected test clock.
func (s *service) todayDate() timezone.Date { return timezone.DateFromTime(s.now()) }

type pickupBucketConfig struct {
	ShortCutoff string
	LongCutoff  string
}

func (s *service) pickupBuckets(ctx context.Context) (pickupBucketConfig, error) {
	// s.settings is guaranteed non-nil (required by NewService). Hold the SHARED
	// variant of the advisory lock the settings writer takes exclusively, before
	// reading either cutoff, so the two ResolveString calls below observe one
	// consistent pair. Without it they run as separate statements under READ
	// COMMITTED, and an administrator lowering BOTH boundaries through two
	// individually valid writes (e.g. 14:30/16:00 → 13:00/14:00) could commit
	// between them and hand this reader the old short cutoff with the new long
	// cutoff — an inverted pair (14:30/14:00) the ordering check below would 500 on,
	// even though nothing invalid was ever persisted (#1565 review pass 12).
	// Shared mode means concurrent readers (/options, pickup preview, exports) do
	// NOT block one another — only a concurrent cutoff writer is excluded — so a
	// slow export no longer serializes every other request in the tenant behind
	// this lock (#1565 review).
	if err := s.settings.LockSlotListCutoffPairShared(ctx); err != nil {
		return pickupBucketConfig{}, err
	}
	short, err := s.settings.ResolveString(ctx, configModel.KeySlotListShortDayCutoff)
	if err != nil {
		return pickupBucketConfig{}, fmt.Errorf("resolve short-day pickup cutoff: %w", err)
	}
	shortCutoff, err := requirePickupCutoff(short, "short-day")
	if err != nil {
		return pickupBucketConfig{}, err
	}

	long, err := s.settings.ResolveString(ctx, configModel.KeySlotListLongDayCutoff)
	if err != nil {
		return pickupBucketConfig{}, fmt.Errorf("resolve long-day pickup cutoff: %w", err)
	}
	longCutoff, err := requirePickupCutoff(long, "long-day")
	if err != nil {
		return pickupBucketConfig{}, err
	}

	if longCutoff <= shortCutoff {
		return pickupBucketConfig{}, fmt.Errorf("long-day pickup cutoff %s must be after short-day pickup cutoff %s", longCutoff, shortCutoff)
	}
	return pickupBucketConfig{ShortCutoff: shortCutoff, LongCutoff: longCutoff}, nil
}

// requirePickupCutoff normalizes a resolved Ganztag pickup cutoff and rejects an
// empty or malformed value. The settings registry always yields a non-empty
// default for these registered keys, so an empty resolved cutoff means the
// tenant's stored configuration is corrupt — a migration or manual repair blanked
// it. Fail fast rather than silently substitute a hardcoded default and build an
// authoritative pickup cohort from values unrelated to the tenant's stored
// configuration (#1565 review pass 12).
func requirePickupCutoff(raw, label string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", fmt.Errorf("%s pickup cutoff is not configured", label)
	}
	normalized, err := normalizeHHMM(raw)
	if err != nil {
		return "", fmt.Errorf("invalid %s pickup cutoff %q", label, raw)
	}
	return normalized, nil
}

type studentReadAccess struct {
	unrestricted bool
	groupIDs     map[int64]struct{}
}

func (a studentReadAccess) canReadGroup(groupID *int64) bool {
	if a.unrestricted {
		return true
	}
	if groupID == nil || a.groupIDs == nil {
		return false
	}
	_, ok := a.groupIDs[*groupID]
	return ok
}

// resolveStudentReadAccess derives which children the caller may see by name.
// Only the two legitimate restrictive conditions collapse to an empty access
// set (an account without a staff profile, a supervisor without groups);
// operational failures (repository/DB errors) propagate so the request fails
// loudly instead of returning a legitimate-looking empty list.
func (s *service) resolveStudentReadAccess(ctx context.Context) (studentReadAccess, error) {
	if authorize.HasPermission("admin:*", jwt.PermissionsFromCtx(ctx)) {
		return studentReadAccess{unrestricted: true}, nil
	}
	if s.userContext == nil {
		return studentReadAccess{groupIDs: map[int64]struct{}{}}, nil
	}

	staff, err := s.userContext.GetCurrentStaff(ctx)
	if err != nil {
		if errors.Is(err, usercontext.ErrUserNotLinkedToStaff) {
			return studentReadAccess{groupIDs: map[int64]struct{}{}}, nil
		}
		return studentReadAccess{}, fmt.Errorf("resolve current staff: %w", err)
	}
	if staff == nil {
		return studentReadAccess{groupIDs: map[int64]struct{}{}}, nil
	}

	scope, err := s.resolveStringSetting(
		ctx,
		configModel.KeyStudentDataScope,
		configModel.StudentDataScopeGroupSupervisorsOnly,
	)
	if err != nil {
		return studentReadAccess{}, err
	}
	if scope == configModel.StudentDataScopeAllStaff {
		return studentReadAccess{unrestricted: true}, nil
	}

	educationGroups, err := s.userContext.GetMyGroups(ctx)
	if err != nil {
		return studentReadAccess{}, fmt.Errorf("resolve supervised groups: %w", err)
	}
	groupIDs := make(map[int64]struct{}, len(educationGroups))
	for _, group := range educationGroups {
		groupIDs[group.ID] = struct{}{}
	}
	return studentReadAccess{groupIDs: groupIDs}, nil
}

// resolveStringSetting returns the tenant override for key, or fallback when no
// override exists (or the override is empty). A settings lookup FAILURE is not a
// fallback: it propagates so the caller fails loudly. For the GDPR read scope a
// silent fallback to the narrower group_supervisors_only would emit a
// preview/export that omits every unsupervised child while looking like a valid,
// authoritative daily list (#1565 review).
func (s *service) resolveStringSetting(ctx context.Context, key, fallback string) (string, error) {
	if s.settings == nil {
		return fallback, nil
	}
	has, err := s.settings.HasTenantOverride(ctx, key)
	if err != nil {
		return "", fmt.Errorf("check tenant override for %s: %w", key, err)
	}
	if !has {
		return fallback, nil
	}
	val, err := s.settings.ResolveString(ctx, key)
	if err != nil {
		return "", fmt.Errorf("resolve setting %s: %w", key, err)
	}
	if val == "" {
		return fallback, nil
	}
	return val, nil
}

func normalizeHHMM(value string) (string, error) {
	parsed, err := time.Parse(timeLayout, strings.TrimSpace(value))
	if err != nil {
		return "", err
	}
	return parsed.Format(timeLayout), nil
}

// listLabel derives the human list title from the already-resolved pickup
// cutoffs. It takes the snapshot rather than resolving its own so the label and
// the row collection observe the identical cutoffs within one build (#1565
// review pass 2); buckets is the zero value for slot lists, which never read it.
func (s *service) listLabel(params Params, buckets pickupBucketConfig) string {
	if params.Target == TargetSlots {
		if params.ListKind != ListKindNone {
			return params.ListKind.Label()
		}
		if len(params.InstanceIDs) == 1 {
			return "Ausgewähltes Angebot"
		}
		return "Freie Angebotsauswahl"
	}
	switch params.PickupCohort {
	case PickupCohortShortDay:
		return "Ganztag bis " + buckets.ShortCutoff
	case PickupCohortLongDay:
		return "Ganztag bis " + buckets.LongCutoff
	default:
		return params.Target.Label()
	}
}

func (s *service) provenance(params Params, listLabel string) string {
	plan := "Geplante Kinder laut Tagesplanung"
	if params.Target == TargetSlots && params.ListKind != ListKindNone {
		plan = "Geplante Kinder laut Tagesplanung (" + listLabel + ")"
	}
	if params.Target == TargetPickupCohort {
		plan = "Geplante Kinder laut Tagesplanung und Abholzeiten (" + listLabel + ")"
	}
	switch params.Source {
	case SourcePlanned:
		return plan
	case SourceActual:
		return "Dokumentierte Anwesenheit am Datum"
	case SourceReconciliation:
		return plan + " mit dokumentierter Anwesenheit abgeglichen"
	default:
		return ""
	}
}

func (s *service) BuildList(ctx context.Context, params Params) (*Result, error) {
	if s.instanceRepo == nil || s.instanceStudentRepo == nil || s.visitRepo == nil ||
		s.attendanceRepo == nil || s.statusDayRepo == nil || s.careDayService == nil ||
		s.pickupScheduleRepo == nil || s.studentRepo == nil ||
		s.personRepo == nil || s.educationGroupRepo == nil || s.roomRepo == nil ||
		s.pickupService == nil || s.arrivalService == nil || s.listExport == nil {
		return nil, fmt.Errorf("slot list service is not configured")
	}
	if !params.Target.Valid() {
		return nil, fmt.Errorf("unknown list target %q", params.Target)
	}
	if !params.Source.Valid() {
		return nil, fmt.Errorf("unknown source %q", params.Source)
	}
	if params.Target == TargetPickupCohort && !params.PickupCohort.Valid() {
		return nil, fmt.Errorf("unknown pickup cohort %q", params.PickupCohort)
	}
	if !params.ListKind.Valid() {
		return nil, fmt.Errorf("unknown list kind %q", params.ListKind)
	}
	if params.Target != TargetSlots && params.ListKind != ListKindNone {
		return nil, fmt.Errorf("list kind %q is not valid for target %q", params.ListKind, params.Target)
	}
	if !params.GroupBy.ValidFor(params.Target) {
		return nil, fmt.Errorf("grouping %q is not valid for target %q", params.GroupBy, params.Target)
	}
	// A Ganztag pickup plan cannot be reconstructed for a past date (the pickup
	// schedule has no history) — refuse rather than project the current schedule
	// backwards. Slot-based lists stay available for any date (#1565 review).
	if params.Target == TargetPickupCohort && params.Date.Before(s.todayDate()) {
		return nil, ErrPickupCohortPastDate
	}
	// A reconciliation on a strictly-future date has no presence evidence to merge
	// against, so every planned child would be labelled "Fehlt". Refuse it rather
	// than emit a list that reports the whole group missing before the day starts
	// (#1565 review). Today is fine — evidence accrues through the day; Plan/Ist
	// stay available for any date. Slots that have not started yet *on today* are
	// handled per-slot in collectSlotEntries (a not-yet-begun occurrence is
	// excluded from the merge, not reported as a group of missing children).
	if params.Source == SourceReconciliation && params.Date.After(s.todayDate()) {
		return nil, ErrReconciliationFutureDate
	}
	access, err := s.resolveStudentReadAccess(ctx)
	if err != nil {
		return nil, err
	}

	// Resolve the pickup-cohort cutoffs ONCE per build and reuse the snapshot for
	// both the list label/provenance and the row collection. Resolving them twice
	// (label, then collectPickupEntries) would let an administrator's mid-build
	// cutoff change land between the two reads under READ COMMITTED, so the header
	// could name a different threshold than the rows were bucketed by (#1565 review
	// pass 2). Only pickup lists need the cutoffs; a slot list neither reads them
	// nor should fail on an invalid pickup config.
	var pickupCfg pickupBucketConfig
	if params.Target == TargetPickupCohort {
		pickupCfg, err = s.pickupBuckets(ctx)
		if err != nil {
			return nil, err
		}
	}

	listLabel := s.listLabel(params, pickupCfg)

	result := &Result{
		Date:         params.Date.String(),
		Target:       params.Target,
		PickupCohort: params.PickupCohort,
		ListKind:     params.ListKind,
		ListLabel:    listLabel,
		Source:       params.Source,
		GroupBy:      params.GroupBy,
		Provenance:   s.provenance(params, listLabel),
		Slots:        []Slot{},
		Rows:         []Row{},
	}

	var entries []mergedEntry
	if params.Target == TargetPickupCohort {
		entries, err = s.collectPickupEntries(ctx, params, pickupCfg, result)
	} else {
		entries, err = s.collectSlotEntries(ctx, params, result)
	}
	if err != nil {
		return nil, err
	}

	// Full (unfiltered) rows for the requested source — used both to derive the
	// available group/class options and as the input to the row filters.
	allRows, err := s.enrichEntries(ctx, entries, params.Source, access, params.Date)
	if err != nil {
		return nil, err
	}
	result.Groups, result.Classes = availableOptions(allRows)

	rows := filterRows(allRows, params)
	applyGrouping(rows, params.GroupBy)
	sort.SliceStable(rows, func(i, j int) bool {
		// Keep grouped rows contiguous: section heading first, then the same
		// slot/name order within each section.
		if rows[i].GroupTitle != rows[j].GroupTitle {
			return rows[i].GroupTitle < rows[j].GroupTitle
		}
		if rows[i].Slot != rows[j].Slot {
			return rows[i].Slot < rows[j].Slot
		}
		if rows[i].Name != rows[j].Name {
			return rows[i].Name < rows[j].Name
		}
		// Stable ID tie-breakers: two rows can share group title, slot and full
		// name — e.g. two identically named unplanned attendees in one slot. Their
		// input order comes from map iteration / an incompletely ordered query,
		// while listSignature hashes InstanceID and StudentID positionally, so
		// without a deterministic order unchanged data could produce different
		// preview/export signatures and trigger false drift refusals (#1565 review
		// pass 9).
		if rows[i].InstanceID != rows[j].InstanceID {
			return rows[i].InstanceID < rows[j].InstanceID
		}
		return rows[i].StudentID < rows[j].StudentID
	})
	result.Rows = rows
	result.Counters = countRows(rows, params.Source)
	// Fingerprint the exact rendered header (title + every export filter line)
	// so the signature reflects the "Enthalten" slot summary and the other
	// metadata lines the export prints, not just rows and counters (#1565 review
	// pass 7). Computed here because the header needs Params.
	result.exportHeaderSig = strings.Join(
		append([]string{documentTitle(result)}, exportFilters(params, result)...),
		"\x1f",
	)
	result.Signature = listSignature(result)
	return result, nil
}

// listSignature is a stable content hash of the rendered list: its label,
// provenance, the precomputed export header (title + filter lines, which carry
// the "Enthalten" slot summary — see Result.exportHeaderSig), counters and every
// row's identity, placement and status. Rows are already sorted deterministically
// by BuildList, so a positional walk is stable across two builds of the same
// unchanged data. Unit/record separators (0x1f / 0x1e) delimit fields so no value
// can be confused with a boundary. See the Result.Signature doc for how the
// export drift guard uses it (#1565 review pass 2 + pass 7).
func listSignature(r *Result) string {
	var b strings.Builder
	b.WriteString(r.ListLabel)
	b.WriteByte('\x1e')
	b.WriteString(r.Provenance)
	b.WriteByte('\x1e')
	// The rendered document header (title + export filter lines), precomputed in
	// BuildList. Binds the signature to the "Enthalten" slot summary and the
	// date/group/class/grouping metadata the export prints — none of which the
	// row/counter hash below covers (#1565 review pass 7).
	b.WriteString(r.exportHeaderSig)
	b.WriteByte('\x1e')
	fmt.Fprintf(&b, "%d\x1f%d\x1f%d\x1f%d\x1f%d\x1e",
		r.Counters.Planned, r.Counters.Present, r.Counters.Missing,
		r.Counters.Excused, r.Counters.Unplanned)
	for _, row := range r.Rows {
		fmt.Fprintf(&b,
			"%d\x1f%s\x1f%s\x1f%s\x1f%s\x1f%d\x1f%s\x1f%s\x1f%s\x1f%s\x1f%t\x1f%t\x1f%t\x1f%t\x1f%s\x1e",
			row.StudentID, row.Name, row.SchoolClass, row.GroupName,
			groupIDSignatureField(row.GroupID), row.InstanceID, row.Slot,
			row.RoomName, row.PickupTime, row.StatusLabel, row.Planned,
			row.Present, row.Unplanned, row.Excused, row.GroupTitle)
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

func groupIDSignatureField(id *int64) string {
	if id == nil {
		return ""
	}
	return strconv.FormatInt(*id, 10)
}

func (s *service) ListOptions(ctx context.Context, date timezone.Date) (*OptionsResult, error) {
	// One pass over the day's instances instead of re-running the full list
	// builder once per list kind: on a normal school day the old shape issued
	// hundreds of per-slot roster/visit queries inside the tenant transaction
	// before the frontend's preview request repeated the same work.
	instances, err := s.instanceRepo.FindByTenantAndDate(ctx, date)
	if err != nil {
		return nil, err
	}
	slots := make([]Slot, 0, len(instances))
	kindInstanceIDs := map[ListKind][]int64{}
	// Resolve each slot's room name and list kind into the options payload so the
	// frontend can detect a room reassignment/rename or a list_kind change that
	// leaves the active instance-ID set unchanged before it exports (#1565 review
	// pass 4). The room lookup is cached per build, matching collectSlotEntries.
	roomCache := map[int64]string{}
	for _, inst := range instances {
		roomName, err := s.lookupRoomName(ctx, inst.RoomID, roomCache)
		if err != nil {
			return nil, err
		}
		listKind := ""
		if inst.ListKind != nil {
			listKind = *inst.ListKind
		}
		slots = append(slots, Slot{
			InstanceID: inst.ID,
			Title:      inst.Title,
			TimeRange:  fmt.Sprintf("%s\u2013%s", inst.StartTime.Format(timeLayout), inst.EndTime.Format(timeLayout)),
			Status:     inst.Status,
			ListKind:   listKind,
			RoomName:   roomName,
		})
		if inst.Status == scheduleModel.InstanceStatusCancelled || inst.ListKind == nil {
			continue
		}
		kind := ListKind(*inst.ListKind)
		if kind != ListKindNone && kind.Valid() {
			kindInstanceIDs[kind] = append(kindInstanceIDs[kind], inst.ID)
		}
	}

	// Options counts must respect the same GDPR read scope and date-effective
	// enrollment as /preview, or the availability hints would leak counts
	// derived from unsupervised children or from children not enrolled on the
	// requested date (#1565 review). Build the readable + enrolled student set
	// once and gate every aggregation on it.
	access, err := s.resolveStudentReadAccess(ctx)
	if err != nil {
		return nil, err
	}
	students, err := s.listEligibleStudents(ctx, date)
	if err != nil {
		return nil, err
	}
	studentIDs := make([]int64, 0, len(students))
	readable := make(map[int64]struct{}, len(students))
	for _, student := range students {
		studentIDs = append(studentIDs, student.ID)
		if access.canReadGroup(student.GroupID) {
			readable[student.ID] = struct{}{}
		}
	}

	// The date's care-day verdict, resolved once for every eligible child and
	// reused by the roster counts and the pickup cohorts below. An assignment
	// alone does not book a child into care on a given weekday (#1747): the
	// not_scheduled marker is only frozen onto the row when the block ends, so a
	// whole-group/year assignment would otherwise inflate the planned counts on
	// the days its members are not scheduled for.
	careDays := map[int64]scheduleSvc.CareDayStatus{}
	if len(studentIDs) > 0 {
		careDays, err = s.careDayService.ResolveForDate(ctx, studentIDs, date)
		if err != nil {
			return nil, fmt.Errorf("resolve care days: %w", err)
		}
	}
	completedByInstance := make(map[int64]bool, len(instances))
	for _, inst := range instances {
		completedByInstance[inst.ID] = inst.Status == scheduleModel.InstanceStatusCompleted
	}

	// One bulk roster load over every classified instance for the per-kind
	// planned row counts. Mirrors collectSlotEntries' planned semantics:
	// walk-in rows (is_unplanned, #1913) and non-booking rows — the frozen
	// not_scheduled marker or the live care-day verdict (#1747) — are not
	// planned children; only readable children count.
	classifiedIDs := make([]int64, 0)
	for _, ids := range kindInstanceIDs {
		classifiedIDs = append(classifiedIDs, ids...)
	}
	// Visit-derived presence for the classified instances, mirroring the list
	// builder's loadSlotPresence. The builder observes attendance through
	// active.visits (presentByGroup), so classifyPlannedRow can emit a child who
	// checked in on a CANCELLED care day as unplanned presence even when the
	// deliberately best-effort attendance sync left the roster row Expected instead
	// of flipping it to Present (visit created, sync failed — #1439/checkin). The
	// roster-only Status==Present check below cannot see that visit-only presence,
	// so it would miscount the child as a planned "Abgemeldet" row and overreport
	// /options.list_kinds[].row_count versus what Plan preview/export render. Load
	// the same visit evidence here and fold it into the presence signal (#1565
	// review pass 2 P2).
	activeGroupByInstance := make(map[int64]*int64, len(instances))
	for _, inst := range instances {
		activeGroupByInstance[inst.ID] = inst.ActiveGroupID
	}
	presentByGroup := map[int64]map[int64]bool{}
	presentViaVisit := func(instanceID, studentID int64) bool {
		activeGroupID := activeGroupByInstance[instanceID]
		if activeGroupID == nil {
			return false
		}
		byStudent := presentByGroup[*activeGroupID]
		return byStudent != nil && byStudent[studentID]
	}
	plannedByInstance := map[int64]int{}
	if len(classifiedIDs) > 0 {
		rosterRows, err := s.instanceStudentRepo.FindByInstanceIDs(ctx, classifiedIDs)
		if err != nil {
			return nil, err
		}
		// Bulk-load the visits of every classified instance that has been started,
		// exactly as loadSlotPresence does: presence is keyed by active-group ID
		// (only a started occurrence carries one), and a nil visitRepo simply
		// contributes no evidence.
		if s.visitRepo != nil {
			activeGroupIDs := make([]int64, 0, len(classifiedIDs))
			for _, id := range classifiedIDs {
				if activeGroupID := activeGroupByInstance[id]; activeGroupID != nil {
					activeGroupIDs = append(activeGroupIDs, *activeGroupID)
				}
			}
			if len(activeGroupIDs) > 0 {
				visits, err := s.visitRepo.FindByActiveGroupIDs(ctx, activeGroupIDs)
				if err != nil {
					return nil, err
				}
				for _, visit := range visits {
					if visit == nil {
						continue
					}
					byStudent := presentByGroup[visit.ActiveGroupID]
					if byStudent == nil {
						byStudent = map[int64]bool{}
						presentByGroup[visit.ActiveGroupID] = byStudent
					}
					byStudent[visit.StudentID] = true
				}
			}
		}
		for _, row := range rosterRows {
			if row.IsUnplanned || row.NotScheduled {
				continue
			}
			if _, ok := readable[row.StudentID]; !ok {
				continue
			}
			completed := completedByInstance[row.InstanceID]
			// Mirror collectSlotEntries exactly, including its RAW care-day
			// handling on unfinished occurrences: a bulk-assigned child who is not
			// booked for the weekday but checks in flips the row to present, so
			// AttendanceRowCareDay reports unknown (a real status tells its own
			// story). collectSlotEntries drops that row as present-only/unplanned
			// via the raw `CareDayNotScheduled` branch, so keying the count on the
			// status-gated verdict alone would claim a planned child the Plan
			// preview and export omit (#1565 review). A manual override still wins,
			// and a completed occurrence trusts its frozen marker (folded into
			// AttendanceRowCareDay below), never the live plan.
			if !completed && row.ManualStatusAt == nil &&
				careDays[row.StudentID] == scheduleSvc.CareDayNotScheduled {
				continue
			}
			// A cancelled care day the child ATTENDED anyway is unplanned presence,
			// not a planned row: classifyPlannedRow's cancelled branch emits such a
			// present child present-only (exactly like the IsUnplanned and
			// not_scheduled branches), never the planned "Abgemeldet" shape, so the
			// generic increment below would miscount it as planned and overreport
			// /options versus the Plan preview and export. Key on the RAW cancellation
			// verdict (a manual override still wins and is excluded), mirroring
			// classifyPlannedRow. Presence is derived exactly as classifyPlannedRows
			// does — `presentSet[student] || row.Status == Present` — so a normal
			// check-in that flipped the row to Present AND a check-in whose row status
			// never updated (the best-effort attendance sync failed but the visit
			// exists) are both recognized as unplanned presence. Without the visit
			// leg, a visit-only presence would count as planned here while the builder
			// classifies it unplanned, so the signatures diverge (#1565 review pass 2
			// P2). A cancelled NO-SHOW keeps its Expected/absent status, has no visit,
			// and still counts below as the "Abgemeldet" planned row it prints as
			// (#1565 review pass 1).
			if row.ManualStatusAt == nil &&
				careDays[row.StudentID] == scheduleSvc.CareDayCancelled &&
				(row.Status == scheduleModel.AttendanceStatusPresent ||
					presentViaVisit(row.InstanceID, row.StudentID)) {
				continue
			}
			verdict := scheduleSvc.AttendanceRowCareDay(completed, row, careDays[row.StudentID])
			// A genuine non-booking (not_scheduled) — whether the row still reads
			// Expected or was already stamped absent by a broad status day on a day
			// the child was never scheduled — is not a planned row and must not be
			// counted. A signed-off cancellation IS retained in collectSlotEntries
			// as a planned "Abgemeldet" row, so it must count here too, otherwise
			// /options underreports the classified list versus preview/export.
			if verdict == scheduleSvc.CareDayNotScheduled {
				continue
			}
			plannedByInstance[row.InstanceID]++
		}
	}
	listKinds := make([]ListKindOption, 0, len(AllListKinds))
	for _, kind := range AllListKinds {
		ids := kindInstanceIDs[kind]
		rowCount := 0
		for _, id := range ids {
			rowCount += plannedByInstance[id]
		}
		listKinds = append(listKinds, ListKindOption{
			Kind:      kind,
			Label:     kind.Label(),
			Available: len(ids) > 0,
			SlotCount: len(ids),
			RowCount:  rowCount,
		})
	}

	// One pickup sweep for both Ganztag cohorts over the same readable +
	// enrolled set, so the availability hint matches what /preview would show.
	// A past date is skipped entirely: BuildList refuses past pickup lists (the
	// schedule has no history), so their availability is always zero (#1565
	// review).
	buckets, err := s.pickupBuckets(ctx)
	if err != nil {
		return nil, err
	}
	cohortCounts := map[PickupCohort]int{}
	if !date.Before(s.todayDate()) && len(studentIDs) > 0 {
		pickupTimes, err := s.pickupService.GetBulkEffectivePickupTimesForDate(ctx, studentIDs, date)
		if err != nil {
			return nil, err
		}
		// Same cancelled-care fallback as collectPickupEntries: a cleared
		// same-day exception drops the effective time, but a cancelled verdict
		// (resolved once above into careDays) keeps the child visible via the
		// regular weekly bucket. Without this, the card reported zero while the
		// preview showed the signed-off child.
		regularRows, err := s.pickupScheduleRepo.FindByStudentIDsAndWeekday(ctx, studentIDs, int(date.Weekday()))
		if err != nil {
			return nil, fmt.Errorf("load regular pickup schedules: %w", err)
		}
		regularBucket := make(map[int64]string, len(regularRows))
		for _, row := range regularRows {
			regularBucket[row.StudentID] = row.PickupTime.Format(timeLayout)
		}
		for _, student := range students {
			if _, ok := readable[student.ID]; !ok {
				continue
			}
			cancelled := careDays[student.ID] == scheduleSvc.CareDayCancelled
			hhmm := cohortPickupTime(cancelled, pickupTimes[student.ID], regularBucket[student.ID])
			if hhmm == "" {
				continue
			}
			for _, cohort := range []PickupCohort{PickupCohortShortDay, PickupCohortLongDay} {
				if pickupMatchesCohort(cohort, hhmm, buckets) {
					cohortCounts[cohort]++
				}
			}
		}
	}
	cohortLabels := map[PickupCohort]string{
		PickupCohortShortDay: "Ganztag bis " + buckets.ShortCutoff,
		PickupCohortLongDay:  "Ganztag bis " + buckets.LongCutoff,
	}
	cohorts := make([]PickupCohortOption, 0, 2)
	for _, cohort := range []PickupCohort{PickupCohortShortDay, PickupCohortLongDay} {
		cohorts = append(cohorts, PickupCohortOption{
			Cohort:    cohort,
			Label:     cohortLabels[cohort],
			Available: cohortCounts[cohort] > 0,
			RowCount:  cohortCounts[cohort],
		})
	}
	return &OptionsResult{Date: date.String(), Slots: slots, PickupCohorts: cohorts, ListKinds: listKinds}, nil
}

// applyGrouping stamps each row's GroupTitle from the chosen dimension. With
// GroupByNone every title stays empty (a flat list). Rows whose group value is
// empty fall into a generic "Ohne …" bucket so they never silently vanish.
func applyGrouping(rows []Row, groupBy GroupBy) {
	if groupBy == GroupByNone {
		return
	}
	// GroupBySlot keys on the Slot label, but that label is NOT unique across
	// activity instances: two concrete instances can share a title and time range
	// (parallel offerings in different rooms, or two templates). Since both the
	// export's repeated-marker suppression and the frontend's group_title
	// bucketing key sections off the GroupTitle string, an undisambiguated label
	// would fold distinct instances into one section. Precompute a per-instance
	// heading suffix that only fires when a label actually collides (#1565 review
	// pass 2).
	slotSuffix := slotHeadingDisambiguation(rows, groupBy)
	for i := range rows {
		var value string
		switch groupBy {
		case GroupBySlot:
			value = rows[i].Slot
			value += slotSuffix[rows[i].InstanceID]
		case GroupByRoom:
			value = rows[i].RoomName
		case GroupByClass:
			value = rows[i].SchoolClass
		case GroupByPickupTime:
			value = rows[i].PickupTime
		}
		if value == "" {
			value = "Ohne " + groupBy.Label()
		}
		rows[i].GroupTitle = groupBy.Label() + ": " + value
	}
}

// slotHeadingDisambiguation returns a per-instance heading suffix for GroupBySlot
// so distinct activity instances that share one Slot label never merge into a
// single section. Instances whose label is already unique get no suffix (the
// map returns "" for them), so the common case renders exactly as before. When a
// label is shared by several instances, each colliding instance is tagged with
// its room name where that uniquely identifies it, otherwise a running ordinal —
// guaranteeing a distinct, deterministic heading without leaking raw IDs. Returns
// nil for any non-slot grouping (no suffix ever applied).
func slotHeadingDisambiguation(rows []Row, groupBy GroupBy) map[int64]string {
	if groupBy != GroupBySlot {
		return nil
	}
	// Per label: the distinct instance IDs (ascending, for stable ordinals) and
	// each instance's room.
	type labelInfo struct {
		ids   []int64
		room  map[int64]string
		known map[int64]struct{}
	}
	byLabel := map[string]*labelInfo{}
	for _, r := range rows {
		li := byLabel[r.Slot]
		if li == nil {
			li = &labelInfo{room: map[int64]string{}, known: map[int64]struct{}{}}
			byLabel[r.Slot] = li
		}
		if _, ok := li.known[r.InstanceID]; ok {
			continue
		}
		li.known[r.InstanceID] = struct{}{}
		li.ids = append(li.ids, r.InstanceID)
		li.room[r.InstanceID] = r.RoomName
	}
	suffix := map[int64]string{}
	for _, li := range byLabel {
		if len(li.ids) < 2 {
			continue // unique label — no disambiguation, heading stays clean
		}
		sort.Slice(li.ids, func(i, j int) bool { return li.ids[i] < li.ids[j] })
		roomCount := map[string]int{}
		for _, id := range li.ids {
			roomCount[li.room[id]]++
		}
		// Two passes with a shared `used` set so the room and ordinal suffixes can
		// never render the same string. A numeric room name like "1" produces the
		// suffix " (1)", which is textually identical to the first ordinal " (1)";
		// emitting both would hand two distinct instances the same GroupTitle and
		// re-merge the sections this function exists to split. First assign every
		// uniquely-identifying room its " (room)" suffix and record it; then fill the
		// remaining instances with running ordinals, SKIPPING any ordinal string a
		// room suffix already claimed. This keeps both forms clean (no verbose prefix,
		// no leaked IDs) while guaranteeing distinct, deterministic headings
		// (#1565 review pass 2 follow-up).
		used := map[string]struct{}{}
		for _, id := range li.ids {
			if room := li.room[id]; room != "" && roomCount[room] == 1 {
				s := " (" + room + ")"
				suffix[id] = s
				used[s] = struct{}{}
			}
		}
		ordinal := 0
		for _, id := range li.ids {
			if _, done := suffix[id]; done {
				continue
			}
			var s string
			for {
				ordinal++
				s = fmt.Sprintf(" (%d)", ordinal)
				if _, taken := used[s]; !taken {
					break
				}
			}
			suffix[id] = s
			used[s] = struct{}{}
		}
	}
	return suffix
}

// availableOptions derives the distinct education groups and school classes
// present in the unfiltered rows, sorted for stable UI dropdowns.
func availableOptions(rows []Row) ([]GroupOption, []string) {
	groupByID := map[int64]string{}
	classSet := map[string]struct{}{}
	for _, row := range rows {
		if row.GroupID != nil && row.GroupName != "" {
			groupByID[*row.GroupID] = row.GroupName
		}
		if row.SchoolClass != "" {
			classSet[row.SchoolClass] = struct{}{}
		}
	}
	groups := make([]GroupOption, 0, len(groupByID))
	for id, name := range groupByID {
		groups = append(groups, GroupOption{ID: id, Name: name})
	}
	sort.SliceStable(groups, func(i, j int) bool { return groups[i].Name < groups[j].Name })
	classes := make([]string, 0, len(classSet))
	for class := range classSet {
		classes = append(classes, class)
	}
	sort.Strings(classes)
	return groups, classes
}

// filterRows applies the offering/group/class filters (AND). An empty filter
// slice means "no restriction" on that dimension.
func filterRows(rows []Row, params Params) []Row {
	if params.Target == TargetSlots && params.InstanceIDsSet && len(params.InstanceIDs) == 0 {
		return []Row{}
	}
	instanceSet := int64Set(params.InstanceIDs)
	groupSet := int64Set(params.GroupIDs)
	classSet := make(map[string]struct{}, len(params.Classes))
	for _, c := range params.Classes {
		classSet[c] = struct{}{}
	}

	out := make([]Row, 0, len(rows))
	for _, row := range rows {
		// InstanceIDs are a slot-list filter only; they are ignored for pickup
		// cohorts (whose rows carry InstanceID 0), where a non-empty selection
		// would otherwise drop every row (#1565 review).
		if params.Target == TargetSlots && len(instanceSet) > 0 && !instanceSet[row.InstanceID] {
			continue
		}
		if len(groupSet) > 0 && (row.GroupID == nil || !groupSet[*row.GroupID]) {
			continue
		}
		if len(classSet) > 0 {
			if _, ok := classSet[row.SchoolClass]; !ok {
				continue
			}
		}
		out = append(out, row)
	}
	return out
}

func int64StructSet(ids []int64) map[int64]struct{} {
	set := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		set[id] = struct{}{}
	}
	return set
}

func int64Set(ids []int64) map[int64]bool {
	set := make(map[int64]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set
}

// mergedEntry is one (slot, student) pair before person/group enrichment.
type mergedEntry struct {
	StudentID        int64
	InstanceID       int64 // 0 for pickup-based cohorts
	SlotLabel        string
	RoomName         string // planned room of the slot; "" for pickup cohorts
	PickupTime       string
	Planned          bool
	Present          bool
	PlannedStatus    string  // schedule.instance_students.status, "" for pickup cohorts
	PlannedSubstatus *string // schedule.instance_students.substatus, nil for pickup cohorts
}

// collectSlotEntries derives the cohort from materialized activity instances
// (Plan) and their bridged active-group visits (Ist). Every visit reachable
// through the occurrence's 1:1 active-group bridge counts as present, including
// historical checked-out visits and sessions run outside the planned window, so
// past lists and late/early starts keep their attendance. A cancelled slot
// that never started produces no rows (selectable context only), but one that
// was cancelled *after* running keeps the visits recorded during its brief run
// as present rows — its void plan is dropped so no planned no-show prints as
// "Fehlt".
func (s *service) collectSlotEntries(ctx context.Context, params Params, result *Result) ([]mergedEntry, error) {
	contexts, deferredInstances, err := s.collectSlotContexts(ctx, params, result)
	if err != nil {
		return nil, err
	}
	rosterByInstance, careDay, presentByGroup, err := s.loadSlotPresence(ctx, contexts, params.Date)
	if err != nil {
		return nil, err
	}
	entries := []mergedEntry{}
	for _, p := range contexts {
		entries = append(entries, s.mergeSlotInstance(p, rosterByInstance, careDay, presentByGroup, deferredInstances, result)...)
	}
	return entries, nil
}

// slotContext is one activity instance selected for row collection, with its
// pre-resolved room name and printable slot label.
type slotContext struct {
	inst      *scheduleModel.ActivityInstance
	slotLabel string
	roomName  string
}

// collectSlotContexts runs the first pass over the date's instances: it publishes
// every matching slot as selectable context on result.Slots, returns the
// instances whose rosters the merge will actually read (selected, plus cancelled
// / not-yet-started occurrences that may still hold documented presence), and the
// set of deferred (not-yet-started reconciliation) occurrences.
func (s *service) collectSlotContexts(ctx context.Context, params Params, result *Result) ([]slotContext, map[int64]struct{}, error) {
	instances, err := s.instanceRepo.FindByTenantAndDate(ctx, params.Date)
	if err != nil {
		return nil, nil, err
	}

	// A non-nil set restricts data collection to the selected instances;
	// nil means "all" (the default when no slots are explicitly selected).
	// The option list (result.Slots) is always populated for every matching
	// instance regardless — only the expensive per-instance roster/visit
	// reads are gated, so selecting one slot no longer costs the same as
	// selecting all, and an explicit empty selection issues no per-slot reads.
	var selected map[int64]struct{}
	if params.InstanceIDsSet || len(params.InstanceIDs) > 0 {
		selected = int64StructSet(params.InstanceIDs)
	}

	roomCache := map[int64]string{}
	process := make([]slotContext, 0, len(instances))
	// deferredInstances flags reconciliation occurrences reached before their
	// scheduled start with no active group yet. They still enter `process` (their
	// roster may hold manually-recorded presence), but the merge suppresses their
	// void plan so no not-yet-arrived child prints as "Fehlt" (#1565 review pass 2).
	var deferredInstances map[int64]struct{}
	for _, inst := range instances {
		if !instanceMatchesListKind(inst, params.ListKind) {
			continue
		}
		sc, deferred, err := s.buildSlotContext(ctx, inst, params, roomCache, result)
		if err != nil {
			return nil, nil, err
		}
		// deferredInstances is recorded before the selection gate — a non-selected
		// deferred slot is never looked up, but keeping the order identical to the
		// pre-split code avoids any behavioral surprise.
		if deferred {
			if deferredInstances == nil {
				deferredInstances = map[int64]struct{}{}
			}
			deferredInstances[inst.ID] = struct{}{}
		}
		if selected != nil {
			if _, ok := selected[inst.ID]; !ok {
				continue
			}
		}
		process = append(process, sc)
	}
	return process, deferredInstances, nil
}

// buildSlotContext publishes one matching instance as selectable context on
// result.Slots and returns its slotContext plus whether it is a deferred
// (not-yet-started reconciliation) occurrence.
func (s *service) buildSlotContext(ctx context.Context, inst *scheduleModel.ActivityInstance, params Params, roomCache map[int64]string, result *Result) (slotContext, bool, error) {
	roomName, err := s.lookupRoomName(ctx, inst.RoomID, roomCache)
	if err != nil {
		return slotContext{}, false, err
	}
	listKind := ""
	if inst.ListKind != nil {
		listKind = *inst.ListKind
	}
	result.Slots = append(result.Slots, Slot{
		InstanceID: inst.ID,
		Title:      inst.Title,
		TimeRange:  fmt.Sprintf("%s–%s", inst.StartTime.Format(timeLayout), inst.EndTime.Format(timeLayout)),
		Status:     inst.Status,
		ListKind:   listKind,
		RoomName:   roomName,
	})
	cancelled := inst.Status == scheduleModel.InstanceStatusCancelled
	// A cancelled or not-yet-started occurrence carries a void plan, but it may
	// still hold documented attendance: InstanceService.Cancel keeps the visits
	// recorded during a brief run, and a timetable PATCH can mark a child
	// present (status=present) with no active group at all — before the slot
	// starts, or on a cancelled slot that never ran. Both cases are resolved in
	// the merge (it suppresses the void plan yet retains the present rows), so
	// route these occurrences into `process` instead of dropping them here, which
	// would erase that attendance from Ist and Abgleich (#1565 review pass 1 P2 /
	// pass 2 P1). They stay selectable context in result.Slots (appended above)
	// regardless.
	//
	// A reconciliation reaching an occurrence before its scheduled start, with
	// no active group yet, has no live presence to merge against, so every
	// planned child would collapse to Present=false and render as "Fehlt" — a
	// safety-relevant false missing-child list before the activity begins. Flag
	// it deferred so the merge suppresses those planned no-shows (while still
	// surfacing any manually-present child) and the export summary does not
	// count a slot that produced no rows. Gate strictly on occurrences that have
	// NOT actually started: a slot Start()ed manually before its nominal time
	// already carries an ActiveGroupID and real visits, so it stays on the
	// normal path. Cancelled occurrences are handled by their own void-plan
	// branch, not deferred here. This only ever fires for reconciliation on
	// today — a future date is refused outright in BuildList; Plan/Ist are
	// unaffected.
	deferred := !cancelled &&
		params.Source == SourceReconciliation && inst.ActiveGroupID == nil
	if deferred {
		if start, _ := instanceTimeRange(inst); !s.currentTime().Before(start) {
			deferred = false
		}
	}
	slotLabel := fmt.Sprintf("%s (%s–%s)", inst.Title, inst.StartTime.Format(timeLayout), inst.EndTime.Format(timeLayout))
	return slotContext{inst: inst, slotLabel: slotLabel, roomName: roomName}, deferred, nil
}

// loadSlotPresence bulk-loads the rosters, the shared care-day verdict, and the
// visit-derived presence for every instance the merge will process.
func (s *service) loadSlotPresence(ctx context.Context, process []slotContext, date timezone.Date) (
	map[int64][]*scheduleModel.InstanceStudent,
	map[int64]scheduleSvc.CareDayStatus,
	map[int64]map[int64]bool,
	error,
) {
	// Bulk-load the rosters of every instance we will process, then resolve the
	// date's care-day verdict once for all planned children. An assignment alone
	// does not book a child into care on a given weekday (#1747): a whole-group
	// or whole-year assignment lists members on the days they are not scheduled
	// for, and the not_scheduled marker is only frozen onto the row when the
	// block ends. Without the live verdict such a child reaches the default
	// branch below, counts as planned, and shows as "Fehlt" in the Abgleich
	// though they were never booked.
	rosterByInstance := make(map[int64][]*scheduleModel.InstanceStudent, len(process))
	careDay := map[int64]scheduleSvc.CareDayStatus{}
	// presentByGroup maps an active-group ID to the students seen present in it.
	// A slot only has visit evidence once it carries an ActiveGroupID, so this is
	// keyed by group, not instance; instances without one (never started — e.g.
	// most Plan requests) contribute nothing and cost no query.
	presentByGroup := map[int64]map[int64]bool{}
	if len(process) == 0 {
		return rosterByInstance, careDay, presentByGroup, nil
	}
	instanceIDs := make([]int64, 0, len(process))
	activeGroupIDs := make([]int64, 0, len(process))
	for _, p := range process {
		instanceIDs = append(instanceIDs, p.inst.ID)
		if p.inst.ActiveGroupID != nil {
			activeGroupIDs = append(activeGroupIDs, *p.inst.ActiveGroupID)
		}
	}
	rosterRows, err := s.instanceStudentRepo.FindByInstanceIDs(ctx, instanceIDs)
	if err != nil {
		return nil, nil, nil, err
	}
	studentIDSet := map[int64]struct{}{}
	for _, row := range rosterRows {
		rosterByInstance[row.InstanceID] = append(rosterByInstance[row.InstanceID], row)
		studentIDSet[row.StudentID] = struct{}{}
	}
	studentIDs := make([]int64, 0, len(studentIDSet))
	for id := range studentIDSet {
		studentIDs = append(studentIDs, id)
	}
	careDay, err = s.careDayService.ResolveForDate(ctx, studentIDs, date)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("resolve care days: %w", err)
	}
	// Bulk-load every started slot's visits in one query rather than probing
	// each instance individually inside the merge loop below — a historical
	// day with many slots otherwise runs N sequential visit queries per
	// preview/export/filter change.
	//
	// The instance↔active.group bridge is 1:1 (a UNIQUE partial index on
	// schedule.activity_instances.active_group_id) and the active.group is
	// created fresh when the occurrence is started, so every visit reachable
	// through it is that occurrence's own attendance — it is deliberately NOT
	// time-filtered against the planned window. Staff may Start an instance
	// late, or start and complete it early: InstanceService.Start permits the
	// transition regardless of the nominal StartTime/EndTime, which pushes
	// those visits wholly outside the scheduled window. Comparing them to the
	// planned times would drop documented attendance, omitting present
	// children from Ist lists and printing attended children as "Fehlt" in the
	// Abgleich (#1565 review pass 2).
	visits, err := s.visitRepo.FindByActiveGroupIDs(ctx, activeGroupIDs)
	if err != nil {
		return nil, nil, nil, err
	}
	for _, visit := range visits {
		if visit == nil {
			continue
		}
		byStudent := presentByGroup[visit.ActiveGroupID]
		if byStudent == nil {
			byStudent = map[int64]bool{}
			presentByGroup[visit.ActiveGroupID] = byStudent
		}
		byStudent[visit.StudentID] = true
	}
	return rosterByInstance, careDay, presentByGroup, nil
}

// mergeSlotInstance merges one instance's roster (Plan) with its visit-derived
// presence (Ist) into display entries and records the export "Enthalten"
// accounting for void-plan slots.
func (s *service) mergeSlotInstance(
	p slotContext,
	rosterByInstance map[int64][]*scheduleModel.InstanceStudent,
	careDay map[int64]scheduleSvc.CareDayStatus,
	presentByGroup map[int64]map[int64]bool,
	deferredInstances map[int64]struct{},
	result *Result,
) []mergedEntry {
	inst := p.inst
	planned := rosterByInstance[inst.ID]
	cancelled := inst.Status == scheduleModel.InstanceStatusCancelled
	_, deferred := deferredInstances[inst.ID]
	// A void plan: the occurrence was called off (cancelled), or a
	// reconciliation reached it before its scheduled start with no active group
	// yet (deferred). In neither case is a roster row a genuine expectation, so
	// none may print as "Fehlt". The retention pass below keeps only the
	// manually-present rows before the switch runs.
	voidPlan := cancelled || deferred
	// Presence was bulk-loaded above; a slot with no active group (never
	// started) simply has no entry and yields a nil map (safe to index).
	var presentSet map[int64]bool
	if inst.ActiveGroupID != nil {
		presentSet = presentByGroup[*inst.ActiveGroupID]
	}
	completed := inst.Status == scheduleModel.InstanceStatusCompleted

	entries := []mergedEntry{}
	seenPlanned := make(map[int64]struct{}, len(planned))
	retainedRows := false
	if voidPlan {
		retained, any := s.retainVoidPlanRows(p, planned, careDay, deferred, seenPlanned)
		entries = append(entries, retained...)
		retainedRows = any
		planned = nil
	}
	entries = append(entries, s.classifyPlannedRows(p, planned, presentSet, completed, careDay, seenPlanned)...)
	unseen, anyPresent := appendUnseenPresent(p, presentSet, seenPlanned)
	entries = append(entries, unseen...)
	if anyPresent {
		retainedRows = true
	}
	recordVoidPlanAccounting(result, inst.ID, cancelled, deferred, retainedRows)
	return entries
}

// classifyPlannedRows classifies every roster row of a live (non-void) slot,
// stamping each accounted-for student into seenPlanned so the presence sweep does
// not re-add them.
func (s *service) classifyPlannedRows(
	p slotContext,
	planned []*scheduleModel.InstanceStudent,
	presentSet map[int64]bool,
	completed bool,
	careDay map[int64]scheduleSvc.CareDayStatus,
	seenPlanned map[int64]struct{},
) []mergedEntry {
	entries := []mergedEntry{}
	for _, row := range planned {
		present := presentSet[row.StudentID] || row.Status == scheduleModel.AttendanceStatusPresent
		// A manual attendance correction is a human decision that outranks stale
		// visit evidence. UpdateAttendanceFields stamps ManualStatusAt whenever
		// staff set the row's status by hand, so when an erroneous scan created a
		// visit (row still in presentSet) and staff then corrected the roster to
		// absent or expected, the visit must NOT force Present=true: trust the
		// corrected row status alone. Without this the Ist/Abgleich lists and their
		// counters would report the child "Anwesend" against the explicit human
		// correction (#1565 review pass 12).
		if row.ManualStatusAt != nil {
			present = row.Status == scheduleModel.AttendanceStatusPresent
		}
		// The canonical care-day verdict decides whether an assignment row
		// is a genuine expectation on this date; it folds the frozen
		// not_scheduled marker and the live care-plan derivation into the
		// single answer every reader (planner, roster, cards) shares (#1747).
		verdict := scheduleSvc.AttendanceRowCareDay(completed, row, careDay[row.StudentID])
		emitted, markSeen := s.classifyPlannedRow(p, row, present, verdict, careDay[row.StudentID])
		if markSeen {
			seenPlanned[row.StudentID] = struct{}{}
		}
		entries = append(entries, emitted...)
	}
	return entries
}

// appendUnseenPresent emits a present-only row for every student with visit
// evidence that the planned classification did not already account for, and
// reports whether any such row was produced (for the void-plan accounting).
func appendUnseenPresent(p slotContext, presentSet map[int64]bool, seenPlanned map[int64]struct{}) ([]mergedEntry, bool) {
	entries := []mergedEntry{}
	anyPresent := false
	for studentID, present := range presentSet {
		if !present {
			continue
		}
		if _, ok := seenPlanned[studentID]; ok {
			continue
		}
		anyPresent = true
		entries = append(entries, mergedEntry{
			StudentID:  studentID,
			InstanceID: p.inst.ID,
			SlotLabel:  p.slotLabel,
			RoomName:   p.roomName,
			Present:    true,
		})
	}
	return entries, anyPresent
}

// recordVoidPlanAccounting records the export "Enthalten" accounting for a
// void-plan slot. A cancelled slot that retained any present child — from a
// manual roster mark or a visit — counts as a contained Termin so the header does
// not undercount; a deferred (not-yet-started) slot that retained no rows at all
// is recorded so the summary does not count a slot the merge produced nothing
// for. A deferred slot that DID retain a row — a present child or a "Abgemeldet"
// registered absence — is deliberately left unrecorded: it contributed rows and
// must count normally. cancelled and deferred are mutually exclusive (the first
// pass never defers a cancelled slot) (#1565 review).
func recordVoidPlanAccounting(result *Result, instanceID int64, cancelled, deferred, retainedRows bool) {
	switch {
	case cancelled && retainedRows:
		if result.retainedCancelledSlots == nil {
			result.retainedCancelledSlots = map[int64]struct{}{}
		}
		result.retainedCancelledSlots[instanceID] = struct{}{}
	case deferred && !retainedRows:
		if result.deferredSlots == nil {
			result.deferredSlots = map[int64]struct{}{}
		}
		result.deferredSlots[instanceID] = struct{}{}
	}
}

// retainVoidPlanRows keeps the roster rows that must survive a void plan
// (cancelled or deferred slot): documented presence always, and — on a deferred
// (not-yet-started) slot only — a registered all-day sign-off rendered
// "Abgemeldet". It records every retained student in seenPlanned and reports
// whether any row was kept (for the export "Enthalten" accounting).
//
// A void plan is never a source of "Fehlt": the caller nils `planned` after this
// so no roster row reaches the switch. Three kinds of row still carry real
// information that must survive that suppression:
//
//   - Present evidence. A timetable PATCH records status=present on the roster row
//     without ever attaching an active group, so such a child is absent from the
//     visit-derived presentSet. Retain those rows as unplanned-present evidence so
//     documented attendance still surfaces in Ist and Abgleich (#1565 review
//     pass 1 P2 / pass 2 P1).
//
//   - A registered absence on a DEFERRED (not-yet-started) slot. A child signed
//     off sick/excused/class-trip (ApplyStatusDay stamps the row absent with the
//     substatus) or with a cancelled care day ("Kommt heute nicht") is a valid
//     all-day sign-off, not a not-yet-due no-show. Retain it as "Abgemeldet" —
//     mirroring the pickup-cohort reconciliation, which defers only the unexplained
//     would-be-"Fehlt" child and keeps registered absences — so it shows in the
//     Abgleich and the excused counter before the slot's scheduled start (#1565
//     review pass 1 P2). A cancelled slot is deliberately excluded: the whole
//     occurrence was called off, so a per-slot sign-off is moot and only present
//     evidence is retained.
//
//   - A manual NON-present correction (ManualStatusAt set, status absent/expected)
//     on a CANCELLED slot. The slot retained its ActiveGroupID and the erroneous
//     scan's visit, so the child is still in presentSet; the human correction must
//     win over that stale visit, exactly as classifyPlannedRows enforces on a live
//     slot. Such a row emits nothing (a void plan prints no "Fehlt") but is marked
//     seen so the presence sweep does not resurrect the visit and report the child
//     "Anwesend" against the correction (#1565 review pass 12 / P1).
func (s *service) retainVoidPlanRows(
	p slotContext,
	planned []*scheduleModel.InstanceStudent,
	careDay map[int64]scheduleSvc.CareDayStatus,
	deferred bool,
	seenPlanned map[int64]struct{},
) ([]mergedEntry, bool) {
	entries := []mergedEntry{}
	retained := false
	for _, row := range planned {
		switch {
		case row.Status == scheduleModel.AttendanceStatusPresent:
			seenPlanned[row.StudentID] = struct{}{}
			retained = true
			entries = append(entries, mergedEntry{
				StudentID:  row.StudentID,
				InstanceID: p.inst.ID,
				SlotLabel:  p.slotLabel,
				RoomName:   p.roomName,
				Present:    true,
			})
		case deferred:
			substatus, ok := voidPlanRegisteredAbsence(row, careDay[row.StudentID])
			if !ok {
				continue
			}
			seenPlanned[row.StudentID] = struct{}{}
			retained = true
			substatusCopy := substatus
			entries = append(entries, mergedEntry{
				StudentID:        row.StudentID,
				InstanceID:       p.inst.ID,
				SlotLabel:        p.slotLabel,
				RoomName:         p.roomName,
				Planned:          true,
				Present:          false,
				PlannedStatus:    scheduleModel.AttendanceStatusAbsent,
				PlannedSubstatus: &substatusCopy,
			})
		case row.ManualStatusAt != nil:
			// A manual correction to a NON-present status (absent or expected; the
			// present case above already caught a manual present) is a human
			// decision that outranks stale visit evidence, exactly as
			// classifyPlannedRows treats it on a live slot. This case is reached
			// only for a CANCELLED slot (the `case deferred` above fully handles
			// deferred rows, routing registered sign-offs — which ApplyStatusDay
			// stamps WITHOUT ManualStatusAt — to "Abgemeldet" and continuing past
			// everything else). A slot cancelled after it ran keeps its
			// ActiveGroupID and the erroneous scan's visit, so the child is still in
			// presentSet; without accounting for the row here appendUnseenPresent
			// would resurrect that visit and report the child "Anwesend" against the
			// explicit correction. Mark the student seen so the presence sweep skips
			// them, and emit nothing — a void plan is never a source of
			// "Fehlt"/"Abgemeldet", so a manual non-present correction simply drops
			// the child from the called-off slot (#1565 review pass 12 / P1).
			seenPlanned[row.StudentID] = struct{}{}
		}
	}
	return entries, retained
}

// classifyPlannedRow maps one roster row of a live (non-void) slot to its merged
// entries and reports whether the student was accounted for (seenPlanned).
// `present` is already corrected for a manual attendance override by the caller;
// `verdict` is the status-gated care-day verdict and `rawCareDay` the raw
// derivation the non-booking / cancellation branches key on.
func (s *service) classifyPlannedRow(
	p slotContext,
	row *scheduleModel.InstanceStudent,
	present bool,
	verdict, rawCareDay scheduleSvc.CareDayStatus,
) ([]mergedEntry, bool) {
	inst := p.inst
	// presentOnly emits a single unplanned-present row (or nothing when absent).
	// Shared by the IsUnplanned, raw not_scheduled and cancelled-but-attended
	// branches, which all surface documented presence as "ungeplant anwesend".
	presentOnly := func() []mergedEntry {
		if !present {
			return nil
		}
		return []mergedEntry{{
			StudentID:  row.StudentID,
			InstanceID: inst.ID,
			SlotLabel:  p.slotLabel,
			RoomName:   p.roomName,
			Present:    true,
		}}
	}
	completed := inst.Status == scheduleModel.InstanceStatusCompleted
	switch {
	case row.IsUnplanned:
		// #1913: the row was created by an observed walk-in visit, not by
		// planning. It is durable presence evidence, never a plan entry —
		// otherwise the Abgleich would relabel the walk-in as "geplant & anwesend".
		return presentOnly(), true
	case !completed && row.ManualStatusAt == nil && rawCareDay == scheduleSvc.CareDayNotScheduled:
		// #1747/#1565 review: the care plan never booked this child into the OGS
		// on this weekday. Key on the raw care-day verdict, not the status-gated
		// `verdict`, exactly as the cancellation branch below does: a real check-in
		// flips the row to present and AttendanceRowCareDay then reports unknown (a
		// real status tells its own story), so keying on `verdict` here would lose
		// the non-booking and mislabel a bulk-assigned walk-in as "geplant &
		// anwesend" instead of "ungeplant anwesend". The row may read Expected (the
		// not_scheduled marker is only frozen when the block ends), already be
		// present, or carry an absent status a broad sick/excused status day
		// stamped onto a day the child was never scheduled. None of those is a
		// genuine expectation, so none may print as planned/"Fehlt"/"Abgemeldet":
		// drop it and leave the child unseen so a live visit still surfaces them as
		// unplanned-present below (an absence on an unbooked day shows nowhere). A
		// manual override (ManualStatusAt) still wins and is excluded here, and a
		// signed-off (cancelled) booked day is handled next and stays "Abgemeldet".
		//
		// This RAW-verdict branch is confined to UNFINISHED occurrences
		// (`!completed`): the live care-day derivation reads today's recurring
		// arrival/pickup plan, which is not historized, so on a completed
		// historical slot it may have drifted from what was true then. Ending the
		// block already froze the authoritative verdict — not_scheduled onto the
		// children it spared, real absences onto the booked no-shows — so a
		// completed slot must trust that frozen state (handled by the
		// Expected/!verdict.Expected() and default cases below), never the live
		// plan. Without this gate a post-hoc care-plan edit could relabel a
		// then-planned present child as unplanned, or make a then-planned absence
		// vanish, making past Plan/Abgleich exports depend on today's schedule
		// (#1565 review).
		//
		// Emit the present-only row directly rather than deferring to the present
		// loop: on the slot path `present` can come from a timetable PATCH (row
		// status) with no active-group visit, and the present loop only re-surfaces
		// visit-derived presence — deferring would drop such a child entirely
		// instead of showing "ungeplant anwesend". This mirrors the IsUnplanned
		// branch above.
		return presentOnly(), true
	case row.ManualStatusAt == nil && rawCareDay == scheduleSvc.CareDayCancelled:
		// #1747/#1565 review: a "Kommt heute nicht" cancelled the booked day. The
		// cancellation is care-plan evidence that outlives the row's attendance
		// status, so key on the raw care-day verdict, not the status-gated
		// `verdict`: a real check-in flips the row to present and a completed block
		// flips a no-show to absent, and for both AttendanceRowCareDay reports
		// unknown (a real/finalized status tells its own story). Keying on `verdict`
		// there would lose the cancellation entirely and mislabel the walk-in as
		// "geplant & anwesend" or the no-show as an unexplained "Fehlt". A manual
		// override (ManualStatusAt) still wins and is excluded here, exactly as
		// AttendanceRowCareDay does.
		//
		// Unlike a genuine non-booking, the day WAS booked and the child signed
		// off, so a no-show belongs on the list as "Abgemeldet" (the exact
		// registered-absence shape the pickup path renders), never silently dropped
		// like not_scheduled. If the child attended anyway, that is unplanned
		// presence: emit the present-only row directly here rather than defer to the
		// present loop below. On the slot path `present` can come from a timetable
		// PATCH (row status) with no active-group visit, and that loop only
		// re-surfaces visit-derived presence (presentSet) — deferring would drop
		// such a child from both Ist and Abgleich entirely instead of showing
		// "ungeplant anwesend" (#1565 review pass 2). This mirrors the IsUnplanned
		// and not_scheduled branches, which emit their present-only row inline for
		// the same reason.
		if present {
			return presentOnly(), true
		}
		cancelledSubstatus := string(scheduleSvc.CareDayCancelled)
		return []mergedEntry{{
			StudentID:        row.StudentID,
			InstanceID:       inst.ID,
			SlotLabel:        p.slotLabel,
			RoomName:         p.roomName,
			Planned:          true,
			Present:          false,
			PlannedStatus:    scheduleModel.AttendanceStatusAbsent,
			PlannedSubstatus: &cancelledSubstatus,
		}}, true
	case row.Status == scheduleModel.AttendanceStatusExpected && !verdict.Expected():
		// #1747 non-booking safety net for a COMPLETED block: ending the slot
		// froze the not_scheduled marker into the row, so AttendanceRowCareDay
		// keeps returning not_scheduled from that frozen flag even if the live
		// care plan has since changed (reading the current plan on a finished
		// day would let a later edit relabel it). The live-derivation non-booking
		// on an active block is already handled by the raw care-day case above;
		// here the frozen verdict is authoritative. Not a genuine expectation:
		// skip, and leave the student unseen so a live visit can still surface them
		// as unplanned below.
		return nil, false
	case row.NotScheduled:
		// Unbooked day the system decided anyway (checked in, or an absence
		// written by a status day). The child was never planned for this slot;
		// presence shows as unplanned, an absence on an unbooked day shows nowhere.
		return presentOnly(), true
	default:
		return []mergedEntry{{
			StudentID:        row.StudentID,
			InstanceID:       inst.ID,
			SlotLabel:        p.slotLabel,
			RoomName:         p.roomName,
			Planned:          true,
			Present:          present,
			PlannedStatus:    row.Status,
			PlannedSubstatus: row.Substatus,
		}}, true
	}
}

func instanceMatchesListKind(inst *scheduleModel.ActivityInstance, listKind ListKind) bool {
	if listKind == ListKindNone {
		return true
	}
	if inst == nil || inst.ListKind == nil {
		return false
	}
	return *inst.ListKind == string(listKind)
}

// lookupRoomName resolves a room's display name, cached per build. A missing
// room is non-fatal (room grouping just shows the "Ohne Raum" bucket).
func (s *service) lookupRoomName(ctx context.Context, roomID int64, cache map[int64]string) (string, error) {
	if roomID == 0 {
		return "", nil
	}
	if name, ok := cache[roomID]; ok {
		return name, nil
	}
	room, err := s.roomRepo.FindByID(ctx, roomID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			cache[roomID] = ""
			return "", nil
		}
		return "", fmt.Errorf("lookup room %d: %w", roomID, err)
	}
	if room == nil {
		cache[roomID] = ""
		return "", nil
	}
	cache[roomID] = room.Name
	return room.Name, nil
}

// beforeEffectiveArrival reports whether the service clock is still earlier than
// the child's effective arrival instant on the date. The arrival value is a
// wall-clock time, so it is anchored to the calendar day in Berlin exactly as
// instanceTimeRange anchors a slot's start — never trusted as a raw instant. A
// nil arrival time (no schedule for this weekday, or a weekend) yields false:
// with no expected arrival to defer against, the normal merge applies and a
// genuine no-show still surfaces.
func (s *service) beforeEffectiveArrival(date timezone.Date, arrival *scheduleSvc.EffectiveArrivalTime) bool {
	if arrival == nil || arrival.ArrivalTime == nil {
		return false
	}
	at := *arrival.ArrivalTime
	start := time.Date(
		date.Year, date.Month, date.Day,
		at.Hour(), at.Minute(), at.Second(), at.Nanosecond(),
		timezone.Berlin,
	)
	return s.currentTime().Before(start)
}

func instanceTimeRange(inst *scheduleModel.ActivityInstance) (time.Time, time.Time) {
	start := time.Date(
		inst.Date.Year, inst.Date.Month, inst.Date.Day,
		inst.StartTime.Hour(), inst.StartTime.Minute(), inst.StartTime.Second(), inst.StartTime.Nanosecond(),
		timezone.Berlin,
	)
	end := time.Date(
		inst.Date.Year, inst.Date.Month, inst.Date.Day,
		inst.EndTime.Hour(), inst.EndTime.Minute(), inst.EndTime.Second(), inst.EndTime.Nanosecond(),
		timezone.Berlin,
	)
	return start, end
}

// eligibleOn reports whether a student is enrolled in the OGS on the given
// calendar date. The enrollment interval (enrolled_from..enrolled_until) is the
// source of truth: it is correct for past and future dates alike, whereas the
// lifecycle status is only the scheduler's projection of "enrolled today" and
// is wrong for any other date — a currently active child whose enrollment ends
// before a future list date would otherwise still be counted, and a pending
// child whose enrollment has already started would be missed (#1565 review).
//
// Immediate activation (enrollment.default_activation_mode = "immediate") is the
// deliberate exception: the enrollment decision service creates an already
// 'active' student while keeping enrolled_from at the phase's future
// ServiceStartDate, so the child appears in lists/attendance from today. An
// active status therefore overrides the enrolled_from lower bound — but only
// from today onward: the override lets the child appear for the current and
// future dates before the phase officially starts, it must NOT make the child
// retroactively enrolled for every past date before enrolled_from. Otherwise a
// stale or manually created slot roster (or the /options counts) would show the
// child as planned/missing before their enrollment ever began (#1565 review).
// enrolled_until still drives deactivation for every status.
//
// When neither bound is recorded (legacy rows, manual create) the interval
// carries no information, so the current lifecycle status is the only signal
// and an inactive student is treated as no longer enrolled.
//
// today is the service clock's calendar day (s.todayDate()), threaded in so
// the immediate-activation cutoff uses the same clock as every other date
// guard in BuildList/ListOptions — deterministic simulations and
// time-controlled tests pin Dependencies.Now, and reading the process clock
// here instead would decide eligibility against a different "today" (#1565
// review).
func eligibleOn(student *userModel.Student, date, today timezone.Date) bool {
	if student == nil {
		return false
	}
	active := student.Status == userModel.StudentStatusActive
	if student.EnrolledFrom != nil && date.Before(*student.EnrolledFrom) {
		// Before the recorded start date, an active child is only eligible from
		// today onward (immediate activation); past dates keep the lower bound.
		if !active || date.Before(today) {
			return false
		}
	}
	if student.EnrolledUntil != nil && date.After(*student.EnrolledUntil) {
		return false
	}
	if student.EnrolledFrom == nil && student.EnrolledUntil == nil {
		return student.Status != userModel.StudentStatusInactive
	}
	return true
}

// listEligibleStudents returns the tenant students enrolled on the given date —
// the cohort candidate set shared by the pickup builder and the options
// aggregation. Loading the full set and filtering by enrollment interval (not a
// status filter) is what keeps pickup cohorts correct for non-today dates.
func (s *service) listEligibleStudents(ctx context.Context, date timezone.Date) ([]*userModel.Student, error) {
	all, err := s.studentRepo.List(ctx, map[string]interface{}{})
	if err != nil {
		return nil, err
	}
	today := s.todayDate()
	eligible := make([]*userModel.Student, 0, len(all))
	for _, student := range all {
		if eligibleOn(student, date, today) {
			eligible = append(eligible, student)
		}
	}
	return eligible, nil
}

// cohortPickupTime returns the HH:MM used to place a student into a Ganztag
// cohort on a date, or "" when the child is not in care that day. A cleared
// same-day exception ("Kommt heute nicht") drops the effective pickup time; a
// cancelled care verdict then falls back to the regular weekly bucket so the
// signed-off child still appears in their cohort. Preview, export, and the
// options availability counts MUST all apply this identical fallback or they
// disagree on cancelled care days (#1565 review).
func cohortPickupTime(cancelled bool, effective *scheduleSvc.EffectivePickupTime, regular string) string {
	if effective != nil && effective.PickupTime != nil {
		return effective.PickupTime.Format(timeLayout)
	}
	if cancelled {
		return regular
	}
	return ""
}

// collectPickupEntries derives the cohort from effective pickup times
// (weekly schedule + exceptions) and presence from attendance records on the
// selected date. Closed attendance rows still count for historical lists.
func (s *service) collectPickupEntries(ctx context.Context, params Params, buckets pickupBucketConfig, result *Result) ([]mergedEntry, error) {
	// buckets is the single per-build cutoff snapshot resolved in BuildList, shared
	// with listLabel so the header and the rows agree on the threshold (#1565
	// review pass 2).
	students, err := s.listEligibleStudents(ctx, params.Date)
	if err != nil {
		return nil, err
	}
	studentIDs := make([]int64, 0, len(students))
	for _, student := range students {
		studentIDs = append(studentIDs, student.ID)
	}

	pickupTimes := map[int64]*scheduleSvc.EffectivePickupTime{}
	if len(studentIDs) > 0 {
		pickupTimes, err = s.pickupService.GetBulkEffectivePickupTimesForDate(ctx, studentIDs, params.Date)
		if err != nil {
			return nil, err
		}
	}

	// A reconciliation on today has only accrued the presence evidence up to
	// "now": a child whose effective arrival time is still in the future has not
	// been expected yet, so an empty attendance row is not a no-show. Load the
	// arrival times to defer those children below rather than print them as
	// "Fehlt" — the pickup analogue of collectSlotEntries excluding a
	// not-yet-started slot from the merge (#1565 review pass 1). Only needed for
	// the Abgleich (Plan lists the expected children, Ist only the present ones).
	arrivalTimes := map[int64]*scheduleSvc.EffectiveArrivalTime{}
	if params.Source == SourceReconciliation && len(studentIDs) > 0 {
		arrivalTimes, err = s.arrivalService.GetBulkEffectiveArrivalTimesForDate(ctx, studentIDs, params.Date)
		if err != nil {
			return nil, fmt.Errorf("load effective arrival times: %w", err)
		}
	}

	attendanceRows, err := s.attendanceRepo.FindForDate(ctx, params.Date)
	if err != nil {
		return nil, err
	}
	presentSet := make(map[int64]struct{}, len(attendanceRows))
	for _, row := range attendanceRows {
		presentSet[row.StudentID] = struct{}{}
	}

	// Broad day statuses (sick / excused / class trip) are the registered
	// sign-offs for pickup cohorts — there is no instance_students row to
	// carry them. Without this evidence a sick child with a recurring pickup
	// time would show as unexplained "Fehlt" in the Abgleich and could
	// trigger an unnecessary missing-child response. Include the scheduler's
	// end-of-day archived rows, not just active ones: after the configured
	// status-clear time (18:00 by default) the sick/excused flag is archived
	// with cleared_at set and source = end_of_day, but it is still valid
	// all-day sign-off evidence for this date (#1565 review pass 1).
	statusDays, err := s.statusDayRepo.FindSignedOffByStudentIDsAndDate(ctx, studentIDs, params.Date)
	if err != nil {
		return nil, fmt.Errorf("load student status days: %w", err)
	}
	statusByStudent := make(map[int64]string, len(statusDays))
	for _, day := range statusDays {
		statusByStudent[day.StudentID] = day.Status
	}

	// A cancelled care day ("Kommt heute nicht") is also a registered absence,
	// but it lives in the arrival/pickup exceptions, not in a status day. Its
	// effective pickup may be nil (a timeless pickup exception) or the regular
	// time (a timeless arrival exception) — so resolve the verdict and keep
	// the regular weekly bucket to place such children into a cohort.
	careDays, err := s.careDayService.ResolveForDate(ctx, studentIDs, params.Date)
	if err != nil {
		return nil, fmt.Errorf("resolve care days: %w", err)
	}
	regularRows, err := s.pickupScheduleRepo.FindByStudentIDsAndWeekday(ctx, studentIDs, int(params.Date.Weekday()))
	if err != nil {
		return nil, fmt.Errorf("load regular pickup schedules: %w", err)
	}
	regularBucket := make(map[int64]string, len(regularRows))
	for _, row := range regularRows {
		regularBucket[row.StudentID] = row.PickupTime.Format(timeLayout)
	}

	slotLabel := result.ListLabel
	entries := []mergedEntry{}
	cohort := map[int64]struct{}{}
	for _, student := range students {
		cancelled := careDays[student.ID] == scheduleSvc.CareDayCancelled
		// No time from either source means the child is not in care on this
		// date — like a not_scheduled slot row, they belong to no cohort.
		hhmm := cohortPickupTime(cancelled, pickupTimes[student.ID], regularBucket[student.ID])
		if hhmm == "" || !pickupMatchesCohort(params.PickupCohort, hhmm, buckets) {
			continue
		}
		cohort[student.ID] = struct{}{}
		_, present := presentSet[student.ID]
		_, hasStatusDay := statusByStudent[student.ID]
		// Defer a not-yet-arrived child on today's Abgleich: with no check-in yet
		// and their effective arrival time still ahead, an empty attendance row is
		// not a no-show, so emitting a planned row here would false-report the
		// child as "Fehlt" and inflate the missing counter before they were ever
		// expected (#1565 review pass 1). Registered absences (a cancelled care day
		// or a sick/excused/trip status day) are sign-offs valid all day and still
		// render "Abgemeldet", so only the would-be-"Fehlt" case defers. The child
		// stays in `cohort` (already recorded above) so the unplanned sweep does not
		// re-add them. Past dates are refused upstream; future dates too — so this
		// only ever fires on today.
		if params.Source == SourceReconciliation && !present && !cancelled && !hasStatusDay &&
			s.beforeEffectiveArrival(params.Date, arrivalTimes[student.ID]) {
			continue
		}
		if cancelled && present {
			// The care day was signed off ("Kommt heute nicht") but the child
			// attended anyway. That is an unplanned presence, not a
			// planned-and-present child — mirror the slot-list path, which leaves
			// the same cancelled-but-attended case unseen so it surfaces as
			// "Ungeplant anwesend". Classifying it as Planned here would label the
			// child "Anwesend" and disagree with collectSlotEntries and its
			// counters (#1565 review). Emit a present-only entry; the child stays
			// in `cohort` so the unplanned sweep below does not re-add them.
			entries = append(entries, mergedEntry{
				StudentID:  student.ID,
				SlotLabel:  slotLabel,
				PickupTime: hhmm,
				Present:    true,
			})
			continue
		}
		entry := mergedEntry{
			StudentID:  student.ID,
			SlotLabel:  slotLabel,
			PickupTime: hhmm,
			Planned:    true,
			Present:    present,
		}
		if !present {
			// A signed-off absence carries absence evidence (sick / excused /
			// class trip, or a cancelled care day below), the shape
			// signedOffAbsence reads to render "Abgemeldet" instead of "Fehlt".
			if status, ok := statusByStudent[student.ID]; ok {
				statusCopy := status
				entry.PlannedStatus = scheduleModel.AttendanceStatusAbsent
				entry.PlannedSubstatus = &statusCopy
			} else if cancelled {
				cancelledSubstatus := string(scheduleSvc.CareDayCancelled)
				entry.PlannedStatus = scheduleModel.AttendanceStatusAbsent
				entry.PlannedSubstatus = &cancelledSubstatus
			}
		}
		entries = append(entries, entry)
	}

	// Children who attended on this date but whose effective pickup time puts
	// them outside this cohort are "unplanned" — but only meaningful in the
	// reconciliation merge. A pickup list has no physical room, so in the pure
	// Ist view "every other present child in the school" is noise, not a member
	// of this list. Plan/Ist therefore stay scoped to the cohort.
	if params.Source == SourceReconciliation {
		for id := range presentSet {
			if _, ok := cohort[id]; ok {
				continue
			}
			pickupLabel := ""
			if pickup := pickupTimes[id]; pickup != nil && pickup.PickupTime != nil {
				pickupLabel = pickup.PickupTime.Format(timeLayout)
			}
			entries = append(entries, mergedEntry{
				StudentID:  id,
				SlotLabel:  slotLabel,
				PickupTime: pickupLabel,
				Present:    true,
			})
		}
	}
	return entries, nil
}

func pickupMatchesCohort(cohort PickupCohort, hhmm string, buckets pickupBucketConfig) bool {
	switch cohort {
	case PickupCohortShortDay:
		return hhmm <= buckets.ShortCutoff
	case PickupCohortLongDay:
		return hhmm > buckets.ShortCutoff && hhmm <= buckets.LongCutoff
	default:
		return false
	}
}

// enrichEntries filters by source/read access, resolves names/classes/groups,
// and maps to display rows.
func (s *service) enrichEntries(ctx context.Context, entries []mergedEntry, source Source, access studentReadAccess, date timezone.Date) ([]Row, error) {
	// matchesSource is the per-source selection predicate. A Plan list selects
	// rows that are planned, an Ist list rows that are present, an Abgleich keeps
	// everything. It is applied once to build `filtered` and re-applied after the
	// enrollment gate below mutates entries, so the two never diverge.
	matchesSource := func(e mergedEntry) bool {
		switch source {
		case SourcePlanned:
			return e.Planned
		case SourceActual:
			return e.Present
		case SourceReconciliation:
			return true
		default:
			return false
		}
	}
	filtered := make([]mergedEntry, 0, len(entries))
	for _, entry := range entries {
		if matchesSource(entry) {
			filtered = append(filtered, entry)
		}
	}
	if len(filtered) == 0 {
		return []Row{}, nil
	}

	studentIDSet := map[int64]struct{}{}
	studentIDs := []int64{}
	for _, entry := range filtered {
		if _, ok := studentIDSet[entry.StudentID]; !ok {
			studentIDSet[entry.StudentID] = struct{}{}
			studentIDs = append(studentIDs, entry.StudentID)
		}
	}
	students, err := s.studentRepo.FindByIDs(ctx, studentIDs)
	if err != nil {
		return nil, err
	}

	// Date-effective enrollment gate: the lifecycle scheduler inactivates a
	// student by flipping users.students.status only — already-materialized
	// schedule.instance_students rows survive. A *planned* row for a child not
	// enrolled on the requested date must be dropped, or it surfaces as an
	// unexplained "Fehlt" (#1565 review). Documented presence is ground truth
	// and always kept: a real visit/attendance record outranks the enrollment
	// interval, so present rows pass regardless — but the enrollment check has
	// established that the roster's *expectation* is invalid, so a present row
	// for a non-enrolled child must shed its planned classification too, or the
	// Abgleich reports it as planned-and-present and inflates the planned
	// counter. Keep the presence, clear the plan, so it reads as "ungeplant
	// anwesend" (#1565 review pass 2).
	today := s.todayDate()
	enrolled := make([]mergedEntry, 0, len(filtered))
	for _, entry := range filtered {
		student := students[entry.StudentID]
		if student == nil {
			continue
		}
		if !eligibleOn(student, date, today) {
			if !entry.Present {
				continue
			}
			entry.Planned = false
			entry.PlannedStatus = ""
			entry.PlannedSubstatus = nil
			// Clearing the plan can drop the entry below the source predicate it
			// originally satisfied. On a Plan list the row only reached `filtered`
			// because it was planned; now de-planned, keeping it would leak an
			// "ungeplant" row into the planned roster — rendered as Geplant yet
			// excluded from the planned counter (a contradictory row). Re-apply the
			// source filter so a de-planned present row survives only where the list
			// actually selects on presence (Ist/Abgleich), not on a Plan list
			// (#1565 review pass 2).
			if !matchesSource(entry) {
				continue
			}
		}
		enrolled = append(enrolled, entry)
	}
	filtered = enrolled
	if len(filtered) == 0 {
		return []Row{}, nil
	}

	readableStudents := make(map[int64]*userModel.Student, len(students))
	personIDs := make([]int64, 0, len(students))
	groupIDSet := map[int64]struct{}{}
	groupIDs := []int64{}
	for _, entry := range filtered {
		student := students[entry.StudentID]
		if student == nil || !access.canReadGroup(student.GroupID) {
			continue
		}
		if _, ok := readableStudents[student.ID]; ok {
			continue
		}
		readableStudents[student.ID] = student
		personIDs = append(personIDs, student.PersonID)
		if student.GroupID != nil {
			if _, ok := groupIDSet[*student.GroupID]; !ok {
				groupIDSet[*student.GroupID] = struct{}{}
				groupIDs = append(groupIDs, *student.GroupID)
			}
		}
	}
	if len(readableStudents) == 0 {
		return []Row{}, nil
	}
	persons, err := s.personRepo.FindByIDs(ctx, personIDs)
	if err != nil {
		return nil, err
	}
	groups := map[int64]*educationModel.Group{}
	if len(groupIDs) > 0 {
		groups, err = s.educationGroupRepo.FindByIDs(ctx, groupIDs)
		if err != nil {
			return nil, err
		}
	}

	rows := make([]Row, 0, len(filtered))
	for _, entry := range filtered {
		student := readableStudents[entry.StudentID]
		if student == nil {
			continue
		}
		person := persons[student.PersonID]
		if person == nil {
			continue
		}
		groupName := ""
		if student.GroupID != nil {
			if group := groups[*student.GroupID]; group != nil {
				groupName = group.Name
			}
		}
		rows = append(rows, Row{
			StudentID:   entry.StudentID,
			Name:        person.GetFullName(),
			SchoolClass: student.SchoolClass,
			GroupName:   groupName,
			GroupID:     student.GroupID,
			InstanceID:  entry.InstanceID,
			Slot:        entry.SlotLabel,
			RoomName:    entry.RoomName,
			PickupTime:  entry.PickupTime,
			Planned:     entry.Planned,
			Present:     entry.Present,
			Unplanned:   entry.Present && !entry.Planned,
			Excused:     entry.Planned && !entry.Present && signedOffAbsence(entry),
			StatusLabel: statusLabel(entry, source),
		})
	}
	return rows, nil
}

// signedOffAbsence reports whether an absent row carries explicit evidence
// that the absence was registered — sick, excused, a field/class trip, or a
// cancelled care day. A merely non-empty substatus is NOT proof: a `late`
// context (a valid state that can linger after a status change) or `other`
// says nothing about a genuine sign-off. Treating those as "Abgemeldet" would
// move a genuinely missing child out of the Missing counter and suppress a
// safety-relevant missing-child signal, so anything short of registered
// absence evidence falls through to "Fehlt".
func signedOffAbsence(entry mergedEntry) bool {
	if entry.PlannedStatus != scheduleModel.AttendanceStatusAbsent || entry.PlannedSubstatus == nil {
		return false
	}
	switch *entry.PlannedSubstatus {
	case scheduleModel.AttendanceSubstatusSick, // instance substatus / status day
		scheduleModel.AttendanceSubstatusExcused,   // instance substatus / status day
		scheduleModel.AttendanceSubstatusFieldTrip, // instance substatus
		activeModel.StudentStatusDayClassTrip,      // status day
		string(scheduleSvc.CareDayCancelled):       // cancelled care day
		return true
	default:
		return false
	}
}

// voidPlanRegisteredAbsence reports whether a deferred (not-yet-started) slot's
// roster row is a registered all-day sign-off that must survive the void plan as
// "Abgemeldet" rather than be suppressed like an unexplained not-yet-due no-show.
// It returns the substatus to stamp on the retained entry. Two shapes qualify,
// exactly the ones the normal merge and the pickup cohort render as "Abgemeldet":
//
//   - A status-day sick/excused/class-trip stamp on the row itself: ApplyStatusDay
//     writes status=absent plus the substatus (class trip → "field_trip") and a
//     student_status_day_id, so the evidence rides the row even before the slot
//     starts.
//   - A cancelled care day ("Kommt heute nicht"), which lives in the timeless
//     arrival/pickup exceptions rather than on the row. A manual override
//     (ManualStatusAt) still wins and is excluded here, exactly as the normal
//     merge's cancelled-care-day branch treats it.
//
// Anything else — an expected no-show, a lifecycle absent without a sign-off
// substatus, a not_scheduled non-booking — is not a registered absence and stays
// suppressed so the deferred slot never prints a not-yet-due child as "Fehlt".
func voidPlanRegisteredAbsence(row *scheduleModel.InstanceStudent, careDay scheduleSvc.CareDayStatus) (string, bool) {
	if row == nil {
		return "", false
	}
	// The care plan never booked this child into the OGS on this weekday
	// (CareDayNotScheduled means a plan exists but does not cover today), yet a
	// broad sick/excused/class-trip status day can still stamp the roster row
	// absent+substatus for a day the child was never expected. On a deferred
	// (not-yet-started) slot that is not a registered absence — the child does
	// not belong on the list at all — so drop it here exactly as the normal
	// merge does at its CareDayNotScheduled branch (service.go ~1207); keeping it
	// would inflate the planned and excused lists with unbooked children. A
	// manual per-slot override (ManualStatusAt) is a human decision that wins and
	// is deliberately excluded from this guard, matching that same merge branch.
	// A child with no plan at all resolves to CareDayUnknown (not NotScheduled),
	// so a genuine sign-off on an unplanned child still survives below (#1565
	// review pass 10).
	if row.ManualStatusAt == nil && careDay == scheduleSvc.CareDayNotScheduled {
		return "", false
	}
	if row.Status == scheduleModel.AttendanceStatusAbsent && row.Substatus != nil {
		switch *row.Substatus {
		case scheduleModel.AttendanceSubstatusSick,
			scheduleModel.AttendanceSubstatusExcused,
			scheduleModel.AttendanceSubstatusFieldTrip:
			return *row.Substatus, true
		}
	}
	if row.ManualStatusAt == nil && careDay == scheduleSvc.CareDayCancelled {
		return string(scheduleSvc.CareDayCancelled), true
	}
	return "", false
}

func statusLabel(entry mergedEntry, source Source) string {
	switch source {
	case SourcePlanned:
		return "Geplant"
	case SourceActual:
		return "Anwesend"
	case SourceReconciliation:
		switch {
		case entry.Present && !entry.Planned:
			return "Ungeplant anwesend"
		case entry.Present:
			return "Anwesend"
		case signedOffAbsence(entry):
			return "Abgemeldet (entschuldigt)"
		default:
			return "Fehlt"
		}
	default:
		return ""
	}
}

func countRows(rows []Row, source Source) Counters {
	// A slot list may include several offerings and the same child can hold one
	// row in each, but these counters are child headcounts — the export subtitle
	// labels them "geplante Kinder" / "anwesende Kinder", not assignment entries.
	// Deduplicate by StudentID per category so a child assigned to two offerings
	// counts once, never twice (#1565 review pass 1 P2). Categories stay
	// independent (as the per-row logic already was): a child present in one slot
	// and missing in another appears in both headcounts, matching the row logic.
	planned := map[int64]struct{}{}
	present := map[int64]struct{}{}
	missing := map[int64]struct{}{}
	excused := map[int64]struct{}{}
	unplanned := map[int64]struct{}{}
	for _, row := range rows {
		if row.Planned {
			planned[row.StudentID] = struct{}{}
		}
		if row.Present {
			present[row.StudentID] = struct{}{}
		}
		switch {
		case row.Planned && !row.Present && row.Excused:
			// Registered sign-off: a justified absence, not an unexplained gap.
			excused[row.StudentID] = struct{}{}
		case row.Planned && !row.Present:
			missing[row.StudentID] = struct{}{}
		}
		if row.Unplanned {
			unplanned[row.StudentID] = struct{}{}
		}
	}
	counters := Counters{
		Planned:   len(planned),
		Present:   len(present),
		Missing:   len(missing),
		Excused:   len(excused),
		Unplanned: len(unplanned),
	}
	// Planned/actual previews intentionally show only their own counter;
	// the merge counters are a reconciliation concept.
	switch source {
	case SourcePlanned:
		return Counters{Planned: counters.Planned}
	case SourceActual:
		return Counters{Present: counters.Present}
	default:
		return counters
	}
}

func (s *service) RenderList(ctx context.Context, params Params, format listexport.Format) (listexport.File, error) {
	result, err := s.BuildList(ctx, params)
	if err != nil {
		return listexport.File{}, err
	}

	// Atomic drift guard: the client echoes the content signature of the preview
	// it verified. If this fresh build no longer matches, live data changed
	// between the client's verification and this render — refuse rather than hand
	// out a file that differs from the approved preview (#1565 review pass 2). An
	// empty ExpectedSignature (older client or a direct render) skips the guard.
	if params.ExpectedSignature != "" && result.Signature != params.ExpectedSignature {
		return listexport.File{}, ErrListDrifted
	}

	pickupBased := params.Target == TargetPickupCohort
	columns := []listexport.Column{
		{ID: listexport.ColumnName, Label: "Name"},
		{ID: listexport.ColumnSchoolClass, Label: "Klasse"},
		{ID: listexport.ColumnGroup, Label: "Gruppe"},
		{ID: listexport.ColumnSlot, Label: "Angebot / Zeitraum"},
	}
	if pickupBased {
		columns = append(columns, listexport.Column{ID: listexport.ColumnPlannedPickup, Label: "Geplante Abholung"})
	}
	columns = append(columns, listexport.Column{ID: listexport.ColumnPresenceStatus, Label: "Status"})

	// The PDF renderer treats a Row with GroupTitle as a pure section marker
	// and drops its Values (pdf_design.go paginate). Emit one marker row per
	// section followed by plain value rows — the same shape the grouped
	// student exports produce. Rows arrive sorted by GroupTitle (sortRows),
	// so sections are contiguous.
	docRows := make([]listexport.Row, 0, len(result.Rows))
	currentGroup := ""
	for _, row := range result.Rows {
		if row.GroupTitle != "" && row.GroupTitle != currentGroup {
			currentGroup = row.GroupTitle
			docRows = append(docRows, listexport.Row{GroupTitle: row.GroupTitle})
		}
		values := map[listexport.ColumnID]string{
			listexport.ColumnName:           row.Name,
			listexport.ColumnSchoolClass:    row.SchoolClass,
			listexport.ColumnGroup:          row.GroupName,
			listexport.ColumnSlot:           row.Slot,
			listexport.ColumnPresenceStatus: row.StatusLabel,
		}
		if pickupBased {
			values[listexport.ColumnPlannedPickup] = row.PickupTime
		}
		docRows = append(docRows, listexport.Row{Values: values})
	}

	doc := listexport.Document{
		Title:       documentTitle(result),
		Subtitle:    subtitle(result),
		GeneratedAt: time.Now(),
		Filters:     exportFilters(params, result),
		Columns:     columns,
		Rows:        docRows,
		Footer:      confidentialityNote,
	}

	filename := exportFilename(params, result)
	return s.listExport.Render(doc, format, filename)
}

func documentTitle(result *Result) string {
	return fmt.Sprintf("Tagesliste %s – %s", result.Source.Label(), result.ListLabel)
}

func exportFilename(params Params, result *Result) string {
	return fmt.Sprintf("Tagesliste %s %s %s", params.Source.Label(), result.ListLabel, params.Date.String())
}

// exportFilters builds the header lines stamped on the printed export.
func exportFilters(params Params, result *Result) []string {
	filters := []string{
		"Datum: " + params.Date.Format("02.01.2006"),
		"Datenbasis: " + params.Source.Label() + " – " + result.Provenance,
		"Enthalten: " + includedSummary(params, result),
	}
	if names := selectedGroupNames(params, result); len(names) > 0 {
		filters = append(filters, "Gruppen: "+strings.Join(names, ", "))
	}
	if len(params.Classes) > 0 {
		filters = append(filters, "Klassen: "+strings.Join(params.Classes, ", "))
	}
	if params.GroupBy != GroupByNone {
		filters = append(filters, "Gruppiert nach: "+params.GroupBy.Label())
	}
	return filters
}

// selectedGroupNames maps the selected education-group IDs to their display
// names via the options the build already resolved, so a filtered export
// discloses the restriction instead of looking like the full cohort.
// result.Groups is derived from the day's rows (availableOptions(allRows)), so a
// saved selection that references a group with no rows for this date/source is
// absent from it. Such an ID is disclosed via an explicit "Gruppe #<id>" marker
// rather than dropped — otherwise an empty or partial confidential export would
// print as if it were unfiltered (#1565 review pass 2).
func selectedGroupNames(params Params, result *Result) []string {
	if len(params.GroupIDs) == 0 {
		return nil
	}
	byID := make(map[int64]string, len(result.Groups))
	for _, group := range result.Groups {
		byID[group.ID] = group.Name
	}
	names := make([]string, 0, len(params.GroupIDs))
	for _, id := range params.GroupIDs {
		if name := byID[id]; name != "" {
			names = append(names, name)
			continue
		}
		names = append(names, fmt.Sprintf("Gruppe #%d", id))
	}
	sort.Strings(names)
	return names
}

func includedSummary(params Params, result *Result) string {
	if params.Target == TargetPickupCohort {
		return result.ListLabel
	}
	count := summarySlotCount(params, result)
	if params.ListKind != ListKindNone {
		return fmt.Sprintf("%s (%d %s)", result.ListLabel, count, plural(count, "Termin", "Termine"))
	}
	return fmt.Sprintf("%d %s", count, plural(count, "Angebot", "Angebote"))
}

// summarySlotCount is the number of slots the export actually contains. When an
// explicit instance selection is present (InstanceIDsSet, even for a classified
// list_kind) the count is restricted to the selected slots — so a filtered
// export reports its real subset instead of claiming the whole cohort (#1565
// review). result.Slots is always the full set of matching slots, so the
// selection has to be re-applied here. A cancelled slot is dropped unless it ran
// briefly before being called off and retained present rows (retainedCancelledSlots);
// deferred slots (a not-yet-started reconciliation occurrence excluded from the
// merge in collectSlotEntries) are dropped in both branches: they yield no export
// rows, so counting them would let the header claim Termine/Angebote the export
// does not contain (#1565).
func summarySlotCount(params Params, result *Result) int {
	all := !params.InstanceIDsSet && len(params.InstanceIDs) == 0
	selected := int64Set(params.InstanceIDs)
	count := 0
	for _, slot := range result.Slots {
		if slot.Status == string(scheduleModel.InstanceStatusCancelled) {
			// A cancelled slot contributes no rows unless it ran briefly before
			// being called off and retained present children — only then does it
			// count as a contained Termin (see collectSlotEntries).
			if _, retained := result.retainedCancelledSlots[slot.InstanceID]; !retained {
				continue
			}
		}
		if _, deferred := result.deferredSlots[slot.InstanceID]; deferred {
			continue
		}
		if all || selected[slot.InstanceID] {
			count++
		}
	}
	return count
}

func subtitle(result *Result) string {
	switch result.Source {
	case SourcePlanned:
		return fmt.Sprintf("%d geplante Kinder", result.Counters.Planned)
	case SourceActual:
		return fmt.Sprintf("%d anwesende Kinder", result.Counters.Present)
	default:
		return fmt.Sprintf(
			"%d geplant, %d anwesend, %d fehlend, %d abgemeldet, %d ungeplant",
			result.Counters.Planned,
			result.Counters.Present,
			result.Counters.Missing,
			result.Counters.Excused,
			result.Counters.Unplanned,
		)
	}
}

func plural(count int, singular string, pluralValue string) string {
	if count == 1 {
		return singular
	}
	return pluralValue
}
