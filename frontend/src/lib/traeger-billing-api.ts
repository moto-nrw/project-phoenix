/**
 * Client of the billing runs in the Träger portal (#4668): the runs of a
 * holiday period with their preview, corrections with a reason, the release
 * that issues the numbered invoices, the overview of a released run for the
 * Kämmerei, and the number range of the invoices.
 * Money stays whole cent. Backend int64 IDs leave here as strings.
 */

import { apiErrorFromBody, transportFetch } from "./api-error";
import { filenameFromDisposition } from "./file-download";
import { traegerRequest } from "./traeger-api";

export type BillingRunStatus = "calculated" | "released";

export interface BillingRun {
  readonly id: string;
  readonly holidayPeriodId: string;
  readonly periodName: string;
  /** Calendar dates "YYYY-MM-DD". */
  readonly serviceFrom: string;
  readonly serviceUntil: string;
  readonly status: BillingRunStatus;
  readonly createdAt: string;
  readonly releasedAt: string | null;
  readonly issuedOn: string | null;
  readonly documentCount: number | null;
  readonly totalCents: number | null;
}

type BillingPositionKind =
  "base" | "sibling" | "benefits_exemption" | "correction" | "credit";

/** An invoice, its Storno or its Gutschrift (#4690). */
export type BillingDocumentKind = "invoice" | "cancellation" | "credit";

/**
 * A document of the history of an invoice: its Storno, its Gutschriften and
 * the invoice that replaces it, which may belong to another run.
 */
interface BillingDocumentLink {
  readonly id: string;
  readonly runId: string;
  readonly kind: BillingDocumentKind;
  readonly number: string;
  readonly issuedOn: string;
  readonly totalCents: number;
  readonly reason: string;
}

export interface BillingPosition {
  readonly kind: BillingPositionKind;
  /** Negative for a discount. */
  readonly amountCents: number;
  readonly weekStartsOn: string | null;
  readonly weekEndsOn: string | null;
  readonly timeModelName: string;
  readonly childRank: number | null;
  readonly discountBasisPoints: number | null;
  readonly discountCents: number | null;
  /** The note of an exemption or the reason of a correction. */
  readonly note: string;
}

interface BillingCorrection {
  readonly id: string;
  readonly amountCents: number;
  readonly totalBeforeCents: number;
  readonly totalAfterCents: number;
  readonly reason: string;
  readonly createdAt: string;
}

export interface BillingChild {
  readonly childId: string;
  readonly firstName: string;
  readonly lastName: string;
  readonly positions: readonly BillingPosition[];
  readonly corrections: readonly BillingCorrection[];
  readonly totalCents: number;
}

export interface BillingDocument {
  /** Empty for a draft; the invoice once issued. */
  readonly id: string;
  readonly kind: BillingDocumentKind;
  /**
   * The number of the invoice a Storno or Gutschrift belongs to, or of the
   * cancelled invoice an invoice replaces; empty otherwise (#4690).
   */
  readonly referenceNumber: string;
  /** The reason of a Storno or Gutschrift. */
  readonly reason: string;
  /** Storno, Gutschriften and the replacing invoice of an issued invoice. */
  readonly history: readonly BillingDocumentLink[];
  /** Empty for a draft. */
  readonly number: string;
  readonly issuedOn: string | null;
  readonly registrationId: string;
  readonly recipientFirstName: string;
  readonly recipientLastName: string;
  readonly recipientEmail: string;
  /** The family's debtor number the invoice copies; empty without (#5621). */
  readonly debtorNumber: string;
  readonly serviceFrom: string;
  readonly serviceUntil: string;
  readonly children: readonly BillingChild[];
  readonly totalCents: number;
}

export interface BillingRunDetail extends BillingRun {
  readonly documents: readonly BillingDocument[];
  readonly previewCount: number;
  readonly previewCents: number;
  /** Confirmed registrations without a frozen contribution. */
  readonly unbilled: number;
  /**
   * Draft invoices without a debtor number while the Träger asks for one
   * (#5621); 0 otherwise.
   */
  readonly missingDebtorNumbers: number;
  /**
   * Draft invoices with an amount and without a debtor number while the
   * payment text names it, whatever the switch (#5622): after the release
   * their letters are refused until the placeholder goes.
   */
  readonly paymentTextMissingDebtorNumbers: number;
  readonly numberPattern: string;
  readonly numberRangeSet: boolean;
  /** Set once the confirmation letters of the released run exist (#4669). */
  readonly letters: BillingLetters | null;
}

