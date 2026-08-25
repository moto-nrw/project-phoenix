// Vertrag (#1459 Demo): die Vertragsdaten der Schule, wie das moto-Team sie
// gepflegt hat, plus der Zahlungsplan. Die Seite /vertrag ist reine Anzeige —
// es gibt hier bewusst keine Schreibfunktion, denn alle Werte pflegt das
// moto-Team im Operator-Portal.

import { sessionFetch } from "./session-cache";

/** Zahlungsstatus, wie er im Backend gespeichert ist. */
export type InvoiceStatus = "offen" | "bezahlt" | "storniert";

interface BackendInvoice {
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

interface BackendContractOverview {
  tier: string;
  tier_label: string;
  booked_children: number;
  active_children: number;
  price_per_child_cents: number;
  billing_cycle: string;
  billing_cycle_label: string;
  term_start?: string | null;
  term_end?: string | null;
  invoice_recipient: string;
  customer_number: string;
  support_email: string;
  note: string;
  configured: boolean;
  reference_date: string;
  invoices: BackendInvoice[] | null;
  open_amount_cents: number;
  next_due?: BackendInvoice | null;
}

export interface Invoice {
  /** Backend-int64 wird im Frontend zu string (Projektkonvention). */
  id: string;
  periodLabel: string;
  invoiceNumber: string;
  amountCents: number;
  /** Kalendertag als "YYYY-MM-DD" — nie aus einem Date abgeleitet. */
  dueDate: string;
  status: InvoiceStatus;
  overdue: boolean;
  paidOn: string | null;
  note: string;
}

export interface ContractOverview {
  tier: string;
  tierLabel: string;
  bookedChildren: number;
  activeChildren: number;
  pricePerChildCents: number;
  billingCycle: string;
  billingCycleLabel: string;
  termStart: string | null;
  termEnd: string | null;
  invoiceRecipient: string;
  customerNumber: string;
  supportEmail: string;
  note: string;
  configured: boolean;
  referenceDate: string;
  invoices: Invoice[];
  openAmountCents: number;
  nextDue: Invoice | null;
}

export function mapInvoice(invoice: BackendInvoice): Invoice {
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

export function mapContractOverview(
  data: BackendContractOverview,
): ContractOverview {
  return {
    tier: data.tier,
    tierLabel: data.tier_label,
    bookedChildren: data.booked_children,
    activeChildren: data.active_children,
    pricePerChildCents: data.price_per_child_cents,
    billingCycle: data.billing_cycle,
    billingCycleLabel: data.billing_cycle_label,
    termStart: data.term_start ?? null,
    termEnd: data.term_end ?? null,
    invoiceRecipient: data.invoice_recipient,
    customerNumber: data.customer_number,
    supportEmail: data.support_email,
    note: data.note,
    configured: data.configured,
    referenceDate: data.reference_date,
    invoices: (data.invoices ?? []).map(mapInvoice),
    openAmountCents: data.open_amount_cents,
    nextDue: data.next_due ? mapInvoice(data.next_due) : null,
  };
}

export async function fetchContractOverview(): Promise<ContractOverview> {
  const response = await sessionFetch("/api/contract");
  if (!response.ok) {
    throw new Error(
      `Failed to fetch contract overview: ${response.statusText}`,
    );
  }
  const json = (await response.json()) as { data: BackendContractOverview };
  return mapContractOverview(json.data);
}

/**
 * Cent-Betrag als Euro. Ganzzahl-Cent statt Fließkomma: 19,90 € ist binär
 * nicht exakt darstellbar, und eine Rundungsabweichung auf Geld ist ein
 * Support-Ticket.
 */
export function formatCents(cents: number): string {
  return new Intl.NumberFormat("de-DE", {
    style: "currency",
    currency: "EUR",
  }).format(cents / 100);
}

/**
 * Anzeigestatus einer Rechnung. "Überfällig" ist kein gespeicherter Status,
 * sondern folgt aus Fälligkeitsdatum und heutigem Tag — deshalb leitet das
 * Backend ihn ab und schickt ihn als eigenes Feld mit.
 */
export function invoiceStatusLabel(invoice: Invoice): string {
  if (invoice.status === "bezahlt") return "Bezahlt";
  if (invoice.status === "storniert") return "Storniert";
  return invoice.overdue ? "Überfällig" : "Offen";
}

/** Farbton des Status-Chips. Muss zu invoiceStatusLabel passen. */
export function invoiceStatusTone(
  invoice: Invoice,
): "green" | "red" | "orange" | "gray" {
  if (invoice.status === "bezahlt") return "green";
  if (invoice.status === "storniert") return "gray";
  return invoice.overdue ? "red" : "orange";
}
