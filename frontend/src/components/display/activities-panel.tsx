"use client";

import type {
  DashboardRunningActivity,
  DashboardUpcomingActivity,
} from "~/lib/display-api";
import { LOCATION_COLORS } from "~/lib/location-helper";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";

interface ActivitiesPanelProps {
  readonly running: DashboardRunningActivity[];
  readonly upcoming: DashboardUpcomingActivity[];
}

export function ActivitiesPanel({ running, upcoming }: ActivitiesPanelProps) {
  return (
    <section className="moto-content-surface rounded-2xl border p-6 shadow-sm lg:p-8">
      <div className="mb-5 flex items-center gap-4">
        <div className="flex h-14 w-14 shrink-0 items-center justify-center rounded-xl bg-gray-100">
          <MotoConceptIcon concept="activities" size={32} />
        </div>
        <h2 className="text-3xl font-bold text-gray-900">Aktivitäten</h2>
      </div>

      {running.length === 0 && upcoming.length === 0 ? (
        <p className="py-8 text-center text-2xl text-gray-400">
          Gerade keine Aktivitäten.
        </p>
      ) : (
        <div className="space-y-6">
          {running.length > 0 && (
            <ul className="space-y-3">
              {running.map((activity) => (
                <li
                  key={activity.id}
                  className="flex items-center gap-4 rounded-2xl border border-gray-200 bg-white p-4"
                >
                  <span
                    className="h-4 w-4 shrink-0 animate-pulse rounded-full"
                    style={{ backgroundColor: LOCATION_COLORS.GROUP_ROOM }}
                    aria-hidden
                  />
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-2xl font-semibold text-gray-900">
                      {activity.name}
                    </p>
                    <p className="truncate text-lg text-gray-500">
                      {activity.room_name || activity.category}
                    </p>
                  </div>
                  <p className="text-2xl font-bold text-gray-900 tabular-nums">
                    {activity.participants}
                    {activity.max_capacity != null && (
                      <span className="text-lg font-medium text-gray-400">
                        {" "}
                        / {activity.max_capacity}
                      </span>
                    )}
                  </p>
                </li>
              ))}
            </ul>
          )}

          {upcoming.length > 0 && (
            <div>
              <h3 className="mb-3 text-xl font-semibold text-gray-500">
                Später heute
              </h3>
              <ul className="space-y-2">
                {upcoming.map((activity) => (
                  <li
                    key={activity.id}
                    className="flex items-center gap-4 rounded-2xl border border-gray-200 bg-white p-4"
                  >
                    <p
                      className="text-2xl font-bold tabular-nums"
                      style={{ color: LOCATION_COLORS.OTHER_ROOM }}
                    >
                      {activity.start_time}
                    </p>
                    <div className="min-w-0 flex-1">
                      <p className="truncate text-2xl font-semibold text-gray-900">
                        {activity.name}
                      </p>
                      <p className="truncate text-lg text-gray-500">
                        {activity.room_name || activity.category}
                      </p>
                    </div>
                  </li>
                ))}
              </ul>
            </div>
          )}
        </div>
      )}
    </section>
  );
}
