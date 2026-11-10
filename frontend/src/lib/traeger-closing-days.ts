/**
 * Schließtage des Trägers und Feiertage seiner Schulen (#4702).
 *
 * Das Träger-Büro plant Schließtage wie Brückentage oder einen
 * Betriebsausflug für einige oder alle Schulen. Sie wirken wie die
 * Schließtage, die eine Schule selbst einträgt. Die gesetzlichen Feiertage
 * kommen aus dem Bundesland jeder Schule; das Bundesland legt das moto-Team
 * fest, der Träger sieht es nur.
 */
import { traegerRequest } from "./traeger-api";

export interface TraegerClosingDay {
  readonly id: string;
  readonly startDate: string;
  readonly endDate: string;
  readonly reason: string;
  readonly schoolIds: readonly string[];
}

export interface TraegerClosingDayInput {
  readonly startDate: string;
  readonly endDate: string;
  readonly reason: string;
  readonly schoolIds: readonly string[];
  /**
   * Bestätigt die Vorschau der betroffenen Ferienbuchungen (#6434); leer,
   * wenn die Vorschau keine zeigte.
   */
  readonly holidayBookingsToken?: string;
}

/** Ein gespeicherter Schließtag mit der Zahl der informierten Familien. */
export interface SavedTraegerClosingDay extends TraegerClosingDay {
  readonly notifiedFamilies: number;
}

/** Ein geschlossener Tag in einer gebuchten Ferienwoche. */
export interface HolidayClosedDay {
  readonly date: string;
  readonly reason: string;
}

/** Eine betroffene Ferienwoche mit der Zahl der Kinder und Familien. */
export interface HolidayClosureWeek {
  readonly weekId: string;
  readonly periodName: string;
  readonly startsOn: string;
  readonly endsOn: string;
  readonly hostSchoolId: string;
  readonly closedDays: readonly HolidayClosedDay[];
  readonly children: number;
  readonly waitlisted: number;
  readonly families: number;
}

/**
 * Was ein Schließtag für die Ferienbuchungen heißt (#6434): die betroffenen
 * Wochen, Kinder und Familien und das Zeichen, mit dem das Speichern die
 * Vorschau bestätigt. Keine Namen von Kindern.
 */
export interface HolidayClosureImpact {
  readonly weeks: readonly HolidayClosureWeek[];
  readonly children: number;
  readonly families: number;
  readonly token: string;
  readonly keepsContribution: boolean;
}

interface BackendHolidayClosureImpact {
  weeks?:
    | {
        week_id: number;
        period_name: string;
        starts_on: string;
        ends_on: string;
        host_school_id: number;
        closed_days?: { date: string; reason: string }[] | null;
        children: number;
        waitlisted: number;
        families: number;
      }[]
    | null;
  children?: number;
  families?: number;
  token?: string;
  keeps_contribution?: boolean;
}

export interface TraegerCalendarSchool {
  readonly id: string;
  readonly name: string;
  readonly federalStateCode: string;
  readonly federalStateName: string;
}

interface TraegerHoliday {
  readonly date: string;
  readonly name: string;
}

interface TraegerRegionHolidays {
  readonly code: string;
  readonly name: string;
  readonly holidays: readonly TraegerHoliday[];
}

export interface TraegerCalendar {
  readonly closingDays: readonly TraegerClosingDay[];
  readonly schools: readonly TraegerCalendarSchool[];
  readonly regions: readonly TraegerRegionHolidays[];
}

interface BackendClosingDay {
  id: number;
  start_date: string;
  end_date: string;
  reason: string;
  school_ids?: number[] | null;
}

interface BackendCalendar {
  closing_days?: BackendClosingDay[] | null;
  schools?:
    | {
        id: number;
        name: string;
        federal_state: { code: string; name: string };
      }[]
    | null;
  holidays?:
    | {
        region: { code: string; name: string };
        holidays?: { date: string; name: string }[] | null;
      }[]
    | null;
}

const basePath = "/api/traeger/closing-days";

function mapTraegerClosingDay(day: BackendClosingDay): TraegerClosingDay {
  return {
    id: String(day.id),
    startDate: day.start_date,
    endDate: day.end_date,
    reason: day.reason,
    schoolIds: (day.school_ids ?? []).map(String),
  };
}

