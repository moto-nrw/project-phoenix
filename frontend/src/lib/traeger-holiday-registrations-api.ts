import {
  holidayAddressRequirement,
  holidayLegalBlocks,
  type HolidayLegalBlockWire,
} from "~/lib/holiday-form-field";
/**
 * Client of the review of the holiday registrations in the Träger portal
 * (#4663): the list of a holiday period with its filters, one registration
 * with its history, the changes of the Träger-Büro and the export as CSV or
 * Excel. Talks to the BFF routes under
 * /api/traeger/holiday-periods/{id}/registrations. Backend int64 IDs leave
 * here as strings.
 */

import {
  type BackendDebtorNumberFormat,
  type DebtorNumberFormat,
  debtorNumberFormatOf,
} from "~/lib/debtor-number-format";
import { apiErrorFromBody, transportFetch } from "./api-error";
import { filenameFromDisposition } from "./file-download";
import {
  type BackendConsentTexts,
  consentTextsFromBackend,
  type HolidayConsentTexts,
} from "./holiday-consent-texts";
import type {
  HolidayFormField,
  HolidayFormVersion,
} from "./holiday-form-field";
import { traegerRequest } from "./traeger-api";

export type HolidayRegistrationStatus = "received" | "confirmed" | "cancelled";

/**
 * The changes the Träger-Büro makes with one action. A confirmation freezes
 * the contribution and a cancellation computes the sibling group again, so
 * both go through the Elternabrechnung (#4664, #5662,
 * `confirmHolidayRegistration` and `cancelHolidayRegistration` in
 * traeger-contributions-api).
 */
export type HolidayRegistrationAction = "freeze" | "reopen";

type HolidayRegistrationEventAction =
  | HolidayRegistrationAction
  | "confirm"
  | "cancel"
  | "rebook"
  | "debtor_number"
  | "edit"
  | "proof_upload"
  | "sibling_merge"
  | "sibling_split"
  | "sibling_cancel"
  | "promote"
  | "wish"
  | "allocate"
  | "criteria_check";

/** The filter of the list and the export. Empty values match everything. */
export interface HolidayRegistrationFilter {
  readonly status: HolidayRegistrationStatus | "";
  /** A school ID, "guest" for holiday guests, or empty for all. */
  readonly school: string;
  readonly weekId: string;
  /** A time model name; time models are per week. */
  readonly timeModel: string;
  /**
   * The holiday care wish (#4703): "check" for children the office still
   * has to check, "yes" or "no", empty for all.
   */
  readonly wish: HolidayWishFilter;
}

export type HolidayWishFilter = "" | "check" | "yes" | "no";

export const EMPTY_HOLIDAY_REGISTRATION_FILTER: HolidayRegistrationFilter = {
  status: "",
  school: "",
  weekId: "",
  timeModel: "",
  wish: "",
};

/** What is known about a child's holiday care wish (#4703). */
type HolidayWish = "yes" | "no" | "";
/** Where the known wish comes from: the parent's account or the office. */
type HolidayWishSource = "account" | "office" | "";

/**
 * Whether a booking has a place in its week or waits for one (#4699). In a
 * period by criteria a booking stays "pending" until the office decides
 * (#4700); it is neither billed nor reported to the OGS.
 */
type HolidayPlacement = "placed" | "waitlisted" | "pending";

interface HolidayRegistrationBooking {
  readonly weekId: string;
  /** Calendar dates "YYYY-MM-DD". */
  readonly weekStartsOn: string;
  readonly weekEndsOn: string;
  readonly hostSchoolId: string;
  readonly timeModelId: string;
  readonly timeModelName: string;
  readonly placement: HolidayPlacement;
  /** Place on the waiting list of the week, from 1; null with a place. */
  readonly waitlistPosition: number | null;
  /** When the booking moved up from the waiting list; null otherwise. */
  readonly promotedAt: string | null;
  /** The booked days of a week by days (#4701); empty for a whole week. */
  readonly days?: readonly string[];
}

export interface HolidayRegistrationChild {
  readonly id: string;
  readonly firstName: string;
  readonly lastName: string;
  readonly birthDate: string;
  /** Null for a holiday guest. */
  readonly homeSchoolId: string | null;
  readonly guestSchoolName: string;
  readonly bookings: readonly HolidayRegistrationBooking[];
  /** Other registrations of the period with the same child. */
  readonly duplicateRegistrationIds: readonly string[];
  /** The child's holiday care wish, empty when unknown (#4703). */
  readonly holidayWish: HolidayWish;
  readonly holidayWishSource: HolidayWishSource;
  /** The office still has to check the wish of this child. */
  readonly holidayWishCheck: boolean;
}

/** Which mail of a registration to the parents (#6737). */
export type HolidayRegistrationMailKind =
  "holiday_registration_received" | "holiday_registration_accepted";

/**
 * What became of a mail that did not reach the parents (#6737): moto gave it
 * up or it came back, the address is blocked, or the outcome is unknown.
 */
export type HolidayRegistrationMailState = "failed" | "blocked" | "unknown";

