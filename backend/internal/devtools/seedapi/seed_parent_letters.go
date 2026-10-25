package api

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// seedParentLetterStep runs after parentEnrollmentSeedStep so its portal-only
// audience resolves the demo guardians and the delivery matrix is populated.
type seedParentLetterStep struct{}

func (seedParentLetterStep) Name() string { return "Seeding parent letter" }

func (seedParentLetterStep) Run(_ context.Context, rt *Runtime) error {
	rt.Client.BindAuth(rt.TenantAuth)
	return seedParentLetter(rt)
}

// seedParentLetterTitle names the Elternbrief; the product screenshot
// eltern-elternbrief-lesen opens it by this title.
const seedParentLetterTitle = "Elternbrief: Unser Laternenfest"

// seedParentLetter publishes the families' mail of the last weeks: three
// notices the first parent has read, the Elternbrief about the Laternenfest
// with its flyer, the announcement with a reminder, and a draft the team is
// still writing.
func seedParentLetter(rt *Runtime) error {
	reader, err := seedLetterReader(rt)
	if err != nil {
		return err
	}
	defer rt.Client.BindAuth(rt.TenantAuth)
	if err := seedEarlierNotices(rt, reader); err != nil {
		return err
	}
	festival := seedLanternFestivalDate(todaySeedDate())
	raw, err := rt.Client.Post("/api/parent-announcements/", map[string]any{
		"title":          seedParentLetterTitle,
		"body":           seedLanternLetterBody(festival),
		"priority":       "important",
		"delivery_mode":  "letter",
		"email_audience": "portal_only",
		"targets": []map[string]any{
			{"target_type": "school_all"},
		},
	})
	if err != nil {
		return fmt.Errorf("create parent letter: %w", err)
	}
	id, err := parseEnvelopeStringID(raw)
	if err != nil {
		return fmt.Errorf("parse parent letter response: %w", err)
	}
	// Anhang VOR dem Veröffentlichen: danach ist die Mitteilung unveränderlich
	// und das Backend lehnt jeden weiteren Anhang ab (#2890). Ohne diese Zeile
	// ist der Anhangsbereich auf jeder Entwicklungsmaschine leer, und genau der
	// Teil — was die Eltern am Ende herunterladen — lässt sich dann nirgends
	// ansehen.
	if _, err := rt.Client.PostFile(
		fmt.Sprintf("/api/announcement-attachments/%d", id),
		"file",
		"Laternenfest Ablauf und Lieder.pdf",
		demoIllustratedPDF("Unser Laternenfest", lanternPicture(), seedLanternFlyerLines(festival)),
	); err != nil {
		return fmt.Errorf("attach file to parent letter: %w", err)
	}
	if err := publishAndRead(rt, id, reader); err != nil {
		return fmt.Errorf("parent letter: %w", err)
	}
	fmt.Println("  3 read notices and 1 parent letter published")
	if err := seedReminderAnnouncement(rt); err != nil {
		return err
	}
	return seedDraftAnnouncement(rt)
}

// seedLetterReader signs in the first demo parent, who has read the letters
// by now. Without parents (a bare profile) nobody reads.
func seedLetterReader(rt *Runtime) (*AuthRef, error) {
	if len(rt.Parents) == 0 || rt.Adapter == nil {
		return nil, nil
	}
	parent := rt.Parents[0]
	auth, err := rt.Adapter.LoginParent(context.Background(), parent.Email, parent.Password)
	if err != nil {
		return nil, fmt.Errorf("login parent to read parent letter: %w", err)
	}
	return &auth, nil
}

// publishAndRead publishes an announcement and, with a reader, marks it read
// by them.
func publishAndRead(rt *Runtime, id int64, reader *AuthRef) error {
	publishedRaw, err := rt.Client.PostWithAuth(rt.TenantAuth, fmt.Sprintf("/api/parent-announcements/%d/publish", id), nil)
	if err != nil {
		return fmt.Errorf("publish: %w", err)
	}
	if reader == nil {
		return nil
	}
	publishedAt, err := parseEnvelopePublishedAt(publishedRaw)
	if err != nil {
		return fmt.Errorf("parse published announcement: %w", err)
	}
	if _, err := rt.Client.PostWithAuth(*reader, fmt.Sprintf("/parent/me/news/%d/read", id), map[string]any{
		"published_at": publishedAt,
	}); err != nil {
		return fmt.Errorf("mark read: %w", err)
	}
	return nil
}

// seedEarlierNotices publishes three everyday notices before the letter, so
// they sort below it, and lets the first parent read them: the parents
// portal shows a list with read and unread mail.
func seedEarlierNotices(rt *Runtime, reader *AuthRef) error {
	evening := seedParentsEveningDate(todaySeedDate())
	notices := []struct{ title, body string }{
		{"Betreuung in den Herbstferien", "Liebe Eltern, in den Herbstferien ist die OGS für angemeldete Kinder von 8:00 bis 15:00 Uhr geöffnet. " +
			"Bitte melden Sie Ihr Kind spätestens zwei Wochen vorher im OGS-Büro an."},
		{"Neuer Caterer für das Mittagessen", "Liebe Eltern, ab Montag kocht ein neuer Caterer für unsere Mensa. " +
			"Den Speiseplan sehen Sie wie immer in moto unter Mittagessen. Ein vegetarisches Gericht gibt es weiterhin jeden Tag."},
		{"Einladung zum Elternabend", fmt.Sprintf("Liebe Eltern, am %s um 19:00 Uhr laden wir Sie herzlich zum Elternabend ein. "+
			"Wir stellen den Tagesablauf vor und planen die Wochen bis zu den Ferien. Bitte sagen Sie im Kalender zu oder ab.", germanDate(evening))},
	}
	for _, n := range notices {
		raw, err := rt.Client.PostWithAuth(rt.TenantAuth, "/api/parent-announcements/", map[string]any{
			"title": n.title, "body": n.body, "priority": "info", "send_email": false,
			"targets": []map[string]any{{"target_type": "school_all"}},
		})
		if err != nil {
			return fmt.Errorf("create notice %s: %w", n.title, err)
		}
		id, err := parseEnvelopeStringID(raw)
		if err != nil {
			return fmt.Errorf("parse notice %s: %w", n.title, err)
		}
		if err := publishAndRead(rt, id, reader); err != nil {
			return fmt.Errorf("notice %s: %w", n.title, err)
		}
	}
	return nil
}

