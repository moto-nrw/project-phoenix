package api

import (
	"context"
	"fmt"
)

// seedStaffMessagingStep fills the OGS-internal Team-Chat (#2598) with two demo
// conversations.
//
// Without it the four users.staff_message_* tables stay empty on every dev
// machine, the Team-Chat screen looks broken to anyone reviewing it, and
// TestSeedCoverageRatchet fails the build for the unlisted empty tables.
//
// The feature defaults OFF, so the step switches it on for the seeded school
// first — otherwise every send below would 403.
type seedStaffMessagingStep struct{}

func (seedStaffMessagingStep) Name() string { return "Seeding staff messages" }

func (seedStaffMessagingStep) Run(_ context.Context, rt *Runtime) error {
	// rt.State is only assembled by buildStateStep at the very end of the
	// workflow, so the credentials have to come from the fixed seeder - same
	// source seedTimeTrackingHistoryStep uses.
	if rt.FixedSeeder == nil {
		return fmt.Errorf("fixed seeder not available")
	}
	staff, _ := buildStaffOrder(rt.FixedSeeder)
	// Drei Konten, weil die zweite Unterhaltung ein ANDERES Paar braucht:
	// GetOrCreateDirect normalisiert das Paar (sortierter participant_key), ein
	// bloss umgedrehtes from/to landet also im selben Thread - dann saet dieser
	// Schritt nur eine Unterhaltung statt zweier.
	if len(staff) < 3 {
		return fmt.Errorf("staff messages require at least three staff accounts, got %d", len(staff))
	}

	// The Team-Chat is opt-in (operations.staff_messaging_enabled, default off).
	// Turn it on with admin auth before writing anything.
	rt.Client.BindAuth(rt.TenantAuth)
	defer rt.Client.BindAuth(rt.TenantAuth)
	if _, err := rt.Client.Put(
		"/api/settings/values/operations.staff_messaging_enabled",
		map[string]any{"value": true},
	); err != nil {
		return fmt.Errorf("enable Team-Chat: %w", err)
	}

	// Two conversations, each opened by a different person, so the demo data
	// shows both an unread and an already-answered thread.
	conversations := []struct {
		from     StaffCredentials
		to       StaffCredentials
		messages []string
	}{
		{
			from: staff[0],
			to:   staff[1],
			messages: []string{
				"Kannst du heute die 2a übernehmen? Ich bin bis 14 Uhr im Elterngespräch.",
				"Danke dir! Die Liste liegt im Gruppenraum.",
			},
		},
		{
			from: staff[1],
			to:   staff[2],
			messages: []string{
				"Die Materialliste für die Bastel-AG liegt im OGS-Büro.",
			},
		},
	}

	// Wer welche Unterhaltung danach oeffnet. Ohne dieses zweite Durchgehen
	// entstehen KEINE Zeilen in users.staff_message_reads: Lesecursor schreibt
	// allein GetThread, und der Sendepfad tut es bewusst nicht mehr (er wuerde
	// sonst die Nachrichten der Gegenseite mit als gelesen markieren, #2598).
	type openedThread struct {
		id     string
		reader StaffCredentials
	}
	var opened []openedThread

	sent := 0
	for _, c := range conversations {
		recipientID, err := staffAccountID(rt, c.to)
		if err != nil {
			return err
		}
		if err := rt.Client.Login(c.from.Email, c.from.Password); err != nil {
			return fmt.Errorf("login as staff message sender %s: %w", c.from.Email, err)
		}

		raw, err := rt.Client.Post(
			"/api/staff-messages/threads/open",
			map[string]any{"account_id": fmt.Sprintf("%d", recipientID)},
		)
		if err != nil {
			return fmt.Errorf("open staff conversation: %w", err)
		}
		threadID, err := threadIDFromResponse(raw)
		if err != nil {
			return fmt.Errorf("decode POST /api/staff-messages/threads/open response: %w", err)
		}

		for _, body := range c.messages {
			if _, err := rt.Client.Post(
				"/api/staff-messages/threads/"+threadID,
				map[string]any{"body": body},
			); err != nil {
				return fmt.Errorf("send staff message to thread %s: %w", threadID, err)
			}
			sent++
		}

		// Die Gegenseite liest spaeter - das ist der realistische Ablauf und
		// zugleich der einzige Weg, wie ein Lesecursor entsteht.
		opened = append(opened, openedThread{id: threadID, reader: c.to})
	}

	// Zweiter Durchgang: jede Empfaengerin oeffnet ihre Unterhaltung einmal.
	read := 0
	for _, o := range opened {
		if err := rt.Client.Login(o.reader.Email, o.reader.Password); err != nil {
			return fmt.Errorf("login as staff message reader %s: %w", o.reader.Email, err)
		}
		if _, err := rt.Client.Get("/api/staff-messages/threads/" + o.id); err != nil {
			return fmt.Errorf("read staff conversation %s: %w", o.id, err)
		}
		read++
	}

	fmt.Printf("  %d staff messages created, %d conversations read\n", sent, read)
	return nil
}

// staffAccountID resolves the recipient's account id from the Team-Chat's own
// recipient endpoint, matched on the display name the seeder knows. The seed
// state carries staff ids, not account ids, and the chat addresses accounts.
func staffAccountID(rt *Runtime, cred StaffCredentials) (int64, error) {
	body, err := rt.Client.Get("/api/staff-messages/recipients")
	if err != nil {
		return 0, fmt.Errorf("list staff message recipients: %w", err)
	}
	var envelope struct {
		Data []struct {
			AccountID string `json:"account_id"`
			Name      string `json:"name"`
		} `json:"data"`
	}
	if err := parseJSON(body, &envelope); err != nil {
		return 0, fmt.Errorf("decode GET /api/staff-messages/recipients response: %w", err)
	}
	for _, row := range envelope.Data {
		if row.Name != cred.Name {
			continue
		}
		var id int64
		if _, err := fmt.Sscanf(row.AccountID, "%d", &id); err != nil {
			return 0, fmt.Errorf("decode account id for staff message recipient %s: %w", cred.Email, err)
		}
		return id, nil
	}
	return 0, fmt.Errorf("staff message recipient account not found for %s", cred.Email)
}

// threadIDFromResponse pulls thread_id out of the {status,data,message} envelope.
func threadIDFromResponse(body []byte) (string, error) {
	var envelope struct {
		Data struct {
			ThreadID string `json:"thread_id"`
		} `json:"data"`
	}
	if err := parseJSON(body, &envelope); err != nil {
		return "", err
	}
	if envelope.Data.ThreadID == "" {
		return "", fmt.Errorf("thread_id missing")
	}
	return envelope.Data.ThreadID, nil
}