export interface HolidayRegistrationMailProblem {
  readonly kind: HolidayRegistrationMailKind;
  readonly state: HolidayRegistrationMailState;
}

export interface HolidayRegistrationSummary {
  readonly id: string;
  readonly status: HolidayRegistrationStatus;
  readonly frozen: boolean;
  readonly submittedAt: string;
  readonly guardianFirstName: string;
  readonly guardianLastName: string;
  readonly guardianEmail: string;
  readonly guardianPhone: string;
  /** The parents state that they receive benefits (#4664). */
  readonly benefitsDeclared: boolean;
  /**
   * The family's number in the finance system of the Träger, entered at the
   * confirmation (#5621); empty without one.
   */
  readonly debtorNumber: string;
  /**
   * The day moto deletes the registration and the day its health information
   * goes (#6691), as "YYYY-MM-DD"; null for a period without weeks.
   */
  readonly deletesOn: string | null;
  readonly healthDeletesOn: string | null;
  /** Receipt and acceptance mails that did not reach the parents (#6737). */
  readonly mailProblems: readonly HolidayRegistrationMailProblem[];
  readonly children: readonly HolidayRegistrationChild[];
}

export interface HolidayRegistrationList {
  readonly registrations: readonly HolidayRegistrationSummary[];
  /** Registrations of the whole period, whatever the filter. */
  readonly total: number;
  readonly statusCounts: Readonly<Record<HolidayRegistrationStatus, number>>;
}

/** The Träger right that opens the health information (#4722). */
export const TRAEGER_HEALTH_RIGHT = "traeger_health:read";

/** The health information of one child of a registration (#4722). */
export interface HolidayRegistrationHealth {
  readonly childId: string;
  readonly allergies: string;
  readonly medication: string;
  readonly healthNotes: string;
  /** Answers to the child questions marked as health information. */
  readonly answers: Readonly<Record<string, unknown>>;
}

/**
 * The health information of one registration, read apart from the detail
 * and only with the right „Gesundheitsangaben sehen“ (#4722).
 */
export interface HolidayRegistrationHealthDetail {
  /** Answers to the family questions marked as health information. */
  readonly answers: Readonly<Record<string, unknown>>;
  readonly children: readonly HolidayRegistrationHealth[];
}

interface BackendHealthDetail {
  custom_data?: Record<string, unknown> | null;
  children?:
    | {
        child_id: number;
        allergies: string;
        medication: string;
        health_notes: string;
        custom_data?: Record<string, unknown> | null;
      }[]
    | null;
}

export interface HolidayRegistrationContact {
  readonly kind: "emergency" | "pickup";
  readonly name: string;
  readonly phone: string;
  readonly relationship: string;
}

export interface HolidayBookingChange {
  readonly weekId: string;
  readonly timeModelId: string;
  /** The days of a week by days (#4701). */
  readonly days?: readonly string[];
}

export interface HolidayRegistrationEvent {
  readonly action: HolidayRegistrationEventAction;
  readonly occurredAt: string;
  /** Status before a confirmation or cancellation. */
  readonly from: HolidayRegistrationStatus | "";
  /** The rebooked child and its bookings before and after. */
  readonly childId: string;
  readonly before: readonly HolidayBookingChange[];
  readonly after: readonly HolidayBookingChange[];
  /**
   * The debtor number before and after, when a confirmation entered it or a
   * change changed it (#5621); null otherwise.
   */
  readonly debtorNumber: {
    readonly before: string;
    readonly after: string;
  } | null;
  /** The fields an edit changed, as paths like "guardian.phone" (#5629). */
  readonly fields: readonly string[];
  /** The other registrations of a sibling merge or split (#5628). */
  readonly siblings: readonly string[];
  /** The office's decision on the holiday care wish (#4703). */
  readonly wish: HolidayWish;
  /** The weeks an allocation decided (#4700). */
  readonly weekIds?: readonly string[];
  /** What an allocation decided; absent for other actions. */
  readonly placement?: "placed" | "waitlisted";
  /** Why the office differed from the proposal; empty otherwise. */
  readonly reason?: string;
  /** The checks of the criteria the office changed (#4700). */
  readonly checks?: readonly HolidayCriterionCheck[];
}

/** "yes"/"no" as the office checked it, null for not checked. */
export type HolidayCriterionCheckValue = "yes" | "no" | null;

/** One check of a criterion by the office (#4700). */
export interface HolidayCriterionCheck {
  readonly criterionId: string;
  /** Null for a criterion of the family. */
  readonly childId: string | null;
  readonly checked: HolidayCriterionCheckValue;
}

/**
 * The answer to one criterion in a registration (#4700): what the parents
 * said and what the office checked. The office's check counts first.
 */
export interface HolidayRegistrationCriterion {
  readonly criterionId: string;
  readonly name: string;
  readonly question: string;
  readonly scope: "family" | "child";
  /** Null for a criterion of the family. */
  readonly childId: string | null;
  readonly stated: "yes" | "no" | "unknown";
  readonly checked: HolidayCriterionCheckValue;
  readonly checkedAt: string | null;
}

