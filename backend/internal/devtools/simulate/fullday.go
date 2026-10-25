package simulate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"time"
)

// Registered error codes (error-registry.json) the simulation expects from
// the device API.
const (
	codeRFIDTagNotFound      = "iot.rfid_tag_not_found"
	codeRoomCapacityExceeded = "iot.room_capacity_exceeded"
)

var feedbackValues = []string{"positive", "neutral", "negative"}

// Eight seeded sessions receive students round-robin. The 85th student would
// become the 11th child in Leseecke, whose seeded capacity is 10.
const fullDayCheckinLimit = 84

// FullDayOptions configures a full-day simulation run.
type FullDayOptions struct {
	StatePath string
	Profile   string
	Close     bool // if true, do daily checkout + end sessions at the end
	Verbose   bool
	Client    ClientFactory
	Now       func() time.Time // nil uses the system clock
	// SetClock moves the shared clock before each wave of the day (see
	// fullday_clock.go); nil runs every wave at once.
	SetClock ClockSetter
}

func (o FullDayOptions) now() time.Time {
	if o.Now != nil {
		return o.Now()
	}
	return time.Now()
}

// RunFullDay runs a one-shot full-day simulation using seed state.
func RunFullDay(ctx context.Context, opts FullDayOptions) error {
	if opts.Client == nil {
		return fmt.Errorf("simulation client factory is required")
	}
	state, err := LoadSeedStateProfile(opts.StatePath, opts.Profile)
	if err != nil {
		return fmt.Errorf("load seed state: %w", err)
	}

	client, err := buildClient(opts.Client, state.BaseURL, opts.Verbose)
	if err != nil {
		return err
	}
	runtime := newRuntime(state, client, opts)
	scenario := fullDayScenario(opts.Close)
	if err := scenario.Run(ctx, runtime); err != nil {
		profile := state.ProfileKey
		if profile == "" {
			profile = state.Bootstrap.TenantSlug
		}
		var failure error
		var actionErr *ActionError
		if errors.As(err, &actionErr) {
			failure = fmt.Errorf("demo school profile %q, workflow step %s failed: %w", profile, actionErr.Action, actionErr.Err)
		} else {
			failure = fmt.Errorf("demo school profile %q, workflow step %s failed: %w", profile, scenario.Name, err)
		}
		if cleanupErr := endActiveSessions(runtime); cleanupErr != nil {
			return errors.Join(failure, fmt.Errorf("end started sessions after workflow failure: %w", cleanupErr))
		}
		return failure
	}
	return nil
}

type verifyServerAction struct{}

func (verifyServerAction) Name() string { return "server health check" }

func (verifyServerAction) Run(_ context.Context, rt *Runtime) error {
	fmt.Println("Phase 1: Loading seed state and verifying server...")
	if err := rt.Client.CheckHealth(); err != nil {
		return fmt.Errorf("server health check: %w", err)
	}
	fmt.Printf("  Server at %s is healthy\n", rt.State.BaseURL)
	return nil
}

type loginAdminAction struct{}

func (loginAdminAction) Name() string { return "admin login" }

func (loginAdminAction) Run(_ context.Context, rt *Runtime) error {
	if len(rt.State.Accounts.Admin) == 0 {
		return fmt.Errorf("no admin accounts in seed state")
	}
	admin := rt.State.Accounts.Admin[0]
	if err := rt.Client.Login(admin.Email, admin.Password, rt.State.Bootstrap.TenantSlug); err != nil {
		return fmt.Errorf("admin login: %w", err)
	}
	rt.adminLoginAt = rt.Options.now()
	fmt.Printf("  Logged in as %s\n", admin.Email)
	return nil
}

type assignRFIDsAction struct{}

func (assignRFIDsAction) Name() string { return "assign RFID tags" }

