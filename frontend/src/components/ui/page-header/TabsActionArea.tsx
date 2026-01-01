// TabsActionArea component - extracted to reduce complexity in PageHeaderWithSearch
"use client";

import type React from "react";
import { StatusIndicator } from "./StatusIndicator";
import { BadgeDisplay, BadgeDisplayCompact } from "./BadgeDisplay";

interface StatusIndicatorConfig {
  color: "green" | "yellow" | "red" | "gray";
  tooltip?: string;
}

interface BadgeConfig {
  count: number | string;
  label?: string;
  icon?: React.ReactNode;
}

interface TabsActionAreaProps {
  readonly hasTitle: boolean;
  readonly actionButton?: React.ReactNode;
  readonly statusIndicator?: StatusIndicatorConfig;
  readonly badge?: BadgeConfig;
  readonly variant: "desktop" | "mobile";
}

/**
 * Action area alongside tabs (badge/status or action button)
 */
export function TabsActionArea({
  hasTitle,
  actionButton,
  statusIndicator,
  badge,
  variant,
}: TabsActionAreaProps) {
  // If there's a title, the action area is handled by PageHeader
  if (hasTitle) return null;

  const isDesktop = variant === "desktop";

  // Render action button if present
  if (actionButton) {
    return <>{isDesktop ? actionButton : actionButton}</>;
  }

  // Render status/badge indicators
  return (
    <>
      {statusIndicator && (
        <StatusIndicator
          color={statusIndicator.color}
          tooltip={statusIndicator.tooltip}
        />
      )}
      {badge &&
        (isDesktop ? (
          <BadgeDisplay
            count={badge.count}
            label={badge.label}
            icon={badge.icon}
            showLabel={true}
          />
        ) : (
          <BadgeDisplayCompact count={badge.count} icon={badge.icon} />
        ))}
    </>
  );
}

/**
 * Wrapper for desktop tabs action area
 */
export function DesktopTabsActionArea(
  props: Omit<TabsActionAreaProps, "variant">,
) {
  const { hasTitle, actionButton, statusIndicator, badge } = props;

  // Don't render anything if hasTitle or no content
  if (hasTitle) return null;
  if (!actionButton && !statusIndicator && !badge) return null;

  return (
    <div className="hidden flex-shrink-0 items-center gap-2 pb-3 md:flex md:gap-3">
      <TabsActionArea {...props} variant="desktop" />
    </div>
  );
}

/**
 * Wrapper for mobile tabs action area
 */
export function MobileTabsActionArea(
  props: Omit<TabsActionAreaProps, "variant">,
) {
  const { hasTitle, actionButton, statusIndicator, badge } = props;

  // Don't render anything if hasTitle or no content
  if (hasTitle) return null;
  if (!actionButton && !statusIndicator && !badge) return null;

  return (
    <div className="mr-2 flex flex-shrink-0 items-center gap-2 pb-3 md:hidden md:gap-3">
      <TabsActionArea {...props} variant="mobile" />
    </div>
  );
}
