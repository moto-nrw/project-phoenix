package simulate

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
)

// The children of a full day, by their index in the seed state:
//
//   - below fullDayRoomStayLimit: arrive and stay in a room all afternoon;
//   - fullDayRoomStayLimit up to fullDayCheckinLimit: arrive in a room and
//     leave it again without going home (unterwegs);
//   - fullDayCheckinLimit up to fullDayArrivalLimit: arrive, find no room
//     place and are picked up early;
//   - from fullDayArrivalLimit: still at school, or reported sick.
const (
	fullDayRoomStayLimit = fullDayCheckinLimit - 2
	fullDayArrivalLimit  = 90
	fullDaySickFrom      = 96
	fullDaySickTo        = 100 // four fall ill: with the seed's, under one child in ten
)

// earlyCareActivity is the activity whose room hosts the Frühbetreuung, and
// earlyCareChildren is how many children come to it.
const (
	earlyCareActivity = "Lesen"
	earlyCareChildren = 6
)

type earlyCareAction struct{}

func (earlyCareAction) Name() string { return "run early care" }

// Run opens the Frühbetreuung: a few children arrive one per minute before
// their lessons, then leave for school, and the session ends. Their arrival
// stays on record for the day. Profiles without the reading room skip it.
func (earlyCareAction) Run(ctx context.Context, rt *Runtime) error {
	fmt.Println("\nPhase 3: Frühbetreuung...")
	activityID := rt.State.Activities[earlyCareActivity]
	roomID := findRoomForActivity(earlyCareActivity, rt.State.Rooms)
	deviceKeys := sortedDeviceKeys(rt.State.Devices)
	if activityID == 0 || roomID == 0 || len(deviceKeys) == 0 {
		fmt.Println("  Skipped: no reading room or device in this profile")
		return nil
	}
	betreuer, err := rt.availableCaregivers()
	if err != nil || len(betreuer) == 0 {
		fmt.Println("  Skipped: no caregiver at work")
		return err
	}
	deviceKey := deviceKeys[0]
	device := rt.State.Devices[deviceKey]
	// The last caregiver opens the day; the midday sessions start with the first.
	if _, err := rt.Client.DevicePost("/api/iot/session/start", map[string]any{
		"activity_id": activityID, "room_id": roomID, "supervisor_ids": []int64{betreuer[len(betreuer)-1].StaffID},
	}, device.APIKey, rt.State.DevicePIN); err != nil {
		return fmt.Errorf("start early care: %w", err)
	}
	rt.ActiveSessionDeviceKeys = append(rt.ActiveSessionDeviceKeys, deviceKey)

	if err := rt.earlyCareChildrenComeAndGo(ctx, device, roomID); err != nil {
		return err
	}
	if _, err := rt.Client.DevicePost("/api/iot/session/end", nil, device.APIKey, rt.State.DevicePIN); err != nil {
		return fmt.Errorf("end early care: %w", err)
	}
	rt.removeActiveSessionDevice(deviceKey)
	fmt.Printf("  %d children in early care\n", rt.Counts.EarlyCareChildren)
	return nil
}

// earlyCareChildrenComeAndGo lets the early care children arrive one per
// minute and leave for their lessons together.
func (rt *Runtime) earlyCareChildrenComeAndGo(ctx context.Context, device SeedDevice, roomID int64) error {
	children := rt.State.Students[:min(earlyCareChildren, len(rt.State.Students), fullDayRoomStayLimit)]
	for n, student := range children {
		if err := rt.advanceClock(ctx, earlyCareStart+wallClock(n)); err != nil {
			return err
		}
		if err := rt.confirmAttendance(student, device); err != nil {
			return err
		}
		if err := rt.checkInto(student, roomID, device); err != nil {
			return err
		}
		rt.Counts.EarlyCareChildren++
	}
	if err := rt.advanceClock(ctx, earlyCareEnd); err != nil {
		return err
	}
	for _, student := range children {
		if err := rt.leaveRoom(student, device); err != nil {
			return err
		}
	}
	return nil
}

type sickCallsAction struct{}

func (sickCallsAction) Name() string { return "report sick children" }

func (sickCallsAction) Run(_ context.Context, rt *Runtime) error {
	for i := fullDaySickFrom; i < len(rt.State.Students) && i < fullDaySickTo; i++ {
		student := rt.State.Students[i]
		if _, err := rt.Client.Put(fmt.Sprintf("/api/students/%d", student.ID), map[string]any{"sick": true}); err != nil {
			return fmt.Errorf("mark student %d sick: %w", student.ID, err)
		}
		rt.Counts.StudentsSick++
	}
	fmt.Printf("  %d children reported sick\n", rt.Counts.StudentsSick)
	return nil
}

