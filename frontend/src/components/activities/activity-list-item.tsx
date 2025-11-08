"use client";

import type { Activity } from "@/lib/activity-helpers";
import {
  formatActivityTimes,
  formatParticipantStatus,
} from "@/lib/activity-helpers";
import { Badge } from "@/components/ui";

interface ActivityListItemProps {
  activity: Activity;
  onClick: (activity: Activity) => void;
}

export function ActivityListItem({ activity, onClick }: ActivityListItemProps) {
  return (
    <div
      onClick={() => onClick(activity)}
      className="group cursor-pointer rounded-lg border border-gray-200 bg-white p-4 shadow-sm transition-all duration-200 hover:border-blue-300 hover:shadow-md active:scale-[0.98]"
    >
      <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
        {/* Mobile/Desktop: Activity Info */}
        <div className="flex items-start gap-3 md:items-center">
          {/* Activity Icon */}
          <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-gradient-to-r from-purple-400 to-pink-500 text-sm font-medium text-white md:h-12 md:w-12 md:text-base">
            {activity.name.slice(0, 1).toUpperCase()}
          </div>

          {/* Activity Details */}
          <div className="flex flex-col">
            <h3 className="font-medium text-gray-900 transition-colors group-hover:text-purple-600">
              {activity.name}
            </h3>

            {/* Category and Supervisor Badges */}
            <div className="mt-2 flex flex-wrap items-center gap-2">
              {activity.category_name && (
                <Badge variant="purple" size="sm">
                  {activity.category_name}
                </Badge>
              )}
              {activity.supervisor_name && (
                <Badge variant="blue" size="sm">
                  Leitung: {activity.supervisor_name}
                </Badge>
              )}
            </div>

            {/* Schedule Times - Mobile */}
            {activity.times && activity.times.length > 0 && (
              <span className="mt-2 text-xs text-gray-500 italic md:hidden">
                {formatActivityTimes(activity)}
              </span>
            )}
          </div>
        </div>

        {/* Mobile/Desktop: Status and Navigation */}
        <div className="flex items-center justify-between gap-4 md:justify-end">
          {/* Schedule Times - Desktop */}
          {activity.times && activity.times.length > 0 && (
            <span className="hidden text-xs text-gray-500 italic md:block">
              {formatActivityTimes(activity)}
            </span>
          )}

          {/* Participant Status and Open Badge */}
          <div className="flex items-center gap-2">
            <span className="text-sm font-medium" title="Teilnehmer / Maximum">
              {formatParticipantStatus(activity)}
            </span>

            <Badge
              variant={activity.is_open_ags ? "green" : "gray"}
              className="text-xs"
            >
              {activity.is_open_ags ? "Offen" : "Geschlossen"}
            </Badge>
          </div>

          {/* Navigation Arrow */}
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
      </div>
    </div>
  );
}
