"use client";

import { Plus, Repeat } from "lucide-react";
import {
  useMemo,
  useState,
  type CSSProperties,
  type PointerEvent as ReactPointerEvent,
} from "react";

import {
  ClosingDayChip,
  ClosingDayConfirmModal,
} from "~/components/planning/closing-day-marker";
import { Button } from "~/components/ui/button";
import { PlanBlock } from "~/components/ui/plan-block";
import { PlanLegend, type PlanLegendEntry } from "~/components/ui/plan-legend";
import { SectionCard } from "~/components/ui/section-card";
import {
  formatColumnDate,
  formatPlannedHours,
  formatShiftLabel,
  type StaffScheduleStaff,
  type StaffShift,
  type StaffWeeklySummary,
} from "~/lib/shift-helpers";
import type { ShiftType } from "~/lib/shift-type-helpers";

// Wochenraster einer Person in Viertelstunden (#3818): Mo–Fr als Spalten, je
// Viertelstunde eine Zeile, Schichten als farbige Blöcke nach Schichtart und
// darunter die Tagessummen je Art. So plant die OGS ihre Arbeitszeit auf
// Papier; die Ansicht bildet dieses Blatt nach und lässt direkt darin planen:
// Viertelstunden aufziehen öffnet das Schicht-Formular mit vorbelegten Zeiten,
// ein Klick auf einen Block öffnet ihn zum Bearbeiten. Das Formular selbst
// bleibt minutengenau; nur das Raster rastet auf 15 Minuten ein.
//
// Ohne onCreate/onEdit ist das Raster eine reine Anzeige (#3821: der eigene
// Dienstplan der Mitarbeitenden): kein Aufziehen, keine Plus-Knöpfe, Blöcke
// ohne Klickverhalten.

const DAY_LABELS = ["Mo", "Di", "Mi", "Do", "Fr", "Sa", "So"] as const;

const SLOT_MINUTES = 15;
/** Höhe einer Viertelstunde in px — eine Stunde sind 64px wie im Kalender. */
const SLOT_PX = 16;
/** Standardfenster der Schule: 08:00 bis 16:00. Schichten außerhalb weiten
 *  das Fenster auf volle Stunden aus. */
const DEFAULT_WINDOW_START = 8 * 60;
const DEFAULT_WINDOW_END = 16 * 60;
const MINUTES_PER_DAY = 24 * 60;
/** Ein einzelner Klick (ohne Ziehen) schlägt eine Stunde vor. */
const CLICK_DEFAULT_MINUTES = 60;

const HEX6_RE = /^#[0-9a-fA-F]{6}$/;
const UNTYPED_SHIFT_LABEL = "Schicht";
const UNTYPED_KEY = "__untyped__";

interface DienstplanPersonWeekGridProps {
  /** Kopf der bearbeitbaren Ansicht; die Leseansicht nennt die Person schon
   *  im Seitenkopf und braucht ihn nicht. */
  readonly member?: StaffScheduleStaff;
  /** date ("YYYY-MM-DD") -> Schichten dieser Person. */
  readonly shiftsByDate: ReadonlyMap<string, readonly StaffShift[]> | undefined;
  /** Die Wochentage als "YYYY-MM-DD" (Montag zuerst; Mo–Fr, in der
   *  Leseansicht bei Wochenend-Diensten auch Sa/So). */
  readonly weekDays: readonly string[];
  readonly todayIso: string;
  /** OGS-Schließtage der Woche (YYYY-MM-DD → Grund, #2032). */
  readonly closingDays?: ReadonlyMap<string, string>;
  /** Solange die Schließtage laden, bleibt das Anlegen gesperrt, damit die
   *  Rückfrage nicht umgangen wird. */
  readonly closingDaysLoading?: boolean;
  readonly typesById: Map<string, ShiftType>;
  readonly shiftTypes: readonly ShiftType[];
  /** Soll/Plan der Woche (nur auf dem vollen Berechtigungspfad). */
  readonly summary?: StaffWeeklySummary;
  /** Fehlt er, lässt sich im Raster nichts anlegen. */
  readonly onCreate?: (
    date: string,
    startTime: string,
    endTime: string,
  ) => void;
  /** Fehlt er, sind die Blöcke reine Anzeige. */
  readonly onEdit?: (date: string, shift: StaffShift) => void;
}

