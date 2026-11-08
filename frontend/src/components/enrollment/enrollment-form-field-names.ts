// Input `name`s of the parent enrollment form. Field errors, the focus jump
// and server error mapping all key on these names (#2515), so every place
// that renders, validates or marks a field builds the name here.

type GuardianField = "first_name" | "last_name" | "email" | "phone";
type ChildField =
  | "first_name"
  | "last_name"
  | "date_of_birth"
  | "target_grade_level"
  | "target_school_class";

export const formFieldName = {
  guardian: (field: GuardianField) => `guardian_${field}`,
  additionalGuardian: (index: number, field: GuardianField) =>
    `additional_guardian_${index}_${field}`,
  custom: (key: string) => `custom_${key}`,
  child: (index: number, field: ChildField) => `children_${index}_${field}`,
  childCustom: (index: number, key: string) =>
    `children_${index}_custom_${key}`,
  childOffering: (index: number, offeringId: string) =>
    `children_${index}_offering_${offeringId}`,
  childCompanionNote: (index: number) =>
    `children_${index}_departure_companion_note`,
  consent: (key: string) => `consent_${key}`,
  signature: "signature",
} as const;

export const CONSENT_FIELD_PREFIX = "consent_";

/**
 * The input name for a field the backend names by its JSON path:
 * `children.0.first_name`, `children.0.custom_data.allergies`,
 * `custom_data.x`, `consent_flags.x`, `additional_guardians.1.email`.
 */
export function fieldNameForServerPath(path: string): string {
  return path
    .replace(
      /^children\.(\d+)\.custom_data\.student\.departure_companion_note$/,
      "children.$1.departure_companion_note",
    )
    .replace(/^additional_guardians\./, "additional_guardian.")
    .replace(/(^|\.)custom_data\./, "$1custom.")
    .replace(/^consent_flags\./, "consent.")
    .replaceAll(".", "_");
}
