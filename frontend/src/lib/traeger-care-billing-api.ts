/**
 * Client of the monthly contribution of the care contract (#4684): the
 * price list of the Träger (a monthly price per booked weekday, module and
 * school) and the monthly runs that bill a month by the contract in force.
 * Money stays whole cent. Backend int64 IDs leave here as strings; calendar
 * dates stay "YYYY-MM-DD".
 */

import { traegerRequest } from "./traeger-api";

export type CareWeekday = "mon" | "tue" | "wed" | "thu" | "fri" | "sat" | "sun";
export type CareRunStatus = "calculated" | "released";
export type CareMonthRule = "version_on_first" | "pro_rata";

export const CARE_WEEKDAYS: readonly CareWeekday[] = [
  "mon",
  "tue",
  "wed",
  "thu",
  "fri",
  "sat",
  "sun",
];

/** A module the contracts of the Träger book at one school. */
export interface CareModule {
  readonly schoolId: string;
  readonly schoolName: string;
  readonly careOfferingId: string;
  readonly moduleName: string;
  /** End time "HH:MM" or empty. */
  readonly endsAt: string;
  readonly bookedWeekdays: readonly CareWeekday[];
  readonly contractCount: number;
}

/** The monthly price of one weekday of a module from a month on. */
export interface CareTariff {
  readonly id: string;
  readonly schoolId: string;
  readonly careOfferingId: string;
  readonly moduleName: string;
  readonly weekday: CareWeekday;
  readonly validFrom: string;
  readonly validUntil: string | null;
  readonly monthlyCents: number;
}

export interface CareTariffs {
  readonly modules: readonly CareModule[];
  readonly tariffs: readonly CareTariff[];
  /** Per school ID the last month an issued invoice bills ("YYYY-MM-01"). */
  readonly billedUntil: Readonly<Record<string, string>>;
}

export interface CarePriceInput {
  readonly careOfferingId: string;
  readonly moduleName: string;
  readonly weekday: CareWeekday;
  readonly monthlyCents: number;
}

export interface CareRun {
  readonly id: string;
  readonly billingMonth: string;
  readonly status: CareRunStatus;
  readonly createdAt: string;
  readonly releasedAt: string | null;
  readonly issuedOn: string | null;
  readonly documentCount: number | null;
  readonly totalCents: number | null;
  /** Empty until a released run records the rule it billed by. */
  readonly monthRule: CareMonthRule | "";
}

export interface CareRunList {
  readonly runs: readonly CareRun[];
  /** The first month from the current one on without a released run. */
  readonly suggestedMonth: string;
}

export interface CareWeekdayPrice {
  readonly weekday: CareWeekday;
  readonly monthlyCents: number;
  readonly amountCents: number;
}

export interface CarePosition {
  readonly careOfferingId: string;
  readonly moduleName: string;
  readonly versionNumber: number;
  readonly serviceFrom: string;
  readonly serviceUntil: string;
  readonly weekdays: readonly CareWeekdayPrice[];
  readonly amountCents: number;
}

export interface CareInvoice {
  /** Empty for a draft. */
  readonly id: string;
  readonly number: string;
  readonly issuedOn: string | null;
  readonly contractId: string;
  readonly schoolId: string;
  readonly schoolName: string;
  readonly childFirstName: string;
  readonly childLastName: string;
  readonly recipient: {
    readonly firstName: string;
    readonly lastName: string;
    readonly email: string;
  };
  readonly positions: readonly CarePosition[];
  readonly totalCents: number;
}

export interface CareUnpriced {
  readonly contractId: string;
  readonly schoolId: string;
  readonly schoolName: string;
  readonly childFirstName: string;
  readonly childLastName: string;
  readonly careOfferingId: string;
  readonly moduleName: string;
  readonly weekday: CareWeekday;
}

export interface CareRunDetail extends CareRun {
  readonly invoices: readonly CareInvoice[];
  readonly unpriced: readonly CareUnpriced[];
  readonly billed: number;
  readonly previewCount: number;
  readonly previewCents: number;
  readonly numberPattern: string;
  readonly numberRangeSet: boolean;
}