/** A proof of benefits the parents uploaded with the registration (#5238). */
interface HolidayRegistrationProof {
  readonly id: string;
  readonly filename: string;
  readonly sizeBytes: number;
  readonly contentType: string;
  readonly uploadedAt: string;
}

export interface HolidayRegistrationDetail extends HolidayRegistrationSummary {
  /**
   * Postal address of the parent, when the form asked for it (#5625). Only
   * the detail and the export name it, the list does not.
   */
  readonly guardianAddress: {
    readonly street: string;
    readonly postalCode: string;
    readonly city: string;
  } | null;
  readonly secondGuardian: {
    readonly firstName: string;
    readonly lastName: string;
    readonly email: string;
    readonly phone: string;
  } | null;
  readonly contacts: readonly HolidayRegistrationContact[];
  readonly privacyConsentAt: string;
  readonly healthConsentAt: string | null;
  readonly bindingConfirmedAt: string | null;
  /**
   * The declaration (#5626): when the parent confirmed the custody and the
   * name typed to give the declarations; a registration before has neither.
   */
  readonly custodyConfirmedAt: string | null;
  readonly declarationName: string;
  /** The version of the consent texts; 0 before #4722. */
  readonly consentVersion: number;
  /** The German texts of that version; null when they are not kept. */
  readonly consentTexts: HolidayConsentTexts | null;
  /** Only the detail names the proofs; list and export never do. */
  readonly benefitsProofs: readonly HolidayRegistrationProof[];
  readonly history: readonly HolidayRegistrationEvent[];
  /**
   * The form version the parents answered (#5198); its labels name the
   * answers, also after a later change of the form.
   */
  readonly legalAcceptances?: Readonly<Record<string, string>>;
  readonly form: HolidayFormVersion;
  /** Answers to the family questions, by key. */
  readonly answers: Readonly<Record<string, unknown>>;
  /** Answers to the questions per child, by child ID and key. */
  readonly childAnswers: Readonly<
    Record<string, Readonly<Record<string, unknown>>>
  >;
  /** Only in a period by criteria (#4700); empty otherwise. */
  readonly criteria?: readonly HolidayRegistrationCriterion[];
}

interface BackendCheck {
  criterion_id: number;
  child_id: number | null;
  checked: HolidayCriterionCheckValue;
}

interface BackendBooking {
  week_id: number;
  week_starts_on: string;
  week_ends_on: string;
  host_school_id: number;
  time_model_id: number;
  time_model_name: string;
  placement?: HolidayPlacement;
  waitlist_position?: number | null;
  promoted_at?: string | null;
  days?: string[] | null;
}

interface BackendChild {
  id: number;
  first_name: string;
  last_name: string;
  birth_date: string;
  home_school_id: number | null;
  guest_school_name: string;
  bookings?: BackendBooking[] | null;
  duplicate_registration_ids?: number[] | null;
  holiday_wish?: HolidayWish;
  holiday_wish_source?: HolidayWishSource;
  holiday_wish_check?: boolean;
}

interface BackendSummary {
  id: number;
  status: HolidayRegistrationStatus;
  frozen: boolean;
  submitted_at: string;
  guardian_first_name: string;
  guardian_last_name: string;
  guardian_email: string;
  guardian_phone: string;
  benefits_declared?: boolean;
  debtor_number?: string;
  deletes_on?: string | null;
  health_deletes_on?: string | null;
  mail_problems?: HolidayRegistrationMailProblem[] | null;
  children?: BackendChild[] | null;
}

interface BackendList {
  registrations?: BackendSummary[] | null;
  total: number;
  status_counts?: Partial<Record<HolidayRegistrationStatus, number>> | null;
}

interface BackendBookingChange {
  week_id: number;
  time_model_id: number;
  days?: string[] | null;
}

interface BackendDetail extends BackendSummary {
  guardian_address?: {
    street: string;
    postal_code: string;
    city: string;
  } | null;
  second_guardian?: {
    first_name: string;
    last_name: string;
    email: string;
    phone: string;
  } | null;
  contacts?: HolidayRegistrationContact[] | null;
  privacy_consent_at: string;
  health_consent_at: string | null;
  binding_confirmed_at: string | null;
  custody_confirmed_at?: string | null;
  declaration_name?: string | null;
  consent_version?: number | null;
  consent_texts?: BackendConsentTexts | null;
  benefits_proofs?:
    | {
        id: number;
        filename: string;
        size_bytes: number;
        content_type: string;
        uploaded_at: string;
      }[]
    | null;
  history?:
    | {
        action: HolidayRegistrationEventAction;
        occurred_at: string;
        details?: {
          from?: HolidayRegistrationStatus;
          child_id?: number;
          before?: BackendBookingChange[] | null;
          after?: BackendBookingChange[] | null;
          debtor_number?: { before: string; after: string } | null;
          fields?: string[] | null;
          siblings?: number[] | null;
          wish?: HolidayWish;
          week_ids?: number[] | null;
          placement?: "placed" | "waitlisted" | null;
          reason?: string | null;
          checks?: BackendCheck[] | null;
        } | null;
      }[]
    | null;
  legal_acceptances?: Record<string, string> | null;
  form?: {
    version: number;
    fields?: HolidayFormField[] | null;
    address_requirement?: string | null;
    legal_blocks?: HolidayLegalBlockWire[] | null;
  } | null;
  custom_data?: Record<string, unknown> | null;
  child_custom_data?:
    { child_id: number; custom_data?: Record<string, unknown> | null }[] | null;
  criteria?:
    | {
        criterion_id: number;
        name: string;
        question: string;
        scope: "family" | "child";
        child_id: number | null;
        stated: "yes" | "no" | "unknown";
        checked: HolidayCriterionCheckValue;
        checked_at: string | null;
      }[]
    | null;
}

