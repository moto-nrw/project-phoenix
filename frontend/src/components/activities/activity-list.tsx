"use client";

import { useRouter } from "next/navigation";
import type { Activity } from "@/lib/activity-api";
import {
  formatActivityTimes,
  formatParticipantStatus,
} from "@/lib/activity-helpers";

interface ActivityListProps {
  activities: Activity[];
  onActivityClick?: (activity: Activity) => void;
  showDetails?: boolean;
  emptyMessage?: string;
}

export default function ActivityList({
  activities,
  onActivityClick,
  showDetails = true,
  emptyMessage = "Keine Aktivitäten vorhanden.",
}: ActivityListProps) {
  const router = useRouter();

  const handleActivityClick = (activity: Activity) => {
    if (onActivityClick) {
      onActivityClick(activity);
    } else {
      router.push(`/database/activities/${activity.id}`);
    }
  };

  if (!activities.length) {
    return (
      <div className="py-8 text-center">
        <p className="text-gray-500">{emptyMessage}</p>
      </div>
    );
  }

  return (
    <div className="space-y-2">
      {activities.map((activity) => (
        <div
          key={activity.id}
          onClick={() => handleActivityClick(activity)}
          className="group cursor-pointer rounded-lg border border-gray-100 bg-white p-4 shadow-sm transition-all duration-200 hover:translate-y-[-1px] hover:border-blue-200 hover:shadow-md"
        >
          <div className="flex items-center justify-between">
            <div className="flex items-center space-x-3">
              <div className="flex h-10 w-10 items-center justify-center rounded-full bg-gradient-to-r from-purple-400 to-pink-500 font-medium text-white">
                {activity.name.slice(0, 1).toUpperCase()}
              </div>

              <div className="flex flex-col">
                <span className="font-medium text-gray-900 transition-colors group-hover:text-purple-600">
                  {activity.name}
                </span>
                {showDetails && (
                  <span className="text-sm text-gray-500">
                    {activity.category_name &&
                      `Kategorie: ${activity.category_name}`}
                    {activity.supervisor_name &&
                      ` | Leitung: ${activity.supervisor_name}`}
                  </span>
                )}
              </div>
            </div>

            <div className="flex flex-col items-end space-y-1">
              <div className="flex items-center space-x-2">
                <span
                  className="text-sm font-medium"
                  title="Teilnehmer / Maximum"
                >
                  {formatParticipantStatus(activity)}
                </span>
                <div
                  className={`h-2.5 w-2.5 rounded-full ${
                    activity.is_open_ags ? "bg-green-500" : "bg-gray-300"
                  }`}
                  title={
                    activity.is_open_ags
                      ? "Offen für Anmeldungen"
                      : "Geschlossen für Anmeldungen"
                  }
                ></div>

                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  className="h-5 w-5 text-gray-400 transition-all duration-200 group-hover:translate-x-1 group-hover:text-purple-500"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M9 5l7 7-7 7"
                  />
                </svg>
              </div>

              {showDetails && activity.times && activity.times.length > 0 && (
                <span className="text-xs text-gray-500 italic">
                  {formatActivityTimes(activity)}
                </span>
              )}
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}
