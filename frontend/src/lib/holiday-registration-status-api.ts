// Stand einer Ferienanmeldung über den geheimen Link aus der Eingangsmail
// (#6676): ohne Elternkonto, ohne Gesundheitsangaben, Anschrift, Kontakte,
// Nachnamen und Geburtsdaten.

import { unavailableApiError } from "~/lib/api-error";
import { readEnrollmentError } from "~/lib/enrollment-api-error";
import { createLogger } from "~/lib/logger";
import type {
  HolidayPlacement,
  HolidayRegistrationEntryStatus,
} from "~/lib/parent-holiday-registration-entries";

const logger = createLogger({ component: "HolidayRegistrationStatusApi" });

/** The address of the status page on the parents host, by its token. */
export function holidayRegistrationStatusPath(token: string): string {
  return `/parents/ferienanmeldung/status/${encodeURIComponent(token)}`;
}

export interface HolidayRegistrationStatusWeek {
  /** "YYYY-MM-DD" */
  readonly firstDay: string;
  /** "YYYY-MM-DD" */
  readonly lastDay: string;
  readonly timeModelName: string;
  /** "HH:MM", or "" when unknown. */
  readonly startsAt: string;
  readonly endsAt: string;
  readonly hostSchoolName: string;
  readonly placement: HolidayPlacement;
  /** The child eats lunch in the week (#5631). */
  readonly lunch?: boolean;
}

export interface HolidayRegistrationStatusChild {
  readonly firstName: string;
  readonly weeks: readonly HolidayRegistrationStatusWeek[];
}

export interface HolidayRegistrationStatus {
  readonly traegerName: string;
  readonly periodName: string;
  /** "YYYY-MM-DD", or "" while the period has no weeks. */
  readonly firstDay: string;
  readonly lastDay: string;
  /** "YYYY-MM-DD": the last day parents can register. */
  readonly registrationClosesOn: string;
  readonly binding: boolean;
  /** "criteria": the office gives the places after the window closes. */
  readonly allocationMode: "arrival" | "criteria";
  /** RFC 3339 instant. */
  readonly submittedAt: string;
  readonly status: HolidayRegistrationEntryStatus;
  readonly children: readonly HolidayRegistrationStatusChild[];
}

interface BackendStatusWeek {
  first_day: string;
  last_day: string;
  time_model_name?: string | null;
  starts_at?: string | null;
  ends_at?: string | null;
  host_school_name?: string | null;
  placement?: string | null;
  lunch?: boolean;
}

interface BackendHolidayRegistrationStatus {
  traeger_name: string;
  period_name: string;
  first_day?: string | null;
  last_day?: string | null;
  registration_closes_on: string;
  binding: boolean;
  allocation_mode?: string | null;
  submitted_at: string;
  status: string;
  children?:
    { first_name: string; weeks?: BackendStatusWeek[] | null }[] | null;
}

function statusOf(value: string): HolidayRegistrationEntryStatus {
  return value === "confirmed" || value === "cancelled" ? value : "received";
}

function placementOf(value: string | null | undefined): HolidayPlacement {
  return value === "waitlisted" || value === "pending" ? value : "placed";
}

function holidayRegistrationStatusFromBackend(
  body: BackendHolidayRegistrationStatus,
): HolidayRegistrationStatus {
  return {
    traegerName: body.traeger_name,
    periodName: body.period_name,
    firstDay: body.first_day ?? "",
    lastDay: body.last_day ?? "",
    registrationClosesOn: body.registration_closes_on,
    binding: body.binding,
    allocationMode:
      body.allocation_mode === "criteria" ? "criteria" : "arrival",
    submittedAt: body.submitted_at,
    status: statusOf(body.status),
    children: (body.children ?? []).map((child) => ({
      firstName: child.first_name,
      weeks: (child.weeks ?? []).map((week) => ({
        firstDay: week.first_day,
        lastDay: week.last_day,
        timeModelName: week.time_model_name ?? "",
        startsAt: week.starts_at ?? "",
        endsAt: week.ends_at ?? "",
        hostSchoolName: week.host_school_name ?? "",
        placement: placementOf(week.placement),
        ...(week.lunch ? { lunch: true } : {}),
      })),
    })),
  };
}

/** Reads the state of the registration a status link names. */
export async function fetchHolidayRegistrationStatus(
  token: string,
): Promise<HolidayRegistrationStatus> {
  let response: Response;
  try {
    response = await fetch(
      `/api/holiday-registrations/status/${encodeURIComponent(token)}`,
      { cache: "no-store" },
    );
  } catch (cause) {
    throw unavailableApiError(cause);
  }
  if (!response.ok) {
    throw await readEnrollmentError(
      response,
      "Stand der Ferienanmeldung konnte nicht geladen werden",
      logger,
      "holiday_registration_status_api_failed",
    );
  }
  const body = (await response.json()) as {
    data: BackendHolidayRegistrationStatus;
  };
  return holidayRegistrationStatusFromBackend(body.data);
}