func (assignRFIDsAction) Run(_ context.Context, rt *Runtime) error {
	fmt.Printf("\nPhase 2: Assigning RFID tags to %d students...\n", len(rt.State.Students))
	deviceKeysForRFID := sortedDeviceKeys(rt.State.Devices)
	if len(deviceKeysForRFID) == 0 {
		return fmt.Errorf("no devices in seed state for RFID assignment")
	}
	if len(rt.State.Students) == 0 {
		return fmt.Errorf("no students in seed state for RFID assignment")
	}
	rfidDevice := rt.State.Devices[deviceKeysForRFID[0]]

	for _, student := range rt.State.Students {
		rfidTag := fmt.Sprintf("DE%06X", student.ID)
		rt.RFIDTags[student.ID] = rfidTag

		body := map[string]string{"rfid_tag": rfidTag}
		_, err := rt.Client.DevicePost(fmt.Sprintf("/api/students/%d/rfid", student.ID), body, rfidDevice.APIKey, rt.State.DevicePIN)
		if err != nil {
			return fmt.Errorf("assign RFID to student %d: %w", student.ID, err)
		}
		rt.Counts.RFIDAssigned++
	}
	fmt.Printf("  %d/%d RFID tags assigned\n", rt.Counts.RFIDAssigned, len(rt.State.Students))
	return nil
}

type startSessionsAction struct{}

func (startSessionsAction) Name() string { return "start sessions" }

// Run starts one session per minute, each with its own caregiver.
func (startSessionsAction) Run(ctx context.Context, rt *Runtime) error {
	fmt.Println("\nPhase 3: Starting activity sessions...")

	rt.DeviceKeys = sortedDeviceKeys(rt.State.Devices)
	rt.ActivityNames = sortedStringKeys(rt.State.Activities)
	betreuer, err := rt.availableCaregivers()
	if err != nil {
		return err
	}

	sessionsToStart := min(len(betreuer), len(rt.DeviceKeys), len(rt.ActivityNames))
	if sessionsToStart == 0 {
		return fmt.Errorf("no sessions can start: %d staff accounts, %d devices, %d activities", len(betreuer), len(rt.DeviceKeys), len(rt.ActivityNames))
	}
	if sessionsToStart > 10 {
		sessionsToStart = 10
	}

	for i := 0; i < sessionsToStart; i++ {
		if err := rt.advanceClock(ctx, sessionsStart+wallClock(i)); err != nil {
			return err
		}
		if err := rt.startSession(rt.DeviceKeys[i], rt.ActivityNames[i], betreuer[i]); err != nil {
			if cleanupErr := endActiveSessions(rt); cleanupErr != nil {
				return errors.Join(err, fmt.Errorf("end previously started sessions: %w", cleanupErr))
			}
			return err
		}
	}

	if len(rt.ActiveRoomIDs) == 0 {
		for _, roomID := range rt.State.Rooms {
			rt.ActiveRoomIDs = append(rt.ActiveRoomIDs, roomID)
		}
	}

	fmt.Printf("  %d sessions started\n", rt.Counts.SessionsStarted)
	return nil
}

// startSession starts the activity on the device with one supervisor, in the
// activity's room when the demo convention names one.
func (rt *Runtime) startSession(deviceKey, actName string, supervisor AccountCredentials) error {
	device := rt.State.Devices[deviceKey]
	roomID := findRoomForActivity(actName, rt.State.Rooms)
	body := map[string]any{
		"activity_id":    rt.State.Activities[actName],
		"supervisor_ids": []int64{supervisor.StaffID},
	}
	if roomID != 0 {
		body["room_id"] = roomID
	}
	if _, err := rt.Client.DevicePost("/api/iot/session/start", body, device.APIKey, rt.State.DevicePIN); err != nil {
		return fmt.Errorf("start session for %s: %w", actName, err)
	}
	rt.ActiveSessionDeviceKeys = append(rt.ActiveSessionDeviceKeys, deviceKey)
	rt.SupervisorStaffIDs = append(rt.SupervisorStaffIDs, supervisor.StaffID)
	if roomID != 0 {
		rt.ActiveRoomIDs = append(rt.ActiveRoomIDs, roomID)
	}
	rt.Counts.SessionsStarted++
	fmt.Printf("  Started: %s (device: %s, supervisor: %s)\n", actName, device.Name, supervisor.Name)
	return nil
}

// isoDate is the wire layout of every calendar date in the moto API.
const isoDate = "2006-01-02"

