package config

import (
	"context"
	"encoding/json"
	"log/slog"
	"sort"

	config "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

// buildSchema constructs the full settings schema for tenant admins.
// Settings marked AccessOperatorOnly are hidden; the per-setting
// ReadPermission filter is applied.
func buildSchema(
	ctx context.Context,
	svc *settingsService,
	userPermissions []string,
) (*config.SettingsSchema, error) {
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
) (*config.SettingsSchema, error) {
	return buildSchemaWithScope(ctx, svc, userPermissions, true)
}

// buildSchemaWithScope is the shared implementation. isOperator selects the
// admin-vs-operator filters: operators see AccessShared + AccessOperatorOnly,
// tenants see AccessShared + AccessAdminOnly. Both see AccessOperatorManaged;
// for tenants it is never writable.
func buildSchemaWithScope(ctx context.Context, svc *settingsService, userPermissions []string, isOperator bool) (*config.SettingsSchema, error) {
	defs := svc.registry.AllDefinitions()
	keys := make([]string, 0, len(defs))
	for key := range defs {
		keys = append(keys, key)
	}
	snapshot, err := svc.ResolveMany(ctx, keys)
	if err != nil {
		return nil, err
	}
	resolvedMap, outputMap := resolveSchemaFields(svc, defs, snapshot, userPermissions, isOperator)
	if err := projectParentReviewScopes(resolvedMap, outputMap, snapshot); err != nil {
		return nil, err
	}
	if err := projectParentReportModes(resolvedMap, outputMap, snapshot); err != nil {
		return nil, err
	}
	visibilityMemo := make(map[string]bool, len(resolvedMap))
	for _, resolved := range resolvedMap {
		resolved.Visible = evaluateDependency(resolved, resolvedMap, visibilityMemo, make(map[string]bool))
	}
	return groupSettingsSchema(svc.registry, outputMap), nil
}

func resolveSchemaFields(svc *settingsService, defs map[string]*config.Definition, snapshot *config.SettingsSnapshot, permissions []string, isOperator bool) (map[string]*config.ResolvedSetting, map[string]*config.ResolvedSetting) {
	resolvedMap := make(map[string]*config.ResolvedSetting, len(defs))
	outputMap := make(map[string]*config.ResolvedSetting, len(defs))
	for key, definition := range defs {
		resolved := resolveSchemaField(key, definition, snapshot, permissions, isOperator, svc.logger)
		if resolved == nil {
			continue
		}
		resolvedMap[key] = resolved
		if shouldIncludeInSchema(definition, permissions, isOperator) {
			outputMap[key] = resolved
		}
	}
	return resolvedMap, outputMap
}

func resolveSchemaField(key string, def *config.Definition, snapshot *config.SettingsSnapshot, userPermissions []string, isOperator bool, logger *slog.Logger) *config.ResolvedSetting {
	value, err := snapshot.Value(key)
	if err != nil {
		logger.Warn("failed to resolve setting",
			"key", key,
			"error", err.Error(),
		)
		return nil
	}
	hasOverride, err := snapshot.HasOverride(key)
	if err != nil {
		logger.Warn("failed to inspect setting override",
			"key", key,
			"error", err.Error(),
		)
		return nil
	}
	source, inherited, traegerChangedAt, err := snapshot.Layer(key)
	if err != nil {
		logger.Warn("failed to inspect setting level",
			"key", key,
			"error", err.Error(),
		)
		return nil
	}
	// IsDefault means "nothing of the school's own": a reset changes
	// nothing. The school inherits the Träger value where there is one.
	isDefault := !hasOverride
	if def.Type == config.FieldBoolean {
		isDefault = jsonValuesEqual(value, inherited)
	}
	displayValue := value
	if def.Type == config.FieldPassword && value != nil && value != "" {
		displayValue = "••••••"
	}
	// A Träger-only setting has no school override: neither the school nor
	// the moto team writes it per school (ADR 0086 section 6).
	writable := def.Levels != config.LevelTraeger && (isOperator || (def.AccessPolicy != config.AccessOperatorManaged &&
		(def.WritePermission == "" || hasPermission(def.WritePermission, userPermissions))))
	return &config.ResolvedSetting{
		Key: key, Label: def.Label, Description: def.Description, Type: def.Type, Default: def.Default,
		Value: displayValue, IsDefault: isDefault, Writable: writable, Visible: true, SortOrder: def.SortOrder,
		AccessPolicy: def.AccessPolicy, Levels: def.Levels, Source: source, TraegerChangedAt: traegerChangedAt,
		Validation: def.Validation, DependsOn: def.DependsOn, Options: def.Options,
	}
}

type schemaCategoryKey struct{ tab, category string }

func groupSettingsSchema(registry *config.Registry, output map[string]*config.ResolvedSetting) *config.SettingsSchema {
	items := make(map[schemaCategoryKey][]*config.ResolvedSetting)
	tabSet := make(map[string]bool)
	for _, resolved := range output {
		definition := registry.GetDefinition(resolved.Key)
		if definition == nil {
			continue
		}
		key := schemaCategoryKey{tab: definition.Tab, category: definition.Category}
		items[key] = append(items[key], resolved)
		tabSet[definition.Tab] = true
	}
	schema := &config.SettingsSchema{}
	for _, key := range orderTabs(tabSet) {
		tab := buildSchemaTab(key, items)
		if len(tab.Categories) > 0 {
			schema.Tabs = append(schema.Tabs, tab)
		}
	}
	return schema
}

func buildSchemaTab(tabKey string, catItems map[schemaCategoryKey][]*config.ResolvedSetting) *config.SchemaTab {
	tab := &config.SchemaTab{Key: tabKey, Label: tabKey}
	for _, name := range sortedSchemaCategories(tabKey, catItems) {
		items := catItems[schemaCategoryKey{tab: tabKey, category: name}]
		if len(items) == 0 {
			continue
		}
		sort.Slice(items, func(i, j int) bool { return items[i].SortOrder < items[j].SortOrder })
		tab.Categories = append(tab.Categories, &config.SchemaCategory{Key: name, Label: name, Items: items})
	}
	return tab
}

func sortedSchemaCategories(tabKey string, catItems map[schemaCategoryKey][]*config.ResolvedSetting) []string {
	var names []string
	seen := make(map[string]bool)
	for key := range catItems {
		if key.tab == tabKey && !seen[key.category] {
			names = append(names, key.category)
			seen[key.category] = true
		}
	}
	sort.Slice(names, func(i, j int) bool {
		minI := minCategorySortOrder(catItems[schemaCategoryKey{tab: tabKey, category: names[i]}])
		minJ := minCategorySortOrder(catItems[schemaCategoryKey{tab: tabKey, category: names[j]}])
		if minI != minJ {
			return minI < minJ
		}
		return names[i] < names[j]
	})
	return names
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
		hasPermission(def.ReadPermission, userPermissions)
}

// evaluateDependency checks if a setting's dependency condition is met.
func evaluateDependency(resolved *config.ResolvedSetting, resolvedMap map[string]*config.ResolvedSetting, memo map[string]bool, visiting map[string]bool) bool {
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
func minCategorySortOrder(items []*config.ResolvedSetting) int {
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