/** "08:15" → 495. Ungültige Werte ergeben null. */
function clockToMinutes(value: string): number | null {
  const match = /^(\d{2}):(\d{2})$/.exec(value);
  if (!match) return null;
  const hours = Number(match[1]);
  const minutes = Number(match[2]);
  if (hours > 23 || minutes > 59) return null;
  return hours * 60 + minutes;
}

/** 495 → "08:15". 1440 (Tagesende) wird zu "23:59", weil eine Schicht nicht
 *  über Mitternacht reichen kann. */
export function minutesToClock(total: number): string {
  if (total >= MINUTES_PER_DAY) return "23:59";
  const hours = Math.floor(total / 60);
  const minutes = total % 60;
  return `${String(hours).padStart(2, "0")}:${String(minutes).padStart(2, "0")}`;
}

/** Netto-Minuten einer Schicht (Spanne minus Pause); ausgefallene zählen nicht. */
function shiftNetMinutes(shift: StaffShift): number {
  if (shift.cancelled) return 0;
  const start = clockToMinutes(shift.startTime);
  const end = clockToMinutes(shift.endTime);
  if (start === null || end === null) return 0;
  return Math.max(0, end - start - shift.breakMinutes);
}

/**
 * Sichtbares Fenster in Minuten: 08:00–16:00, auf volle Stunden ausgeweitet,
 * sobald eine Schicht der Woche früher beginnt oder später endet.
 */
export function gridWindow(shifts: readonly StaffShift[]): {
  start: number;
  end: number;
} {
  let start = DEFAULT_WINDOW_START;
  let end = DEFAULT_WINDOW_END;
  for (const shift of shifts) {
    const s = clockToMinutes(shift.startTime);
    const e = clockToMinutes(shift.endTime);
    if (s !== null) start = Math.min(start, Math.floor(s / 60) * 60);
    if (e !== null) end = Math.max(end, Math.ceil(e / 60) * 60);
  }
  return { start, end: Math.min(end, MINUTES_PER_DAY) };
}

/**
 * Spanne, die ein Klick auf eine Viertelstunde vorschlägt: eine Stunde ab dem
 * Slot, aber nie über das Fenster oder den Beginn der nächsten stattfindenden
 * Schicht hinaus — sonst lehnte das Speichern die Überschneidung ab.
 */
export function clickSpan(
  slotStart: number,
  windowEnd: number,
  dayShifts: readonly StaffShift[],
): { start: number; end: number } | null {
  let end = Math.min(slotStart + CLICK_DEFAULT_MINUTES, windowEnd);
  for (const shift of dayShifts) {
    if (shift.cancelled) continue;
    const s = clockToMinutes(shift.startTime);
    if (s !== null && s > slotStart && s < end) end = s;
  }
  return end > slotStart ? { start: slotStart, end } : null;
}

interface PlacedShift {
  shift: StaffShift;
  start: number;
  end: number;
  column: number;
  columnCount: number;
}

// Überlappende Blöcke (z. B. eine ausgefallene Schicht und ihr verkürzter
// Ersatz) stehen nebeneinander statt übereinander.
function layoutShifts(shifts: readonly StaffShift[]): PlacedShift[] {
  const items = shifts
    .map((shift) => ({
      shift,
      start: clockToMinutes(shift.startTime),
      end: clockToMinutes(shift.endTime),
    }))
    .filter(
      (item): item is { shift: StaffShift; start: number; end: number } =>
        item.start !== null && item.end !== null && item.end > item.start,
    )
    .sort((a, b) => a.start - b.start || b.end - a.end);

  const placed: PlacedShift[] = [];
  let cluster: PlacedShift[] = [];
  let clusterEnd = -1;
  let columnEnds: number[] = [];
  const flush = () => {
    for (const item of cluster) item.columnCount = columnEnds.length;
    placed.push(...cluster);
    cluster = [];
    columnEnds = [];
    clusterEnd = -1;
  };
  for (const item of items) {
    if (clusterEnd >= 0 && item.start >= clusterEnd) flush();
    let column = columnEnds.findIndex((end) => end <= item.start);
    if (column === -1) {
      column = columnEnds.length;
      columnEnds.push(item.end);
    } else {
      columnEnds[column] = item.end;
    }
    cluster.push({ ...item, column, columnCount: 1 });
    clusterEnd = Math.max(clusterEnd, item.end);
  }
  flush();
  return placed;
}