// calendarPeriodWindow is the subset of a calendar period the demo
// cancellation needs from the timetable periods API. The fields beyond the
// window itself are what PUT /periods requires to echo a period back
// unchanged except for is_active.
type calendarPeriodWindow struct {
	ID              int64   `json:"id"`
	Name            string  `json:"name"`
	PeriodType      string  `json:"period_type"`
	StartDate       string  `json:"start_date"`
	EndDate         string  `json:"end_date"`
	WeekCycleLength int     `json:"week_cycle_length"`
	WeekCycleAnchor *string `json:"week_cycle_anchor,omitempty"`
	IsActive        bool    `json:"is_active"`
}

// firstFutureWeekday returns the earliest weekday after today that falls into
// [start, end], or the zero time when the window holds none.
func firstFutureWeekday(start, end, today time.Time) time.Time {
	candidate := today.AddDate(0, 0, 1)
	if start.After(candidate) {
		candidate = start
	}
	for candidate.Weekday() == time.Saturday || candidate.Weekday() == time.Sunday {
		candidate = candidate.AddDate(0, 0, 1)
	}
	if candidate.After(end) {
		return time.Time{}
	}
	return candidate
}

// firstFutureWeekdayInPeriods returns the earliest weekday after today that
// lies inside an active calendar period, or the zero time when no period
// covers one. The server rejects instances outside every active period.
func firstFutureWeekdayInPeriods(periods []calendarPeriodWindow, today time.Time, loc *time.Location) (time.Time, error) {
	var date time.Time
	for _, period := range periods {
		if !period.IsActive {
			continue
		}
		start, startErr := time.ParseInLocation(isoDate, period.StartDate, loc)
		if startErr != nil {
			return time.Time{}, fmt.Errorf("decode calendar period start date for demo cancellation: %w", startErr)
		}
		end, endErr := time.ParseInLocation(isoDate, period.EndDate, loc)
		if endErr != nil {
			return time.Time{}, fmt.Errorf("decode calendar period end date for demo cancellation: %w", endErr)
		}
		candidate := firstFutureWeekday(start, end, today)
		if candidate.IsZero() || (!date.IsZero() && !candidate.Before(date)) {
			continue
		}
		date = candidate
	}
	return date, nil
}

// schoolYearBounds mirrors DefaultSchoolYear in
// backend/modules/schoolcalendar/week_cycle.go: a German school year
// runs from August 1st to July 31st of the following year. The simulate
// package must not import internal/timezone (scripts/backend-architecture.sh),
// so the bounds are recomputed here on plain time values.
func schoolYearBounds(startYear int, loc *time.Location) (name string, start, end time.Time) {
	return fmt.Sprintf("Schuljahr %d/%d", startYear, startYear+1),
		time.Date(startYear, time.August, 1, 0, 0, 0, 0, loc),
		time.Date(startYear+1, time.July, 31, 0, 0, 0, 0, loc)
}

// schoolYearStartYear returns the year whose August opened the school year
// containing today.
func schoolYearStartYear(today time.Time) int {
	if today.Month() < time.August {
		return today.Year() - 1
	}
	return today.Year()
}

// ensureSchoolYearWithFutureWeekday gives the tenant an active school year
// that still holds a weekday after today and returns that weekday. Bootstrap
// only ever creates a period for a tenant without any, so at the end of a
// school year — and permanently after a school-year change on an already
// seeded tenant — the demo cancellation would otherwise have no date left to
// use (#3173). Depending on what the tenant already has, the school year is
// created or merely activated.
func (rt *Runtime) ensureSchoolYearWithFutureWeekday(existing []calendarPeriodWindow, today time.Time, loc *time.Location) (time.Time, error) {
	startYear := schoolYearStartYear(today)
	// The school year containing today is the first candidate: an existing
	// tenant that has not rolled over yet is missing exactly that one. Later
	// candidates cover a school year that is already there but unusable, plus
	// the hand-edited calendars that make a create conflict.
	for _, candidateYear := range []int{startYear, startYear + 1, startYear + 2} {
		name, start, end := schoolYearBounds(candidateYear, loc)
		date := firstFutureWeekday(start, end, today)
		if date.IsZero() {
			continue
		}
		index := slices.IndexFunc(existing, func(p calendarPeriodWindow) bool {
			return p.StartDate == start.Format(isoDate) && p.EndDate == end.Format(isoDate)
		})
		switch {
		case index < 0:
			err := rt.createSchoolYearPeriod(name, start, end)
			if isConflictError(err) {
				// A hand-edited calendar holds this name or an active
				// same-type period overlapping these bounds. Neither is
				// something the simulation may rewrite; try the next year.
				continue
			}
			if err != nil {
				return time.Time{}, err
			}
		case existing[index].IsActive:
			// Already considered above and rejected for want of a weekday.
			continue
		default:
			if err := rt.activatePeriod(existing[index]); err != nil {
				return time.Time{}, err
			}
		}
		return date, nil
	}
	return time.Time{}, nil
}

