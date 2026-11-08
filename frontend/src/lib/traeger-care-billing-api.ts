/**
 * Client of the monthly contribution of the care contract (#4684): the
 * price list of the Träger (a monthly price per booked weekday, module and
 * school) and the monthly runs that bill a month by the contract in force.
 * Money stays whole cent. Backend int64 IDs leave here as strings; calendar
 * dates stay "YYYY-MM-DD".
 */

import {
  type BackendMealChanged,
  type BackendMealGap,
  type BackendMealPosition,
  type BackendMealUnbilled,
  type CareMealChanged,
  type CareMealGap,
  type CareMealPosition,
  type CareMealUnbilled,
  mealChangedFromBackend,
  mealGapsFromBackend,
  mealPositionsFromBackend,
  mealUnbilledFromBackend,
} from "./traeger-care-meal-api";
import type {
  MailOutcome,
  MailProviderBlock,
} from "~/lib/email-delivery-status";
import { traegerRequest } from "./traeger-api";
import {
  type BackendInvoicePayment,
  type BackendRunDueDate,
  type InvoicePayment,
  type PostalAddress,
  type RecipientKind,
  type RunDueDate,
  invoicePaymentFromBackend,
  runDueDatesFromBackend,
} from "./traeger-billing-terms-api";
import {
  type BillingFile,
  type LetterMailStatus,
  billingFile,
} from "./traeger-billing-api";

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
  readonly bookedDaysPerWeek: readonly number[];
  readonly contractCount: number;
}

/**
 * How a module is priced (#6196): a price per booked weekday (the sum of
 * the booked days) or a scale by the number of booked days per week.
 */
export type CarePriceForm = "weekday" | "days_per_week";

/**
 * The monthly price of one weekday of a module from a month on, or with
 * `daysPerWeek` the price for that many booked days per week. `siblingCents`
 * is the price for every further child; null keeps the first child's.
 */
interface CareTariff {
  readonly id: string;
  readonly schoolId: string;
  readonly careOfferingId: string;
  readonly moduleName: string;
  readonly weekday: CareWeekday | "";
  readonly daysPerWeek?: number | null;
  readonly validFrom: string;
  readonly validUntil: string | null;
  readonly monthlyCents: number;
  readonly siblingCents?: number | null;
  readonly mealExtra?: boolean;
}

/** The head of a price list: the months it bills nothing for (1–12). */
interface CarePriceListHead {
  readonly schoolId: string;
  readonly validFrom: string;
  readonly freeMonths: readonly number[];
}

export interface CareSchool {
  readonly id: string;
  readonly name: string;
}

export interface CareTariffs {
  readonly schools: readonly CareSchool[];
  readonly modules: readonly CareModule[];
  readonly tariffs: readonly CareTariff[];
  readonly lists?: readonly CarePriceListHead[];
  /** Per school ID the last month an issued invoice bills ("YYYY-MM-01"). */
  readonly billedUntil: Readonly<Record<string, string>>;
}

export interface CarePriceInput {
  readonly careOfferingId: string;
  readonly moduleName: string;
  /** Set for a price per weekday … */
  readonly weekday?: CareWeekday;
  /** … or for a scale step. */
  readonly daysPerWeek?: number;
  readonly monthlyCents: number;
  readonly siblingCents: number | null;
  readonly mealExtra: boolean;
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
  /** The schools the run bills (#5916). */
  readonly schoolIds: readonly string[];
  /** The invoice date of the run; null takes the day of the release. */
  readonly invoiceDate: string | null;
}

export interface CareRunList {
  readonly runs: readonly CareRun[];
  /** The first month from the current one on without a released run. */
  readonly suggestedMonth: string;
}

interface CareWeekdayPrice {
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
  /** The scale step a module priced by days per week billed (#6196). */
  readonly daysPerWeek?: number | null;
  readonly tierMonthlyCents?: number | null;
  /** The price excludes the meal, which the Mensa bills. */
  readonly mealExtra?: boolean;
  readonly amountCents: number;
  /** The amount a removal deducts after reductions and cost shares. */
  readonly payableCents?: number;
  /** Names the position for a change of the review (#6197). */
  readonly key: string;
}

export type CareAmendmentKind =
  "amount" | "add_position" | "remove_position" | "recipient" | "address";

/** Why a change of the review is not applied (#6197). */
type CareAmendmentConflict =
  "basis_changed" | "position_missing" | "negative_total";

/** One side of a change: a total, a position, a recipient or an address. */
interface CareAmendmentValue {
  readonly totalCents: number | null;
  readonly label: string;
  readonly amountCents: number | null;
  readonly recipient: {
    readonly firstName: string;
    readonly lastName: string;
    readonly secondName: string;
    readonly email: string;
  } | null;
  readonly address: PostalAddress | null;
}

