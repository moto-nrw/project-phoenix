package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"strconv"
	"time"
)

// seedEnrollmentSignatureStep seeds an enrollment with the parent's
// signature (#4786): a holiday phase whose form requires the signature and
// one family that signed it, so the staff detail and the status link show
// the signature and its proof.
type seedEnrollmentSignatureStep struct{}

func (seedEnrollmentSignatureStep) Name() string { return "Seeding signed enrollment" }

func (seedEnrollmentSignatureStep) Run(_ context.Context, rt *Runtime) error {
	if rt.Bootstrap == nil {
		return nil
	}
	auth := rt.TenantAuth
	raw, err := rt.Client.PostWithAuth(auth, "/api/enrollment/schema/", map[string]any{
		"name": "Ferienbetreuung mit Unterschrift",
		"fields": []map[string]any{
			{"key": "allergies", "label": "Allergien und Unverträglichkeiten", "type": "textarea", "sort_order": 10},
		},
		"core_requirements": map[string]bool{"signature": true},
	})
	if err != nil {
		return fmt.Errorf("create signed enrollment form: %w", err)
	}
	schemaID, err := parseEnvelopeStringID(raw)
	if err != nil {
		return fmt.Errorf("parse signed enrollment form: %w", err)
	}
	phaseID, err := createSignedEnrollmentPhase(rt, auth, schemaID)
	if err != nil {
		return err
	}
	signature, err := seedSignaturePNG()
	if err != nil {
		return err
	}
	body := map[string]any{
		"phase_id":            phaseID,
		"guardian_first_name": "Sabine",
		"guardian_last_name":  "Wolff",
		"guardian_email":      "sabine.wolff@example.test",
		"guardian_phone":      "+49 221 555 734",
		"consent_flags":       map[string]any{"data_processing": true},
		"children": []map[string]any{{
			"first_name": "Lotta", "last_name": "Wolff", "date_of_birth": "2018-03-21", "target_grade_level": 2,
			"custom_data": map[string]any{"allergies": "Nüsse"},
		}},
		"signature": map[string]any{"method": "drawn", "image_png": signature},
	}
	rt.Client.BindAuth(rt.TenantAuth)
	if _, err := rt.Client.PostPublicWithHeaders("/api/enrollment/"+rt.Bootstrap.TenantSlug+"/submit", body, publicEnrollmentSeedHeaders(91)); err != nil {
		return fmt.Errorf("submit signed enrollment: %w", err)
	}
	fmt.Println("Seeded 1 enrollment request with a signature")
	return nil
}

// createSignedEnrollmentPhase opens a holiday phase over the next summer
// that uses the signed form.
func createSignedEnrollmentPhase(rt *Runtime, auth AuthRef, schemaID int64) (int64, error) {
	now := time.Now().UTC()
	start := time.Date(now.Year()+1, time.July, 6, 0, 0, 0, 0, time.UTC)
	body := map[string]any{
		"name":                         fmt.Sprintf("Sommerferien %d mit Unterschrift", start.Year()),
		"kind":                         "holiday",
		"service_start_date":           start.Format("2006-01-02"),
		"service_end_date":             start.AddDate(0, 0, 25).Format("2006-01-02"),
		"enrollment_open_at":           now.Add(-24 * time.Hour).Format(time.RFC3339),
		"enrollment_close_at":          now.AddDate(0, 2, 0).Format(time.RFC3339),
		"show_status_reason_to_parent": true,
		"care_overflow_mode":           "waitlist",
		"care_offering_selection_mode": "optional",
		"audience":                     "open",
		"is_active":                    true,
		"form_schema_id":               strconv.FormatInt(schemaID, 10),
	}
	raw, err := rt.Client.PostWithAuth(auth, "/api/enrollment/phases", body)
	if err != nil {
		return 0, fmt.Errorf("create signed enrollment phase: %w", err)
	}
	id, err := parseEnvelopeStringID(raw)
	if err != nil {
		return 0, fmt.Errorf("parse signed enrollment phase: %w", err)
	}
	return id, nil
}

// seedSignaturePNG draws a short signature-like stroke as base64 PNG.
func seedSignaturePNG() (string, error) {
	img := image.NewNRGBA(image.Rect(0, 0, 480, 160))
	ink := color.NRGBA{R: 17, G: 24, B: 39, A: 255}
	for x := 30; x < 450; x++ {
		y := 80 + int(28*math.Sin(float64(x)/22)) + int(10*math.Sin(float64(x)/7))
		for dy := -1; dy <= 1; dy++ {
			img.Set(x, y+dy, ink)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", fmt.Errorf("encode seed signature: %w", err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}
