// Die Feldpfade einer Änderung der Angaben (#5629) als Text für den Verlauf:
// „guardian.phone“ wird „Telefon des Elternteils“, „children.0.last_name“
// wird „Nachname von Kind 1“. Der Verlauf nennt nur die Felder, nie Werte.

import type { HolidayFormField } from "~/lib/holiday-form-field";

/** The title of every edit state of the detail. */
export const EDIT_TITLE = "Angaben ändern";

/** The hint at the top of every edit state. */
export const EDIT_HINT =
  "Die Änderung steht im Verlauf. Ein schon erstelltes Schreiben an die Familie bleibt, wie es ist.";

/** Why a cancelled registration shows no „Ändern“. */
export const CANCELLED_EDIT_TEXT =
  "Die Anmeldung ist storniert. Die Angaben lassen sich nicht mehr ändern.";

/**
 * The questions of one scope the Träger-Büro may answer, as the backend
 * checks them (`holidayEditForm`): no question with health information, its
 * answer stays sealed. A question whose condition depends on a health
 * question, directly or through another such question, cannot be evaluated
 * without the sealed answer: it is shown without its condition and is
 * optional, so its answer stays visible and is kept.
 */
export function editableQuestions(
  fields: readonly HolidayFormField[],
  child: boolean,
): HolidayFormField[] {
  const sealed = new Set(
    fields.filter((field) => field.health).map((field) => field.key),
  );
  const unknown = new Set(sealed);
  for (let changed = true; changed;) {
    changed = false;
    for (const field of fields) {
      const condition = field.visible_when;
      if (
        condition?.source === "field" &&
        condition.field !== undefined &&
        unknown.has(condition.field) &&
        !unknown.has(field.key)
      ) {
        unknown.add(field.key);
        changed = true;
      }
    }
  }
  return fields
    .filter(
      (field) =>
        field.type !== "information" &&
        !sealed.has(field.key) &&
        Boolean(field.applies_to_child) === child,
    )
    .map((field) =>
      unknown.has(field.key)
        ? { ...field, visible_when: null, required: false }
        : field,
    );
}

const GUARDIAN: Readonly<Record<string, string>> = {
  first_name: "Vorname des Elternteils",
  last_name: "Nachname des Elternteils",
  email: "E-Mail des Elternteils",
  phone: "Telefon des Elternteils",
};

const SECOND_GUARDIAN: Readonly<Record<string, string>> = {
  first_name: "Vorname der zweiten Person",
  last_name: "Nachname der zweiten Person",
  email: "E-Mail der zweiten Person",
  phone: "Telefon der zweiten Person",
};

const CHILD: Readonly<Record<string, string>> = {
  first_name: "Vorname",
  last_name: "Nachname",
  birth_date: "Geburtsdatum",
  home_school_id: "Schule",
  guest_school_name: "eigene Schule",
};

const CONTACT: Readonly<Record<string, string>> = {
  name: "Name",
  phone: "Telefon",
  relationship: "Beziehung",
};

const CONTACT_KINDS: Readonly<Record<string, string>> = {
  emergency_contacts: "Notfallkontakt",
  pickup_persons: "Abholperson",
};

function position(segment: string | undefined): number {
  return Number(segment ?? 0) + 1;
}

function questionLabel(
  fields: readonly HolidayFormField[],
  key: string | undefined,
): string {
  return fields.find((field) => field.key === key)?.label ?? "eine Antwort";
}

function fieldLabel(path: string, fields: readonly HolidayFormField[]): string {
  const [section, second, third, fourth] = path.split(".");
  if (section === "guardian") return GUARDIAN[second ?? ""] ?? "Elternteil";
  if (section === "guardian_address") return "Anschrift";
  if (section === "second_guardian") {
    return second
      ? (SECOND_GUARDIAN[second] ?? "zweite sorgeberechtigte Person")
      : "zweite sorgeberechtigte Person";
  }
  if (section === "custom_data") return questionLabel(fields, second);
  if (section === "children") {
    const child = `Kind ${position(second)}`;
    if (third === "custom_data") {
      return `${questionLabel(fields, fourth)} von ${child}`;
    }
    return `${CHILD[third ?? ""] ?? "Angabe"} von ${child}`;
  }
  const kind = CONTACT_KINDS[section ?? ""];
  if (kind) {
    const contact = `${kind} ${position(second)}`;
    return third ? `${CONTACT[third] ?? "Angabe"} von ${contact}` : contact;
  }
  return "eine Angabe";
}

/** The changed fields of an edit, one label each, in their order. */
export function editedFieldsText(
  paths: readonly string[],
  fields: readonly HolidayFormField[],
): string {
  return [...new Set(paths.map((path) => fieldLabel(path, fields)))].join(", ");
}
