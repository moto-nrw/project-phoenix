/**
 * Rules of the form builder's custom fields that the editor checks in more
 * than one place: the saved key of a field, which types only exist as fixed
 * suggestions, which questions can control another one, and where a field
 * may be asked per child. The backend enforces the same rules in
 * backend/modules/enrollment/form_schema.go.
 */
import type {
  FormField,
  FormFieldType,
} from "~/lib/enrollment-form-schema-api";

/**
 * Lowercase letters, digits and single underscores, derived from a label
 * ("Allergien / Unverträglichkeiten" -> "allergien_unvertraglichkeiten").
 *
 * A character loop on purpose: the regex form needs quantifiers on
 * character classes, the shape static analysis flags as potentially
 * super-linear. Inputs are capped; labels never need more than a few
 * hundred characters.
 */
export function normalizeFieldKey(value: string): string {
  const MAX_INPUT = 256;
  const source = value
    .slice(0, MAX_INPUT)
    .trim()
    .toLowerCase()
    .normalize("NFD");

  let out = "";
  let pendingUnderscore = false;
  for (let i = 0; i < source.length; i++) {
    const code = source.charCodeAt(i);
    // Combining diacritical marks introduced by NFD — skip.
    if (code >= 0x0300 && code <= 0x036f) continue;
    const isLower = code >= 0x61 && code <= 0x7a; // a-z
    const isDigit = code >= 0x30 && code <= 0x39; // 0-9
    if (isLower || isDigit) {
      if (pendingUnderscore && out.length > 0) {
        out += "_";
      }
      out += source[i];
      pendingUnderscore = false;
    } else {
      // Any other character → collapse into a single underscore
      // (deferred so we don't leave a trailing one).
      pendingUnderscore = true;
    }
  }
  return out;
}

/**
 * The key a custom field is saved under: the stored key, else one derived
 * from the label; an information text without either is numbered by its
 * position. Empty for a question whose label has no letter or digit.
 */
export function savedFieldKey(field: FormField, index: number): string {
  const key = field.key.trim() || normalizeFieldKey(field.label);
  if (key !== "" || field.type !== "information") return key;
  return `infotext_${index + 1}`;
}

/** Types only a fixed suggestion (a field with a target) may use. */
const STRUCTURED_FIELD_TYPES: ReadonlySet<FormFieldType> = new Set([
  "phone_list",
  "weekday_schedule",
  "weekday_boolean",
  "weekday_mode",
  "weekday_multi_mode",
  "contact_list",
]);

export function isStructuredFieldType(type: FormFieldType): boolean {
  return STRUCTURED_FIELD_TYPES.has(type);
}

/** A file field always belongs to the whole enrollment, never to a child. */
export function canAskPerChild(field: FormField): boolean {
  return field.type !== "file";
}

/** Only yes/no and selection answers can control a visibility rule. */
export function hasControllingType(field: FormField): boolean {
  return field.type === "boolean" || field.type === "select";
}

/**
 * Whether `candidate` can be the controlling question of a visibility rule
 * on `dependent`: its type can control, and its answer exists in the
 * dependent's scope (a family question cannot depend on a per-child one).
 */
export function canControlVisibility(
  candidate: FormField,
  dependent: FormField,
): boolean {
  return (
    hasControllingType(candidate) &&
    (Boolean(dependent.applies_to_child) || !candidate.applies_to_child)
  );
}
