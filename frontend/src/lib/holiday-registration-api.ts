import {
  holidayAddressRequirement,
  holidayLegalBlocks,
  type HolidayLegalBlockWire,
} from "~/lib/holiday-form-field";
// Öffentliche Ferienanmeldung (#4661): der eine Link eines Ferienzeitraums
// für alle Schulen des Trägers, ohne Elternkonto.

import { unavailableApiError, type ApiError } from "~/lib/api-error";
import type {
  HolidayFormField,
  HolidayFormVersion,
} from "~/lib/holiday-form-field";
import { readEnrollmentError } from "~/lib/enrollment-api-error";
import {
  type BackendContribution,
  contributionFromBackend,
  type HolidayContribution,
} from "~/lib/holiday-contribution";
import { type NewUploadValue, postUpload } from "~/lib/enrollment-uploads";
import {
  type BackendConsentTexts,
  consentTextsFromBackend,
  type HolidayConsentTexts,
} from "~/lib/holiday-consent-texts";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "HolidayRegistrationApi" });

export interface HolidayRegistrationTimeModel {
  readonly id: string;
  readonly name: string;
  readonly startsAt: string;
  readonly endsAt: string;
  /**
   * Price in cent: per week, or per day in a week by days (#4701); the base
   * line of the contribution (#5237).
   */
  readonly priceCents: number;
}

export interface HolidayRegistrationWeek {
  readonly id: string;
  readonly startsOn: string;
  readonly endsOn: string;
  readonly hostSchoolName: string;
  readonly timeModels: readonly HolidayRegistrationTimeModel[];
  /** "day": parents choose single days and prices are per day (#4701). */
  readonly bookingUnit?: "week" | "day";
  /**
   * The days a week by days offers, Monday to Friday without the days
   * without care, "YYYY-MM-DD".
   */
  readonly careDays?: readonly string[];
  /** Days of a week by days without a free place: a booking of them waits. */
  readonly fullDays?: readonly string[];
  /**
   * Days without care at the host school (#4702); a week booked as a whole
   * with such a day is not bookable, a week by days leaves the day out.
   */
  readonly noCareDays?: readonly {
    readonly date: string;
    readonly name: string;
  }[];
}

interface HolidayRegistrationSchool {
  readonly id: string;
  readonly name: string;
}

/**
 * A yes/no question of the Träger for the allocation of places (#4700),
 * asked once for the family or for every child. Parents see the question
 * only, never the answers of other families.
 */
interface HolidayRegistrationCriterion {
  readonly id: string;
  readonly question: string;
  readonly scope: "family" | "child";
}

/** The published holiday period behind a link. */
export interface HolidayRegistrationOffer {
  readonly traegerName: string;
  readonly name: string;
  readonly registrationOpensOn: string;
  readonly registrationClosesOn: string;
  readonly binding: boolean;
  readonly notes: string;
  readonly open: boolean;
  readonly weeks: readonly HolidayRegistrationWeek[];
  /** Care days one child may book over all weeks; null is no bound (#4701). */
  readonly maxDaysPerChild?: number | null;
  readonly schools: readonly HolidayRegistrationSchool[];
  readonly captchaSiteKey: string;
  /** The questions of the Träger in their current version (#5198). */
  readonly form: HolidayFormVersion;
  /** The German texts of the declarations the form shows (#5626). */
  readonly consentTexts: HolidayConsentTexts;
  /**
   * "criteria": the office gives the places after the Träger's criteria;
   * nobody gets a place at once (#4700). Absent reads as arrival.
   */
  readonly allocationMode?: "arrival" | "criteria";
  /** The active criteria, only by criteria; empty otherwise. */
  readonly criteria?: readonly HolidayRegistrationCriterion[];
}

interface BackendOffer {
  traeger_name: string;
  name: string;
  registration_opens_on: string;
  registration_closes_on: string;
  binding: boolean;
  notes: string;
  open: boolean;
  weeks?:
    | {
        id: number;
        starts_on: string;
        ends_on: string;
        host_school_name: string;
        no_care_days?: { date: string; name: string }[] | null;
        time_models?:
          | {
              id: number;
              name: string;
              starts_at: string;
              ends_at: string;
              price_cents: number;
            }[]
          | null;
        booking_unit?: "week" | "day";
        care_days?: string[] | null;
        full_days?: string[] | null;
      }[]
    | null;
  max_days_per_child?: number | null;
  schools?: { id: number; name: string }[] | null;
  captcha_site_key: string;
  form?: {
    version: number;
    fields?: HolidayFormField[] | null;
    legal_blocks?: HolidayLegalBlockWire[] | null;
    address_requirement?: string | null;
  } | null;
  consent_texts?: BackendConsentTexts | null;
  allocation_mode?: "arrival" | "criteria";
  criteria?:
    { id: number; question: string; scope: "family" | "child" }[] | null;
}