function checksFromBackend(
  checks: BackendCheck[] | null | undefined,
): HolidayCriterionCheck[] {
  return (checks ?? []).map((check) => ({
    criterionId: String(check.criterion_id),
    childId: check.child_id === null ? null : String(check.child_id),
    checked: check.checked,
  }));
}

function summaryFromBackend(
  registration: BackendSummary,
): HolidayRegistrationSummary {
  return {
    id: String(registration.id),
    status: registration.status,
    frozen: registration.frozen,
    submittedAt: registration.submitted_at,
    guardianFirstName: registration.guardian_first_name,
    guardianLastName: registration.guardian_last_name,
    guardianEmail: registration.guardian_email,
    guardianPhone: registration.guardian_phone,
    benefitsDeclared: registration.benefits_declared ?? false,
    debtorNumber: registration.debtor_number ?? "",
    deletesOn: registration.deletes_on ?? null,
    healthDeletesOn: registration.health_deletes_on ?? null,
    mailProblems: (registration.mail_problems ?? []).map((problem) => ({
      kind: problem.kind,
      state: problem.state,
    })),
    children: (registration.children ?? []).map((child) => ({
      id: String(child.id),
      firstName: child.first_name,
      lastName: child.last_name,
      birthDate: child.birth_date,
      homeSchoolId:
        child.home_school_id === null ? null : String(child.home_school_id),
      guestSchoolName: child.guest_school_name,
      bookings: (child.bookings ?? []).map((booking) => ({
        weekId: String(booking.week_id),
        weekStartsOn: booking.week_starts_on,
        weekEndsOn: booking.week_ends_on,
        hostSchoolId: String(booking.host_school_id),
        timeModelId: String(booking.time_model_id),
        timeModelName: booking.time_model_name,
        placement: booking.placement ?? "placed",
        waitlistPosition: booking.waitlist_position ?? null,
        promotedAt: booking.promoted_at ?? null,
        ...(booking.days?.length ? { days: booking.days } : {}),
      })),
      duplicateRegistrationIds: (child.duplicate_registration_ids ?? []).map(
        String,
      ),
      holidayWish: child.holiday_wish ?? "",
      holidayWishSource: child.holiday_wish_source ?? "",
      holidayWishCheck: child.holiday_wish_check ?? false,
    })),
  };
}

function changesFromBackend(
  changes: BackendBookingChange[] | null | undefined,
): HolidayBookingChange[] {
  return (changes ?? []).map((change) => ({
    weekId: String(change.week_id),
    timeModelId: String(change.time_model_id),
    ...(change.days?.length ? { days: change.days } : {}),
  }));
}

function detailFromBackend(
  registration: BackendDetail,
): HolidayRegistrationDetail {
  return {
    ...summaryFromBackend(registration),
    guardianAddress: registration.guardian_address
      ? {
          street: registration.guardian_address.street,
          postalCode: registration.guardian_address.postal_code,
          city: registration.guardian_address.city,
        }
      : null,
    secondGuardian: registration.second_guardian
      ? {
          firstName: registration.second_guardian.first_name,
          lastName: registration.second_guardian.last_name,
          email: registration.second_guardian.email,
          phone: registration.second_guardian.phone,
        }
      : null,
    contacts: registration.contacts ?? [],
    privacyConsentAt: registration.privacy_consent_at,
    healthConsentAt: registration.health_consent_at,
    bindingConfirmedAt: registration.binding_confirmed_at,
    custodyConfirmedAt: registration.custody_confirmed_at ?? null,
    declarationName: registration.declaration_name ?? "",
    consentVersion: registration.consent_version ?? 0,
    consentTexts: registration.consent_texts
      ? consentTextsFromBackend(registration.consent_texts)
      : null,
    benefitsProofs: (registration.benefits_proofs ?? []).map((proof) => ({
      id: String(proof.id),
      filename: proof.filename,
      sizeBytes: proof.size_bytes,
      contentType: proof.content_type,
      uploadedAt: proof.uploaded_at,
    })),
    history: (registration.history ?? []).map((event) => ({
      action: event.action,
      occurredAt: event.occurred_at,
      from: event.details?.from ?? "",
      childId: event.details?.child_id ? String(event.details.child_id) : "",
      before: changesFromBackend(event.details?.before),
      after: changesFromBackend(event.details?.after),
      debtorNumber: event.details?.debtor_number ?? null,
      fields: event.details?.fields ?? [],
      siblings: (event.details?.siblings ?? []).map(String),
      wish: event.details?.wish ?? "",
      // Only an allocation and a check carry these (#4700).
      ...(event.details?.week_ids
        ? { weekIds: event.details.week_ids.map(String) }
        : {}),
      ...(event.details?.placement
        ? { placement: event.details.placement }
        : {}),
      ...(event.details?.reason ? { reason: event.details.reason } : {}),
      ...(event.details?.checks
        ? { checks: checksFromBackend(event.details.checks) }
        : {}),
    })),
    legalAcceptances: registration.legal_acceptances ?? {},
    form: {
      legalBlocks: holidayLegalBlocks(registration.form?.legal_blocks),
      version: registration.form?.version ?? 0,
      fields: registration.form?.fields ?? [],
      addressRequirement: holidayAddressRequirement(
        registration.form?.address_requirement,
      ),
    },
    answers: registration.custom_data ?? {},
    childAnswers: Object.fromEntries(
      (registration.child_custom_data ?? []).map((entry) => [
        String(entry.child_id),
        entry.custom_data ?? {},
      ]),
    ),
    criteria: (registration.criteria ?? []).map((criterion) => ({
      criterionId: String(criterion.criterion_id),
      name: criterion.name,
      question: criterion.question,
      scope: criterion.scope,
      childId: criterion.child_id === null ? null : String(criterion.child_id),
      stated: criterion.stated,
      checked: criterion.checked,
      checkedAt: criterion.checked_at,
    })),
  };
}

