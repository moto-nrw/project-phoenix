// Wording and checks of the SEPA direct debit (#4695): the names of the
// mandate kinds, sequences, states and trail entries, and the shape checks
// the forms run before they send an IBAN or a creditor identifier. The
// backend checks the same again; these only point at the field early.

import type { StatusBadgeTone } from "~/components/ui/status-badge";
import type { DebitGap } from "~/lib/traeger-billing-terms-api";
import type {
  MandateAction,
  MandateKind,
  MandateSequence,
  MandateSource,
  MissingMandate,
  SepaMandate,
  SepaMandateEvent,
} from "~/lib/traeger-sepa-api";

export const MANDATE_KIND_LABELS: Record<MandateKind, string> = {
  recurring: "Dauermandat",
  one_off: "Einzelmandat",
};

export const MANDATE_KIND_HINTS: Record<MandateKind, string> = {
  recurring: "Für jeden Monat.",
  one_off: "Nur für eine Abbuchung.",
};

const MANDATE_SEQUENCE_LABELS: Record<MandateSequence, string> = {
  first: "Erste Lastschrift",
  recurring: "Folgelastschrift",
};

export const MISSING_MANDATE_LABELS: Record<MissingMandate, string> = {
  transfer: "Als Überweisung ausstellen",
  block: "Freigabe anhalten",
};

export const MISSING_MANDATE_HINTS: Record<MissingMandate, string> = {
  transfer:
    "Die Familie bekommt eine Rechnung zum Überweisen. Die Prüfung zeigt sie als Hinweis.",
  block:
    "Der Lauf lässt sich erst freigeben, wenn jede Familie ein Mandat hat oder überweist.",
};

export const DEBIT_GAP_LABELS: Record<DebitGap, string> = {
  mandate_missing: "Kein gültiges Mandat",
  creditor_missing: "Gläubiger-ID fehlt",
};

const SOURCE_LABELS: Record<MandateSource | "run", string> = {
  office: "im Büro",
  import: "beim Import",
  parent: "von den Eltern",
  run: "im Monatslauf",
};

const ACTION_LABELS: Record<MandateAction, string> = {
  create: "Erfasst",
  update: "Geändert",
  revoke: "Widerrufen",
  read: "Angesehen",
  reveal: "Für die Lastschriftdatei geöffnet",
};

const FIELD_LABELS: Record<string, string> = {
  account_holder: "Kontoinhaber",
  iban: "IBAN",
  bic: "BIC",
  sequence: "Lastschriftart",
};

/** The state of a mandate as a badge. */
export function mandateState(mandate: SepaMandate): {
  readonly label: string;
  readonly tone: StatusBadgeTone;
} {
  return mandate.revokedOn
    ? { label: "Widerrufen", tone: "gray" }
    : { label: "Gültig", tone: "green" };
}

/** What the next collection of a mandate is. */
export function nextDebitLabel(mandate: SepaMandate): string {
  if (mandate.revokedOn) return "–";
  if (mandate.kind === "one_off") return "Einmalige Lastschrift";
  return MANDATE_SEQUENCE_LABELS[mandate.sequence];
}

/** One line of the trail: what happened and where. */
export function mandateEventLabel(event: SepaMandateEvent): string {
  const action = ACTION_LABELS[event.action];
  const fields = event.fields
    .map((field) => FIELD_LABELS[field] ?? field)
    .join(", ");
  const what = fields ? `${action}: ${fields}` : action;
  return `${what} ${SOURCE_LABELS[event.source]}`;
}

/** The name of the family of a mandate. */
export function mandateFamily(mandate: SepaMandate): string {
  const name = `${mandate.partyFirstName} ${mandate.partyLastName}`.trim();
  return name || mandate.familyEmail;
}

/** Removes white space and upper-cases what people type. */
export function compactCode(value: string): string {
  return value.replace(/\s+/g, "").toUpperCase();
}

/** ISO 7064 MOD 97-10 remainder; letters count as two digits (A = 10). */
function mod97(value: string): number {
  let remainder = 0;
  for (const char of value) {
    const digit = /\d/.test(char)
      ? Number(char)
      : char.charCodeAt(0) - "A".charCodeAt(0) + 10;
    remainder = Number(`${remainder}${digit}`) % 97;
  }
  return remainder;
}

/** Shape and check digits of an IBAN; the backend also checks the length. */
export function ibanLooksValid(raw: string): boolean {
  const iban = compactCode(raw);
  if (!/^[A-Z]{2}\d{2}[A-Z0-9]{11,30}$/.test(iban)) return false;
  if (iban.startsWith("DE") && !/^DE\d{20}$/.test(iban)) return false;
  return mod97(iban.slice(4) + iban.slice(0, 4)) === 1;
}

/** Shape and check digits of a SEPA creditor identifier. */
export function creditorIdLooksValid(raw: string): boolean {
  const id = compactCode(raw);
  if (!/^[A-Z]{2}\d{2}[A-Z0-9]{3}[A-Z0-9]{1,28}$/.test(id)) return false;
  if (id.startsWith("DE") && !/^DE\d{2}[A-Z0-9]{3}\d{11}$/.test(id)) {
    return false;
  }
  return mod97(id.slice(7) + id.slice(0, 4)) === 1;
}

/** The SEPA character set of a mandate reference. */
export function referenceLooksValid(raw: string): boolean {
  const reference = raw.trim();
  return (
    reference === "" ||
    (/^[A-Za-z0-9+?/\-:().,' ]{1,35}$/.test(reference) &&
      !reference.startsWith("/") &&
      !reference.includes("//"))
  );
}