/** The registration as the backend reads it; IDs travel as numbers. */
export interface HolidayRegistrationPayload {
  readonly guardian: {
    readonly first_name: string;
    readonly last_name: string;
    readonly email: string;
    readonly phone: string;
  };
  readonly second_guardian?: HolidayRegistrationPayload["guardian"];
  /** Only when the form asks for it and the parent filled it in (#5625). */
  readonly guardian_address?: {
    readonly street: string;
    readonly postal_code: string;
    readonly city: string;
  };
  readonly children: readonly {
    readonly first_name: string;
    readonly last_name: string;
    readonly birth_date: string;
    readonly home_school_id: number;
    readonly guest_school_name: string;
    readonly allergies: string;
    readonly medication: string;
    readonly health_notes: string;
    readonly bookings: readonly {
      readonly week_id: number;
      readonly time_model_id: number;
      /** The chosen days of a week by days (#4701). */
      readonly days?: readonly string[];
    }[];
    /** Answers to the questions per child, by key (#5198). */
    readonly custom_data: Readonly<Record<string, unknown>>;
    /** Answers to the criteria per child, by criterion ID (#4700). */
    readonly criteria?: Readonly<Record<string, boolean>>;
  }[];
  readonly emergency_contacts: readonly HolidayContactPayload[];
  readonly pickup_persons: readonly HolidayContactPayload[];
  readonly privacy_consent: boolean;
  readonly health_consent: boolean;
  readonly binding_confirmed: boolean;
  /** The custody and the typed name of the declaration (#5626). */
  readonly custody_confirmed: boolean;
  readonly declaration_name: string;
  readonly benefits_declared: boolean;
  /** Tokens of the uploaded proofs of benefits (#5238). */
  readonly benefits_proof_tokens: readonly string[];
  readonly captcha_token: string;
  /** The form version the parent answered (#5198). */
  readonly legal_acceptances?: Readonly<Record<string, boolean>>;
  readonly form_version: number;
  /**
   * Version of the consent texts the parent agreed to (#4722): from 2 on the
   * hosting OGS reads the health information, 3 adds the declaration (#5626).
   */
  readonly consent_version: number;
  /** Answers to the questions per family, by key (#5198). */
  readonly custom_data: Readonly<Record<string, unknown>>;
  /** Answers to the criteria of the family, by criterion ID (#4700). */
  readonly criteria?: Readonly<Record<string, boolean>>;
}

export interface HolidayContactPayload {
  readonly name: string;
  readonly phone: string;
  readonly relationship: string;
}

function offerFromBackend(offer: BackendOffer): HolidayRegistrationOffer {
  return {
    traegerName: offer.traeger_name,
    name: offer.name,
    registrationOpensOn: offer.registration_opens_on,
    registrationClosesOn: offer.registration_closes_on,
    binding: offer.binding,
    notes: offer.notes,
    open: offer.open,
    weeks: (offer.weeks ?? []).map((week) => ({
      id: String(week.id),
      startsOn: week.starts_on,
      endsOn: week.ends_on,
      hostSchoolName: week.host_school_name,
      noCareDays: (week.no_care_days ?? []).map((day) => ({
        date: day.date,
        name: day.name,
      })),
      timeModels: (week.time_models ?? []).map((model) => ({
        id: String(model.id),
        name: model.name,
        startsAt: model.starts_at,
        endsAt: model.ends_at,
        priceCents: model.price_cents,
      })),
      bookingUnit: week.booking_unit ?? "week",
      careDays: week.care_days ?? [],
      fullDays: week.full_days ?? [],
    })),
    maxDaysPerChild: offer.max_days_per_child ?? null,
    schools: (offer.schools ?? []).map((school) => ({
      id: String(school.id),
      name: school.name,
    })),
    captchaSiteKey: offer.captcha_site_key,
    form: {
      legalBlocks: holidayLegalBlocks(offer.form?.legal_blocks),
      version: offer.form?.version ?? 0,
      fields: offer.form?.fields ?? [],
      addressRequirement: holidayAddressRequirement(
        offer.form?.address_requirement,
      ),
    },
    consentTexts: consentTextsFromBackend(offer.consent_texts),
    allocationMode: offer.allocation_mode ?? "arrival",
    criteria:
      offer.allocation_mode === "criteria"
        ? (offer.criteria ?? []).map((criterion) => ({
            id: String(criterion.id),
            question: criterion.question,
            scope: criterion.scope,
          }))
        : [],
  };
}

function registrationPath(token: string): string {
  return `/api/holiday-registrations/${encodeURIComponent(token)}`;
}

async function request(input: string, init?: RequestInit): Promise<Response> {
  try {
    return await fetch(input, init);
  } catch (cause) {
    throw unavailableApiError(cause);
  }
}

function readError(response: Response, fallback: string): Promise<ApiError> {
  return readEnrollmentError(
    response,
    fallback,
    logger,
    "holiday_registration_api_failed",
  );
}

export async function fetchHolidayRegistrationOffer(
  token: string,
): Promise<HolidayRegistrationOffer> {
  const response = await request(registrationPath(token), {
    cache: "no-store",
  });
  if (!response.ok) {
    throw await readError(
      response,
      "Ferienanmeldung konnte nicht geladen werden",
    );
  }
  const body = (await response.json()) as { data: BackendOffer };
  return offerFromBackend(body.data);
}

