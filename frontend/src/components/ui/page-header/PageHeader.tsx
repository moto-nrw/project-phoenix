"use client";

import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { OverflowMenu } from "./OverflowMenu";
import { StatusIndicator } from "./StatusIndicator";
import type { PageHeaderProps } from "./types";

export function PageHeader({
  title,
  concept,
  badge,
  statusIndicator,
  actionButton,
  overflowMenu,
  className = "",
}: Readonly<PageHeaderProps>) {
  const hasOverflowMenu = overflowMenu !== undefined && overflowMenu.length > 0;

  // Don't render anything if no title (conditional title pattern)
  if (!title) {
    return null;
  }

  return (
    <div className={`mb-6 md:hidden ${className}`}>
      <div className="flex items-center justify-between gap-4">
        {/* Title, optionally preceded by a concept icon tile */}
        <div className="flex min-w-0 items-center gap-3">
          {concept ? (
            <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-gray-100 sm:h-12 sm:w-12">
              <MotoConceptIcon concept={concept} size={26} />
            </div>
          ) : null}
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
                  <StatusIndicator
                    color={statusIndicator.color}
                    tooltip={statusIndicator.tooltip}
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
