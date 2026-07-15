"use client";

import {
  ArrowRightLeft,
  CalendarOff,
  Clock,
  MapPin,
  Plus,
  Repeat,
  Thermometer,
  TriangleAlert,
} from "lucide-react";
import { useId, useMemo, useState, type ReactNode } from "react";

import { ShiftMoveDialog } from "~/components/staff/shift-move-dialog";
import { CapacityStrip } from "~/components/ui/capacity-strip";
import { CoverageIndicator } from "~/components/ui/coverage-indicator";
import {
  OverflowMenu,
  type OverflowMenuEntry,
} from "~/components/ui/page-header/OverflowMenu";
import { PlanBlock } from "~/components/ui/plan-block";
import { PlanLegend, type PlanLegendEntry } from "~/components/ui/plan-legend";
import {
  ResourceGrid,
  type ResourceGridColumn,
} from "~/components/ui/resource-grid";
import { Tooltip } from "~/components/ui/tooltip";
import { PLAN_CACHE_KEY_PREFIXES } from "~/lib/hooks/use-dienstplan-data";
import { LOCATION_COLORS } from "~/lib/location-helper";
import {
  formatColumnDate,
  formatPlannedHours,
  formatShiftLabel,
  summaryLabel,
  summaryTone,
  type StaffScheduleAssignment,
  type StaffScheduleStaff,
  type StaffShift,
  type StaffWeeklySummary,
} from "~/lib/shift-helpers";
import type { ShiftType } from "~/lib/shift-type-helpers";
import { useTenantMutateMatching } from "~/lib/swr";
import { useTenantRouter } from "~/lib/tenant-router";

// Week matrix built on the generic kit ResourceGrid (docs/05-dienstplan.md
// Abschnitt 2, docs/04-designsprache.md Abschnitt 6.2): rows are staff, columns
// the weekdays Mo–Fr. All domain-to-block mapping lives here in the screen
// view; the kit primitives stay generic (Fertig-Kriterium Y7). No data fetching
// happens in this component — everything arrives as props from the view/hook.

const DAY_LABELS = ["Mo", "Di", "Mi", "Do", "Fr"] as const;

// Reference window for the capacity footer (docs/05 Abschnitt 2.4). "HH:MM"
// strings compare lexicographically because they are zero-padded.
const CAPACITY_WINDOW_START = "12:00";
const CAPACITY_WINDOW_END = "16:00";

// PlanBlock only accepts a 6-digit hex; an embedded/typed color that is not one
// is dropped so the block falls back to its neutral edge instead of producing
// an invalid CSS color.
const HEX6_RE = /^#[0-9a-fA-F]{6}$/;

// Fallback label for a shift without a Schichtart — the compact PlanBlock always
// needs a label (the Bestandsraster simply omitted the type line there).
const UNTYPED_SHIFT_LABEL = "Schicht";

// The daily checkout reason the sick cascade stamps on cancelled shifts (#1843);
// a second derivation source for the "krank" row-header note (Abgleich B2).
const SICK_CHANGE_REASON = "Krankheit";

interface DienstplanResourceGridProps {
  readonly staff: readonly StaffScheduleStaff[];
  /** staffId -> date ("YYYY-MM-DD") -> shifts */
  readonly shiftsByStaff: Map<string, Map<string, StaffShift[]>>;
  /** staffId -> date ("YYYY-MM-DD") -> concrete Betreuungsplan assignments.
   *  Empty on the reduced permission path (no overview). */
  readonly assignmentsByStaff: Map<
    string,
    Map<string, StaffScheduleAssignment[]>
  >;
  /** staffId -> weekly planned/target summary for the displayed week. Empty on
   *  the reduced permission path. */
  readonly summaryByStaff: Map<string, StaffWeeklySummary>;
  /** The five weekday dates as "YYYY-MM-DD" (Monday first). */
  readonly weekDays: readonly string[];
  /** Today as "YYYY-MM-DD" for the column tint and the absence-entry jump. */
  readonly todayIso: string;
  /** Shift type lookup (id -> type) for per-shift color + label fallback. */
  readonly typesById: Map<string, ShiftType>;
  /** Every shift type (Schichtart) for the legend. Data source: the view. */
  readonly shiftTypes: readonly ShiftType[];
  /** Reduced permission path (only `time_tracking:manage`): the row header
   *  shows the name only — no weekly summary, no absence note, no menu. */
  readonly reducedPath?: boolean;
  /** The logged-in user's own staff id (int64 as string), or null when it
   *  cannot be resolved. Drives the "Zeiterfassung öffnen" target. */
  readonly currentStaffId?: string | null;
  readonly onCellClick: (
    staff: StaffScheduleStaff,
    date: string,
    shift: StaffShift | null,
  ) => void;
  /** Opens the "Krank melden" flow (#1843). Set only when the caller has
   *  `time_tracking:manage`; the menu item is hidden otherwise. */
  readonly onSickReport?: (staff: StaffScheduleStaff) => void;
}

