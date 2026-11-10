"use client";

import { differenceInCalendarDays } from "date-fns/differenceInCalendarDays";
import { CalendarPlus, SquarePen, Thermometer } from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { useInvalidateMatching } from "~/lib/swr";

import { EditHistoryAccordion } from "~/components/time-tracking/edit-history-accordion";
import { Alert } from "~/components/ui/alert";
import {
  DataTable,
  type DataTableColumn,
  type DataTableExtraRow,
} from "~/components/ui/data-table";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import {
  StatusBadge,
  type StatusBadgeTone,
} from "~/components/ui/status-badge";
import { StatusColorBadge } from "~/components/ui/status-color-badge";
import { MOTO_COLOR_PALETTE } from "~/lib/location-helper";
import { ABSENCE_TYPE_HEX, ABSENCE_TYPE_LABEL } from "~/lib/absence-helpers";
import {
  berlinClockFromISO,
  berlinTodayISO,
  endOfBerlinDayISO,
  isoWeekNumber,
  parseISODate,
  toISODate,
} from "~/lib/date-helpers";
import {
  dayLaborHints,
  indexLaborDays,
  weekLaborHints,
  type LaborHint,
} from "~/lib/labor-time-hints";
import { useApiLoadError } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import type {
  StaffAbsenceRow,
  StaffHistorySession,
  StaffSchedule,
} from "~/lib/staff-api";
import { staffSessionEditsService } from "~/lib/staff-api";
import type { StaffShift } from "~/lib/shift-helpers";
import {
  indexCreditingAbsenceByDay,
  isEffectiveAbsenceStatus,
  isHalfAbsenceBoundary,
  resolveTargetForDate,
  toIsoDayOfWeek,
} from "~/lib/staff-metrics-helpers";
import type {
  WeeklySummary,
  WorkDayLaborHint,
  WorkSessionEdit,
  WorkSessionHistory,
} from "~/lib/time-tracking-helpers";
import {
  balanceSessionEnd,
  formatDuration,
  indexWorkSessionMinutesByBerlinDate,
} from "~/lib/time-tracking-helpers";
import type { DayProjection } from "~/lib/time-tracking-helpers";

import {
  AdminSessionEditModal,
  type AdminEditMode,
} from "./admin-session-edit-modal";
import { formatSignedDuration } from "./staff-time-views";

const logger = createLogger({ component: "StaffSessionTable" });

const dayLabels = ["Mo", "Di", "Mi", "Do", "Fr", "Sa", "So"];

// Every cache that a correction or backfill in the admin edit modal makes
// stale. Beyond the sessions, absences and month aggregates: the daily
// projection, which since #2443 no longer carries the day's Soll alone but the
// server-computed Ist, Gutschrift and Saldo — exactly the numbers this save
// just moved. The account-range chart key
// ("staff-schedule-targets-account-<id>-…") is deliberately NOT matched: it is
// fetched with target_only=true and holds pure Soll, which no session edit can
// change. The id-less "time-tracking-schedule-targets-" / month-summary keys
// belong to the self-service portal and matter when the manager is editing
// their OWN days. useSWRAuth prefixes keys with the tenant slug
// ("phoenix:staff-history-…"), so a plain startsWith match would never fire —
// we use includes instead.
export function isStaleAfterSessionSave(
  key: unknown,
  staffId: string,
): boolean {
  return (
    typeof key === "string" &&
    (key.includes("staff-history-") ||
      key.includes("staff-absences-") ||
      key.includes("staff-month-summary-") ||
      key.includes(`staff-schedule-targets-${staffId}-`) ||
      key.includes("time-tracking-month-summary-") ||
      key.includes("time-tracking-schedule-targets-"))
  );
}

// Tooltip der beiden Zeit-Zellen auf einem Mehrblock-Tag (#2402).
const multiBlockBoundsHint =
  "Tagesgrenzen über alle Blöcke — die Zeit zwischen zwei Blöcken zählt nicht als Arbeitszeit. Die gearbeiteten Zeiten stehen je Block darunter.";

// Checkout des letzten Segments: der gespeicherte Wert, wenn er die gezeigten
// Minuten deckt, sonst das gekappte Ende. Ein offener Block behält null und
// rendert weiter als laufend.
function cappedCheckOut(
  stored: string | null,
  checkOut: Date | null,
  effectiveEnd: Date,
): string | null {
  if (stored === null || checkOut === null) return null;
  if (checkOut.getTime() <= effectiveEnd.getTime()) return stored;
  return effectiveEnd.toISOString();
}

// Zerlegt einen Block in seine Berliner Kalendertage. `now` ist der Zeitpunkt,
// gegen den ein laufender Block gemessen wird; `todayISO` der Tag, auf den er
// fällt — die Tabelle reicht ihn herein, statt ihn hier aus der Uhr zu ziehen,
// damit die Zerlegung beim Tageswechsel neu läuft (siehe `todayKey`).
function splitSessionByBerlinDate(
  session: StaffHistorySession,
  now: Date,
  todayISO: string,
): StaffHistorySession[] {
  const sourceDate = session.date.slice(0, 10);
  const yesterday = parseISODate(todayISO);
  yesterday.setDate(yesterday.getDate() - 1);
  // A genuinely open block can only span last night. Older open rows are
  // stale data awaiting cleanup; keep their historical table row intact
  // rather than fabricating one open segment for every intervening day.
  if (session.check_out_time === null && sourceDate < toISODate(yesterday)) {
    return [session];
  }
  const historySession: WorkSessionHistory = {
    id: session.id ?? "",
    staffId: "",
    date: session.date,
    status: session.status ?? "present",
    source: session.source,
    checkInTime: session.check_in_time,
    checkOutTime: session.check_out_time,
    breakMinutes: session.break_minutes,
    notes: session.notes ?? "",
    autoCheckedOut: session.auto_checked_out ?? false,
    createdBy: "",
    updatedBy: null,
    createdAt: "",
    updatedAt: "",
    netMinutes: session.net_minutes,
    isOvertime: false,
    isBreakCompliant: true,
    restPeriodWarning: null,
    breaks: (session.breaks ?? []).map((workBreak, index) => ({
      id: index.toString(),
      sessionId: session.id ?? "",
      startedAt: workBreak.started_at,
      endedAt: workBreak.ended_at ?? null,
      durationMinutes: 0,
      plannedEndTime: null,
    })),
    editCount: session.edit_count ?? 0,
    auditCount: session.audit_count ?? 0,
  };
  const valuesByDate = indexWorkSessionMinutesByBerlinDate(
    [historySession],
    now,
  );
  // Ein Block ohne volle Minute (gerade eingestempelt, Sekunden-Korrektur)
  // liefert keine Tageswerte. Er bleibt trotzdem als eigene Zeile stehen,
  // statt bis zur ersten vollen Minute aus der Tabelle zu verschwinden.
  if (valuesByDate.size === 0) return [session];

  const start = new Date(session.check_in_time);
  const checkOut = session.check_out_time
    ? new Date(session.check_out_time)
    : null;
  // Dasselbe Ende, aus dem die Minuten oben stammen: ein offener Block und
  // ein Checkout in der Zukunft werden gekappt. Vergliche man stattdessen mit
  // dem rohen Checkout, fände kein Segment seinen letzten Tag und die letzte
  // Zeile bekäme 23:59 statt der Zeit, an der die gezeigten Minuten enden.
  const end = balanceSessionEnd(
    { date: session.date, checkIn: start, checkOut },
    now,
  );

  return [...valuesByDate.entries()].map(([date, values], index) => {
    const isFirstDay = date === berlinTodayISO(start);
    const isLastDay = date === berlinTodayISO(end);
    const previousDay = parseISODate(date);
    previousDay.setDate(previousDay.getDate() - 1);
    const segmentStart = isFirstDay
      ? session.check_in_time
      : new Date(
          new Date(endOfBerlinDayISO(previousDay)).getTime() + 1_000,
        ).toISOString();
    // Der letzte Tag trägt den gespeicherten Checkout, solange der innerhalb
    // der Kappung liegt; ein offener Block bleibt offen (null), ein Checkout
    // jenseits der Kappung wird auf sie zurückgeholt.
    const segmentEnd = isLastDay
      ? cappedCheckOut(session.check_out_time, checkOut, end)
      : endOfBerlinDayISO(parseISODate(date));
    return {
      ...session,
      date,
      check_in_time: segmentStart,
      check_out_time: segmentEnd,
      net_minutes: values.netMinutes,
      break_minutes: values.breakMinutes,
      // Die Änderungshistorie hängt am Block, nicht am Segment: ein Nachtblock
      // steht an zwei Tagen, korrigiert wurde er aber einmal. Nur das erste
      // Segment führt sie, sonst klappen beide Tageszeilen dieselbe Liste auf
      // und dieselbe Korrektur sieht aus wie zwei. Die Fortsetzung behält
      // `edit_count`, damit auch dort erkennbar bleibt, dass die gezeigten
      // Minuten aus einem korrigierten Block stammen.
      audit_count: index === 0 ? session.audit_count : 0,
    };
  });
}

