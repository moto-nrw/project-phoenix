package config

import (
	"context"
	"encoding/json"
	"sort"

	"github.com/moto-nrw/project-phoenix/models/config"
)

// buildSchema constructs the full settings schema, filtered by permissions.
func buildSchema(
	ctx context.Context,
	svc *settingsService,
	userPermissions []string,
) (*SettingsSchema, error) {
	permSet := make(map[string]bool, len(userPermissions))
	for _, p := range userPermissions {
		permSet[p] = true
	}

	defs := config.AllDefinitions()

	// Resolve all values and build the resolved map
	resolvedMap := make(map[string]*ResolvedSetting, len(defs))
	for key, def := range defs {
		// Permission filter: if ReadPermission is set, user must have it
		if def.ReadPermission != "" && !permSet[def.ReadPermission] {
			continue
		}

		value, err := svc.Resolve(ctx, key)
		if err != nil {
			svc.logger.Warn("failed to resolve setting",
				"key", key,
				"error", err.Error(),
			)
			continue
		}

		// Determine if this is the default or an override
		isDefault := isDefaultValue(value, def.Default)

		// Mask password values (only when actually set, not empty defaults)
		displayValue := value
		if def.Type == config.FieldPassword && value != nil && value != "" {
			displayValue = "••••••"
		}

		resolved := &ResolvedSetting{
			Key:         key,
			Label:       def.Label,
			Description: def.Description,
			Type:        def.Type,
			Default:     def.Default,
			Value:       displayValue,
			IsDefault:   isDefault,
			Writable:    def.WritePermission == "" || permSet[def.WritePermission],
			Visible:     true,
			SortOrder:   def.SortOrder,
			Validation:  def.Validation,
			DependsOn:   def.DependsOn,
			Options:     def.Options,
		}
		resolvedMap[key] = resolved
	}

	// Evaluate DependsOn visibility
	for _, resolved := range resolvedMap {
		resolved.Visible = evaluateDependency(resolved, resolvedMap)
	}

	// Group by tab → category
	type catKey struct{ tab, category string }
	catItems := make(map[catKey][]*ResolvedSetting)
	tabSet := make(map[string]bool)

	for _, resolved := range resolvedMap {
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

		// Collect and sort categories
		var categoryKeys []string
		seen := make(map[string]bool)
		for ck := range catItems {
			if ck.tab == tabKey && !seen[ck.category] {
				categoryKeys = append(categoryKeys, ck.category)
				seen[ck.category] = true
			}
		}
		sort.Strings(categoryKeys)

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

// evaluateDependency checks if a setting's dependency condition is met.
func evaluateDependency(resolved *ResolvedSetting, resolvedMap map[string]*ResolvedSetting) bool {
	if resolved.DependsOn == nil {
		return true
	}

	parent, ok := resolvedMap[resolved.DependsOn.Key]
	if !ok {
		return false
	}

	switch resolved.DependsOn.Condition {
	case "eq":
		return jsonValuesEqual(parent.Value, resolved.DependsOn.Value)
	case "neq":
		return !jsonValuesEqual(parent.Value, resolved.DependsOn.Value)
	case "not_empty":
		return parent.Value != nil && parent.Value != ""
	default:
		return true
	}
}

// jsonValuesEqual compares two values by their JSON representation.
func jsonValuesEqual(a, b any) bool {
	aj, _ := json.Marshal(a)
	bj, _ := json.Marshal(b)
	return string(aj) == string(bj)
}

// isDefaultValue checks if the resolved value matches the registry default.
func isDefaultValue(value, defaultVal any) bool {
	return jsonValuesEqual(value, defaultVal)
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
