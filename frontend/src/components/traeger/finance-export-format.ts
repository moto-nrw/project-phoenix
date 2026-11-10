/**
 * Labels and the preview of the finance exports (#4693): the fields a
 * column of the Sollstellung can take, the options of the file, who
 * downloaded a file, and a sample of the file as the mapping writes it.
 */

import { formatBerlinDate, formatBerlinTime } from "~/lib/date-helpers";
import {
  type ExportFormat,
  type SollColumn,
  type SollDateFormat,
  type SollDecimal,
  type SollDelimiter,
  type SollEncoding,
  type SollField,
  type SollMapping,
  SOLL_BOOKING_FIELDS,
} from "~/lib/traeger-finance-exports-api";

/** What a column of the Sollstellung shows, in the order of the choice. */
export const SOLL_FIELD_LABELS: Record<SollField, string> = {
  debtor_number: "Debitorennummer",
  invoice_number: "Rechnungsnummer",
  invoice_date: "Rechnungsdatum",
  due_on: "Fällig am",
  amount: "Betrag",
  recipient_name: "Name des Empfängers",
  recipient_street: "Straße",
  recipient_postal_code: "Postleitzahl",
  recipient_city: "Ort",
  child_name: "Name des Kindes",
  school_name: "Schule",
  service_from: "Leistung von",
  service_until: "Leistung bis",
  payment_method: "Zahlungsweg",
  mandate_reference: "Mandatsreferenz",
  document_kind: "Belegart",
  file_number: "Dateinummer",
  revenue_account: "Erlöskonto",
  cost_center: "Kostenstelle (KOST1)",
  cost_center_2: "Zweite Kostenstelle (KOST2)",
  constant: "Fester Wert",
};

export const SOLL_FIELDS = Object.keys(SOLL_FIELD_LABELS) as SollField[];

export const SOLL_DELIMITERS: { value: SollDelimiter; label: string }[] = [
  { value: ";", label: "Semikolon" },
  { value: ",", label: "Komma" },
  { value: "\t", label: "Tabulator" },
  { value: "|", label: "Strich" },
];

export const SOLL_DECIMALS: { value: SollDecimal; label: string }[] = [
  { value: ",", label: "12,50" },
  { value: ".", label: "12.50" },
];

export const SOLL_DATES: { value: SollDateFormat; label: string }[] = [
  { value: "dd.mm.yyyy", label: "31.10.2026" },
  { value: "yyyy-mm-dd", label: "2026-10-31" },
];

export const SOLL_ENCODINGS: { value: SollEncoding; label: string }[] = [
  { value: "utf-8-bom", label: "UTF-8 für Excel (empfohlen)" },
  { value: "utf-8", label: "UTF-8 ohne Kennung" },
  { value: "windows-1252", label: "Windows-1252 (ältere Programme)" },
];

export const FINANCE_FILE_KINDS: Record<ExportFormat, string> = {
  sollstellung: "Sollstellung",
  pain008: "Lastschrift",
  datev: "DATEV",
};

/** "09.11.2026, 15:34" in the school's time zone. */
export function exportedAtLabel(iso: string): string {
  return `${formatBerlinDate(iso)}, ${formatBerlinTime(iso)}`;
}

/** Who downloaded a file or did a step. */
export function byLabel(byYou: boolean, byAccount: boolean): string {
  if (byYou) return "von Ihnen";
  return byAccount ? "von einem anderen Konto" : "von einem gelöschten Konto";
}

/** The file name the pattern gives a file number. */
export function sollFileName(pattern: string, fileNumber: number): string {
  return pattern.replace("{NR}", String(fileNumber));
}

/** One invoice and its Storno as the preview shows them. */
const SAMPLE_ROWS: Record<SollField, readonly [string, string]> = {
  debtor_number: ["10 4711", "10 4711"],
  invoice_number: ["MB-2026-0110", "MB-2026-0111"],
  invoice_date: ["2026-10-30", "2026-11-02"],
  due_on: ["2026-11-04", ""],
  amount: ["7800", "-7800"],
  recipient_name: ["Eva Müller", "Eva Müller"],
  recipient_street: ["Sonnenweg 7", "Sonnenweg 7"],
  recipient_postal_code: ["75177", "75177"],
  recipient_city: ["Pforzheim", "Pforzheim"],
  child_name: ["Lia Müller", "Lia Müller"],
  school_name: ["GS Ispringen", "GS Ispringen"],
  service_from: ["2026-10-01", "2026-10-01"],
  service_until: ["2026-10-31", "2026-10-31"],
  payment_method: ["Lastschrift", "Lastschrift"],
  mandate_reference: ["SJR-0001", "SJR-0001"],
  document_kind: ["Rechnung", "Storno"],
  file_number: ["", ""],
  revenue_account: ["4400", "4400"],
  cost_center: ["KST100", "KST100"],
  cost_center_2: ["", ""],
  constant: ["", ""],
};