interface BackendModule {
  school_id: number;
  school_name: string;
  care_offering_id: number;
  module_name: string;
  ends_at: string;
  booked_weekdays: CareWeekday[] | null;
  contract_count: number;
}

interface BackendTariff {
  id: number;
  school_id: number;
  care_offering_id: number;
  module_name: string;
  weekday: CareWeekday;
  valid_from: string;
  valid_until: string | null;
  monthly_cents: number;
}

interface BackendTariffs {
  modules: BackendModule[] | null;
  tariffs: BackendTariff[] | null;
  billed_until: Record<string, string> | null;
}

interface BackendRun {
  id: number;
  billing_month: string;
  status: CareRunStatus;
  created_at: string;
  released_at: string | null;
  issued_on: string | null;
  document_count: number | null;
  total_cents: number | null;
  month_rule: CareMonthRule | "";
}

interface BackendInvoice {
  id: number;
  number: string;
  issued_on: string | null;
  contract_id: number;
  school_id: number;
  school_name: string;
  child_first_name: string;
  child_last_name: string;
  recipient: { first_name: string; last_name: string; email: string };
  positions:
    | {
        care_offering_id: number;
        module_name: string;
        version_number: number;
        service_from: string;
        service_until: string;
        weekdays:
          | {
              weekday: CareWeekday;
              monthly_cents: number;
              amount_cents: number;
            }[]
          | null;
        amount_cents: number;
      }[]
    | null;
  total_cents: number;
}

interface BackendRunDetail extends BackendRun {
  invoices: BackendInvoice[] | null;
  unpriced:
    | {
        contract_id: number;
        school_id: number;
        school_name: string;
        child_first_name: string;
        child_last_name: string;
        care_offering_id: number;
        module_name: string;
        weekday: CareWeekday;
      }[]
    | null;
  billed: number;
  preview_count: number;
  preview_cents: number;
  number_pattern: string;
  number_range_set: boolean;
}

const TARIFFS_PATH = "/api/traeger/care-tariffs";
const RUNS_PATH = "/api/traeger/care-billing-runs";

function id(value: number): string {
  return value ? String(value) : "";
}

function tariffsFromBackend(body: BackendTariffs): CareTariffs {
  return {
    modules: (body.modules ?? []).map((module) => ({
      schoolId: id(module.school_id),
      schoolName: module.school_name,
      careOfferingId: id(module.care_offering_id),
      moduleName: module.module_name,
      endsAt: module.ends_at,
      bookedWeekdays: module.booked_weekdays ?? [],
      contractCount: module.contract_count,
    })),
    tariffs: (body.tariffs ?? []).map((tariff) => ({
      id: id(tariff.id),
      schoolId: id(tariff.school_id),
      careOfferingId: id(tariff.care_offering_id),
      moduleName: tariff.module_name,
      weekday: tariff.weekday,
      validFrom: tariff.valid_from,
      validUntil: tariff.valid_until,
      monthlyCents: tariff.monthly_cents,
    })),
    billedUntil: Object.fromEntries(
      Object.entries(body.billed_until ?? {}).map(([school, month]) => [
        school,
        month,
      ]),
    ),
  };
}

function runFromBackend(run: BackendRun): CareRun {
  return {
    id: id(run.id),
    billingMonth: run.billing_month,
    status: run.status,
    createdAt: run.created_at,
    releasedAt: run.released_at,
    issuedOn: run.issued_on,
    documentCount: run.document_count,
    totalCents: run.total_cents,
    monthRule: run.month_rule,
  };
}

