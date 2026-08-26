// Zahlungsplan einer Schule (#1459 Demo) — Operator-Seite.
//
// Das moto-Team pflegt hier, welche Rechnung wann fällig ist und ob sie
// bezahlt wurde. Es gibt bewusst keinen Zahlungsdienstleister dahinter:
// "bezahlt" heißt, ein Mensch bei moto hat das Geld gesehen. Die Schule liest
// dieselben Zeilen unter /vertrag, kann sie aber nicht ändern.

import { operatorFetch } from "./api-helpers";

export type InvoiceStatus = "offen" | "bezahlt" | "storniert";

interface BackendSchoolInvoice {
  id: number;
  period_label: string;
  invoice_number: string;
  amount_cents: number;
  due_date: string;
  status: InvoiceStatus;
  overdue: boolean;
  paid_on?: string | null;
  note: string;
}

export interface SchoolInvoice {
  id: string;
  periodLabel: string;
  invoiceNumber: string;
  amountCents: number;
  dueDate: string;
  status: InvoiceStatus;
  overdue: boolean;
  paidOn: string | null;
  note: string;
}

export interface SchoolInvoiceInput {
  periodLabel: string;
  invoiceNumber: string;
  amountCents: number;
  dueDate: string;
  status: InvoiceStatus;
  paidOn: string | null;
  note: string;
}

export function mapSchoolInvoice(invoice: BackendSchoolInvoice): SchoolInvoice {
  return {
    id: invoice.id.toString(),
    periodLabel: invoice.period_label,
    invoiceNumber: invoice.invoice_number,
    amountCents: invoice.amount_cents,
    dueDate: invoice.due_date,
    status: invoice.status,
    overdue: invoice.overdue,
    paidOn: invoice.paid_on ?? null,
    note: invoice.note,
  };
}

function toBody(input: SchoolInvoiceInput) {
  return {
    period_label: input.periodLabel,
    invoice_number: input.invoiceNumber,
    amount_cents: input.amountCents,
    due_date: input.dueDate,
    status: input.status,
    // Leerer String heißt "kein Zahlungsdatum" — das Backend löscht es dann.
    paid_on: input.paidOn ?? "",
    note: input.note,
  };
}

function invoicesEndpoint(schoolId: string): string {
  return `/api/operator/provisioning/schools/${encodeURIComponent(schoolId)}/invoices`;
}

export async function listSchoolInvoices(
  schoolId: string,
): Promise<SchoolInvoice[]> {
  const data = await operatorFetch<BackendSchoolInvoice[] | null>(
    invoicesEndpoint(schoolId),
  );
  return (data ?? []).map(mapSchoolInvoice);
}

export async function createSchoolInvoice(
  schoolId: string,
  input: SchoolInvoiceInput,
): Promise<SchoolInvoice> {
  const data = await operatorFetch<BackendSchoolInvoice>(
    invoicesEndpoint(schoolId),
    { method: "POST", body: toBody(input) },
  );
  return mapSchoolInvoice(data);
}

export async function updateSchoolInvoice(
  schoolId: string,
  invoiceId: string,
  input: SchoolInvoiceInput,
): Promise<SchoolInvoice> {
  const data = await operatorFetch<BackendSchoolInvoice>(
    `${invoicesEndpoint(schoolId)}/${encodeURIComponent(invoiceId)}`,
    { method: "PUT", body: toBody(input) },
  );
  return mapSchoolInvoice(data);
}

export async function deleteSchoolInvoice(
  schoolId: string,
  invoiceId: string,
): Promise<void> {
  await operatorFetch<Record<string, never>>(
    `${invoicesEndpoint(schoolId)}/${encodeURIComponent(invoiceId)}`,
    { method: "DELETE" },
  );
}

/** Euro-Anzeige aus ganzzahligen Cent. */
export function formatCents(cents: number): string {
  return new Intl.NumberFormat("de-DE", {
    style: "currency",
    currency: "EUR",
  }).format(cents / 100);
}

/**
 * Eingabe wie "19,90" oder "19.90" in ganzzahlige Cent. Gibt null zurück, wenn
 * die Eingabe kein Betrag ist — der Aufrufer zeigt dann eine Meldung, statt
 * still 0 € zu speichern.
 */
export function parseEuroToCents(value: string): number | null {
  const normalized = value.trim().replace(/\s/g, "").replace(",", ".");
  if (normalized === "" || !/^\d+(\.\d{1,2})?$/.test(normalized)) return null;

  const [euros, cents = ""] = normalized.split(".");
  if (!euros) return null;

  const amountCents = BigInt(euros) * 100n + BigInt(cents.padEnd(2, "0"));
  if (amountCents > BigInt(Number.MAX_SAFE_INTEGER)) return null;
  return Number(amountCents);
}

/** Cent als bearbeitbarer Euro-Text ("1990" → "19,90"). */
export function centsToEuroInput(cents: number): string {
  return (cents / 100).toFixed(2).replace(".", ",");
}

/** Anzeige-Status inklusive des abgeleiteten "Überfällig". */
export function invoiceStatusLabel(invoice: SchoolInvoice): string {
  if (invoice.status === "bezahlt") return "Bezahlt";
  if (invoice.status === "storniert") return "Storniert";
  return invoice.overdue ? "Überfällig" : "Offen";
}

export function invoiceStatusTone(
  invoice: SchoolInvoice,
): "green" | "red" | "orange" | "gray" {
  if (invoice.status === "bezahlt") return "green";
  if (invoice.status === "storniert") return "gray";
  return invoice.overdue ? "red" : "orange";
}
