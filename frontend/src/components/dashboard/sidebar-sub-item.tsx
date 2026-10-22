"use client";

import { NavLink } from "~/components/ui/nav-link";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import {
  UnreadBadge,
  type UnreadBadgeTone,
} from "~/components/messaging/unread-badge";
import { SIDEBAR_SUB_ITEM_CLASSES } from "~/components/dashboard/sidebar-geometry";

interface SidebarSubItemProps {
  readonly href: string;
  readonly label: string;
  readonly isActive: boolean;
  readonly count?: number | string;
  // Die Person führt hier selbst Aufsicht, z. B. in einem offenen Raum
  // (#4349). Das Auge ist dasselbe Symbol wie bei „Aktuelle Aufsicht“; als
  // Text wäre der Hinweis zu breit und kürzte den Raumnamen.
  readonly ownSupervision?: boolean;
  // Domain-colored pill for unread messages or pending requests. Distinct from
  // `count`, which is the muted gray attendance count.
  readonly badgeCount?: number;
  readonly badgeTone?: UnreadBadgeTone;
  readonly badgeNoun?: string;
  // Ziel der geführten Tour der ersten Schritte (#2832).
  readonly tourId?: string;
}

export function SidebarSubItem({
  href,
  label,
  isActive,
  count,
  ownSupervision = false,
  badgeCount = 0,
  badgeTone,
  badgeNoun,
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
      {ownSupervision && (
        <span title="Eigene Aufsicht" className="ml-2 flex shrink-0">
          <MotoConceptIcon
            concept="supervision"
            size={16}
            label="Eigene Aufsicht"
          />
        </span>
      )}
      {badgeCount > 0 ? (
        <UnreadBadge
          count={badgeCount}
          tone={badgeTone}
          noun={badgeNoun}
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