/**
 * A change of an invoice in the review (#6197): on a preview its history,
 * also revoked and conflicting entries; on an issued invoice what its
 * release applied. `amountCents` is the correction line it adds.
 */
export interface CareAmendment {
  readonly id: string;
  readonly kind: CareAmendmentKind;
  readonly label: string;
  readonly amountCents: number;
  readonly positionKey: string;
  readonly before: CareAmendmentValue;
  readonly after: CareAmendmentValue;
  readonly reason: string;
  readonly amendedAt: string;
  /** Taken over from the cancelled invoice with this number. */
  readonly carriedFromNumber: string | null;
  readonly revocation: {
    readonly reason: string;
    readonly revokedAt: string;
  } | null;
  readonly conflict: CareAmendmentConflict | null;
}

/** Whether a change shapes the invoice. */
export function amendmentApplied(amendment: CareAmendment): boolean {
  return amendment.revocation === null && amendment.conflict === null;
}

/** The correction lines of the applied changes; recipient and address add none. */
export function careCorrectionLines(
  invoice: CareInvoice,
): readonly CareAmendment[] {
  return (invoice.amendments ?? []).filter(
    (amendment) =>
      amendmentApplied(amendment) &&
      amendment.kind !== "recipient" &&
      amendment.kind !== "address",
  );
}

/** Which changes of an invoice the Träger allows in the review (#6197). */
interface CareAmendable {
  readonly amount: boolean;
  readonly recipient: boolean;
  readonly address: boolean;
  readonly positions: boolean;
}

export interface CareInvoice {
  /** Empty for a draft. */
  readonly id: string;
  readonly number: string;
  readonly issuedOn: string | null;
  readonly contractId: string;
  /** What the invoice bills: the care with the Mensa bundled, or the Mensa on an invoice of its own (#4686). */
  readonly service: "care" | "meal";
  /** Original contract-party address, independent of the payer. */
  readonly familyEmail?: string;
  readonly schoolId: string;
  readonly schoolName: string;
  readonly childFirstName: string;
  readonly childLastName: string;
  readonly recipient: {
    readonly firstName: string;
    readonly lastName: string;
    readonly email: string;
    /** Who gets it (#4689); "" on an invoice issued before. */
    readonly kind?: RecipientKind | "";
    /** The second parent when both parents get it. */
    readonly secondName?: string;
    /** The payer's address when another person pays. */
    readonly address?: PostalAddress | null;
  };
  /** How and by when it is paid (#4689); null on an invoice issued before. */
  readonly payment?: InvoicePayment | null;
  readonly positions: readonly CarePosition[];
  /** The rules of the chain in their order (#4685); negative amounts. */
  readonly reductions: readonly CareReduction[];
  /** The Mensa months of the child (#4686); no rule reduces them. */
  readonly mealPositions: readonly CareMealPosition[];
  /**
   * What cost bearers pay of the contribution after the reductions (#4687);
   * `totalCents` is the rest the family pays.
   */
  readonly costShares: readonly CareInvoiceCostShare[];
  /** The changes of the review (#6197); `totalCents` adds their lines. */
  readonly amendments?: readonly CareAmendment[];
  readonly totalCents: number;
  /** Set on a preview invoice the release leaves out (#5916). */
  readonly exclusion?: CareExclusion | null;
}

/** Why the release leaves an invoice of the preview out (#5916). */
interface CareExclusion {
  readonly reason: string;
  readonly excludedAt: string;
}

/**
 * One reduction of an invoice with a copy of its rule (#4685). A sibling
 * reduction names the child's rank among the children its rule counted.
 */
export interface CareReduction {
  readonly kind: "income_level" | "sibling" | "single_parent" | "cap";
  readonly name: string;
  readonly childRank: number | null;
  readonly familyChildren: number | null;
  readonly discountBasisPoints: number | null;
  readonly discountCents: number | null;
  readonly priceCents: number | null;
  readonly capCents: number | null;
  /** The sibling prices of the price list (#6196). */
  readonly tariffSiblingPrice?: boolean;
  readonly amountCents: number;
}

export type CareCostShareKind =
  "education_participation" | "youth_office" | "other_subsidy";

/** What a share covers (#4686): the modules, the Mensa or both. */
export type CareCostShareScope = "care" | "meal" | "both";

/** The part one cost bearer pays of an invoice, a positive amount (#4687). */
interface CareInvoiceCostShare {
  readonly kind: CareCostShareKind;
  readonly costBearerId: string;
  readonly costBearerName: string;
  readonly shareBasisPoints: number | null;
  readonly shareCents: number | null;
  readonly scope: CareCostShareScope;
  readonly amountCents: number;
}

