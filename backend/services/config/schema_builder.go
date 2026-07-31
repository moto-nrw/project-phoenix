package config

import (
	"context"
	"encoding/json"
	"log/slog"
	"sort"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/models/config"
)

// buildSchema constructs the full settings schema for tenant admins.
// Settings marked AccessOperatorOnly are hidden; the per-setting
// ReadPermission filter is applied.
func buildSchema(
	ctx context.Context,
	svc *settingsService,
	userPermissions []string,
) (*SettingsSchema, error) {
	return buildSchemaWithScope(ctx, svc, userPermissions, false)
}

// buildSchemaForOperator constructs the full settings schema for platform operators.
// Settings marked AccessAdminOnly are hidden; the per-setting ReadPermission filter
// is skipped (operator access is gated at the route level by RequiresOperatorScope
// and per-setting by AccessPolicy).
func buildSchemaForOperator(
	ctx context.Context,
	svc *settingsService,
	userPermissions []string,
) (*SettingsSchema, error) {
	return buildSchemaWithScope(ctx, svc, userPermissions, true)
}

// buildSchemaWithScope is the shared implementation. isOperator selects the
// admin-vs-operator filters: operators see AccessShared + AccessOperatorOnly,
// tenants see AccessShared + AccessAdminOnly.
func buildSchemaWithScope(
	ctx context.Context,
	svc *settingsService,
	userPermissions []string,
	isOperator bool,
) (*SettingsSchema, error) {
	defs := config.AllDefinitions()

	// Resolve all values for dependency evaluation, then filter the output
	// separately. Tenant-visible settings can depend on operator-only
	// provisioning flags such as attendance.nfc_enabled; hiding those parents
	// before evaluating DependsOn would make the tenant-visible children vanish.
	keys := make([]string, 0, len(defs))
	for key := range defs {
		keys = append(keys, key)
	}
	snapshot, err := svc.ResolveMany(ctx, keys)
	if err != nil {
		return nil, err
	}

	resolvedMap := make(map[string]*ResolvedSetting, len(defs))
	outputMap := make(map[string]*ResolvedSetting, len(defs))
	for key, def := range defs {
		value, err := snapshot.Value(key)
		if err != nil {
			svc.logger.Warn("failed to resolve setting",
				"key", key,
				"error", err.Error(),
			)
			continue
		}

		// Determine if this is using the default. Default semantics are
		// override-existence-based EXCEPT for boolean toggles, which have no
		// reset button: without a value-based check their badge would stay
		// "not default" forever once toggled, even after toggling back to the
		// default value (issue #1680). For resettable settings we deliberately
		// keep override-existence semantics so the reset button stays available
		// to clear an explicit override — even one that currently equals the
		// registry default (otherwise an env-fallback consumer would silently
		// stay tenant-pinned with no way to clear it from the UI).
		// jsonValuesEqual handles the JSONB roundtrip (float64 vs int) and
		// type-correct comparison.
		hasOverride, overrideErr := snapshot.HasOverride(key)
		if overrideErr != nil {
			svc.logger.Warn("failed to inspect setting override",
				"key", key,
				"error", overrideErr.Error(),
			)
			continue
		}
		isDefault := !hasOverride
		if def.Type == config.FieldBoolean {
			isDefault = jsonValuesEqual(value, def.Default)
		}

		// Mask password values (only when actually set, not empty defaults)
		displayValue := value
		if def.Type == config.FieldPassword && value != nil && value != "" {
			displayValue = "••••••"
		}

		// Operators can always write (AccessPolicy gate already enforced at
		// the route level; they bypass WritePermission in SetValue).
		writable := isOperator ||
			def.WritePermission == "" ||
			authorize.HasPermission(def.WritePermission, userPermissions)

		resolved := &ResolvedSetting{
			Key:          key,
			Label:        def.Label,
			Description:  def.Description,
			Type:         def.Type,
			Default:      def.Default,
			Value:        displayValue,
			IsDefault:    isDefault,
			Writable:     writable,
			Visible:      true,
			SortOrder:    def.SortOrder,
			AccessPolicy: def.AccessPolicy,
			Validation:   def.Validation,
			DependsOn:    def.DependsOn,
			Options:      def.Options,
		}
		resolvedMap[key] = resolved
		if shouldIncludeInSchema(def, userPermissions, isOperator) {
			outputMap[key] = resolved
		}
	}

	// Evaluate DependsOn visibility. Dependencies may be nested, so resolve
	// parent visibility recursively instead of relying on map iteration order.
	visibilityMemo := make(map[string]bool, len(resolvedMap))
	for _, resolved := range resolvedMap {
		resolved.Visible = evaluateDependency(resolved, resolvedMap, visibilityMemo, make(map[string]bool))
	}

	// Group by tab → category
	type catKey struct{ tab, category string }
	catItems := make(map[catKey][]*ResolvedSetting)
	tabSet := make(map[string]bool)

	for _, resolved := range outputMap {
		def := config.GetDefinition(resolved.Key)
		if def == nil {
			continue
		}
		ck := catKey{tab: def.Tab, category: def.Category}
		catItems[ck] = append(catItems[ck], resolved)
		tabSet[def.Tab] = true
	}

	// Build ordered tab structure
	schema := &SettingsSchema{}
	orderedTabs := orderTabs(tabSet)

	for _, tabKey := range orderedTabs {
		schemaTab := &SchemaTab{
			Key:   tabKey,
			Label: tabKey,
		}

		// Collect and sort categories by minimum SortOrder of their items,
		// with alphabetical as tiebreaker.
		var categoryKeys []string
		seen := make(map[string]bool)
		for ck := range catItems {
			if ck.tab == tabKey && !seen[ck.category] {
				categoryKeys = append(categoryKeys, ck.category)
				seen[ck.category] = true
			}
		}
		sort.Slice(categoryKeys, func(i, j int) bool {
			minI := minCategorySortOrder(catItems[catKey{tab: tabKey, category: categoryKeys[i]}])
			minJ := minCategorySortOrder(catItems[catKey{tab: tabKey, category: categoryKeys[j]}])
			if minI != minJ {
				return minI < minJ
			}
			return categoryKeys[i] < categoryKeys[j]
		})

		for _, catName := range categoryKeys {
			ck := catKey{tab: tabKey, category: catName}
			items := catItems[ck]
			if len(items) == 0 {
				continue
			}

			sort.Slice(items, func(i, j int) bool {
				return items[i].SortOrder < items[j].SortOrder
			})

			schemaTab.Categories = append(schemaTab.Categories, &SchemaCategory{
				Key:   catName,
				Label: catName,
				Items: items,
			})
		}

		if len(schemaTab.Categories) > 0 {
			schema.Tabs = append(schema.Tabs, schemaTab)
		}
	}

	return schema, nil
}