/** An emergency number of a hosting OGS on the letters of a run. */
export interface BillingEmergencyContact {
  readonly name: string;
  readonly phone: string;
}

/** What the confirmation letters of a released run print (#4669). */
export interface BillingLettersInput {
  /** Calendar date "YYYY-MM-DD". */
  readonly paymentDueOn: string;
  readonly notes: string;
  readonly emergencyContacts: readonly BillingEmergencyContact[];
}

export interface BillingLetters extends BillingLettersInput {
  readonly createdAt: string;
}

export interface BillingPeriod {
  readonly id: string;
  readonly name: string;
  readonly startsOn: string;
  readonly endsOn: string;
}

export interface NumberRange {
  readonly service: string;
  readonly pattern: string;
  /** The first number of this year in the pattern. */
  readonly example: string;
}

interface BackendRun {
  id: number;
  holiday_period_id?: number;
  period_name: string;
  service_from: string;
  service_until: string;
  status: BillingRunStatus;
  created_at: string;
  released_at: string | null;
  issued_on: string | null;
  document_count: number | null;
  total_cents: number | null;
}

interface BackendPosition {
  kind: BillingPositionKind;
  amount_cents: number;
  week_starts_on: string | null;
  week_ends_on: string | null;
  time_model_name: string;
  child_rank: number | null;
  discount_basis_points: number | null;
  discount_cents: number | null;
  note: string;
}

interface BackendDocumentLink {
  id: number;
  run_id: number;
  kind: BillingDocumentKind;
  number: string;
  issued_on: string;
  total_cents: number;
  reason: string;
}

interface BackendDocument {
  id: number;
  kind?: BillingDocumentKind;
  reference_number?: string;
  reason?: string;
  history?: BackendDocumentLink[] | null;
  number: string;
  issued_on: string | null;
  registration_id: number;
  recipient_first_name: string;
  recipient_last_name: string;
  recipient_email: string;
  debtor_number?: string;
  service_from: string;
  service_until: string;
  children?:
    | {
        child_id: number;
        first_name: string;
        last_name: string;
        positions?: BackendPosition[] | null;
        corrections?:
          | {
              id: number;
              amount_cents: number;
              total_before_cents: number;
              total_after_cents: number;
              reason: string;
              created_at: string;
            }[]
          | null;
        total_cents: number;
      }[]
    | null;
  total_cents: number;
}

interface BackendRunDetail extends BackendRun {
  documents?: BackendDocument[] | null;
  preview_count: number;
  preview_cents: number;
  unbilled: number;
  missing_debtor_numbers?: number;
  payment_text_missing_debtor_numbers?: number;
  number_pattern: string;
  number_range_set: boolean;
  letters?: BackendLetters | null;
}

interface BackendLetters {
  payment_due_on: string;
  notes: string;
  emergency_contacts?: { name: string; phone: string }[] | null;
  created_at: string;
}

function lettersFromBackend(letters: BackendLetters): BillingLetters {
  return {
    paymentDueOn: letters.payment_due_on,
    notes: letters.notes,
    emergencyContacts: (letters.emergency_contacts ?? []).map((contact) => ({
      name: contact.name,
      phone: contact.phone,
    })),
    createdAt: letters.created_at,
  };
}

function runFromBackend(run: BackendRun): BillingRun {
  return {
    id: String(run.id),
    holidayPeriodId: String(run.holiday_period_id ?? ""),
    periodName: run.period_name,
    serviceFrom: run.service_from,
    serviceUntil: run.service_until,
    status: run.status,
    createdAt: run.created_at,
    releasedAt: run.released_at,
    issuedOn: run.issued_on,
    documentCount: run.document_count,
    totalCents: run.total_cents,
  };
}

