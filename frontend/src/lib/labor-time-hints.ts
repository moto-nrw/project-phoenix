// Hinweise zu Pausen und Höchstarbeitszeit (#4804). Die Werte und Schwellen
// kommen vom Backend (labor_days, is_over_weekly_max); hier entsteht nur der
// Text. Die Hinweise sperren nichts.

import type {
  WeeklySummary,
  WorkDayLaborHint,
} from "~/lib/time-tracking-helpers";

/** Ein Hinweis zu einem Tag oder einer Woche, wie er angezeigt wird. */
export interface LaborHint {
  readonly key: "break" | "daily-max" | "weekly-max";
  /** Kurzer Text für das Badge. */
  readonly label: string;
  /** Die Zahlen zum Badge, eine eigene Zeile darunter („30 Min. nötig …“). */
  readonly detail?: string;
  /** Der ganze Satz für Stellen ohne Badge: Stempeluhr und Dialog. */
  readonly text: string;
  /** Langer Text mit Gesetzesbezug für den Tooltip. */
  readonly title: string;
}

export const breakTooShortLabel = "Pause zu kurz";

/** „30 Min. nötig, 15 Min. gemacht“. */
export function breakTooShortDetail(
  requiredMinutes: number,
  countedMinutes: number,
): string {
  return `${requiredMinutes} Min. nötig, ${countedMinutes} Min. gemacht`;
}

const breakTitle =
  "Nach 6 Stunden Arbeit sind 30 Minuten Pause nötig, nach 9 Stunden 45 Minuten (§ 4 Arbeitszeitgesetz). Es zählen nur Pausen ab 15 Minuten.";
const dailyMaxTitle =
  "An einem Tag sind höchstens 10 Stunden Arbeit erlaubt (§ 3 Arbeitszeitgesetz).";
const weeklyMaxTitle =
  "In einer Woche sind höchstens 48 Stunden Arbeit erlaubt (§ 3 Arbeitszeitgesetz).";

export const dailyMaxLabel = "Mehr als 10 Std. gearbeitet";
export const weeklyMaxLabel = "Mehr als 48 Std. in der Woche";

/** Hinweise eines Arbeitstags in fester Reihenfolge: Pause, dann Tageslimit. */
export function dayLaborHints(day: WorkDayLaborHint | undefined): LaborHint[] {
  if (!day) return [];
  const hints: LaborHint[] = [];
  if (day.isBreakTooShort) {
    const detail = breakTooShortDetail(
      day.requiredBreakMinutes,
      day.countedBreakMinutes,
    );
    hints.push({
      key: "break",
      label: breakTooShortLabel,
      detail,
      text: `${breakTooShortLabel}: ${detail}`,
      title: breakTitle,
    });
  }
  if (day.isOverDailyMax) {
    hints.push({
      key: "daily-max",
      label: dailyMaxLabel,
      text: dailyMaxLabel,
      title: dailyMaxTitle,
    });
  }
  return hints;
}

export function weekLaborHints(week: WeeklySummary | undefined): LaborHint[] {
  if (!week?.isOverWeeklyMax) return [];
  return [
    {
      key: "weekly-max",
      label: weeklyMaxLabel,
      text: weeklyMaxLabel,
      title: weeklyMaxTitle,
    },
  ];
}

/**
 * Hinweise eines laufenden Tags zum Zeitpunkt `now`. Der Server liefert für
 * einen laufenden Tag ohne laufende Pause die Zeitpunkte, ab denen ein
 * Hinweis gilt; so wechselt die Anzeige ohne neue Abfrage und ohne eigene
 * Schwellen im Browser.
 */
export function liveDayLaborHints(
  day: WorkDayLaborHint | undefined,
  now: Date,
): LaborHint[] {
  if (!day) return [];
  const reached = (at: string | null) =>
    at !== null && now.getTime() >= new Date(at).getTime();
  const live: WorkDayLaborHint = {
    ...day,
    isOverDailyMax: day.isOverDailyMax || reached(day.overDailyMaxAt),
    isBreakTooShort: day.isBreakTooShort || reached(day.breakTooShortAt),
    requiredBreakMinutes:
      !day.isBreakTooShort && reached(day.breakTooShortAt)
        ? day.breakTooShortRequiredMinutes
        : day.requiredBreakMinutes,
  };
  return dayLaborHints(live);
}

/** Index der Arbeitstage nach Datum ("YYYY-MM-DD"). */
export function indexLaborDays(
  days: readonly WorkDayLaborHint[] | undefined,
): Map<string, WorkDayLaborHint> {
  return new Map((days ?? []).map((day) => [day.date, day]));
}
