package display

import (
	"cmp"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	displayModels "github.com/moto-nrw/project-phoenix/models/display"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/services/facilities"
	"github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
)

const maxDisplayNameLength = 100

// Dependencies wires the display service. The aggregate deliberately reuses
// existing repositories/services instead of new queries (issue #1325).
type Dependencies struct {
	DisplayRepo       displayModels.Repository
	SchoolRepo        platformModels.SchoolRepository
	Facilities        facilities.Service
	ActiveGroupRepo   activeModels.GroupRepository
	VisitRepo         activeModels.VisitRepository
	ActivityGroupRepo activitiesModels.GroupRepository
	InstanceRepo      scheduleModels.ActivityInstanceRepository
	AttendanceRepo    activeModels.AttendanceRepository
	PickupSchedule    schedule.PickupScheduleService
	// SettingsService resolves the display.enabled opt-in toggle. The feature
	// defaults off; a school must explicitly enable it before the public
	// dashboard endpoint serves data (issue #1325 opt-in follow-up).
	SettingsService configSvc.SettingsService
	DB              *bun.DB
	Logger          *slog.Logger
	Now             func() time.Time
}

type service struct {
	Dependencies
}

// NewService creates the display service.
func NewService(deps Dependencies) Service {
	if deps.Now == nil {
		deps.Now = timezone.Now
	}
	return &service{Dependencies: deps}
}

func (s *service) getLogger() *slog.Logger {
	return cmp.Or(s.Logger, slog.Default())
}

// newDisplayToken returns 32 random bytes as URL-safe base64 — same shape as
// the enrollment status token (services/enrollment/request_service.go).
func newDisplayToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate display token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// displayTokenHash returns the SHA-256 hex digest of a raw token. Only the
// hash is persisted; a database leak exposes no usable dashboard URLs.
func displayTokenHash(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

func validateDisplayName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("%w: display name is required", displayModels.ErrInvalidInput)
	}
	if len(name) > maxDisplayNameLength {
		return "", fmt.Errorf("%w: display name cannot exceed %d characters", displayModels.ErrInvalidInput, maxDisplayNameLength)
	}
	return name, nil
}

// isNotFound reports whether err means "no such display" — either the repo's
// own sentinel (FindByTokenHash) or a bare sql.ErrNoRows wrapped by the
// generic base repository (FindByID). Anything else is an infrastructure
// failure and must NOT be presented as a 404.
func isNotFound(err error) bool {
	return errors.Is(err, displayModels.ErrNotFound) || errors.Is(err, sql.ErrNoRows)
}

func (s *service) List(ctx context.Context) ([]*displayModels.Display, error) {
	return s.DisplayRepo.List(ctx, nil)
}

func (s *service) Create(ctx context.Context, name string) (*displayModels.Display, string, error) {
	name, err := validateDisplayName(name)
	if err != nil {
		return nil, "", err
	}

	rawToken, err := newDisplayToken()
	if err != nil {
		return nil, "", err
	}

	d := &displayModels.Display{
		TokenHash: displayTokenHash(rawToken),
	}
	d.Name = name
	d.IsActive = true

	if err := s.DisplayRepo.Create(ctx, d); err != nil {
		return nil, "", fmt.Errorf("failed to create display: %w", err)
	}

	s.getLogger().Info("display created",
		"display_id", d.ID,
		"tenant_id", d.TenantID,
	)
	return d, rawToken, nil
}

func (s *service) Update(ctx context.Context, id int64, name *string, isActive *bool) (*displayModels.Display, error) {
	d, err := s.DisplayRepo.FindByID(ctx, id)
	if err != nil {
		if isNotFound(err) {
			return nil, displayModels.ErrNotFound
		}
		return nil, fmt.Errorf("failed to load display: %w", err)
	}

	columns := make([]string, 0, 2)
	if name != nil {
		validated, err := validateDisplayName(*name)
		if err != nil {
			return nil, err
		}
		d.Name = validated
		columns = append(columns, "name")
	}
	if isActive != nil {
		d.IsActive = *isActive
		columns = append(columns, "is_active")
	}
	if len(columns) == 0 {
		return d, nil
	}
	d.UpdatedAt = time.Now()
	columns = append(columns, "updated_at")

	updated, err := s.DisplayRepo.UpdateColumns(ctx, d, columns...)
	if err != nil {
		return nil, fmt.Errorf("failed to update display: %w", err)
	}
	if updated == 0 {
		// Row vanished between FindByID and the update (concurrent delete).
		return nil, displayModels.ErrNotFound
	}

	s.getLogger().Info("display updated",
		"display_id", d.ID,
		"tenant_id", d.TenantID,
		"columns", strings.Join(columns, ","),
	)
	return d, nil
}