function positionFromBackend(position: BackendPosition): BillingPosition {
  return {
    kind: position.kind,
    amountCents: position.amount_cents,
    weekStartsOn: position.week_starts_on,
    weekEndsOn: position.week_ends_on,
    timeModelName: position.time_model_name,
    childRank: position.child_rank,
    discountBasisPoints: position.discount_basis_points,
    discountCents: position.discount_cents,
    note: position.note,
  };
}

function documentFromBackend(document: BackendDocument): BillingDocument {
  return {
    id: document.id ? String(document.id) : "",
    kind: document.kind ?? "invoice",
    referenceNumber: document.reference_number ?? "",
    reason: document.reason ?? "",
    history: (document.history ?? []).map((link) => ({
      id: String(link.id),
      runId: String(link.run_id),
      kind: link.kind,
      number: link.number,
      issuedOn: link.issued_on,
      totalCents: link.total_cents,
      reason: link.reason,
    })),
    number: document.number,
    issuedOn: document.issued_on,
    registrationId: String(document.registration_id),
    recipientFirstName: document.recipient_first_name,
    recipientLastName: document.recipient_last_name,
    recipientEmail: document.recipient_email,
    debtorNumber: document.debtor_number ?? "",
    serviceFrom: document.service_from,
    serviceUntil: document.service_until,
    totalCents: document.total_cents,
    children: (document.children ?? []).map((child) => ({
      childId: String(child.child_id),
      firstName: child.first_name,
      lastName: child.last_name,
      totalCents: child.total_cents,
      positions: (child.positions ?? []).map(positionFromBackend),
      corrections: (child.corrections ?? []).map((correction) => ({
        id: String(correction.id),
        amountCents: correction.amount_cents,
        totalBeforeCents: correction.total_before_cents,
        totalAfterCents: correction.total_after_cents,
        reason: correction.reason,
        createdAt: correction.created_at,
      })),
    })),
  };
}

function detailFromBackend(run: BackendRunDetail): BillingRunDetail {
  return {
    ...runFromBackend(run),
    documents: (run.documents ?? []).map(documentFromBackend),
    previewCount: run.preview_count,
    previewCents: run.preview_cents,
    unbilled: run.unbilled,
    missingDebtorNumbers: run.missing_debtor_numbers ?? 0,
    paymentTextMissingDebtorNumbers:
      run.payment_text_missing_debtor_numbers ?? 0,
    numberPattern: run.number_pattern,
    numberRangeSet: run.number_range_set,
    letters: run.letters ? lettersFromBackend(run.letters) : null,
  };
}

const RUNS_PATH = "/api/traeger/billing-runs";
const runPath = (id: string) => `${RUNS_PATH}/${encodeURIComponent(id)}`;

/** The runs of the Träger, newest first. */
export async function fetchBillingRuns(): Promise<BillingRun[]> {
  const runs = await traegerRequest<BackendRun[] | null>(RUNS_PATH);
  return (runs ?? []).map(runFromBackend);
}

/** The published holiday periods a run can bill, latest first. */
export async function fetchBillingPeriods(): Promise<BillingPeriod[]> {
  const periods = await traegerRequest<
    { id: number; name: string; starts_on: string; ends_on: string }[] | null
  >(`${RUNS_PATH}/periods`);
  return (periods ?? []).map((period) => ({
    id: String(period.id),
    name: period.name,
    startsOn: period.starts_on,
    endsOn: period.ends_on,
  }));
}

export async function startBillingRun(periodId: string): Promise<BillingRun> {
  return runFromBackend(
    await traegerRequest<BackendRun>(RUNS_PATH, {
      method: "POST",
      body: { holiday_period_id: Number(periodId) },
    }),
  );
}

/** A run with its preview, or with its issued invoices once released. */
export async function fetchBillingRun(id: string): Promise<BillingRunDetail> {
  return detailFromBackend(await traegerRequest<BackendRunDetail>(runPath(id)));
}

/** Sets the amount of one child to totalCents, with a reason. */
export async function correctBillingRun(
  id: string,
  correction: {
    readonly registrationId: string;
    readonly childId: string;
    readonly totalCents: number;
    readonly reason: string;
  },
): Promise<BillingRunDetail> {
  return detailFromBackend(
    await traegerRequest<BackendRunDetail>(`${runPath(id)}/corrections`, {
      method: "POST",
      body: {
        registration_id: Number(correction.registrationId),
        child_id: Number(correction.childId),
        total_cents: correction.totalCents,
        reason: correction.reason,
      },
    }),
  );
}

