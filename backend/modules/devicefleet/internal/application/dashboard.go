package application

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet/internal/ports"
)

// Dashboard is the public info-point aggregate. GDPR contract: it carries
// counts and room/activity metadata ONLY — never student names, student IDs,
// photos, or per-child pickup times.
type Dashboard struct {
	Status             string
	SchoolName         string
	DisplayName        string
	ServerTime         time.Time
	Date               time.Time
	RoomOccupancy      []RoomOccupancy
	RunningActivities  []RunningActivity
	UpcomingActivities []UpcomingActivity
	PickupTimes        []PickupBucket
	StudentsPresent    int
	RoomsOccupied      int
	ActivitiesRunning  int
}

// RoomOccupancy is one room with its live student count.
type RoomOccupancy struct {
	Name         string
	GroupName    *string
	CategoryName *string
	StudentCount int
	Capacity     *int
	IsOccupied   bool
}

// RunningActivity is one currently running activity session.
type RunningActivity struct {
	ID           string
	Name         string
	Category     string
	RoomName     string
	Participants int
	MaxCapacity  *int
}

// UpcomingActivity is one planned activity instance later today.
type UpcomingActivity struct {
	ID        string
	Name      string
	Category  string
	StartTime string
	RoomName  string
}

// PickupBucket aggregates how many present students share a pickup time.
type PickupBucket struct {
	Time  string
	Count int
}

const (
	spontaneousActivityName = "Spontane Aktivität"
	fallbackCategoryName    = "Sonstiges"
)

// ResolveDashboard turns a raw display token into the public dashboard
// aggregate. The token is resolved under the admin scope because no tenant is
// known yet; every fact afterwards is read inside that display's tenant
// transaction.
func (s *Service) ResolveDashboard(ctx context.Context, rawToken string) (Dashboard, error) {
	var dashboard Dashboard
	err := s.run(ctx, "display_dashboard", func(ctx context.Context, stats *domain.OperationStats) error {
		display, school, err := s.resolveDisplayToken(ctx, rawToken, stats)
		if err != nil {
			return err
		}
		// A disabled or offboarded school must not keep serving live tenant
		// data to already-open displays. Not found — not "inactive": the link
		// is dead and the response must not reveal the school's state.
		if school.Deleted || !school.Active {
			return domain.ErrDisplayNotFound
		}
		// The feature is opt-in and defaults off. A token for a school that
		// never enabled it must behave exactly like an unknown token.
		enabled, err := s.deps.Tenants.DisplayEnabled(ctx, display.TenantID)
		if err != nil {
			return fmt.Errorf("failed to resolve display.enabled: %w", err)
		}
		if !enabled {
			return domain.ErrDisplayNotFound
		}
		if !display.IsActive {
			return domain.ErrDisplayInactive
		}
		if err := s.deps.Tx.TenantScope(ctx, display.TenantID, func(tenantCtx context.Context) error {
			built, aggErr := s.aggregate(tenantCtx)
			if aggErr != nil {
				return aggErr
			}
			dashboard = built
			return nil
		}); err != nil {
			return fmt.Errorf("failed to aggregate dashboard data: %w", err)
		}
		dashboard.Status = "active"
		dashboard.SchoolName = school.Name
		dashboard.DisplayName = display.Name
		return nil
	})
	if err != nil {
		return Dashboard{}, err
	}
	return dashboard, nil
}

func (s *Service) resolveDisplayToken(
	ctx context.Context,
	rawToken string,
	stats *domain.OperationStats,
) (domain.Display, domain.School, error) {
	var display domain.Display
	var school domain.School
	err := s.deps.Tx.AdminScope(ctx, func(adminCtx context.Context) error {
		found, exists, queryStats, err := s.deps.Displays.FindByTokenHash(adminCtx, s.deps.Tokens.Hash(rawToken))
		stats.Add(queryStats)
		if err != nil {
			return err
		}
		if !exists {
			return domain.ErrDisplayNotFound
		}
		display = found
		school, err = s.deps.Tenants.School(adminCtx, found.TenantID)
		return err
	})
	if err != nil {
		if errors.Is(err, domain.ErrDisplayNotFound) {
			return domain.Display{}, domain.School{}, domain.ErrDisplayNotFound
		}
		return domain.Display{}, domain.School{}, fmt.Errorf("failed to resolve display token: %w", err)
	}
	return display, school, nil
}

// observePickupFailure records the degraded pickup panel without failing the
// dashboard.
func observePickupFailure(err error) ports.Observation {
	return ports.Observation{Operation: "display_dashboard_pickup", Err: err}
}

