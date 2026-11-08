/**
 * Client of the Jahresbescheide of the monthly care (#6219, ADR 0177): a
 * run assesses the months of a year from its first month to December in one
 * fee notice per contract, with monthly installments and, after a change of
 * the contract, a change notice with „bisher / neu / Zu- bzw. Abgang“.
 * Money stays whole cent; backend IDs leave here as strings, calendar dates
 * stay "YYYY-MM-DD".
 */

import { traegerRequest } from "./traeger-api";

const RUNS_PATH = "/api/traeger/care-notice-runs";

export type CareNoticeRunStatus = "calculated" | "released";
export type CareNoticeKind = "initial" | "change";
type CareNoticeLineKind = "module" | "reduction" | "cost_share";

export interface CareNoticeRun {
  readonly id: string;
  readonly year: number;
  readonly periodFrom: string;
  readonly periodUntil: string;
  readonly status: CareNoticeRunStatus;
  readonly schoolIds: readonly string[];
  readonly noticeDate: string | null;
  readonly issuedOn: string | null;
  readonly noticeCount: number | null;
  readonly totalCents: number | null;
}

export interface CareNoticeRunList {
  readonly runs: readonly CareNoticeRun[];
  readonly suggestedYear: number;
  readonly suggestedMonth: string;
}

/** One basis of calculation over a span of months with one monthly amount. */
export interface CareNoticeLine {
  readonly kind: CareNoticeLineKind;
  readonly label: string;
  readonly detail: string;
  readonly periodFrom: string;
  readonly periodUntil: string;
  readonly months: number;
  readonly quantity: number;
  readonly monthlyPreviousCents: number;
  readonly monthlyCents: number;
  readonly annualPreviousCents: number;
  readonly annualCents: number;
  readonly deltaCents: number;
}

export interface CareNoticeInstallment {
  readonly month: string;
  readonly dueOn: string;
  readonly immediatelyDue: boolean;
  readonly previousCents: number;
  readonly amountCents: number;
  readonly deltaCents: number;
}

interface CareNoticeReference {
  readonly id: string;
  readonly number: string;
  readonly issuedOn: string | null;
}

interface CareNoticeRecipient {
  readonly firstName: string;
  readonly lastName: string;
  readonly email: string;
  readonly secondName: string;
  readonly address: {
    readonly street: string;
    readonly postalCode: string;
    readonly city: string;
  } | null;
}

export interface CareNotice {
  readonly id: string;
  readonly number: string;
  readonly issuedOn: string | null;
  readonly kind: CareNoticeKind;
  readonly contractId: string;
  readonly schoolId: string;
  readonly schoolName: string;
  readonly childFirstName: string;
  readonly childLastName: string;
  readonly recipient: CareNoticeRecipient;
  readonly paymentMethod: string;
  readonly year: number;
  readonly periodFrom: string;
  readonly periodUntil: string;
  readonly lines: readonly CareNoticeLine[];
  readonly installments: readonly CareNoticeInstallment[];
  readonly previousCents: number;
  readonly totalCents: number;
  readonly deltaCents: number;
  readonly amends: CareNoticeReference | null;
  readonly amendedBy: CareNoticeReference | null;
  readonly legalRemedy: string;
}

export interface CareNoticeRunDetail extends CareNoticeRun {
  readonly notices: readonly CareNotice[];
  readonly unchanged: number;
  readonly billed: number;
  readonly unpricedCount: number;
  readonly duplicateCount: number;
  readonly previewCount: number;
  readonly previewCents: number;
  readonly previewDeltaCents: number;
  readonly fingerprint: string;
  readonly numberPattern: string;
  readonly numberRangeSet: boolean;
  readonly legalRemedyMissing: boolean;
}

/** A notice with the letterhead of the Träger, as page 1 shows it. */
export interface CareNoticeLetter {
  readonly notice: CareNotice;
  readonly senderText: string;
  readonly footerText: string;
  readonly contactName: string;
  readonly contactPhone: string;
  readonly contactEmail: string;
  readonly accentColor: string;
  readonly logoDataUrl: string;
  readonly bankAccount: {
    readonly accountHolder: string;
    readonly iban: string;
    readonly bic: string;
    readonly bankName: string;
  } | null;
}