const registrationsPath = (periodId: string) =>
  `/api/traeger/holiday-periods/${encodeURIComponent(periodId)}/registrations`;

const registrationPath = (periodId: string, registrationId: string) =>
  `${registrationsPath(periodId)}/${encodeURIComponent(registrationId)}`;

/** Address of the proxy that opens one proof of benefits (#5238). */
export function holidayProofHref(
  periodId: string,
  registrationId: string,
  proofId: string,
): string {
  return `${registrationPath(periodId, registrationId)}/proofs/${encodeURIComponent(proofId)}`;
}

/**
 * Uploads a proof of benefits the family brought on paper (#5627) and
 * returns the registration with its proofs. The backend checks type, size
 * and content like the parents' upload.
 */
export async function uploadHolidayProof(
  periodId: string,
  registrationId: string,
  file: File,
): Promise<HolidayRegistrationDetail> {
  const body = new FormData();
  body.append("file", file);
  const response = await transportFetch(
    `${registrationPath(periodId, registrationId)}/proofs`,
    { method: "POST", body, credentials: "include" },
  );
  const payload = (await response.json().catch(() => null)) as {
    data?: BackendDetail;
  } | null;
  if (!response.ok || !payload?.data) {
    throw apiErrorFromBody(
      `Request failed (${response.status})`,
      response.status,
      payload,
    );
  }
  return detailFromBackend(payload.data);
}

/** The query string of a filter, empty values left out. */
function holidayRegistrationQuery(
  filter: HolidayRegistrationFilter,
  extra: Record<string, string> = {},
): string {
  const params = new URLSearchParams();
  if (filter.status) params.set("status", filter.status);
  if (filter.school) params.set("school", filter.school);
  if (filter.weekId) params.set("week_id", filter.weekId);
  if (filter.timeModel) params.set("time_model", filter.timeModel);
  if (filter.wish) params.set("wish", filter.wish);
  for (const [key, value] of Object.entries(extra)) params.set(key, value);
  const query = params.toString();
  return query ? `?${query}` : "";
}

/** The registrations of a period that match the filter. */
export async function fetchHolidayRegistrations(
  periodId: string,
  filter: HolidayRegistrationFilter,
): Promise<HolidayRegistrationList> {
  const list = await traegerRequest<BackendList>(
    `${registrationsPath(periodId)}${holidayRegistrationQuery(filter)}`,
  );
  return {
    registrations: (list.registrations ?? []).map(summaryFromBackend),
    total: list.total,
    statusCounts: {
      received: list.status_counts?.received ?? 0,
      confirmed: list.status_counts?.confirmed ?? 0,
      cancelled: list.status_counts?.cancelled ?? 0,
    },
  };
}

/** One registration with contacts and history, without health information. */
export async function fetchHolidayRegistration(
  periodId: string,
  registrationId: string,
): Promise<HolidayRegistrationDetail> {
  return detailFromBackend(
    await traegerRequest<BackendDetail>(
      registrationPath(periodId, registrationId),
    ),
  );
}

/**
 * The registered children a school of the Träger knows (#6683): child ID to
 * the one school that manages a child with the same names and date of birth
 * today. Only a hint; moto links nothing. The backend records every
 * comparison, so a page asks only for what it shows: the whole period, or
 * one registration.
 */