type recordAttendanceAction struct{}

func (recordAttendanceAction) Name() string { return "record attendance and checkins" }

// Run lets the children arrive after their lessons, class by class and a few
// per minute, and checks them into the running sessions in turn.
func (recordAttendanceAction) Run(ctx context.Context, rt *Runtime) error {
	fmt.Println("\nPhase 4: Recording attendance and checking in students...")

	primaryDevice, err := rt.primaryDevice()
	if err != nil {
		return err
	}
	if err := recordUnregisteredTag(rt, primaryDevice); err != nil {
		return err
	}
	for w, indices := range arrivalOrder(rt.State.Students) {
		for n, i := range indices {
			if err := rt.advanceClock(ctx, arrivalWaves[w].start+wallClock(n/arrivalsPerMinute)); err != nil {
				return err
			}
			if err := rt.arrive(i, primaryDevice); err != nil {
				return err
			}
		}
	}

	fmt.Printf("  %d attendance records, %d students checked in\n", rt.Counts.AttendanceRecords, rt.Counts.StudentsCheckedIn)
	return nil
}

// recordUnregisteredTag exercises the real kiosk failure path too. The
// endpoint intentionally returns 404, while recording the unknown tag for
// operator follow-up.
func recordUnregisteredTag(rt *Runtime, device SeedDevice) error {
	_, err := rt.Client.DevicePost("/api/iot/checkin", map[string]any{
		"student_rfid": "DEMO-UNREGISTERED-TAG", "action": "checkin",
	}, device.APIKey, rt.State.DevicePIN)
	var expectedErr interface {
		HTTPStatusCode() int
		HTTPErrorCode() string
	}
	if !errors.As(err, &expectedErr) || expectedErr.HTTPStatusCode() != 404 || expectedErr.HTTPErrorCode() != codeRFIDTagNotFound {
		return fmt.Errorf("record unregistered tag scan: expected 404 (rfid_tag_not_found), got %w", err)
	}
	return nil
}

// arrivalOrder groups the arriving children's indices by arrival wave.
func arrivalOrder(students []SeedStudent) [][]int {
	waves := make([][]int, len(arrivalWaves))
	for i := 0; i < len(students) && i < fullDayArrivalLimit; i++ {
		w := arrivalWaveIndex(students[i].Class)
		waves[w] = append(waves[w], i)
	}
	return waves
}

// arrive records the child's attendance unless the early care already did,
// and checks the child into its room. The rooms take the children in turn;
// children from fullDayCheckinLimit on find no place left.
func (rt *Runtime) arrive(i int, device SeedDevice) error {
	student := rt.State.Students[i]
	if !rt.Present[student.ID] {
		if err := rt.confirmAttendance(student, device); err != nil {
			return err
		}
	}
	if i >= fullDayCheckinLimit || len(rt.ActiveRoomIDs) == 0 {
		return nil
	}
	if err := rt.checkInto(student, rt.ActiveRoomIDs[i%len(rt.ActiveRoomIDs)], device); err != nil {
		return err
	}
	rt.Counts.StudentsCheckedIn++
	return nil
}

func (rt *Runtime) studentTag(student SeedStudent) (string, error) {
	tag, ok := rt.RFIDTags[student.ID]
	if !ok {
		return "", fmt.Errorf("RFID assignment missing for student %d", student.ID)
	}
	return tag, nil
}

func (rt *Runtime) confirmAttendance(student SeedStudent, device SeedDevice) error {
	tag, err := rt.studentTag(student)
	if err != nil {
		return err
	}
	if _, err := rt.Client.DevicePost("/api/iot/attendance/toggle", map[string]string{"rfid": tag, "action": "confirm"}, device.APIKey, rt.State.DevicePIN); err != nil {
		return fmt.Errorf("record attendance for student %d: %w", student.ID, err)
	}
	rt.Present[student.ID] = true
	rt.Counts.AttendanceRecords++
	return nil
}

func (rt *Runtime) checkInto(student SeedStudent, roomID int64, device SeedDevice) error {
	tag, err := rt.studentTag(student)
	if err != nil {
		return err
	}
	if _, err := rt.Client.DevicePost("/api/iot/checkin", map[string]any{
		"student_rfid": tag, "action": "checkin", "room_id": roomID,
	}, device.APIKey, rt.State.DevicePIN); err != nil {
		return fmt.Errorf("check in student %d: %w", student.ID, err)
	}
	return nil
}