interface BackendRun {
  id: number;
  year: number;
  period_from: string;
  period_until: string;
  status: CareNoticeRunStatus;
  school_ids: number[] | null;
  notice_date: string | null;
  issued_on: string | null;
  notice_count: number | null;
  total_cents: number | null;
}

interface BackendLine {
  kind: CareNoticeLineKind;
  label: string;
  detail: string;
  period_from: string;
  period_until: string;
  months: number;
  quantity: number;
  monthly_previous_cents: number;
  monthly_cents: number;
  annual_previous_cents: number;
  annual_cents: number;
  delta_cents: number;
}

interface BackendInstallment {
  month: string;
  due_on: string;
  immediately_due: boolean;
  previous_cents: number;
  amount_cents: number;
  delta_cents: number;
}

interface BackendReference {
  id: number;
  number: string;
  issued_on: string | null;
}

interface BackendNotice {
  id: number;
  number: string;
  issued_on: string | null;
  kind: CareNoticeKind;
  contract_id: number;
  school_id: number;
  school_name: string;
  child_first_name: string;
  child_last_name: string;
  recipient: {
    first_name?: string;
    last_name?: string;
    email?: string;
    second_name?: string;
    address?: { street: string; postal_code: string; city: string } | null;
  } | null;
  payment_method: string;
  year: number;
  period_from: string;
  period_until: string;
  lines: BackendLine[] | null;
  installments: BackendInstallment[] | null;
  previous_cents: number;
  total_cents: number;
  delta_cents: number;
  amends: BackendReference | null;
  amended_by: BackendReference | null;
  legal_remedy: string;
}

interface BackendRunDetail extends BackendRun {
  notices: BackendNotice[] | null;
  unchanged: number;
  billed: number;
  unpriced: unknown[] | null;
  duplicates: unknown[] | null;
  preview_count: number;
  preview_cents: number;
  preview_delta_cents: number;
  fingerprint: string;
  number_pattern: string;
  number_range_set: boolean;
  legal_remedy_missing: boolean;
}

interface BackendLetter {
  notice: BackendNotice;
  sender_text: string;
  footer_text: string;
  contact_name: string;
  contact_phone: string;
  contact_email: string;
  accent_color: string;
  logo_data_url: string;
  bank_account: {
    account_holder: string;
    iban: string;
    bic: string;
    bank_name: string;
  } | null;
}

function runFromBackend(run: BackendRun): CareNoticeRun {
  return {
    id: String(run.id),
    year: run.year,
    periodFrom: run.period_from,
    periodUntil: run.period_until,
    status: run.status,
    schoolIds: (run.school_ids ?? []).map(String),
    noticeDate: run.notice_date,
    issuedOn: run.issued_on,
    noticeCount: run.notice_count,
    totalCents: run.total_cents,
  };
}

function referenceFromBackend(
  reference: BackendReference | null,
): CareNoticeReference | null {
  return reference
    ? {
        id: String(reference.id),
        number: reference.number,
        issuedOn: reference.issued_on,
      }
    : null;
}

function noticeFromBackend(notice: BackendNotice): CareNotice {
  const recipient = notice.recipient ?? {};
  return {
    id: String(notice.id),
    number: notice.number,
    issuedOn: notice.issued_on,
    kind: notice.kind,
    contractId: String(notice.contract_id),
    schoolId: String(notice.school_id),
    schoolName: notice.school_name,
    childFirstName: notice.child_first_name,
    childLastName: notice.child_last_name,
    recipient: {
      firstName: recipient.first_name ?? "",
      lastName: recipient.last_name ?? "",
      email: recipient.email ?? "",
      secondName: recipient.second_name ?? "",
      address: recipient.address
        ? {
            street: recipient.address.street,
            postalCode: recipient.address.postal_code,
            city: recipient.address.city,
          }
        : null,
    },
    paymentMethod: notice.payment_method,
    year: notice.year,
    periodFrom: notice.period_from,
    periodUntil: notice.period_until,
    lines: (notice.lines ?? []).map((line) => ({
      kind: line.kind,
      label: line.label,
      detail: line.detail,
      periodFrom: line.period_from,
      periodUntil: line.period_until,
      months: line.months,
      quantity: line.quantity,
      monthlyPreviousCents: line.monthly_previous_cents,
      monthlyCents: line.monthly_cents,
      annualPreviousCents: line.annual_previous_cents,
      annualCents: line.annual_cents,
      deltaCents: line.delta_cents,
    })),
    installments: (notice.installments ?? []).map((installment) => ({
      month: installment.month,
      dueOn: installment.due_on,
      immediatelyDue: installment.immediately_due,
      previousCents: installment.previous_cents,
      amountCents: installment.amount_cents,
      deltaCents: installment.delta_cents,
    })),
    previousCents: notice.previous_cents,
    totalCents: notice.total_cents,
    deltaCents: notice.delta_cents,
    amends: referenceFromBackend(notice.amends),
    amendedBy: referenceFromBackend(notice.amended_by),
    legalRemedy: notice.legal_remedy,
  };
}