// Row-header "krank" note: any absent assignment this week, or a cancelled
// shift stamped with the sick-cascade reason (Abgleich B2 — covers people
// without assignments). Full path only; pure so it can be precomputed once per
// data change instead of on every render (memoized in the component).
function absenceNoteForStaff(
  staffId: string,
  assignmentsByStaff: Map<string, Map<string, StaffScheduleAssignment[]>>,
  shiftsByStaff: Map<string, Map<string, StaffShift[]>>,
): string | null {
  const assignmentDays = assignmentsByStaff.get(staffId);
  if (assignmentDays) {
    for (const dayAssignments of assignmentDays.values()) {
      for (const assignment of dayAssignments) {
        if (assignment.isAbsent) return assignment.absenceReason ?? "krank";
      }
    }
  }
  const shiftDays = shiftsByStaff.get(staffId);
  if (shiftDays) {
    for (const dayShifts of shiftDays.values()) {
      for (const shift of dayShifts) {
        if (shift.cancelled && shift.changeReason === SICK_CHANGE_REASON) {
          return "krank";
        }
      }
    }
  }
  return null;
}

// Ported 1:1 from the retired per-cell week grid (pure, prop-only) — the accent color
// of a read-only Betreuungsplan assignment card.
function assignmentAccentColor(assignment: StaffScheduleAssignment): string {
  if (assignment.isAbsent) return LOCATION_COLORS.HOME;
  if (assignment.coverageStatus === "uncovered") return LOCATION_COLORS.SICK;
  if (assignment.isSubstitute) return LOCATION_COLORS.OTHER_ROOM;
  return LOCATION_COLORS.UNKNOWN;
}

// Ported 1:1 from the retired per-cell week grid — the visually subordinate second
// tier under the shift blocks (docs/05 Abschnitt 2.2). Only rendered on the
// full permission path where assignments exist.
function AssignmentCard({
  assignment,
}: {
  readonly assignment: StaffScheduleAssignment;
}) {
  const isUncovered = assignment.coverageStatus === "uncovered";
  let assignmentStatus = null;
  if (assignment.isAbsent) {
    assignmentStatus = (
      <span
        className="mt-1 block text-xs font-semibold"
        style={{ color: LOCATION_COLORS.HOME }}
      >
        Abwesend
        {assignment.absenceReason ? ` · ${assignment.absenceReason}` : ""}
      </span>
    );
  } else if (assignment.isSubstitute) {
    assignmentStatus = (
      <span
        className="mt-1 block text-xs font-semibold"
        style={{ color: LOCATION_COLORS.OTHER_ROOM }}
      >
        Vertretung
      </span>
    );
  }

  return (
    <div
      data-testid={`dienstplan-assignment-${assignment.instanceId}-${assignment.staffId}`}
      style={{ borderLeftColor: assignmentAccentColor(assignment) }}
      className="border-l-2 bg-gray-50/70 px-2 py-1.5 text-left"
    >
      <div className="flex items-start justify-between gap-1.5">
        <span className="font-semibold text-gray-900 tabular-nums">
          {assignment.startTime}–{assignment.endTime}
        </span>
        {isUncovered ? (
          <TriangleAlert
            className="mt-0.5 h-3.5 w-3.5 shrink-0"
            style={{ color: LOCATION_COLORS.SICK }}
            aria-label="Nicht vollständig durch Schicht abgedeckt"
          />
        ) : null}
      </div>
      <span className="block truncate text-xs font-medium text-gray-700">
        {assignment.activityTitle}
      </span>
      <span className="flex items-center gap-1 truncate text-xs text-gray-500">
        <MapPin className="h-3 w-3 shrink-0" aria-hidden />
        {assignment.roomName}
      </span>
      {assignmentStatus}
      {isUncovered
        ? assignment.uncoveredIntervals.map((interval) => (
            <span
              key={`${interval.startTime}-${interval.endTime}`}
              className="mt-1 block text-xs font-medium text-gray-700"
            >
              Nicht abgedeckt: {interval.startTime}–{interval.endTime}
            </span>
          ))
        : null}
    </div>
  );
}