func (s *service) Regenerate(ctx context.Context, id int64) (string, error) {
	d, err := s.DisplayRepo.FindByID(ctx, id)
	if err != nil {
		if isNotFound(err) {
			return "", displayModels.ErrNotFound
		}
		return "", fmt.Errorf("failed to load display: %w", err)
	}

	rawToken, err := newDisplayToken()
	if err != nil {
		return "", err
	}
	d.TokenHash = displayTokenHash(rawToken)
	d.UpdatedAt = time.Now()

	updated, err := s.DisplayRepo.UpdateColumns(ctx, d, "token_hash", "updated_at")
	if err != nil {
		return "", fmt.Errorf("failed to regenerate display token: %w", err)
	}
	if updated == 0 {
		// Never hand out a token that was not persisted: the display was
		// deleted between FindByID and the update.
		return "", displayModels.ErrNotFound
	}

	s.getLogger().Info("display token regenerated",
		"display_id", d.ID,
		"tenant_id", d.TenantID,
	)
	return rawToken, nil
}

func (s *service) Delete(ctx context.Context, id int64) error {
	d, err := s.DisplayRepo.FindByID(ctx, id)
	if err != nil {
		if isNotFound(err) {
			return displayModels.ErrNotFound
		}
		return fmt.Errorf("failed to load display: %w", err)
	}
	if err := s.DisplayRepo.Delete(ctx, d.ID); err != nil {
		return fmt.Errorf("failed to delete display: %w", err)
	}
	s.getLogger().Info("display deleted",
		"display_id", d.ID,
		"tenant_id", d.TenantID,
	)
	return nil
}

func (s *service) Dashboard(ctx context.Context, rawToken string) (*DashboardPayload, error) {
	tokenHash := displayTokenHash(rawToken)

	// Step 1: resolve token → display + school under the BYPASSRLS admin
	// role. The token is the only auth signal; no tenant is known yet.
	var d *displayModels.Display
	var school *platformModels.School
	err := tenant.WithAdminTx(ctx, s.DB, func(ctx context.Context, _ bun.Tx) error {
		found, err := s.DisplayRepo.FindByTokenHash(ctx, tokenHash)
		if err != nil {
			return err
		}
		d = found
		school, err = s.SchoolRepo.FindByID(ctx, d.TenantID)
		return err
	})
	if err != nil {
		if isNotFound(err) {
			return nil, displayModels.ErrNotFound
		}
		return nil, fmt.Errorf("failed to resolve display token: %w", err)
	}
	// A disabled or offboarded school must not keep serving live tenant data
	// to already-open displays. 404 (not "inactive") — the link is dead and
	// the response must not reveal the school's state.
	if school.IsDeleted() || !school.Active {
		return nil, displayModels.ErrNotFound
	}
	// The feature is opt-in and defaults off. A token for a school that never
	// enabled (or has since disabled) display.enabled must behave exactly
	// like an unknown token — 404, not a distinct error that would reveal
	// the feature exists but is off.
	enabled, err := s.SettingsService.ResolveBoolForTenant(ctx, d.TenantID, configModel.KeyDisplayEnabled)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve display.enabled: %w", err)
	}
	if !enabled {
		return nil, displayModels.ErrNotFound
	}
	if !d.IsActive {
		return nil, displayModels.ErrInactive
	}

	// Step 2: aggregate inside a tenant-scoped transaction so every query
	// runs under RLS for exactly this display's tenant.
	var payload *DashboardPayload
	err = tenant.WithTenantTx(ctx, s.DB, d.TenantID, func(ctx context.Context, _ bun.Tx) error {
		var aggErr error
		payload, aggErr = s.aggregate(ctx)
		return aggErr
	})
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate dashboard data: %w", err)
	}

	payload.Status = "active"
	payload.SchoolName = school.Name
	payload.DisplayName = d.Name

	s.getLogger().Info("display dashboard served",
		"display_id", d.ID,
		"tenant_id", d.TenantID,
		"students_present", payload.Totals.StudentsPresent,
	)
	return payload, nil
}

