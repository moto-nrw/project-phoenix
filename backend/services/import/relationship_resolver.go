package importpkg

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/agnivade/levenshtein"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	importModels "github.com/moto-nrw/project-phoenix/models/import"
)

// RelationshipResolver resolves human-readable names to database IDs with fuzzy matching
type RelationshipResolver struct {
	groupRepo education.GroupRepository
	roomRepo  facilities.RoomRepository

	// In-memory caches (pre-loaded)
	groupCache map[string]*education.Group // lowercase name → group
	roomCache  map[string]*facilities.Room // lowercase name → room
}

// NewRelationshipResolver creates a new relationship resolver
func NewRelationshipResolver(groupRepo education.GroupRepository, roomRepo facilities.RoomRepository) *RelationshipResolver {
	return &RelationshipResolver{
		groupRepo:  groupRepo,
		roomRepo:   roomRepo,
		groupCache: make(map[string]*education.Group),
		roomCache:  make(map[string]*facilities.Room),
	}
}

// PreloadGroups loads all groups into memory cache (called once before import)
func (r *RelationshipResolver) PreloadGroups(ctx context.Context) error {
	options := base.NewQueryOptions()
	options.WithPagination(1, 1000) // Get up to 1000 groups

	groups, err := r.groupRepo.ListWithOptions(ctx, options)
	if err != nil {
		return fmt.Errorf("preload groups: %w", err)
	}

	for _, group := range groups {
		key := strings.ToLower(strings.TrimSpace(group.Name))
		r.groupCache[key] = group
	}

	return nil
}

// PreloadRooms loads all rooms into memory cache (called once before import)
func (r *RelationshipResolver) PreloadRooms(ctx context.Context) error {
	rooms, err := r.roomRepo.List(ctx, map[string]interface{}{})
	if err != nil {
		return fmt.Errorf("preload rooms: %w", err)
	}

	for _, room := range rooms {
		key := strings.ToLower(strings.TrimSpace(room.Name))
		r.roomCache[key] = room
	}

	return nil
}

// ResolveGroup resolves human-readable group name to ID with fuzzy matching
func (r *RelationshipResolver) ResolveGroup(ctx context.Context, groupName string) (*int64, []importModels.ValidationError) {
	if groupName == "" {
		return nil, nil // Optional field - empty is OK
	}

	normalized := strings.ToLower(strings.TrimSpace(groupName))

	// 1. Exact match (case-insensitive)
	if group, exists := r.groupCache[normalized]; exists {
		return &group.ID, nil
	}

	// 2. Fuzzy match (Levenshtein distance ≤ 3)
	suggestions := r.findSimilarGroups(groupName, 3)

	if len(suggestions) > 0 {
		return nil, []importModels.ValidationError{{
			Field:       "group",
			Message:     fmt.Sprintf("Gruppe '%s' nicht gefunden. Meinten Sie: %s?", groupName, strings.Join(suggestions, ", ")),
			Code:        "group_not_found_with_suggestions",
			Severity:    importModels.ErrorSeverityError,
			Suggestions: suggestions,
			AutoFix: &importModels.AutoFix{
				Action:      "replace",
				Replacement: suggestions[0], // Best match
				Description: fmt.Sprintf("Automatisch zu '%s' ändern", suggestions[0]),
			},
		}}
	}

	// 3. No matches - suggest creating or leaving empty
	return nil, []importModels.ValidationError{{
		Field:    "group",
		Message:  fmt.Sprintf("Gruppe '%s' existiert nicht. Bitte erstellen Sie die Gruppe zuerst oder lassen Sie das Feld leer.", groupName),
		Code:     "group_not_found",
		Severity: importModels.ErrorSeverityError,
	}}
}

// ResolveRoom resolves human-readable room name to ID with fuzzy matching
func (r *RelationshipResolver) ResolveRoom(ctx context.Context, roomName string) (*int64, []importModels.ValidationError) {
	if roomName == "" {
		return nil, nil // Optional field - empty is OK
	}

	normalized := strings.ToLower(strings.TrimSpace(roomName))

	// 1. Exact match (case-insensitive)
	if room, exists := r.roomCache[normalized]; exists {
		return &room.ID, nil
	}

	// 2. Fuzzy match (Levenshtein distance ≤ 3)
	suggestions := r.findSimilarRooms(roomName, 3)

	if len(suggestions) > 0 {
		return nil, []importModels.ValidationError{{
			Field:       "room",
			Message:     fmt.Sprintf("Raum '%s' nicht gefunden. Meinten Sie: %s?", roomName, strings.Join(suggestions, ", ")),
			Code:        "room_not_found_with_suggestions",
			Severity:    importModels.ErrorSeverityError,
			Suggestions: suggestions,
			AutoFix: &importModels.AutoFix{
				Action:      "replace",
				Replacement: suggestions[0], // Best match
				Description: fmt.Sprintf("Automatisch zu '%s' ändern", suggestions[0]),
			},
		}}
	}

	// 3. No matches
	return nil, []importModels.ValidationError{{
		Field:    "room",
		Message:  fmt.Sprintf("Raum '%s' existiert nicht. Bitte erstellen Sie den Raum zuerst oder lassen Sie das Feld leer.", roomName),
		Code:     "room_not_found",
		Severity: importModels.ErrorSeverityError,
	}}
}

// findSimilarGroups finds group names within Levenshtein distance threshold
func (r *RelationshipResolver) findSimilarGroups(input string, maxDistance int) []string {
	type match struct {
		name     string
		distance int
	}

	var matches []match
	inputLower := strings.ToLower(input)

	for _, group := range r.groupCache {
		nameLower := strings.ToLower(group.Name)
		distance := levenshtein.ComputeDistance(inputLower, nameLower)

		if distance <= maxDistance {
			matches = append(matches, match{name: group.Name, distance: distance})
		}
	}

	// Sort by distance (closest first)
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].distance < matches[j].distance
	})

	// Return top 3 suggestions
	result := make([]string, 0, 3)
	for i := 0; i < len(matches) && i < 3; i++ {
		result = append(result, matches[i].name)
	}

	return result
}

// findSimilarRooms finds room names within Levenshtein distance threshold
func (r *RelationshipResolver) findSimilarRooms(input string, maxDistance int) []string {
	type match struct {
		name     string
		distance int
	}

	var matches []match
	inputLower := strings.ToLower(input)

	for _, room := range r.roomCache {
		nameLower := strings.ToLower(room.Name)
		distance := levenshtein.ComputeDistance(inputLower, nameLower)

		if distance <= maxDistance {
			matches = append(matches, match{name: room.Name, distance: distance})
		}
	}

	// Sort by distance (closest first)
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].distance < matches[j].distance
	})

	// Return top 3 suggestions
	result := make([]string, 0, 3)
	for i := 0; i < len(matches) && i < 3; i++ {
		result = append(result, matches[i].name)
	}

	return result
}
