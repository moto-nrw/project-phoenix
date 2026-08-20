"use client";

import { ChevronRight, SquarePen } from "lucide-react";
import { Fragment, useCallback, useEffect, useMemo, useState } from "react";
import { useSWRConfig } from "swr";

import { EditHistoryAccordion } from "~/components/time-tracking/edit-history-accordion";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import {
  StatusBadge,
  type StatusBadgeTone,
} from "~/components/ui/status-badge";
import { StatusDotBadge } from "~/components/ui/status-dot-badge";
import { MOTO_COLOR_PALETTE } from "~/lib/location-helper";
import { ABSENCE_TYPE_HEX, ABSENCE_TYPE_LABEL } from "~/lib/absence-helpers";
import {
  berlinTodayISO,
  endOfBerlinDayISO,
  parseISODate,
  toISODate,
} from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import type {
  StaffAbsenceRow,
  StaffHistorySession,
  StaffSchedule,
} from "~/lib/staff-api";
import { staffSessionEditsService } from "~/lib/staff-api";
import type { StaffShift } from "~/lib/shift-helpers";
import {
  isEffectiveAbsenceStatus,
  isHalfAbsenceBoundary,
  resolveTargetForDate,
  toIsoDayOfWeek,
} from "~/lib/staff-metrics-helpers";
import type {
  WorkSessionEdit,
  WorkSessionHistory,
} from "~/lib/time-tracking-helpers";
import {
  formatDuration,
  indexWorkSessionMinutesByBerlinDate,
} from "~/lib/time-tracking-helpers";

import {
  AdminSessionEditModal,
  type AdminEditMode,
} from "./admin-session-edit-modal";
import { formatSignedDuration } from "./staff-time-views";

const logger = createLogger({ component: "StaffSessionTable" });

// Number of columns in the table — the inline edit-history row spans all
// of them. Keep in sync with the <thead> below.
const COLUMN_COUNT = 11;

const dayLabels = ["Mo", "Di", "Mi", "Do", "Fr", "Sa", "So"];

// Tooltip der beiden Zeit-Zellen auf einem Mehrblock-Tag (#2402).
const multiBlockBoundsHint =
  "Tagesgrenzen über alle Blöcke — die Zeit zwischen zwei Blöcken zählt nicht als Arbeitszeit. Die gearbeiteten Zeiten stehen je Block darunter.";