// aggregate builds the tenant-scoped dashboard body. Must run inside
// tenant.WithTenantTx.
func (s *service) aggregate(ctx context.Context) (*DashboardPayload, error) {
	now := s.Now()
	today := timezone.DateFromTime(now)

	rooms, err := s.Facilities.ListRooms(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("rooms: %w", err)
	}

	activeGroups, err := s.ActiveGroupRepo.FindActiveGroups(ctx)
	if err != nil {
		return nil, fmt.Errorf("active groups: %w", err)
	}

	visits, err := s.VisitRepo.FindActiveVisits(ctx)
	if err != nil {
		return nil, fmt.Errorf("active visits: %w", err)
	}

	templates, err := s.ActivityGroupRepo.ListWithCategory(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("activity templates: %w", err)
	}

	instances, err := s.InstanceRepo.FindByTenantAndDate(ctx, today)
	if err != nil {
		return nil, fmt.Errorf("activity instances: %w", err)
	}

	attendance, err := s.AttendanceRepo.FindForDate(ctx, today)
	if err != nil {
		return nil, fmt.Errorf("attendance: %w", err)
	}

	roomNames := make(map[int64]string, len(rooms))
	occupancy := make([]RoomOccupancy, 0, len(rooms))
	roomsOccupied := 0
	for _, room := range rooms {
		roomNames[room.ID] = room.Name
		if room.IsOccupied {
			roomsOccupied++
		}
		occupancy = append(occupancy, RoomOccupancy{
			Name:         room.Name,
			GroupName:    room.GroupName,
			CategoryName: room.CategoryName,
			StudentCount: room.StudentCount,
			Capacity:     room.Capacity,
			IsOccupied:   room.IsOccupied,
		})
	}

	running := buildRunningActivities(activeGroups, visits, templates, instances, roomNames)
	upcoming := buildUpcomingActivities(instances, templates, roomNames, now)
	pickups, present := s.buildPickupBuckets(ctx, attendance, today, now)

	return &DashboardPayload{
		ServerTime:         now.Format(time.RFC3339),
		Date:               today.String(),
		RoomOccupancy:      occupancy,
		RunningActivities:  running,
		UpcomingActivities: upcoming,
		PickupTimes:        pickups,
		Totals: DashboardTotals{
			StudentsPresent:   present,
			RoomsOccupied:     roomsOccupied,
			ActivitiesRunning: len(running),
		},
	}, nil
}

// buildRunningActivities maps currently active sessions to display rows.
// Template-backed sessions take name/category/capacity from the template;
// spontaneous sessions fall back to today's linked activity instance title.
func buildRunningActivities(
	activeGroups []*activeModels.Group,
	visits []*activeModels.Visit,
	templates []*activitiesModels.Group,
	instances []*scheduleModels.ActivityInstance,
	roomNames map[int64]string,
) []RunningActivity {
	participants := make(map[int64]map[int64]struct{})
	for _, v := range visits {
		if v.ExitTime != nil {
			continue
		}
		if participants[v.ActiveGroupID] == nil {
			participants[v.ActiveGroupID] = make(map[int64]struct{})
		}
		participants[v.ActiveGroupID][v.StudentID] = struct{}{}
	}

	templatesByID := make(map[int64]*activitiesModels.Group, len(templates))
	for _, t := range templates {
		templatesByID[t.ID] = t
	}

	instanceByActiveGroup := make(map[int64]*scheduleModels.ActivityInstance, len(instances))
	for _, inst := range instances {
		if inst.ActiveGroupID != nil {
			instanceByActiveGroup[*inst.ActiveGroupID] = inst
		}
	}

	running := make([]RunningActivity, 0, len(activeGroups))
	for _, g := range activeGroups {
		if !g.IsActive() {
			continue
		}

		name := "Spontane Aktivität"
		category := "Sonstiges"
		var maxCapacity *int
		if g.GroupID != nil {
			tmpl, ok := templatesByID[*g.GroupID]
			if !ok {
				continue // template-backed session whose template vanished — nothing to display
			}
			name = tmpl.Name
			if tmpl.Category != nil {
				category = tmpl.Category.Name
			}
			maxCapacity = tmpl.ParticipantLimit()
		} else if inst, ok := instanceByActiveGroup[g.ID]; ok {
			name = inst.Title
		}

		running = append(running, RunningActivity{
			ID:           strconv.FormatInt(g.ID, 10),
			Name:         name,
			Category:     category,
			RoomName:     roomNames[g.RoomID],
			Participants: len(participants[g.ID]),
			MaxCapacity:  maxCapacity,
		})
	}
	return running
}

