// Texte des Mandatsimports (#6198): Namen der Felder, Stand einer Zeile und
// was eine Zeile aufhält. Kein Text nennt einen Wert aus der Datei; die IBAN
// kommt nur verdeckt an.

import type { StatusBadgeTone } from "~/components/ui/status-badge";
import type {
  ImportResult,
  ImportField,
  ImportProblem,
  ImportRow,
  ImportRowStatus,
  PayerKey,
} from "~/lib/traeger-sepa-import-api";

/** Die Felder in der Reihenfolge der Zuordnung. */
export const IMPORT_FIELDS: readonly ImportField[] = [
  "reference",
  "signed_on",
  "payer",
  "account_holder",
  "iban",
  "bic",
  "creditor_id",
  "sequence",
  "last_collected_on",
  "debtor_number",
];

/** Ohne diese Spalten liest moto keine Zeile. */
export const REQUIRED_IMPORT_FIELDS: ReadonlySet<ImportField> = new Set([
  "reference",
  "signed_on",
  "payer",
  "account_holder",
  "iban",
]);

export const IMPORT_FIELD_LABELS: Record<ImportField, string> = {
  reference: "Mandatsreferenz",
  signed_on: "Unterschrieben am",
  payer: "Zahler",
  account_holder: "Kontoinhaber",
  iban: "IBAN",
  bic: "BIC",
  creditor_id: "Gläubiger-ID",
  sequence: "Erst- oder Folgelastschrift",
  last_collected_on: "Letzte Abbuchung",
  debtor_number: "Debitorennummer",
};

export const IMPORT_FIELD_HINTS: Partial<Record<ImportField, string>> = {
  bic: "Nur für Konten außerhalb der EU nötig.",
  sequence: "Zum Beispiel FRST und RCUR oder Erst- und Folgelastschrift.",
  last_collected_on: "Belegt, dass schon abgebucht wurde.",
  debtor_number:
    "Gilt für den Zahler oder für das Kind, wie unter „Debitorennummer gilt je“ eingestellt. moto überschreibt keine Nummer, die schon eingetragen ist.",
};

/**
 * Was der Import mit der Debitorennummer einer Zeile macht (#4693): beim
 * Zahler oder, wenn der Träger sie je Kind führt, beim Kind (#7007).
 */
export function importDebtorNumberText(row: ImportRow): string | null {
  switch (row.debtorNumberStatus) {
    case "new":
      return `Debitorennummer ${row.debtorNumber} wird eingetragen.`;
    case "same":
      return `Debitorennummer ${row.debtorNumber} ist schon eingetragen.`;
    case "siblings_differ":
      return "Debitorennummer: Geschwister haben verschiedene Nummern. moto übernimmt keine. Tragen Sie die richtige Nummer bei der Familie ein.";
    case "differs":
      return "Debitorennummer: In moto steht schon eine andere Nummer. moto ändert sie nicht.";
    case "invalid":
      return "Debitorennummer ist zu lang oder enthält eine IBAN.";
    case "no_family":
      return "Debitorennummer: moto kennt die Familie nicht.";
    case "no_child":
      return "Debitorennummer: Die Familie hat mehrere Kinder mit Vertrag. Wählen Sie als Zahler die Vertragsnummer, dann erkennt moto das Kind.";
    case "rows_differ":
      return "Debitorennummer: Für dieses Kind stehen verschiedene Nummern in der Datei. moto übernimmt keine.";
    default:
      return null;
  }
}

export const PAYER_KEY_LABELS: Record<PayerKey, string> = {
  email: "E-Mail der Familie",
  contract_id: "Vertragsnummer",
  debtor_number: "Debitorennummer",
};

export const IMPORT_STATUS: Record<
  ImportRowStatus,
  { readonly label: string; readonly tone: StatusBadgeTone }
> = {
  ready: { label: "Bereit", tone: "green" },
  clarify: { label: "Bitte klären", tone: "orange" },
  duplicate: { label: "Schon vorhanden", tone: "gray" },
  same_mandate: { label: "Gleiches Mandat", tone: "gray" },
  conflict: { label: "Konflikt", tone: "red" },
  invalid: { label: "Fehlerhaft", tone: "red" },
  imported: { label: "Übernommen", tone: "green" },
};

const DELIMITER_LABELS: Record<string, string> = {
  ";": "Semikolon",
  ",": "Komma",
  "\t": "Tabulator",
};

const ENCODING_LABELS: Record<string, string> = {
  "utf-8": "UTF-8",
  "windows-1252": "Windows-1252",
};

