/**
 * Settings types and helper functions for scoped settings
 */

// === Backend Response Types (snake_case) ===

export interface BackendSettingDefinition {
  id: number;
  key: string;
  type: string;
  default_value: unknown;
  category: string;
  description?: string;
  validation?: BackendValidation;
  allowed_scopes: string[];
  scope_permissions: Record<string, string>;
  depends_on?: BackendSettingDependency;
  group_name?: string;
  sort_order: number;
}

export interface BackendValidation {
  min?: number;
  max?: number;
  options?: string[];
  pattern?: string;
}

export interface BackendSettingDependency {
  key: string;
  condition: string;
  value: unknown;
}

export interface BackendResolvedSetting {
  key: string;
  value: unknown;
  type: string;
  category: string;
  description?: string;
  group_name?: string;
  source?: BackendScopeRef;
  is_default: boolean;
  is_active: boolean;
  can_modify: boolean;
  depends_on?: BackendSettingDependency;
  validation?: BackendValidation;
}

export interface BackendScopeRef {
  type: string;
  id?: number;
}

export interface BackendSettingChange {
  id: number;
  setting_key: string;
  scope_type: string;
  scope_id?: number;
  change_type: string;
  old_value?: unknown;
  new_value?: unknown;
  reason?: string;
  account_id?: number;
  account_email?: string;
  created_at: string;
}

// === Frontend Types (camelCase) ===

export type SettingType = "string" | "int" | "bool" | "enum" | "json" | "time";

export interface SettingDefinition {
  id: string;
  key: string;
  type: SettingType;
  defaultValue: unknown;
  category: string;
  description?: string;
  validation?: SettingValidation;
  allowedScopes: string[];
  scopePermissions: Record<string, string>;
  dependsOn?: SettingDependency;
  groupName?: string;
  sortOrder: number;
}

export interface SettingValidation {
  min?: number;
  max?: number;
  options?: string[];
  pattern?: string;
}

export interface SettingDependency {
  key: string;
  condition: string;
  value: unknown;
}

export interface ResolvedSetting {
  key: string;
  value: unknown;
  type: SettingType;
  category: string;
  description?: string;
  groupName?: string;
  source?: ScopeRef;
  isDefault: boolean;
  isActive: boolean;
  canModify: boolean;
  dependsOn?: SettingDependency;
  validation?: SettingValidation;
}

export interface ScopeRef {
  type: string;
  id?: string;
}

export interface UpdateSettingRequest {
  value: unknown;
  reason?: string;
}

export interface SettingChange {
  id: string;
  settingKey: string;
  scopeType: string;
  scopeId?: string;
  changeType: "create" | "update" | "delete" | "reset";
  oldValue?: unknown;
  newValue?: unknown;
  reason?: string;
  accountId?: string;
  accountEmail?: string;
  createdAt: Date;
}

// === Mapping Functions ===

export function mapDefinitionResponse(
  data: BackendSettingDefinition,
): SettingDefinition {
  return {
    id: data.id.toString(),
    key: data.key,
    type: data.type as SettingType,
    defaultValue: data.default_value,
    category: data.category,
    description: data.description,
    validation: data.validation ? mapValidation(data.validation) : undefined,
    allowedScopes: data.allowed_scopes,
    scopePermissions: data.scope_permissions,
    dependsOn: data.depends_on ? mapDependency(data.depends_on) : undefined,
    groupName: data.group_name,
    sortOrder: data.sort_order,
  };
}

export function mapResolvedSettingResponse(
  data: BackendResolvedSetting,
): ResolvedSetting {
  return {
    key: data.key,
    value: data.value,
    type: data.type as SettingType,
    category: data.category,
    description: data.description,
    groupName: data.group_name,
    source: data.source ? mapScopeRef(data.source) : undefined,
    isDefault: data.is_default,
    isActive: data.is_active,
    canModify: data.can_modify,
    dependsOn: data.depends_on ? mapDependency(data.depends_on) : undefined,
    validation: data.validation ? mapValidation(data.validation) : undefined,
  };
}

function mapValidation(data: BackendValidation): SettingValidation {
  return {
    min: data.min,
    max: data.max,
    options: data.options,
    pattern: data.pattern,
  };
}

function mapDependency(data: BackendSettingDependency): SettingDependency {
  return {
    key: data.key,
    condition: data.condition,
    value: data.value,
  };
}

