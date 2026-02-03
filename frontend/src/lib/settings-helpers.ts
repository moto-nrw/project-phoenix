// Settings type definitions and helper functions

// ========== Backend Types (from API) ==========

export type BackendValueType =
  | "string"
  | "int"
  | "float"
  | "bool"
  | "enum"
  | "time"
  | "duration"
  | "object_ref"
  | "json";

export type BackendScope = "system" | "user" | "device";

export interface BackendValidationSchema {
  min?: number;
  max?: number;
  pattern?: string;
  required?: boolean;
}

// EnumOption represents a single option for enum-type settings
export interface BackendEnumOption {
  value: string;
  label: string;
}

export interface BackendSettingDefinition {
  id: number;
  key: string;
  value_type: BackendValueType;
  default_value: string;
  category: string;
  tab: string;
  display_order: number;
  label?: string;
  description?: string;
  allowed_scopes: BackendScope[];
  view_permission?: string;
  edit_permission?: string;
  validation?: BackendValidationSchema;
  enum_values?: string[];
  enum_options?: BackendEnumOption[];
  object_ref_type?: string;
  requires_restart: boolean;
  is_sensitive: boolean;
}

export interface BackendSettingTab {
  key: string;
  name: string;
  icon?: string;
  display_order: number;
}

export interface BackendResolvedSetting {
  key: string;
  definition: BackendSettingDefinition;
  effective_value: string;
  effective_scope: BackendScope;
  is_overridden: boolean;
  can_edit: boolean;
}

export interface BackendSettingCategory {
  key: string;
  name: string;
  settings: BackendResolvedSetting[];
}

export interface BackendTabSettingsResponse {
  tab: BackendSettingTab;
  categories: BackendSettingCategory[];
}

export interface BackendObjectRefOption {
  id: number;
  name: string;
  extra?: Record<string, string>;
}

export interface BackendSettingAuditEntry {
  id: number;
  setting_key: string;
  scope_type: BackendScope;
  scope_id?: number;
  old_value?: string;
  new_value?: string;
  action: "create" | "update" | "delete" | "restore";
  changed_by_name: string;
  changed_at: string;
}

// ========== Frontend Types ==========

export type ValueType =
  | "string"
  | "int"
  | "float"
  | "bool"
  | "enum"
  | "time"
  | "duration"
  | "objectRef"
  | "json";

export type Scope = "system" | "user" | "device";

export interface ValidationSchema {
  min?: number;
  max?: number;
  pattern?: string;
  required?: boolean;
}

// EnumOption represents a single option for enum-type settings
export interface EnumOption {
  value: string;
  label: string;
}

export interface SettingDefinition {
  id: string;
  key: string;
  valueType: ValueType;
  defaultValue: string;
  category: string;
  tab: string;
  displayOrder: number;
  label?: string;
  description?: string;
  allowedScopes: Scope[];
  viewPermission?: string;
  editPermission?: string;
  validation?: ValidationSchema;
  enumValues?: string[];
  enumOptions?: EnumOption[];
  objectRefType?: string;
  requiresRestart: boolean;
  isSensitive: boolean;
}

export interface SettingTab {
  key: string;
  name: string;
  icon?: string;
  displayOrder: number;
}

export interface ResolvedSetting {
  key: string;
  definition: SettingDefinition;
  effectiveValue: string;
  effectiveScope: Scope;
  isOverridden: boolean;
  canEdit: boolean;
}

export interface SettingCategory {
  key: string;
  name: string;
  settings: ResolvedSetting[];
}

export interface TabSettingsResponse {
  tab: SettingTab;
  categories: SettingCategory[];
}

export interface ObjectRefOption {
  id: string;
  name: string;
  extra?: Record<string, string>;
}

export interface SettingAuditEntry {
  id: string;
  settingKey: string;
  scopeType: Scope;
  scopeId?: string;
  oldValue?: string;
  newValue?: string;
  action: "create" | "update" | "delete" | "restore";
  changedByName: string;
  changedAt: Date;
}

// ========== Request Types ==========

export interface SetValueRequest {
  value: string;
  scope: Scope;
  scopeId?: string;
}

export interface DeleteValueRequest {
  scope: Scope;
  scopeId?: string;
}

export interface RestoreValueRequest {
  scope: Scope;
  scopeId?: string;
}