/** A sample row: the invoice or its Storno, and the line of where it books. */
interface SampleRow {
  readonly document: 0 | 1;
  readonly line?: {
    readonly cents: number;
    readonly account: string;
    readonly kost1: string;
    readonly kost2: string;
  };
}

/**
 * With an account field, a document writes one row per revenue account and
 * cost centers: the sample invoice books the Frühbetreuung on its own
 * account and the rest on the account of the monthly contributions.
 */
function sampleRows(mapping: SollMapping): SampleRow[] {
  if (!sollSplitsLines(mapping)) return [{ document: 0 }, { document: 1 }];
  const lines = [
    { cents: 3250, account: "4410", kost1: "KST100", kost2: "FRUEH" },
    { cents: 4550, account: "4400", kost1: "KST100", kost2: "" },
  ];
  return [
    ...lines.map((line) => ({ document: 0 as const, line })),
    ...lines.map((line) => ({
      document: 1 as const,
      line: { ...line, cents: -line.cents },
    })),
  ];
}

/** Whether the mapping writes one row per account line (#4821). */
export function sollSplitsLines(mapping: SollMapping): boolean {
  return mapping.columns.some((column) =>
    SOLL_BOOKING_FIELDS.includes(column.field),
  );
}

function formatCents(cents: number, decimal: SollDecimal): string {
  const sign = cents < 0 ? "-" : "";
  const abs = Math.abs(cents);
  return `${sign}${Math.floor(abs / 100)}${decimal}${String(abs % 100).padStart(2, "0")}`;
}

function lineValue(
  column: SollColumn,
  line: NonNullable<SampleRow["line"]>,
  decimal: SollDecimal,
): string | null {
  switch (column.field) {
    case "amount":
      return formatCents(line.cents, decimal);
    case "revenue_account":
      return line.account;
    case "cost_center":
      return line.kost1;
    case "cost_center_2":
      return line.kost2;
    default:
      return null;
  }
}

const DATE_FIELDS: readonly SollField[] = [
  "invoice_date",
  "due_on",
  "service_from",
  "service_until",
];

function sampleValue(
  mapping: SollMapping,
  column: SollColumn,
  sample: SampleRow,
  fileNumber: number,
): string {
  if (column.field === "constant") return column.value;
  if (column.field === "file_number") return String(fileNumber);
  if (sample.line) {
    const value = lineValue(column, sample.line, mapping.decimalSeparator);
    if (value !== null) return value;
  }
  const raw = SAMPLE_ROWS[column.field][sample.document];
  if (raw === "") return "";
  if (
    DATE_FIELDS.includes(column.field) &&
    mapping.dateFormat === "dd.mm.yyyy"
  ) {
    const [year, month, day] = raw.split("-");
    return `${day}.${month}.${year}`;
  }
  if (column.field === "amount") {
    return formatCents(Number(raw), mapping.decimalSeparator);
  }
  return raw;
}

function quoted(value: string, delimiter: SollDelimiter): string {
  return value.includes(delimiter) || value.includes('"')
    ? `"${value.replaceAll('"', '""')}"`
    : value;
}

/**
 * The first lines of the file as the mapping writes them, with an invented
 * invoice and its Storno, one row per account line when the mapping names
 * an account field; the tabulator shows as „→“ so it can be seen.
 */
export function sollPreview(
  mapping: SollMapping,
  fileNumber: number,
): string[] {
  const shown = mapping.delimiter === "\t" ? " → " : mapping.delimiter;
  const line = (values: string[]) =>
    values.map((value) => quoted(value, mapping.delimiter)).join(shown);
  const lines: string[] = [];
  if (mapping.headerRow) {
    lines.push(line(mapping.columns.map((column) => column.header)));
  }
  for (const sample of sampleRows(mapping)) {
    lines.push(
      line(
        mapping.columns.map((column) =>
          sampleValue(mapping, column, sample, fileNumber),
        ),
      ),
    );
  }
  return lines;
}
