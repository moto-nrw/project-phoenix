// Types and value helpers the OGS-Anmeldung (enrollment-form.tsx) shares with
// the field renderers split out next to it. No React here: the renderers and
// the form's own submit check read the same answer shapes through these.

import type { PublicFormSchema } from "~/lib/enrollment-form-schema-api";

export type TranslationValues = Record<string, string | number | Date>;
export type TranslationFn = (key: string, values?: TranslationValues) => string;
export type EnrollmentFormTranslator = TranslationFn & {
  raw: (key: string) => unknown;
};

export type FormField = PublicFormSchema["fields"][number];

export interface CustomFieldInputProps {
  readonly field: FormField;
  // The control's `name`, unique per child, so a server field error and the
  // focus jump find the right input (#2515).
  readonly inputName: string;
  readonly value: unknown;
  readonly onChange: (v: unknown) => void;
  readonly error?: string;
  readonly tr: EnrollmentFormTranslator;
  readonly relevantDays?: readonly string[];
  // True when relevantDays come from selected care offerings (not the
  // all-weekdays fallback). Drives the weekday_schedule "every care day needs a
  // time" requirement and its hint copy.
  readonly careConstrained?: boolean;
  // Free-text "mit wem" for the accompanied departure mode (#1694), coupled
  // into the weekday_multi_mode field so it is always available to parents
  // without the admin adding a separate field. Only wired for the
  // allowed-departure-modes field.
  readonly companionNote?: string;
  readonly onCompanionNoteChange?: (v: string) => void;
  // Per-field error for the coupled "mit wem" note (#1694), keyed separately
  // from `error` so the modes fieldset and the note input show their own
  // messages.
  readonly companionNoteError?: string;
  readonly companionNoteName?: string;
  // Heimweg-Beschränkung (#2381): true when the field's single_mode_grades
  // restrict this child's target grade — the multi-mode picker then allows
  // at most one way home per care day (selecting a mode replaces the
  // previous one). Backend re-validates, this is the honest UI for it.
  readonly singleMode?: boolean;
}

export function asStringMap(value: unknown): Record<string, string> {
  if (!value || typeof value !== "object" || Array.isArray(value)) return {};
  return Object.fromEntries(
    Object.entries(value as Record<string, unknown>).filter(
      (entry): entry is [string, string] => typeof entry[1] === "string",
    ),
  );
}

// ---- Structured answer shapes -------------------------------------------
//
// The shapes here mirror the Go types in
// backend/models/enrollment/form_schema.go (PhoneEntry, WeekdaySchedule,
// ContactEntry). The decision service consumes whatever JSON these
// components emit, so the field names must match exactly.

export interface PhoneEntry {
  phone_number: string;
  phone_type: "mobile" | "home" | "work" | "other";
  label?: string;
  is_primary?: boolean;
}

export interface ContactEntryValue {
  first_name: string;
  last_name: string;
  email?: string;
  relationship_type?: string;
  phone_numbers?: PhoneEntry[];
  can_pickup?: boolean;
  is_emergency_contact?: boolean;
  emergency_priority?: number;
}

export const WEEKDAYS = ["mon", "tue", "wed", "thu", "fri"] as const;

export function asScheduleObject(v: unknown): Record<string, string> {
  if (!v || typeof v !== "object" || Array.isArray(v)) return {};
  const out: Record<string, string> = {};
  for (const weekday of WEEKDAYS) {
    const raw = (v as Record<string, unknown>)[weekday];
    if (typeof raw === "string") out[weekday] = raw;
  }
  return out;
}

export function asWeekdayBooleanObject(v: unknown): Record<string, boolean> {
  if (!v || typeof v !== "object" || Array.isArray(v)) return {};
  const out: Record<string, boolean> = {};
  for (const w of WEEKDAYS) {
    const raw = (v as Record<string, unknown>)[w];
    if (typeof raw === "boolean") out[w] = raw;
  }
  return out;
}

export type DepartureModeValue = "alone" | "bus" | "pickup" | "accompanied";

const DEPARTURE_MULTI_MODE_OPTIONS: ReadonlyArray<DepartureModeValue> = [
  "alone",
  "bus",
  "pickup",
  "accompanied",
];

const DEPARTURE_MULTI_MODE_OPTIONS_WITHOUT_COMPANION: ReadonlyArray<DepartureModeValue> =
  ["alone", "bus", "pickup"];

export function departureMultiModeOptionsForField(
  field: FormField,
): ReadonlyArray<DepartureModeValue> {
  return field.target === "student.allowed_departure_modes"
    ? DEPARTURE_MULTI_MODE_OPTIONS
    : DEPARTURE_MULTI_MODE_OPTIONS_WITHOUT_COMPANION;
}

// Reserved child-custom key carrying the coupled "mit wem" note for the
// accompanied mode (#1694). Matches the backend reserved target so the
// decision service can apply it when it processes allowed_departure_modes.
// It travels with the modes field instead of being a separate admin field, so
// the note is always available to parents.
export const DEPARTURE_COMPANION_KEY = "student.departure_companion_note";

export function asWeekdayModeObject(
  v: unknown,
): Record<string, DepartureModeValue> {
  if (!v || typeof v !== "object" || Array.isArray(v)) return {};
  const out: Record<string, DepartureModeValue> = {};
  for (const w of WEEKDAYS) {
    const raw = (v as Record<string, unknown>)[w];
    if (
      raw === "bus" ||
      raw === "pickup" ||
      raw === "alone" ||
      raw === "accompanied"
    ) {
      out[w] = raw;
    }
  }
  return out;
}

export function asWeekdayMultiModeObject(
  v: unknown,
  allowedDays: readonly string[] = WEEKDAYS,
  allowedModes: ReadonlyArray<DepartureModeValue> = DEPARTURE_MULTI_MODE_OPTIONS,
): Record<string, DepartureModeValue[]> {
  if (!v || typeof v !== "object" || Array.isArray(v)) return {};
  const allowed = new Set(allowedDays);
  const allowedModeSet = new Set(allowedModes);
  const out: Record<string, DepartureModeValue[]> = {};
  for (const w of WEEKDAYS) {
    if (!allowed.has(w)) continue;
    const raw = (v as Record<string, unknown>)[w];
    if (!Array.isArray(raw)) continue;
    const seen = new Set<DepartureModeValue>();
    for (const item of raw) {
      if (
        item === "alone" ||
        item === "bus" ||
        item === "pickup" ||
        item === "accompanied"
      ) {
        if (allowedModeSet.has(item)) seen.add(item);
      }
    }
    const modes = allowedModes.filter((mode) => seen.has(mode));
    if (modes.length > 0) out[w] = modes;
  }
  return out;
}
