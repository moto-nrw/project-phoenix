import { ApiError, apiErrorFromResponse } from "./api-error";
import { sessionFetch } from "./session-cache";
import { createLogger } from "./logger";

const logger = createLogger({ component: "SettingsApi" });

export const SETTINGS_SCHEMA_SWR_KEY = "settings-schema";

// --- Types ---

export interface SettingsSchema {
  tabs: SchemaTab[];
}

export interface SchemaTab {
  key: string;
  label: string;
  categories: SchemaCategory[];
}

export interface SchemaCategory {
  key: string;
  label: string;
  items: ResolvedSetting[];
}

type AccessPolicy =
  | "shared"
  | "admin_only"
  | "operator_only"
  // Operator ändert, Schule liest (ADR 0048): die Löschfristen.
  | "operator_managed";

/**
 * Where a value may be set (ADR 0086): only at the school, at the Träger with
 * a school deviation allowed, or only at the Träger.
 */
export type SettingLevels = "schule" | "beide" | "traeger";

/** Where the effective value comes from: registry default, Träger, school. */
export type SettingSource = "default" | "traeger" | "school";

export interface ResolvedSetting {
  key: string;
  label: string;
  description: string;
  type:
    | "boolean"
    | "number"
    | "time"
    | "date"
    | "text"
    | "textarea"
    | "password"
    | "select"
    | "multiselect";
  default: unknown;
  value: unknown;
  /**
   * School schema: the school has no value of its own, so a reset changes
   * nothing (booleans: the value equals the inherited one). Träger schema:
   * the Träger has no value.
   */
  is_default: boolean;
  writable: boolean;
  visible: boolean;
  sort_order: number;
  access_policy: AccessPolicy;
  levels: SettingLevels;
  source: SettingSource;
  /**
   * Last change of the Träger value (ISO timestamp). Absent without a Träger
   * value; stays set beneath a school deviation.
   */
  traeger_changed_at?: string | null;
  validation?: {
    required?: boolean;
    min?: number;
    max?: number;
    pattern?: string;
  } | null;
  depends_on?: {
    key: string;
    condition: string;
    value: unknown;
  } | null;
  options?: {
    static?: { label: string; value: unknown }[];
  } | null;
}

/** Returns a resolved setting value without duplicating schema traversal. */
export function getSettingValue(
  schema: SettingsSchema | null | undefined,
  key: string,
): unknown | undefined {
  if (!schema || !Array.isArray(schema.tabs)) return undefined;
  for (const tab of schema.tabs) {
    if (!tab || !Array.isArray(tab.categories)) continue;
    for (const category of tab.categories) {
      if (!category || !Array.isArray(category.items)) continue;
      for (const item of category.items) {
        if (item?.key === key) return item.value;
      }
    }
  }
  return undefined;
}

interface ApiResponse<T> {
  status: string;
  data: T;
  message?: string;
}

// --- Optimistic Update Helper ---

/**
 * The value a boolean falls back to without the school's own value: the
 * default, or the Träger value where there is one (ADR 0086). The schema
 * carries no Träger value of its own; for a boolean it is the current value
 * while it comes from the Träger or equals it, and its opposite otherwise.
 */
function inheritedBooleanValue(item: ResolvedSetting): unknown {
  if (!item.traeger_changed_at || typeof item.value !== "boolean") {
    return item.default;
  }
  if (item.source === "traeger" || item.is_default) return item.value;
  return !item.value;
}

/**
 * Returns a new schema with `key` set to `value` and every entry's `visible`
 * flag re-evaluated against its `depends_on` rule. Pure — does not mutate the
 * input. Used as the optimistic-data argument to SWR's mutate so the UI
 * reflects the change before the server confirms.
 */
