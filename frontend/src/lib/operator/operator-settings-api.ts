import { operatorFetch, isOperatorApiError } from "./api-helpers";
import type { SettingsSchema } from "~/lib/settings-api";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "OperatorSettingsApi" });

interface BookingAuthorityImpactChild {
  studentId: string;
  firstName: string;
  lastName: string;
  schoolClass: string;
  firstBookinglessDay?: string;
}

export interface BookingAuthorityImpact {
  referenceDate: string;
  blockingChildren: BookingAuthorityImpactChild[];
  plannedCompletions: BookingAuthorityImpactChild[];
}

interface BookingAuthorityImpactWireChild {
  student_id: string;
  first_name: string;
  last_name: string;
  school_class: string;
  first_bookingless_day?: string;
}

interface BookingAuthorityImpactWire {
  reference_date: string;
  blocking_children: BookingAuthorityImpactWireChild[];
  planned_completions: BookingAuthorityImpactWireChild[];
}

export async function fetchBookingAuthorityImpact(
  schoolId: string,
): Promise<BookingAuthorityImpact> {
  const wire = await operatorFetch<BookingAuthorityImpactWire>(
    `/api/operator/provisioning/schools/${schoolId}/settings/booking-authority-impact`,
    { method: "GET" },
  );
  return {
    referenceDate: wire.reference_date,
    blockingChildren: wire.blocking_children.map(mapImpactChild),
    plannedCompletions: wire.planned_completions.map(mapImpactChild),
  };
}

function mapImpactChild(
  child: BookingAuthorityImpactWireChild,
): BookingAuthorityImpactChild {
  return {
    studentId: child.student_id,
    firstName: child.first_name,
    lastName: child.last_name,
    schoolClass: child.school_class,
    firstBookinglessDay: child.first_bookingless_day,
  };
}

/**
 * Fetch the settings schema for a specific school.
 * Operators see all settings regardless of per-setting permissions. Every
 * failure throws, an unknown school too, so the page shows it (#2519).
 */
export async function fetchOperatorSettingsSchema(
  schoolId: string,
): Promise<SettingsSchema> {
  try {
    return await operatorFetch<SettingsSchema>(
      `/api/operator/provisioning/schools/${schoolId}/settings/schema`,
      { method: "GET" },
    );
  } catch (error) {
    logger.error("fetch_operator_settings_schema_failed", {
      school_id: schoolId,
      error: error instanceof Error ? error.message : String(error),
    });
    throw error;
  }
}

/**
 * Set a setting value for a school. Throws the ApiError of a failed save,
 * which the settings field shows on the shared error path (#2519).
 */
export async function setOperatorSettingValue(
  schoolId: string,
  key: string,
  value: unknown,
): Promise<void> {
  try {
    await operatorFetch<unknown>(
      `/api/operator/provisioning/schools/${schoolId}/settings/values/${key}`,
      { method: "PUT", body: { value } },
    );
  } catch (error) {
    logger.warn("set_operator_setting_value_failed", {
      school_id: schoolId,
      key,
      status: isOperatorApiError(error) ? error.status : undefined,
      error: error instanceof Error ? error.message : String(error),
    });
    throw error;
  }
}

/** Reset a setting value for a school. Throws the ApiError of a failure. */
export async function resetOperatorSettingValue(
  schoolId: string,
  key: string,
): Promise<void> {
  try {
    await operatorFetch<unknown>(
      `/api/operator/provisioning/schools/${schoolId}/settings/values/${key}`,
      { method: "DELETE" },
    );
  } catch (error) {
    logger.warn("reset_operator_setting_value_failed", {
      school_id: schoolId,
      key,
      error: error instanceof Error ? error.message : String(error),
    });
    throw error;
  }
}

/**
 * Reveal the unmasked value of a password/PIN setting for a school.
 * Returns null when the setting holds no text value; a failed request throws.
 */
export async function revealOperatorSettingValue(
  schoolId: string,
  key: string,
): Promise<string | null> {
  const result = await operatorFetch<{ value: unknown }>(
    `/api/operator/provisioning/schools/${schoolId}/settings/values/${key}/reveal`,
    { method: "GET" },
  );
  return typeof result.value === "string" ? result.value : null;
}
