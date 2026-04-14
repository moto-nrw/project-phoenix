import { operatorFetch, isOperatorApiError } from "./api-helpers";
import type { SettingsSchema } from "~/lib/settings-api";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "OperatorSettingsApi" });

/**
 * Fetch the settings schema for a specific school.
 * Operators see all settings regardless of per-setting permissions.
 */
export async function fetchOperatorSettingsSchema(
  schoolId: string,
): Promise<SettingsSchema | null> {
  try {
    return await operatorFetch<SettingsSchema>(
      `/api/operator/provisioning/schools/${schoolId}/settings/schema`,
      { method: "GET" },
    );
  } catch (error) {
    if (isOperatorApiError(error) && error.status === 404) {
      return null;
    }
    logger.error("fetch_operator_settings_schema_failed", {
      school_id: schoolId,
      error: error instanceof Error ? error.message : String(error),
    });
    throw error;
  }
}

function translateValidationError(apiError: string): string {
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
 * Set a setting value for a school. Returns a user-facing error message
 * on failure, or null on success.
 */
export async function setOperatorSettingValue(
  schoolId: string,
  key: string,
  value: unknown,
): Promise<string | null> {
  try {
    await operatorFetch<unknown>(
      `/api/operator/provisioning/schools/${schoolId}/settings/values/${key}`,
      { method: "PUT", body: { value } },
    );
    return null;
  } catch (error) {
    if (isOperatorApiError(error)) {
      logger.warn("set_operator_setting_value_rejected", {
        school_id: schoolId,
        key,
        status: error.status,
        api_error: error.message,
      });
      if (error.status === 404) return "Einstellung nicht gefunden.";
      if (error.status === 400) return translateValidationError(error.message);
      return "Einstellung konnte nicht gespeichert werden.";
    }
    logger.warn("set_operator_setting_value_failed", {
      school_id: schoolId,
      key,
      error: error instanceof Error ? error.message : String(error),
    });
    return "Netzwerkfehler beim Speichern der Einstellung.";
  }
}

/**
 * Reset a setting value for a school. Returns a user-facing error message
 * on failure, or null on success.
 */
export async function resetOperatorSettingValue(
  schoolId: string,
  key: string,
): Promise<string | null> {
  try {
    await operatorFetch<unknown>(
      `/api/operator/provisioning/schools/${schoolId}/settings/values/${key}`,
      { method: "DELETE" },
    );
    return null;
  } catch (error) {
    logger.warn("reset_operator_setting_value_failed", {
      school_id: schoolId,
      key,
      error: error instanceof Error ? error.message : String(error),
    });
    return "Einstellung konnte nicht zurückgesetzt werden.";
  }
}

/**
 * Reveal the unmasked value of a password/PIN setting for a school.
 */
export async function revealOperatorSettingValue(
  schoolId: string,
  key: string,
): Promise<string | null> {
  try {
    const result = await operatorFetch<{ value: unknown }>(
      `/api/operator/provisioning/schools/${schoolId}/settings/values/${key}/reveal`,
      { method: "GET" },
    );
    return typeof result.value === "string" ? result.value : null;
  } catch (error) {
    logger.warn("reveal_operator_setting_value_failed", {
      school_id: schoolId,
      key,
      error: error instanceof Error ? error.message : String(error),
    });
    return null;
  }
}
