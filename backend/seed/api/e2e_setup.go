package api

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/moto-nrw/project-phoenix/e2e/scenarios"
	"github.com/moto-nrw/project-phoenix/integration/phoenixapi"
)

type e2eCheckinProvision struct {
	studentID         int64
	studentRFIDTag    string
	sickStudentID     int64
	sickRFIDTag       string
	roomID            int64
	activityID        int64
	supervisorStaffID int64
	deviceAPIKey      string
}

func (s *Seeder) provisionE2ECheckinFixture(ctx context.Context, rt *Runtime) error {
	if rt == nil || rt.Client == nil {
		return fmt.Errorf("e2e checkin fixture requires tenant API client")
	}

	definition, ok := scenarios.Lookup(s.options.Scenario)
	if !ok {
		return fmt.Errorf("unknown e2e scenario %q", s.options.Scenario)
	}

	provision, err := s.resolveE2ECheckinProvision(rt, definition)
	if err != nil {
		return err
	}

	deviceClient := NewClientWithAdapter(rt.Adapter, rt.Verbose)
	deviceClient.BindAuth(phoenixapi.AuthRef{
		Kind:  phoenixapi.AuthBearer,
		Label: "seed-e2e-device",
		Token: provision.deviceAPIKey,
	})

	if _, err := deviceClient.PostWithHeaders("/api/iot/session/start", map[string]any{
		"activity_id":    provision.activityID,
		"room_id":        provision.roomID,
		"supervisor_ids": []int64{provision.supervisorStaffID},
		"force":          true,
	}, map[string]string{
		"X-Staff-PIN": rt.StaffPIN,
	}); err != nil {
		return fmt.Errorf("start e2e device session: %w", err)
	}

	assignPath := fmt.Sprintf("/api/students/%d/rfid", provision.studentID)
	if _, err := deviceClient.PostWithHeaders(assignPath, map[string]any{
		"rfid_tag": provision.studentRFIDTag,
	}, map[string]string{
		"X-Staff-PIN": rt.StaffPIN,
	}); err != nil {
		return fmt.Errorf("assign e2e RFID tag: %w", err)
	}

	sickAssignPath := fmt.Sprintf("/api/students/%d/rfid", provision.sickStudentID)
	if _, err := deviceClient.PostWithHeaders(sickAssignPath, map[string]any{
		"rfid_tag": provision.sickRFIDTag,
	}, map[string]string{
		"X-Staff-PIN": rt.StaffPIN,
	}); err != nil {
		return fmt.Errorf("assign sick-present E2E RFID tag: %w", err)
	}

	checkinRaw, err := deviceClient.PostWithHeaders("/api/iot/checkin", map[string]any{
		"student_rfid": provision.sickRFIDTag,
		"action":       "checkin",
		"room_id":      provision.roomID,
	}, map[string]string{
		"X-Staff-PIN": rt.StaffPIN,
	})
	if err != nil {
		return fmt.Errorf("prepare sick-present e2e student: %w", err)
	}

	var checkinResp struct {
		Data struct {
			StudentID int64  `json:"student_id"`
			Action    string `json:"action"`
		} `json:"data"`
	}
	if err := json.Unmarshal(checkinRaw, &checkinResp); err != nil {
		return fmt.Errorf("parse sick-present e2e checkin response: %w", err)
	}
	if checkinResp.Data.StudentID != provision.sickStudentID {
		return fmt.Errorf(
			"prepare sick-present e2e student: expected student %d, got %d",
			provision.sickStudentID,
			checkinResp.Data.StudentID,
		)
	}
	if checkinResp.Data.Action != "checked_in" {
		return fmt.Errorf(
			"prepare sick-present e2e student: expected checked_in action, got %q",
			checkinResp.Data.Action,
		)
	}

	if _, err := rt.Client.Put(fmt.Sprintf("/api/students/%d", provision.sickStudentID), map[string]any{
		"sick":    true,
		"excused": false,
	}); err != nil {
		return fmt.Errorf("mark sick-present e2e student sick: %w", err)
	}

	rt.FixedSeeder.studentRFID[provision.studentID] = provision.studentRFIDTag
	rt.FixedSeeder.studentRFID[provision.sickStudentID] = provision.sickRFIDTag
	return nil
}

