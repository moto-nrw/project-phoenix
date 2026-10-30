package api

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// seedAccidentReportsStep records three accident reports (#4771) so the
// Dokumente tab shows every state on a fresh stack: an open report, a signed
// one, and a signed one with its open correction.
type seedAccidentReportsStep struct{}

func (seedAccidentReportsStep) Name() string { return "Seeding accident reports" }

// demoAccidentReports are the accidents of the first three demo children.
var demoAccidentReports = []struct {
	daysAgo     int
	hour        int
	location    string
	description string
	bodyParts   string
	injury      string
	firstAid    string
	doctor      string
	witnesses   string
	supervisor  string
	sign        bool
	correction  string
}{
	{
		daysAgo: 1, hour: 14, location: "Schulhof, am Klettergerüst",
		description: "Beim Klettern mit der Hand abgerutscht und aus etwa einem Meter Höhe auf den Rasen gefallen. Das Kind hat danach über Schmerzen am linken Handgelenk geklagt.",
		bodyParts:   "Linkes Handgelenk", injury: "Prellung", firstAid: "Gekühlt und ruhiggestellt",
		witnesses: "Zwei Kinder der Gruppe", supervisor: "Aufsicht Schulhof",
	},
	{
		daysAgo: 6, hour: 15, location: "Turnhalle, Bewegungsangebot",
		description: "Beim Fangenspielen mit einem anderen Kind zusammengestoßen und mit dem Kopf gegen dessen Schulter gestoßen.",
		bodyParts:   "Stirn, rechte Seite", injury: "Beule", firstAid: "Gekühlt, eine halbe Stunde beobachtet",
		doctor: "Kinderarztpraxis am Markt", witnesses: "Leitung des Bewegungsangebots", supervisor: "Leitung des Bewegungsangebots", sign: true,
	},
	{
		daysAgo: 9, hour: 13, location: "Mensa",
		description: "Beim Tragen des Tabletts gestolpert, das Glas ist zerbrochen und hat das Kind an der Hand geschnitten.",
		bodyParts:   "Rechter Zeigefinger", injury: "Schnittverletzung", firstAid: "Wunde gereinigt, Pflaster",
		witnesses: "Küchenpersonal", supervisor: "Aufsicht Mensa", sign: true,
		correction: "Wunde gereinigt, Pflaster und Fingerverband",
	},
}

func (seedAccidentReportsStep) Run(_ context.Context, rt *Runtime) error {
	if rt.FixedSeeder == nil {
		return fmt.Errorf("fixed seeder not available")
	}
	today := calendar.TodayDate()
	for index, accident := range demoAccidentReports {
		studentID, ok := rt.FixedSeeder.studentIDByIndex[index]
		if !ok {
			return fmt.Errorf("student ID not available for accident report index %d", index)
		}
		occurredAt := today.AddDays(-accident.daysAgo).BerlinMidnight().Add(time.Duration(accident.hour) * time.Hour)
		notifiedAt := occurredAt.Add(20 * time.Minute)
		body := map[string]any{
			"occurred_at": occurredAt.Format(time.RFC3339), "location": accident.location, "description": accident.description,
			"injured_body_parts": accident.bodyParts, "injury_kind": accident.injury, "witnesses": accident.witnesses,
			"first_aid": accident.firstAid, "doctor_visited": accident.doctor != "", "doctor": accident.doctor,
			"parents_notified": "Mutter, telefonisch", "parents_notified_at": notifiedAt.Format(time.RFC3339),
			"supervisor": accident.supervisor,
		}
		reportID, err := postAccidentReport(rt, fmt.Sprintf("/api/students/%d/accident-reports", studentID), body)
		if err != nil {
			return err
		}
		if !accident.sign {
			continue
		}
		if _, err := rt.Client.Post(fmt.Sprintf("/api/students/%d/accident-reports/%d/signature", studentID, reportID), map[string]any{
			"method": "drawn", "image_png": demoSignaturePNG("Leitung"),
		}); err != nil {
			return fmt.Errorf("sign accident report of student %d: %w", studentID, err)
		}
		if accident.correction == "" {
			continue
		}
		body["first_aid"] = accident.correction
		if _, err := postAccidentReport(rt, fmt.Sprintf("/api/students/%d/accident-reports/%d/corrections", studentID, reportID), body); err != nil {
			return err
		}
	}
	fmt.Printf("  %d accident reports recorded\n", len(demoAccidentReports))
	return nil
}

func postAccidentReport(rt *Runtime, path string, body map[string]any) (int64, error) {
	raw, err := rt.Client.Post(path, body)
	if err != nil {
		return 0, fmt.Errorf("record accident report %s: %w", path, err)
	}
	var envelope struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil || envelope.Data.ID <= 0 {
		return 0, fmt.Errorf("decode accident report %s: %w", path, err)
	}
	return envelope.Data.ID, nil
}
