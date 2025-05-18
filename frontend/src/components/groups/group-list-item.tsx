"use client";

import type { Group } from "@/lib/api";

interface GroupListItemProps {
  group: Group;
  onClick: () => void;
}

export default function GroupListItem({ group, onClick }: GroupListItemProps) {
  console.log("Rendering GroupListItem for:", group);
  
  return (
    <div
      className="group-item group flex cursor-pointer items-center justify-between rounded-lg border border-gray-100 bg-white p-4 shadow-sm transition-all duration-200 hover:translate-y-[-1px] hover:border-blue-200 hover:shadow-md"
      onClick={onClick}
    >
      <div className="flex flex-col transition-transform duration-200 group-hover:translate-x-1">
        <span className="font-semibold text-gray-900 transition-colors duration-200 group-hover:text-blue-600">
          {group.name}
        </span>
        <span className="text-sm text-gray-500">
          {group.room_name
            ? `Raum: ${group.room_name}`
            : "Kein Raum zugewiesen"}
          {group.student_count !== undefined
            ? ` | Schüler: ${group.student_count}`
            : ""}
        </span>
      </div>
      <svg
        xmlns="http://www.w3.org/2000/svg"
        className="h-5 w-5 text-gray-400 transition-all duration-200 group-hover:translate-x-1 group-hover:transform group-hover:text-blue-500"
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
  );
}