// Day-row table for the Zeiterfassung tab. Industry-standard layout: one
// row per calendar day, comparing Check-in, Check-out, Pause, Soll, Ist
// and Saldo. Status/Quelle/Hinweis pills carry the rest of the per-day
// metadata. Anomalies (auto-closed sessions, future ArbZG flags) are
// colour-coded so the admin can spot corrections at a glance.
//
// Rows with audit history (audit_count > 0) are clickable and expand
// inline to reveal an EditHistoryAccordion — same component and UX as
// the MA-side /time-tracking page.
//
// MVP Limitation tracker:
//   - admin edit-in-place (SquarePen action column) — Tranche 1b
//   - ArbZG-Hinweise (#4804): Pause zu kurz und mehr als 10 Std. je Tag,
//     mehr als 48 Std. je Woche — vom Server bewertet, nur Hinweise
export function StaffSessionTable({
  staffId,
  from,
  to,
  sessions,
  absences,
  absencesUnresolved,
  schedule,
  dailyProjection,
  dailyProjectionError,
  dailyProjectionPending,
  onRetryDailyProjection,
  holidays,
  closingDays,
  accountStartDate,
  accountStartDatePending,
  accountStartDateError,
  onRetryAccountStartDate,
  today,
  isAdminView,
  plannedShifts,
  onEditDay,
  onBackfillAbsence,
  fetchEdits,
  laborDays,
  weeklySummaries,
}: {
  readonly staffId: string;
  readonly from: Date;
  readonly to: Date;
  readonly sessions: readonly StaffHistorySession[];
  readonly absences?: readonly StaffAbsenceRow[];
  readonly absencesUnresolved?: boolean;
  readonly schedule: StaffSchedule | null;
  // Servergerechnete Tagesprojektion (#1842, #2443), keyed YYYY-MM-DD: Soll,
  // Gutschrift, Ist und Saldo je Tag, aus derselben Rechnung wie die
  // Monatskarte. Wenn gesetzt, ist das die Quelle für die Spalten Soll,
  // Gutschrift und Saldo.
  //
  // `schedule` beschreibt NUR den heute gültigen Plan; angewendet auf
  // vergangene Tage widerspricht er der Monatskarte, sobald jemand seine
  // Stunden ändert. Und der Saldo lässt sich hier nicht selbst ableiten: „Ist
  // minus Soll" kennt weder die Gutschrift eines Urlaubstags noch den vollen
  // Abzug eines Freizeitausgleichs, und beide Tage haben oft gar keine
  // Session, aus der die Zeile rechnen könnte (#2443).
  readonly dailyProjection?: ReadonlyMap<string, DayProjection>;
  // Fehler beim Laden der Projektion. Dann bleiben Soll, Gutschrift und Saldo
  // ungelöst ("?") statt auf `schedule` zurückzufallen: der heutige Plan als
  // historisches Soll auszugeben wäre eine stille Falschaussage, die der
  // Monatskarte widerspricht (die weiter mit datumsgültigen Versionen rechnet).
  // Der Fehler selbst (oder `true`), damit der Hinweis den Katalogtext mit
  // Wiederholen und Vorgangskennung zeigen kann (#2514).
  readonly dailyProjectionError?: unknown;
  readonly onRetryDailyProjection?: () => void;
  // Die Projektion für den SICHTBAREN Zeitraum wird noch geladen. Die Fetches
  // laufen mit keepPreviousData, `dailyProjection` hält nach einem Wechsel des
  // Zeitraums also noch die Keys des VORHERIGEN — die neuen Tage sind schlicht
  // nicht enthalten. Ohne dieses Flag fielen genau diese Tage auf `schedule`
  // (den heutigen Plan) zurück und zeigten kurzzeitig falsche historische
  // Soll-/Saldo-Werte, bis die Antwort eintrifft (#1842). Tage, die der
  // vorherige Zeitraum bereits aufgelöst hat, bleiben gültig: dasselbe Datum
  // liefert für dieselbe Person immer dieselben Werte.
  readonly dailyProjectionPending?: boolean;
  // Gesetzliche Feiertage im sichtbaren Zeitraum, keyed YYYY-MM-DD → Name
  // (#1418 3a). Feiertage tragen ein eigenes Status-Badge statt "Nicht
  // erfasst", und eine Session an einem Feiertag bekommt eine
  // Feiertagsarbeit-Warnung (§9 ArbZG). Das Soll dieser Tage kommt bereits
  // als 0 vom Server — die Map ist reine Anzeige, keine Rechengrundlage.
  readonly holidays?: ReadonlyMap<string, string>;
  // OGS-Schließtage im sichtbaren Zeitraum, keyed YYYY-MM-DD → Grund
  // (#1418 3b). Analog zu den Feiertagen: eigenes Status-Badge statt "Nicht
  // erfasst", Soll kommt bereits als 0 vom Server, die Map ist reine
  // Anzeige. Bei Überschneidung gewinnt der Feiertag (gesetzliche Grundlage
  // vor Schulentscheidung).
  readonly closingDays?: ReadonlyMap<string, string>;
  // `null` means no time-tracking config data is available; pending/error
  // distinguish whether that is temporary or a failed request. An empty
  // string means the loaded config has no explicit anchor. The distinction
  // matters for sessions with Soll 0: they are overtime on a planned day off,
  // but must not enter the Saldo before a mid-month account start.
  readonly accountStartDate: string | null;
  readonly accountStartDatePending: boolean;
  readonly accountStartDateError: unknown;
  readonly onRetryAccountStartDate?: () => void;
  readonly today: Date;
  readonly isAdminView: boolean;
  // Planned Dienstplan shifts for the visible range. When provided (admin
  // detail view, #1844), a "Plan" column shows the geplante Schicht next to the
  // Ist times so plan, actual and — via the audit expand — the deviation reason
  // read together. Omitted on the staff self-view. Does NOT feed the Soll/Saldo
  // math (that stays the Arbeitszeitmodell, #1842).
  readonly plannedShifts?: readonly StaffShift[];
  // Optional external edit handler. When provided, the SquarePen click
  // delegates to the caller instead of opening the bundled
  // AdminSessionEditModal. Used by /time-tracking where the MA-side modal
  // also handles absences (which the admin modal does not).
  readonly onEditDay?: (
    date: Date,
    session: StaffHistorySession | null,
    absence: StaffAbsenceRow | null,
  ) => void;
  // Admin: a past day without any entry can get an absence instead of work
  // time (#3258). Only offered when the caller can book absences.
  readonly onBackfillAbsence?: (date: Date, kind: "absence" | "sick") => void;
  // Audit-trail fetcher for the expanded row. Defaults to the admin route
  // (time_tracking:manage); the MA self-view passes the self-scoped
  // /api/time-tracking/{id}/edits fetcher so staff can read their own
  // Abweichungsgründe without manage permission (#1842 AC8).
  readonly fetchEdits?: (
    staffId: string,
    sessionId: string,
  ) => Promise<WorkSessionEdit[]>;
  // ArbZG-Sicht je Arbeitstag vom Server (#4804), nach Tag des Einstempelns.
  // Ein Nachtblock trägt seinen Hinweis deshalb am Tag, an dem er begann.
  readonly laborDays?: readonly WorkDayLaborHint[];
  // Wochensummen vom Server. Nur wenn gesetzt, schließt jede Woche mit einer
  // Summenzeile ab, die mehr als 48 Std. markiert. Der Aufrufer lädt dafür
  // ganze Kalenderwochen, auch über den Rand des sichtbaren Monats hinaus.
  readonly weeklySummaries?: readonly WeeklySummary[];
}) {
  // Die Segmentierung unten liest die Uhr: ein offener Block wird an ihr
  // gekappt, und ein Nachtblock bekommt sein zweites Segment erst, wenn der
  // Tag gewechselt hat. Deshalb hängt das Memo auch am aktuellen Berliner Tag
  // (`today` folgt dem Rollover, useBerlinToday) — eine Seite, die über
  // Mitternacht offen bleibt, zeigte den laufenden Block sonst weiter
  // ausschließlich am Vortag, solange die Sessions-Referenz dieselbe bleibt.
  const todayKey = toISODate(today);
  // A day can carry several work blocks since #2402 (Homeoffice-Vormittag,
  // OGS-Nachmittag), including a part of an overnight block. The lookup keeps
  // a list per Berlin calendar day in check-in order.
  const sessionsByDate = useMemo(() => {
    const map = new Map<string, StaffHistorySession[]>();
    for (const session of sessions) {
      for (const segment of splitSessionByBerlinDate(
        session,
        new Date(),
        todayKey,
      )) {
        const list = map.get(segment.date);
        if (list) list.push(segment);
        else map.set(segment.date, [segment]);
      }
    }
    for (const list of map.values()) {
      list.sort((a, b) =>
        (a.check_in_time ?? "").localeCompare(b.check_in_time ?? ""),
      );
    }
    return map;
  }, [sessions, todayKey]);
  const originalSessionsByID = useMemo(
    () =>
      new Map(
        sessions
          .filter((session): session is StaffHistorySession & { id: string } =>
            Boolean(session.id),
          )
          .map((session) => [session.id, session]),
      ),
    [sessions],
  );

  // Planned Dienstplan shifts per day for the "Plan" column (#1844). Absent
  // unless the caller passes plannedShifts (admin detail view).
  const showPlan = plannedShifts != null;
  const shiftsByDate = useMemo(() => {
    const map = new Map<string, StaffShift[]>();
    for (const shift of plannedShifts ?? []) {
      const list = map.get(shift.date);
      if (list) list.push(shift);
      else map.set(shift.date, [shift]);
    }
    for (const list of map.values()) {
      list.sort((a, b) => a.startTime.localeCompare(b.startTime));
    }
    return map;
  }, [plannedShifts]);
  const laborDaysByDate = useMemo(() => indexLaborDays(laborDays), [laborDays]);
  const weeklySummaryByKey = useMemo(
    () =>
      weeklySummaries
        ? new Map(
            weeklySummaries.map((week) => [
              `${week.year}-${week.weekNumber}`,
              week,
            ]),
          )
        : null,
    [weeklySummaries],
  );

  // Absences arrive as date ranges; expand to a per-day lookup so each row in
  // the table can ask "is this date covered by Krank/Urlaub/etc?". Mirrors the
  // MA-side logic in time-tracking/page.tsx (absenceMap).
  const absencesByDate = useMemo(() => {
    const map = new Map<string, StaffAbsenceRow>();
    if (!absences) return map;
    for (const absence of absences) {
      const start = absence.date_start.slice(0, 10);
      const end = absence.date_end.slice(0, 10);
      const startDate = new Date(`${start}T00:00:00`);
      const endDate = new Date(`${end}T00:00:00`);
      // Calendar days, not elapsed 24-hour blocks: a range across the March
      // clock change is an hour short and lost its last day.
      const dayCount = differenceInCalendarDays(endDate, startDate) + 1;
      for (let offset = 0; offset < dayCount; offset++) {
        const cursor = new Date(startDate);
        cursor.setDate(cursor.getDate() + offset);
        const key = toDateKey(cursor);
        // Existing entries win — the first absence for a day owns the badge.
        if (!map.has(key)) {
          map.set(key, absence);
        }
      }
    }
    return map;
  }, [absences]);

  // Welche Abwesenheit die Gutschrift eines Tages trägt, entscheidet die
  // Server-Regel (walkCreditedAbsenceDays): nur reported/approved zählen, und
  // bei Überschneidungen VERBRAUCHT die Abwesenheit mit der niedrigsten ID den
  // Tag. absencesByDate folgt dagegen der API-Reihenfolge (nach Startdatum)
  // und würde im Tooltip der Gutschrift-Zelle die falsche Abwesenheitsart
  // nennen, sobald sich zwei Abwesenheiten überschneiden.
  const creditAbsencesByDate = useMemo(
    () => indexCreditingAbsenceByDay(absences),
    [absences],
  );

  // Soll eines Tages, exakt so wie die Zeile es später anzeigt: server-
  // aufgelöst, sonst der aktuelle Plan — und ungelöst, solange der Fetch
  // läuft oder fehlgeschlagen ist (#1842).
  const projectionFailed = Boolean(dailyProjectionError);
  const projectionLoad = useApiLoadError();
  const accountStartLoad = useApiLoadError();
  const showProjectionError = projectionLoad.show;
  const clearProjectionError = projectionLoad.clear;
  useEffect(() => {
    if (dailyProjectionError) {
      void showProjectionError(dailyProjectionError, {
        object: "das Soll für diesen Zeitraum",
        retry: onRetryDailyProjection,
      });
    } else {
      clearProjectionError();
    }
  }, [
    dailyProjectionError,
    onRetryDailyProjection,
    showProjectionError,
    clearProjectionError,
  ]);
  const showAccountStartError = accountStartLoad.show;
  const clearAccountStartError = accountStartLoad.clear;
  useEffect(() => {
    if (accountStartDateError) {
      void showAccountStartError(accountStartDateError, {
        object: "die Einstellung zum Stundenkonto",
        retry: onRetryAccountStartDate,
      });
    } else {
      clearAccountStartError();
    }
  }, [
    accountStartDateError,
    onRetryAccountStartDate,
    showAccountStartError,
    clearAccountStartError,
  ]);

  const resolveDayTarget = useCallback(
    (day: Date) => {
      const resolved = dailyProjection?.get(toDateKey(day));
      const unresolved =
        resolved === undefined &&
        (projectionFailed || dailyProjectionPending === true);
      const planned = schedule ? resolveTargetForDate(schedule, day) : 0;
      return {
        unresolved,
        planned,
        displayed: resolved?.targetMinutes ?? (unresolved ? 0 : planned),
      };
    },
    [dailyProjection, projectionFailed, dailyProjectionPending, schedule],
  );

  // Sa/So erscheinen nur, wenn es dort etwas zu zeigen gibt (#1967). Ein
  // harter Wochenend-Filter versteckte Ferienbetreuung, Elternabende und
  // Wochenend-Schichten komplett, während Monatskarte und KPI-Karten sie
  // serverseitig weiter mitzählen — die Summe der sichtbaren Zeilen ergab
  // dann nicht mehr das ausgewiesene Ist. Feiertage zählen ebenfalls als
  // Inhalt, damit z. B. Oster- und Pfingstsonntag ihr Badge zeigen. Wirklich
  // leere Wochenenden bleiben raus, damit die Tabelle nicht aufgebläht wird.
  const days = useMemo(() => {
    const result: Date[] = [];
    const start = new Date(from);
    const dayCount = differenceInCalendarDays(to, start) + 1;
    for (let i = 0; i < dayCount; i++) {
      const d = new Date(start);
      d.setDate(d.getDate() + i);
      if (toIsoDayOfWeek(d) >= 5) {
        const key = toDateKey(d);
        const { unresolved, planned, displayed } = resolveDayTarget(d);
        // Fehlende Target-Daten sind kein Beleg dafür, dass der Tag leer ist:
        // solange das Soll ungelöst ist, entscheidet der aktuelle Plan über
        // die Sichtbarkeit. Ein vertraglich verplanter Samstag bleibt damit
        // sichtbar (mit „?“/„…“ im Soll) und korrigierbar, statt beim Laden
        // oder nach einem Fehler ganz zu verschwinden. Für die ANGEZEIGTEN
        // Werte bleibt der Plan tabu — nur fürs Ein-/Ausblenden zählt er.
        const hasContent =
          sessionsByDate.has(key) ||
          absencesByDate.has(key) ||
          (shiftsByDate.get(key)?.length ?? 0) > 0 ||
          holidays?.has(key) === true ||
          closingDays?.has(key) === true ||
          displayed > 0 ||
          (unresolved && planned > 0);
        if (!hasContent) continue;
      }
      result.push(d);
    }
    return result;
  }, [
    from,
    to,
    sessionsByDate,
    absencesByDate,
    shiftsByDate,
    holidays,
    closingDays,
    resolveDayTarget,
  ]);

  // Clicking a row with audit history opens an EditHistoryAccordion directly
  // below it. Mirrors the MA-side /time-tracking page so the audit-trail UX is
  // identical for both audiences. We deliberately avoid a modal here — all
  // per-day numbers (Soll, Ist, Saldo, …) are already visible in the row, so a
  // modal would only duplicate them.

  // Edit/nachtragen modal state. Lifted into the table because both the
  // SquarePen action button and the "Weitere Änderung vornehmen" button
  // inside the expanded audit accordion open the same modal.
  const [editModal, setEditModal] = useState<{
    mode: AdminEditMode;
    date: Date;
    session: StaffHistorySession | null;
  } | null>(null);

  // Scoped invalidation — used after a successful save to refresh the visible window
  // (week/month table), its daily projection, the cumulative range (KpiCards)
  // and the Monatskarte, whose Ist, Gutschriften, Saldo and Übertrag are all
  // derived from the very rows this modal just changed. Which keys those are:
  // isStaleAfterSessionSave above.
  const invalidate = useInvalidateMatching();
  const handleSaved = () => {
    void invalidate((key) => isStaleAfterSessionSave(key, staffId));
  };

  // One entry per visible day, with everything the cells below read.
  const rows = days.map((day, dayIndex) => {
    const key = toDateKey(day);
    const weekKey = isoWeekKey(day);
    const nextDay = days[dayIndex + 1];
    const weeklySummary = weeklySummaryByKey?.get(weekKey);
    const closesWeek =
      weeklySummary !== undefined &&
      (nextDay === undefined || isoWeekKey(nextDay) !== weekKey);
    const dow = toIsoDayOfWeek(day);
    const daySessions = sessionsByDate.get(key) ?? [];
    // Single-block days keep their familiar row; multi-block days (#2402)
    // aggregate on the day row and list each block in a sub-row underneath.
    const session = daySessions[0];
    const originalSession = session?.id
      ? (originalSessionsByID.get(session.id) ?? session)
      : session;
    const originalSessionDate = originalSession
      ? parseISODate(originalSession.date)
      : day;
    const absence = absencesByDate.get(key);
    const holidayName = holidays?.get(key);
    const closingReason = closingDays?.get(key);
    // Ein fehlgeschlagener ODER noch laufender Targets-Fetch darf NICHT auf
    // den aktuellen Plan zurückfallen (#1842): der Tag bleibt ungelöst, damit
    // die Tabelle keinen erfundenen Soll-/Saldo-Wert behauptet — weder
    // dauerhaft (Fehler) noch kurzzeitig beim Zeitraumwechsel (pending, stale
    // Map).
    const { unresolved: targetUnresolved, displayed: target } =
      resolveDayTarget(day);
    const isFuture = day > today;
    const ist = daySessions.reduce((sum, s) => sum + (s.net_minutes ?? 0), 0);
    const pause = daySessions.reduce(
      (sum, s) => sum + (s.break_minutes ?? 0),
      0,
    );
    // Saldo eines Tages ist Ist minus Soll — auch bei Soll 0 auf einem
    // geplanten freien Tag (#1967). Vor einem untermonatigen Stundenkonto-Start
    // zählt die Monatskarte allerdings weder Soll noch Ist. GetDailyTargets
    // liefert dort ebenfalls 0, kann diesen Fall also nicht allein von einem
    // freien Tag unterscheiden; dafür braucht die Zeile zusätzlich den
    // konfigurierten Starttag.
    const isBeforeAccountStart =
      accountStartDate !== null &&
      accountStartDate !== "" &&
      key.slice(0, 7) === accountStartDate.slice(0, 7) &&
      key < accountStartDate;
    const balanceUnresolved =
      targetUnresolved || (target === 0 && accountStartDatePending);
    // Gutschrift und Saldo kommen vom Server (#2443) — dieselbe Rechnung, die
    // die Monatskarte summiert. Ohne Projektion (Storybook, Tests ohne
    // Serverdaten) bleibt es beim alten Verhalten: nur Tage mit Session zeigen
    // Ist minus Soll.
    const projected = dailyProjection?.get(key);
    const credit = projected?.creditMinutes ?? 0;
    const delta = projected?.balanceMinutes ?? (session ? ist - target : 0);
    // Ein Tag ohne Soll, ohne Gutschrift und ohne Session hat nichts zu
    // verrechnen — dort bleibt die Spalte leer, statt ein „0min" zu behaupten.
    // Fehlt die Projektion für diesen Tag ganz, bleibt es beim alten Verhalten:
    // nur eine vorhandene Session trägt dann einen Saldo, denn ohne Serverwert
    // wäre jede Zahl an einem session-losen Tag geraten.
    const hasBalance =
      projected != null
        ? session != null || target > 0 || credit > 0
        : session != null;
    // Edit / nachtragen is available for past or present days, regardless of
    // whether a session already exists. On an absence-only day, however, only
    // a half-day boundary or a non-effective absence may receive additional
    // work: effective full-day absences already credit the complete target in
    // the monthly balance (#2361).
    //
    // Wochenendtage bekommen sie sehr wohl (#1967): eine Zeile ist nur
    // sichtbar, wenn dort real gearbeitet oder geplant wurde, und genau die
    // muss korrigierbar sein.
    const absenceIsHalfDay = absence
      ? isHalfAbsenceBoundary(
          absence,
          key,
          absence.date_start.slice(0, 10),
          absence.date_end.slice(0, 10),
        )
      : false;
    const absenceBlocksBackfill =
      absence != null &&
      isEffectiveAbsenceStatus(absence.status) &&
      !absenceIsHalfDay;
    return {
      key,
      day,
      dow,
      closesWeek,
      weeklySummary,
      daySessions,
      session,
      originalSession,
      originalSessionDate,
      hasMultipleBlocks: daySessions.length > 1,
      openSession: daySessions.find((s) => !s.check_out_time),
      lastSession: daySessions[daySessions.length - 1],
      absence,
      // Die Gutschrift stammt vom Server; ihr Tooltip folgt deshalb der
      // Server-Auswahl, nicht der Badge-Abwesenheit.
      creditAbsence: creditAbsencesByDate.get(key),
      holidayName,
      closingReason,
      targetUnresolved,
      target,
      isFuture,
      isToday: sameDay(day, today),
      isWeekend: dow >= 5,
      ist,
      pause,
      isBeforeAccountStart,
      balanceUnresolved,
      projected,
      credit,
      delta,
      hasBalance,
      status: computeRowStatus(
        daySessions,
        absence,
        key,
        holidayName,
        closingReason,
        target,
        isFuture,
      ),
      // Only rows with audit history are interactive — clicking an unchanged
      // row should not toggle anything since there is nothing to reveal.
      canExpand:
        session != null && daySessions.some((s) => (s.audit_count ?? 0) > 0),
      canEdit:
        isAdminView &&
        !isFuture &&
        (session != null || (!absencesUnresolved && !absenceBlocksBackfill)),
    };
  });
  type DayRow = (typeof rows)[number];

  // Bearbeiten öffnet entweder den Dialog der aufrufenden Seite oder den
  // mitgelieferten.
  const editBlock = (block: StaffHistorySession, absence?: StaffAbsenceRow) => {
    const originalBlock = block.id
      ? (originalSessionsByID.get(block.id) ?? block)
      : block;
    const originalBlockDate = parseISODate(originalBlock.date);
    if (onEditDay) {
      onEditDay(originalBlockDate, originalBlock, absence ?? null);
    } else {
      setEditModal({
        mode: "edit",
        date: originalBlockDate,
        session: originalBlock,
      });
    }
  };

  // Spalten nach dem Muster der Tagesansicht: Check-in, Check-out, Pause, Soll,
  // Ist und Saldo.
  const columns: DataTableColumn<DayRow>[] = [
    {
      key: "date",
      header: "Datum",
      className: "tabular-nums whitespace-nowrap",
      render: (row) => (
        <span className="text-gray-700">{formatShortDate(row.day)}</span>
      ),
    },
    {
      key: "weekday",
      header: "Tag",
      render: (row) => (
        <span className="text-gray-500">{dayLabels[row.dow]}</span>
      ),
    },
    ...(showPlan
      ? [
          {
            key: "plan",
            header: "Plan (Schicht)",
            headerClassName: "whitespace-nowrap",
            className: "tabular-nums whitespace-nowrap",
            render: (row: DayRow) => (
              <PlannedShiftCell shifts={shiftsByDate.get(row.key)} />
            ),
          },
        ]
      : []),
    // Auf einem Mehrblock-Tag sind diese beiden Zellen Tagesgrenzen, kein
    // durchgehender Zeitraum: zwischen erstem Check-in und letztem Check-out
    // liegen Lücken, die nicht als Arbeitszeit zählen. "ab"/"bis" sagt das an
    // der Zelle selbst — die tatsächlich gearbeiteten Intervalle stehen in den
    // Block-Unterzeilen.
    {
      key: "checkIn",
      header: "Check-in",
      headerClassName: "whitespace-nowrap",
      className: "tabular-nums whitespace-nowrap",
      render: (row) => (
        <span className="text-gray-700">
          {row.session?.check_in_time ? (
            row.hasMultipleBlocks ? (
              <span title={multiBlockBoundsHint}>
                <span className="mr-1 text-xs text-gray-400">ab</span>
                <span>{formatTimeOnly(row.session.check_in_time)}</span>
              </span>
            ) : (
              formatTimeOnly(row.session.check_in_time)
            )
          ) : (
            "–"
          )}
        </span>
      ),
    },
    {
      key: "checkOut",
      header: "Check-out",
      headerClassName: "whitespace-nowrap",
      className: "tabular-nums whitespace-nowrap",
      render: (row) => (
        <span className="text-gray-700">
          {row.openSession ? (
            <StatusBadge label="eingestempelt" tone="green" compact />
          ) : row.lastSession?.check_out_time ? (
            row.hasMultipleBlocks ? (
              <span title={multiBlockBoundsHint}>
                <span className="mr-1 text-xs text-gray-400">bis</span>
                <span>{formatTimeOnly(row.lastSession.check_out_time)}</span>
              </span>
            ) : (
              formatTimeOnly(row.lastSession.check_out_time)
            )
          ) : (
            "–"
          )}
        </span>
      ),
    },
    {
      key: "pause",
      header: "Pause",
      align: "right",
      className: "tabular-nums whitespace-nowrap",
      render: (row) => (
        <span className="text-gray-500">
          {row.session ? formatDuration(row.pause) : "–"}
        </span>
      ),
    },
    {
      key: "target",
      header: "Soll",
      align: "right",
      className: "tabular-nums whitespace-nowrap",
      render: (row) => (
        <span className="text-gray-500">
          {row.targetUnresolved ? (
            <span
              title={
                projectionFailed
                  ? "Soll konnte nicht geladen werden"
                  : "Soll wird geladen"
              }
            >
              {projectionFailed ? "?" : "…"}
            </span>
          ) : row.target > 0 || row.projected?.isOverride ? (
            formatDuration(row.target)
          ) : (
            "–"
          )}
          {!row.targetUnresolved && row.projected?.isOverride && (
            <span className="block text-xs text-gray-400">
              {row.closingReason
                ? "Sonderarbeitszeit · Schließtag"
                : "Sonderarbeitszeit"}
            </span>
          )}
        </span>
      ),
    },
    {
      key: "actual",
      header: "Ist",
      align: "right",
      className: "tabular-nums whitespace-nowrap",
      render: (row) => (
        <span className="font-medium text-gray-700">
          {row.session ? formatDuration(row.ist) : "–"}
        </span>
      ),
    },
    {
      key: "credit",
      header: "Gutschrift",
      headerTitle:
        "Zeit, die eine Abwesenheit dem Tagessoll gutschreibt: Krank, Urlaub, Fortbildung und Sonstige. Freizeitausgleich schreibt nichts gut.",
      align: "right",
      className: "tabular-nums whitespace-nowrap",
      render: (row) => (
        <span className="text-gray-500">
          {row.credit > 0 ? (
            <span title={creditHint(row.creditAbsence)}>
              {formatDuration(row.credit)}
            </span>
          ) : (
            "–"
          )}
        </span>
      ),
    },
    {
      key: "balance",
      header: "Saldo",
      align: "right",
      className: "tabular-nums whitespace-nowrap",
      render: (row) =>
        row.hasBalance &&
        !row.isFuture &&
        !row.balanceUnresolved &&
        !row.isBeforeAccountStart ? (
          <span className={deltaClass(row.delta)}>
            {formatSignedDuration(row.delta)}
          </span>
        ) : (
          "–"
        ),
    },
    {
      key: "status",
      header: "Status",
      // Die Tageszeile wiederholt keine Block-Details: gestapelte
      // Status-/Quelle-Badges nebeneinander suggerieren falsche Paare (OGS
      // neben App, obwohl der OGS-Block per NFC kam). Der Zähler verweist auf
      // die Block-Unterzeilen, die die Fakten je Block korrekt gepaart tragen.
      render: (row) =>
        row.hasMultipleBlocks ? (
          <StatusBadge label={`${row.daySessions.length} Blöcke`} tone="blue" />
        ) : (
          row.status && <RowStatusBadge status={row.status} />
        ),
    },
    {
      key: "source",
      header: "Quelle",
      render: (row) => <SourceBadges sessions={row.daySessions} />,
    },
    {
      key: "hint",
      header: "Hinweis",
      render: (row) => (
        <div className="flex items-center justify-between gap-3">
          <HintBadges
            sessions={row.daySessions}
            holidayName={row.holidayName}
            laborHints={dayLaborHints(laborDaysByDate.get(row.key))}
          />
        </div>
      ),
    },
    ...(isAdminView
      ? [
          {
            key: "actions",
            header: "Aktionen",
            headerSrOnly: true,
            render: (row: DayRow) =>
              row.canEdit && (
                // Zeilenaktion nur im Kebab der Zeile (BAUARTEN-SPEC Bauart 1
                // Regel 4). stopPropagation hält den Menüklick vom Zeilenklick
                // fern, der die Änderungshistorie auf- und zuklappt.
                <div
                  className="flex shrink-0 items-center"
                  onClick={(e) => e.stopPropagation()}
                  onKeyDown={(e) => e.stopPropagation()}
                  role="presentation"
                >
                  <OverflowMenu
                    ariaLabel={`Aktionen für ${formatShortDate(row.day)}`}
                    triggerSize="sm"
                    items={[
                      {
                        label:
                          row.session == null
                            ? "Eintrag nachtragen"
                            : row.hasMultipleBlocks
                              ? "Block nachtragen"
                              : "Eintrag bearbeiten",
                        icon: <SquarePen className="h-4 w-4" aria-hidden />,
                        onClick: () => {
                          // On a multi-block day the entry adds a further
                          // block; each existing block has its own menu in its
                          // sub-row.
                          const targetSession = row.hasMultipleBlocks
                            ? null
                            : (row.originalSession ?? null);
                          const targetDate = targetSession
                            ? row.originalSessionDate
                            : row.day;
                          if (onEditDay) {
                            onEditDay(
                              targetDate,
                              targetSession,
                              row.absence ?? null,
                            );
                          } else {
                            setEditModal({
                              mode:
                                targetSession == null ? "nachtragen" : "edit",
                              date: targetDate,
                              session: targetSession,
                            });
                          }
                        },
                      },
                      ...(row.session == null &&
                      row.absence == null &&
                      toISODate(row.day) < todayKey &&
                      onBackfillAbsence
                        ? [
                            {
                              label: "Abwesenheit nachtragen",
                              icon: (
                                <CalendarPlus className="h-4 w-4" aria-hidden />
                              ),
                              onClick: () =>
                                onBackfillAbsence(row.day, "absence"),
                            },
                            {
                              label: "Krankmeldung nachtragen",
                              icon: (
                                <Thermometer className="h-4 w-4" aria-hidden />
                              ),
                              onClick: () => onBackfillAbsence(row.day, "sick"),
                            },
                          ]
                        : []),
                    ]}
                  />
                </div>
              ),
          },
        ]
      : []),
  ];

  const renderSubRows = (
    row: DayRow,
    expanded: boolean,
  ): DataTableExtraRow[] => {
    const blockRows: DataTableExtraRow[] = row.hasMultipleBlocks
      ? row.daySessions.map((block, blockIndex) => ({
          key: `${row.key}-block-${block.id ?? blockIndex}`,
          // Gutschrift und Saldo gehören dem TAG, nicht dem einzelnen Block:
          // die beiden Zellen bleiben leer.
          cells: [
            {
              columnKey: "date",
              content: (
                <span className="whitespace-nowrap text-gray-400">
                  Block {blockIndex + 1}
                </span>
              ),
            },
            {
              columnKey: "checkIn",
              content: (
                <span className="text-gray-600 tabular-nums">
                  {block.check_in_time
                    ? formatTimeOnly(block.check_in_time)
                    : "–"}
                </span>
              ),
            },
            {
              columnKey: "checkOut",
              content: block.check_out_time ? (
                <span className="text-gray-600 tabular-nums">
                  {formatTimeOnly(block.check_out_time)}
                </span>
              ) : (
                <span className="text-moto-green-strong font-medium">
                  eingestempelt
                </span>
              ),
            },
            {
              columnKey: "pause",
              content: (
                <span className="tabular-nums">
                  {formatDuration(block.break_minutes ?? 0)}
                </span>
              ),
            },
            {
              columnKey: "actual",
              content: (
                <span className="whitespace-nowrap text-gray-600 tabular-nums">
                  {formatDuration(block.net_minutes ?? 0)}
                </span>
              ),
            },
            {
              columnKey: "status",
              content: (
                <RowStatusBadge
                  status={
                    block.status === "home_office"
                      ? { kind: "home-office" }
                      : { kind: "present" }
                  }
                />
              ),
            },
            {
              columnKey: "source",
              content: <SourceBadges sessions={[block]} />,
            },
            {
              columnKey: "hint",
              content: (
                <div className="flex items-center justify-between gap-3">
                  <HintBadges sessions={[block]} holidayName={undefined} />
                </div>
              ),
            },
            ...(isAdminView && row.canEdit
              ? [
                  {
                    columnKey: "actions",
                    content: (
                      <div className="flex shrink-0 items-center">
                        <OverflowMenu
                          ariaLabel={`Aktionen für Block ${blockIndex + 1}`}
                          triggerSize="sm"
                          items={[
                            {
                              label: `Block ${blockIndex + 1} bearbeiten`,
                              icon: (
                                <SquarePen className="h-4 w-4" aria-hidden />
                              ),
                              onClick: () => editBlock(block, row.absence),
                            },
                          ]}
                        />
                      </div>
                    ),
                  },
                ]
              : []),
          ],
        }))
      : [];
    if (!expanded) return blockRows;
    const auditRows: DataTableExtraRow[] = row.daySessions
      .filter((block) => (block.audit_count ?? 0) > 0)
      .map((block, blockIndex) => ({
        key: `${row.key}-audit-${block.id ?? blockIndex}`,
        content: (
          // Auf dem Telefon ist die Tabelle breiter als der Bildschirm. Die
          // Historie bleibt im sichtbaren Ausschnitt, sonst stehen Bearbeiter
          // und Knopf rechts außerhalb.
          <div className="max-md:sticky max-md:left-3 max-md:w-[calc(100vw-5.75rem)]">
            {row.hasMultipleBlocks && (
              <p className="mb-1 text-xs font-medium text-gray-500">
                {`Block ${row.daySessions.indexOf(block) + 1}${
                  block.check_in_time
                    ? ` · ${formatTimeOnly(block.check_in_time)}${
                        block.check_out_time
                          ? `–${formatTimeOnly(block.check_out_time)}`
                          : ""
                      }`
                    : ""
                }`}
              </p>
            )}
            <SessionEditHistory
              staffId={staffId}
              sessionId={block.id}
              fetchEdits={fetchEdits}
              onEdit={
                isAdminView ? () => editBlock(block, row.absence) : undefined
              }
            />
          </div>
        ),
      }));
    return [...blockRows, ...auditRows];
  };

  return (
    <div className="space-y-3">
      {projectionLoad.error ? (
        <div className="space-y-1">
          <LoadErrorAlert error={projectionLoad.error} />
          <p className="text-xs text-gray-500">Tage ohne Soll zeigen „?“.</p>
        </div>
      ) : null}
      {accountStartLoad.error ? (
        <div className="space-y-1">
          <LoadErrorAlert error={accountStartLoad.error} />
          <p className="text-xs text-gray-500">
            Salden vor dem Kontostart können deshalb von der Monatskarte
            abweichen.
          </p>
        </div>
      ) : null}
      <DataTable
        columns={columns}
        rows={rows}
        getRowKey={(row) => row.key}
        dense
        expandable={{
          canExpand: (row) => row.canExpand,
          rowLabel: (row, expanded) =>
            `${formatShortDate(row.day)}: Änderungshistorie ${expanded ? "schließen" : "öffnen"}`,
        }}
        renderSubRows={renderSubRows}
        summaryRow={(row) =>
          row.closesWeek
            ? {
                key: `${row.key}-week`,
                cells: weekSummaryCells(row.day, from, to, row.weeklySummary, {
                  leadingColumns: showPlan ? 7 : 6,
                }),
              }
            : null
        }
        rowTone={(row) =>
          row.isToday ? "today" : row.isWeekend ? "muted" : null
        }
        rowClassName={(row) => (row.isFuture ? "opacity-40" : "")}
        emptyState="In diesem Zeitraum gibt es keine Tage."
      />
      {!isAdminView && (
        <Alert
          type="info"
          message="Eigene Arbeitszeiten können auf der Seite Zeiterfassung bearbeitet werden."
        />
      )}
      {!onEditDay && editModal && (
        <AdminSessionEditModal
          isOpen
          mode={editModal.mode}
          staffId={staffId}
          date={editModal.date}
          session={editModal.session}
          onClose={() => setEditModal(null)}
          onSaved={handleSaved}
        />
      )}
    </div>
  );
}