export function mapSettingChangeResponse(
  data: BackendSettingChange,
): SettingChange {
  return {
    id: data.id.toString(),
    settingKey: data.setting_key,
    scopeType: data.scope_type,
    scopeId: data.scope_id?.toString(),
    changeType: data.change_type as SettingChange["changeType"],
    oldValue: data.old_value,
    newValue: data.new_value,
    reason: data.reason,
    accountId: data.account_id?.toString(),
    accountEmail: data.account_email,
    createdAt: new Date(data.created_at),
  };
}

function mapScopeRef(data: BackendScopeRef): ScopeRef {
  return {
    type: data.type,
    id: data.id?.toString(),
  };
}

// === Grouping Helpers ===

export interface SettingGroup {
  name: string;
  settings: ResolvedSetting[];
}

export interface SettingCategory {
  name: string;
  groups: SettingGroup[];
}

/**
 * Groups settings by category and then by group_name
 */
export function groupSettingsByCategory(
  settings: ResolvedSetting[],
): SettingCategory[] {
  // First group by category
  const categoryMap = new Map<string, ResolvedSetting[]>();

  for (const setting of settings) {
    const category = setting.category;
    if (!categoryMap.has(category)) {
      categoryMap.set(category, []);
    }
    categoryMap.get(category)!.push(setting);
  }

  // Then group by group_name within each category
  const categories: SettingCategory[] = [];

  for (const [categoryName, categorySettings] of categoryMap) {
    const groupMap = new Map<string, ResolvedSetting[]>();

    for (const setting of categorySettings) {
      const groupName = setting.groupName ?? "_ungrouped";
      if (!groupMap.has(groupName)) {
        groupMap.set(groupName, []);
      }
      groupMap.get(groupName)!.push(setting);
    }

    // Sort settings within each group by sortOrder
    const groups: SettingGroup[] = [];
    for (const [groupName, groupSettings] of groupMap) {
      groups.push({
        name: groupName,
        settings: groupSettings.toSorted((a, b) => {
          const aOrder =
            categorySettings.find((s) => s.key === a.key)?.validation?.min ?? 0;
          const bOrder =
            categorySettings.find((s) => s.key === b.key)?.validation?.min ?? 0;
          return aOrder - bOrder;
        }),
      });
    }

    categories.push({
      name: categoryName,
      groups,
    });
  }

  return categories;
}

// === Display Helpers ===

export const categoryLabels: Record<string, string> = {
  session: "Sitzung",
  pickup: "Abholung",
  notifications: "Benachrichtigungen",
  appearance: "Darstellung",
  audit: "Protokollierung",
  checkin: "Check-in",
  device: "Gerät",
};

export const scopeLabels: Record<string, string> = {
  system: "System",
  school: "Schule",
  og: "OG",
  user: "Benutzer",
  device: "Gerät",
};

export function getCategoryLabel(category: string): string {
  return categoryLabels[category] ?? category;
}

export function getScopeLabel(scope: string): string {
  return scopeLabels[scope] ?? scope;
}

/**
 * Returns a human-readable label for where a setting value comes from
 */
export function getSourceLabel(setting: ResolvedSetting): string {
  if (setting.isDefault) {
    return "Standard";
  }
  if (setting.source) {
    return `Von ${getScopeLabel(setting.source.type)} geerbt`;
  }
  return "Eigener Wert";
}

// === Utility Functions ===

/**
 * Safely convert a setting value to string for input fields
 */
export function valueToString(value: unknown): string {
  if (value === null || value === undefined) {
    return "";
  }
  if (typeof value === "string" || typeof value === "number") {
    return String(value);
  }
  return "";
}

/**
 * Check if a setting should be shown based on dependencies
 */
export function isSettingActive(
  setting: ResolvedSetting,
  allSettings: ResolvedSetting[],
): boolean {
  if (!setting.dependsOn) return true;

  const dependentSetting = allSettings.find(
    (s) => s.key === setting.dependsOn?.key,
  );
  if (!dependentSetting) return true;

  const { condition, value: expectedValue } = setting.dependsOn;
  const actualValue = dependentSetting.value;

  switch (condition) {
    case "equals":
      return actualValue === expectedValue;
    case "not_equals":
      return actualValue !== expectedValue;
    case "in":
      return (
        Array.isArray(expectedValue) && expectedValue.includes(actualValue)
      );
    case "not_empty":
      return (
        actualValue !== null && actualValue !== undefined && actualValue !== ""
      );
    default:
      return true;
  }
}
