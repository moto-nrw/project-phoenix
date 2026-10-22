package api

import (
	"encoding/json"
	"fmt"
	"strings"
)

// roomCategorySeed is one room category with the badge color its rooms show
// (#4352). Seeded rooms carry no color of their own: like a school that
// never picked one, they take the color of their category.
type roomCategorySeed struct {
	Name  string
	Color string
}

type roomCategoryClient interface {
	Get(path string) ([]byte, error)
	Post(path string, body any) ([]byte, error)
}

// ensureRoomCategories creates the wanted categories the school does not
// have yet. A new school already holds the defaults from provisioning, so
// only categories beyond them are created; an existing one keeps its color.
func ensureRoomCategories(client roomCategoryClient, wanted []roomCategorySeed) error {
	raw, err := client.Get("/api/room-categories")
	if err != nil {
		return fmt.Errorf("list room categories: %w", err)
	}
	var resp struct {
		Data []struct {
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Errorf("parse room categories: %w", err)
	}
	existing := make([]string, 0, len(resp.Data))
	for _, category := range resp.Data {
		existing = append(existing, category.Name)
	}
	for _, category := range missingRoomCategories(existing, wanted) {
		if _, err := client.Post("/api/room-categories", map[string]any{"name": category.Name, "color": category.Color}); err != nil {
			return fmt.Errorf("create room category %s: %w", category.Name, err)
		}
	}
	return nil
}

// missingRoomCategories returns the wanted categories whose name, ignoring
// case, the school does not have, each once and in the wanted order.
func missingRoomCategories(existing []string, wanted []roomCategorySeed) []roomCategorySeed {
	have := make(map[string]struct{}, len(existing)+len(wanted))
	for _, name := range existing {
		have[strings.ToLower(name)] = struct{}{}
	}
	missing := []roomCategorySeed{}
	for _, category := range wanted {
		key := strings.ToLower(category.Name)
		if _, ok := have[key]; ok {
			continue
		}
		have[key] = struct{}{}
		missing = append(missing, category)
	}
	return missing
}
