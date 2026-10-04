/**
 * Shared validation logic for student forms
 * Eliminates duplication between create and edit modals
 */

import type { DepartureDayKey, Student } from "~/lib/student-helpers";
import { accompaniedWeekdayKeys } from "~/lib/student-helpers";

/**
 * Validates data retention days field
 * @param retentionDays - The retention days value to validate
 * @returns Error message if invalid, undefined if valid
 */
export function validateDataRetentionDays(
  retentionDays: number | null | undefined,
): string | undefined {
  if (retentionDays === null || retentionDays === undefined) {
    return "Bitte geben Sie eine Zahl von 1 bis 31 ein.";
  }
  if (retentionDays < 1 || retentionDays > 31) {
    return "Bitte geben Sie eine Zahl von 1 bis 31 ein.";
  }
  return undefined;
}

/**
 * Validates required student fields
 * @param formData - The form data to validate
 * @param requiredFields - Which fields are required
 * @returns Hint per invalid field, keyed by the API field name, so the
 *   shared error path marks and focuses the same control a backend field
 *   error would (#2513).
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
    errors.first_name = "Bitte geben Sie den Vornamen ein.";
  }
  if (requiredFields.lastName && !formData.second_name?.trim()) {
    errors.last_name = "Bitte geben Sie den Nachnamen ein.";
  }
  if (requiredFields.schoolClass && !formData.school_class?.trim()) {
    errors.school_class = "Bitte geben Sie die Klasse ein.";
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
        "Bitte geben Sie an, mit wem das Kind nach Hause geht.";
    }
  }

  return errors;
}