function splitSessionByBerlinDate(
  session: StaffHistorySession,
  now: Date,
): StaffHistorySession[] {
  const sourceDate = session.date.slice(0, 10);
  const yesterday = parseISODate(berlinTodayISO(now));
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
      endedAt: workBreak.ended_at,
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
  const start = new Date(session.check_in_time);
  const end = new Date(session.check_out_time ?? now);

  return [...valuesByDate.entries()].map(([date, values]) => {
    const isFirstDay = date === berlinTodayISO(start);
    const isLastDay = date === berlinTodayISO(end);
    const previousDay = parseISODate(date);
    previousDay.setDate(previousDay.getDate() - 1);
    const segmentStart = isFirstDay
      ? session.check_in_time
      : new Date(
          new Date(endOfBerlinDayISO(previousDay)).getTime() + 1_000,
        ).toISOString();
    const segmentEnd = isLastDay
      ? session.check_out_time
      : endOfBerlinDayISO(parseISODate(date));
    return {
      ...session,
      date,
      check_in_time: segmentStart,
      check_out_time: segmentEnd,
      net_minutes: values.netMinutes,
      break_minutes: values.breakMinutes,
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
//   - ArbZG warning chips - Tranche 3 (#896)
export function StaffSessionTable({
  staffId,
  from,
  to,
  sessions,
  absences,
  absencesUnresolved,
  schedule,
  dailyTargets,
  dailyTargetsError,
  dailyTargetsPending,
  holidays,
  closingDays,
  accountStartDate,
  accountStartDatePending,
  accountStartDateError,
  today,
  isAdminView,
  plannedShifts,
  onEditDay,
  fetchEdits,
}: {
  readonly staffId: string;
  readonly from: Date;
  readonly to: Date;
  readonly sessions: readonly StaffHistorySession[];
  readonly absences?: readonly StaffAbsenceRow[];
  readonly absencesUnresolved?: boolean;
  readonly schedule: StaffSchedule | null;
  // Server-resolved Soll je Tag (#1842), keyed YYYY-MM-DD. Wenn gesetzt, ist
  // das die Quelle für die Soll-Spalte: `schedule` beschreibt NUR den heute
  // gültigen Plan, angewendet auf vergangene Tage widerspricht er der
  // Monatskarte, sobald jemand seine Stunden ändert.
  readonly dailyTargets?: ReadonlyMap<string, number>;
  // Fehler beim Laden der Targets. Dann bleibt die Soll-Spalte ungelöst ("?")
  // statt auf `schedule` zurückzufallen: der heutige Plan als historisches
  // Soll auszugeben wäre eine stille Falschaussage, die der Monatskarte
  // widerspricht (die weiter mit datumsgültigen Versionen rechnet).
  readonly dailyTargetsError?: boolean;
  // Die Targets für den SICHTBAREN Zeitraum werden noch geladen. Die Fetches
  // laufen mit keepPreviousData, `dailyTargets` hält nach einem Wechsel des
  // Zeitraums also noch die Keys des VORHERIGEN — die neuen Tage sind schlicht
  // nicht enthalten. Ohne dieses Flag fielen genau diese Tage auf `schedule`
  // (den heutigen Plan) zurück und zeigten kurzzeitig falsche historische
  // Soll-/Saldo-Werte, bis die Antwort eintrifft (#1842). Tage, die der
  // vorherige Zeitraum bereits aufgelöst hat, bleiben gültig: dasselbe Datum
  // liefert für dieselbe Person immer dasselbe Soll.
  readonly dailyTargetsPending?: boolean;
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
  readonly accountStartDateError: boolean;
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
  // Audit-trail fetcher for the expanded row. Defaults to the admin route
  // (time_tracking:manage); the MA self-view passes the self-scoped
  // /api/time-tracking/{id}/edits fetcher so staff can read their own
  // Abweichungsgründe without manage permission (#1842 AC8).
  readonly fetchEdits?: (
    staffId: string,
    sessionId: string,
  ) => Promise<WorkSessionEdit[]>;
}) {
  // A day can carry several work blocks since #2402 (Homeoffice-Vormittag,
  // OGS-Nachmittag), including a part of an overnight block. The lookup keeps
  // a list per Berlin calendar day in check-in order.
  const sessionsByDate = useMemo(() => {
    const map = new Map<string, StaffHistorySession[]>();
    for (const session of sessions) {
      for (const segment of splitSessionByBerlinDate(session, new Date())) {
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
  }, [sessions]);

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
  const columnCount = COLUMN_COUNT + (showPlan ? 1 : 0);

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
      const dayCount =
        Math.floor((endDate.getTime() - startDate.getTime()) / 86_400_000) + 1;
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

  // Soll eines Tages, exakt so wie die Zeile es später anzeigt: server-
  // aufgelöst, sonst der aktuelle Plan — und ungelöst, solange der Fetch
  // läuft oder fehlgeschlagen ist (#1842).
  const resolveDayTarget = useCallback(
    (day: Date) => {
      const resolved = dailyTargets?.get(toDateKey(day));
      const unresolved =
        resolved === undefined &&
        (dailyTargetsError === true || dailyTargetsPending === true);
      const planned = schedule ? resolveTargetForDate(schedule, day) : 0;
      return {
        unresolved,
        planned,
        displayed: resolved ?? (unresolved ? 0 : planned),
      };
    },
    [dailyTargets, dailyTargetsError, dailyTargetsPending, schedule],
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
    const dayCount =
      Math.floor((to.getTime() - start.getTime()) / 86_400_000) + 1;
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

  // Inline-expandable row: clicking a row toggles an EditHistoryAccordion in
  // a second <tr> directly below. Mirrors the MA-side /time-tracking page so
  // the audit-trail UX is identical for both audiences. We deliberately avoid
  // a modal here — all per-day numbers (Soll, Ist, Saldo, …) are already
  // visible in the row, so a modal would only duplicate them.
  const [expandedKey, setExpandedKey] = useState<string | null>(null);

  // Edit/nachtragen modal state. Lifted into the table because both the
  // SquarePen action button and the "Weitere Änderung vornehmen" button
  // inside the expanded audit accordion open the same modal.
  const [editModal, setEditModal] = useState<{
    mode: AdminEditMode;
    date: Date;
    session: StaffHistorySession | null;
  } | null>(null);

  // SWR mutate — used after a successful save to refresh the visible window
  // (week/month table), the cumulative range (KpiCards) and the Monatskarte,
  // whose Ist, Gutschriften, Saldo and Übertrag are all derived from the very
  // rows this modal just changed. useSWRAuth prefixes keys with the tenant
  // slug ("phoenix:staff-history-…"), so a plain startsWith match would never
  // fire — we use includes instead.
  const { mutate } = useSWRConfig();
  const handleSaved = () => {
    void mutate(
      (key) =>
        typeof key === "string" &&
        (key.includes("staff-history-") ||
          key.includes("staff-absences-") ||
          key.includes("staff-month-summary-") ||
          key.includes("time-tracking-month-summary-")),
    );
  };

  return (
    <div className="space-y-3">
      {dailyTargetsError === true && (
        <Alert
          type="warning"
          message="Das Soll konnte für diesen Zeitraum nicht geladen werden. Betroffene Tage zeigen „?“ statt eines Soll- und Saldo-Werts — der aktuelle Arbeitszeitplan wird bewusst nicht auf vergangene Tage angewendet."
        />
      )}
      {accountStartDateError && (
        <Alert
          type="error"
          message="Der Stundenkonto-Start konnte nicht geladen werden. Soweit das Soll geladen wurde, werden Salden an Tagen ohne Soll weiterhin angezeigt; vor dem Kontostart können sie aber von der Monatskarte abweichen. Bitte die Seite neu laden."
        />
      )}
      {/* Radius, Rand und Ausschnitt bleiben auf der Fläche, gescrollt wird im
          inneren Container. Sonst zeichnet der Browser die Scrollleiste quer
          durch die abgerundeten Ecken und über den unteren Rand hinaus. */}
      <div className="max-w-full overflow-hidden rounded-2xl border border-gray-100">
        <div className="max-w-full overflow-x-auto">
          <table className="w-full min-w-[960px] text-left text-sm">
            <thead className="bg-gray-50 text-xs font-semibold tracking-wider text-gray-500 uppercase">
              <tr>
                <th className="px-3 py-3">Datum</th>
                <th className="px-3 py-3">Tag</th>
                {showPlan && (
                  <th className="px-3 py-3 tabular-nums">Plan (Schicht)</th>
                )}
                <th className="px-3 py-3 tabular-nums">Check-in</th>
                <th className="px-3 py-3 tabular-nums">Check-out</th>
                <th className="px-3 py-3 text-right tabular-nums">Pause</th>
                <th className="px-3 py-3 text-right tabular-nums">Soll</th>
                <th className="px-3 py-3 text-right tabular-nums">Ist</th>
                <th className="px-3 py-3 text-right tabular-nums">Saldo</th>
                <th className="px-3 py-3">Status</th>
                <th className="px-3 py-3">Quelle</th>
                <th className="px-3 py-3">Hinweis</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {days.map((day) => {
                const key = toDateKey(day);
                const dow = toIsoDayOfWeek(day);
                const daySessions = sessionsByDate.get(key) ?? [];
                // Single-block days keep their familiar row; multi-block days
                // (#2402) aggregate on the day row and list each block in a
                // sub-row underneath.
                const session = daySessions[0];
                const hasMultipleBlocks = daySessions.length > 1;
                const openSession = daySessions.find((s) => !s.check_out_time);
                const lastSession = daySessions[daySessions.length - 1];
                const absence = absencesByDate.get(key);
                const holidayName = holidays?.get(key);
                const closingReason = closingDays?.get(key);
                // Ein fehlgeschlagener ODER noch laufender Targets-Fetch darf
                // NICHT auf den aktuellen Plan zurückfallen (#1842): der Tag
                // bleibt ungelöst, damit die Tabelle keinen erfundenen
                // Soll-/Saldo-Wert behauptet — weder dauerhaft (Fehler) noch
                // kurzzeitig beim Zeitraumwechsel (pending, stale Map).
                const { unresolved: targetUnresolved, displayed: target } =
                  resolveDayTarget(day);
                const isFuture = day > today;
                const isToday = sameDay(day, today);
                const ist = daySessions.reduce(
                  (sum, s) => sum + (s.net_minutes ?? 0),
                  0,
                );
                const pause = daySessions.reduce(
                  (sum, s) => sum + (s.break_minutes ?? 0),
                  0,
                );
                // Saldo eines Tages ist Ist minus Soll — auch bei Soll 0 auf
                // einem geplanten freien Tag (#1967). Vor einem untermonatigen
                // Stundenkonto-Start zählt die Monatskarte allerdings weder
                // Soll noch Ist. GetDailyTargets liefert dort ebenfalls 0, kann
                // diesen Fall also nicht allein von einem freien Tag
                // unterscheiden; dafür braucht die Zeile zusätzlich den
                // konfigurierten Starttag.
                const isBeforeAccountStart =
                  accountStartDate !== null &&
                  accountStartDate !== "" &&
                  key.slice(0, 7) === accountStartDate.slice(0, 7) &&
                  key < accountStartDate;
                const balanceUnresolved =
                  targetUnresolved || (target === 0 && accountStartDatePending);
                const delta = session ? ist - target : 0;
                const status = computeRowStatus(
                  daySessions,
                  absence,
                  key,
                  holidayName,
                  closingReason,
                  target,
                  isFuture,
                );
                const isExpanded = expandedKey === key;
                const hasAuditHistory = daySessions.some(
                  (s) => (s.audit_count ?? 0) > 0,
                );
                // Only rows with audit history are interactive — clicking an
                // unchanged row should not toggle anything since there is
                // nothing to reveal.
                const canExpand = hasAuditHistory && session != null;
                // Edit / nachtragen is available for past or present days,
                // regardless of whether a session already exists. On an
                // absence-only day, however, only a half-day boundary or a
                // non-effective absence may receive additional work: effective
                // full-day absences already credit the complete target in the
                // monthly balance (#2361).
                //
                // Wochenendtage bekommen sie sehr wohl (#1967): eine Zeile ist
                // nur sichtbar, wenn dort real gearbeitet oder geplant wurde,
                // und genau die muss korrigierbar sein.
                const isWeekend = dow >= 5;
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
                const canEdit =
                  isAdminView &&
                  !isFuture &&
                  (session != null ||
                    (!absencesUnresolved && !absenceBlocksBackfill));
                return (
                  <Fragment key={key}>
                    <tr
                      onClick={() => {
                        if (!canExpand) return;
                        setExpandedKey((prev) => (prev === key ? null : key));
                      }}
                      onKeyDown={(event) => {
                        if (
                          !canExpand ||
                          (event.key !== "Enter" && event.key !== " ")
                        ) {
                          return;
                        }
                        event.preventDefault();
                        setExpandedKey((prev) => (prev === key ? null : key));
                      }}
                      tabIndex={canExpand ? 0 : undefined}
                      aria-expanded={canExpand ? isExpanded : undefined}
                      aria-label={
                        canExpand
                          ? `${formatShortDate(day)}: Änderungshistorie ${isExpanded ? "schließen" : "öffnen"}`
                          : undefined
                      }
                      className={`${canExpand ? "cursor-pointer" : ""} transition-colors hover:bg-gray-50 ${
                        isExpanded
                          ? "bg-gray-50"
                          : isToday
                            ? "bg-[#F78C10]/5"
                            : isWeekend
                              ? "bg-gray-50/60"
                              : ""
                      } ${isFuture ? "opacity-40" : ""}`}
                    >
                      <td className="px-3 py-3 text-gray-700 tabular-nums">
                        <div className="flex items-center gap-1.5">
                          {canExpand ? (
                            <ChevronRight
                              className={`h-3.5 w-3.5 shrink-0 text-gray-400 transition-transform ${
                                isExpanded ? "rotate-90" : ""
                              }`}
                            />
                          ) : (
                            <span className="inline-block h-3.5 w-3.5 shrink-0" />
                          )}
                          {formatShortDate(day)}
                        </div>
                      </td>
                      <td className="px-3 py-3 text-gray-500">
                        {dayLabels[dow]}
                      </td>
                      {showPlan && (
                        <td className="px-3 py-3 text-gray-500 tabular-nums">
                          <PlannedShiftCell shifts={shiftsByDate.get(key)} />
                        </td>
                      )}
                      {/* Auf einem Mehrblock-Tag sind diese beiden Zellen
                          Tagesgrenzen, kein durchgehender Zeitraum: zwischen
                          erstem Check-in und letztem Check-out liegen Lücken,
                          die nicht als Arbeitszeit zählen. "ab"/"bis" sagt
                          das an der Zelle selbst — die tatsächlich
                          gearbeiteten Intervalle stehen in den
                          Block-Unterzeilen. */}
                      <td className="px-3 py-3 text-gray-700 tabular-nums">
                        {session?.check_in_time ? (
                          hasMultipleBlocks ? (
                            <span title={multiBlockBoundsHint}>
                              <span className="mr-1 text-xs text-gray-400">
                                ab
                              </span>
                              <span>
                                {formatTimeOnly(session.check_in_time)}
                              </span>
                            </span>
                          ) : (
                            formatTimeOnly(session.check_in_time)
                          )
                        ) : (
                          "–"
                        )}
                      </td>
                      <td className="px-3 py-3 text-gray-700 tabular-nums">
                        {openSession ? (
                          <span className="bg-moto-green/10 text-moto-green-strong inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium">
                            <span className="bg-moto-green mr-1.5 h-1.5 w-1.5 animate-pulse rounded-full" />
                            eingestempelt
                          </span>
                        ) : lastSession?.check_out_time ? (
                          hasMultipleBlocks ? (
                            <span title={multiBlockBoundsHint}>
                              <span className="mr-1 text-xs text-gray-400">
                                bis
                              </span>
                              <span>
                                {formatTimeOnly(lastSession.check_out_time)}
                              </span>
                            </span>
                          ) : (
                            formatTimeOnly(lastSession.check_out_time)
                          )
                        ) : (
                          "–"
                        )}
                      </td>
                      <td className="px-3 py-3 text-right text-gray-500 tabular-nums">
                        {session ? formatDuration(pause) : "–"}
                      </td>
                      <td className="px-3 py-3 text-right text-gray-500 tabular-nums">
                        {targetUnresolved ? (
                          <span
                            title={
                              dailyTargetsError === true
                                ? "Soll konnte nicht geladen werden"
                                : "Soll wird geladen"
                            }
                          >
                            {dailyTargetsError === true ? "?" : "…"}
                          </span>
                        ) : target > 0 ? (
                          formatDuration(target)
                        ) : (
                          "–"
                        )}
                      </td>
                      <td className="px-3 py-3 text-right font-medium text-gray-700 tabular-nums">
                        {session ? formatDuration(ist) : "–"}
                      </td>
                      <td className="px-3 py-3 text-right tabular-nums">
                        {session &&
                        !isFuture &&
                        !balanceUnresolved &&
                        !isBeforeAccountStart ? (
                          <span className={deltaClass(delta)}>
                            {formatSignedDuration(delta)}
                          </span>
                        ) : (
                          "–"
                        )}
                      </td>
                      <td className="px-3 py-3">
                        {/* Die Tageszeile wiederholt keine Block-Details:
                            gestapelte Status-/Quelle-Badges nebeneinander
                            suggerieren falsche Paare (OGS neben App, obwohl
                            der OGS-Block per NFC kam). Der Zähler verweist
                            auf die Block-Unterzeilen, die die Fakten je
                            Block korrekt gepaart tragen. */}
                        {hasMultipleBlocks ? (
                          <StatusBadge
                            label={`${daySessions.length} Blöcke`}
                            tone="blue"
                          />
                        ) : (
                          status && <RowStatusBadge status={status} />
                        )}
                      </td>
                      <td className="px-3 py-3">
                        <SourceBadges sessions={daySessions} />
                      </td>
                      <td className="px-3 py-3">
                        <div className="flex items-center justify-between gap-3">
                          <HintBadges
                            sessions={daySessions}
                            holidayName={holidayName}
                          />
                          {canEdit && (
                            <Button
                              type="button"
                              variant="ghost"
                              size="icon"
                              onClick={(e) => {
                                e.stopPropagation();
                                // On a multi-block day the pencil adds a
                                // further block; each existing block has its
                                // own pencil in its sub-row.
                                const targetSession = hasMultipleBlocks
                                  ? null
                                  : (session ?? null);
                                if (onEditDay) {
                                  onEditDay(
                                    day,
                                    targetSession,
                                    absence ?? null,
                                  );
                                } else {
                                  setEditModal({
                                    mode:
                                      targetSession == null
                                        ? "nachtragen"
                                        : "edit",
                                    date: day,
                                    session: targetSession,
                                  });
                                }
                              }}
                              aria-label={
                                session == null
                                  ? "Eintrag nachtragen"
                                  : hasMultipleBlocks
                                    ? "Block nachtragen"
                                    : "Eintrag bearbeiten"
                              }
                              title={
                                session == null
                                  ? "Eintrag nachtragen"
                                  : hasMultipleBlocks
                                    ? "Block nachtragen"
                                    : "Eintrag bearbeiten"
                              }
                              className="h-7 w-7 shrink-0 text-gray-400 hover:text-gray-700"
                            >
                              <SquarePen className="h-4 w-4" />
                            </Button>
                          )}
                        </div>
                      </td>
                    </tr>
                    {hasMultipleBlocks &&
                      daySessions.map((block, blockIndex) => (
                        <tr
                          key={`${key}-block-${block.id ?? blockIndex}`}
                          className="bg-gray-50/40 text-xs"
                        >
                          <td className="py-2 pr-3 pl-8 text-gray-400">
                            Block {blockIndex + 1}
                          </td>
                          <td className="px-3 py-2" />
                          {showPlan && <td className="px-3 py-2" />}
                          <td className="px-3 py-2 text-gray-600 tabular-nums">
                            {block.check_in_time
                              ? formatTimeOnly(block.check_in_time)
                              : "–"}
                          </td>
                          <td className="px-3 py-2 text-gray-600 tabular-nums">
                            {block.check_out_time ? (
                              formatTimeOnly(block.check_out_time)
                            ) : (
                              <span className="text-moto-green-strong font-medium">
                                eingestempelt
                              </span>
                            )}
                          </td>
                          <td className="px-3 py-2 text-right text-gray-500 tabular-nums">
                            {formatDuration(block.break_minutes ?? 0)}
                          </td>
                          <td className="px-3 py-2" />
                          <td className="px-3 py-2 text-right text-gray-600 tabular-nums">
                            {formatDuration(block.net_minutes ?? 0)}
                          </td>
                          <td className="px-3 py-2" />
                          <td className="px-3 py-2">
                            <RowStatusBadge
                              status={
                                block.status === "home_office"
                                  ? { kind: "home-office" }
                                  : { kind: "present" }
                              }
                            />
                          </td>
                          <td className="px-3 py-2">
                            <SourceBadges sessions={[block]} />
                          </td>
                          <td className="px-3 py-2">
                            <div className="flex items-center justify-between gap-3">
                              <HintBadges
                                sessions={[block]}
                                holidayName={undefined}
                              />
                              {canEdit && (
                                <Button
                                  type="button"
                                  variant="ghost"
                                  size="icon"
                                  onClick={(e) => {
                                    e.stopPropagation();
                                    if (onEditDay) {
                                      onEditDay(day, block, absence ?? null);
                                    } else {
                                      setEditModal({
                                        mode: "edit",
                                        date: day,
                                        session: block,
                                      });
                                    }
                                  }}
                                  aria-label={`Block ${blockIndex + 1} bearbeiten`}
                                  title={`Block ${blockIndex + 1} bearbeiten`}
                                  className="h-6 w-6 shrink-0 text-gray-400 hover:text-gray-700"
                                >
                                  <SquarePen className="h-3.5 w-3.5" />
                                </Button>
                              )}
                            </div>
                          </td>
                        </tr>
                      ))}
                    {isExpanded &&
                      daySessions
                        .filter((block) => (block.audit_count ?? 0) > 0)
                        .map((block, blockIndex) => (
                          <tr
                            key={`${key}-audit-${block.id ?? blockIndex}`}
                            className="border-b border-gray-100 bg-gray-50/50"
                          >
                            <td colSpan={columnCount} className="px-3 py-3">
                              {hasMultipleBlocks && (
                                <p className="mb-1 text-xs font-medium text-gray-500">
                                  {`Block ${daySessions.indexOf(block) + 1}${
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
                                  isAdminView
                                    ? () => {
                                        if (onEditDay) {
                                          onEditDay(
                                            day,
                                            block,
                                            absence ?? null,
                                          );
                                        } else {
                                          setEditModal({
                                            mode: "edit",
                                            date: day,
                                            session: block,
                                          });
                                        }
                                      }
                                    : undefined
                                }
                              />
                            </td>
                          </tr>
                        ))}
                  </Fragment>
                );
              })}
            </tbody>
          </table>
        </div>
        {!isAdminView && (
          <p className="border-t border-gray-100 bg-gray-50 px-4 py-2 text-xs text-gray-500">
            Eigene Arbeitszeiten können in der Zeiterfassung-Seite bearbeitet
            werden.
          </p>
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

  useEffect(() => {
    if (sessionId == null) return;
    let cancelled = false;
    setIsLoading(true);
    setEdits([]);
    const load =
      fetchEdits ??
      ((sid: string, sess: string) =>
        staffSessionEditsService.getEdits(sid, sess));
    load(staffId, sessionId)
      .then((result) => {
        if (!cancelled) setEdits(result);
      })
      .catch((error: unknown) => {
        logger.error("session_edits_fetch_failed", {
          staff_id: staffId,
          session_id: sessionId,
          error: error instanceof Error ? error.message : String(error),
        });
      })
      .finally(() => {
        if (!cancelled) setIsLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [staffId, sessionId, fetchEdits]);

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
  | { kind: "absence"; absenceType: string; halfDay: boolean }
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
            <span className="text-[11px]">entfällt</span>
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
  // ranks below it: on overlap the gesetzliche Feiertag names the day.
  if (closingReason) {
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
      halfDay: isHalfAbsenceBoundary(absence, dateKey, startDate, endDate),
    };
  }
  if (target > 0 && !isFuture) {
    return { kind: "missing" };
  }
  return null;
}

// Every fixed-outcome pill is the kit StatusBadge; only the absence pill stays
// StatusDotBadge because its colour is data (ABSENCE_TYPE_HEX), not one of the
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
      <StatusDotBadge
        label="Homeoffice"
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
      ABSENCE_TYPE_LABEL[status.absenceType] ?? ABSENCE_TYPE_LABEL.other!;
    const label = status.halfDay
      ? `${absenceLabel} · Halber Tag`
      : absenceLabel;
    const color =
      ABSENCE_TYPE_HEX[status.absenceType] ?? ABSENCE_TYPE_HEX.other!;
    return <StatusDotBadge label={label} color={color} />;
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

// Hinweis-Spalte für orthogonale System- und Korrektur-Hinweise. Mehrere
// Pills können gleichzeitig sichtbar sein (z.B. NFC-Session mit Auto-Checkout
// UND nachträglicher Korrektur).
function HintBadges({
  sessions,
  holidayName,
}: {
  readonly sessions: readonly StaffHistorySession[];
  readonly holidayName?: string;
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
      tone: "orange",
    });
  }
  if (pills.length === 0) {
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
    </div>
  );
}

// Saldo values sit as plain text on a white row — identical to KpiCard in
// staff-time-views, which prices the same figure on /staff/[id]. The -strong
// ramp steps are used deliberately even though they read brownish: amber only
// becomes legible on white once it is that dark. The previous text-amber-600
// (#D97706) sat at 3.19:1 and missed AA for normal text, moto-amber-strong
// (#92400E) reaches 7.09:1. Green is the brand green because it is legible and
// unambiguous at this size.
function deltaClass(delta: number): string {
  if (delta > 0) return "text-moto-amber-strong font-medium";
  if (delta < -15) return "text-moto-red-strong font-medium";
  if (delta < 0) return "font-medium text-gray-500";
  return "text-moto-green-strong font-medium";
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

function formatTimeOnly(iso: string): string {
  const date = new Date(iso);
  const hh = String(date.getHours()).padStart(2, "0");
  const mm = String(date.getMinutes()).padStart(2, "0");
  return `${hh}:${mm}`;
}