export function resolveShiftColor(
  shift: StaffShift,
  typesById: Map<string, ShiftType>,
): string | undefined {
  const raw =
    shift.shiftTypeColor ??
    (shift.shiftTypeId ? typesById.get(shift.shiftTypeId)?.color : undefined);
  return raw && HEX6_RE.test(raw) ? raw : undefined;
}

export function shiftLabel(
  shift: StaffShift,
  typesById: Map<string, ShiftType>,
): string {
  return (
    shift.shiftTypeName ??
    (shift.shiftTypeId ? typesById.get(shift.shiftTypeId)?.name : undefined) ??
    UNTYPED_SHIFT_LABEL
  );
}

interface SumRow {
  key: string;
  label: string;
  color: string | undefined;
  byDay: Map<string, number>;
  total: number;
}

/**
 * Tagessummen je Schichtart (Netto-Minuten, ausgefallene Schichten zählen
 * nicht). Reihenfolge wie die Schichtarten in der Datenverwaltung, Schichten
 * ohne Art zuletzt.
 */
export function sumsByType(
  weekDays: readonly string[],
  shiftsByDate: ReadonlyMap<string, readonly StaffShift[]> | undefined,
  typesById: Map<string, ShiftType>,
  shiftTypes: readonly ShiftType[],
): { rows: SumRow[]; dayTotals: Map<string, number>; weekTotal: number } {
  const rowsByKey = new Map<string, SumRow>();
  const dayTotals = new Map<string, number>();
  let weekTotal = 0;
  for (const date of weekDays) {
    for (const shift of shiftsByDate?.get(date) ?? []) {
      if (shift.cancelled) continue;
      const net = shiftNetMinutes(shift);
      const key = shift.shiftTypeId ?? UNTYPED_KEY;
      let row = rowsByKey.get(key);
      if (!row) {
        row = {
          key,
          label: shiftLabel(shift, typesById),
          color: resolveShiftColor(shift, typesById),
          byDay: new Map(),
          total: 0,
        };
        rowsByKey.set(key, row);
      }
      row.byDay.set(date, (row.byDay.get(date) ?? 0) + net);
      row.total += net;
      dayTotals.set(date, (dayTotals.get(date) ?? 0) + net);
      weekTotal += net;
    }
  }
  const order = new Map(shiftTypes.map((type, index) => [type.id, index]));
  const rows = [...rowsByKey.values()].sort((a, b) => {
    const ai = a.key === UNTYPED_KEY ? Infinity : (order.get(a.key) ?? 1e6);
    const bi = b.key === UNTYPED_KEY ? Infinity : (order.get(b.key) ?? 1e6);
    return ai - bi || a.label.localeCompare(b.label, "de");
  });
  return { rows, dayTotals, weekTotal };
}

interface DragState {
  date: string;
  anchor: number;
  current: number;
  pointerId: number;
}