export async function fetchHolidayKnownChildren(
  periodId: string,
  registrationId?: string,
): Promise<ReadonlyMap<string, string>> {
  const query = registrationId
    ? `?registration_id=${encodeURIComponent(registrationId)}`
    : "";
  const known = await traegerRequest<{
    children?: { child_id: number; school_id: number }[] | null;
  } | null>(
    `/api/traeger/holiday-periods/${encodeURIComponent(periodId)}/registrations/known-children${query}`,
  );
  return new Map(
    (known?.children ?? []).map((child) => [
      String(child.child_id),
      String(child.school_id),
    ]),
  );
}

/**
 * The health information of a registration (#4722). The backend answers 403
 * without the right „Gesundheitsangaben sehen“ and records every read.
 */
export async function fetchHolidayRegistrationHealth(
  periodId: string,
  registrationId: string,
): Promise<HolidayRegistrationHealthDetail> {
  const health = await traegerRequest<BackendHealthDetail>(
    `${registrationPath(periodId, registrationId)}/health`,
  );
  return {
    answers: health.custom_data ?? {},
    children: (health.children ?? []).map((child) => ({
      childId: String(child.child_id),
      allergies: child.allergies,
      medication: child.medication,
      healthNotes: child.health_notes,
      answers: child.custom_data ?? {},
    })),
  };
}

/**
 * The detail with the health answers merged into its answers, so the
 * questions marked as health information show their answer beside the
 * others (#4722).
 */
export function withHolidayHealthAnswers(
  registration: HolidayRegistrationDetail,
  health: HolidayRegistrationHealthDetail | null,
): HolidayRegistrationDetail {
  if (!health) return registration;
  const childAnswers: Record<string, Readonly<Record<string, unknown>>> = {
    ...registration.childAnswers,
  };
  for (const child of health.children) {
    childAnswers[child.childId] = {
      ...childAnswers[child.childId],
      ...child.answers,
    };
  }
  return {
    ...registration,
    answers: { ...registration.answers, ...health.answers },
    childAnswers,
  };
}

/** Freezes or reopens a registration. */
export async function changeHolidayRegistration(
  periodId: string,
  registrationId: string,
  action: HolidayRegistrationAction,
): Promise<HolidayRegistrationDetail> {
  return detailFromBackend(
    await traegerRequest<BackendDetail>(
      `${registrationPath(periodId, registrationId)}/${action}`,
      { method: "POST", body: {} },
    ),
  );
}

/**
 * Saves the office's checks of the criteria (#4700). A check changes the
 * proposal for open bookings and the order of the waiting list.
 */
export async function checkHolidayRegistrationCriteria(
  periodId: string,
  registrationId: string,
  checks: readonly HolidayCriterionCheck[],
): Promise<HolidayRegistrationDetail> {
  return detailFromBackend(
    await traegerRequest<BackendDetail>(
      `${registrationPath(periodId, registrationId)}/criteria`,
      {
        method: "PUT",
        body: {
          checks: checks.map((check) => ({
            criterion_id: Number(check.criterionId),
            child_id: check.childId === null ? null : Number(check.childId),
            checked: check.checked,
          })),
        },
      },
    ),
  );
}

/**
 * The office decides the holiday care wish of a child of its schools
 * (#4703). It only orders the waiting list; no place is given or taken.
 */
export async function setHolidayChildWish(
  periodId: string,
  registrationId: string,
  childId: string,
  wish: "yes" | "no",
): Promise<HolidayRegistrationDetail> {
  return detailFromBackend(
    await traegerRequest<BackendDetail>(
      `${registrationPath(periodId, registrationId)}/children/${encodeURIComponent(childId)}/holiday-wish`,
      { method: "PUT", body: { wish } },
    ),
  );
}

/** Replaces the bookings of one child: one time model per week. */
export async function rebookHolidayChild(
  periodId: string,
  registrationId: string,
  childId: string,
  bookings: readonly HolidayBookingChange[],
): Promise<HolidayRegistrationDetail> {
  return detailFromBackend(
    await traegerRequest<BackendDetail>(
      `${registrationPath(periodId, registrationId)}/children/${encodeURIComponent(childId)}/bookings`,
      {
        method: "PUT",
        body: {
          bookings: bookings.map((booking) => ({
            week_id: Number(booking.weekId),
            time_model_id: Number(booking.timeModelId),
            ...(booking.days && booking.days.length > 0
              ? { days: booking.days }
              : {}),
          })),
        },
      },
    ),
  );
}

/** A person of the registration as the Träger-Büro edits it (#5629). */
export interface HolidayRegistrationPersonEdit {
  readonly firstName: string;
  readonly lastName: string;
  readonly email: string;
  readonly phone: string;
}

/** The postal address of the parent who registered (#5625). */
interface HolidayRegistrationAddressEdit {
  readonly street: string;
  readonly postalCode: string;
  readonly city: string;
}

/** An emergency contact or a person allowed to collect the children. */
export interface HolidayRegistrationContactEdit {
  readonly name: string;
  readonly phone: string;
  readonly relationship: string;
}