/**
 * Uploads one proof of benefits through the public link (#5238). Like the
 * uploads of the online registration, the answer's token is single-use and
 * claimed by the registration.
 */
export function uploadHolidayProof(
  token: string,
  file: File,
  signal?: AbortSignal,
): Promise<NewUploadValue> {
  const body = new FormData();
  body.append("file", file);
  return postUpload(`${registrationPath(token)}/uploads`, body, file, signal);
}

/**
 * Whether one booking of the sent registration got a place or waits on the
 * waiting list of its week (#4699), or waits for the office to give the
 * places (#4700). The child is named by its position in the form, from 1.
 */
export interface HolidayReceiptPlacement {
  readonly childPosition: number;
  readonly weekId: string;
  readonly placement: "placed" | "waitlisted" | "pending";
}

function receiptPlacement(value: string): HolidayReceiptPlacement["placement"] {
  if (value === "waitlisted" || value === "pending") return value;
  return "placed";
}

/** Sends the registration and answers with the place of every booking. */
export async function submitHolidayRegistration(
  token: string,
  payload: HolidayRegistrationPayload,
): Promise<readonly HolidayReceiptPlacement[]> {
  const response = await request(registrationPath(token), {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  if (!response.ok) {
    throw await readError(
      response,
      "Ferienanmeldung konnte nicht gesendet werden",
    );
  }
  const body = (await response.json().catch(() => null)) as {
    data?: {
      placements?:
        { child_position: number; week_id: number; placement: string }[] | null;
    };
  } | null;
  return (body?.data?.placements ?? []).map((entry) => ({
    childPosition: entry.child_position,
    weekId: String(entry.week_id),
    placement: receiptPlacement(entry.placement),
  }));
}

/** The bookings of one child of the form, for its contribution (#4664). */
export type HolidayQuoteChild = readonly {
  readonly week_id: number;
  readonly time_model_id: number;
  readonly days?: readonly string[];
}[];

/**
 * The contribution of the form before it is sent (#4664): computed by the
 * Träger's rules from the bookings and the order of the children by age,
 * stored nowhere.
 */
export async function quoteHolidayContribution(
  token: string,
  children: readonly HolidayQuoteChild[],
  benefitsDeclared: boolean,
  ageRanks: readonly number[] = [],
): Promise<HolidayContribution> {
  const response = await request(`${registrationPath(token)}/contribution`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      children: children.map((bookings, index) => ({
        bookings,
        age_rank: ageRanks[index] ?? 0,
      })),
      benefits_declared: benefitsDeclared,
    }),
  });
  if (!response.ok) {
    throw await readError(
      response,
      "Beitrag der Ferienanmeldung konnte nicht berechnet werden",
    );
  }
  const body = (await response.json()) as { data: BackendContribution };
  return contributionFromBackend(body.data);
}

/** A child of the parent's account the registration can start from (#4662). */
export interface HolidayKnownChild {
  readonly studentId: string;
  readonly schoolId: string;
  readonly firstName: string;
  readonly lastName: string;
  /** "YYYY-MM-DD", or "" when the school knows no date of birth. */
  readonly birthDate: string;
}

/** What the schools of the Träger know of a parent with an account. */
export interface HolidayKnownFamily {
  readonly guardian: {
    readonly firstName: string;
    readonly lastName: string;
    readonly email: string;
    readonly phone: string;
  };
  readonly children: readonly HolidayKnownChild[];
}

interface BackendFamily {
  guardian: {
    first_name: string;
    last_name: string;
    email: string;
    phone: string;
  };
  children?:
    | {
        student_id: string;
        school_id: string;
        first_name: string;
        last_name: string;
        birth_date?: string;
      }[]
    | null;
}

/**
 * Loads the prefill for a parent with an account: the children the parent
 * may register at the schools of the link's Träger, and the parent's contact
 * data. Answers null without a parent session; the form then starts empty.
 */
export async function fetchHolidayRegistrationFamily(
  token: string,
): Promise<HolidayKnownFamily | null> {
  const response = await request(
    `/api/parent/holiday-registrations/${encodeURIComponent(token)}/family`,
    { cache: "no-store" },
  );
  if (response.status === 401) return null;
  if (!response.ok) {
    throw await readError(
      response,
      "Bekannte Angaben konnten nicht geladen werden",
    );
  }
  const { data } = (await response.json()) as { data: BackendFamily };
  return {
    guardian: {
      firstName: data.guardian.first_name,
      lastName: data.guardian.last_name,
      email: data.guardian.email,
      phone: data.guardian.phone,
    },
    children: (data.children ?? []).map((child) => ({
      studentId: child.student_id,
      schoolId: child.school_id,
      firstName: child.first_name,
      lastName: child.last_name,
      birthDate: child.birth_date ?? "",
    })),
  };
}