// createSchoolYearPeriod adds one active school year through the regular
// periods API.
func (rt *Runtime) createSchoolYearPeriod(name string, start, end time.Time) error {
	body := map[string]any{
		"name":              name,
		"period_type":       "school_year",
		"start_date":        start.Format(isoDate),
		"end_date":          end.Format(isoDate),
		"week_cycle_length": 1,
		"is_active":         true,
	}
	if _, err := rt.Client.Post("/api/timetable/periods", body); err != nil {
		return fmt.Errorf("create %s for demo cancellation: %w", name, err)
	}
	return nil
}

// activatePeriod flips an existing period active, echoing every other field
// back unchanged as PUT /periods requires.
func (rt *Runtime) activatePeriod(period calendarPeriodWindow) error {
	body := map[string]any{
		"name":              period.Name,
		"period_type":       period.PeriodType,
		"start_date":        period.StartDate,
		"end_date":          period.EndDate,
		"week_cycle_length": period.WeekCycleLength,
		"is_active":         true,
	}
	if period.WeekCycleAnchor != nil {
		body["week_cycle_anchor"] = *period.WeekCycleAnchor
	}
	if _, err := rt.Client.Put(fmt.Sprintf("/api/timetable/periods/%d", period.ID), body); err != nil {
		return fmt.Errorf("activate %s for demo cancellation: %w", period.Name, err)
	}
	return nil
}

// isConflictError reports whether the API rejected a write with 409 — a name
// clash or an overlapping active period of the same type.
func isConflictError(err error) bool {
	var httpErr interface{ HTTPStatusCode() int }
	return errors.As(err, &httpErr) && httpErr.HTTPStatusCode() == 409
}

type seedStaffFeedTombstoneAction struct{}

func (seedStaffFeedTombstoneAction) Name() string { return "seed staff feed tombstone" }

func (seedStaffFeedTombstoneAction) Run(_ context.Context, rt *Runtime) error {
	if len(rt.State.Accounts.Betreuer) == 0 {
		return fmt.Errorf("no staff accounts in seed state")
	}
	roomNames := sortedStringKeys(rt.State.Rooms)
	if len(roomNames) == 0 {
		return fmt.Errorf("no rooms in seed state")
	}

	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		return fmt.Errorf("load Berlin timezone: %w", err)
	}
	periodResponse, err := rt.Client.Post("/api/timetable/periods/bootstrap", nil)
	if err != nil {
		return fmt.Errorf("bootstrap calendar periods for demo cancellation: %w", err)
	}
	var periodEnvelope struct {
		Data struct {
			Periods []calendarPeriodWindow `json:"periods"`
		} `json:"data"`
	}
	if err := json.Unmarshal(periodResponse, &periodEnvelope); err != nil {
		return fmt.Errorf("decode POST /api/timetable/periods/bootstrap response: %w", err)
	}
	today, err := time.ParseInLocation(isoDate, rt.Options.now().In(berlin).Format(isoDate), berlin)
	if err != nil {
		return fmt.Errorf("normalize current date: %w", err)
	}
	date, err := firstFutureWeekdayInPeriods(periodEnvelope.Data.Periods, today, berlin)
	if err != nil {
		return err
	}
	if date.IsZero() {
		// End of the school year, or a tenant whose only period already
		// expired: the bootstrap endpoint returns existing periods unchanged,
		// so the simulation rolls the calendar forward itself (#3173).
		date, err = rt.ensureSchoolYearWithFutureWeekday(periodEnvelope.Data.Periods, today, berlin)
		if err != nil {
			return err
		}
	}
	if date.IsZero() {
		return fmt.Errorf("no future weekday in an active calendar period for demo cancellation")
	}
	body := map[string]any{
		"date":       date.Format(isoDate),
		"start_time": "07:00",
		"end_time":   "07:30",
		"title":      "Abgesagter Demo-Termin",
		"room_id":    rt.State.Rooms[roomNames[0]],
		"staff_ids":  []int64{rt.State.Accounts.Betreuer[0].StaffID},
	}
	created, err := rt.Client.Post("/api/timetable/instances", body)
	if err != nil {
		return fmt.Errorf("create demo cancellation: %w", err)
	}
	var envelope struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(created, &envelope); err != nil {
		return fmt.Errorf("decode POST /api/timetable/instances response: %w", err)
	}
	if envelope.Data.ID <= 0 {
		return fmt.Errorf("decode demo cancellation: response has no instance id")
	}
	if _, err := rt.Client.Delete(fmt.Sprintf("/api/timetable/instances/%d", envelope.Data.ID)); err != nil {
		return fmt.Errorf("delete demo cancellation: %w", err)
	}
	return nil
}

