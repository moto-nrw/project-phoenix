"use client";

/* oxlint-disable jsx-a11y/no-noninteractive-tabindex -- the labeled horizontal scroll region must be keyboard-focusable */

import { MapPin, Plus, TriangleAlert } from "lucide-react";

import { Skeleton } from "~/components/ui/skeleton";
import { LOCATION_COLORS } from "~/lib/location-helper";
import {
  formatShiftLabel,
  type StaffScheduleAssignment,
  type StaffScheduleStaff,
  type StaffShift,
} from "~/lib/shift-helpers";
import type { ShiftType } from "~/lib/shift-type-helpers";

// Neutral left-border color for shifts without a shift type (Schichtart).
const UNTYPED_SHIFT_COLOR = "#D1D5DB";

// Week matrix: one row per staff member, one column per weekday (Mo–Fr).
// A matrix layout doesn't fit the shared DataTable (row-per-record) shape,
// so this reuses the dense hand-rolled table styling precedent from
// staff-session-table.tsx instead.

interface DienstplanWeekGridProps {
  readonly staff: readonly StaffScheduleStaff[];
  /** staffId -> date ("YYYY-MM-DD") -> shifts */
  readonly shiftsByStaff: Map<string, Map<string, StaffShift[]>>;
  /** staffId -> date ("YYYY-MM-DD") -> concrete Betreuungsplan assignments */
  readonly assignmentsByStaff: Map<
    string,
    Map<string, StaffScheduleAssignment[]>
  >;
  /** The five weekday dates as "YYYY-MM-DD" (Monday first) */
  readonly weekDays: readonly string[];
  /** Today as "YYYY-MM-DD" for the column tint */
  readonly todayIso: string;
  /** Shift type lookup (id -> type) for per-shift color + label */
  readonly typesById: Map<string, ShiftType>;
  readonly isLoading: boolean;
  readonly onCellClick: (
    staff: StaffScheduleStaff,
    date: string,
    shift: StaffShift | null,
  ) => void;
}

const DAY_LABELS = ["Mo", "Di", "Mi", "Do", "Fr"] as const;

function formatColumnDate(isoDate: string): string {
  const [, m, d] = isoDate.split("-");
  return `${d}.${m}.`;
}

function assignmentAccentColor(assignment: StaffScheduleAssignment): string {
  if (assignment.isAbsent) return LOCATION_COLORS.HOME;
  if (assignment.coverageStatus === "uncovered") return LOCATION_COLORS.SICK;
  if (assignment.isSubstitute) return LOCATION_COLORS.OTHER_ROOM;
  return LOCATION_COLORS.UNKNOWN;
}

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