function mapTraegerCalendar(
  data: BackendCalendar | null | undefined,
): TraegerCalendar {
  return {
    closingDays: (data?.closing_days ?? []).map(mapTraegerClosingDay),
    schools: (data?.schools ?? []).map((school) => ({
      id: String(school.id),
      name: school.name,
      federalStateCode: school.federal_state.code,
      federalStateName: school.federal_state.name,
    })),
    regions: (data?.holidays ?? []).map((region) => ({
      code: region.region.code,
      name: region.region.name,
      holidays: region.holidays ?? [],
    })),
  };
}

function backendInput(input: TraegerClosingDayInput) {
  return {
    start_date: input.startDate,
    end_date: input.endDate,
    reason: input.reason.trim(),
    school_ids: input.schoolIds.map(Number),
    holiday_bookings_token: input.holidayBookingsToken ?? "",
  };
}

function mapSavedClosingDay(
  day: BackendClosingDay & { notified_families?: number },
): SavedTraegerClosingDay {
  return {
    ...mapTraegerClosingDay(day),
    notifiedFamilies: day.notified_families ?? 0,
  };
}

function mapHolidayClosureImpact(
  data: BackendHolidayClosureImpact | null | undefined,
): HolidayClosureImpact {
  return {
    weeks: (data?.weeks ?? []).map((week) => ({
      weekId: String(week.week_id),
      periodName: week.period_name,
      startsOn: week.starts_on,
      endsOn: week.ends_on,
      hostSchoolId: String(week.host_school_id),
      closedDays: week.closed_days ?? [],
      children: week.children,
      waitlisted: week.waitlisted,
      families: week.families,
    })),
    children: data?.children ?? 0,
    families: data?.families ?? 0,
    token: data?.token ?? "",
    keepsContribution: data?.keeps_contribution ?? true,
  };
}

const closuresPath = "/api/traeger/holiday-closures";

/** Welche Ferienbuchungen der geplante Schließtag betrifft (#6434). */
export async function previewTraegerClosingDayHolidayBookings(
  input: TraegerClosingDayInput,
): Promise<HolidayClosureImpact> {
  return mapHolidayClosureImpact(
    await traegerRequest<BackendHolidayClosureImpact>(
      `${closuresPath}/preview`,
      { method: "POST", body: backendInput(input) },
    ),
  );
}

/**
 * Schließtage in gebuchten Ferienwochen, über die noch niemand die Eltern
 * informiert hat, etwa ein Schließtag, den eine Schule selbst eingetragen hat.
 */
export async function fetchPendingHolidayClosures(): Promise<HolidayClosureImpact> {
  return mapHolidayClosureImpact(
    await traegerRequest<BackendHolidayClosureImpact>(
      `${closuresPath}/pending`,
    ),
  );
}

/** Informiert die Eltern der offenen Schließtage; gibt die Zahl der Familien. */
export async function notifyPendingHolidayClosures(
  token: string,
): Promise<number> {
  const result = await traegerRequest<{ families?: number }>(
    `${closuresPath}/pending/notify`,
    { method: "POST", body: { token } },
  );
  return result?.families ?? 0;
}

/** Der Kalender des Trägers im Fenster [from, to]. */
export async function fetchTraegerCalendar(
  from: string,
  to: string,
): Promise<TraegerCalendar> {
  const query = new URLSearchParams({ from, to });
  return mapTraegerCalendar(
    await traegerRequest<BackendCalendar>(`${basePath}?${query.toString()}`),
  );
}

export async function createTraegerClosingDay(
  input: TraegerClosingDayInput,
): Promise<SavedTraegerClosingDay> {
  return mapSavedClosingDay(
    await traegerRequest<BackendClosingDay>(basePath, {
      method: "POST",
      body: backendInput(input),
    }),
  );
}

export async function updateTraegerClosingDay(
  id: string,
  input: TraegerClosingDayInput,
): Promise<SavedTraegerClosingDay> {
  return mapSavedClosingDay(
    await traegerRequest<BackendClosingDay>(
      `${basePath}/${encodeURIComponent(id)}`,
      { method: "PUT", body: backendInput(input) },
    ),
  );
}

export async function deleteTraegerClosingDay(id: string): Promise<void> {
  await traegerRequest<unknown>(`${basePath}/${encodeURIComponent(id)}`, {
    method: "DELETE",
  });
}