function detailFromBackend(detail: BackendRunDetail): CareNoticeRunDetail {
  return {
    ...runFromBackend(detail),
    notices: (detail.notices ?? []).map(noticeFromBackend),
    unchanged: detail.unchanged,
    billed: detail.billed,
    unpricedCount: (detail.unpriced ?? []).length,
    duplicateCount: (detail.duplicates ?? []).length,
    previewCount: detail.preview_count,
    previewCents: detail.preview_cents,
    previewDeltaCents: detail.preview_delta_cents,
    fingerprint: detail.fingerprint,
    numberPattern: detail.number_pattern,
    numberRangeSet: detail.number_range_set,
    legalRemedyMissing: detail.legal_remedy_missing,
  };
}

export async function fetchCareNoticeRuns(): Promise<CareNoticeRunList> {
  const body = await traegerRequest<{
    runs: BackendRun[] | null;
    suggested_year: number;
    suggested_month: string;
  }>(RUNS_PATH);
  return {
    runs: (body.runs ?? []).map(runFromBackend),
    suggestedYear: body.suggested_year,
    suggestedMonth: body.suggested_month,
  };
}

/** What a new run assesses: the year from its first month on. */
export interface CareNoticeRunStartInput {
  readonly year: number;
  readonly fromMonth: string;
  /** Empty: every school of the Träger. */
  readonly schoolIds: readonly string[];
  /** Null takes the day of the release. */
  readonly noticeDate: string | null;
}

export async function startCareNoticeRun(
  input: CareNoticeRunStartInput,
): Promise<CareNoticeRun> {
  return runFromBackend(
    await traegerRequest<BackendRun>(RUNS_PATH, {
      method: "POST",
      body: {
        year: input.year,
        from_month: input.fromMonth,
        school_ids: input.schoolIds.map(Number),
        notice_date: input.noticeDate,
      },
    }),
  );
}

export async function fetchCareNoticeRun(
  runId: string,
): Promise<CareNoticeRunDetail> {
  return detailFromBackend(
    await traegerRequest<BackendRunDetail>(
      `${RUNS_PATH}/${encodeURIComponent(runId)}`,
    ),
  );
}

/** Issues the reviewed notices; a changed state is refused (409). */
export async function releaseCareNoticeRun(
  run: Pick<
    CareNoticeRunDetail,
    "id" | "previewCount" | "previewCents" | "fingerprint"
  >,
): Promise<CareNoticeRunDetail> {
  return detailFromBackend(
    await traegerRequest<BackendRunDetail>(
      `${RUNS_PATH}/${encodeURIComponent(run.id)}/release`,
      {
        method: "POST",
        body: {
          document_count: run.previewCount,
          total_cents: run.previewCents,
          fingerprint: run.fingerprint,
        },
      },
    ),
  );
}

export async function fetchCareNoticeLetter(
  runId: string,
  contractId: string,
): Promise<CareNoticeLetter> {
  const letter = await traegerRequest<BackendLetter>(
    `${RUNS_PATH}/${encodeURIComponent(runId)}/notices/${encodeURIComponent(contractId)}/letter`,
  );
  return {
    notice: noticeFromBackend(letter.notice),
    senderText: letter.sender_text,
    footerText: letter.footer_text,
    contactName: letter.contact_name,
    contactPhone: letter.contact_phone,
    contactEmail: letter.contact_email,
    accentColor: letter.accent_color,
    logoDataUrl: letter.logo_data_url,
    bankAccount: letter.bank_account
      ? {
          accountHolder: letter.bank_account.account_holder,
          iban: letter.bank_account.iban,
          bic: letter.bank_account.bic,
          bankName: letter.bank_account.bank_name,
        }
      : null,
  };
}
