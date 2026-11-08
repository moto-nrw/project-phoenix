/**
 * Client of the SEPA direct debit file (#4697): the debits of an issued
 * monthly or holiday run, the PAIN.008 file the office submits to its bank,
 * the confirmation that it did, and the collections of one mandate. No
 * answer carries an IBAN of a family; only the file does, and it is never
 * cached. Backend int64 IDs leave here as strings; calendar dates stay
 * "YYYY-MM-DD".
 */

import { traegerRequest } from "./traeger-api";
import { type BillingFile, billingFile } from "./traeger-billing-api";

/** The kind of run a file belongs to, as the path names it. */
export type SepaDebitRunKind = "care-runs" | "holiday-runs";
export type SepaSequenceType = "FRST" | "RCUR" | "OOFF";
type SepaDebitStatus = "pending" | "submitted";
/** Why a pending debit stays out of the file. */
export type SepaDebitLeft =
  "cancelled" | "revoked" | "expired" | "creditor_changed";

export interface SepaDebit {
  readonly id: string;
  readonly mandateId: string;
  readonly contractId: string;
  readonly invoiceNumber: string;
  readonly recipientName: string;
  readonly amountCents: number;
  /** Calendar date the invoice is due, "YYYY-MM-DD". */
  readonly dueOn: string;
  readonly sequenceType: SepaSequenceType;
  readonly mandateReference: string;
  readonly ibanMasked: string;
  readonly status: SepaDebitStatus;
  readonly left: SepaDebitLeft | null;
  readonly submittedAt: string | null;
}

export interface SepaDebitGroup {
  readonly sequenceType: SepaSequenceType;
  /** Calendar date the bank collects, "YYYY-MM-DD". */
  readonly collectOn: string;
  readonly count: number;
  readonly totalCents: number;
}

export interface SepaDebitRun {
  readonly creditorId: string;
  /** The account holder of the Träger's default bank account. */
  readonly creditorName: string;
  readonly creditorIbanMasked: string;
  /** The Träger has no default bank account: there is no file. */
  readonly accountMissing: boolean;
  /** Names the file as it is now; the confirmation carries it. */
  readonly messageId: string;
  readonly count: number;
  readonly totalCents: number;
  readonly groups: readonly SepaDebitGroup[];
  /** Debits whose due date passed; the file collects them tomorrow. */
  readonly pastDue: number;
  readonly debits: readonly SepaDebit[];
  readonly submitted: readonly SepaDebit[];
  readonly left: readonly SepaDebit[];
  readonly submittedAt: string | null;
}

interface BackendDebit {
  id: number;
  mandate_id: number;
  contract_id: number;
  invoice_number: string;
  recipient_name: string;
  amount_cents: number;
  due_on: string;
  sequence_type: SepaSequenceType;
  mandate_reference: string;
  iban_masked: string;
  status: SepaDebitStatus;
  left?: SepaDebitLeft;
  submitted_at: string | null;
}

interface BackendDebitRun {
  creditor_id: string;
  creditor_name: string;
  creditor_iban_masked: string;
  account_missing: boolean;
  message_id: string;
  count: number;
  total_cents: number;
  groups:
    | {
        sequence_type: SepaSequenceType;
        collect_on: string;
        count: number;
        total_cents: number;
      }[]
    | null;
  past_due: number;
  debits: BackendDebit[] | null;
  submitted: BackendDebit[] | null;
  left: BackendDebit[] | null;
  submitted_at: string | null;
}

function debitFromBackend(debit: BackendDebit): SepaDebit {
  return {
    id: String(debit.id),
    mandateId: String(debit.mandate_id),
    contractId: String(debit.contract_id),
    invoiceNumber: debit.invoice_number,
    recipientName: debit.recipient_name,
    amountCents: debit.amount_cents,
    dueOn: debit.due_on,
    sequenceType: debit.sequence_type,
    mandateReference: debit.mandate_reference,
    ibanMasked: debit.iban_masked,
    status: debit.status,
    left: debit.left ?? null,
    submittedAt: debit.submitted_at,
  };
}

function debitRunFromBackend(run: BackendDebitRun): SepaDebitRun {
  return {
    creditorId: run.creditor_id,
    creditorName: run.creditor_name,
    creditorIbanMasked: run.creditor_iban_masked,
    accountMissing: run.account_missing,
    messageId: run.message_id,
    count: run.count,
    totalCents: run.total_cents,
    groups: (run.groups ?? []).map((group) => ({
      sequenceType: group.sequence_type,
      collectOn: group.collect_on,
      count: group.count,
      totalCents: group.total_cents,
    })),
    pastDue: run.past_due,
    debits: (run.debits ?? []).map(debitFromBackend),
    submitted: (run.submitted ?? []).map(debitFromBackend),
    left: (run.left ?? []).map(debitFromBackend),
    submittedAt: run.submitted_at,
  };
}

function runPath(kind: SepaDebitRunKind, runId: string): string {
  return `/api/traeger/sepa-debits/${kind}/${encodeURIComponent(runId)}`;
}

/** The debit file of an issued run, without bank data of a family. */
export async function fetchSepaDebitRun(
  kind: SepaDebitRunKind,
  runId: string,
): Promise<SepaDebitRun> {
  return debitRunFromBackend(
    await traegerRequest<BackendDebitRun>(runPath(kind, runId)),
  );
}

/** The PAIN.008 file of the pending debits; it changes nothing. */
export async function downloadSepaDebitFile(
  kind: SepaDebitRunKind,
  runId: string,
): Promise<BillingFile> {
  return billingFile(`${runPath(kind, runId)}/file`, {}, "lastschrift.xml");
}

/** Confirms that the office submitted the file messageId to its bank. */
export async function submitSepaDebits(
  kind: SepaDebitRunKind,
  runId: string,
  messageId: string,
): Promise<SepaDebitRun> {
  return debitRunFromBackend(
    await traegerRequest<BackendDebitRun>(
      `${runPath(kind, runId)}/submission`,
      {
        method: "POST",
        body: { message_id: messageId },
      },
    ),
  );
}

/** The debits of a mandate, newest first: the collection history. */
export async function fetchSepaMandateCollections(
  mandateId: string,
): Promise<SepaDebit[]> {
  const debits = await traegerRequest<BackendDebit[] | null>(
    `/api/traeger/sepa-mandates/${encodeURIComponent(mandateId)}/collections`,
  );
  return (debits ?? []).map(debitFromBackend);
}