export function DienstplanPersonWeekGrid({
  member,
  shiftsByDate,
  weekDays,
  todayIso,
  closingDays,
  closingDaysLoading = false,
  typesById,
  shiftTypes,
  summary,
  onCreate,
  onEdit,
}: DienstplanPersonWeekGridProps) {
  const readOnly = onCreate === undefined && onEdit === undefined;
  const [drag, setDrag] = useState<DragState | null>(null);
  const [hover, setHover] = useState<{ date: string; slot: number } | null>(
    null,
  );
  const [closingDayPrompt, setClosingDayPrompt] = useState<{
    date: string;
    reason: string;
    start: string;
    end: string;
  } | null>(null);

  const weekShifts = useMemo(
    () => weekDays.flatMap((date) => [...(shiftsByDate?.get(date) ?? [])]),
    [weekDays, shiftsByDate],
  );
  const timeWindow = useMemo(() => gridWindow(weekShifts), [weekShifts]);
  const slotCount = (timeWindow.end - timeWindow.start) / SLOT_MINUTES;
  const bodyHeight = slotCount * SLOT_PX;

  const sums = useMemo(
    () => sumsByType(weekDays, shiftsByDate, typesById, shiftTypes),
    [weekDays, shiftsByDate, typesById, shiftTypes],
  );

  const legendEntries: PlanLegendEntry[] = useMemo(
    () => [
      ...shiftTypes
        .filter((type) => type.isActive)
        .map((type) => ({
          key: `type-${type.id}`,
          label: type.name,
          color: type.color,
          variant: "bar" as const,
        })),
      { key: "state-cancelled", label: "Fällt aus", variant: "cancelled" },
    ],
    [shiftTypes],
  );

  const requestCreate = (date: string, start: number, end: number) => {
    if (!onCreate) return;
    const startClock = minutesToClock(start);
    const endClock = minutesToClock(end);
    const reason = closingDays?.get(date);
    if (reason !== undefined) {
      setClosingDayPrompt({ date, reason, start: startClock, end: endClock });
      return;
    }
    onCreate(date, startClock, endClock);
  };

  const slotFromPointer = (
    event: ReactPointerEvent<HTMLDivElement>,
  ): number => {
    const rect = event.currentTarget.getBoundingClientRect();
    const raw = Math.floor((event.clientY - rect.top) / SLOT_PX);
    return Math.min(Math.max(raw, 0), slotCount - 1);
  };

  const handlePointerDown = (
    date: string,
    event: ReactPointerEvent<HTMLDivElement>,
  ) => {
    if (!onCreate || closingDaysLoading || event.button !== 0) return;
    const slot = slotFromPointer(event);
    event.currentTarget.setPointerCapture?.(event.pointerId);
    setDrag({ date, anchor: slot, current: slot, pointerId: event.pointerId });
  };

  const handlePointerMove = (
    date: string,
    event: ReactPointerEvent<HTMLDivElement>,
  ) => {
    const slot = slotFromPointer(event);
    if (drag && drag.pointerId === event.pointerId) {
      if (slot !== drag.current) setDrag({ ...drag, current: slot });
      return;
    }
    if (
      onCreate &&
      event.pointerType === "mouse" &&
      (hover?.date !== date || hover.slot !== slot)
    ) {
      setHover({ date, slot });
    }
  };

  const handlePointerUp = (event: ReactPointerEvent<HTMLDivElement>) => {
    if (!drag || drag.pointerId !== event.pointerId) return;
    const from = Math.min(drag.anchor, drag.current);
    const to = Math.max(drag.anchor, drag.current);
    setDrag(null);
    const slotStart = timeWindow.start + from * SLOT_MINUTES;
    if (from === to) {
      const span = clickSpan(
        slotStart,
        timeWindow.end,
        shiftsByDate?.get(drag.date) ?? [],
      );
      if (span) requestCreate(drag.date, span.start, span.end);
      return;
    }
    requestCreate(
      drag.date,
      slotStart,
      timeWindow.start + (to + 1) * SLOT_MINUTES,
    );
  };

  const hourMarks: number[] = [];
  for (let m = timeWindow.start; m <= timeWindow.end; m += 60)
    hourMarks.push(m);
  const quarterLines: number[] = [];
  for (let i = 1; i < slotCount; i += 1) quarterLines.push(i);

  const columnTemplate: CSSProperties = {
    gridTemplateColumns: `repeat(${weekDays.length}, minmax(0, 1fr))`,
  };

  // Der Server fasst die Vertragswoche Mo–So zusammen, obwohl das Raster
  // absichtlich nur Mo–Fr zeigt. Für die Kopfzahl verwenden wir deshalb den
  // serverseitigen Wochenwert, damit Ist und Soll denselben Zeitraum meinen.
  const plannedLabel = formatPlannedHours(
    summary?.plannedMinutes ?? sums.weekTotal,
  );
  const targetLabel =
    summary?.targetMinutes != null
      ? formatPlannedHours(summary.targetMinutes)
      : null;

  return (
    <div className="space-y-3">
      <SectionCard className="!p-0">
        <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1 border-b border-gray-200 px-4 py-3">
          {readOnly ? (
            // Die Leseansicht nennt Person und Wochensummen schon im
            // Seitenkopf; hier steht nur, wer den Plan ändert.
            <p className="w-full text-xs text-gray-500">
              Nur zur Information. Ihre Schichten plant die Leitung im
              Dienstplan.
            </p>
          ) : (
            <>
              {member && (
                <h2 className="text-sm font-semibold text-gray-900">
                  {member.lastName}, {member.firstName}
                </h2>
              )}
              <p className="text-xs text-gray-600 tabular-nums">
                Diese Woche geplant:{" "}
                <span className="font-semibold text-gray-900">
                  {plannedLabel}
                </span>
                {targetLabel ? ` · Soll ${targetLabel}` : ""}
              </p>
              <p className="w-full text-xs text-gray-500">
                Ziehen Sie über die Viertelstunden, um eine Schicht anzulegen.
                Klicken Sie auf eine Schicht, um sie zu ändern.
              </p>
            </>
          )}
          <p className="w-full text-xs text-gray-500 sm:hidden">
            Wischen Sie zur Seite, um weitere Tage zu sehen.
          </p>
        </div>

        <div className="overflow-x-auto">
          <div className="min-w-[640px]">
            {/* Kopfzeile: Wochentage */}
            <div className="flex border-b border-gray-200 bg-gray-50">
              <div className="sticky left-0 z-30 w-24 shrink-0 border-r border-gray-200 bg-gray-50 sm:w-36" />
              <div className="grid flex-1" style={columnTemplate}>
                {weekDays.map((date, index) => {
                  const closingReason = closingDays?.get(date);
                  const isToday = date === todayIso;
                  const label = `${DAY_LABELS[index] ?? ""} ${formatColumnDate(date)}`;
                  return (
                    <div
                      key={date}
                      className="flex items-start justify-between gap-1 border-r border-gray-200 px-2 py-1.5 last:border-r-0"
                    >
                      <div className="min-w-0">
                        <div
                          className={`text-sm font-semibold tabular-nums ${
                            isToday ? "text-gray-900" : "text-gray-700"
                          }`}
                        >
                          {label}
                          {isToday && (
                            <span
                              aria-label="heute"
                              className="bg-moto-red ml-1.5 inline-block h-1.5 w-1.5 rounded-full align-middle"
                            />
                          )}
                        </div>
                        {closingReason !== undefined && (
                          <ClosingDayChip reason={closingReason} />
                        )}
                      </div>
                      {onCreate && (
                        <Button
                          type="button"
                          variant="ghost"
                          size="icon"
                          className="shrink-0"
                          disabled={closingDaysLoading}
                          aria-label={`Schicht anlegen, ${label}`}
                          onClick={() =>
                            requestCreate(
                              date,
                              DEFAULT_WINDOW_START,
                              DEFAULT_WINDOW_END,
                            )
                          }
                        >
                          <Plus className="h-4 w-4" aria-hidden />
                        </Button>
                      )}
                    </div>
                  );
                })}
              </div>
            </div>

            {/* Raster: eine Zeile je Viertelstunde */}
            <div className="flex" style={{ height: bodyHeight }}>
              <div
                className="sticky left-0 z-30 w-24 shrink-0 border-r border-gray-200 bg-white sm:w-36"
                aria-hidden
              >
                {hourMarks.slice(0, -1).map((minute) => (
                  <div
                    key={minute}
                    className="absolute right-2 text-xs text-gray-500 tabular-nums"
                    style={{
                      top:
                        ((minute - timeWindow.start) / SLOT_MINUTES) * SLOT_PX,
                    }}
                  >
                    {minutesToClock(minute)}
                  </div>
                ))}
              </div>
              <div className="grid flex-1" style={columnTemplate}>
                {weekDays.map((date) => {
                  const dayShifts = shiftsByDate?.get(date) ?? [];
                  const placed = layoutShifts(dayShifts);
                  const isClosing = closingDays?.has(date) ?? false;
                  const dragHere = drag?.date === date ? drag : null;
                  const hoverHere =
                    !drag && hover?.date === date ? hover.slot : null;
                  let cursorClass = "cursor-cell";
                  if (!onCreate) cursorClass = "";
                  else if (closingDaysLoading) cursorClass = "cursor-wait";
                  return (
                    <div
                      key={date}
                      data-testid={`person-week-day-${date}`}
                      className={`relative touch-pan-x border-r border-gray-200 select-none last:border-r-0 ${
                        isClosing ? "bg-gray-50" : ""
                      } ${cursorClass}`}
                      style={{ height: bodyHeight }}
                      onPointerDown={(event) => handlePointerDown(date, event)}
                      onPointerMove={(event) => handlePointerMove(date, event)}
                      onPointerUp={handlePointerUp}
                      onPointerCancel={() => setDrag(null)}
                      onPointerLeave={() => setHover(null)}
                    >
                      {quarterLines.map((line) => (
                        <div
                          key={line}
                          aria-hidden
                          className={`pointer-events-none absolute inset-x-0 border-t ${
                            (timeWindow.start + line * SLOT_MINUTES) % 60 === 0
                              ? "border-gray-200"
                              : "border-gray-100"
                          }`}
                          style={{ top: line * SLOT_PX }}
                        />
                      ))}
                      {hoverHere !== null && (
                        <div
                          aria-hidden
                          className="pointer-events-none absolute inset-x-0.5 rounded-sm bg-gray-100"
                          style={{ top: hoverHere * SLOT_PX, height: SLOT_PX }}
                        />
                      )}
                      {dragHere && (
                        <DragPreview
                          drag={dragHere}
                          windowStart={timeWindow.start}
                        />
                      )}
                      {placed.map((item) => (
                        <ShiftBlock
                          key={item.shift.id}
                          item={item}
                          windowStart={timeWindow.start}
                          typesById={typesById}
                          onEdit={
                            onEdit ? () => onEdit(date, item.shift) : undefined
                          }
                        />
                      ))}
                    </div>
                  );
                })}
              </div>
            </div>

            {/* Tagessummen je Schichtart */}
            <div className="border-t border-gray-200 bg-gray-50 text-xs">
              {sums.rows.map((row) => (
                <div key={row.key} className="flex border-b border-gray-100">
                  <div className="sticky left-0 z-30 flex w-24 shrink-0 items-start gap-1.5 border-r border-gray-200 bg-gray-50 px-2 py-1 text-gray-700 sm:w-36">
                    <span
                      aria-hidden
                      className={`mt-0.5 h-3 w-0.5 shrink-0 rounded-full ${
                        row.color ? "" : "bg-gray-300"
                      }`}
                      style={
                        row.color ? { backgroundColor: row.color } : undefined
                      }
                    />
                    <span
                      className="line-clamp-2 hyphens-auto"
                      title={row.label}
                    >
                      {row.label}
                    </span>
                  </div>
                  <div className="grid flex-1" style={columnTemplate}>
                    {weekDays.map((date) => {
                      const minutes = row.byDay.get(date) ?? 0;
                      return (
                        <div
                          key={date}
                          className="border-r border-gray-100 px-2 py-1 text-right text-gray-600 tabular-nums last:border-r-0"
                        >
                          {minutes > 0 ? formatPlannedHours(minutes) : "–"}
                        </div>
                      );
                    })}
                  </div>
                </div>
              ))}
              <div className="flex">
                <div className="sticky left-0 z-30 w-24 shrink-0 border-r border-gray-200 bg-gray-50 px-2 py-1.5 font-semibold text-gray-900 sm:w-36">
                  Tagessumme
                </div>
                <div className="grid flex-1" style={columnTemplate}>
                  {weekDays.map((date) => (
                    <div
                      key={date}
                      data-testid={`person-week-total-${date}`}
                      className="border-r border-gray-100 px-2 py-1.5 text-right font-semibold text-gray-900 tabular-nums last:border-r-0"
                    >
                      {formatPlannedHours(sums.dayTotals.get(date) ?? 0)}
                    </div>
                  ))}
                </div>
              </div>
            </div>
          </div>
        </div>

        <div className="border-t border-gray-200 px-4 py-2">
          <PlanLegend
            entries={legendEntries}
            aria-label="Legende Schichtarten und Zustände"
          />
        </div>
      </SectionCard>

      {closingDayPrompt && (
        <ClosingDayConfirmModal
          dateISO={closingDayPrompt.date}
          reason={closingDayPrompt.reason}
          subject="schicht"
          onCancel={() => setClosingDayPrompt(null)}
          onConfirm={() => {
            const { date, start, end } = closingDayPrompt;
            setClosingDayPrompt(null);
            onCreate?.(date, start, end);
          }}
        />
      )}
    </div>
  );
}

