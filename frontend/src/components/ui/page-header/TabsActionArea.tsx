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
function TabsActionArea({
  hasTitle,
  actionButton,
  statusIndicator,
  badge,
  variant,
}: TabsActionAreaProps) {
  const isDesktop = variant === "desktop";

  // When there's a title, badge/status are rendered by PageHeader (mobile).
  // But action buttons must still render here alongside tabs (desktop),
  // since PageHeader is md:hidden and doesn't show on desktop.
  return (
    <>
      {actionButton}
      {!hasTitle && statusIndicator && (
        <StatusIndicator
          color={statusIndicator.color}
          tooltip={statusIndicator.tooltip}
        />
      )}
      {!hasTitle &&
        badge &&
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
  props: Readonly<Omit<TabsActionAreaProps, "variant">>,
) {
  const { hasTitle, actionButton, statusIndicator, badge } = props;

  // When hasTitle, only render if there's an action button (badge/status handled by PageHeader)
  if (hasTitle && !actionButton) return null;
  if (!hasTitle && !actionButton && !statusIndicator && !badge) return null;

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
  props: Readonly<Omit<TabsActionAreaProps, "variant">>,
) {
  const { hasTitle, actionButton, statusIndicator, badge } = props;

  // When hasTitle, PageHeader already renders the mobile action button
  if (hasTitle) return null;
  if (!actionButton && !statusIndicator && !badge) return null;

  const stackActionAndBadge = Boolean(actionButton && badge);

  return (
    <div
      className={`mr-2 flex flex-shrink-0 items-center md:hidden ${
        stackActionAndBadge ? "flex-col-reverse gap-1" : "gap-2"
      }`}
    >
      <TabsActionArea {...props} variant="mobile" />
    </div>
  );
}