export function DienstplanWeekGrid({
  staff,
  shiftsByStaff,
  assignmentsByStaff,
  weekDays,
  todayIso,
  typesById,
  isLoading,
  onCellClick,
}: DienstplanWeekGridProps) {
  if (isLoading) {
    return (
      <div className="space-y-2">
        {Array.from({ length: 6 }, (_, i) => (
          <Skeleton key={i} className="h-12 w-full" />
        ))}
      </div>
    );
  }

  if (staff.length === 0) {
    return (
      <p className="py-12 text-center text-sm text-gray-500">
        Keine Mitarbeitenden gefunden.
      </p>
    );
  }

  return (
    <div>
      <p
        id="dienstplan-scroll-hint"
        className="mb-2 text-xs text-gray-600 sm:hidden"
      >
        Wische horizontal, um weitere Wochentage zu sehen.
      </p>
      <div
        role="region"
        aria-label="Dienstplan-Wochenansicht"
        aria-describedby="dienstplan-scroll-hint"
        tabIndex={0}
        onKeyDown={(event) => {
          if (event.key !== "ArrowLeft" && event.key !== "ArrowRight") return;
          event.preventDefault();
          event.currentTarget.scrollBy({
            left: event.key === "ArrowLeft" ? -240 : 240,
            behavior: "smooth",
          });
        }}
        className="max-w-full overflow-x-auto overscroll-x-contain rounded-2xl border border-gray-100 focus-visible:ring-2 focus-visible:ring-[#83CD2D] focus-visible:outline-none"
      >
        <table className="w-full min-w-[960px] border-collapse text-sm">
          <thead>
            <tr className="bg-gray-50 text-left text-xs tracking-wider text-gray-500 uppercase">
              <th className="sticky left-0 z-10 bg-gray-50 px-4 py-3 font-semibold">
                Mitarbeiter
              </th>
              {weekDays.map((date, i) => (
                <th
                  key={date}
                  className={`px-3 py-3 font-semibold ${date === todayIso ? "bg-amber-50/60" : ""}`}
                >
                  {DAY_LABELS[i]} {formatColumnDate(date)}
                </th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100">
            {staff.map((member) => {
              const byDate = shiftsByStaff.get(member.id);
              const assignmentsByDate = assignmentsByStaff.get(member.id);
              return (
                <tr key={member.id} className="bg-white">
                  <th
                    scope="row"
                    className="sticky left-0 z-10 bg-white px-4 py-2 text-left font-medium whitespace-nowrap text-gray-900"
                  >
                    {member.lastName}, {member.firstName}
                  </th>
                  {weekDays.map((date) => {
                    const shifts = byDate?.get(date) ?? [];
                    const assignments = assignmentsByDate?.get(date) ?? [];
                    return (
                      <td
                        key={date}
                        className={`group px-3 py-2 align-top ${date === todayIso ? "bg-amber-50/40" : ""}`}
                      >
                        <div className="flex min-h-9 flex-col gap-1">
                          {shifts.map((shift) => {
                            const type = shift.shiftTypeId
                              ? typesById.get(shift.shiftTypeId)
                              : undefined;
                            return (
                              <button
                                key={shift.id}
                                type="button"
                                onClick={() => onCellClick(member, date, shift)}
                                style={{
                                  borderLeftColor:
                                    type?.color ?? UNTYPED_SHIFT_COLOR,
                                  // Light tint of the shift-type color as the slot
                                  // background (~10% opacity via 8-digit hex).
                                  backgroundColor: type
                                    ? `${type.color}1A`
                                    : undefined,
                                }}
                                className="w-full rounded-md border border-l-2 border-gray-200 bg-white px-2 py-1 text-left transition-shadow hover:shadow-sm"
                              >
                                <span className="font-semibold tabular-nums">
                                  {formatShiftLabel(shift)}
                                </span>
                                {type && (
                                  <span className="block truncate text-xs text-gray-600">
                                    {type.name}
                                  </span>
                                )}
                                {shift.breakMinutes > 0 && (
                                  <span className="block text-xs text-gray-500">
                                    Pause {shift.breakMinutes} min
                                  </span>
                                )}
                              </button>
                            );
                          })}
                          {assignments.map((assignment) => (
                            <AssignmentCard
                              key={`${assignment.instanceId}-${assignment.staffId}`}
                              assignment={assignment}
                            />
                          ))}
                          <button
                            type="button"
                            onClick={() => onCellClick(member, date, null)}
                            aria-label={`Schicht anlegen für ${member.firstName} ${member.lastName} am ${formatColumnDate(date)}`}
                            className={`flex h-7 w-full items-center justify-center rounded-md border border-dashed border-gray-200 text-gray-400 transition-opacity hover:bg-gray-50 hover:text-gray-600 focus:opacity-100 ${
                              shifts.length > 0
                                ? "opacity-100 [@media(hover:hover)_and_(pointer:fine)]:opacity-0 [@media(hover:hover)_and_(pointer:fine)]:group-hover:opacity-100"
                                : "opacity-60 hover:opacity-100"
                            }`}
                          >
                            <Plus className="h-4 w-4" aria-hidden />
                          </button>
                        </div>
                      </td>
                    );
                  })}
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}