// independentRoomStayCount is how many children the demo moves into the
// released Sporthalle beside the football children.
const independentRoomStayCount = 3

type independentRoomStaysAction struct{}

func (independentRoomStaysAction) Name() string { return "record independent room stays" }

// Run moves a few checked-in children from other rooms into the released
// Sporthalle through the staff move-to-room endpoint (#3066), so the shared
// room view shows independent children next to the football children.
func (independentRoomStaysAction) Run(_ context.Context, rt *Runtime) error {
	fmt.Println("\nPhase 5b: Independent stays in a released room...")

	sporthalle := rt.State.Rooms["Sporthalle"]
	if sporthalle == 0 || len(rt.ActiveRoomIDs) == 0 {
		fmt.Println("  Skipped: no Sporthalle or no checked-in children in this profile")
		return nil
	}

	// The first children placed outside the Sporthalle are moved.
	studentIDs := checkedInChildrenOutside(rt, sporthalle, 0, independentRoomStayCount)
	if len(studentIDs) == 0 {
		fmt.Println("  Skipped: every checked-in child is already in the Sporthalle")
		return nil
	}

	if _, err := rt.Client.Post("/api/active/visits/move-to-room", map[string]any{
		"student_ids":    studentIDs,
		"target_room_id": sporthalle,
	}); err != nil {
		return fmt.Errorf("move children into the released Sporthalle: %w", err)
	}
	rt.Counts.IndependentStays += len(studentIDs)

	fmt.Printf("  %d children stay in the Sporthalle without joining football\n", rt.Counts.IndependentStays)
	return nil
}

// checkedInChildrenOutside lists, in check-in order, up to limit children who
// stay checked in through the day and were placed outside roomID, after
// skipping the first skip of them. Children below fullDayRoomStayLimit stay
// checked in.
func checkedInChildrenOutside(rt *Runtime, roomID int64, skip, limit int) []int64 {
	ids := make([]int64, 0, limit)
	for i := 0; i < fullDayRoomStayLimit && i < len(rt.State.Students) && len(rt.ActiveRoomIDs) > 0; i++ {
		if rt.ActiveRoomIDs[i%len(rt.ActiveRoomIDs)] == roomID {
			continue
		}
		if skip > 0 {
			skip--
			continue
		}
		ids = append(ids, rt.State.Students[i].ID)
		if len(ids) == limit {
			break
		}
	}
	return ids
}

// openRoomBlockTitle names the timetable block the demo runs in the released
// Sporthalle, and openRoomBlockChildren is how many children it checks in.
const (
	openRoomBlockTitle    = "Turnen"
	openRoomBlockChildren = 2
)

type openRoomBlockAction struct{}

func (openRoomBlockAction) Name() string { return "run a block in a released room" }

