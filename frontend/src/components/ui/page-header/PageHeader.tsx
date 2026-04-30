"use client";

import { OverflowMenu } from "./OverflowMenu";
import type { PageHeaderProps } from "./types";

export function PageHeader({
  title,
  badge,
  statusIndicator,
  actionButton,
  overflowMenu,
  className = "",
}: Readonly<PageHeaderProps>) {
  const hasOverflowMenu = overflowMenu !== undefined && overflowMenu.length > 0;
  const getStatusColor = (color: "green" | "yellow" | "red" | "gray") => {
    switch (color) {
      case "green":
        return "bg-green-500";
      case "yellow":
        return "bg-yellow-500";
      case "red":
        return "bg-red-500";
      case "gray":
        return "bg-gray-400";
    }
  };

  // Don't render anything if no title (conditional title pattern)
  if (!title) {
    return null;
  }

  return (
    <div className={`mb-6 md:hidden ${className}`}>
      <div className="flex items-center justify-between gap-4">
        {/* Title */}
        <div>
          <h1 className="text-2xl font-bold text-gray-900">{title}</h1>
        </div>

        {/* Action Button OR Badge and Status */}
        {(actionButton ?? statusIndicator ?? badge ?? hasOverflowMenu) && (
          <div className="mr-2 flex flex-shrink-0 items-center gap-2">
            {/* Action Button (priority over badge/status) */}
            {actionButton ?? (
              <>
                {/* Status Indicator Dot */}
                {statusIndicator && (
                  <div
                    className={`h-2.5 w-2.5 flex-shrink-0 rounded-full ${getStatusColor(statusIndicator.color)} ${statusIndicator.color === "green" ? "animate-pulse" : ""}`}
                    title={statusIndicator.tooltip}
                  />
                )}

                {/* Badge */}
                {badge && (
                  <div className="flex items-center gap-2 rounded-full border border-gray-100 bg-gray-50 px-3 py-1.5">
                    {badge.icon && (
                      <span className="text-gray-500">{badge.icon}</span>
                    )}
                    <span className="text-sm font-semibold text-gray-900">
                      {badge.count}
                    </span>
                    {badge.label && (
                      <span className="text-xs text-gray-500">
                        {badge.label}
                      </span>
                    )}
                  </div>
                )}
              </>
            )}

            {hasOverflowMenu ? <OverflowMenu items={overflowMenu} /> : null}
          </div>
        )}
      </div>
    </div>
  );
}
