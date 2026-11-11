"use client";

import { NavLink } from "~/components/ui/nav-link";
import { NotificationBadge } from "~/components/ui/notification-badge";
import { SIDEBAR_SUB_ITEM_CLASSES } from "~/components/ui/sidebar-geometry";
import { ModuleLockMark } from "~/components/ui/module-lock-mark";
import type { SidebarRowBadge } from "~/components/ui/sidebar-row";

interface SidebarSubItemProps {
  readonly href: string;
  readonly label: string;
  readonly isActive: boolean;
  readonly count?: number | string;
  // Domain-colored pill for unread messages or pending requests. Distinct from
  // `count`, which is the muted gray attendance count.
  readonly badge?: SidebarRowBadge | null;
  // Ziel der geführten Tour der ersten Schritte (#2832).
  readonly tourId?: string;
  // Modul nicht gebucht (#4894): Schloss statt Zähler.
  readonly locked?: boolean;
}

export function SidebarSubItem({
  href,
  label,
  isActive,
  count,
  badge,
  tourId,
  locked = false,
}: SidebarSubItemProps) {
  return (
    <NavLink
      href={href}
      data-setup-tour={tourId}
      className={`${SIDEBAR_SUB_ITEM_CLASSES} ${
        isActive
          ? "bg-gray-100 font-semibold text-gray-900"
          : "font-medium text-gray-500 hover:bg-gray-50 hover:text-gray-700"
      }`}
    >
      <span className="truncate">{label}</span>
      {locked ? (
        <ModuleLockMark className="ml-2" />
      ) : badge != null && badge.count > 0 ? (
        <NotificationBadge
          count={badge.count}
          tone={badge.tone}
          ariaLabel={badge.ariaLabel}
          className="ml-2"
        />
      ) : (
        count !== undefined && (
          <span className="ml-2 shrink-0 text-xs text-gray-400">{count}</span>
        )
      )}
    </NavLink>
  );
}