/** What one cost bearer pays of the invoices of a run (#4687). */
export interface CareCostBearerTotal {
  readonly costBearerId: string;
  readonly costBearerName: string;
  readonly invoiceCount: number;
  readonly amountCents: number;
}

/** The contribution before the cost bearer shares. */
export function careContributionCents(invoice: CareInvoice): number {
  // The correction lines of the review come after the shares (#6197).
  const corrections = careCorrectionLines(invoice).reduce(
    (sum, line) => sum + line.amountCents,
    0,
  );
  return invoice.costShares.reduce(
    (sum, share) => sum + share.amountCents,
    invoice.totalCents - corrections,
  );
}

export interface CareUnpriced {
  readonly contractId: string;
  readonly schoolId: string;
  readonly schoolName: string;
  readonly childFirstName: string;
  readonly childLastName: string;
  readonly careOfferingId: string;
  readonly moduleName: string;
  /** Empty for a missing scale step. */
  readonly weekday: CareWeekday | "";
  /** The missing scale step of a module priced by days per week. */
  readonly daysPerWeek?: number | null;
}

/**
 * A child the run holds back for a duplicate contract (ADR 0158, Nachtrag
 * Kind-Monat): in a preview one entry per module and weekday billed twice,
 * naming every contract of the child; in a released run one entry per
 * child of the contracts its release held back, without a module.
 */
export interface CareDuplicate {
  readonly childFirstName: string;
  readonly childLastName: string;
  readonly moduleName: string;
  readonly weekday: CareWeekday;
  readonly contracts: readonly {
    readonly contractId: string;
    readonly schoolId: string;
    readonly schoolName: string;
  }[];
}

export interface CareRunDetail extends CareRun {
  readonly invoices: readonly CareInvoice[];
  readonly unpriced: readonly CareUnpriced[];
  readonly duplicates: readonly CareDuplicate[];
  /** The children of `duplicates`; the release bills none of them. */
  readonly duplicateChildren: number;
  /**
   * Families a sibling rule that counts the family ranks without a
   * confirmed number for the month (#5918); it counts their children with a
   * contract alone.
   */
  readonly uncountedFamilies?: number;
  readonly billed: number;
  /** Contracts a free month of their price list bills nothing for. */
  readonly freeOfCharge?: number;
  readonly previewCount: number;
  readonly previewCents: number;
  readonly numberPattern: string;
  readonly numberRangeSet: boolean;
  /** Ended months a school has not released for the Mensa (#4686). */
  readonly mealGaps: readonly CareMealGap[];
  /** Released Mensa months of children the run does not bill (#4686). */
  readonly mealUnbilled: readonly CareMealUnbilled[];
  /** Mensa months issued invoices bill with an amount a later release changed (#4686). */
  readonly mealChanged: readonly CareMealChanged[];
  /** The due dates of the run per payment method (#4689). */
  readonly dueDates?: readonly RunDueDate[];
  /** The review of a calculated run (#5916). */
  readonly schools: readonly CareSchool[];
  readonly excludedCount: number;
  readonly excludedCents: number;
  /** The reviewed state; the release sends it back. */
  readonly fingerprint: string;
  /** Which changes of an invoice the Träger allows (#6197). */
  readonly amendable?: CareAmendable;
  /** Changes in conflict on invoices the release issues; it refuses while any is open. */
  readonly amendmentConflicts?: number;
  /** The kind of document the run issues (#4688). */
  readonly documentKind?: string;
  /** A release would refuse: the fee notice has no legal remedy (#4688). */
  readonly legalRemedyMissing?: boolean;
  /** What each cost bearer pays of the invoices (#4687). */
  readonly costBearers: readonly CareCostBearerTotal[];
  /**
   * Children whose Jahresbescheid an amendment set to 0,00 € for the month:
   * the month stays locked, only a new notice bills it (ADR 0180, #6219).
   */
  readonly feeNoticeHeld?: readonly CareFeeNoticeHold[];
}

export interface CareFeeNoticeHold {
  readonly contractId: string;
  readonly childFirstName: string;
  readonly childLastName: string;
  readonly noticeNumber: string;
}

interface BackendModule {
  school_id: number;
  school_name: string;
  care_offering_id: number;
  module_name: string;
  ends_at: string;
  booked_weekdays: CareWeekday[] | null;
  booked_days_per_week?: number[] | null;
  contract_count: number;
}

interface BackendTariff {
  id: number;
  school_id: number;
  care_offering_id: number;
  module_name: string;
  weekday: CareWeekday | "";
  days_per_week?: number | null;
  valid_from: string;
  valid_until: string | null;
  monthly_cents: number;
  sibling_cents?: number | null;
  meal_extra?: boolean;
}

