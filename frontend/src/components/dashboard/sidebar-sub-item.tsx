"use client";

import Link from "next/link";
import { UnreadBadge } from "~/components/messaging/unread-badge";

interface SidebarSubItemProps {
  readonly href: string;
  readonly label: string;
  readonly isActive: boolean;
  readonly count?: number | string;
  // Red unread pill (e.g. unread messages / pending requests). Renders nothing
  // when 0. Distinct from `count`, which is the muted gray attendance count.
  readonly badgeCount?: number;
}

export function SidebarSubItem({
  href,
  label,
  isActive,
  count,
  badgeCount = 0,
}: SidebarSubItemProps) {
  return (
    <Link
      href={href}
      className={`flex items-center justify-between rounded-md py-1.5 pr-3 pl-10 text-sm transition-colors ${
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
    </Link>
  );
}
