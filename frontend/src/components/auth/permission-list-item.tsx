"use client";

import type { Permission } from "@/lib/auth-helpers";
import { Badge } from "@/components/ui";

interface PermissionListItemProps {
  permission: Permission;
  onClick?: (permission: Permission) => void;
}

export function PermissionListItem({
  permission,
  onClick,
}: PermissionListItemProps) {
  const isClickable = !!onClick;

  return (
    <div
      onClick={isClickable ? () => onClick(permission) : undefined}
      className={`group rounded-lg border border-gray-200 bg-white p-4 shadow-sm transition-all duration-200 ${
        isClickable
          ? "cursor-pointer hover:border-blue-300 hover:shadow-md active:scale-[0.98]"
          : ""
      }`}
    >
      <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
        {/* Mobile/Desktop: Permission Info */}
        <div className="flex items-start gap-3 md:items-center">
          {/* Permission Icon */}
          <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-gradient-to-r from-amber-400 to-orange-500 text-sm font-medium text-white md:h-12 md:w-12 md:text-base">
            {permission.action.slice(0, 1).toUpperCase()}
          </div>

          {/* Permission Details */}
          <div className="flex flex-col">
            <h3
              className={`font-mono text-sm font-medium text-gray-900 ${
                isClickable ? "transition-colors group-hover:text-blue-600" : ""
              }`}
            >
              {permission.name}
            </h3>

            {/* Description */}
            <p className="mt-1 text-sm text-gray-600">
              {permission.description}
            </p>

            {/* Resource and Action Badges - Mobile */}
            <div className="mt-2 flex flex-wrap items-center gap-2 md:hidden">
              <Badge variant="gray" size="sm">
                {permission.resource}
              </Badge>
              <Badge variant="yellow" size="sm">
                {permission.action}
              </Badge>
            </div>
          </div>
        </div>

        {/* Mobile/Desktop: Badges and Navigation */}
        <div className="flex items-center justify-between gap-4 md:justify-end">
          {/* Resource and Action Badges - Desktop */}
          <div className="hidden items-center gap-2 md:flex">
            <Badge variant="gray" size="sm">
              {permission.resource}
            </Badge>
            <Badge variant="yellow" size="sm">
              {permission.action}
            </Badge>
          </div>

          {/* Navigation Arrow - Only show if clickable */}
          {isClickable && (
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
          )}
        </div>
      </div>
    </div>
  );
}
