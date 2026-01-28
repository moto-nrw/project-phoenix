// Search row helper components - extracted to reduce complexity in PageHeaderWithSearch
"use client";

import type React from "react";
import { StatusIndicator } from "./StatusIndicator";
import { BadgeDisplayCompact, BadgeDisplay } from "./BadgeDisplay";

interface StatusIndicatorConfig {
  color: "green" | "yellow" | "red" | "gray";
  tooltip?: string;
}

interface BadgeConfig {
  count: number | string;
  label?: string;
  icon?: React.ReactNode;
}

interface InlineStatusBadgeProps {
  readonly statusIndicator?: StatusIndicatorConfig;
  readonly badge?: BadgeConfig;
  readonly variant: "mobile" | "desktop";
}

/**
 * Inline status and badge display for search rows
 */
export function InlineStatusBadge({
  statusIndicator,
  badge,
  variant,
}: InlineStatusBadgeProps) {
  if (!statusIndicator && !badge) return null;

  const containerClass =
    variant === "mobile"
      ? "flex flex-shrink-0 items-center gap-2"
      : "ml-auto flex flex-shrink-0 items-center gap-3";

  return (
    <div className={containerClass}>
      {statusIndicator && (
        <StatusIndicator
          color={statusIndicator.color}
          tooltip={statusIndicator.tooltip}
        />
      )}
      {badge &&
        (variant === "mobile" ? (
          <BadgeDisplayCompact count={badge.count} icon={badge.icon} />
        ) : (
          <BadgeDisplay
            count={badge.count}
            label={badge.label}
            icon={badge.icon}
            showLabel={true}
          />
        ))}
    </div>
  );
}

/**
 * Determines if inline status/badge should be shown
 */
export function shouldShowInlineStatusBadge(
  hasTabs: boolean,
  hasTitle: boolean,
  hasActionButton: boolean,
  statusIndicator?: StatusIndicatorConfig,
  badge?: BadgeConfig,
): boolean {
  // Only show if: no tabs, no title, no action button, and has status or badge
  return (
    !hasTabs &&
    !hasTitle &&
    !hasActionButton &&
    Boolean(statusIndicator ?? badge)
  );
}

/**
 * Desktop search row action area (action button or status/badge)
 */
interface DesktopSearchActionProps {
  readonly hasTabs: boolean;
  readonly hasTitle: boolean;
  readonly actionButton?: React.ReactNode;
  readonly statusIndicator?: StatusIndicatorConfig;
  readonly badge?: BadgeConfig;
}

export function DesktopSearchAction({
  hasTabs,
  hasTitle,
  actionButton,
  statusIndicator,
  badge,
}: DesktopSearchActionProps) {
  // Action button for pages WITHOUT tabs
  if (!hasTabs && actionButton) {
    return <div className="ml-auto">{actionButton}</div>;
  }

  // Status/badge for pages without tabs, title, or action button
  if (
    shouldShowInlineStatusBadge(
      hasTabs,
      hasTitle,
      !!actionButton,
      statusIndicator,
      badge,
    )
  ) {
    return (
      <InlineStatusBadge
        statusIndicator={statusIndicator}
        badge={badge}
        variant="desktop"
      />
    );
  }

  return null;
}
