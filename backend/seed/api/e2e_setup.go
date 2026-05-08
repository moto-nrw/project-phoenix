package api

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/integration/phoenixapi"
)

const e2eScenarioCheckinRFIDTag = "E2EFE110001"

type e2eCheckinProvision struct {
	studentID         int64
	roomID            int64
	activityID        int64
	supervisorStaffID int64
	deviceAPIKey      string
	rfidTag           string
}

func (s *Seeder) provisionE2ECheckinFixture(ctx context.Context, rt *Runtime) error {
	provision, err := s.resolveE2ECheckinProvision(rt)
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
		"rfid_tag": provision.rfidTag,
	}, map[string]string{
		"X-Staff-PIN": rt.StaffPIN,
	}); err != nil {
		return fmt.Errorf("assign e2e RFID tag: %w", err)
	}

	rt.FixedSeeder.studentRFID[provision.studentID] = provision.rfidTag
	return nil
}

func (s *Seeder) resolveE2ECheckinProvision(rt *Runtime) (e2eCheckinProvision, error) {
	if rt == nil || rt.FixedSeeder == nil {
		return e2eCheckinProvision{}, fmt.Errorf("e2e checkin fixture requires fixed seeder state")
	}
	if rt.Adapter == nil {
		return e2eCheckinProvision{}, fmt.Errorf("e2e checkin fixture requires adapter")
	}
	if rt.StaffPIN == "" {
		return e2eCheckinProvision{}, fmt.Errorf("e2e checkin fixture requires staff PIN")
	}

	studentID, err := lookupSeededStudentID(rt.FixedSeeder, e2eScenarioCheckinStudent)
	if err != nil {
		return e2eCheckinProvision{}, err
	}

	roomID, ok := rt.FixedSeeder.roomIDs[e2eScenarioCheckinRoomName]
	if !ok || roomID <= 0 {
		return e2eCheckinProvision{}, fmt.Errorf(
			`e2e checkin fixture missing room %q`,
			e2eScenarioCheckinRoomName,
		)
	}

	activityID, ok := rt.FixedSeeder.activityIDs[e2eScenarioCheckinActivityName]
	if !ok || activityID <= 0 {
		return e2eCheckinProvision{}, fmt.Errorf(
			`e2e checkin fixture missing activity %q`,
			e2eScenarioCheckinActivityName,
		)
	}

	deviceAPIKey, ok := rt.FixedSeeder.deviceKeys[e2eScenarioCheckinDeviceKey]
	if !ok || deviceAPIKey == "" {
		return e2eCheckinProvision{}, fmt.Errorf(
			"e2e checkin fixture missing API key for %q",
			e2eScenarioCheckinDeviceKey,
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
		roomID:            roomID,
		activityID:        activityID,
		supervisorStaffID: supervisorStaffID,
		deviceAPIKey:      deviceAPIKey,
		rfidTag:           e2eScenarioCheckinRFIDTag,
	}, nil
}

func lookupSeededStudentID(fs *FixedSeeder, ref namedStudentRef) (int64, error) {
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