// Lazy-loads the audit trail for one session when expanded. Kept in its own
// component so the fetch is scoped to the row and reset cleanly when the user
// expands a different day.
function SessionEditHistory({
  staffId,
  sessionId,
  onEdit,
  fetchEdits,
}: {
  readonly staffId: string;
  readonly sessionId: string | undefined;
  readonly onEdit?: () => void;
  readonly fetchEdits?: (
    staffId: string,
    sessionId: string,
  ) => Promise<WorkSessionEdit[]>;
}) {
  const [edits, setEdits] = useState<WorkSessionEdit[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [attempt, setAttempt] = useState(0);
  const load = useApiLoadError();
  const showLoadError = load.show;
  const clearLoadError = load.clear;

  useEffect(() => {
    if (sessionId == null) return;
    let cancelled = false;
    setIsLoading(true);
    setEdits([]);
    clearLoadError();
    const fetchSessionEdits =
      fetchEdits ??
      ((sid: string, sess: string) =>
        staffSessionEditsService.getEdits(sid, sess));
    fetchSessionEdits(staffId, sessionId)
      .then((result) => {
        if (!cancelled) setEdits(result);
      })
      .catch((error: unknown) => {
        logger.error("session_edits_fetch_failed", {
          staff_id: staffId,
          session_id: sessionId,
          error: error instanceof Error ? error.message : String(error),
        });
        if (cancelled) return;
        void showLoadError(error, {
          object: "die Liste der Änderungen",
          retry: () => setAttempt((current) => current + 1),
        });
      })
      .finally(() => {
        if (!cancelled) setIsLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [staffId, sessionId, fetchEdits, attempt, showLoadError, clearLoadError]);

  // Ein Ladefehler steht an Stelle der Liste, nicht als „keine Änderungen“.
  if (load.error) {
    return <LoadErrorAlert error={load.error} />;
  }
  return (
    <EditHistoryAccordion edits={edits} isLoading={isLoading} onEdit={onEdit} />
  );
}

// Row-Status drückt den Arbeitsmodus oder ein Fehlen aus. Vor Ort vs.
// Homeoffice ist Mitarbeiter-Intent. Krank/Urlaub/Fortbildung kommen aus
// active.staff_absences und überschreiben "Nicht erfasst", wenn der Tag durch
// eine Abwesenheit abgedeckt ist. "Nicht erfasst" bleibt für Werktage ohne
// Session UND ohne Absence (echte Lücke im Audit-Trail). Auto-Close- und
// Edit-Hinweise gehören NICHT hier rein — die fließen in die Quelle-Spalte
// (Issue #1368).
type RowStatus =
  | { kind: "present" }
  | { kind: "home-office" }
  | {
      kind: "absence";
      absenceType: string;
      /** The school's own Abwesenheitsart wording, "" for a standard type. */
      absenceTypeLabel: string;
      halfDay: boolean;
    }
  | { kind: "holiday"; name: string }
  | { kind: "closing"; reason: string }
  | { kind: "missing" };

// Renders the planned Dienstplan window(s) for one day: each active shift as
// "HH:MM–HH:MM", cancelled shifts struck through as "entfällt". Split shifts
// stack. "–" when nothing is planned (#1844).
function PlannedShiftCell({
  shifts,
}: {
  readonly shifts: readonly StaffShift[] | undefined;
}) {
  if (!shifts || shifts.length === 0) {
    return <span className="text-gray-300">–</span>;
  }
  return (
    <div className="space-y-0.5">
      {shifts.map((shift) =>
        shift.cancelled ? (
          <div key={shift.id} className="text-gray-400">
            <span className="line-through">
              {shift.startTime}–{shift.endTime}
            </span>{" "}
            <span className="text-xs">entfällt</span>
          </div>
        ) : (
          <div key={shift.id} className="text-gray-600">
            {shift.startTime}–{shift.endTime}
          </div>
        ),
      )}
    </div>
  );
}

function computeRowStatus(
  sessions: readonly StaffHistorySession[],
  absence: StaffAbsenceRow | undefined,
  dateKey: string,
  holidayName: string | undefined,
  closingReason: string | undefined,
  target: number,
  isFuture: boolean,
): RowStatus | null {
  // Mehrblock-Tage rendern in der Statuszelle den Block-Zähler statt eines
  // RowStatus (siehe Tageszeile) — hier zählt nur, OB gearbeitet wurde.
  if (sessions.length > 0) {
    if (sessions.some((s) => s.status === "home_office")) {
      return { kind: "home-office" };
    }
    return { kind: "present" };
  }
  // Feiertag wins over absence and "missing": the day has no Soll for
  // EVERYONE, so neither a Krank badge nor "Nicht erfasst" describes it
  // (#1418 3a). A session on a holiday still shows present + the
  // Feiertagsarbeit hint.
  if (holidayName) {
    return { kind: "holiday", name: holidayName };
  }
  // Schließtag behaves like a holiday (Soll 0 for everyone, #1418 3b) but
  // ranks below it: on overlap the gesetzliche Feiertag names the day. A
  // Sonderarbeitszeit (#3259) can give the day a Soll again; then absence and
  // „Nicht erfasst" must show, not the closure.
  if (closingReason && target === 0) {
    return { kind: "closing", reason: closingReason };
  }
  // Absence wins over "missing" so an admin sees Krank/Urlaub instead of a
  // misleading "Nicht erfasst" badge (the MA-side already does this).
  if (absence) {
    const startDate = absence.date_start.slice(0, 10);
    const endDate = absence.date_end.slice(0, 10);
    return {
      kind: "absence",
      absenceType: absence.absence_type,
      absenceTypeLabel: absence.absence_type_label ?? "",
      halfDay: isHalfAbsenceBoundary(absence, dateKey, startDate, endDate),
    };
  }
  if (target > 0 && !isFuture) {
    return { kind: "missing" };
  }
  return null;
}

// Every fixed-outcome pill is the kit StatusBadge; only the absence pill stays
// StatusColorBadge because its colour is data (ABSENCE_TYPE_HEX), not one of the
// five semantic tones. `title` lives on a wrapper — StatusBadge takes no
// tooltip prop and does not need one.
function RowStatusBadge({ status }: { readonly status: RowStatus }) {
  if (status.kind === "home-office") {
    // Not StatusBadge tone="blue": that resolves to the same blue.soft/base/
    // strong triple getLocationBadgeTone returns for ABSENCE_TYPE_HEX.vacation
    // (#5080D8), so "Homeoffice" and "Urlaub" rendered as identical pills in
    // this very column. #0EA5E9 is the hue staff-helpers already uses for
    // Homeoffice.
    return (
      <StatusColorBadge
        label="Unterwegs"
        color={MOTO_COLOR_PALETTE.timeTracking.base}
      />
    );
  }
  if (status.kind === "present") {
    return <StatusBadge label="OGS" tone="green" />;
  }
  if (status.kind === "holiday") {
    return <StatusBadge label="Feiertag" tone="orange" title={status.name} />;
  }
  if (status.kind === "closing") {
    return <StatusBadge label="Schließtag" tone="gray" title={status.reason} />;
  }
  if (status.kind === "absence") {
    const absenceLabel =
      status.absenceTypeLabel ||
      (ABSENCE_TYPE_LABEL[status.absenceType] ?? ABSENCE_TYPE_LABEL.other!);
    const label = status.halfDay
      ? `${absenceLabel} · Halber Tag`
      : absenceLabel;
    const color =
      ABSENCE_TYPE_HEX[status.absenceType] ?? ABSENCE_TYPE_HEX.other!;
    return <StatusColorBadge label={label} color={color} />;
  }
  return <StatusBadge label="Nicht erfasst" tone="gray" />;
}

// Quelle-Badges sind der reine Origin der Blöcke: über welchen Kanal sie
// entstanden sind. Korrekturen und Auto-Checkouts sind orthogonal und landen
// in der Hinweis-Spalte (HintBadges), damit z.B. eine NFC-Session mit
// nachträglicher Korrektur weiterhin als "NFC" erkennbar bleibt.
//
// Ein Mehrblock-Tag (#2402) zeigt hier nur dann ein Badge, wenn ALLE Blöcke
// über denselben Kanal kamen. Bei gemischten Quellen bleibt die Tageszeile
// leer: zwei gestapelte Badges neben den Status-Badges lesen sich als Paare
// ("OGS · App"), die so nie gestempelt wurden — die korrekte Zuordnung steht
// in den Block-Unterzeilen.
function SourceBadges({
  sessions,
}: {
  readonly sessions: readonly StaffHistorySession[];
}) {
  if (sessions.length === 0) {
    return <span className="text-xs text-gray-300">–</span>;
  }
  const sources = [...new Set(sessions.map((s) => s.source ?? "app"))];
  if (sources.length > 1) {
    return <span className="text-xs text-gray-300">–</span>;
  }
  const source = sources[0];
  if (source === "nfc") {
    return <StatusBadge label="NFC" tone="blue" />;
  }
  if (source === "unknown") {
    // Pre-Migration Legacy-Row (Tristan PR #1398).
    return <span className="text-xs text-gray-400">–</span>;
  }
  return <StatusBadge label="App" tone="gray" />;
}

const NO_LABOR_HINTS: readonly LaborHint[] = [];

// Hinweis-Spalte für orthogonale System- und Korrektur-Hinweise. Mehrere
// Pills können gleichzeitig sichtbar sein (z.B. NFC-Session mit Auto-Checkout
// UND nachträglicher Korrektur).
function HintBadges({
  sessions,
  holidayName,
  laborHints = NO_LABOR_HINTS,
}: {
  readonly sessions: readonly StaffHistorySession[];
  readonly holidayName?: string;
  readonly laborHints?: readonly LaborHint[];
}) {
  if (sessions.length === 0) {
    return <span className="text-xs text-gray-300">–</span>;
  }
  const pills: {
    key: string;
    label: string;
    title?: string;
    tone: StatusBadgeTone;
  }[] = [];
  // Arbeit an einem gesetzlichen Feiertag ist nach §9 ArbZG grundsätzlich
  // untersagt (OGS-Ausnahmen nach §10 möglich) — Warnung, kein Block
  // (#1418 3a/3f).
  if (holidayName) {
    pills.push({
      key: "holiday-work",
      label: "Feiertagsarbeit",
      title: `Arbeitszeit an ${holidayName}: nach §9 ArbZG nur ausnahmsweise zulässig (ggf. Sondergenehmigung erforderlich).`,
      tone: "red",
    });
  }
  if (sessions.some((s) => (s.edit_count ?? 0) > 0)) {
    pills.push({ key: "edited", label: "Manuell korrigiert", tone: "orange" });
  }
  if (sessions.some((s) => s.auto_checked_out)) {
    pills.push({
      key: "auto-checkout",
      label: "Auto-Checkout",
      tone: "warning",
    });
  }
  if (pills.length === 0 && laborHints.length === 0) {
    return <span className="text-xs text-gray-300">–</span>;
  }
  return (
    <div className="flex flex-wrap gap-1">
      {pills.map((pill) => (
        <StatusBadge
          key={pill.key}
          label={pill.label}
          tone={pill.tone}
          title={pill.title}
        />
      ))}
      <LaborHintList hints={laborHints} />
    </div>
  );
}

// Saldo values sit as plain text on a white row — identical to KpiCard in
// staff-time-views, which prices the same figure on /staff/[id]. The -strong
// ramp steps are used deliberately even though they read brownish: amber only
// becomes legible on white once it is that dark. The previous amber text token
// (#D97706) sat at 3.19:1 and missed AA for normal text, moto-amber-strong
// (#92400E) reaches 7.09:1. Green is the brand green because it is legible and
// unambiguous at this size.
// Die Gutschrift-Zelle nennt die Abwesenheit, aus der sie stammt — sonst steht
// dort eine Zahl ohne Herkunft.
function creditHint(absence: StaffAbsenceRow | undefined): string {
  if (!absence) return "Gutschrift auf das Tagessoll";
  const label =
    absence.absence_type_label ||
    (ABSENCE_TYPE_LABEL[absence.absence_type] ?? ABSENCE_TYPE_LABEL.other!);
  return `${label}: Das Tagessoll wird gutgeschrieben.`;
}

function deltaClass(delta: number): string {
  if (delta > 0) return "text-moto-amber-strong font-medium";
  if (delta < -15) return "text-moto-red-strong font-medium";
  if (delta < 0) return "font-medium text-gray-500";
  return "text-moto-green-strong font-medium";
}

// Wochensumme am Ende jeder Kalenderwoche (#4804). Die Summe kommt vom
// Server und gilt für die GANZE Woche: im Monatsblick reicht die erste oder
// letzte Woche über den Monatsrand, und die 48-Std.-Grenze gilt je Woche,
// nicht je sichtbarem Ausschnitt.
function weekSummaryCells(
  day: Date,
  from: Date,
  to: Date,
  summary: WeeklySummary | undefined,
  { leadingColumns }: { readonly leadingColumns: number },
) {
  const monday = new Date(day);
  monday.setDate(monday.getDate() - toIsoDayOfWeek(day));
  const sunday = new Date(monday);
  sunday.setDate(sunday.getDate() + 6);
  const reachesBeyond =
    toDateKey(monday) < toDateKey(from) || toDateKey(sunday) > toDateKey(to);
  return [
    {
      columnKey: "date",
      colSpan: leadingColumns,
      content: (
        <span
          title={
            reachesBeyond
              ? "Summe der ganzen Kalenderwoche, auch der Tage außerhalb dieser Ansicht"
              : undefined
          }
        >
          {`Summe KW ${isoWeekNumber(toDateKey(day))}`}
          {reachesBeyond ? " (ganze Woche)" : ""}
        </span>
      ),
    },
    {
      columnKey: "actual",
      content: (
        <span className="whitespace-nowrap text-gray-700 tabular-nums">
          {summary ? formatDuration(summary.totalNetMinutes) : "–"}
        </span>
      ),
    },
    // Der Wochenhinweis steht direkt neben der Summe, über die freien
    // Tagesspalten, damit er nicht in der schmalen Hinweis-Spalte am Rand
    // abgeschnitten wird.
    {
      columnKey: "credit",
      colSpan: 5,
      content: <LaborHintList hints={weekLaborHints(summary)} />,
    },
  ];
}

// Hinweise zu Pausen und Höchstarbeitszeit (#4804): ein kurzes Badge und die
// Zahlen als umbrechende Zeile darunter. Als ein langes Badge liefe der Text
// in der letzten Tabellenspalte aus dem sichtbaren Bereich.
function LaborHintList({ hints }: { readonly hints: readonly LaborHint[] }) {
  if (hints.length === 0) return null;
  return (
    <>
      {hints.map((hint) => (
        <span key={hint.key} className="flex flex-col items-start gap-0.5">
          <StatusBadge label={hint.label} tone="warning" title={hint.title} />
          {hint.detail ? (
            <span className="text-moto-amber-strong text-xs">
              {hint.detail}
            </span>
          ) : null}
        </span>
      ))}
    </>
  );
}

// ISO-Jahr und -Woche eines Tags, im Format der Server-Wochensummen. Das Jahr
// ist das des Donnerstags der Woche, nicht das Kalenderjahr des Tags.
function isoWeekKey(d: Date): string {
  const thursday = new Date(d);
  thursday.setDate(thursday.getDate() + 3 - toIsoDayOfWeek(d));
  return `${thursday.getFullYear()}-${isoWeekNumber(toDateKey(d))}`;
}

function toDateKey(d: Date): string {
  const yyyy = d.getFullYear();
  const mm = String(d.getMonth() + 1).padStart(2, "0");
  const dd = String(d.getDate()).padStart(2, "0");
  return `${yyyy}-${mm}-${dd}`;
}

function sameDay(a: Date, b: Date): boolean {
  return (
    a.getFullYear() === b.getFullYear() &&
    a.getMonth() === b.getMonth() &&
    a.getDate() === b.getDate()
  );
}

function formatShortDate(d: Date): string {
  const dd = String(d.getDate()).padStart(2, "0");
  const mm = String(d.getMonth() + 1).padStart(2, "0");
  return `${dd}.${mm}.`;
}

// School time, whatever the viewer's zone (#6293).
function formatTimeOnly(iso: string): string {
  return berlinClockFromISO(iso);
}
