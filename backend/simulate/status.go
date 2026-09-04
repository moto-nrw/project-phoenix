package simulate

import (
	"context"
	"encoding/json"
	"fmt"
)

// StatusOptions configures the status query.
type StatusOptions struct {
	StatePath string
	Profile   string
	Verbose   bool
	Client    ClientFactory
}

// RunStatus queries the current simulation state and prints a summary.
func RunStatus(ctx context.Context, opts StatusOptions) error {
	if opts.Client == nil {
		return fmt.Errorf("simulation client factory is required")
	}
	state, err := LoadSeedStateProfile(opts.StatePath, opts.Profile)
	if err != nil {
		return fmt.Errorf("load seed state: %w", err)
	}

	client, err := buildClient(opts.Client, state.BaseURL, opts.Verbose)
	if err != nil {
		return err
	}

	if err := client.CheckHealth(); err != nil {
		return fmt.Errorf("server health check: %w", err)
	}

	if len(state.Accounts.Admin) == 0 {
		return fmt.Errorf("no admin accounts in seed state")
	}
	admin := state.Accounts.Admin[0]
	if err := client.Login(admin.Email, admin.Password, state.Bootstrap.TenantSlug); err != nil {
		return fmt.Errorf("admin login: %w", err)
	}

	fmt.Printf("Server: %s\n", state.BaseURL)
	fmt.Printf("Profile: %s\n", state.ProfileKey)
	fmt.Printf("Seed state: %s (created %s)\n\n", opts.StatePath, state.CreatedAt.Format("2006-01-02 15:04"))

	// Query active groups/sessions
	fmt.Println("=== Active Sessions ===")
	groupsResp, err := client.Get("/api/active/groups")
	if err != nil {
		fmt.Printf("  (could not fetch active groups: %v)\n", err)
	} else {
		printActiveGroups(groupsResp)
	}

	// Query active visits
	fmt.Println("\n=== Active Visits ===")
	visitsResp, err := client.Get("/api/active/visits")
	if err != nil {
		fmt.Printf("  (could not fetch active visits: %v)\n", err)
	} else {
		printActiveVisits(visitsResp)
	}

	// Seed state summary
	fmt.Println("\n=== Seed State ===")
	fmt.Printf("  Accounts:   %d admin, %d betreuer\n", len(state.Accounts.Admin), len(state.Accounts.Betreuer))
	fmt.Printf("  Students:   %d\n", len(state.Students))
	fmt.Printf("  Devices:    %d\n", len(state.Devices))
	fmt.Printf("  Rooms:      %d\n", len(state.Rooms))
	fmt.Printf("  Activities: %d\n", len(state.Activities))
	fmt.Printf("  Groups:     %d\n", len(state.Groups))

	return nil
}

func printActiveGroups(respBody []byte) {
	var envelope struct {
		Status string          `json:"status"`
		Data   json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		fmt.Printf("  (parse error: %v)\n", err)
		return
	}

	var groups []map[string]any
	if err := json.Unmarshal(envelope.Data, &groups); err != nil {
		// Might be directly an array
		if err2 := json.Unmarshal(respBody, &groups); err2 != nil {
			fmt.Printf("  (parse error: %v)\n", err)
			return
		}
	}

	if len(groups) == 0 {
		fmt.Println("  No active sessions")
		return
	}

	fmt.Printf("  %-4s %-20s %-20s %s\n", "ID", "Activity", "Room", "Supervisors")
	fmt.Printf("  %-4s %-20s %-20s %s\n", "----", "--------------------", "--------------------", "-----------")

	for _, g := range groups {
		id := fmt.Sprintf("%v", g["id"])
		activity := stringField(g, "activity_name")
		room := stringField(g, "room_name")
		supervisors := stringField(g, "supervisor_names")
		if activity == "" {
			activity = stringField(g, "name")
		}
		fmt.Printf("  %-4s %-20s %-20s %s\n", id, truncateStatusValue(activity), truncateStatusValue(room), supervisors)
	}
	fmt.Printf("  Total: %d active sessions\n", len(groups))
}

func truncateStatusValue(value string) string {
	if len(value) <= 20 {
		return value
	}
	return value[:20] + "..."
}

func printActiveVisits(respBody []byte) {
	var envelope struct {
		Status string          `json:"status"`
		Data   json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		fmt.Printf("  (parse error: %v)\n", err)
		return
	}

	var visits []map[string]any
	if err := json.Unmarshal(envelope.Data, &visits); err != nil {
		if err2 := json.Unmarshal(respBody, &visits); err2 != nil {
			fmt.Printf("  (parse error: %v)\n", err)
			return
		}
	}

	fmt.Printf("  Total students with active visits: %d\n", len(visits))

	// Count by room if possible
	roomCounts := make(map[string]int)
	for _, v := range visits {
		room := stringField(v, "room_name")
		if room == "" {
			room = "(unknown)"
		}
		roomCounts[room]++
	}

	if len(roomCounts) > 0 {
		fmt.Println("\n  Students per room:")
		for room, count := range roomCounts {
			fmt.Printf("    %-25s %d\n", room, count)
		}
	}
}

func stringField(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	s, ok := v.(string)
	if ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}