// Run plans a block for today in the released Sporthalle, starts it, and
// checks a few children into it. The Sporthalle already holds the football
// session and the independent stays, so its room page shows a block section
// beside them (#3281). The first caregiver without a running session is
// planned on the block, so nobody supervises two rooms at once. There is no
// care on weekends, so the phase skips them.
func (openRoomBlockAction) Run(_ context.Context, rt *Runtime) error {
	fmt.Println("\nPhase 5c: A block in a released room...")

	sporthalle := rt.State.Rooms["Sporthalle"]
	caregiver, err := rt.freeCaregiver()
	if err != nil {
		return err
	}
	if sporthalle == 0 || caregiver == 0 {
		fmt.Println("  Skipped: no Sporthalle or no free caregiver in this profile")
		return nil
	}
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		return fmt.Errorf("load Berlin time zone: %w", err)
	}
	today := rt.Options.now().In(berlin)
	if today.Weekday() == time.Saturday || today.Weekday() == time.Sunday {
		fmt.Println("  Skipped: no care day on weekends")
		return nil
	}
	studentIDs := checkedInChildrenOutside(rt, sporthalle, independentRoomStayCount, openRoomBlockChildren)
	if len(studentIDs) == 0 {
		fmt.Println("  Skipped: no checked-in child left outside the Sporthalle")
		return nil
	}

	created, err := rt.Client.Post("/api/timetable/instances", map[string]any{
		"date":        today.Format(isoDate),
		"start_time":  "14:00",
		"end_time":    "15:30",
		"title":       openRoomBlockTitle,
		"room_id":     sporthalle,
		"staff_ids":   []int64{caregiver},
		"student_ids": studentIDs,
	})
	if err != nil {
		return fmt.Errorf("plan the block in the released Sporthalle: %w", err)
	}
	instanceID, err := responseDataID(created)
	if err != nil {
		return fmt.Errorf("parse the planned block: %w", err)
	}
	if _, err := rt.Client.Post(fmt.Sprintf("/api/timetable/instances/%d/start", instanceID), nil); err != nil {
		return fmt.Errorf("start the block in the released Sporthalle: %w", err)
	}
	rt.OpenRoomBlockInstanceID = instanceID
	for _, studentID := range studentIDs {
		path := fmt.Sprintf("/api/timetable/operations/instances/%d/students/%d/check-in", instanceID, studentID)
		if _, err := rt.Client.Post(path, nil); err != nil {
			return fmt.Errorf("check student %d into the block: %w", studentID, err)
		}
		rt.Counts.OpenRoomBlockChildren++
	}

	fmt.Printf("  %s runs in the Sporthalle with %d children\n", openRoomBlockTitle, rt.Counts.OpenRoomBlockChildren)
	return nil
}

type kioskOpenRoomStayAction struct{}

func (kioskOpenRoomStayAction) Name() string { return "choose a released room at the kiosk" }

// Run lets one more child leave its room at the kiosk and choose the released
// Sporthalle as destination (#3067). The kiosk books the independent stay
// itself; the Sporthalle needs no device and no second scan.
func (kioskOpenRoomStayAction) Run(_ context.Context, rt *Runtime) error {
	fmt.Println("\nPhase 5d: A released room chosen at the kiosk...")

	sporthalle := rt.State.Rooms["Sporthalle"]
	if sporthalle == 0 || len(rt.DeviceKeys) == 0 {
		fmt.Println("  Skipped: no Sporthalle or no kiosk in this profile")
		return nil
	}
	studentIDs := checkedInChildrenOutside(rt, sporthalle, independentRoomStayCount+openRoomBlockChildren, 1)
	if len(studentIDs) == 0 {
		fmt.Println("  Skipped: no checked-in child left outside the Sporthalle")
		return nil
	}
	rfidTag, ok := rt.RFIDTags[studentIDs[0]]
	if !ok {
		return fmt.Errorf("RFID assignment missing for student %d", studentIDs[0])
	}
	primaryDevice, err := rt.primaryDevice()
	if err != nil {
		return err
	}
	if _, err := rt.Client.DevicePost("/api/iot/move-to-room", map[string]any{
		"student_rfid": rfidTag,
		"room_id":      sporthalle,
	}, primaryDevice.APIKey, rt.State.DevicePIN); err != nil {
		return fmt.Errorf("book the released Sporthalle at the kiosk: %w", err)
	}
	rt.Counts.IndependentStays++

	fmt.Println("  1 child chose the Sporthalle at the kiosk")
	return nil
}

// completeOpenRoomBlock ends the block the full-day run started, once its
// children have gone home.
func completeOpenRoomBlock(rt *Runtime) error {
	if rt.OpenRoomBlockInstanceID == 0 {
		return nil
	}
	if _, err := rt.Client.Post(fmt.Sprintf("/api/timetable/instances/%d/complete", rt.OpenRoomBlockInstanceID), nil); err != nil {
		return fmt.Errorf("complete the block in the released Sporthalle: %w", err)
	}
	rt.OpenRoomBlockInstanceID = 0
	return nil
}