interface BackendTariffs {
  schools: { id: number; name: string }[] | null;
  modules: BackendModule[] | null;
  tariffs: BackendTariff[] | null;
  lists?:
    | { school_id: number; valid_from: string; free_months: number[] | null }[]
    | null;
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
  school_ids?: number[] | null;
  invoice_date?: string | null;
}

interface BackendInvoice {
  id: number;
  number: string;
  issued_on: string | null;
  contract_id: number;
  service?: string;
  family_email?: string;
  school_id: number;
  school_name: string;
  child_first_name: string;
  child_last_name: string;
  recipient: {
    first_name: string;
    last_name: string;
    email: string;
    kind?: RecipientKind | "";
    second_name?: string;
    address?: { street: string; postal_code: string; city: string } | null;
  };
  payment?: BackendInvoicePayment | null;
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
        days_per_week?: number | null;
        tier_monthly_cents?: number | null;
        meal_extra?: boolean;
        amount_cents: number;
        payable_cents?: number;
        key?: string;
      }[]
    | null;
  reductions:
    | {
        kind: CareReduction["kind"];
        name: string;
        child_rank: number | null;
        family_children: number | null;
        discount_basis_points: number | null;
        discount_cents: number | null;
        price_cents: number | null;
        cap_cents: number | null;
        tariff_sibling_price?: boolean;
        amount_cents: number;
      }[]
    | null;
  meal_positions?: BackendMealPosition[] | null;
  cost_shares?:
    | {
        kind: CareCostShareKind;
        cost_bearer_id: number;
        cost_bearer_name: string;
        share_basis_points: number | null;
        share_cents: number | null;
        scope?: CareCostShareScope;
        amount_cents: number;
      }[]
    | null;
  amendments?: BackendAmendment[] | null;
  total_cents: number;
  exclusion?: { reason: string; excluded_at: string } | null;
}

interface BackendAmendmentValue {
  total_cents?: number;
  label?: string;
  amount_cents?: number;
  recipient?: {
    first_name: string;
    last_name: string;
    second_name: string;
    email: string;
  };
  address?: { street: string; postal_code: string; city: string };
}

interface BackendAmendment {
  id: number;
  kind: CareAmendmentKind;
  label: string;
  amount_cents: number;
  position_key?: string;
  before?: BackendAmendmentValue;
  after?: BackendAmendmentValue;
  reason: string;
  amended_at: string;
  carried?: boolean;
  carried_from_number?: string;
  revocation?: { reason: string; revoked_at: string } | null;
  conflict?: CareAmendmentConflict | "";
}

function amendmentValue(
  value: BackendAmendmentValue | undefined,
): CareAmendmentValue {
  return {
    totalCents: value?.total_cents ?? null,
    label: value?.label ?? "",
    amountCents: value?.amount_cents ?? null,
    recipient: value?.recipient
      ? {
          firstName: value.recipient.first_name,
          lastName: value.recipient.last_name,
          secondName: value.recipient.second_name,
          email: value.recipient.email,
        }
      : null,
    address: value?.address
      ? {
          street: value.address.street,
          postalCode: value.address.postal_code,
          city: value.address.city,
        }
      : null,
  };
}

function amendmentFromBackend(amendment: BackendAmendment): CareAmendment {
  return {
    id: String(amendment.id),
    kind: amendment.kind,
    label: amendment.label,
    amountCents: amendment.amount_cents,
    positionKey: amendment.position_key ?? "",
    before: amendmentValue(amendment.before),
    after: amendmentValue(amendment.after),
    reason: amendment.reason,
    amendedAt: amendment.amended_at,
    carriedFromNumber: amendment.carried
      ? (amendment.carried_from_number ?? "")
      : null,
    revocation: amendment.revocation
      ? {
          reason: amendment.revocation.reason,
          revokedAt: amendment.revocation.revoked_at,
        }
      : null,
    conflict: amendment.conflict ? amendment.conflict : null,
  };
}

export interface BackendRunDetail extends BackendRun {
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
        weekday: CareWeekday | "";
        days_per_week?: number | null;
      }[]
    | null;
  duplicates:
    | {
        child_first_name: string;
        child_last_name: string;
        module_name: string;
        weekday: CareWeekday;
        contracts:
          | { contract_id: number; school_id: number; school_name: string }[]
          | null;
      }[]
    | null;
  duplicate_children: number;
  uncounted_families?: number;
  billed: number;
  free_of_charge?: number;
  preview_count: number;
  preview_cents: number;
  number_pattern: string;
  number_range_set: boolean;
  meal_gaps?: BackendMealGap[] | null;
  meal_unbilled?: BackendMealUnbilled[] | null;
  meal_changed?: BackendMealChanged[] | null;
  due_dates?: BackendRunDueDate[] | null;
  schools?: { id: number; name: string }[] | null;
  excluded_count?: number;
  excluded_cents?: number;
  fingerprint?: string;
  amendable?: {
    amount: boolean;
    recipient: boolean;
    address: boolean;
    positions: boolean;
  };
  amendment_conflicts?: number;
  document_kind?: string;
  legal_remedy_missing?: boolean;
  cost_bearers?:
    | {
        cost_bearer_id: number;
        cost_bearer_name: string;
        invoice_count: number;
        amount_cents: number;
      }[]
    | null;
  fee_notice_held?:
    | {
        contract_id: number;
        child_first_name: string;
        child_last_name: string;
        notice_number: string;
      }[]
    | null;
}