/**
 * Cancels an issued invoice with a Storno of its own (#4690). The Storno
 * gets the next number of the range; the registration comes back into the
 * next run of the period.
 */
export async function cancelBillingDocument(
  runId: string,
  documentId: string,
  reason: string,
): Promise<BillingDocument> {
  return documentFromBackend(
    await traegerRequest<BackendDocument>(
      `${runPath(runId)}/documents/${encodeURIComponent(documentId)}/cancellation`,
      { method: "POST", body: { reason } },
    ),
  );
}

/** Credits amountCents of one child of an issued invoice (#4690). */
export async function creditBillingDocument(
  runId: string,
  documentId: string,
  credit: {
    readonly childId: string;
    readonly amountCents: number;
    readonly reason: string;
  },
): Promise<BillingDocument> {
  return documentFromBackend(
    await traegerRequest<BackendDocument>(
      `${runPath(runId)}/documents/${encodeURIComponent(documentId)}/credits`,
      {
        method: "POST",
        body: {
          child_id: Number(credit.childId),
          amount_cents: credit.amountCents,
          reason: credit.reason,
        },
      },
    ),
  );
}

/**
 * Issues the invoices. The backend refuses when the preview no longer
 * matches the count and the sum the person checked.
 */
export async function releaseBillingRun(
  run: Pick<BillingRunDetail, "id" | "previewCount" | "previewCents">,
): Promise<BillingRunDetail> {
  return detailFromBackend(
    await traegerRequest<BackendRunDetail>(`${runPath(run.id)}/release`, {
      method: "POST",
      body: {
        document_count: run.previewCount,
        total_cents: run.previewCents,
      },
    }),
  );
}

function lettersBody(input: BillingLettersInput) {
  return {
    payment_due_on: input.paymentDueOn,
    notes: input.notes,
    emergency_contacts: input.emergencyContacts.map((contact) => ({
      name: contact.name,
      phone: contact.phone,
    })),
  };
}

/**
 * Creates the confirmation letter of every invoice of a released run once;
 * afterwards they never change.
 */
export async function createBillingLetters(
  runId: string,
  input: BillingLettersInput,
): Promise<BillingLetters> {
  return lettersFromBackend(
    await traegerRequest<BackendLetters>(`${runPath(runId)}/letters`, {
      method: "POST",
      body: lettersBody(input),
    }),
  );
}

/** A file with the name the backend chose. */
export interface BillingFile {
  readonly blob: Blob;
  readonly filename: string;
}

async function billingFile(
  path: string,
  init: RequestInit,
  fallbackName: string,
): Promise<BillingFile> {
  const response = await transportFetch(path, {
    credentials: "include",
    cache: "no-store",
    ...init,
  });
  if (!response.ok) {
    const text = await response.text();
    let body: unknown = null;
    try {
      body = text ? (JSON.parse(text) as unknown) : null;
    } catch {
      body = null;
    }
    throw apiErrorFromBody(
      `Request failed (${response.status})`,
      response.status,
      body,
    );
  }
  return {
    blob: await response.blob(),
    filename: filenameFromDisposition(response) ?? fallbackName,
  };
}

/** The letter of the first invoice with the input, without storing it. */
export async function previewBillingLetter(
  runId: string,
  input: BillingLettersInput,
): Promise<BillingFile> {
  return billingFile(
    `${runPath(runId)}/letters/preview`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(lettersBody(input)),
    },
    "vorschau.pdf",
  );
}

/** The stored letter of one invoice of the run. */
export async function downloadBillingLetter(
  runId: string,
  documentId: string,
): Promise<BillingFile> {
  return billingFile(
    `${runPath(runId)}/documents/${encodeURIComponent(documentId)}/letter`,
    {},
    "bestaetigung.pdf",
  );
}

/** All stored letters of the run in one PDF to print (#4670). */
export async function printBillingLetters(runId: string): Promise<BillingFile> {
  return billingFile(
    `${runPath(runId)}/letters/print`,
    {},
    "bestaetigungsschreiben.pdf",
  );
}

