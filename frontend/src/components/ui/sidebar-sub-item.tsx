"use client";

import NavigationLink from "~/components/ui/navigation-link";
import { ModuleLockMark } from "~/components/ui/module-lock-mark";
import { NotificationBadge } from "~/components/ui/notification-badge";
import { sidebarSubItemClasses } from "~/components/ui/sidebar-geometry";
import {
  SidebarRowContent,
  type SidebarRowBadge,
} from "~/components/ui/sidebar-row";

interface SidebarSubItemProps {
  readonly href: string;
  readonly label: string;
  readonly isActive: boolean;
  // Die graue Anwesenheit einer Gruppe („12/20“), kein Zähler.
  readonly count?: number | string;
  // Zähler für ungelesene Nachrichten oder offene Anfragen; er ersetzt die
  // graue Anwesenheit.
  readonly badge?: SidebarRowBadge | null;
  // Ziel der geführten Tour der ersten Schritte (#2832).
  readonly tourId?: string;
  // Modul nicht gebucht (#4894): Schloss statt Zähler.
  readonly locked?: boolean;
}

/** Ein Unterpunkt unter einem Akkordeon-Bereich: Text ohne Icon. */
export function SidebarSubItem({
  href,
  label,
  isActive,
  count,
  badge,
  tourId,
  locked = false,
}: SidebarSubItemProps) {
  // Ein Unterpunkt steht nie im Streifen und blendet nicht: sein Zähler ist
  // Beiwerk der Zeile, kein wandernder Zähler mit Punkt.
  const trailing = locked ? (
    <ModuleLockMark />
  ) : badge != null && badge.count > 0 ? (
    <NotificationBadge
      count={badge.count}
      tone={badge.tone}
      ariaLabel={badge.ariaLabel}
    />
  ) : count === undefined ? null : (
    <span className="text-xs text-gray-400">{count}</span>
  );
  return (
    <NavigationLink
      href={href}
      data-setup-tour={tourId}
      className={sidebarSubItemClasses(isActive)}
    >
      <SidebarRowContent label={label} trailing={trailing} />
    </NavigationLink>
  );
}
