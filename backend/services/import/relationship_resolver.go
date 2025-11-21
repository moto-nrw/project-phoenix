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

// resolveEntity is a generic function to resolve entity names with fuzzy matching
func (r *RelationshipResolver) resolveEntity(
	name string,
	fieldName string,
	entityType string,
	getID func(string) (*int64, bool),
	findSimilar func(string, int) []string,
) (*int64, []importModels.ValidationError) {
	if name == "" {
		return nil, nil // Optional field - empty is OK
	}

	normalized := strings.ToLower(strings.TrimSpace(name))

	// 1. Exact match (case-insensitive)
	if id, exists := getID(normalized); exists {
		return id, nil
	}

	// 2. Fuzzy match (Levenshtein distance ≤ 3)
	suggestions := findSimilar(name, 3)

	if len(suggestions) > 0 {
		return nil, []importModels.ValidationError{{
			Field:       fieldName,
			Message:     fmt.Sprintf("%s '%s' nicht gefunden. Meinten Sie: %s? (Wird ohne %s importiert)", entityType, name, strings.Join(suggestions, ", "), entityType),
			Code:        fmt.Sprintf("%s_not_found_with_suggestions", fieldName),
			Severity:    importModels.ErrorSeverityWarning,
			Suggestions: suggestions,
			AutoFix: &importModels.AutoFix{
				Action:      "replace",
				Replacement: suggestions[0], // Best match
				Description: fmt.Sprintf("Automatisch zu '%s' ändern", suggestions[0]),
			},
		}}
	}

	// 3. No matches - proceed without entity (warning only)
	return nil, []importModels.ValidationError{{
		Field:    fieldName,
		Message:  fmt.Sprintf("%s '%s' existiert nicht. Schüler wird ohne %s importiert.", entityType, name, entityType),
		Code:     fmt.Sprintf("%s_not_found", fieldName),
		Severity: importModels.ErrorSeverityWarning,
	}}
}

// ResolveGroup resolves human-readable group name to ID with fuzzy matching
func (r *RelationshipResolver) ResolveGroup(ctx context.Context, groupName string) (*int64, []importModels.ValidationError) {
	return r.resolveEntity(
		groupName,
		"group",
		"Gruppe",
		func(normalized string) (*int64, bool) {
			if group, exists := r.groupCache[normalized]; exists {
				return &group.ID, true
			}
			return nil, false
		},
		r.findSimilarGroups,
	)
}

// ResolveRoom resolves human-readable room name to ID with fuzzy matching
func (r *RelationshipResolver) ResolveRoom(ctx context.Context, roomName string) (*int64, []importModels.ValidationError) {
	return r.resolveEntity(
		roomName,
		"room",
		"Raum",
		func(normalized string) (*int64, bool) {
			if room, exists := r.roomCache[normalized]; exists {
				return &room.ID, true
			}
			return nil, false
		},
		r.findSimilarRooms,
	)
}

// findSimilar is a generic helper to find similar names using Levenshtein distance
func findSimilar(input string, maxDistance int, getNames func() []string) []string {
	type match struct {
		name     string
		distance int
	}

	var matches []match
	inputLower := strings.ToLower(input)

	for _, name := range getNames() {
		nameLower := strings.ToLower(name)
		distance := levenshtein.ComputeDistance(inputLower, nameLower)

		if distance <= maxDistance {
			matches = append(matches, match{name: name, distance: distance})
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

// findSimilarGroups finds group names within Levenshtein distance threshold
func (r *RelationshipResolver) findSimilarGroups(input string, maxDistance int) []string {
	return findSimilar(input, maxDistance, func() []string {
		names := make([]string, 0, len(r.groupCache))
		for _, group := range r.groupCache {
			names = append(names, group.Name)
		}
		return names
	})
}

// findSimilarRooms finds room names within Levenshtein distance threshold
func (r *RelationshipResolver) findSimilarRooms(input string, maxDistance int) []string {
	return findSimilar(input, maxDistance, func() []string {
		names := make([]string, 0, len(r.roomCache))
		for _, room := range r.roomCache {
			names = append(names, room.Name)
		}
		return names
	})
}
