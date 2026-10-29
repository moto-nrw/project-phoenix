// Eine Frage des Anmeldeformulars eines Ferienzeitraums (#5198). Das
// Träger-Portal baut sie, die öffentliche Ferienanmeldung zeigt sie.

import type {
  FormLegalBlock,
  FormField,
  FormFieldType,
} from "./enrollment-form-schema-api";

/**
 * A question of the holiday form: a form field of the online registration
 * plus the marker „Gesundheitsangabe“. An answer to such a field needs the
 * consent to the health information and appears only in the detail of a
 * registration.
 */
export interface HolidayFormField extends FormField {
  /** Always present in answers of the backend, false included. */
  health?: boolean;
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
  readonly legalBlocks?: readonly FormLegalBlock[];
  readonly version: number;
  readonly fields: readonly HolidayFormField[];
}

export type HolidayLegalBlockWire = Omit<FormLegalBlock, "attachments"> & {
  attachments?: { label: string; document_url: string; document_id?: string }[];
};
/** Stored document IDs stay IDs until a reader chooses its authorized route. */
export function holidayLegalBlocks(
  blocks: readonly HolidayLegalBlockWire[] | null | undefined,
): FormLegalBlock[] {
  return (blocks ?? []).map((block) => ({
    ...block,
    attachments: block.attachments?.map((a) => ({
      label: a.label,
      document_url: a.document_id ?? "",
    })),
  }));
}