// responseDataID reads data.id of an API envelope, which the API writes as a
// number or a string.
func responseDataID(raw []byte) (int64, error) {
	var envelope struct {
		Data struct {
			ID json.Number `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return 0, err
	}
	id, err := envelope.Data.ID.Int64()
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid id %q", envelope.Data.ID)
	}
	return id, nil
}

func endActiveSessions(rt *Runtime) error {
	var errs []error
	for _, deviceKey := range slices.Clone(rt.ActiveSessionDeviceKeys) {
		device := rt.State.Devices[deviceKey]
		_, err := rt.Client.DevicePost("/api/iot/session/end", nil, device.APIKey, rt.State.DevicePIN)
		if err != nil {
			errs = append(errs, fmt.Errorf("end session on device %s: %w", deviceKey, err))
			continue
		}
		rt.removeActiveSessionDevice(deviceKey)
		rt.Counts.SessionsEnded++
	}
	return errors.Join(errs...)
}

func (rt *Runtime) removeActiveSessionDevice(deviceKey string) {
	for i, activeDeviceKey := range rt.ActiveSessionDeviceKeys {
		if activeDeviceKey == deviceKey {
			rt.ActiveSessionDeviceKeys = append(rt.ActiveSessionDeviceKeys[:i], rt.ActiveSessionDeviceKeys[i+1:]...)
			return
		}
	}
}

type printSummaryAction struct{}

func (printSummaryAction) Name() string { return "print summary" }

func (printSummaryAction) Run(_ context.Context, rt *Runtime) error {
	fmt.Println("\n--- Simulation Summary ---")
	fmt.Printf("  RFID tags assigned:  %d\n", rt.Counts.RFIDAssigned)
	fmt.Printf("  Sessions started:    %d\n", rt.Counts.SessionsStarted)
	fmt.Printf("  Attendance records:  %d\n", rt.Counts.AttendanceRecords)
	fmt.Printf("  Students checked in: %d\n", rt.Counts.StudentsCheckedIn)
	fmt.Printf("  Students sick:       %d\n", rt.Counts.StudentsSick)
	fmt.Printf("  Students unterwegs:  %d\n", rt.Counts.StudentsCheckedOut)
	fmt.Printf("  Early care children: %d\n", rt.Counts.EarlyCareChildren)
	fmt.Printf("  Early pickups:       %d\n", rt.Counts.EarlyPickups)
	fmt.Printf("  Staff clocked in:    %d (%d on leave)\n", rt.Counts.StaffClockedIn, rt.Counts.StaffAbsent)
	fmt.Printf("  Independent stays:  %d\n", rt.Counts.IndependentStays)
	fmt.Printf("  Open-room block:    %d children\n", rt.Counts.OpenRoomBlockChildren)
	if rt.Options.Close {
		fmt.Printf("  Feedback submitted:  %d\n", rt.Counts.FeedbackSubmitted)
		fmt.Println("  End-of-day:          completed")
	}
	fmt.Println("--------------------------")
	return nil
}

// findRoomForActivity maps activity names to their default rooms using the demo data convention.
func findRoomForActivity(activityName string, rooms map[string]int64) int64 {
	activityRoomMap := map[string]string{
		"Hausaufgaben": "OGS-Raum 1",
		"Fußball":      "Sporthalle",
		"Basteln":      "Kreativraum",
		"Kochen":       "Mensa",
		"Lesen":        "Leseecke",
		"Musik":        "Musikraum",
		"Tanzen":       "Bewegungsraum",
		"Schach":       "OGS-Raum 2",
		"Garten":       "Schulhof",
		"Freispiel":    "OGS-Raum 3",
	}

	roomName, ok := activityRoomMap[activityName]
	if !ok {
		return 0
	}
	return rooms[roomName]
}

func sortedDeviceKeys(devices map[string]SeedDevice) []string {
	return slices.Sorted(maps.Keys(devices))
}

func sortedStringKeys(m map[string]int64) []string {
	return slices.Sorted(maps.Keys(m))
}