export function applyOptimisticSchemaUpdate(
  schema: SettingsSchema,
  key: string,
  value: unknown,
): SettingsSchema {
  const valueMap = new Map<string, unknown>();
  const itemMap = new Map<string, ResolvedSetting>();
  for (const tab of schema.tabs) {
    for (const cat of tab.categories) {
      for (const item of cat.items) {
        valueMap.set(item.key, item.key === key ? value : item.value);
        itemMap.set(item.key, item);
      }
    }
  }

  const visibilityMemo = new Map<string, boolean>();
  const evaluateVisibility = (
    item: ResolvedSetting,
    visiting = new Set<string>(),
  ): boolean => {
    const cached = visibilityMemo.get(item.key);
    if (cached !== undefined) return cached;
    if (!item.depends_on) {
      visibilityMemo.set(item.key, true);
      return true;
    }
    if (visiting.has(item.key)) {
      visibilityMemo.set(item.key, false);
      return false;
    }
    visiting.add(item.key);
    const parent = itemMap.get(item.depends_on.key);
    if (!parent || !evaluateVisibility(parent, visiting)) {
      visibilityMemo.set(item.key, false);
      visiting.delete(item.key);
      return false;
    }
    const parentVal = valueMap.get(item.depends_on.key);
    const cond = item.depends_on.condition;
    const expected = item.depends_on.value;
    let visible = true;
    if (cond === "eq") {
      visible = JSON.stringify(parentVal) === JSON.stringify(expected);
    } else if (cond === "neq") {
      visible = JSON.stringify(parentVal) !== JSON.stringify(expected);
    } else if (cond === "not_empty") {
      visible = parentVal != null && parentVal !== "";
    } else {
      visible = false;
    }
    visibilityMemo.set(item.key, visible);
    visiting.delete(item.key);
    return visible;
  };

  return {
    ...schema,
    tabs: schema.tabs.map((tab) => ({
      ...tab,
      categories: tab.categories.map((cat) => ({
        ...cat,
        items: cat.items.map((item) => {
          const optimisticValue = item.type === "password" ? "••••••" : value;
          // Mirror the backend's is_default semantics (issue #1680): boolean
          // toggles are value-based (no reset button, so toggling back to the
          // inherited value must restore the "Standard" badge without a
          // refetch). The inherited value is the Träger value where there is
          // one (ADR 0086), else the default. All other
          // types just got an override row written, so is_default is false
          // and the reset button stays available.
          const optimisticIsDefault =
            item.type === "boolean"
              ? JSON.stringify(value) ===
                JSON.stringify(inheritedBooleanValue(item))
              : false;
          const updated =
            item.key === key
              ? {
                  ...item,
                  value: optimisticValue,
                  is_default: optimisticIsDefault,
                  // A school save always writes the school's own value.
                  source: "school" as const,
                }
              : item;
          return { ...updated, visible: evaluateVisibility(updated) };
        }),
      })),
    })),
  };
}

// --- API Functions ---

/**
 * Fetch the settings schema. Returns null when no session is ready yet or the
 * account may not read settings (401/403, expected, no error logged).
 * Any other failure throws an ApiError (#2517): a network failure must not
 * look like "no settings for your role".
 */
export async function fetchSettingsSchema(): Promise<SettingsSchema | null> {
  let response: Response;
  try {
    response = await sessionFetch("/api/settings/schema", { method: "GET" });
  } catch (error) {
    // No token yet: the session is still starting, not an error.
    if (error instanceof ApiError && error.status === 401) {
      return null;
    }
    logger.error("fetch_settings_schema_network_error", {
      error: error instanceof Error ? error.message : String(error),
    });
    throw error;
  }

  // 401/403 = not authenticated or no permission — expected, not an error
  if (response.status === 401 || response.status === 403) {
    return null;
  }

  if (!response.ok) {
    logger.error("fetch_settings_schema_failed", {
      status: response.status,
    });
    throw await apiErrorFromResponse(
      response,
      `Einstellungen konnten nicht geladen werden (${response.status})`,
    );
  }

  const result = (await response.json()) as ApiResponse<SettingsSchema>;
  return result.data;
}

/**
 * Save a setting value (#2517). Throws an ApiError with the backend code, so
 * the caller shows the catalog text on the shared error path.
 */
export async function saveSettingValue(
  key: string,
  value: unknown,
): Promise<void> {
  const response = await sessionFetch(`/api/settings/values/${key}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ value }),
  });
  if (!response.ok) {
    logger.warn("save_setting_value_rejected", {
      key,
      status: response.status,
    });
    throw await apiErrorFromResponse(
      response,
      `Setting ${key} could not be saved (${response.status})`,
    );
  }
}

/** Reset a setting to its default (#2517). Throws an ApiError on failure. */
export async function clearSettingValue(key: string): Promise<void> {
  const response = await sessionFetch(`/api/settings/values/${key}`, {
    method: "DELETE",
  });
  if (!response.ok && response.status !== 204) {
    logger.warn("clear_setting_value_rejected", {
      key,
      status: response.status,
    });
    throw await apiErrorFromResponse(
      response,
      `Setting ${key} could not be reset (${response.status})`,
    );
  }
}

/**
 * Reveal the unmasked value of a password/PIN setting.
 * Returns the raw value, or null when the setting holds no text value.
 * A failed request throws an ApiError (#2517); the field shows it.
 */
export async function revealSettingValue(key: string): Promise<string | null> {
  const response = await sessionFetch(`/api/settings/values/${key}/reveal`, {
    method: "GET",
  });

  if (!response.ok) {
    logger.warn("reveal_setting_value_failed", {
      key,
      status: response.status,
    });
    throw await apiErrorFromResponse(
      response,
      `Setting ${key} could not be revealed (${response.status})`,
    );
  }

  // The Next.js proxy wraps the backend response, so the structure is:
  // { status, data: { status, data: { value }, message } }
  const json = await response.json();
  const value = json?.data?.data?.value ?? json?.data?.value;
  return typeof value === "string" ? value : null;
}