// summaryTone / summaryLabel now live in ~/lib/shift-helpers (shared with the
// half-year grid). Tooltip naming the Soll source ("Arbeitszeitmodell") until the R7 model
// question is decided (docs/05 Abschnitt 2.3, Fertig-Kriterium 4).
function summaryTooltip(summary: StaffWeeklySummary): string {
  const planned = formatPlannedHours(summary.plannedMinutes);
  return summary.targetMinutes !== null
    ? `Geplant ${planned} · Soll ${formatPlannedHours(summary.targetMinutes)} (aus Arbeitszeitmodell)`
    : `Geplant ${planned} · kein Arbeitszeitmodell hinterlegt`;
}

function WeeklySummaryIndicator({
  summary,
}: {
  readonly summary: StaffWeeklySummary;
}) {
  const tooltip = summaryTooltip(summary);
  return (
    <Tooltip className="mt-0.5 block w-fit" content={tooltip}>
      <CoverageIndicator
        state="covered"
        size="sm"
        label={summaryLabel(summary)}
        tone={summaryTone(summary)}
        title={tooltip}
      />
    </Tooltip>
  );
}

// Resolve the shift's Schichtart color, preferring the embedded value (#1844)
// so the reduced path colors blocks too. Anything but a 6-digit hex is dropped.
function resolveShiftColor(
  shift: StaffShift,
  typesById: Map<string, ShiftType>,
): string | undefined {
  const raw =
    shift.shiftTypeColor ??
    (shift.shiftTypeId ? typesById.get(shift.shiftTypeId)?.color : undefined);
  return raw && HEX6_RE.test(raw) ? raw : undefined;
}

function shiftLabel(
  shift: StaffShift,
  typesById: Map<string, ShiftType>,
): string {
  const typeName =
    shift.shiftTypeName ??
    (shift.shiftTypeId ? typesById.get(shift.shiftTypeId)?.name : undefined);
  return typeName ?? UNTYPED_SHIFT_LABEL;
}

function createShiftAriaLabel(
  member: StaffScheduleStaff,
  column: ResourceGridColumn,
): string {
  return `Schicht anlegen, ${member.firstName} ${member.lastName}, ${column.label} ${column.sublabel ?? ""}`.trim();
}

