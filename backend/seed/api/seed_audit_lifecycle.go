package api

import (
	"context"
	"fmt"
)

type seedAuditLifecycleStep struct{}

func (seedAuditLifecycleStep) Name() string { return "Seeding audited lifecycle changes" }

func (seedAuditLifecycleStep) Run(_ context.Context, rt *Runtime) error {
	if rt.FixedSeeder == nil {
		return fmt.Errorf("audit lifecycle prerequisites not available")
	}
	groupID := firstSeedGroupID(rt.FixedSeeder)
	if groupID == 0 {
		return fmt.Errorf("audit lifecycle group not available")
	}
	rt.Client.BindAuth(rt.TenantAuth)
	studentID, err := createDisposableStudent(rt, groupID)
	if err != nil {
		return err
	}
	if err := seedDisposableGuardianChange(rt, studentID); err != nil {
		return err
	}
	if err := deleteDisposableStudent(rt, studentID); err != nil {
		return err
	}
	fmt.Println("  1 student deletion and 1 guardian change audited")
	return nil
}

func firstSeedGroupID(fs *FixedSeeder) int64 {
	for _, id := range fs.groupIDs {
		return id
	}
	return 0
}

func createDisposableStudent(rt *Runtime, groupID int64) (int64, error) {
	studentRaw, err := rt.Client.Post("/api/students", map[string]any{
		"first_name": "Löschdemo", "last_name": "Kind", "school_class": "Klasse 2b",
		"group_id": groupID, "birthday": "2018-02-14", "pickup_status": "pickup",
	})
	if err != nil {
		return 0, fmt.Errorf("create disposable student: %w", err)
	}
	studentID, err := parseEnvelopeStringID(studentRaw)
	if err != nil {
		return 0, fmt.Errorf("parse disposable student: %w", err)
	}
	return studentID, nil
}

func seedDisposableGuardianChange(rt *Runtime, studentID int64) error {
	guardianRaw, err := rt.Client.Post("/api/guardians", map[string]any{
		"first_name": "Demo", "last_name": "Abholkontakt",
		"email": "demo.abholkontakt@example.test", "preferred_contact_method": "email",
		"language_preference": "de",
	})
	if err != nil {
		return fmt.Errorf("create disposable guardian: %w", err)
	}
	guardianID, err := parseEnvelopeStringID(guardianRaw)
	if err != nil {
		return fmt.Errorf("parse disposable guardian: %w", err)
	}
	if _, err := rt.Client.Post(fmt.Sprintf("/api/guardians/students/%d/guardians", studentID), map[string]any{
		"guardian_profile_id": guardianID, "relationship_type": "other", "guardian_role": "custom",
		"is_primary": false, "is_emergency_contact": false, "can_pickup": false, "emergency_priority": 1,
	}); err != nil {
		return fmt.Errorf("link disposable guardian: %w", err)
	}
	if _, err := rt.Client.Delete(fmt.Sprintf("/api/guardians/students/%d/guardians/%d", studentID, guardianID)); err != nil {
		return fmt.Errorf("unlink disposable guardian: %w", err)
	}
	return nil
}

func deleteDisposableStudent(rt *Runtime, studentID int64) error {
	impactRaw, err := rt.Client.Get(fmt.Sprintf("/api/students/%d/delete-impact", studentID))
	if err != nil {
		return fmt.Errorf("preview disposable student deletion: %w", err)
	}
	var impact struct {
		Data struct {
			ConfirmationName string `json:"confirmation_name"`
			Fingerprint      string `json:"fingerprint"`
		} `json:"data"`
	}
	if err := parseJSON(impactRaw, &impact); err != nil {
		return fmt.Errorf("parse disposable student deletion preview: %w", err)
	}
	return deleteDisposableStudentWithImpact(rt, studentID, impact.Data.ConfirmationName, impact.Data.Fingerprint)
}

func deleteDisposableStudentWithImpact(rt *Runtime, studentID int64, confirmationName, fingerprint string) error {
	if _, err := rt.Client.DeleteWithBody(fmt.Sprintf("/api/students/%d", studentID), map[string]any{
		"expected_fingerprint": fingerprint,
		"confirmation_name":    confirmationName,
		"reason":               "test_data",
		"acknowledged":         true,
	}); err != nil {
		return fmt.Errorf("delete disposable student: %w", err)
	}
	return nil
}
