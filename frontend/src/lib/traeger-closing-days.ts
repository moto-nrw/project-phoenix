/**
 * Schließtage des Trägers und Feiertage seiner Schulen (#4702).
 *
 * Das Träger-Büro plant Schließtage wie Brückentage oder einen
 * Betriebsausflug für einige oder alle Schulen. Sie wirken wie die
 * Schließtage, die eine Schule selbst einträgt. Die gesetzlichen Feiertage
 * kommen aus dem Bundesland jeder Schule; das Bundesland legt das moto-Team
 * fest, der Träger sieht es nur.
 */
import {
  type BackendHolidayClosureNotice,
  type HolidayClosureNotice,
  mapHolidayClosureNotice,
} from "./holiday-closure-notices";
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

/** Ein geschlossener oder wieder geöffneter Tag in einer gebuchten Ferienwoche. */
interface HolidayClosedDay {
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
  /** Tage, die nach einer Änderung oder Löschung doch geöffnet sind (#6457). */
  readonly reopenedDays: readonly HolidayClosedDay[];
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

/**
 * Ein Elternhinweis zu einem Schul-Schließtag in der Liste des Träger-Büros
 * (ADR 0196): mit den betroffenen Wochen und Kindern, solange er auf einen
 * Klick wartet. Den Namen der Schule liefert der Kalender des Trägers.
 */
export interface TraegerHolidayClosureNotice extends HolidayClosureNotice {
  readonly weeks: readonly HolidayClosureWeek[];
  readonly children: number;
}

/** Die Liste „Eltern noch nicht informiert“ mit allen Hinweisen. */
export interface PendingHolidayClosures extends HolidayClosureImpact {
  readonly retryableMails?: number;
  readonly unknownMails?: number;
  readonly notices: readonly TraegerHolidayClosureNotice[];
}

interface BackendHolidayClosureWeek {
  week_id: number;
  period_name: string;
  starts_on: string;
  ends_on: string;
  host_school_id: number;
  closed_days?: { date: string; reason: string }[] | null;
  reopened_days?: { date: string; reason: string }[] | null;
  children: number;
  waitlisted: number;
  families: number;
}

interface BackendHolidayClosureImpact {
  weeks?: BackendHolidayClosureWeek[] | null;
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

/**
 * Der gespeicherte Stand eines Schließtags, den die Vorschau beim Ändern oder
 * Löschen mitschickt: Seine Tage können wieder öffnen (#6457).
 */
export interface TraegerClosingDayChange {
  readonly closingDay: TraegerClosingDay;
  /** Vorschau zum Löschen statt zum Speichern. */
  readonly delete?: boolean;
}

function backendChange(change: TraegerClosingDayChange | undefined) {
  if (!change) return {};
  return {
    closing_day_id: Number(change.closingDay.id),
    previous: {
      school_ids: change.closingDay.schoolIds.map(Number),
      start_date: change.closingDay.startDate,
      end_date: change.closingDay.endDate,
    },
    ...(change.delete ? { delete: true } : {}),
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

function mapHolidayClosureWeek(
  week: BackendHolidayClosureWeek,
): HolidayClosureWeek {
  return {
    weekId: String(week.week_id),
    periodName: week.period_name,
    startsOn: week.starts_on,
    endsOn: week.ends_on,
    hostSchoolId: String(week.host_school_id),
    closedDays: week.closed_days ?? [],
    reopenedDays: week.reopened_days ?? [],
    children: week.children,
    waitlisted: week.waitlisted,
    families: week.families,
  };
}

type BackendTraegerNotice = BackendHolidayClosureNotice & {
  weeks?: BackendHolidayClosureWeek[] | null;
  children?: number;
};

function mapTraegerNotice(
  notice: BackendTraegerNotice,
): TraegerHolidayClosureNotice {
  return {
    ...mapHolidayClosureNotice(notice),
    weeks: (notice.weeks ?? []).map(mapHolidayClosureWeek),
    children: notice.children ?? 0,
  };
}

function mapHolidayClosureImpact(
  data: BackendHolidayClosureImpact | null | undefined,
): HolidayClosureImpact {
  return {
    weeks: (data?.weeks ?? []).map(mapHolidayClosureWeek),
    children: data?.children ?? 0,
    families: data?.families ?? 0,
    token: data?.token ?? "",
    keepsContribution: data?.keeps_contribution ?? true,
  };
}

const closuresPath = "/api/traeger/holiday-closures";

/**
 * Welche Ferienbuchungen der geplante Schließtag betrifft (#6434). Beim
 * Ändern oder Löschen nennt `change` den gespeicherten Stand; dann zeigt die
 * Vorschau auch die Tage, die wieder öffnen (#6457).
 */
export async function previewTraegerClosingDayHolidayBookings(
  input: TraegerClosingDayInput,
  change?: TraegerClosingDayChange,
): Promise<HolidayClosureImpact> {
  return mapHolidayClosureImpact(
    await traegerRequest<BackendHolidayClosureImpact>(
      `${closuresPath}/preview`,
      {
        method: "POST",
        body: { ...backendInput(input), ...backendChange(change) },
      },
    ),
  );
}

/**
 * Elternhinweise zu Schließtagen in gebuchten Ferienwochen (ADR 0196): die
 * Vorschläge, die auf einen Klick warten, mit ihren Wochen und Familien, und
 * jeder beauftragte Hinweis mit seinem Versandstand.
 */
export async function fetchPendingHolidayClosures(): Promise<PendingHolidayClosures> {
  const data = await traegerRequest<
    | (BackendHolidayClosureImpact & {
        notices?: BackendTraegerNotice[] | null;
        retryable_mails?: number;
        unknown_mails?: number;
      })
    | null
  >(`${closuresPath}/pending`);
  return {
    ...mapHolidayClosureImpact(data),
    notices: (data?.notices ?? []).map(mapTraegerNotice),
    retryableMails: data?.retryable_mails ?? 0,
    unknownMails: data?.unknown_mails ?? 0,
  };
}

/**
 * Beauftragt einen Hinweis mit dem Stand, den das Büro gesehen hat, oder holt
 * einen gescheiterten Versand nach. Derselbe Auftrag wie beim Klick der Schule.
 */
export async function notifyTraegerHolidayClosureNotice(
  id: string,
  revision: number,
  token: string,
): Promise<HolidayClosureNotice> {
  return mapHolidayClosureNotice(
    await traegerRequest<BackendHolidayClosureNotice>(
      `${closuresPath}/pending/${encodeURIComponent(id)}/notify`,
      { method: "POST", body: { revision, token } },
    ),
  );
}

/** Beauftragt alle Vorschläge der Liste; gibt die Zahl der Familien. */
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

/**
 * Löscht den Schließtag. Betrifft er Ferienbuchungen, bestätigt
 * `holidayBookingsToken` die Vorschau; die Eltern erfahren dann, dass ihre
 * Tage wieder geöffnet sind (#6457). Gibt die Zahl der Familien.
 */
export async function deleteTraegerClosingDay(
  id: string,
  holidayBookingsToken = "",
): Promise<number> {
  const query = holidayBookingsToken
    ? `?${new URLSearchParams({ holiday_bookings_token: holidayBookingsToken }).toString()}`
    : "";
  const result = await traegerRequest<{ notified_families?: number } | null>(
    `${basePath}/${encodeURIComponent(id)}${query}`,
    { method: "DELETE" },
  );
  return result?.notified_families ?? 0;
}
