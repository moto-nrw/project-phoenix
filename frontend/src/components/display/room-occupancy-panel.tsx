"use client";

import type { DashboardRoomOccupancy } from "~/lib/display-api";
import { LOCATION_COLORS } from "~/lib/location-helper";
import { ConceptIconTile } from "~/components/ui/concept-icon-tile";

interface RoomOccupancyPanelProps {
  readonly rooms: DashboardRoomOccupancy[];
  readonly totals?: {
    students_present: number;
    rooms_occupied: number;
    activities_running: number;
  };
}

/** Occupancy ratio → brand color: green = free, orange = filling, red = full. */
function occupancyColor(
  studentCount: number,
  capacity?: number | null,
): string {
  if (capacity == null || capacity <= 0) {
    return LOCATION_COLORS.UNKNOWN;
  }
  const ratio = studentCount / capacity;
  if (ratio >= 1) return LOCATION_COLORS.DANGER;
  if (ratio >= 0.8) return LOCATION_COLORS.SCHOOLYARD;
  return LOCATION_COLORS.GROUP_ROOM;
}

export function RoomOccupancyPanel({ rooms, totals }: RoomOccupancyPanelProps) {
  return (
    <section className="moto-content-surface rounded-2xl border p-6 shadow-sm lg:p-8">
      <div className="mb-6 flex flex-wrap items-center justify-between gap-4">
        <div className="flex items-center gap-4">
          <ConceptIconTile concept="rooms" variant="display" />
          <h2 className="text-3xl font-bold text-gray-900">Räume</h2>
        </div>
        {totals && (
          <p className="text-2xl text-gray-500">
            <span className="font-bold text-gray-900">
              {totals.students_present}
            </span>{" "}
            Kinder da
          </p>
        )}
      </div>

      {rooms.length === 0 ? (
        <p className="py-10 text-center text-2xl text-gray-400">
          Keine Räume eingerichtet.
        </p>
      ) : (
        <ul className="grid grid-cols-1 gap-4 md:grid-cols-2">
          {rooms.map((room) => {
            const color = occupancyColor(room.student_count, room.capacity);
            const hasCapacity = room.capacity != null && room.capacity > 0;
            const ratio = hasCapacity
              ? Math.min(room.student_count / (room.capacity ?? 1), 1)
              : 0;
            return (
              <li
                key={room.name}
                className="moto-content-surface rounded-2xl border p-5 shadow-sm"
              >
                <div className="flex items-baseline justify-between gap-3">
                  <p className="truncate text-2xl font-semibold text-gray-900">
                    {room.name}
                  </p>
                  <p
                    className="text-3xl font-bold whitespace-nowrap tabular-nums"
                    style={{ color }}
                  >
                    {room.student_count}
                    {hasCapacity && (
                      <span className="text-xl font-medium text-gray-400">
                        {" "}
                        / {room.capacity}
                      </span>
                    )}
                  </p>
                </div>
                {room.group_name && (
                  <p className="mt-1 truncate text-xl text-gray-500">
                    {room.group_name}
                  </p>
                )}
                {hasCapacity && (
                  <div className="mt-3 h-3 overflow-hidden rounded-full bg-gray-100">
                    <div
                      className="h-full rounded-full transition-all duration-700"
                      style={{
                        width: `${Math.round(ratio * 100)}%`,
                        backgroundColor: color,
                      }}
                    />
                  </div>
                )}
              </li>
            );
          })}
        </ul>
      )}
    </section>
  );
}