export function DienstplanResourceGrid({
  staff,
  shiftsByStaff,
  assignmentsByStaff,
  summaryByStaff,
  weekDays,
  todayIso,
  typesById,
  shiftTypes,
  reducedPath = false,
  currentStaffId = null,
  onCellClick,
  onSickReport,
}: DienstplanResourceGridProps) {
  const router = useTenantRouter();
  const scrollHintId = useId();

  // "Verschieben nach" (docs/05 Abschnitt 2.7): the move dialog lives here so
  // the Dienstplan view needs no new prop. After a move (success, or the
  // POST-succeeded/DELETE-failed recovery case) refresh every cache a shift
  // change can affect — the same PLAN_CACHE_KEY_PREFIXES set the sick-report
  // flow uses: a moved shift can shift Vertretung coverage/gaps too, not just
  // the two Dienstplan data paths.
  const [moveTarget, setMoveTarget] = useState<{
    member: StaffScheduleStaff;
    shift: StaffShift;
  } | null>(null);
  const refreshAfterMove = useTenantMutateMatching(PLAN_CACHE_KEY_PREFIXES);

  const columns: ResourceGridColumn[] = useMemo(
    () =>
      weekDays.map((date, i) => ({
        key: date,
        label: DAY_LABELS[i] ?? "",
        sublabel: formatColumnDate(date),
        isCurrent: date === todayIso,
      })),
    [weekDays, todayIso],
  );

  // Per-day capacity (12–16 window), computed once per data change instead of
  // per footer cell, so opening/closing the move dialog does not re-scan every
  // person × day.
  const capacityByDay = useMemo(() => {
    const byDay = new Map<string, number>();
    for (const date of weekDays) {
      let count = 0;
      for (const member of staff) {
        const dayShifts = shiftsByStaff.get(member.id)?.get(date) ?? [];
        const staffed = dayShifts.some(
          (shift) =>
            !shift.cancelled &&
            shift.startTime < CAPACITY_WINDOW_END &&
            shift.endTime > CAPACITY_WINDOW_START,
        );
        if (staffed) count += 1;
      }
      byDay.set(date, count);
    }
    return byDay;
  }, [staff, shiftsByStaff, weekDays]);

  // Row-header "krank" note per person, precomputed so a pure UI-state change
  // (opening the move dialog) does not re-walk every assignment/shift.
  const absenceNoteByStaff = useMemo(() => {
    const byStaff = new Map<string, string | null>();
    for (const member of staff) {
      byStaff.set(
        member.id,
        absenceNoteForStaff(member.id, assignmentsByStaff, shiftsByStaff),
      );
    }
    return byStaff;
  }, [staff, assignmentsByStaff, shiftsByStaff]);

  // "Für heute abwesend melden" jumps to Vertretung for today if it is in the
  // visible week, otherwise the Monday of that week (docs/05 Abschnitt 2.6).
  const absenceEntryDate = weekDays.includes(todayIso)
    ? todayIso
    : (weekDays[0] ?? todayIso);

  // Own person → /time-tracking; anyone else → the staff detail page's
  // Zeiterfassung tab (docs/05 Abschnitt 2.3 / 08 Abschnitt 3.4). The tab param
  // is forward-compatible: staff/[id] does not yet honor it (separate package).
  const timeTrackingHref = (member: StaffScheduleStaff): string =>
    currentStaffId != null && member.id === currentStaffId
      ? "/time-tracking"
      : `/staff/${member.id}?tab=zeiterfassung`;

  const buildMenuItems = (member: StaffScheduleStaff): OverflowMenuEntry[] => {
    const items: OverflowMenuEntry[] = [];
    // "Krank melden" (#1843) survives the reduced path — the row header there is
    // name-only otherwise, but the sick-report flow needs the same audience it
    // had before (Fertig-Kriterium 14). The navigation items are full-path only.
    if (onSickReport) {
      items.push({
        label: "Krank melden",
        icon: <Thermometer className="h-4 w-4" aria-hidden />,
        onClick: () => onSickReport(member),
      });
    }
    if (!reducedPath) {
      items.push({
        label: "Für heute abwesend melden",
        icon: <CalendarOff className="h-4 w-4" aria-hidden />,
        onClick: () => router.push(`/vertretung?d=${absenceEntryDate}`),
      });
      items.push({
        label: "Zeiterfassung öffnen",
        icon: <Clock className="h-4 w-4" aria-hidden />,
        onClick: () => router.push(timeTrackingHref(member)),
      });
    }
    return items;
  };

  const renderRowHeader = (member: StaffScheduleStaff): ReactNode => {
    const summary = reducedPath ? undefined : summaryByStaff.get(member.id);
    const absenceNote = reducedPath
      ? null
      : (absenceNoteByStaff.get(member.id) ?? null);
    const menuItems = buildMenuItems(member);
    return (
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <span className="block truncate text-sm font-medium text-gray-900">
            {member.lastName}, {member.firstName}
          </span>
          {summary && <WeeklySummaryIndicator summary={summary} />}
          {absenceNote && (
            <span
              className="mt-0.5 block text-[11px] font-medium"
              style={{ color: LOCATION_COLORS.SICK }}
            >
              {absenceNote}
            </span>
          )}
        </div>
        {menuItems.length > 0 && (
          <OverflowMenu
            items={menuItems}
            ariaLabel={`Aktionen für ${member.firstName} ${member.lastName}`}
          />
        )}
      </div>
    );
  };

  const renderShiftEntry = (
    member: StaffScheduleStaff,
    date: string,
    shift: StaffShift,
    dayIsAbsent: boolean,
  ): ReactNode => {
    const isReplacement = shift.originShiftId != null;
    // cancelled beats hatched; hatched only applies to non-cancelled blocks on
    // a day the person is absent (Abgleich B1, Fertig-Kriterium 12).
    const status = shift.cancelled
      ? "cancelled"
      : dayIsAbsent
        ? "hatched"
        : "default";
    const timeRange = formatShiftLabel(shift);
    const label = shiftLabel(shift, typesById);

    // Exactly one icon: only a regular series block carries Repeat; cancelled
    // and hatched blocks show none (Abgleich B1).
    const showSeriesIcon = status === "default" && shift.seriesId != null;
    const statusIcon = showSeriesIcon ? (
      <Repeat
        aria-label={
          shift.detached
            ? "Serie, für diese Woche angepasst"
            : "Teil einer Serie"
        }
        className={shift.detached ? "text-[#EAB308]" : "text-gray-400"}
      />
    ) : undefined;

    const ariaParts = [`${timeRange} ${label}`];
    if (shift.cancelled) {
      ariaParts.push("fällt aus");
    } else {
      if (isReplacement) ariaParts.push("Vertretung");
      if (dayIsAbsent) ariaParts.push("abwesend");
    }

    // "Verschieben nach" only on non-cancelled shifts: moving an ausgefallene
    // Schicht is pointless (Ersatz-Schichten are real shifts and keep the menu).
    // The trigger is a DOM sibling of the PlanBlock, never nested inside its
    // <button> — a button inside a button is invalid HTML — and always visible
    // (no hover) so it is reachable by touch and keyboard.
    const moveMenuItems: OverflowMenuEntry[] = shift.cancelled
      ? []
      : [
          {
            label: "Verschieben nach",
            icon: <ArrowRightLeft className="h-4 w-4" aria-hidden />,
            onClick: () => setMoveTarget({ member, shift }),
          },
        ];

    return (
      <div key={shift.id} className="space-y-0.5">
        <div className="flex items-center gap-0.5">
          <div className="min-w-0 flex-1">
            <PlanBlock
              size="compact"
              status={status}
              timeRange={timeRange}
              label={label}
              color={resolveShiftColor(shift, typesById)}
              statusIcon={statusIcon}
              onClick={() => onCellClick(member, date, shift)}
              aria-label={ariaParts.join(", ")}
            />
          </div>
          {moveMenuItems.length > 0 && (
            <OverflowMenu
              items={moveMenuItems}
              triggerSize="sm"
              ariaLabel={`Aktionen zur Schicht ${timeRange}`}
            />
          )}
        </div>
        {shift.cancelled ? (
          <p
            className="text-[11px] font-medium"
            style={{ color: LOCATION_COLORS.HOME }}
          >
            Fällt aus{shift.changeReason ? ` · ${shift.changeReason}` : ""}
          </p>
        ) : (
          <>
            {isReplacement && (
              <p
                className="text-[11px] font-medium"
                style={{ color: LOCATION_COLORS.OTHER_ROOM }}
              >
                Vertretung{shift.changeReason ? ` · ${shift.changeReason}` : ""}
              </p>
            )}
            {shift.breakMinutes > 0 && (
              <p className="text-[11px] text-gray-500">
                Pause {shift.breakMinutes} min
              </p>
            )}
          </>
        )}
      </div>
    );
  };

  const renderCell = (
    member: StaffScheduleStaff,
    column: ResourceGridColumn,
  ): ReactNode => {
    const date = column.key;
    const shifts = shiftsByStaff.get(member.id)?.get(date) ?? [];
    const assignments = assignmentsByStaff.get(member.id)?.get(date) ?? [];
    // Empty cell: let the ResourceGrid render its labelled empty-cell button.
    if (shifts.length === 0 && assignments.length === 0) return null;

    const dayIsAbsent = assignments.some((a) => a.isAbsent);
    const sortedShifts = [...shifts].sort((a, b) =>
      a.startTime.localeCompare(b.startTime),
    );

    return (
      <div className="group flex min-h-14 flex-col gap-1">
        {sortedShifts.map((shift) =>
          renderShiftEntry(member, date, shift, dayIsAbsent),
        )}
        {assignments.map((assignment) => (
          <AssignmentCard
            key={`${assignment.instanceId}-${assignment.staffId}`}
            assignment={assignment}
          />
        ))}
        {/* Add-shift affordance in a FILLED cell: hidden until the cell is
            hovered on hover-capable pointers (matches the retired week grid),
            but always visible on touch and reachable by keyboard
            (focus / focus-visible force it back to full opacity). */}
        <button
          type="button"
          onClick={() => onCellClick(member, date, null)}
          aria-label={createShiftAriaLabel(member, column)}
          className="flex h-7 w-full items-center justify-center rounded-md border border-dashed border-gray-200 text-gray-400 opacity-100 transition hover:bg-gray-50 hover:text-gray-600 focus:opacity-100 focus-visible:opacity-100 focus-visible:ring-2 focus-visible:ring-gray-900 focus-visible:outline-none [@media(hover:hover)_and_(pointer:fine)]:opacity-0 [@media(hover:hover)_and_(pointer:fine)]:group-hover:opacity-100"
        >
          <Plus className="h-4 w-4" aria-hidden />
        </button>
      </div>
    );
  };

  const legendEntries: PlanLegendEntry[] = useMemo(
    () => [
      ...shiftTypes.map((type) => ({
        key: `type-${type.id}`,
        label: type.name,
        color: type.color,
        variant: "bar" as const,
      })),
      { key: "state-absent", label: "Abwesend", variant: "hatched" },
      { key: "state-cancelled", label: "Fällt aus", variant: "cancelled" },
      {
        key: "state-substitute",
        label: "Vertretung",
        color: LOCATION_COLORS.OTHER_ROOM,
        variant: "bar",
      },
    ],
    [shiftTypes],
  );

  return (
    <div>
      <p id={scrollHintId} className="mb-2 text-xs text-gray-600 sm:hidden">
        Wische horizontal, um weitere Wochentage zu sehen.
      </p>
      <ResourceGrid
        columns={columns}
        rows={staff}
        getRowKey={(member) => member.id}
        renderRowHeader={renderRowHeader}
        renderCell={renderCell}
        columnMode="days"
        cornerHeader="Person"
        scrollHintId={scrollHintId}
        emptyCellLabel={createShiftAriaLabel}
        onEmptyCellClick={(member, column) =>
          onCellClick(member, column.key, null)
        }
        footer={
          <CapacityStrip
            rowLabel={
              <span title="Übergangslösung: Pausen sind in dieser Zahl nicht herausgerechnet.">
                Kapazität 12–16
              </span>
            }
            cells={weekDays.map((date) => ({
              key: date,
              content: capacityByDay.get(date) ?? 0,
            }))}
          />
        }
        ariaLabel="Dienstplan-Wochenansicht"
      />
      <PlanLegend
        className="mt-3 px-1"
        entries={legendEntries}
        aria-label="Legende Schichtarten und Zustände"
      />
      {moveTarget && (
        <ShiftMoveDialog
          isOpen
          shift={moveTarget.shift}
          sourceMember={moveTarget.member}
          staff={staff}
          shiftTypes={shiftTypes}
          onClose={() => setMoveTarget(null)}
          onDataChanged={() => {
            void refreshAfterMove();
          }}
        />
      )}
    </div>
  );
}
