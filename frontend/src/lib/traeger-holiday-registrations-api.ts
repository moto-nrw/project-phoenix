/**
 * Client of the review of the holiday registrations in the Träger portal
 * (#4663): the list of a holiday period with its filters, one registration
 * with its history, the changes of the Träger-Büro and the export as CSV or
 * Excel. Talks to the BFF routes under
 * /api/traeger/holiday-periods/{id}/registrations. Backend int64 IDs leave
 * here as strings.
 */

import { apiErrorFromBody, transportFetch } from "./api-error";
import { filenameFromDisposition } from "./file-download";
import { traegerRequest } from "./traeger-api";

export type HolidayRegistrationStatus = "received" | "confirmed" | "cancelled";

/**
 * The changes the Träger-Büro makes with one action. A confirmation freezes
 * the contribution and goes through the Elternabrechnung (#4664,
 * `confirmHolidayRegistration` in traeger-contributions-api).
 */
export type HolidayRegistrationAction = "cancel" | "freeze" | "reopen";

type HolidayRegistrationEventAction =
  HolidayRegistrationAction | "confirm" | "rebook";

/** The filter of the list and the export. Empty values match everything. */
export interface HolidayRegistrationFilter {
  readonly status: HolidayRegistrationStatus | "";
  /** A school ID, "guest" for holiday guests, or empty for all. */
  readonly school: string;
  readonly weekId: string;
  /** A time model name; time models are per week. */
  readonly timeModel: string;
}

export const EMPTY_HOLIDAY_REGISTRATION_FILTER: HolidayRegistrationFilter = {
  status: "",
  school: "",
  weekId: "",
  timeModel: "",
};

interface HolidayRegistrationBooking {
  readonly weekId: string;
  /** Calendar dates "YYYY-MM-DD". */
  readonly weekStartsOn: string;
  readonly weekEndsOn: string;
  readonly hostSchoolId: string;
  readonly timeModelId: string;
  readonly timeModelName: string;
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
  readonly children: readonly HolidayRegistrationChild[];
}

export interface HolidayRegistrationList {
  readonly registrations: readonly HolidayRegistrationSummary[];
  /** Registrations of the whole period, whatever the filter. */
  readonly total: number;
  readonly statusCounts: Readonly<Record<HolidayRegistrationStatus, number>>;
}

export interface HolidayRegistrationHealth {
  readonly childId: string;
  readonly allergies: string;
  readonly medication: string;
  readonly healthNotes: string;
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
}

export interface HolidayRegistrationDetail extends HolidayRegistrationSummary {
  readonly health: readonly HolidayRegistrationHealth[];
  readonly contacts: readonly HolidayRegistrationContact[];
  readonly privacyConsentAt: string;
  readonly healthConsentAt: string | null;
  readonly bindingConfirmedAt: string | null;
  readonly history: readonly HolidayRegistrationEvent[];
}

interface BackendBooking {
  week_id: number;
  week_starts_on: string;
  week_ends_on: string;
  host_school_id: number;
  time_model_id: number;
  time_model_name: string;
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
}

interface BackendDetail extends BackendSummary {
  health?:
    | {
        child_id: number;
        allergies: string;
        medication: string;
        health_notes: string;
      }[]
    | null;
  contacts?: HolidayRegistrationContact[] | null;
  privacy_consent_at: string;
  health_consent_at: string | null;
  binding_confirmed_at: string | null;
  history?:
    | {
        action: HolidayRegistrationEventAction;
        occurred_at: string;
        details?: {
          from?: HolidayRegistrationStatus;
          child_id?: number;
          before?: BackendBookingChange[] | null;
          after?: BackendBookingChange[] | null;
        } | null;
      }[]
    | null;
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
      })),
      duplicateRegistrationIds: (child.duplicate_registration_ids ?? []).map(
        String,
      ),
    })),
  };
}

function changesFromBackend(
  changes: BackendBookingChange[] | null | undefined,
): HolidayBookingChange[] {
  return (changes ?? []).map((change) => ({
    weekId: String(change.week_id),
    timeModelId: String(change.time_model_id),
  }));
}

function detailFromBackend(
  registration: BackendDetail,
): HolidayRegistrationDetail {
  return {
    ...summaryFromBackend(registration),
    health: (registration.health ?? []).map((entry) => ({
      childId: String(entry.child_id),
      allergies: entry.allergies,
      medication: entry.medication,
      healthNotes: entry.health_notes,
    })),
    contacts: registration.contacts ?? [],
    privacyConsentAt: registration.privacy_consent_at,
    healthConsentAt: registration.health_consent_at,
    bindingConfirmedAt: registration.binding_confirmed_at,
    history: (registration.history ?? []).map((event) => ({
      action: event.action,
      occurredAt: event.occurred_at,
      from: event.details?.from ?? "",
      childId: event.details?.child_id ? String(event.details.child_id) : "",
      before: changesFromBackend(event.details?.before),
      after: changesFromBackend(event.details?.after),
    })),
  };
}

const registrationsPath = (periodId: string) =>
  `/api/traeger/holiday-periods/${encodeURIComponent(periodId)}/registrations`;

const registrationPath = (periodId: string, registrationId: string) =>
  `${registrationsPath(periodId)}/${encodeURIComponent(registrationId)}`;

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

/** One registration with health information, contacts and history. */
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

/** Cancels, freezes or reopens a registration. */
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
          })),
        },
      },
    ),
  );
}

/** The export of a filter as CSV or Excel, with the backend's file name. */
export async function downloadHolidayRegistrations(
  periodId: string,
  filter: HolidayRegistrationFilter,
  format: "csv" | "xlsx",
): Promise<{ readonly blob: Blob; readonly filename: string | null }> {
  const response = await transportFetch(
    `${registrationsPath(periodId)}/export${holidayRegistrationQuery(filter, { format })}`,
    { credentials: "include", cache: "no-store" },
  );
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