func shouldIncludeInSchema(def *config.Definition, userPermissions []string, isOperator bool) bool {
	if isOperator && def.AccessPolicy == config.AccessAdminOnly {
		return false
	}
	if !isOperator && def.AccessPolicy == config.AccessOperatorOnly {
		return false
	}

	// Permission filter is only applied for tenant callers. Operators bypass
	// the per-setting ReadPermission because operator access is route-gated.
	return isOperator ||
		def.ReadPermission == "" ||
		authorize.HasPermission(def.ReadPermission, userPermissions)
}

// evaluateDependency checks if a setting's dependency condition is met.
func evaluateDependency(resolved *ResolvedSetting, resolvedMap map[string]*ResolvedSetting, memo map[string]bool, visiting map[string]bool) bool {
	if visible, ok := memo[resolved.Key]; ok {
		return visible
	}
	if visiting[resolved.Key] {
		slog.Warn("settings dependency cycle detected, hiding setting",
			slog.String("key", resolved.Key),
		)
		memo[resolved.Key] = false
		return false
	}
	if resolved.DependsOn == nil {
		memo[resolved.Key] = true
		return true
	}

	visiting[resolved.Key] = true
	defer delete(visiting, resolved.Key)

	parent, ok := resolvedMap[resolved.DependsOn.Key]
	if !ok {
		memo[resolved.Key] = false
		return false
	}

	if !evaluateDependency(parent, resolvedMap, memo, visiting) {
		memo[resolved.Key] = false
		return false
	}

	visible := false
	switch resolved.DependsOn.Condition {
	case "eq":
		visible = jsonValuesEqual(parent.Value, resolved.DependsOn.Value)
	case "neq":
		visible = !jsonValuesEqual(parent.Value, resolved.DependsOn.Value)
	case "not_empty":
		visible = parent.Value != nil && parent.Value != ""
	default:
		slog.Warn("unknown dependency condition, treating as not met",
			slog.String("key", resolved.DependsOn.Key),
			slog.String("condition", resolved.DependsOn.Condition),
		)
		visible = false
	}
	memo[resolved.Key] = visible
	return visible
}

// jsonValuesEqual compares two values by their JSON representation.
// Returns false if either value fails to marshal.
func jsonValuesEqual(a, b any) bool {
	aj, errA := json.Marshal(a)
	bj, errB := json.Marshal(b)
	if errA != nil || errB != nil {
		return false
	}
	return string(aj) == string(bj)
}

// minCategorySortOrder returns the minimum SortOrder among items in a category.
func minCategorySortOrder(items []*ResolvedSetting) int {
	if len(items) == 0 {
		return 0
	}
	min := items[0].SortOrder
	for _, item := range items[1:] {
		if item.SortOrder < min {
			min = item.SortOrder
		}
	}
	return min
}

// orderTabs returns tab keys ordered by TabOrder, with unknown tabs appended alphabetically.
func orderTabs(tabSet map[string]bool) []string {
	var ordered []string
	seen := make(map[string]bool)

	for _, t := range config.TabOrder {
		if tabSet[t] {
			ordered = append(ordered, t)
			seen[t] = true
		}
	}

	var extra []string
	for t := range tabSet {
		if !seen[t] {
			extra = append(extra, t)
		}
	}
	sort.Strings(extra)

	return append(ordered, extra...)
}
