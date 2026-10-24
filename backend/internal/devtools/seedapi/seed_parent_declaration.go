package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"time"
)

// seedParentDeclarationStep runs after the parent letter: the Erklärung
// (#3430) reaches the same demo guardians.
type seedParentDeclarationStep struct{}

func (seedParentDeclarationStep) Name() string { return "Seeding parent declaration" }

func (seedParentDeclarationStep) Run(_ context.Context, rt *Runtime) error {
	rt.Client.BindAuth(rt.TenantAuth)
	return seedParentDeclaration(rt)
}

// seedDeclarationTitle names the seeded Erklärung; the parent step finds it
// in the feed by it.
const seedDeclarationTitle = "Einverständnis: Ausflug in den Zoo"

// seedParentDeclaration publishes an Erklärung (#3430) with a PDF and lets
// the first demo parent consent for one child, so the staff status, the
// proof and the parent view all have something to show on every machine.
func seedParentDeclaration(rt *Runtime) error {
	deadline := time.Now().AddDate(0, 0, 14)
	raw, err := rt.Client.Post("/api/parent-announcements/", map[string]any{
		"title":                         seedDeclarationTitle,
		"body":                          "Liebe Eltern, am Freitag fahren wir mit dem Bus in den Zoo. Wir sind um 16:00 Uhr zurück an der Schule. Bitte geben Sie an, ob Ihr Kind mitfahren darf.",
		"priority":                      "important",
		"delivery_mode":                 "declaration",
		"declaration_kind":              "consent",
		"declaration_signers":           "any",
		"declaration_revocable":         true,
		"declaration_requires_password": false,
		// Agreeing needs a drawn signature (#4785), so the proof, the report
		// and the parent view show one on every machine.
		"declaration_requires_signature": true,
		"response_deadline":              deadline.Format(time.RFC3339),
		"send_email":                     true,
		"targets":                        []map[string]any{{"target_type": "school_all"}},
	})
	if err != nil {
		return fmt.Errorf("create parent declaration: %w", err)
	}
	id, err := parseEnvelopeStringID(raw)
	if err != nil {
		return fmt.Errorf("parse parent declaration response: %w", err)
	}
	// The document belongs to the frozen version, so it is attached before
	// the first publication (after that the attachments are fixed).
	if _, err := rt.Client.PostFile(
		fmt.Sprintf("/api/announcement-attachments/%d", id), "file", "Einverständnis Zoo.pdf",
		demoPDF("Einverständnis Ausflug in den Zoo", []string{
			"Abfahrt: Freitag, 8:30 Uhr an der Schule.",
			"Rückkehr: 16:00 Uhr an der Schule.",
			"Bitte geben Sie Ihrem Kind etwas zu trinken mit.",
		}),
	); err != nil {
		return fmt.Errorf("attach file to parent declaration: %w", err)
	}
	if _, err := rt.Client.Post(fmt.Sprintf("/api/parent-announcements/%d/publish", id), nil); err != nil {
		return fmt.Errorf("publish parent declaration: %w", err)
	}
	if len(rt.Parents) > 0 && rt.Adapter != nil {
		if err := seedDeclarationAnswer(rt); err != nil {
			return err
		}
	}
	fmt.Println("  1 parent declaration published")
	return nil
}

// demoSignaturePNG draws a handwritten-looking stroke as the canvas of the
// portal would send it: a transparent PNG with dark ink, base64 encoded.
func demoSignaturePNG() string {
	const width, height = 600, 180
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	ink := color.NRGBA{R: 23, G: 37, B: 84, A: 255}
	for step := 0; step <= 2400; step++ {
		t := float64(step) / 2400
		x := 40 + t*520
		y := 95 + 38*math.Sin(t*11) - 22*math.Cos(t*23) + 18*t
		for dx := -1; dx <= 1; dx++ {
			for dy := -1; dy <= 1; dy++ {
				img.Set(int(x)+dx, int(y)+dy, ink)
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		panic(fmt.Sprintf("encode demo signature: %v", err))
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

// seedDeclarationAnswer lets the first demo parent consent for their first
// child through the parent API, exactly as the portal does.
func seedDeclarationAnswer(rt *Runtime) error {
	defer rt.Client.BindAuth(rt.TenantAuth)
	parent := rt.Parents[0]
	auth, err := rt.Adapter.LoginParent(context.Background(), parent.Email, parent.Password)
	if err != nil {
		return fmt.Errorf("login parent to declare: %w", err)
	}
	feedRaw, err := rt.Client.GetWithAuth(auth, "/parent/me/news")
	if err != nil {
		return fmt.Errorf("load parent feed for declaration: %w", err)
	}
	var feed struct {
		Data []struct {
			ID          string `json:"id"`
			Title       string `json:"title"`
			Declaration *struct {
				Version struct {
					ID string `json:"id"`
				} `json:"version"`
				Children []struct {
					StudentID string `json:"student_id"`
					CanSubmit bool   `json:"can_submit"`
				} `json:"children"`
			} `json:"declaration"`
		} `json:"data"`
	}
	if err := json.Unmarshal(feedRaw, &feed); err != nil {
		return fmt.Errorf("parse parent feed for declaration: %w", err)
	}
	for _, item := range feed.Data {
		if item.Title != seedDeclarationTitle || item.Declaration == nil {
			continue
		}
		for _, child := range item.Declaration.Children {
			if !child.CanSubmit {
				continue
			}
			if _, err := rt.Client.PostWithAuth(auth, "/parent/me/news/"+item.ID+"/declaration", map[string]any{
				"student_id": child.StudentID, "action": "agreed", "version_id": item.Declaration.Version.ID,
				"signature": map[string]any{"method": "drawn", "image_png": demoSignaturePNG()},
			}); err != nil {
				return fmt.Errorf("declare consent: %w", err)
			}
			return nil
		}
	}
	return errors.New("seeded parent declaration is not answerable by the first demo parent")
}
