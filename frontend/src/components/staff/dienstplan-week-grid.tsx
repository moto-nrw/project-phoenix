"use client";

import { Plus } from "lucide-react";

import { Skeleton } from "~/components/ui/skeleton";
import type { Staff } from "~/lib/staff-api";
import { formatShiftLabel, type StaffShift } from "~/lib/shift-helpers";
import type { ShiftType } from "~/lib/shift-type-helpers";

// Neutral left-border color for shifts without a shift type (Schichtart).
const UNTYPED_SHIFT_COLOR = "#D1D5DB";

// Week matrix: one row per staff member, one column per weekday (Mo–Fr).
// A matrix layout doesn't fit the shared DataTable (row-per-record) shape,
// so this reuses the dense hand-rolled table styling precedent from
// staff-session-table.tsx instead.

interface DienstplanWeekGridProps {
  readonly staff: readonly Staff[];
  /** staffId -> date ("YYYY-MM-DD") -> shifts */
  readonly shiftsByStaff: Map<string, Map<string, StaffShift[]>>;
  /** The five weekday dates as "YYYY-MM-DD" (Monday first) */
  readonly weekDays: readonly string[];
  /** Today as "YYYY-MM-DD" for the column tint */
  readonly todayIso: string;
  /** Shift type lookup (id -> type) for per-shift color + label */
  readonly typesById: Map<string, ShiftType>;
  readonly isLoading: boolean;
  readonly onCellClick: (
    staff: Staff,
    date: string,
    shift: StaffShift | null,
  ) => void;
}

const DAY_LABELS = ["Mo", "Di", "Mi", "Do", "Fr"] as const;

function formatColumnDate(isoDate: string): string {
  const [, m, d] = isoDate.split("-");
  return `${d}.${m}.`;
}

export function DienstplanWeekGrid({
  staff,
  shiftsByStaff,
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
    <div className="max-w-full overflow-x-auto rounded-2xl border border-gray-100">
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
            return (
              <tr key={member.id} className="bg-white">
                <td className="sticky left-0 z-10 bg-white px-4 py-2 font-medium whitespace-nowrap text-gray-900">
                  {member.lastName}, {member.firstName}
                </td>
                {weekDays.map((date) => {
                  const shifts = byDate?.get(date) ?? [];
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
                          <Plus className="h-4 w-4" />
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
  );
}