const TARIFFS_PATH = "/api/traeger/care-tariffs";
const RUNS_PATH = "/api/traeger/care-billing-runs";

function id(value: number): string {
  return value ? String(value) : "";
}

function tariffsFromBackend(body: BackendTariffs): CareTariffs {
  return {
    schools: (body.schools ?? []).map((school) => ({
      id: id(school.id),
      name: school.name,
    })),
    modules: (body.modules ?? []).map((module) => ({
      schoolId: id(module.school_id),
      schoolName: module.school_name,
      careOfferingId: id(module.care_offering_id),
      moduleName: module.module_name,
      endsAt: module.ends_at,
      bookedWeekdays: module.booked_weekdays ?? [],
      bookedDaysPerWeek: module.booked_days_per_week ?? [],
      contractCount: module.contract_count,
    })),
    tariffs: (body.tariffs ?? []).map((tariff) => ({
      id: id(tariff.id),
      schoolId: id(tariff.school_id),
      careOfferingId: id(tariff.care_offering_id),
      moduleName: tariff.module_name,
      weekday: tariff.weekday,
      daysPerWeek: tariff.days_per_week ?? null,
      validFrom: tariff.valid_from,
      validUntil: tariff.valid_until,
      monthlyCents: tariff.monthly_cents,
      siblingCents: tariff.sibling_cents ?? null,
      mealExtra: tariff.meal_extra ?? false,
    })),
    lists: (body.lists ?? []).map((list) => ({
      schoolId: id(list.school_id),
      validFrom: list.valid_from,
      freeMonths: list.free_months ?? [],
    })),
    billedUntil: { ...body.billed_until },
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
    schoolIds: (run.school_ids ?? []).map(id),
    invoiceDate: run.invoice_date ?? null,
  };
}

function invoiceFromBackend(invoice: BackendInvoice): CareInvoice {
  return {
    id: id(invoice.id),
    number: invoice.number,
    issuedOn: invoice.issued_on,
    contractId: id(invoice.contract_id),
    service: invoice.service === "meal" ? "meal" : "care",
    familyEmail: invoice.family_email ?? "",
    schoolId: id(invoice.school_id),
    schoolName: invoice.school_name,
    childFirstName: invoice.child_first_name,
    childLastName: invoice.child_last_name,
    recipient: {
      firstName: invoice.recipient.first_name,
      lastName: invoice.recipient.last_name,
      email: invoice.recipient.email,
      kind: invoice.recipient.kind ?? "",
      secondName: invoice.recipient.second_name ?? "",
      address: invoice.recipient.address
        ? {
            street: invoice.recipient.address.street,
            postalCode: invoice.recipient.address.postal_code,
            city: invoice.recipient.address.city,
          }
        : null,
    },
    payment: invoicePaymentFromBackend(invoice.payment),
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
      daysPerWeek: position.days_per_week ?? null,
      tierMonthlyCents: position.tier_monthly_cents ?? null,
      mealExtra: position.meal_extra ?? false,
      amountCents: position.amount_cents,
      payableCents: position.payable_cents,
      key: position.key ?? "",
    })),
    reductions: (invoice.reductions ?? []).map((reduction) => ({
      kind: reduction.kind,
      name: reduction.name,
      childRank: reduction.child_rank,
      familyChildren: reduction.family_children,
      discountBasisPoints: reduction.discount_basis_points,
      discountCents: reduction.discount_cents,
      priceCents: reduction.price_cents,
      capCents: reduction.cap_cents,
      tariffSiblingPrice: reduction.tariff_sibling_price ?? false,
      amountCents: reduction.amount_cents,
    })),
    mealPositions: mealPositionsFromBackend(invoice.meal_positions),
    costShares: (invoice.cost_shares ?? []).map((share) => ({
      kind: share.kind,
      costBearerId: id(share.cost_bearer_id),
      costBearerName: share.cost_bearer_name,
      shareBasisPoints: share.share_basis_points,
      shareCents: share.share_cents,
      scope: share.scope ?? "care",
      amountCents: share.amount_cents,
    })),
    amendments: (invoice.amendments ?? []).map(amendmentFromBackend),
    totalCents: invoice.total_cents,
    exclusion: invoice.exclusion
      ? {
          reason: invoice.exclusion.reason,
          excludedAt: invoice.exclusion.excluded_at,
        }
      : null,
  };
}