func (s *Seeder) resolveE2ECheckinProvision(rt *Runtime, definition scenarios.Definition) (e2eCheckinProvision, error) {
	if rt == nil || rt.FixedSeeder == nil {
		return e2eCheckinProvision{}, fmt.Errorf("e2e checkin fixture requires fixed seeder state")
	}
	if rt.Adapter == nil {
		return e2eCheckinProvision{}, fmt.Errorf("e2e checkin fixture requires adapter")
	}
	if rt.StaffPIN == "" {
		return e2eCheckinProvision{}, fmt.Errorf("e2e checkin fixture requires staff PIN")
	}

	studentID, err := lookupSeededStudentID(rt.FixedSeeder, definition.Fixtures.Checkin.Student)
	if err != nil {
		return e2eCheckinProvision{}, err
	}
	sickStudentID, err := lookupSeededStudentID(
		rt.FixedSeeder,
		definition.Fixtures.Students.SickPresent.Student,
	)
	if err != nil {
		return e2eCheckinProvision{}, err
	}

	roomName := definition.Fixtures.Checkin.RoomName
	roomID, ok := rt.FixedSeeder.roomIDs[roomName]
	if !ok || roomID <= 0 {
		return e2eCheckinProvision{}, fmt.Errorf(
			`e2e checkin fixture missing room %q`,
			roomName,
		)
	}

	activityName := definition.Fixtures.Checkin.ActivityName
	activityID, ok := rt.FixedSeeder.activityIDs[activityName]
	if !ok || activityID <= 0 {
		return e2eCheckinProvision{}, fmt.Errorf(
			`e2e checkin fixture missing activity %q`,
			activityName,
		)
	}

	deviceKey := definition.Fixtures.Checkin.DeviceKey
	deviceAPIKey, ok := rt.FixedSeeder.deviceKeys[deviceKey]
	if !ok || deviceAPIKey == "" {
		return e2eCheckinProvision{}, fmt.Errorf(
			"e2e checkin fixture missing API key for %q",
			deviceKey,
		)
	}

	supervisorEmail, err := resolveE2EAdminEmail(rt.FixedSeeder, rt.SecondTenant)
	if err != nil {
		return e2eCheckinProvision{}, err
	}
	supervisorStaffID, err := resolveE2EStaffIDByEmail(rt.FixedSeeder, supervisorEmail)
	if err != nil {
		return e2eCheckinProvision{}, err
	}

	return e2eCheckinProvision{
		studentID:         studentID,
		studentRFIDTag:    definition.Fixtures.Checkin.RFIDTag,
		sickStudentID:     sickStudentID,
		sickRFIDTag:       definition.Fixtures.Students.SickPresent.RFIDTag,
		roomID:            roomID,
		activityID:        activityID,
		supervisorStaffID: supervisorStaffID,
		deviceAPIKey:      deviceAPIKey,
	}, nil
}

func lookupSeededStudentID(fs *FixedSeeder, ref scenarios.StudentNameRef) (int64, error) {
	if fs == nil {
		return 0, fmt.Errorf("e2e checkin fixture missing fixed seeder")
	}
	key := fmt.Sprintf("%s %s", ref.FirstName, ref.LastName)
	studentID, ok := fs.studentIDs[key]
	if !ok || studentID <= 0 {
		return 0, fmt.Errorf("e2e checkin fixture missing seeded student %q", key)
	}
	return studentID, nil
}

func resolveE2EAdminEmail(fs *FixedSeeder, secondTenant *SeedStateSecondTenant) (string, error) {
	cred, err := resolveE2EAdminCredential(fs, secondTenant)
	if err != nil {
		return "", fmt.Errorf("e2e checkin fixture %w", err)
	}
	return cred.Email, nil
}

func resolveE2EStaffIDByEmail(fs *FixedSeeder, email string) (int64, error) {
	if fs == nil {
		return 0, fmt.Errorf("e2e checkin fixture missing fixed seeder")
	}
	for _, cred := range fs.staffCredentials {
		if cred.Email != email {
			continue
		}
		staffID, ok := fs.staffIDs[cred.Name]
		if !ok || staffID <= 0 {
			return 0, fmt.Errorf("e2e checkin fixture missing staff record for %q", email)
		}
		return staffID, nil
	}
	return 0, fmt.Errorf("e2e checkin fixture could not resolve staff account %q", email)
}

type provisionE2ECheckinStep struct {
	seeder *Seeder
}

func (provisionE2ECheckinStep) Name() string { return "Provisioning E2E check-in fixture" }

func (s provisionE2ECheckinStep) Run(ctx context.Context, rt *Runtime) error {
	return s.seeder.provisionE2ECheckinFixture(ctx, rt)
}