function DragPreview({
  drag,
  windowStart,
}: Readonly<{ drag: DragState; windowStart: number }>) {
  const from = Math.min(drag.anchor, drag.current);
  const to = Math.max(drag.anchor, drag.current);
  const start = windowStart + from * SLOT_MINUTES;
  const end = windowStart + (to + 1) * SLOT_MINUTES;
  return (
    <div
      aria-hidden
      className="pointer-events-none absolute inset-x-0.5 z-20 overflow-hidden rounded-md border border-dashed border-gray-500 bg-gray-900/5 px-1.5 text-xs font-medium text-gray-700 tabular-nums"
      style={{ top: from * SLOT_PX, height: (to - from + 1) * SLOT_PX }}
    >
      {minutesToClock(start)}–{minutesToClock(end)}
    </div>
  );
}

function ShiftBlock({
  item,
  windowStart,
  typesById,
  onEdit,
}: Readonly<{
  item: PlacedShift;
  windowStart: number;
  typesById: Map<string, ShiftType>;
  onEdit: (() => void) | undefined;
}>) {
  const { shift, start, end, column, columnCount } = item;
  const duration = end - start;
  const timeRange = formatShiftLabel(shift);
  const label = shiftLabel(shift, typesById);
  const widthPercent = 100 / columnCount;
  const isShort = duration < 45;
  const ariaParts = [`${timeRange} ${label}`];
  if (shift.breakMinutes > 0) ariaParts.push(`Pause ${shift.breakMinutes} min`);
  if (shift.cancelled) ariaParts.push("fällt aus");
  if (shift.originShiftId != null) ariaParts.push("Vertretung");
  const showSeriesIcon = !shift.cancelled && shift.seriesId != null;

  return (
    // Stoppt den Pointer-Start des Rasters: ein Klick auf einen Block
    // bearbeitet ihn und zieht keine neue Spanne auf.
    <div
      className="absolute z-10 rounded-md bg-white"
      style={{
        top: ((start - windowStart) / SLOT_MINUTES) * SLOT_PX + 1,
        height: (duration / SLOT_MINUTES) * SLOT_PX - 2,
        left: `calc(${column * widthPercent}% + 2px)`,
        width: `calc(${widthPercent}% - 4px)`,
      }}
      onPointerDown={(event) => event.stopPropagation()}
    >
      <PlanBlock
        size={isShort ? "compact" : "default"}
        status={shift.cancelled ? "cancelled" : "default"}
        timeRange={timeRange}
        label={label}
        color={resolveShiftColor(shift, typesById)}
        statusIcon={
          showSeriesIcon ? (
            <Repeat
              aria-label={
                shift.detached
                  ? "Serie, für diese Woche angepasst"
                  : "Teil einer Serie"
              }
              className={shift.detached ? "text-moto-amber" : "text-gray-400"}
            />
          ) : undefined
        }
        footer={
          !isShort && shift.breakMinutes > 0 && duration >= 75 ? (
            <span className="text-xs text-gray-500">
              Pause {shift.breakMinutes} min
            </span>
          ) : undefined
        }
        className={`flex h-full flex-col overflow-hidden ${isShort ? "justify-center py-0!" : "justify-start"}`}
        interactive={onEdit !== undefined}
        onClick={onEdit}
        aria-label={ariaParts.join(", ")}
      />
    </div>
  );
}
