/**
 * Shared validation logic for student forms
 * Eliminates duplication between create and edit modals
 */

import type { Student } from "~/lib/student-helpers";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "StudentFormValidation" });

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
 */
export async function handleStudentFormSubmit(
  e: React.FormEvent,
  formData: Partial<Student>,
  validateForm: () => boolean,
  onSubmit: (data: Partial<Student>) => Promise<void>,
  setLoading: (loading: boolean) => void,
  setErrors: (errors: Record<string, string>) => void,
): Promise<void> {
  e.preventDefault();

  if (!validateForm()) {
    return;
  }

  try {
    setLoading(true);
    await onSubmit(formData);
  } catch (error) {
    logger.error("error saving student", { error: String(error) });
    setErrors({
      submit: "Fehler beim Speichern. Bitte versuchen Sie es erneut.",
    });
  } finally {
    setLoading(false);
  }
}
