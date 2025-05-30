"use client";

import type { Role } from "@/lib/auth-helpers";
import { Badge } from "@/components/ui";

interface RoleListItemProps {
  role: Role;
  onClick: (role: Role) => void;
}

export function RoleListItem({ role, onClick }: RoleListItemProps) {
  return (
    <div
      onClick={() => onClick(role)}
      className="group cursor-pointer rounded-lg border border-gray-200 bg-white p-4 shadow-sm transition-all duration-200 hover:border-blue-300 hover:shadow-md active:scale-[0.98]"
    >
      <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
        {/* Mobile/Desktop: Role Info */}
        <div className="flex items-start gap-3 md:items-center">
          {/* Role Icon */}
          <div className="flex h-10 w-10 md:h-12 md:w-12 shrink-0 items-center justify-center rounded-full bg-gradient-to-r from-blue-400 to-indigo-500 font-medium text-white text-sm md:text-base">
            {role.name.slice(0, 1).toUpperCase()}
          </div>
          
          {/* Role Details */}
          <div className="flex flex-col">
            <h3 className="font-medium text-gray-900 transition-colors group-hover:text-blue-600">
              {role.name}
            </h3>
            
            {/* Description */}
            {role.description && (
              <p className="text-sm text-gray-500 mt-1">
                {role.description}
              </p>
            )}

            {/* Permission Count - Mobile */}
            {role.permissions && role.permissions.length > 0 && (
              <div className="flex items-center gap-2 mt-2 md:hidden">
                <Badge variant="blue" size="sm">
                  {role.permissions.length} Berechtigungen
                </Badge>
              </div>
            )}
          </div>
        </div>

        {/* Mobile/Desktop: Status and Navigation */}
        <div className="flex items-center justify-between md:justify-end gap-4">
          {/* Permission Count - Desktop */}
          {role.permissions && role.permissions.length > 0 && (
            <div className="hidden md:block">
              <Badge variant="blue" size="sm">
                {role.permissions.length} Berechtigungen
              </Badge>
            </div>
          )}

          {/* Navigation Arrow */}
          <svg
            xmlns="http://www.w3.org/2000/svg"
            className="h-5 w-5 text-gray-400 transition-all duration-200 group-hover:translate-x-1 group-hover:text-blue-500"
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