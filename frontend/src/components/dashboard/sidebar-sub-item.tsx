"use client";

import { NavLink } from "~/components/ui/nav-link";
import { UnreadBadge } from "~/components/messaging/unread-badge";
import { SIDEBAR_SUB_ITEM_CLASSES } from "~/components/dashboard/sidebar-geometry";

interface SidebarSubItemProps {
  readonly href: string;
  readonly label: string;
  readonly isActive: boolean;
  readonly count?: number | string;
  // Domain-colored pill for unread messages or pending requests. Distinct from
  // `count`, which is the muted gray attendance count.
  readonly badgeCount?: number;
  // Ziel der geführten Tour der ersten Schritte (#2832).
  readonly tourId?: string;
}

export function SidebarSubItem({
  href,
  label,
  isActive,
  count,
  badgeCount = 0,
  tourId,
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
      {badgeCount > 0 ? (
        <UnreadBadge count={badgeCount} className="ml-2" />
      ) : (
        count !== undefined && (
          <span className="ml-2 shrink-0 text-xs text-gray-400">{count}</span>
        )
      )}
    </NavLink>
  );
}