export function detailFromBackend(detail: BackendRunDetail): CareRunDetail {
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
      daysPerWeek: entry.days_per_week ?? null,
    })),
    duplicates: (detail.duplicates ?? []).map((entry) => ({
      childFirstName: entry.child_first_name,
      childLastName: entry.child_last_name,
      moduleName: entry.module_name,
      weekday: entry.weekday,
      contracts: (entry.contracts ?? []).map((contract) => ({
        contractId: id(contract.contract_id),
        schoolId: id(contract.school_id),
        schoolName: contract.school_name,
      })),
    })),
    duplicateChildren: detail.duplicate_children ?? 0,
    uncountedFamilies: detail.uncounted_families ?? 0,
    billed: detail.billed,
    freeOfCharge: detail.free_of_charge ?? 0,
    previewCount: detail.preview_count,
    previewCents: detail.preview_cents,
    numberPattern: detail.number_pattern,
    numberRangeSet: detail.number_range_set,
    mealGaps: mealGapsFromBackend(detail.meal_gaps),
    mealUnbilled: mealUnbilledFromBackend(detail.meal_unbilled),
    mealChanged: mealChangedFromBackend(detail.meal_changed),
    dueDates: runDueDatesFromBackend(detail.due_dates),
    schools: (detail.schools ?? []).map((school) => ({
      id: id(school.id),
      name: school.name,
    })),
    excludedCount: detail.excluded_count ?? 0,
    excludedCents: detail.excluded_cents ?? 0,
    fingerprint: detail.fingerprint ?? "",
    amendable: {
      amount: detail.amendable?.amount ?? false,
      recipient: detail.amendable?.recipient ?? false,
      address: detail.amendable?.address ?? false,
      positions: detail.amendable?.positions ?? false,
    },
    amendmentConflicts: detail.amendment_conflicts ?? 0,
    documentKind: detail.document_kind,
    legalRemedyMissing: detail.legal_remedy_missing ?? false,
    costBearers: (detail.cost_bearers ?? []).map((total) => ({
      costBearerId: id(total.cost_bearer_id),
      costBearerName: total.cost_bearer_name,
      invoiceCount: total.invoice_count,
      amountCents: total.amount_cents,
    })),
    feeNoticeHeld: (detail.fee_notice_held ?? []).map((hold) => ({
      contractId: id(hold.contract_id),
      childFirstName: hold.child_first_name,
      childLastName: hold.child_last_name,
      noticeNumber: hold.notice_number,
    })),
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
  freeMonths: readonly number[] = [],
): Promise<CareTariffs> {
  return tariffsFromBackend(
    await traegerRequest<BackendTariffs>(TARIFFS_PATH, {
      method: "PUT",
      body: {
        school_id: Number(schoolId),
        valid_from: validFrom,
        free_months: freeMonths,
        prices: prices.map((price) => ({
          care_offering_id: Number(price.careOfferingId),
          module_name: price.moduleName,
          weekday: price.weekday ?? "",
          days_per_week: price.daysPerWeek ?? null,
          monthly_cents: price.monthlyCents,
          sibling_cents: price.siblingCents,
          meal_extra: price.mealExtra,
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

/** What a new run bills (#5916): month, schools and invoice date. */
export interface CareRunStartInput {
  readonly billingMonth: string;
  /** Empty: every school no open run of the month bills. */
  readonly schoolIds: readonly string[];
  /** Null takes the day of the release. */
  readonly invoiceDate: string | null;
}

export async function startCareRun(input: CareRunStartInput): Promise<CareRun> {
  return runFromBackend(
    await traegerRequest<BackendRun>(RUNS_PATH, {
      method: "POST",
      body: {
        billing_month: input.billingMonth,
        school_ids: input.schoolIds.map(Number),
        invoice_date: input.invoiceDate,
      },
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

/**
 * Releases the preview the Abrechnung checked: its count, sum and
 * fingerprint. A preview that changed since is refused (#5916).
 */
export async function releaseCareRun(
  run: Pick<
    CareRunDetail,
    "id" | "previewCount" | "previewCents" | "fingerprint"
  >,
): Promise<CareRunDetail> {
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

// The letters of a released monthly run (#4684): one letter per invoice and
// per Storno of a sent invoice, the mail of each letter, the print and the
// Storno itself.

/** The letter of an invoice or of its Storno. */
export type CareLetterKind = "invoice" | "cancellation";

/** What the Abrechnung wrote into every letter of a run. */
interface CareLetterSet {
  readonly paymentDueOn: string;
  readonly notes: string;
  readonly createdAt: string;
}

export interface CareLetterInput {
  readonly paymentDueOn: string;
  readonly notes: string;
}

/**
 * One invoice or Storno of a run with its letter and its last mail. A
 * Storno follows its invoice directly. `hasLetter` is false before the
 * letters exist and for the Storno of an invoice that never went out.
 */
export interface CareLetterMailing {
  readonly invoiceId: string;
  readonly kind: CareLetterKind;
  /** The number of this document: the invoice or the Storno. */
  readonly number: string;
  readonly invoiceNumber: string;
  readonly issuedOn: string;
  readonly childFirstName: string;
  readonly childLastName: string;
  readonly schoolName: string;
  readonly recipientFirstName: string;
  readonly recipientLastName: string;
  readonly recipientEmail: string;
  /** Negative for a Storno. */
  readonly totalCents: number;
  readonly reason: string;
  /** The invoice has a Storno. */
  readonly cancelled: boolean;
  readonly cancellationNumber: string;
  readonly hasLetter: boolean;
  /**
   * The sent letter also lands in the parents inbox. False when the child has
   * no record yet: the family gets it by mail or on paper only.
   */
  readonly inInbox: boolean;
  /** A paper copy was downloaded through the print action. */
  readonly printed?: boolean;
  /** A later released invoice replaces this cancelled invoice. */
  readonly replaced?: boolean;
  readonly status: LetterMailStatus;
  readonly sends: number;
  readonly attempts: number;
  readonly requestedAt: string | null;
  readonly sentAt: string | null;
  readonly mailOutcome?: MailOutcome;
  readonly providerBlock?: MailProviderBlock;
}

/**
 * The letters of a run by the state of their last mail. `notSent` includes
 * `withoutEmail`, the letters of families without an address.
 */
export interface CareLetterSummary {
  readonly letters: number;
  readonly sent: number;
  readonly queued: number;
  readonly failed: number;
  readonly notSent: number;
  readonly withoutEmail: number;
  /** Letters that reach the family by mail or paper only (no child record). */
  readonly withoutInbox: number;
}

export interface CareLetters {
  readonly letters: CareLetterSet | null;
  readonly documents: readonly CareLetterMailing[];
  readonly summary: CareLetterSummary;
}

/** What a send to every family did. */
export interface CareLetterSeries {
  readonly queued: number;
  readonly already: number;
  readonly withoutEmail: number;
  /** Letters held back: the address is no longer a party of the contract. */
  readonly recipientChanged: number;
}

interface BackendCareLetterMailing {
  invoice_id: number;
  kind: CareLetterKind;
  number: string;
  invoice_number: string;
  issued_on: string;
  child_first_name: string;
  child_last_name: string;
  school_name: string;
  recipient_first_name: string;
  recipient_last_name: string;
  recipient_email: string;
  total_cents: number;
  reason?: string;
  cancelled: boolean;
  cancellation_number?: string;
  has_letter: boolean;
  in_inbox?: boolean;
  printed?: boolean;
  replaced?: boolean;
  status: LetterMailStatus;
  sends: number;
  attempts: number;
  requested_at: string | null;
  sent_at: string | null;
  mail_outcome?: MailOutcome;
  provider_block?: MailProviderBlock;
}

interface BackendCareLetters {
  letters: {
    payment_due_on: string;
    notes: string;
    created_at: string;
  } | null;
  documents: BackendCareLetterMailing[] | null;
  summary: {
    letters: number;
    sent: number;
    queued: number;
    failed: number;
    not_sent: number;
    without_email: number;
    without_inbox?: number;
  } | null;
}

function careMailingFromBackend(
  mailing: BackendCareLetterMailing,
): CareLetterMailing {
  return {
    invoiceId: id(mailing.invoice_id),
    kind: mailing.kind === "cancellation" ? "cancellation" : "invoice",
    number: mailing.number,
    invoiceNumber: mailing.invoice_number,
    issuedOn: mailing.issued_on,
    childFirstName: mailing.child_first_name,
    childLastName: mailing.child_last_name,
    schoolName: mailing.school_name,
    recipientFirstName: mailing.recipient_first_name,
    recipientLastName: mailing.recipient_last_name,
    recipientEmail: mailing.recipient_email,
    totalCents: mailing.total_cents,
    reason: mailing.reason ?? "",
    cancelled: mailing.cancelled,
    cancellationNumber: mailing.cancellation_number ?? "",
    hasLetter: mailing.has_letter,
    inInbox: mailing.in_inbox ?? false,
    printed: mailing.printed ?? false,
    replaced: mailing.replaced ?? false,
    status: mailing.status,
    sends: mailing.sends,
    attempts: mailing.attempts,
    requestedAt: mailing.requested_at,
    sentAt: mailing.sent_at,
    mailOutcome: mailing.mail_outcome || undefined,
    providerBlock: mailing.provider_block || undefined,
  };
}

function careLettersFromBackend(body: BackendCareLetters): CareLetters {
  const summary = body.summary;
  return {
    letters: body.letters
      ? {
          paymentDueOn: body.letters.payment_due_on,
          notes: body.letters.notes,
          createdAt: body.letters.created_at,
        }
      : null,
    documents: (body.documents ?? []).map(careMailingFromBackend),
    summary: {
      letters: summary?.letters ?? 0,
      sent: summary?.sent ?? 0,
      queued: summary?.queued ?? 0,
      failed: summary?.failed ?? 0,
      notSent: summary?.not_sent ?? 0,
      withoutEmail: summary?.without_email ?? 0,
      withoutInbox: summary?.without_inbox ?? 0,
    },
  };
}

function lettersPath(runId: string): string {
  return `${RUNS_PATH}/${encodeURIComponent(runId)}/letters`;
}

function invoicePath(runId: string, invoiceId: string): string {
  return `${RUNS_PATH}/${encodeURIComponent(runId)}/invoices/${encodeURIComponent(invoiceId)}`;
}

function careLetterBody(input: CareLetterInput) {
  return { payment_due_on: input.paymentDueOn, notes: input.notes };
}

/** The letters of a released run with the state of every invoice. */
export async function fetchCareLetters(runId: string): Promise<CareLetters> {
  return careLettersFromBackend(
    await traegerRequest<BackendCareLetters>(lettersPath(runId)),
  );
}

/** Creates the letter of every invoice once; families see nothing yet. */
export async function createCareLetters(
  runId: string,
  input: CareLetterInput,
): Promise<CareLetters> {
  return careLettersFromBackend(
    await traegerRequest<BackendCareLetters>(lettersPath(runId), {
      method: "POST",
      body: careLetterBody(input),
    }),
  );
}

/** The letter of the first invoice with the input, without storing it. */
export async function previewCareLetter(
  runId: string,
  input: CareLetterInput,
): Promise<BillingFile> {
  return billingFile(
    `${lettersPath(runId)}/preview`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(careLetterBody(input)),
    },
    "vorschau.pdf",
  );
}

/**
 * Every letter the run may still send in one PDF to print; with
 * `withoutEmailOnly` only those of families without an address.
 */
export async function printCareLetters(
  runId: string,
  withoutEmailOnly = false,
): Promise<BillingFile> {
  return billingFile(
    `${lettersPath(runId)}/print${withoutEmailOnly ? "?only=without_email" : ""}`,
    {},
    "monatsrechnungen.pdf",
  );
}

/** Mails every letter with an address and no mail yet; idempotent. */
export async function mailCareLetters(
  runId: string,
): Promise<CareLetterSeries> {
  const series = await traegerRequest<{
    queued: number;
    already: number;
    without_email: number;
    recipient_changed?: number;
  }>(`${lettersPath(runId)}/mail`, { method: "POST" });
  return {
    queued: series.queued,
    already: series.already,
    withoutEmail: series.without_email,
    recipientChanged: series.recipient_changed ?? 0,
  };
}

/** The stored letter of an invoice or of its Storno. */
export async function downloadCareLetter(
  runId: string,
  invoiceId: string,
  kind: CareLetterKind,
): Promise<BillingFile> {
  return billingFile(
    `${invoicePath(runId, invoiceId)}/letter${kind === "cancellation" ? "?kind=cancellation" : ""}`,
    {},
    kind === "cancellation" ? "storno.pdf" : "rechnung.pdf",
  );
}

/** Mails one letter, first or again. */
export async function mailCareLetter(
  runId: string,
  invoiceId: string,
  kind: CareLetterKind,
): Promise<CareLetterMailing> {
  return careMailingFromBackend(
    await traegerRequest<BackendCareLetterMailing>(
      `${invoicePath(runId, invoiceId)}/${kind === "cancellation" ? "cancellation/mail" : "mail"}`,
      { method: "POST" },
    ),
  );
}

/** Issues the Storno of an invoice with its reason. */
export async function cancelCareInvoice(
  runId: string,
  invoiceId: string,
  reason: string,
): Promise<CareLetters> {
  return careLettersFromBackend(
    await traegerRequest<BackendCareLetters>(
      `${invoicePath(runId, invoiceId)}/cancellation`,
      { method: "POST", body: { reason } },
    ),
  );
}
