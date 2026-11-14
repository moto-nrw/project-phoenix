// Die eigenen Ferienanmeldungen eines Elternkontos (#6673, ADR 0216): die
// Anmeldungen, die mit dem Konto verknüpft sind, neueste zuerst. Ohne
// Gesundheitsangaben, Anschrift und Kontakte.

/** Platz, Warteliste oder offener Platz einer Woche (#4699, #4700). */
export type HolidayPlacement = "placed" | "waitlisted" | "pending";

export type HolidayRegistrationEntryStatus =
  "received" | "confirmed" | "cancelled";

/** One week of one child: its time model and whether it got a place. */
interface HolidayRegistrationEntryWeek {
  /** "YYYY-MM-DD" */
  readonly firstDay: string;
  /** "YYYY-MM-DD" */
  readonly lastDay: string;
  readonly timeModelName: string;
  readonly placement: HolidayPlacement;
  /** The child eats lunch in the week (#5631). */
  readonly lunch?: boolean;
}

interface HolidayRegistrationEntryChild {
  readonly firstName: string;
  readonly weeks: readonly HolidayRegistrationEntryWeek[];
}

export interface HolidayRegistrationEntry {
  readonly id: string;
  readonly traegerName: string;
  readonly periodName: string;
  /** "YYYY-MM-DD", or "" while the period has no weeks. */
  readonly firstDay: string;
  readonly lastDay: string;
  /** RFC 3339 instant. */
  readonly submittedAt: string;
  readonly status: HolidayRegistrationEntryStatus;
  readonly children: readonly HolidayRegistrationEntryChild[];
}

interface BackendHolidayRegistrationEntry {
  id: number;
  traeger_name: string;
  period_name: string;
  first_day?: string | null;
  last_day?: string | null;
  submitted_at: string;
  status: string;
  children?:
    | {
        first_name: string;
        weeks?:
          | {
              first_day: string;
              last_day: string;
              time_model_name: string;
              placement: string;
              lunch?: boolean;
            }[]
          | null;
      }[]
    | null;
}

export interface BackendHolidayRegistrationEntries {
  registrations?: BackendHolidayRegistrationEntry[] | null;
}

function entryStatus(value: string): HolidayRegistrationEntryStatus {
  return value === "confirmed" || value === "cancelled" ? value : "received";
}

function weekPlacement(value: string): HolidayPlacement {
  return value === "waitlisted" || value === "pending" ? value : "placed";
}

function holidayRegistrationEntryFromBackend(
  entry: BackendHolidayRegistrationEntry,
): HolidayRegistrationEntry {
  return {
    id: String(entry.id),
    traegerName: entry.traeger_name,
    periodName: entry.period_name,
    firstDay: entry.first_day ?? "",
    lastDay: entry.last_day ?? "",
    submittedAt: entry.submitted_at,
    status: entryStatus(entry.status),
    children: (entry.children ?? []).map((child) => ({
      firstName: child.first_name,
      weeks: (child.weeks ?? []).map((week) => ({
        firstDay: week.first_day,
        lastDay: week.last_day,
        timeModelName: week.time_model_name,
        placement: weekPlacement(week.placement),
        ...(week.lunch ? { lunch: true } : {}),
      })),
    })),
  };
}

/** The entries of the answer in the backend's order, newest first. */
export function holidayRegistrationEntriesFromBackend(
  body: BackendHolidayRegistrationEntries | null | undefined,
): HolidayRegistrationEntry[] {
  return (body?.registrations ?? []).map(holidayRegistrationEntryFromBackend);
}
