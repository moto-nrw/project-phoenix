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
// The Team-Chat is on by default (opt-out since #3254), so the step relies on
// the registry default instead of writing an override. If that default ever
// flips back, every send below fails with 403 and the seeder stops loudly.
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

	// The deferred bind hands the admin session back to later steps.
	defer rt.Client.BindAuth(rt.TenantAuth)

	conversations := staffChatConversations(staff)

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
		// Look the recipient up as the sender: the list leaves out the caller,
		// and the previous sender may be this recipient.
		if err := rt.Client.Login(c.from.Email, c.from.Password); err != nil {
			return fmt.Errorf("login as staff message sender %s: %w", c.from.Email, err)
		}
		recipientID, err := staffAccountID(rt, c.to)
		if err != nil {
			return err
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

type staffChatConversation struct {
	from, to StaffCredentials
	messages []string
}

// staffChatConversations are the demo team chats, each opened by a different
// person, so the data shows unread and already-answered threads. The second
// staff member (the admin of the product screenshots) takes part in most.
func staffChatConversations(staff []StaffCredentials) []staffChatConversation {
	conversations := []staffChatConversation{
		{staff[0], staff[1], []string{
			"Kannst du heute die 2a übernehmen? Ich bin bis 14 Uhr im Elterngespräch.",
			"Danke dir! Die Liste liegt im Gruppenraum.",
		}},
		{staff[1], staff[2], []string{"Die Materialliste für die Bastel-AG liegt im OGS-Büro."}},
	}
	if len(staff) >= 5 {
		conversations = append(conversations,
			staffChatConversation{staff[3], staff[1], []string{
				"Ich bin morgen erst ab 11 Uhr da, ich habe einen Arzttermin. Die Frühbetreuung übernimmt Sabine.",
			}},
			staffChatConversation{staff[1], staff[4], []string{
				"Denkst du an die Laternen für Freitag? Der Kleber ist fast leer.",
				"Ich bringe morgen zwei neue Tuben mit.",
			}},
		)
	}
	return conversations
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
