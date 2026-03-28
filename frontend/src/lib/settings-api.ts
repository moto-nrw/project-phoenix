import { sessionFetch } from "./session-cache";
import { createLogger } from "./logger";

const logger = createLogger({ component: "SettingsApi" });

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

export interface ResolvedSetting {
  key: string;
  label: string;
  description: string;
  type: "boolean" | "number" | "time" | "text" | "password" | "select";
  default: unknown;
  value: unknown;
  is_default: boolean;
  writable: boolean;
  visible: boolean;
  sort_order: number;
  validation?: {
    required?: boolean;
    min?: number;
    max?: number;
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

interface ApiResponse<T> {
  status: string;
  data: T;
  message?: string;
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
    return null;
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
