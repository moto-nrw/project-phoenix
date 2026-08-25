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
		fmt.Println("  WARNING: no fixed seeder, skipping staff messages")
		return nil
	}
	staff, _ := buildStaffOrder(rt.FixedSeeder)
	if len(staff) < 2 {
		fmt.Println("  WARNING: fewer than two staff accounts, skipping staff messages")
		return nil
	}

	// The Team-Chat is opt-in (operations.staff_messaging_enabled, default off).
	// Turn it on with admin auth before writing anything.
	rt.Client.BindAuth(rt.TenantAuth)
	if _, err := rt.Client.Put(
		"/api/settings/values/operations.staff_messaging_enabled",
		map[string]any{"value": true},
	); err != nil {
		fmt.Printf("  WARNING: could not enable the Team-Chat, skipping staff messages: %v\n", err)
		return nil
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
			from:     staff[1],
			to:       staff[0],
			messages: []string{"Alles klar, ich mache das."},
		},
	}

	sent := 0
	for _, c := range conversations {
		recipientID := staffAccountID(rt, c.to)
		if recipientID == 0 {
			fmt.Printf("  WARNING: no account id for %s, skipping conversation\n", c.to.Email)
			continue
		}
		if err := rt.Client.Login(c.from.Email, c.from.Password); err != nil {
			fmt.Printf("  WARNING: login as %s failed: %v\n", c.from.Email, err)
			continue
		}

		raw, err := rt.Client.Post(
			"/api/staff-messages/threads/open",
			map[string]any{"account_id": fmt.Sprintf("%d", recipientID)},
		)
		if err != nil {
			fmt.Printf("  WARNING: could not open a staff conversation: %v\n", err)
			continue
		}
		threadID := threadIDFromResponse(raw)
		if threadID == "" {
			fmt.Println("  WARNING: staff conversation response carried no thread id")
			continue
		}

		for _, body := range c.messages {
			if _, err := rt.Client.Post(
				"/api/staff-messages/threads/"+threadID,
				map[string]any{"body": body},
			); err != nil {
				fmt.Printf("  WARNING: could not send a staff message: %v\n", err)
				continue
			}
			sent++
		}
	}

	// Restore the tenant auth the surrounding workflow expects.
	rt.Client.BindAuth(rt.TenantAuth)
	fmt.Printf("  %d staff messages created\n", sent)
	return nil
}

// staffAccountID resolves the recipient's account id from the Team-Chat's own
// recipient endpoint, matched on the display name the seeder knows. The seed
// state carries staff ids, not account ids, and the chat addresses accounts.
func staffAccountID(rt *Runtime, cred StaffCredentials) int64 {
	body, err := rt.Client.Get("/api/staff-messages/recipients")
	if err != nil {
		return 0
	}
	var envelope struct {
		Data []struct {
			AccountID string `json:"account_id"`
			Name      string `json:"name"`
		} `json:"data"`
	}
	if err := parseJSON(body, &envelope); err != nil {
		return 0
	}
	for _, row := range envelope.Data {
		if row.Name != cred.Name {
			continue
		}
		var id int64
		if _, err := fmt.Sscanf(row.AccountID, "%d", &id); err != nil {
			return 0
		}
		return id
	}
	return 0
}

// threadIDFromResponse pulls thread_id out of the {status,data,message} envelope.
func threadIDFromResponse(body []byte) string {
	var envelope struct {
		Data struct {
			ThreadID string `json:"thread_id"`
		} `json:"data"`
	}
	if err := parseJSON(body, &envelope); err != nil {
		return ""
	}
	return envelope.Data.ThreadID
}
