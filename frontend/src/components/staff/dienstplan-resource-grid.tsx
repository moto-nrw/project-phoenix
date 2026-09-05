"use client";

import { ArrowRightLeft, Repeat, TriangleAlert } from "lucide-react";
import {
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";

import {
  ClosingDayChip,
  ClosingDayConfirmModal,
} from "~/components/planning/closing-day-marker";
import { ShiftMoveDialog } from "~/components/staff/shift-move-dialog";
import { CapacityStrip } from "~/components/ui/capacity-strip";
import { CoverageIndicator } from "~/components/ui/coverage-indicator";
import {
  OverflowMenu,
  type OverflowMenuEntry,
} from "~/components/ui/page-header/OverflowMenu";
import { PlanAddAffordance } from "~/components/ui/plan-add-affordance";
import { PlanBlock } from "~/components/ui/plan-block";
import { PlanLegend, type PlanLegendEntry } from "~/components/ui/plan-legend";
import {
  ResourceGrid,
  type ResourceGridColumn,
} from "~/components/ui/resource-grid";
import { Tooltip } from "~/components/ui/tooltip";
import type { ClosingDayRange } from "~/lib/closing-day-helpers";
import { PLAN_CACHE_KEY_PREFIXES } from "~/lib/hooks/use-dienstplan-data";
import { BELOW_LG, useMediaQuery } from "~/lib/hooks/use-media-query";
import { LOCATION_COLORS } from "~/lib/location-helper";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import { MOTO_CONCEPTS } from "~/lib/moto-concepts";
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
  /** OGS-Schließtage der Woche, keyed YYYY-MM-DD → Grund (#2032). Markiert
   *  die Spalte und löst vor dem Anlegen einer Schicht die Rückfrage aus. */
  readonly closingDays?: ReadonlyMap<string, string>;
  /** While the lookup is pending, empty cells stay inactive so creation
   *  cannot bypass the closing-day confirmation. */
  readonly closingDaysLoading?: boolean;
  /** Alle gespeicherten Schließtag-Zeiträume — für den Zieltag im
   *  „Verschieben nach“-Dialog, der außerhalb der Woche liegen kann. */
  readonly closingDayRanges?: readonly ClosingDayRange[];
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
  if (assignment.isAbsent) return LOCATION_COLORS.DANGER;
  // WARNING, not SICK: SICK is red now, which would match the isAbsent
  // DANGER accent one line above and flatten the two states into one.
  if (assignment.coverageStatus === "uncovered") return LOCATION_COLORS.WARNING;
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
        style={{ color: LOCATION_COLORS.DANGER }}
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
            // Same condition as the WARNING accent in assignmentAccentColor —
            // must be the same amber, or the card gets an amber border with a
            // red triangle and the triangle matches the DANGER "Abwesend"
            // state instead.
            style={{ color: LOCATION_COLORS.WARNING }}
            aria-label="Nicht vollständig durch Schicht abgedeckt"
          />
        ) : null}
      </div>
      <span className="block truncate text-xs font-medium text-gray-700">
        {assignment.activityTitle}
      </span>
      <span className="flex items-center gap-1 truncate text-xs text-gray-500">
        <MotoDuotoneIcon
          icon={MOTO_CONCEPTS.rooms.icon}
          tone="neutral"
          size={12}
          className="shrink-0"
        />
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
  closingDays,
  closingDaysLoading = false,
  closingDayRanges,
  typesById,
  shiftTypes,
  reducedPath = false,
  currentStaffId = null,
  onCellClick,
  onSickReport,
}: DienstplanResourceGridProps) {
  const router = useTenantRouter();
  const scrollHintId = useId();

  // Schließtag-Rückfrage (#2032): eine NEUE Schicht auf einem Schließtag wird
  // einmal bestätigt, dann wie gewohnt im Bearbeiten-Dialog geöffnet. Das
  // Öffnen einer bestehenden Schicht fragt nicht — die liegt schon dort.
  const [closingDayPrompt, setClosingDayPrompt] = useState<{
    member: StaffScheduleStaff;
    date: string;
    reason: string;
  } | null>(null);

  const openCell = (
    member: StaffScheduleStaff,
    date: string,
    shift: StaffShift | null,
  ) => {
    const reason = shift === null ? closingDays?.get(date) : undefined;
    if (reason !== undefined) {
      setClosingDayPrompt({ member, date, reason });
      return;
    }
    onCellClick(member, date, shift);
  };

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
      weekDays.map((date, i) => {
        const closingReason = closingDays?.get(date);
        return {
          key: date,
          label: DAY_LABELS[i] ?? "",
          sublabel: formatColumnDate(date),
          isCurrent: date === todayIso,
          isMuted: closingReason !== undefined,
          headerNote:
            closingReason === undefined ? undefined : (
              <ClosingDayChip reason={closingReason} />
            ),
        };
      }),
    [weekDays, todayIso, closingDays],
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
        icon: <MotoConceptIcon concept="sick" size={18} />,
        onClick: () => onSickReport(member),
      });
    }
    if (!reducedPath) {
      items.push({
        label: "Für heute abwesend melden",
        icon: <MotoConceptIcon concept="notArrival" size={18} />,
        onClick: () => router.push(`/vertretung?d=${absenceEntryDate}`),
      });
      items.push({
        label: "Zeiterfassung öffnen",
        icon: <MotoConceptIcon concept="timeTracking" size={18} />,
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
        className={shift.detached ? "text-moto-amber" : "text-gray-400"}
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
              onClick={() => openCell(member, date, shift)}
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
            style={{ color: LOCATION_COLORS.DANGER }}
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

    // Die Mindesthöhe der Zelle liegt im ResourceGrid, damit leere und gefüllte
    // Zellen dieselbe Zeilenhöhe ergeben (Issue #2026).
    return (
      <div className="group flex flex-1 flex-col gap-1">
        {sortedShifts.map((shift) =>
          renderShiftEntry(member, date, shift, dayIsAbsent),
        )}
        {assignments.map((assignment) => (
          <AssignmentCard
            key={`${assignment.instanceId}-${assignment.staffId}`}
            assignment={assignment}
          />
        ))}
        {/* Anlege-Geste in einer GEFÜLLTEN Zelle: dieselbe Fläche wie in einer
            leeren Zelle und im Stundenraster des Betreuungsplans (#2031), nur
            schmaler, weil darüber schon Inhalt steht. Ein- und Ausblendung
            steckt in PlanAddAffordance; `group` hier am Button, damit auch
            Tastaturfokus die Fläche zeigt (die Zelle selbst ist nicht
            fokussierbar). */}
        <button
          type="button"
          disabled={closingDaysLoading}
          onClick={() => openCell(member, date, null)}
          aria-label={createShiftAriaLabel(member, column)}
          className="group flex w-full rounded-md focus-visible:ring-2 focus-visible:ring-gray-900 focus-visible:outline-none disabled:cursor-wait disabled:opacity-40"
        >
          <PlanAddAffordance size="inline" />
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

  // Ausgewählter Tag der mobilen Tagesansicht. Voreinstellung ist heute, wenn
  // es in der gezeigten Woche liegt, sonst der Wochenanfang — dieselbe Regel
  // wie im Tagesstreifen des Betreuungsplan-Rasters.
  const todayColumnIndex = weekDays.indexOf(todayIso);
  const weekStart = weekDays[0];
  const [mobileDayIndex, setMobileDayIndex] = useState(
    todayColumnIndex >= 0 ? todayColumnIndex : 0,
  );
  const mobileWeekStart = useRef(weekStart);
  useEffect(() => {
    if (mobileWeekStart.current === weekStart) return;
    mobileWeekStart.current = weekStart;
    setMobileDayIndex(todayColumnIndex >= 0 ? todayColumnIndex : 0);
  }, [todayColumnIndex, weekStart]);
  const safeMobileDayIndex = Math.min(
    Math.max(mobileDayIndex, 0),
    Math.max(columns.length - 1, 0),
  );
  const mobileColumn = columns[safeMobileDayIndex];

  // Tagesansicht und Wochenmatrix schließen einander aus — sie dürfen NICHT
  // beide im Dokument stehen und per `lg:hidden` weggeblendet werden: jede
  // Person, jeder Schichtblock und jedes Aktionsmenü läge sonst doppelt im
  // Baum, Screenreader läsen den Plan zweimal vor und die Tabulator-Reihenfolge
  // liefe durch eine unsichtbare Kopie.
  const isCompact = useMediaQuery(BELOW_LG);

  return (
    <div>
      {/* Unterhalb lg ersetzt eine Tagesansicht die Wochenmatrix: Personen×Tage
          passt nicht auf ein Telefon — die Namensspalte fraß dort die halbe
          Breite, sichtbar blieb rund eine Tagesspalte, und der Rest der Woche
          lag hinter einer Wischgeste. Gezeigt wird EIN Tag, die Personen
          untereinander. Es ist bewusst dieselbe Darstellung wie im
          Betreuungsplan, damit die drei Planungsflächen sich mobil gleich
          verhalten. Zeilenkopf und Zelleninhalt sind exakt dieselben
          Render-Funktionen wie in der Matrix, also bleiben Menüs, Blöcke und
          die Anlege-Geste identisch. */}
      {isCompact && mobileColumn ? (
        <div>
          <div className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm">
            <div className="flex gap-1 border-b border-gray-200 bg-white p-2">
              {columns.map((column, index) => {
                const isSelected = index === safeMobileDayIndex;
                return (
                  <button
                    key={column.key}
                    type="button"
                    onClick={() => setMobileDayIndex(index)}
                    aria-pressed={isSelected}
                    className={`flex min-w-0 flex-1 flex-col items-center gap-0.5 rounded-lg px-1 py-1.5 transition-colors focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none ${
                      isSelected
                        ? "bg-gray-900 text-white"
                        : "text-gray-600 hover:bg-gray-100"
                    }`}
                  >
                    <span
                      className={`text-[10px] font-medium tracking-wide uppercase ${
                        isSelected ? "text-white/80" : "text-gray-500"
                      }`}
                    >
                      {column.label}
                    </span>
                    <span className="text-sm font-semibold tabular-nums">
                      {column.sublabel}
                    </span>
                    {column.headerNote}
                    {column.isCurrent && !isSelected && (
                      <span
                        aria-hidden
                        className="bg-moto-red h-1 w-1 rounded-full"
                      />
                    )}
                  </button>
                );
              })}
            </div>

            <ul className="divide-y divide-gray-100">
              {staff.map((member) => (
                <li key={member.id} className="px-3 py-2.5">
                  {renderRowHeader(member)}
                  <div className="mt-1.5">
                    {renderCell(member, mobileColumn) ?? (
                      // Leerer Tag: dieselbe Anlege-Fläche, die das Raster in
                      // einer leeren Zelle rendert — ohne sie ließe sich mobil
                      // keine Schicht anlegen.
                      <button
                        type="button"
                        disabled={closingDaysLoading}
                        onClick={() => openCell(member, mobileColumn.key, null)}
                        aria-label={createShiftAriaLabel(member, mobileColumn)}
                        className="group flex w-full rounded-md focus-visible:ring-2 focus-visible:ring-gray-900 focus-visible:outline-none disabled:cursor-wait disabled:opacity-40"
                      >
                        <PlanAddAffordance size="inline" />
                      </button>
                    )}
                  </div>
                </li>
              ))}
            </ul>

            <div className="flex items-center justify-between border-t border-gray-200 bg-gray-50 px-3 py-2 text-xs text-gray-600">
              <span title="Übergangslösung: Pausen sind in dieser Zahl nicht herausgerechnet.">
                Kapazität 12–16
              </span>
              <span className="font-semibold text-gray-900 tabular-nums">
                {capacityByDay.get(mobileColumn.key) ?? 0}
              </span>
            </div>
            <div className="border-t border-gray-200 px-3 py-2">
              <PlanLegend
                entries={legendEntries}
                aria-label="Legende Schichtarten und Zustände"
              />
            </div>
          </div>
        </div>
      ) : (
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
            onEmptyCellClick={
              closingDaysLoading
                ? undefined
                : (member, column) => openCell(member, column.key, null)
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
            legend={
              <PlanLegend
                entries={legendEntries}
                aria-label="Legende Schichtarten und Zustände"
              />
            }
          />
        </div>
      )}

      {moveTarget && (
        <ShiftMoveDialog
          isOpen
          shift={moveTarget.shift}
          sourceMember={moveTarget.member}
          staff={staff}
          shiftTypes={shiftTypes}
          closingDayRanges={closingDayRanges}
          onClose={() => setMoveTarget(null)}
          onDataChanged={() => {
            void refreshAfterMove();
          }}
        />
      )}
      {closingDayPrompt && (
        <ClosingDayConfirmModal
          dateISO={closingDayPrompt.date}
          reason={closingDayPrompt.reason}
          subject="schicht"
          onCancel={() => setClosingDayPrompt(null)}
          onConfirm={() => {
            const { member, date } = closingDayPrompt;
            setClosingDayPrompt(null);
            onCellClick(member, date, null);
          }}
        />
      )}
    </div>
  );
}
