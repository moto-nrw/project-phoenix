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
  };
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
): Promise<TraegerClosingDay> {
  return mapTraegerClosingDay(
    await traegerRequest<BackendClosingDay>(basePath, {
      method: "POST",
      body: backendInput(input),
    }),
  );
}

export async function updateTraegerClosingDay(
  id: string,
  input: TraegerClosingDayInput,
): Promise<TraegerClosingDay> {
  return mapTraegerClosingDay(
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
