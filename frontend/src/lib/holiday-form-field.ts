// Eine Frage des Anmeldeformulars eines Ferienzeitraums (#5198). Das
// Träger-Portal baut sie, die öffentliche Ferienanmeldung zeigt sie.

import type { FormField, FormFieldType } from "./enrollment-form-schema-api";

/**
 * A question of the holiday form: a form field of the online registration
 * plus the markers „Gesundheitsangabe“ and „OGS sieht die Antwort“. An answer to such a field needs the
 * consent to the health information and appears only in the detail of a
 * registration.
 */
export interface HolidayFormField extends FormField {
  /** Always present in answers of the backend, false included. */
  health?: boolean;
  /**
   * The school that hosts a week sees the answer (#5202). Never on a health
   * question; the backend refuses the combination.
   */
  share_with_host?: boolean;
}

/** The field types the holiday form offers; no structured types, no file. */
export const HOLIDAY_FORM_FIELD_TYPES = [
  "boolean",
  "text",
  "textarea",
  "date",
  "select",
] as const satisfies readonly FormFieldType[];

/** One version of the holiday form, as a registration uses it. */
export interface HolidayFormVersion {
  readonly version: number;
  readonly fields: readonly HolidayFormField[];
}
