"use client";

import { ConceptIconTile } from "~/components/ui/concept-icon-tile";
import { ContextHelpLink } from "~/components/help/context-help-link";
import { OverflowMenu } from "./OverflowMenu";
import { StatusIndicator } from "./StatusIndicator";
import type { PageHeaderProps } from "./types";

export function PageHeader({
  title,
  concept,
  badge,
  statusIndicator,
  actionButton,
  helpTopic,
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
            <ConceptIconTile concept={concept} variant="page" />
          ) : null}
          <div className="flex min-w-0 items-center gap-1">
            <h1 className="truncate text-2xl font-bold text-gray-900">
              {title}
            </h1>
            {helpTopic ? (
              <ContextHelpLink topic={helpTopic} className="size-11" />
            ) : null}
          </div>
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