/** The details of one child; a null school makes it a holiday guest. */
interface HolidayRegistrationChildEdit {
  readonly id: string;
  readonly firstName: string;
  readonly lastName: string;
  readonly birthDate: string;
  readonly homeSchoolId: string | null;
  readonly guestSchoolName: string;
  /** Its answers to the questions without health information. */
  readonly answers: Readonly<Record<string, unknown>>;
}

/**
 * One change of the details (#5629). A section that is absent stays as it
 * is; a null `secondGuardian` removes the second guardian.
 */
export interface HolidayRegistrationEdit {
  readonly guardians?: {
    readonly guardian: HolidayRegistrationPersonEdit;
    /** Null removes the address; the form version decides whether it is asked. */
    readonly address: HolidayRegistrationAddressEdit | null;
    readonly secondGuardian: HolidayRegistrationPersonEdit | null;
  };
  readonly child?: HolidayRegistrationChildEdit;
  readonly contacts?: {
    readonly emergency: readonly HolidayRegistrationContactEdit[];
    readonly pickup: readonly HolidayRegistrationContactEdit[];
  };
  /** The family's answers to the questions without health information. */
  readonly answers?: Readonly<Record<string, unknown>>;
}

function personBody(person: HolidayRegistrationPersonEdit) {
  return {
    first_name: person.firstName,
    last_name: person.lastName,
    email: person.email,
    phone: person.phone,
  };
}

/** The request body; the field names are those of the public form. */
function holidayRegistrationEditBody(
  edit: HolidayRegistrationEdit,
): Record<string, unknown> {
  const body: Record<string, unknown> = {};
  if (edit.guardians) {
    body.guardian = personBody(edit.guardians.guardian);
    const address = edit.guardians.address;
    body.guardian_address = address
      ? {
          street: address.street,
          postal_code: address.postalCode,
          city: address.city,
        }
      : null;
    body.second_guardian = edit.guardians.secondGuardian
      ? personBody(edit.guardians.secondGuardian)
      : null;
  }
  if (edit.child) {
    body.children = [
      {
        id: Number(edit.child.id),
        first_name: edit.child.firstName,
        last_name: edit.child.lastName,
        birth_date: edit.child.birthDate,
        home_school_id: edit.child.homeSchoolId
          ? Number(edit.child.homeSchoolId)
          : 0,
        guest_school_name: edit.child.guestSchoolName,
        custom_data: edit.child.answers,
      },
    ];
  }
  if (edit.contacts) {
    body.emergency_contacts = edit.contacts.emergency;
    body.pickup_persons = edit.contacts.pickup;
  }
  if (edit.answers) body.custom_data = edit.answers;
  return body;
}

/** Changes the details a family entered (#5629). */
export async function editHolidayRegistration(
  periodId: string,
  registrationId: string,
  edit: HolidayRegistrationEdit,
): Promise<HolidayRegistrationDetail> {
  return detailFromBackend(
    await traegerRequest<BackendDetail>(
      `${registrationPath(periodId, registrationId)}/details`,
      { method: "PUT", body: holidayRegistrationEditBody(edit) },
    ),
  );
}

interface DownloadedFile {
  readonly blob: Blob;
  readonly filename: string | null;
}

