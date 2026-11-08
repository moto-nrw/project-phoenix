"use client";

import { ClosingDayChip } from "~/components/planning/closing-day-marker";
import {
  resolveShiftColor,
  shiftLabel,
} from "~/components/staff/dienstplan-person-week-grid";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { plannedWeekMinutes } from "~/lib/own-dienstplan-helpers";
import {
  formatColumnDate,
  formatPlannedHours,
  type StaffShift,
} from "~/lib/shift-helpers";
import type { ShiftType } from "~/lib/shift-type-helpers";
import { ColorBar } from "~/components/ui/color-bar";
import { ListGroupHeader } from "~/components/ui/list-group-header";

// Handy-Fassung des eigenen Dienstplans (#3821), wie „Mein Kalender“ auf dem
// Handy: statt des Wochenrasters, das man seitlich wischen müsste, eine
// Liste je Tag. Nur lesend; Tage ohne Schicht fallen weg.

// Dieselbe Tageszeile wie „Mein Kalender“ auf dem Handy: „Mo 05.10.“.
const WEEKDAY_NAMES = ["Mo", "Di", "Mi", "Do", "Fr", "Sa", "So"] as const;

interface OwnWeekAgendaProps {
  /** Die Tage der Woche ab Montag ("YYYY-MM-DD"). */
  readonly weekDays: readonly string[];
  readonly shiftsByDate: ReadonlyMap<string, readonly StaffShift[]>;
  readonly typesById: Map<string, ShiftType>;
  readonly closingDays?: ReadonlyMap<string, string>;
}

export function OwnWeekAgenda({
  weekDays,
  shiftsByDate,
  typesById,
  closingDays,
}: OwnWeekAgendaProps) {
  const days = weekDays
    .map((date, index) => ({
      date,
      label: `${WEEKDAY_NAMES[index] ?? ""} ${formatColumnDate(date)}`,
      shifts: [...(shiftsByDate.get(date) ?? [])].sort((a, b) =>
        a.startTime.localeCompare(b.startTime),
      ),
    }))
    .filter((day) => day.shifts.length > 0);

  return (
    <SectionCard padding="none" testId="own-week-agenda">
      <div className="divide-y divide-gray-100">
        <p className="px-4 py-3 text-xs text-gray-500">
          Nur zur Information. Ihre Schichten plant die Leitung im Dienstplan.
        </p>
        {days.map((day) => {
          const closingReason = closingDays?.get(day.date);
          return (
            <section key={day.date} className="divide-y divide-gray-100">
              <ListGroupHeader
                as="h2"
                title={day.label}
                addon={
                  closingReason === undefined ? undefined : (
                    <ClosingDayChip reason={closingReason} />
                  )
                }
                meta={formatPlannedHours(plannedWeekMinutes(day.shifts))}
              />
              {day.shifts.map((shift) => (
                <AgendaShiftRow
                  key={shift.id}
                  shift={shift}
                  typesById={typesById}
                />
              ))}
            </section>
          );
        })}
      </div>
    </SectionCard>
  );
}

function AgendaShiftRow({
  shift,
  typesById,
}: Readonly<{ shift: StaffShift; typesById: Map<string, ShiftType> }>) {
  // Eine ausgefallene Schicht zeigt keine Farbe ihrer Art, wie im Raster.
  const color = shift.cancelled
    ? undefined
    : resolveShiftColor(shift, typesById);
  const details = [
    shift.breakMinutes > 0 ? `Pause ${shift.breakMinutes} min` : null,
    shift.originShiftId == null ? null : "Vertretung",
  ].filter(Boolean);
  return (
    <div
      className={`flex items-center gap-3 px-4 py-3 ${
        shift.cancelled ? "opacity-70" : ""
      }`}
    >
      <div className="flex w-12 shrink-0 flex-col items-start leading-tight tabular-nums">
        <span className="text-sm font-semibold text-gray-900">
          {shift.startTime}
        </span>
        <span className="text-xs text-gray-500">{shift.endTime}</span>
      </div>
      <ColorBar color={color} size="lg" />
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-1.5">
          <span
            className={`truncate text-sm font-semibold text-gray-900 ${
              shift.cancelled ? "line-through" : ""
            }`}
          >
            {shiftLabel(shift, typesById)}
          </span>
          {shift.cancelled && (
            <StatusBadge label="Fällt aus" tone="gray" compact />
          )}
        </div>
        {details.length > 0 && (
          <div className="mt-0.5 truncate text-xs text-gray-500">
            {details.join(" · ")}
          </div>
        )}
      </div>
    </div>
  );
}