// aggregate builds the tenant-scoped dashboard body. Callers run it inside
// the display tenant's transaction.
func (s *Service) aggregate(ctx context.Context) (Dashboard, error) {
	now := s.deps.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	rooms, err := s.deps.Rooms.ListRooms(ctx)
	if err != nil {
		return Dashboard{}, fmt.Errorf("rooms: %w", err)
	}
	sessions, err := s.deps.Dashboard.ListActiveSessions(ctx)
	if err != nil {
		return Dashboard{}, fmt.Errorf("active groups: %w", err)
	}
	visits, err := s.deps.Presence.ListOpenVisits(ctx)
	if err != nil {
		return Dashboard{}, fmt.Errorf("active visits: %w", err)
	}
	templates, err := s.deps.Dashboard.ListActivityTemplates(ctx)
	if err != nil {
		return Dashboard{}, fmt.Errorf("activity templates: %w", err)
	}
	planned, err := s.deps.Dashboard.ListPlannedActivities(ctx, today)
	if err != nil {
		return Dashboard{}, fmt.Errorf("activity instances: %w", err)
	}
	present, err := s.deps.Presence.ListPresentStudents(ctx, today)
	if err != nil {
		return Dashboard{}, fmt.Errorf("attendance: %w", err)
	}

	roomNames := roomNamesByID(rooms)
	occupancy, roomsOccupied := buildRoomOccupancy(rooms, sessions, visits, templates)
	running := buildRunningActivities(sessions, visits, templates, planned, roomNames)
	upcoming := buildUpcomingActivities(planned, templates, roomNames, now)
	pickups := s.buildPickupBuckets(ctx, present, today, now)

	return Dashboard{
		ServerTime: now, Date: today, RoomOccupancy: occupancy, RunningActivities: running,
		UpcomingActivities: upcoming, PickupTimes: pickups, StudentsPresent: len(present),
		RoomsOccupied: roomsOccupied, ActivitiesRunning: len(running),
	}, nil
}

func roomNamesByID(rooms []domain.Room) map[int64]string {
	result := make(map[int64]string, len(rooms))
	for _, room := range rooms {
		result[room.ID] = room.Name
	}
	return result
}

func templatesByID(templates []domain.ActivityTemplate) map[int64]domain.ActivityTemplate {
	result := make(map[int64]domain.ActivityTemplate, len(templates))
	for _, template := range templates {
		result[template.ID] = template
	}
	return result
}

func studentsBySession(visits []domain.PresentVisit) map[int64]map[int64]struct{} {
	result := make(map[int64]map[int64]struct{})
	for _, visit := range visits {
		if result[visit.ActiveGroupID] == nil {
			result[visit.ActiveGroupID] = make(map[int64]struct{})
		}
		result[visit.ActiveGroupID][visit.StudentID] = struct{}{}
	}
	return result
}

func buildRoomOccupancy(
	rooms []domain.Room,
	sessions []domain.ActiveSession,
	visits []domain.PresentVisit,
	templates []domain.ActivityTemplate,
) ([]RoomOccupancy, int) {
	byTemplate := templatesByID(templates)
	sessionsByRoom := make(map[int64][]domain.ActiveSession)
	for _, session := range sessions {
		sessionsByRoom[session.RoomID] = append(sessionsByRoom[session.RoomID], session)
	}
	studentsByGroup := studentsBySession(visits)

	result := make([]RoomOccupancy, 0, len(rooms))
	occupied := 0
	for _, room := range rooms {
		row := roomOccupancy(room, sessionsByRoom[room.ID], studentsByGroup, byTemplate)
		if row.IsOccupied {
			occupied++
		}
		result = append(result, row)
	}
	return result, occupied
}

func roomOccupancy(
	room domain.Room,
	sessions []domain.ActiveSession,
	studentsByGroup map[int64]map[int64]struct{},
	templates map[int64]domain.ActivityTemplate,
) RoomOccupancy {
	names := make([]string, 0, len(sessions))
	categories := make([]string, 0, len(sessions))
	students := make(map[int64]struct{})
	for _, session := range sessions {
		if session.TemplateID != nil {
			if template, ok := templates[*session.TemplateID]; ok {
				names = append(names, template.Name)
				if template.CategoryName != nil {
					categories = append(categories, *template.CategoryName)
				}
			}
		}
		for studentID := range studentsByGroup[session.ID] {
			students[studentID] = struct{}{}
		}
	}
	return RoomOccupancy{
		Name: room.Name, GroupName: joinedNames(names), CategoryName: joinedNames(categories),
		StudentCount: len(students), Capacity: room.Capacity, IsOccupied: len(sessions) > 0,
	}
}

func joinedNames(values []string) *string {
	if len(values) == 0 {
		return nil
	}
	sort.Strings(values)
	values = slices.Compact(values)
	joined := strings.Join(values, ", ")
	return &joined
}

