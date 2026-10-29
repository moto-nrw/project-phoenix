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
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "HolidayRegistrationApi" });

export interface HolidayRegistrationTimeModel {
  readonly id: string;
  readonly name: string;
  readonly startsAt: string;
  readonly endsAt: string;
}

export interface HolidayRegistrationWeek {
  readonly id: string;
  readonly startsOn: string;
  readonly endsOn: string;
  readonly hostSchoolName: string;
  readonly timeModels: readonly HolidayRegistrationTimeModel[];
}

interface HolidayRegistrationSchool {
  readonly id: string;
  readonly name: string;
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
  readonly schools: readonly HolidayRegistrationSchool[];
  readonly captchaSiteKey: string;
  /** The questions of the Träger in their current version (#5198). */
  readonly form: HolidayFormVersion;
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
        time_models?:
          | { id: number; name: string; starts_at: string; ends_at: string }[]
          | null;
      }[]
    | null;
  schools?: { id: number; name: string }[] | null;
  captcha_site_key: string;
  form?: { version: number; fields?: HolidayFormField[] | null } | null;
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
    }[];
    /** Answers to the questions per child, by key (#5198). */
    readonly custom_data: Readonly<Record<string, unknown>>;
  }[];
  readonly emergency_contacts: readonly HolidayContactPayload[];
  readonly pickup_persons: readonly HolidayContactPayload[];
  readonly privacy_consent: boolean;
  readonly health_consent: boolean;
  readonly binding_confirmed: boolean;
  readonly benefits_declared: boolean;
  readonly captcha_token: string;
  /** The form version the parent answered (#5198). */
  readonly form_version: number;
  /** Answers to the questions per family, by key (#5198). */
  readonly custom_data: Readonly<Record<string, unknown>>;
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
      timeModels: (week.time_models ?? []).map((model) => ({
        id: String(model.id),
        name: model.name,
        startsAt: model.starts_at,
        endsAt: model.ends_at,
      })),
    })),
    schools: (offer.schools ?? []).map((school) => ({
      id: String(school.id),
      name: school.name,
    })),
    captchaSiteKey: offer.captcha_site_key,
    form: {
      version: offer.form?.version ?? 0,
      fields: offer.form?.fields ?? [],
    },
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

export async function submitHolidayRegistration(
  token: string,
  payload: HolidayRegistrationPayload,
): Promise<void> {
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
}

/** The bookings of one child of the form, for its contribution (#4664). */
export type HolidayQuoteChild = readonly {
  readonly week_id: number;
  readonly time_model_id: number;
}[];

/**
 * The contribution of the form before it is sent (#4664): computed by the
 * Träger's rules from the bookings alone, stored nowhere.
 */
export async function quoteHolidayContribution(
  token: string,
  children: readonly HolidayQuoteChild[],
  benefitsDeclared: boolean,
): Promise<HolidayContribution> {
  const response = await request(`${registrationPath(token)}/contribution`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      children: children.map((bookings) => ({ bookings })),
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