export interface PurgeRequest {
  days: number;
}

// ========== Mapping Functions ==========

function mapValueType(type: BackendValueType): ValueType {
  if (type === "object_ref") return "objectRef";
  return type as ValueType;
}

export function mapSettingDefinition(
  data: BackendSettingDefinition,
): SettingDefinition {
  return {
    id: data.id.toString(),
    key: data.key,
    valueType: mapValueType(data.value_type),
    defaultValue: data.default_value,
    category: data.category,
    tab: data.tab,
    displayOrder: data.display_order,
    label: data.label,
    description: data.description,
    allowedScopes: data.allowed_scopes,
    viewPermission: data.view_permission,
    editPermission: data.edit_permission,
    validation: data.validation,
    enumValues: data.enum_values,
    enumOptions: data.enum_options,
    objectRefType: data.object_ref_type,
    requiresRestart: data.requires_restart,
    isSensitive: data.is_sensitive,
  };
}

export function mapSettingTab(data: BackendSettingTab): SettingTab {
  return {
    key: data.key,
    name: data.name,
    icon: data.icon,
    displayOrder: data.display_order,
  };
}

export function mapResolvedSetting(
  data: BackendResolvedSetting,
): ResolvedSetting | null {
  // Guard against undefined data or missing definition
  if (!data || !data.definition) {
    console.error("mapResolvedSetting: Invalid data or missing definition", data);
    return null;
  }
  return {
    key: data.key,
    definition: mapSettingDefinition(data.definition),
    effectiveValue: data.effective_value,
    effectiveScope: data.effective_scope,
    isOverridden: data.is_overridden,
    canEdit: data.can_edit,
  };
}

export function mapSettingCategory(
  data: BackendSettingCategory,
): SettingCategory {
  return {
    key: data.key,
    name: data.name,
    settings: data.settings
      .map(mapResolvedSetting)
      .filter((s): s is ResolvedSetting => s !== null),
  };
}

export function mapTabSettingsResponse(
  data: BackendTabSettingsResponse,
): TabSettingsResponse {
  return {
    tab: mapSettingTab(data.tab),
    categories: data.categories.map(mapSettingCategory),
  };
}

export function mapObjectRefOption(
  data: BackendObjectRefOption,
): ObjectRefOption {
  return {
    id: data.id.toString(),
    name: data.name,
    extra: data.extra,
  };
}

export function mapSettingAuditEntry(
  data: BackendSettingAuditEntry,
): SettingAuditEntry {
  return {
    id: data.id.toString(),
    settingKey: data.setting_key,
    scopeType: data.scope_type,
    scopeId: data.scope_id?.toString(),
    oldValue: data.old_value,
    newValue: data.new_value,
    action: data.action,
    changedByName: data.changed_by_name,
    changedAt: new Date(data.changed_at),
  };
}

// ========== Display Helpers ==========

export function getScopeName(scope: Scope): string {
  switch (scope) {
    case "system":
      return "System";
    case "user":
      return "Benutzer";
    case "device":
      return "Gerät";
  }
}

export function getScopeColor(scope: Scope): string {
  switch (scope) {
    case "system":
      return "bg-gray-100 text-gray-800";
    case "user":
      return "bg-blue-100 text-blue-800";
    case "device":
      return "bg-purple-100 text-purple-800";
  }
}

export function formatAuditAction(action: SettingAuditEntry["action"]): string {
  switch (action) {
    case "create":
      return "Erstellt";
    case "update":
      return "Geändert";
    case "delete":
      return "Gelöscht";
    case "restore":
      return "Wiederhergestellt";
  }
}

export function getActionColor(action: SettingAuditEntry["action"]): string {
  switch (action) {
    case "create":
      return "text-green-600";
    case "update":
      return "text-blue-600";
    case "delete":
      return "text-red-600";
    case "restore":
      return "text-purple-600";
  }
}

export function getValueTypeLabel(type: ValueType): string {
  switch (type) {
    case "string":
      return "Text";
    case "int":
      return "Ganzzahl";
    case "float":
      return "Dezimalzahl";
    case "bool":
      return "Ja/Nein";
    case "enum":
      return "Auswahl";
    case "time":
      return "Uhrzeit";
    case "duration":
      return "Dauer";
    case "objectRef":
      return "Referenz";
    case "json":
      return "JSON";
  }
}
