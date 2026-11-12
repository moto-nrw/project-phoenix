// Wording and checks of the SEPA direct debit (#4695): the names of the
// mandate kinds, sequences, states and trail entries, and the shape checks
// the forms run before they send an IBAN or a creditor identifier. The
// backend checks the same again; these only point at the field early.

import type { StatusBadgeTone } from "~/components/ui/status-badge";
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
  collect: "Abgebucht",
};

const FIELD_LABELS: Record<string, string> = {
  account_holder: "Kontoinhaber",
  iban: "IBAN",
  bic: "BIC",
  sequence: "Lastschriftart",
};

/** A mandate whose 36 months without a collection ran out on the school
 * day today (#4697). */
export function mandateExpired(mandate: SepaMandate, today: string): boolean {
  return mandate.expiresOn !== null && mandate.expiresOn <= today;
}

/** The state of a mandate as a badge on the school day today. */
export function mandateState(
  mandate: SepaMandate,
  today: string,
): {
  readonly label: string;
  readonly tone: StatusBadgeTone;
} {
  if (mandate.revokedOn) return { label: "Widerrufen", tone: "gray" };
  if (!mandate.creditorCurrent) {
    return { label: "Neues Mandat nötig", tone: "warning" };
  }
  if (mandateExpired(mandate, today)) {
    return { label: "Verfallen", tone: "red" };
  }
  return { label: "Gültig", tone: "green" };
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

/** Who acted in a trail entry: the e-mail of a known account, else its
 * number, and a deleted account as such. */
export function mandateEventActor(
  event: SepaMandateEvent,
  emails: ReadonlyMap<string, string>,
): string {
  if (event.accountId === null) return "gelöschtes Konto";
  return emails.get(event.accountId) ?? `Konto Nr. ${event.accountId}`;
}

/** The name of the family of a mandate. */
export function mandateFamily(mandate: SepaMandate): string {
  const name = `${mandate.partyFirstName} ${mandate.partyLastName}`.trim();
  return name || mandate.familyEmail;
}

export {
  compactCode,
  creditorIdLooksValid,
  ibanLooksValid,
} from "~/lib/sepa-checks";

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