function invoiceFromBackend(invoice: BackendInvoice): CareInvoice {
  return {
    id: id(invoice.id),
    number: invoice.number,
    issuedOn: invoice.issued_on,
    contractId: id(invoice.contract_id),
    schoolId: id(invoice.school_id),
    schoolName: invoice.school_name,
    childFirstName: invoice.child_first_name,
    childLastName: invoice.child_last_name,
    recipient: {
      firstName: invoice.recipient.first_name,
      lastName: invoice.recipient.last_name,
      email: invoice.recipient.email,
    },
    positions: (invoice.positions ?? []).map((position) => ({
      careOfferingId: id(position.care_offering_id),
      moduleName: position.module_name,
      versionNumber: position.version_number,
      serviceFrom: position.service_from,
      serviceUntil: position.service_until,
      weekdays: (position.weekdays ?? []).map((weekday) => ({
        weekday: weekday.weekday,
        monthlyCents: weekday.monthly_cents,
        amountCents: weekday.amount_cents,
      })),
      amountCents: position.amount_cents,
    })),
    totalCents: invoice.total_cents,
  };
}

function detailFromBackend(detail: BackendRunDetail): CareRunDetail {
  return {
    ...runFromBackend(detail),
    invoices: (detail.invoices ?? []).map(invoiceFromBackend),
    unpriced: (detail.unpriced ?? []).map((entry) => ({
      contractId: id(entry.contract_id),
      schoolId: id(entry.school_id),
      schoolName: entry.school_name,
      childFirstName: entry.child_first_name,
      childLastName: entry.child_last_name,
      careOfferingId: id(entry.care_offering_id),
      moduleName: entry.module_name,
      weekday: entry.weekday,
    })),
    billed: detail.billed,
    previewCount: detail.preview_count,
    previewCents: detail.preview_cents,
    numberPattern: detail.number_pattern,
    numberRangeSet: detail.number_range_set,
  };
}

export async function fetchCareTariffs(): Promise<CareTariffs> {
  return tariffsFromBackend(await traegerRequest<BackendTariffs>(TARIFFS_PATH));
}

/** Sets the prices of a school from `validFrom` (the 1st of a month) on. */
export async function saveCarePriceList(
  schoolId: string,
  validFrom: string,
  prices: readonly CarePriceInput[],
): Promise<CareTariffs> {
  return tariffsFromBackend(
    await traegerRequest<BackendTariffs>(TARIFFS_PATH, {
      method: "PUT",
      body: {
        school_id: Number(schoolId),
        valid_from: validFrom,
        prices: prices.map((price) => ({
          care_offering_id: Number(price.careOfferingId),
          module_name: price.moduleName,
          weekday: price.weekday,
          monthly_cents: price.monthlyCents,
        })),
      },
    }),
  );
}

/** Removes the prices of a school that start on `validFrom`. */
export async function deleteCarePriceList(
  schoolId: string,
  validFrom: string,
): Promise<CareTariffs> {
  return tariffsFromBackend(
    await traegerRequest<BackendTariffs>(
      `${TARIFFS_PATH}/${encodeURIComponent(schoolId)}/${encodeURIComponent(validFrom)}`,
      { method: "DELETE" },
    ),
  );
}

export async function fetchCareRuns(): Promise<CareRunList> {
  const body = await traegerRequest<{
    runs: BackendRun[] | null;
    suggested_month: string;
  }>(RUNS_PATH);
  return {
    runs: (body.runs ?? []).map(runFromBackend),
    suggestedMonth: body.suggested_month,
  };
}

export async function startCareRun(billingMonth: string): Promise<CareRun> {
  return runFromBackend(
    await traegerRequest<BackendRun>(RUNS_PATH, {
      method: "POST",
      body: { billing_month: billingMonth },
    }),
  );
}

export async function fetchCareRun(runId: string): Promise<CareRunDetail> {
  return detailFromBackend(
    await traegerRequest<BackendRunDetail>(
      `${RUNS_PATH}/${encodeURIComponent(runId)}`,
    ),
  );
}

/** Releases the preview the Abrechnung checked: its count and sum. */
export async function releaseCareRun(
  run: Pick<CareRunDetail, "id" | "previewCount" | "previewCents">,
): Promise<CareRunDetail> {
  return detailFromBackend(
    await traegerRequest<BackendRunDetail>(
      `${RUNS_PATH}/${encodeURIComponent(run.id)}/release`,
      {
        method: "POST",
        body: {
          document_count: run.previewCount,
          total_cents: run.previewCents,
        },
      },
    ),
  );
}
