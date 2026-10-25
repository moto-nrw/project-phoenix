package api

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// Formatting of the seed summary the seeder prints at the end.

func printSeedProfiles(state *SeedState) {
	fmt.Println("DEMO SCHOOL PROFILES:")
	for _, key := range slices.Sorted(maps.Keys(state.Profiles)) {
		profile := state.Profiles[key]
		if profile == nil {
			continue
		}
		fmt.Printf("  %s: %s (slug: %s)\n", profile.Key, profile.Name, profile.School.TenantSlug)
		fmt.Printf("    School admin: %s / %s\n", profile.Credentials.SchoolAdmin.Email, profile.Credentials.SchoolAdmin.Password)
		fmt.Printf("    Account keys: %s\n", formatAccountKeys(profile.Credentials.Accounts))
		fmt.Printf("    Student keys: %s\n", formatSortedKeys(profile.Entities.Students))
		fmt.Printf("    Contact keys: %s\n", formatSortedKeys(profile.Entities.Guardians))
		fmt.Printf("    Group keys: %s\n", formatSortedKeys(profile.Entities.Groups))
		fmt.Printf("    Device keys: %s\n", formatSortedKeys(profile.Devices))
	}
	fmt.Println()
}

func formatAccountKeys(accounts SeedStateAccounts) string {
	keys := make([]string, 0, len(accounts.Admin)+len(accounts.Betreuer))
	for _, account := range accounts.Admin {
		keys = append(keys, account.Key)
	}
	for _, account := range accounts.Betreuer {
		keys = append(keys, account.Key)
	}
	if len(keys) == 0 {
		return "-"
	}
	slices.Sort(keys)
	return strings.Join(keys, ", ")
}

func formatSortedKeys[V any](values map[string]V) string {
	if len(values) == 0 {
		return "-"
	}
	return strings.Join(slices.Sorted(maps.Keys(values)), ", ")
}

func formatSeedIDList(ids []int64) string {
	if len(ids) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, fmt.Sprintf("%d", id))
	}
	return strings.Join(parts, ", ")
}

func formatEnrollmentStatusCounts(requests []SeedEnrollmentRequest) string {
	return formatSortedCountMap(countBy(requests, func(request SeedEnrollmentRequest) string {
		if status := strings.TrimSpace(request.Status); status != "" {
			return status
		}
		return "submitted"
	}))
}

func formatParentActionCounts(actions []SeedParentPortalAction) string {
	return formatSortedCountMap(countBy(actions, func(action SeedParentPortalAction) string {
		if actionType := strings.TrimSpace(action.Type); actionType != "" {
			return actionType
		}
		return "unknown"
	}))
}

// formatSortedCountMap renders a string->number map as "key=value" pairs sorted
// by key, or "-" when empty.
func formatSortedCountMap[V int | int64](values map[string]V) string {
	if len(values) == 0 {
		return "-"
	}
	keys := slices.Sorted(maps.Keys(values))
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", key, values[key]))
	}
	return strings.Join(parts, ", ")
}

// countBy tallies items by the string key derived from each element.
func countBy[T any](items []T, key func(T) string) map[string]int {
	counts := make(map[string]int)
	for _, item := range items {
		counts[key(item)]++
	}
	return counts
}