// buildRunningActivities maps currently running sessions to display rows.
// Template-backed sessions take name/category/capacity from the template;
// spontaneous sessions fall back to today's linked activity instance title.
func buildRunningActivities(
	sessions []domain.ActiveSession,
	visits []domain.PresentVisit,
	templates []domain.ActivityTemplate,
	planned []domain.PlannedActivity,
	roomNames map[int64]string,
) []RunningActivity {
	participants := studentsBySession(visits)
	byTemplate := templatesByID(templates)

	plannedBySession := make(map[int64]domain.PlannedActivity, len(planned))
	for _, instance := range planned {
		if instance.ActiveGroupID != nil {
			plannedBySession[*instance.ActiveGroupID] = instance
		}
	}

	running := make([]RunningActivity, 0, len(sessions))
	for _, session := range sessions {
		if !session.Running {
			continue
		}
		name := spontaneousActivityName
		category := fallbackCategoryName
		var maxCapacity *int
		if session.TemplateID != nil {
			template, ok := byTemplate[*session.TemplateID]
			if !ok {
				// Template-backed session whose template vanished — nothing
				// to display.
				continue
			}
			name = template.Name
			if template.CategoryName != nil {
				category = *template.CategoryName
			}
			maxCapacity = template.ParticipantLimit
		} else if instance, ok := plannedBySession[session.ID]; ok {
			name = instance.Title
		}
		running = append(running, RunningActivity{
			ID: strconv.FormatInt(session.ID, 10), Name: name, Category: category,
			RoomName: roomNames[session.RoomID], Participants: len(participants[session.ID]),
			MaxCapacity: maxCapacity,
		})
	}
	return running
}

// buildUpcomingActivities lists today's still-planned instances that start
// later than now. Both sides are wall clocks normalised by the composition
// root, so no time zone conversion happens here.
func buildUpcomingActivities(
	planned []domain.PlannedActivity,
	templates []domain.ActivityTemplate,
	roomNames map[int64]string,
	now time.Time,
) []UpcomingActivity {
	byTemplate := templatesByID(templates)
	nowClock := wallClock(now)

	upcoming := make([]UpcomingActivity, 0)
	for _, instance := range planned {
		if !instance.Planned {
			continue
		}
		if wallClock(instance.StartWallClock).Before(nowClock) {
			continue
		}
		category := fallbackCategoryName
		if instance.ActivityGroupID != nil {
			if template, ok := byTemplate[*instance.ActivityGroupID]; ok && template.CategoryName != nil {
				category = *template.CategoryName
			}
		}
		upcoming = append(upcoming, UpcomingActivity{
			ID: strconv.FormatInt(instance.ID, 10), Name: instance.Title, Category: category,
			StartTime: instance.StartWallClock.Format("15:04"), RoomName: roomNames[instance.RoomID],
		})
	}
	sort.Slice(upcoming, func(i, j int) bool { return upcoming[i].StartTime < upcoming[j].StartTime })
	return upcoming
}

// wallClock anchors a time of day to the same zero date the composition root
// uses when it normalises a TIME column, so both sides of a comparison carry
// the clock value only.
func wallClock(value time.Time) time.Time {
	return time.Date(1, time.January, 1, value.Hour(), value.Minute(), value.Second(), value.Nanosecond(), time.UTC)
}

// buildPickupBuckets aggregates today's effective pickup times of present
// students into HH:MM count buckets. Only counts leave this function — never
// identities.
func (s *Service) buildPickupBuckets(
	ctx context.Context,
	presentIDs []int64,
	today time.Time,
	now time.Time,
) []PickupBucket {
	buckets := make([]PickupBucket, 0)
	if len(presentIDs) == 0 {
		return buckets
	}
	times, err := s.deps.Dashboard.ListPickupTimes(ctx, presentIDs, today)
	if err != nil {
		// Pickup data is a nice-to-have panel; the dashboard must not go dark
		// because of it. Counts stay correct, the panel just empties.
		s.deps.Observe(observePickupFailure(err))
		return buckets
	}
	nowClock := wallClock(now)
	counts := make(map[string]int)
	for _, effective := range times {
		if effective.PickupWallTime == nil {
			continue
		}
		if wallClock(*effective.PickupWallTime).Before(nowClock) {
			continue
		}
		counts[effective.PickupWallTime.Format("15:04")]++
	}
	for clock, count := range counts {
		buckets = append(buckets, PickupBucket{Time: clock, Count: count})
	}
	sort.Slice(buckets, func(i, j int) bool { return buckets[i].Time < buckets[j].Time })
	return buckets
}
