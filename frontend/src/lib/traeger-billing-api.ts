/**
 * Client of the billing runs in the Träger portal (#4668): the runs of a
 * holiday period with their preview, corrections with a reason, the release
 * that issues the numbered invoices, and the number range of the invoices.
 * Money stays whole cent. Backend int64 IDs leave here as strings.
 */

import { traegerRequest } from "./traeger-api";

export type BillingRunStatus = "calculated" | "released";

export interface BillingRun {
  readonly id: string;
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
  "base" | "sibling" | "benefits_exemption" | "correction";

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
  /** Empty for a draft. */
  readonly number: string;
  readonly issuedOn: string | null;
  readonly registrationId: string;
  readonly recipientFirstName: string;
  readonly recipientLastName: string;
  readonly recipientEmail: string;
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
  readonly numberPattern: string;
  readonly numberRangeSet: boolean;
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

interface BackendDocument {
  number: string;
  issued_on: string | null;
  registration_id: number;
  recipient_first_name: string;
  recipient_last_name: string;
  recipient_email: string;
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
  number_pattern: string;
  number_range_set: boolean;
}

function runFromBackend(run: BackendRun): BillingRun {
  return {
    id: String(run.id),
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
    number: document.number,
    issuedOn: document.issued_on,
    registrationId: String(document.registration_id),
    recipientFirstName: document.recipient_first_name,
    recipientLastName: document.recipient_last_name,
    recipientEmail: document.recipient_email,
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
    numberPattern: run.number_pattern,
    numberRangeSet: run.number_range_set,
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