/** Fetches a file of the Träger portal with the backend's file name. */
async function downloadTraegerFile(path: string): Promise<DownloadedFile> {
  const response = await transportFetch(path, {
    credentials: "include",
    cache: "no-store",
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
    filename: filenameFromDisposition(response),
  };
}

/** The export of a filter as CSV or Excel, with the backend's file name. */
export async function downloadHolidayRegistrations(
  periodId: string,
  filter: HolidayRegistrationFilter,
  format: "csv" | "xlsx",
): Promise<DownloadedFile> {
  return downloadTraegerFile(
    `${registrationsPath(periodId)}/export${holidayRegistrationQuery(filter, { format })}`,
  );
}

/** The query that asks for the health information in a file (#6691). */
function healthQuery(withHealth: boolean): Record<string, string> {
  return withHealth ? { health: "true" } : {};
}

/**
 * One registration as a PDF (#6691). With `withHealth` the file names the
 * health information; the backend admits that only with the right to see it.
 */
export async function downloadHolidayRegistrationPdf(
  periodId: string,
  registrationId: string,
  withHealth: boolean,
): Promise<DownloadedFile> {
  return downloadTraegerFile(
    `${registrationPath(periodId, registrationId)}/pdf${holidayRegistrationQuery(EMPTY_HOLIDAY_REGISTRATION_FILTER, healthQuery(withHealth))}`,
  );
}

/** The registrations of a filter as a ZIP with one PDF each (#6691). */
export async function downloadHolidayRegistrationPdfs(
  periodId: string,
  filter: HolidayRegistrationFilter,
  withHealth: boolean,
): Promise<DownloadedFile> {
  return downloadTraegerFile(
    `${registrationsPath(periodId)}/pdf${holidayRegistrationQuery(filter, healthQuery(withHealth))}`,
  );
}

/**
 * What a confirmation of the Träger needs (#5621). It lives here, not in the
 * contributions client, because the list reads it for its column too and
 * should not load that client.
 */
/** An active cost bearer the confirmation offers (#5627). */
export interface CostBearerChoice {
  readonly id: string;
  readonly name: string;
}

export interface HolidayConfirmationSettings {
  /** The Träger asks for the debtor number of the family. */
  readonly debtorNumberRequired: boolean;
  /** The format the number must have (#6675); none without the setting. */
  readonly debtorNumberFormat?: DebtorNumberFormat;
  /** The active cost bearers of the Träger, by name (#5627). */
  readonly costBearers?: readonly CostBearerChoice[];
}

export async function fetchHolidayConfirmationSettings(): Promise<HolidayConfirmationSettings> {
  const settings = await traegerRequest<{
    debtor_number_required: boolean;
    debtor_number_format?: BackendDebtorNumberFormat | null;
    cost_bearers?: { id: number; name: string }[] | null;
  }>("/api/traeger/holiday-contributions/confirmation-settings");
  return {
    debtorNumberRequired: settings.debtor_number_required,
    debtorNumberFormat: debtorNumberFormatOf(settings.debtor_number_format),
    costBearers: (settings.cost_bearers ?? []).map((bearer) => ({
      id: String(bearer.id),
      name: bearer.name,
    })),
  };
}

/** One booking on the waiting list of a week, in its order (#4699). */
interface HolidayWaitlistEntry {
  readonly position: number;
  readonly registrationId: string;
  readonly registrationStatus: HolidayRegistrationStatus;
  readonly submittedAt: string;
  readonly childId: string;
  readonly childFirstName: string;
  readonly childLastName: string;
  readonly timeModelName: string;
  /** Waits behind the others: no holiday care wish (#4703). */
  readonly deferred: boolean;
  /** The wanted days of a booking of days (#4701); empty for a whole week. */
  readonly days?: readonly string[];
}

/**
 * The places of one week of a holiday period (#4699): its capacity, null for
 * unlimited, the children with a place and the waiting list.
 */
export interface HolidayWeekPlaces {
  readonly weekId: string;
  /** Calendar dates "YYYY-MM-DD". */
  readonly startsOn: string;
  readonly endsOn: string;
  readonly hostSchoolId: string;
  readonly capacity: number | null;
  readonly placed: number;
  /** Bookings the office has not decided yet (#4700). */
  readonly pending?: number;
  readonly waitlist: readonly HolidayWaitlistEntry[];
  /** "day": the capacity holds for each day (#4701). */
  readonly bookingUnit?: "week" | "day";
  /** Placed bookings of each care day of a week by days, in order. */
  readonly days?: readonly { readonly date: string; readonly placed: number }[];
}

interface BackendWeekPlaces {
  week_id: number;
  starts_on: string;
  ends_on: string;
  host_school_id: number;
  capacity: number | null;
  placed: number;
  pending?: number;
  booking_unit?: "week" | "day";
  days?: { date: string; placed: number }[] | null;
  waitlist?:
    | {
        position: number;
        registration_id: number;
        registration_status: HolidayRegistrationStatus;
        submitted_at: string;
        child_id: number;
        child_first_name: string;
        child_last_name: string;
        time_model_name: string;
        deferred?: boolean;
        days?: string[] | null;
      }[]
    | null;
}

/** The places and waiting lists of every week of a period (#4699). */
export async function fetchHolidayWeekPlaces(
  periodId: string,
): Promise<readonly HolidayWeekPlaces[]> {
  const weeks = await traegerRequest<BackendWeekPlaces[] | null>(
    `/api/traeger/holiday-periods/${encodeURIComponent(periodId)}/places`,
  );
  return (weeks ?? []).map((week) => ({
    weekId: String(week.week_id),
    startsOn: week.starts_on,
    endsOn: week.ends_on,
    hostSchoolId: String(week.host_school_id),
    capacity: week.capacity,
    placed: week.placed,
    pending: week.pending ?? 0,
    bookingUnit: week.booking_unit ?? "week",
    days: week.days ?? [],
    waitlist: (week.waitlist ?? []).map((entry) => ({
      position: entry.position,
      registrationId: String(entry.registration_id),
      registrationStatus: entry.registration_status,
      submittedAt: entry.submitted_at,
      childId: String(entry.child_id),
      childFirstName: entry.child_first_name,
      childLastName: entry.child_last_name,
      timeModelName: entry.time_model_name,
      deferred: entry.deferred ?? false,
      ...(entry.days?.length ? { days: entry.days } : {}),
    })),
  }));
}

/**
 * Sets the places of a week, null for unlimited (#4699). Freed places go to
 * the waiting list at once; the answer names the registrations that moved
 * up.
 */
export async function setHolidayWeekCapacity(
  periodId: string,
  weekId: string,
  capacity: number | null,
): Promise<{ readonly movedUp: readonly string[] }> {
  const change = await traegerRequest<{ moved_up?: number[] | null }>(
    `/api/traeger/holiday-periods/${encodeURIComponent(periodId)}/weeks/${encodeURIComponent(weekId)}/capacity`,
    { method: "PUT", body: { capacity } },
  );
  return { movedUp: (change.moved_up ?? []).map(String) };
}
