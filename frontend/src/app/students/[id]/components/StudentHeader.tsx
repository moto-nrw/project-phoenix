// StudentHeader - extracted from student detail page
"use client";

import { LocationBadge } from "@/components/ui/location-badge";

interface StudentBadgeData {
  current_location?: string;
  location_since?: string;
  group_id?: string;
  group_name?: string;
}

interface StudentHeaderProps {
  readonly firstName: string;
  readonly secondName: string;
  readonly groupName?: string;
  readonly badgeStudent: StudentBadgeData;
  readonly myGroups: string[];
  readonly myGroupRooms: string[];
  readonly mySupervisedRooms: string[];
}

export function StudentHeader({
  firstName,
  secondName,
  groupName,
  badgeStudent,
  myGroups,
  myGroupRooms,
  mySupervisedRooms,
}: StudentHeaderProps) {
  return (
    <div className="mb-6">
      <div className="flex items-end justify-between gap-4">
        {/* Title */}
        <div className="ml-6 flex-1">
          <h1 className="text-2xl font-bold text-gray-900 md:text-3xl">
            {firstName} {secondName}
          </h1>
          {groupName && (
            <div className="mt-2 flex items-center gap-2 text-sm text-gray-600">
              <GroupIcon />
              <span className="truncate">{groupName}</span>
            </div>
          )}
        </div>

        {/* Status Badge */}
        <div className="mr-4 flex-shrink-0 pb-3">
          <LocationBadge
            student={badgeStudent}
            displayMode="contextAware"
            userGroups={myGroups}
            groupRooms={myGroupRooms}
            supervisedRooms={mySupervisedRooms}
            variant="modern"
            size="md"
            showLocationSince={true}
          />
        </div>
      </div>
    </div>
  );
}

function GroupIcon() {
  return (
    <svg
      className="h-4 w-4 text-gray-400"
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z"
      />
    </svg>
  );
}
