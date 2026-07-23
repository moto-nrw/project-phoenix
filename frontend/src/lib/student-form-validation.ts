/**
 * Shared validation logic for student forms
 * Eliminates duplication between create and edit modals
 */

import type { DepartureDayKey, Student } from "~/lib/student-helpers";
import { accompaniedWeekdayKeys } from "~/lib/student-helpers";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "StudentFormValidation" });

const GENERIC_SUBMIT_ERROR =
  "Fehler beim Speichern. Bitte versuchen Sie es erneut.";

/**
 * Picks the message to show in the form's submit-error box.
 *
 * Backend validation failures (HTTP 4xx) carry a user-facing German message —
 * e.g. a guardian's email already exists or an invalid relationship type from
 * the atomic student+guardian create flow (#1500). Those must reach the user,
 * because a retry won't fix them. Network/server errors (5xx) and untyped
 * errors stay generic: surfacing "Network error" or "API error: 500" is noise.
 *
 * The HTTP status is attached by the CRUD service's fetch layer
 * (`createCrudService`); an error without a numeric `status` is treated as
 * technical and gets the generic message.
 */
function toSubmitErrorMessage(error: unknown): string {
  if (error instanceof Error) {
    const status = (error as { status?: number }).status;
    const message = error.message.trim();
    if (
      typeof status === "number" &&
      status >= 400 &&
      status < 500 &&
      message !== "" &&
      !message.startsWith("API error")
    ) {
      return message;
    }
  }
  return GENERIC_SUBMIT_ERROR;
}

/**
 * Validates data retention days field
 * @param retentionDays - The retention days value to validate
 * @returns Error message if invalid, undefined if valid
 */
export function validateDataRetentionDays(
  retentionDays: number | null | undefined,
): string | undefined {
  if (retentionDays === null || retentionDays === undefined) {
    return "Aufbewahrungsdauer ist erforderlich (1-31 Tage)";
  }
  if (retentionDays < 1 || retentionDays > 31) {
    return "Aufbewahrungsdauer muss zwischen 1 und 31 Tagen liegen";
  }
  return undefined;
}

/**
 * Validates required student fields
 * @param formData - The form data to validate
 * @param requiredFields - Which fields are required
 * @returns Record of field errors
 */
export function validateStudentForm(
  formData: Partial<Student>,
  requiredFields: {
    firstName?: boolean;
    lastName?: boolean;
    schoolClass?: boolean;
  } = {},
  options: {
    /**
     * The weekdays covered by the child's Laufgemeinschaft links. A link
     * answers "mit wem" for exactly its own weekdays — the backend checks the
     * cover PER DAY, so an accompanied Tuesday backed only by a Monday link
     * still needs the free-text note. Pass "unknown" while the stored links
     * are still loading (or failed to load): claiming "no links" then would
     * block an unrelated edit for the wrong reason, and the backend re-checks
     * the rule against the stored links either way. Omitting the option means
     * "no links" (forms without a companion picker keep requiring the note).
     */
    companionLinkDays?: DepartureDayKey[] | "unknown";
  } = {},
): Record<string, string> {
  const errors: Record<string, string> = {};

  if (requiredFields.firstName && !formData.first_name?.trim()) {
    errors.first_name = "Vorname ist erforderlich";
  }
  if (requiredFields.lastName && !formData.second_name?.trim()) {
    errors.second_name = "Nachname ist erforderlich";
  }
  if (requiredFields.schoolClass && !formData.school_class?.trim()) {
    errors.school_class = "Klasse ist erforderlich";
  }

  const retentionError = validateDataRetentionDays(
    formData.data_retention_days,
  );
  if (retentionError) {
    errors.data_retention_days = retentionError;
  }

  // When a child may leave "Mit anderem Kind", something must say with whom —
  // an accompanied plan with no detail at all defeats the point (#1694). A
  // linked child covers exactly its own weekdays; every accompanied day
  // without a link needs the free-text note.
  const accompaniedDays = accompaniedWeekdayKeys(
    formData.allowed_departure_modes,
    formData.departure_days,
  );
  if (
    accompaniedDays.length > 0 &&
    options.companionLinkDays !== "unknown" &&
    !formData.departure_companion_note?.trim()
  ) {
    const covered = new Set(options.companionLinkDays ?? []);
    if (accompaniedDays.some((day) => !covered.has(day))) {
      errors.departure_companion_note =
        "Bitte angeben, mit welchem Kind das Kind nach Hause geht";
    }
  }

  return errors;
}

/**
 * Handles form submission with loading state and error handling
 * @param e - Form event
 * @param formData - The form data to submit
 * @param validateForm - Validation function
 * @param onSubmit - Submit handler
 * @param setLoading - Loading state setter
 * @param setErrors - Error state setter
 * @param onError - Optional; fired after errors are set (client validation
 *   failure or server rejection) so callers can scroll to the first error.
 */
export async function handleStudentFormSubmit(
  e: React.FormEvent,
  formData: Partial<Student>,
  validateForm: () => boolean,
  onSubmit: (data: Partial<Student>) => Promise<void>,
  setLoading: (loading: boolean) => void,
  setErrors: (errors: Record<string, string>) => void,
  onError?: () => void,
): Promise<void> {
  e.preventDefault();

  if (!validateForm()) {
    onError?.();
    return;
  }

  try {
    setLoading(true);
    await onSubmit(formData);
  } catch (error) {
    logger.error("error saving student", { error: String(error) });
    setErrors({ submit: toSubmitErrorMessage(error) });
    onError?.();
  } finally {
    setLoading(false);
  }
}