/** The state of the last mail of a family's letter (#4670). */
export type LetterMailStatus =
  "not_sent" | "queued" | "sent" | "failed" | "unknown";

/** The mail of the letter of one family. */
export interface LetterMailing {
  readonly documentId: string;
  /** An invoice, a Storno or a Gutschrift (#4690). */
  readonly kind: BillingDocumentKind;
  readonly number: string;
  readonly recipientFirstName: string;
  readonly recipientLastName: string;
  readonly recipientEmail: string;
  readonly status: LetterMailStatus;
  readonly sends: number;
  readonly requestedAt: string | null;
  readonly sentAt: string | null;
}

/** What a send to every family of the run did. */
export interface LetterSeries {
  readonly queued: number;
  readonly already: number;
  readonly withoutEmail: number;
}

interface BackendLetterMailing {
  document_id: number;
  kind?: BillingDocumentKind;
  number: string;
  recipient_first_name: string;
  recipient_last_name: string;
  recipient_email: string;
  status: LetterMailStatus;
  sends: number;
  requested_at: string | null;
  sent_at: string | null;
}

function mailingFromBackend(mailing: BackendLetterMailing): LetterMailing {
  return {
    documentId: String(mailing.document_id),
    kind: mailing.kind ?? "invoice",
    number: mailing.number,
    recipientFirstName: mailing.recipient_first_name,
    recipientLastName: mailing.recipient_last_name,
    recipientEmail: mailing.recipient_email,
    status: mailing.status,
    sends: mailing.sends,
    requestedAt: mailing.requested_at,
    sentAt: mailing.sent_at,
  };
}

/** The mail state of every family of a released run with letters. */
export async function fetchLetterMailings(
  runId: string,
): Promise<LetterMailing[]> {
  const body = await traegerRequest<{
    documents: BackendLetterMailing[] | null;
  }>(`${runPath(runId)}/mailings`);
  return (body.documents ?? []).map(mailingFromBackend);
}

/**
 * Mails the letter of every family with an address and no mail yet; a
 * second call sends nothing twice.
 */
export async function mailBillingLetters(runId: string): Promise<LetterSeries> {
  const series = await traegerRequest<{
    queued: number;
    already: number;
    without_email: number;
  }>(`${runPath(runId)}/mailings`, { method: "POST" });
  return {
    queued: series.queued,
    already: series.already,
    withoutEmail: series.without_email,
  };
}

/** Mails the letter of one family, first or again. */
export async function mailBillingLetter(
  runId: string,
  documentId: string,
): Promise<LetterMailing> {
  return mailingFromBackend(
    await traegerRequest<BackendLetterMailing>(
      `${runPath(runId)}/documents/${encodeURIComponent(documentId)}/mailings`,
      { method: "POST" },
    ),
  );
}

export type BillingRunExportFormat = "pdf" | "xlsx" | "csv";

/**
 * The overview of a released run for the Kämmerei (#4672): every invoice
 * with its positions and sums.
 */
export async function downloadBillingRunOverview(
  id: string,
  format: BillingRunExportFormat,
): Promise<BillingFile> {
  return billingFile(
    `${runPath(id)}/export?format=${format}`,
    {},
    `abrechnungsanordnung.${format}`,
  );
}

const NUMBER_RANGES_PATH = "/api/traeger/number-ranges";

/** The number range of the holiday invoices; the pattern may be empty. */
export async function fetchHolidayNumberRange(): Promise<NumberRange> {
  const ranges = await traegerRequest<
    { service: string; pattern: string; example: string }[] | null
  >(NUMBER_RANGES_PATH);
  const holiday = (ranges ?? []).find((range) => range.service === "holiday");
  return {
    service: "holiday",
    pattern: holiday?.pattern ?? "",
    example: holiday?.example ?? "",
  };
}

export async function saveHolidayNumberRange(
  pattern: string,
): Promise<NumberRange> {
  const range = await traegerRequest<{
    service: string;
    pattern: string;
    example: string;
  }>(`${NUMBER_RANGES_PATH}/holiday`, { method: "PUT", body: { pattern } });
  return {
    service: range.service,
    pattern: range.pattern,
    example: range.example,
  };
}
