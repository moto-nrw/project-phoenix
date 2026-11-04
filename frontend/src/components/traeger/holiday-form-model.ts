// Entwurf des Anmeldeformulars eines Ferienzeitraums (#5198): Fragen je
// Familie oder je Kind, Pflicht, Bedingung, Gesundheitsangabe und die
// Freigabe für die ausrichtende OGS (#5202). Die
// Feldnamen der Prüfung folgen dem Backend ("fields.2.label"), damit Fehler
// vom Server an derselben Frage landen.

import { normalizeFieldKey } from "~/components/enrollment/form-field-builder";
import type { HolidayFormField } from "~/lib/traeger-holiday-form-api";

/** A question as the holiday form stores it: no target, attachment or translation. */
export function prepareHolidayFieldsForSave(
  fields: readonly HolidayFormField[],
): HolidayFormField[] {
  return fields.map((field, index) => {
    const common = {
      label: field.label.trim(),
      sort_order: index,
      applies_to_child: Boolean(field.applies_to_child),
      ...(field.visible_when ? { visible_when: field.visible_when } : {}),
    };
    if (field.type === "information") {
      return {
        ...common,
        key:
          field.key.trim() ||
          normalizeFieldKey(field.label) ||
          `infotext_${index + 1}`,
        type: "information",
        content: field.content?.trim() ?? "",
      };
    }
    return {
      ...common,
      key: field.key.trim() || normalizeFieldKey(field.label),
      type: field.type,
      required: Boolean(field.required),
      help_text: field.help_text?.trim() ?? "",
      ...(field.type === "select" ? { options: field.options ?? [] } : {}),
      // Always sent, false included: the marker tells health questions from
      // the others, also for later readers such as #5202.
      health: Boolean(field.health),
      // A health question can be shared with the hosting school since #4722.
      share_with_host: Boolean(field.share_with_host),
    };
  });
}

/**
 * Adds the questions of the standard template whose key the draft lacks, at
 * the end and in the template's order. Questions the Träger-Büro changed or
 * kept stay as they are.
 */
export function insertTemplateFields(
  fields: readonly HolidayFormField[],
  template: readonly HolidayFormField[],
): HolidayFormField[] {
  const keys = new Set(fields.map((field) => field.key));
  const missing = template
    .filter((field) => !keys.has(field.key))
    .map((field, index) => ({
      ...field,
      sort_order: fields.length + index,
    }));
  return [...fields, ...missing];
}

/** Whether a question of the template is missing in the draft. */
export function templateFieldsMissing(
  fields: readonly HolidayFormField[],
  template: readonly HolidayFormField[],
): number {
  const keys = new Set(fields.map((field) => field.key));
  return template.filter((field) => !keys.has(field.key)).length;
}

/** A stable comparison value for unsaved changes. */
export function holidayFieldsSignature(
  fields: readonly HolidayFormField[],
): string {
  return JSON.stringify(prepareHolidayFieldsForSave(fields));
}

const FILL_IN = "Bitte füllen Sie dieses Feld aus.";

/** Checks the draft before saving; empty means it can be sent. */
export function validateHolidayForm(
  fields: readonly HolidayFormField[],
): Record<string, string> {
  const errors: Record<string, string> = {};
  const prepared = prepareHolidayFieldsForSave(fields);
  const seen = new Set<string>();
  prepared.forEach((field, index) => {
    if (field.type === "information") {
      if (field.content?.trim() === "") {
        errors[`fields.${index}.content`] = FILL_IN;
      }
    } else if (field.label === "") {
      errors[`fields.${index}.label`] = FILL_IN;
    } else if (field.key === "") {
      errors[`fields.${index}.label`] =
        "Bitte verwenden Sie in der Frage mindestens einen Buchstaben oder eine Zahl.";
    }
    if (field.type === "select" && (field.options ?? []).length === 0) {
      errors[`fields.${index}.options`] =
        "Bitte geben Sie mindestens eine Auswahl an.";
    }
    if (field.key !== "" && seen.has(field.key)) {
      errors[`fields.${index}.label`] =
        "Diese Frage gibt es schon. Bitte formulieren Sie sie anders.";
    }
    seen.add(field.key);
  });
  return errors;
}
