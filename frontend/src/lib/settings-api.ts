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

type AccessPolicy = "shared" | "admin_only" | "operator_only";

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
    | "select";
  default: unknown;
  value: unknown;
  is_default: boolean;
  writable: boolean;
  visible: boolean;
  sort_order: number;
  access_policy: AccessPolicy;
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
          // default must restore the "Standard" badge without a refetch). All
          // other types just got an override row written, so is_default is
          // false and the reset button stays available.
          const optimisticIsDefault =
            item.type === "boolean"
              ? JSON.stringify(value) === JSON.stringify(item.default)
              : false;
          const updated =
            item.key === key
              ? {
                  ...item,
                  value: optimisticValue,
                  is_default: optimisticIsDefault,
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
 * Fetch the settings schema. Returns null if user has no access or session
 * is not ready (expected cases — no error logged).
 * Throws only on unexpected server errors.
 */
export async function fetchSettingsSchema(): Promise<SettingsSchema | null> {
  let response: Response;
  try {
    response = await sessionFetch("/api/settings/schema", { method: "GET" });
  } catch (error) {
    // No token / session not ready — expected during initial page load
    if (
      error instanceof Error &&
      error.message === "No authentication token available"
    ) {
      return null;
    }
    // Network error — unexpected
    logger.error("fetch_settings_schema_network_error", {
      error: error instanceof Error ? error.message : String(error),
    });
    return null;
  }

  // 401/403 = not authenticated or no permission — expected, not an error
  if (response.status === 401 || response.status === 403) {
    return null;
  }

  if (!response.ok) {
    logger.error("fetch_settings_schema_failed", {
      status: response.status,
    });
    throw new Error(
      `Einstellungen konnten nicht geladen werden (${response.status})`,
    );
  }

  const result = (await response.json()) as ApiResponse<SettingsSchema>;
  return result.data;
}

/**
 * Extracts the validation reason from a backend error and translates it
 * to a short German message suitable for inline display below a field.
 */
function translateValidationError(apiError: string): string {
  // Backend format: "invalid value for setting {key}: {reason}"
  const colonIdx = apiError.lastIndexOf(": ");
  const reason = colonIdx >= 0 ? apiError.slice(colonIdx + 2) : apiError;

  if (reason === "value is required") return "Dieser Wert ist erforderlich.";
  if (reason === "expected a number") return "Bitte eine Zahl eingeben.";

  const belowMin = /below minimum (\d+)/.exec(reason);
  if (belowMin) return `Minimum: ${belowMin[1]}`;

  const aboveMax = /exceeds maximum (\d+)/.exec(reason);
  if (aboveMax) return `Maximum: ${aboveMax[1]}`;

  return "Ungültiger Wert.";
}

/**
 * Set a setting value. Returns a user-facing error message on failure,
 * or null on success.
 */
export async function setSettingValue(
  key: string,
  value: unknown,
): Promise<string | null> {
  try {
    const response = await sessionFetch(`/api/settings/values/${key}`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ value }),
    });

    if (!response.ok) {
      // Log the raw API error for debugging, show clean German message to user
      let apiError = "";
      try {
        const result = (await response.json()) as { error?: string };
        apiError = result.error ?? "";
      } catch {
        // ignore parse errors
      }
      logger.warn("set_setting_value_rejected", {
        key,
        status: response.status,
        api_error: apiError,
      });

      if (response.status === 404) {
        return "Einstellung nicht gefunden.";
      }
      if (response.status === 400 && apiError) {
        return translateValidationError(apiError);
      }
      return "Einstellung konnte nicht gespeichert werden.";
    }

    return null;
  } catch (error) {
    logger.warn("set_setting_value_failed", {
      key,
      error: error instanceof Error ? error.message : String(error),
    });
    return "Netzwerkfehler beim Speichern der Einstellung.";
  }
}

/**
 * Reset a setting value. Returns a user-facing error message on failure,
 * or null on success.
 */
export async function resetSettingValue(key: string): Promise<string | null> {
  try {
    const response = await sessionFetch(`/api/settings/values/${key}`, {
      method: "DELETE",
    });

    if (!response.ok && response.status !== 204) {
      logger.warn("reset_setting_value_rejected", {
        key,
        status: response.status,
      });
      return "Einstellung konnte nicht zurückgesetzt werden.";
    }

    return null;
  } catch (error) {
    logger.warn("reset_setting_value_failed", {
      key,
      error: error instanceof Error ? error.message : String(error),
    });
    return "Netzwerkfehler beim Zurücksetzen der Einstellung.";
  }
}

/**
 * Reveal the unmasked value of a password/PIN setting.
 * Returns the raw value on success, or null on failure.
 */
export async function revealSettingValue(key: string): Promise<string | null> {
  try {
    const response = await sessionFetch(`/api/settings/values/${key}/reveal`, {
      method: "GET",
    });

    if (!response.ok) {
      logger.warn("reveal_setting_value_failed", {
        key,
        status: response.status,
      });
      return null;
    }

    // The Next.js proxy wraps the backend response, so the structure is:
    // { status, data: { status, data: { value }, message } }
    const json = await response.json();
    const value = json?.data?.data?.value ?? json?.data?.value;
    return typeof value === "string" ? value : null;
  } catch (error) {
    logger.warn("reveal_setting_value_error", {
      key,
      error: error instanceof Error ? error.message : String(error),
    });
    return null;
  }
}
