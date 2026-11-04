/** Display helpers of the review of the holiday registrations (#4663). */

import type { StatusBadgeTone } from "~/components/ui/status-badge";
import { formatDate } from "~/lib/date-helpers";
import type { HolidayPeriod, TraegerSchool } from "~/lib/traeger-api";
import type {
  HolidayRegistrationChild,
  HolidayRegistrationStatus,
} from "~/lib/traeger-holiday-registrations-api";

export const HOLIDAY_REGISTRATION_STATUS_LABELS: Readonly<
  Record<HolidayRegistrationStatus, string>
> = {
  received: "Eingegangen",
  confirmed: "Bestätigt",
  cancelled: "Storniert",
};

export const HOLIDAY_REGISTRATION_STATUS_TONES: Readonly<
  Record<HolidayRegistrationStatus, StatusBadgeTone>
> = {
  received: "blue",
  confirmed: "green",
  cancelled: "gray",
};

/** "04.01. bis 08.01.2027". */
export function formatHolidayWeek(startsOn: string, endsOn: string): string {
  return `${formatDate(startsOn).slice(0, 6)} bis ${formatDate(endsOn)}`;
}

/** Names of the Träger's schools by ID. */
export function schoolNames(
  schools: readonly TraegerSchool[] | null,
): ReadonlyMap<string, string> {
  return new Map((schools ?? []).map((school) => [school.id, school.name]));
}

/** The school of a child: its school, or the holiday guest with its school. */
export function childSchoolLabel(
  child: HolidayRegistrationChild,
  names: ReadonlyMap<string, string>,
): string {
  if (child.homeSchoolId === null) {
    return child.guestSchoolName
      ? `Feriengast (${child.guestSchoolName})`
      : "Feriengast";
  }
  return names.get(child.homeSchoolId) ?? "Schule nicht mehr vorhanden";
}

/** The time model names of a period, each once, in the order of the weeks. */
export function holidayTimeModelNames(period: HolidayPeriod): string[] {
  const names: string[] = [];
  for (const week of period.weeks) {
    for (const model of week.timeModels) {
      if (!names.includes(model.name)) names.push(model.name);
    }
  }
  return names;
}