// leaveRoom ends the child's room visit; the child stays present for the day.
func (rt *Runtime) leaveRoom(student SeedStudent, device SeedDevice) error {
	tag, err := rt.studentTag(student)
	if err != nil {
		return err
	}
	if _, err := rt.Client.DevicePost("/api/iot/checkin", map[string]any{
		"student_rfid": tag, "action": "checkout",
	}, device.APIKey, rt.State.DevicePIN); err != nil {
		return fmt.Errorf("check out student %d: %w", student.ID, err)
	}
	return nil
}

type middayActivityAction struct{}

func (middayActivityAction) Name() string { return "run mid-day activity" }

// Run lets two children leave their room without going home, so the day
// shows a few children unterwegs.
func (middayActivityAction) Run(_ context.Context, rt *Runtime) error {
	fmt.Println("\nPhase 5: Mid-day activity (children unterwegs)...")

	primaryDevice, err := rt.primaryDevice()
	if err != nil {
		return err
	}
	for i := fullDayRoomStayLimit; i < fullDayCheckinLimit && i < len(rt.State.Students); i++ {
		if err := rt.leaveRoom(rt.State.Students[i], primaryDevice); err != nil {
			return err
		}
		rt.Counts.StudentsCheckedOut++
	}

	fmt.Printf("  %d children left their room\n", rt.Counts.StudentsCheckedOut)
	return nil
}

type earlyPickupsAction struct{}

func (earlyPickupsAction) Name() string { return "pick up children early" }

// Run sends the children who found no room place home for the day.
func (earlyPickupsAction) Run(_ context.Context, rt *Runtime) error {
	fmt.Println("\nPhase 5e: Early pickups...")
	primaryDevice, err := rt.primaryDevice()
	if err != nil {
		return err
	}
	for i := fullDayCheckinLimit; i < fullDayArrivalLimit && i < len(rt.State.Students); i++ {
		if err := rt.dailyCheckout(rt.State.Students[i], primaryDevice); err != nil {
			return err
		}
		rt.Counts.EarlyPickups++
	}
	fmt.Printf("  %d children went home\n", rt.Counts.EarlyPickups)
	return nil
}

type endOfDayAction struct{}

func (endOfDayAction) Name() string { return "run end-of-day flow" }

func (endOfDayAction) Run(_ context.Context, rt *Runtime) error {
	fmt.Println("\nPhase 6: End of day (daily checkout + end sessions)...")

	primaryDevice, err := rt.primaryDevice()
	if err != nil {
		return err
	}
	for i := 0; i < fullDayCheckinLimit && i < len(rt.State.Students); i++ {
		if err := rt.dailyCheckout(rt.State.Students[i], primaryDevice); err != nil {
			return err
		}
	}
	fmt.Printf("  %d daily checkouts, %d feedback submitted\n", rt.Counts.DailyCheckouts, rt.Counts.FeedbackSubmitted)

	if err := completeOpenRoomBlock(rt); err != nil {
		return err
	}
	if err := endActiveSessions(rt); err != nil {
		return err
	}
	fmt.Printf("  %d sessions ended\n", rt.Counts.SessionsEnded)
	return nil
}

// dailyCheckout sends a child home through the kiosk: pickup query, daily
// checkout and the smiley feedback, as PyrePortal does.
func (rt *Runtime) dailyCheckout(student SeedStudent, device SeedDevice) error {
	tag, err := rt.studentTag(student)
	if err != nil {
		return err
	}
	if _, err := rt.Client.DevicePost("/api/iot/pickup-query", map[string]any{
		"student_rfid": tag,
	}, device.APIKey, rt.State.DevicePIN); err != nil {
		return fmt.Errorf("query pickup for student %d: %w", student.ID, err)
	}
	if _, err := rt.Client.DevicePost("/api/iot/attendance/toggle", map[string]any{
		"rfid": tag, "action": "confirm_daily_checkout", "destination": "zuhause",
	}, device.APIKey, rt.State.DevicePIN); err != nil {
		return fmt.Errorf("record daily checkout for student %d: %w", student.ID, err)
	}
	rt.Counts.DailyCheckouts++

	if _, err := rt.Client.DevicePost("/api/iot/feedback", map[string]any{
		"student_id": student.ID,
		"value":      feedbackValues[rand.Intn(len(feedbackValues))],
	}, device.APIKey, rt.State.DevicePIN); err != nil {
		return fmt.Errorf("submit feedback for student %d: %w", student.ID, err)
	}
	rt.Counts.FeedbackSubmitted++
	return nil
}