/** „Windows-1252 · Semikolon“: wie moto die Datei gelesen hat. */
export function importFormatLabel(encoding: string, delimiter: string): string {
  return `${ENCODING_LABELS[encoding] ?? encoding} · ${DELIMITER_LABELS[delimiter] ?? delimiter}`;
}

const DATE_FIELDS: ReadonlySet<ImportField> = new Set([
  "signed_on",
  "last_collected_on",
]);

/** Ein Satz je Problem, ohne den Wert aus der Datei. */
export function importProblemText(problem: ImportProblem): string {
  const field = IMPORT_FIELD_LABELS[problem.field];
  switch (problem.code) {
    case "missing":
      return `${field} fehlt.`;
    case "invalid":
      if (DATE_FIELDS.has(problem.field)) {
        return `${field}: Das Datum stimmt nicht. Bitte als TT.MM.JJJJ.`;
      }
      return `${field} stimmt nicht.`;
    case "future":
      return `${field} liegt in der Zukunft.`;
    case "before_signed":
      return "Die letzte Abbuchung liegt vor der Unterschrift.";
    case "bic_required":
      return "Das Konto liegt außerhalb der EU. Die BIC fehlt.";
    case "too_long":
      return `${field} hat mehr als 70 Zeichen.`;
    case "payer_unknown":
      return "Zu diesem Zahler gibt es bei Ihnen keinen Vertrag.";
    case "payer_ambiguous":
      return "Diese Debitorennummer haben bei Ihnen mehrere Familien oder Kinder. moto weiß nicht, wer zahlt.";
    case "family_unknown":
      return "Im Vertrag fehlt die E-Mail-Adresse der Familie.";
    case "creditor_mismatch":
      return "Das Mandat gehört zu einer anderen Gläubiger-ID.";
    case "reference_taken":
      return "Diese Mandatsreferenz gibt es schon mit anderen Angaben.";
    case "reference_repeated":
      return "Die Mandatsreferenz steht mehrmals in der Datei.";
    case "payer_has_mandate":
      return "Die Familie hat schon ein gültiges Mandat.";
    case "payer_repeated":
      return "Die Familie steht mehrmals in der Datei.";
    case "sequence_unknown":
      return "Die Datei sagt nicht, ob schon abgebucht wurde.";
    case "sequence_contradicts":
      return "Erstlastschrift, aber mit letzter Abbuchung.";
    case "one_off":
      return "Einzelmandate übernimmt moto nicht.";
    case "account_differs":
      return problem.field === "account_holder"
        ? "Zu dieser Mandatsreferenz ist ein anderer Kontoinhaber gespeichert."
        : "Zu dieser Mandatsreferenz ist ein anderes Konto gespeichert.";
    case "expired":
      return "Seit 36 Monaten nicht genutzt. Das Mandat gilt nicht mehr.";
    default:
      return `${field} prüfen.`;
  }
}

/** Hinweise, die eine Zeile nicht aufhalten. */
export function importHintText(hint: string): string {
  if (hint === "last_collection_unknown") {
    return "Offen: Die letzte Abbuchung fehlt. Die 36 Monate zählen ab dem Import.";
  }
  return hint;
}

/** Die Familie einer Zeile, „–“ ohne Vertrag. */
export function importPayerName(row: ImportRow): string {
  const name = `${row.payerFirstName} ${row.payerLastName}`.trim();
  return name === "" ? "–" : name;
}

export type ImportFilter =
  "all" | "ready" | "clarify" | "problems" | "duplicate";

/** Ob eine Zeile zum gewählten Filter gehört. */
export function matchesImportFilter(
  row: ImportRow,
  filter: ImportFilter,
): boolean {
  switch (filter) {
    case "all":
      return true;
    case "ready":
      return row.status === "ready" || row.status === "imported";
    case "problems":
      return row.status === "conflict" || row.status === "invalid";
    case "duplicate":
      // A sibling's row repeats a mandate of the file (#7009).
      return row.status === "duplicate" || row.status === "same_mandate";
    default:
      return row.status === filter;
  }
}

/** Mandate und Familiennummern zählen getrennt, auch bei Geschwistern. */
export function importChangesLabel(mandates: number, debtors: number): string {
  const parts: string[] = [];
  if (mandates > 0 || debtors === 0)
    parts.push(mandates === 1 ? "1 Mandat" : `${mandates} Mandate`);
  if (debtors > 0)
    parts.push(
      debtors === 1 ? "1 Debitorennummer" : `${debtors} Debitorennummern`,
    );
  return parts.join(" und ");
}

/** Nur die vom Backend bestätigten Änderungen lassen sich übernehmen. */
export function importableCount(result: ImportResult): number {
  return result.counts.ready + result.counts.debtorNumbers;
}