func seedLanternLetterBody(festival seedDate) string {
	return fmt.Sprintf("Liebe Eltern,\n\n"+
		"am %s feiern wir unser Laternenfest. Wir treffen uns um 17:00 Uhr auf dem Schulhof. "+
		"Dann ziehen wir mit den Laternen durch das Viertel. Zum Schluss gibt es Kinderpunsch und Brezeln.\n\n"+
		"Die Laternen basteln die Kinder in den nächsten Wochen in der OGS. "+
		"Bitte geben Sie Ihrem Kind dafür ein leeres Marmeladenglas mit.\n\n"+
		"Geschwister und Großeltern sind herzlich willkommen. Den Ablauf und die Liedtexte finden Sie im Anhang. "+
		"Bitte bestätigen Sie kurz, dass Sie den Brief gelesen haben.\n\n"+
		"Viele Grüße\nIhr OGS-Team", germanDate(festival))
}

func seedLanternFlyerLines(festival seedDate) []string {
	return []string{
		"Wann: " + germanDate(festival) + ", 17:00 bis 19:00 Uhr",
		"Treffpunkt: Schulhof",
		"17:00 Uhr: Wir treffen uns und zünden die Laternen an.",
		"17:30 Uhr: Laternenumzug durch das Viertel",
		"18:15 Uhr: Kinderpunsch und Brezeln am Feuerkorb",
		"",
		"Laterne, Laterne, Sonne, Mond und Sterne.",
		"Brenne auf, mein Licht, brenne auf, mein Licht,",
		"aber nur meine liebe Laterne nicht.",
		"",
		"Ich geh mit meiner Laterne und meine Laterne mit mir.",
		"Dort oben leuchten die Sterne, und unten, da leuchten wir.",
	}
}

// seedReminderAnnouncement publishes the announcement the reminder feature
// (#3162) was asked for: written early so families can plan, reminded the
// morning before it matters. Without it the reminder fields are empty on
// every development machine and nobody ever sees the "Erinnerung" state.
func seedReminderAnnouncement(rt *Runtime) error {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		return fmt.Errorf("load Berlin location: %w", err)
	}
	now := time.Now().In(berlin)
	// Tomorrow 08:00 Berlin: in the future on every machine, so the create
	// validation accepts it, and the tick delivers it the next morning.
	reminderAt := time.Date(now.Year(), now.Month(), now.Day()+1, 8, 0, 0, 0, berlin)
	raw, err := rt.Client.Post("/api/parent-announcements/", map[string]any{
		"title":         "Betreuung endet am letzten Schultag um 13:00 Uhr",
		"body":          "Liebe Eltern, am letzten Schultag vor den Ferien endet die Betreuung bereits um 13:00 Uhr. Bitte holen Sie Ihr Kind bis dahin ab oder geben Sie uns Bescheid, wenn es allein nach Hause gehen darf.",
		"priority":      "info",
		"send_email":    true,
		"reminder_at":   reminderAt.Format(time.RFC3339),
		"reminder_text": "Morgen endet die Betreuung um 13:00 Uhr. Bitte holen Sie Ihr Kind bis dahin ab.",
		"targets": []map[string]any{
			{"target_type": "school_all"},
		},
	})
	if err != nil {
		return fmt.Errorf("create reminder announcement: %w", err)
	}
	id, err := parseEnvelopeStringID(raw)
	if err != nil {
		return fmt.Errorf("parse reminder announcement response: %w", err)
	}
	if _, err := rt.Client.Post(fmt.Sprintf("/api/parent-announcements/%d/publish", id), nil); err != nil {
		return fmt.Errorf("publish reminder announcement: %w", err)
	}
	fmt.Println("  1 parent announcement with scheduled reminder published")
	return nil
}

// seedDraftAnnouncement leaves a notice unpublished, so the staff list shows
// a draft next to the sent mail. The API has no scheduled publication.
func seedDraftAnnouncement(rt *Runtime) error {
	if _, err := rt.Client.Post("/api/parent-announcements/", map[string]any{
		"title": "Weihnachtsbasteln: Wer hilft mit?",
		"body": "Liebe Eltern, am letzten Freitag vor den Weihnachtsferien basteln wir mit den Kindern Sterne und Geschenkanhänger. " +
			"Wer hat Zeit und hilft am Nachmittag mit? Bitte melden Sie sich bei uns.",
		"priority": "info", "send_email": false,
		"targets": []map[string]any{{"target_type": "school_all"}},
	}); err != nil {
		return fmt.Errorf("create draft announcement: %w", err)
	}
	fmt.Println("  1 draft announcement saved")
	return nil
}

func parseEnvelopePublishedAt(raw []byte) (time.Time, error) {
	var envelope struct {
		Data struct {
			PublishedAt time.Time `json:"published_at"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return time.Time{}, err
	}
	if envelope.Data.PublishedAt.IsZero() {
		return time.Time{}, fmt.Errorf("published_at missing")
	}
	return envelope.Data.PublishedAt, nil
}