// buildUpcomingActivities lists today's still-planned instances that start
// later than now. activity_instances.start_time is a TIME column (wall
// clock), so the comparison uses timezone.NormalizeWallClock on both sides.
func buildUpcomingActivities(
	instances []*scheduleModels.ActivityInstance,
	templates []*activitiesModels.Group,
	roomNames map[int64]string,
	now time.Time,
) []UpcomingActivity {
	templatesByID := make(map[int64]*activitiesModels.Group, len(templates))
	for _, t := range templates {
		templatesByID[t.ID] = t
	}

	nowWall := timezone.NormalizeWallClock(now.In(timezone.Berlin))
	upcoming := make([]UpcomingActivity, 0)
	for _, inst := range instances {
		if inst.Status != scheduleModels.InstanceStatusPlanned {
			continue
		}
		if timezone.NormalizeWallClock(inst.StartTime).Before(nowWall) {
			continue
		}

		category := "Sonstiges"
		if inst.ActivityGroupID != nil {
			if tmpl, ok := templatesByID[*inst.ActivityGroupID]; ok && tmpl.Category != nil {
				category = tmpl.Category.Name
			}
		}

		upcoming = append(upcoming, UpcomingActivity{
			ID:        strconv.FormatInt(inst.ID, 10),
			Name:      inst.Title,
			Category:  category,
			StartTime: inst.StartTime.Format("15:04"),
			RoomName:  roomNames[inst.RoomID],
		})
	}
	sort.Slice(upcoming, func(i, j int) bool { return upcoming[i].StartTime < upcoming[j].StartTime })
	return upcoming
}

// buildPickupBuckets aggregates today's effective pickup times of currently
// present students (checked in, not yet checked out) into HH:MM count
// buckets. Returns the buckets plus the present-student total. Only counts
// leave this function — never identities.
func (s *service) buildPickupBuckets(
	ctx context.Context,
	attendance []*activeModels.Attendance,
	today timezone.Date,
	now time.Time,
) ([]PickupBucket, int) {
	presentIDs := make([]int64, 0, len(attendance))
	for _, a := range attendance {
		if a.CheckOutTime == nil {
			presentIDs = append(presentIDs, a.StudentID)
		}
	}

	buckets := make([]PickupBucket, 0)
	if len(presentIDs) == 0 {
		return buckets, 0
	}

	times, err := s.PickupSchedule.GetBulkEffectivePickupTimesForDate(ctx, presentIDs, today)
	if err != nil {
		// Pickup data is a nice-to-have panel; the dashboard must not go
		// dark because of it. Counts stay correct, the panel just empties.
		s.getLogger().Warn("display dashboard pickup aggregation failed",
			"error", err.Error(),
		)
		return buckets, len(presentIDs)
	}

	nowWall := timezone.NormalizeWallClock(now.In(timezone.Berlin))
	counts := make(map[string]int)
	for _, effective := range times {
		if effective == nil || effective.PickupTime == nil {
			continue
		}
		if timezone.NormalizeWallClock(*effective.PickupTime).Before(nowWall) {
			continue
		}
		counts[effective.PickupTime.Format("15:04")]++
	}

	for clock, count := range counts {
		buckets = append(buckets, PickupBucket{Time: clock, Count: count})
	}
	sort.Slice(buckets, func(i, j int) bool { return buckets[i].Time < buckets[j].Time })
	return buckets, len(presentIDs)
}
