"use client";

import { usePathname } from "next/navigation";
import { useSession } from "next-auth/react";
import {
  PortalOverflowBottomNav,
  type PortalOverflowNavItem,
} from "~/components/ui/portal-overflow-bottom-nav";
import { traegerPath } from "~/lib/traeger-url";
import {
  isTraegerNavActive,
  isTraegerSettingsPath,
} from "./traeger-nav-active";
import {
  TRAEGER_BACK_NAV,
  TRAEGER_HELP_NAV,
  TRAEGER_SETTINGS_NAV,
  traegerNavGroupsFor,
  traegerSettingsNavItemsFor,
  type TraegerNavItem,
} from "./traeger-nav-items";
import { useTraegerReturnHref } from "./traeger-return-path";

type BottomNavItem = Omit<TraegerNavItem, "right"> & {
  readonly help?: true;
  /** Das Ziel ist schon eine fertige Adresse und nie die aktuelle Seite. */
  readonly direct?: true;
};

const HELP_ITEM: BottomNavItem = {
  ...TRAEGER_HELP_NAV,
  help: true,
  direct: true,
};

/**
 * Die mobile Hauptnavigation des Träger-Portals (#5145): die geteilte
 * `PortalBottomNav` des Schul-Portals mit den Zielen, die die Rechte der
 * Person öffnen. Sie folgt der Seitenleiste (#5685): erst die Hauptnavigation
 * mit Hilfe und Einstellungen, in den Einstellungen deren Einträge mit dem
 * Rückweg zur App vorne.
 */
export function TraegerBottomNav() {
  const pathname = usePathname() ?? "";
  const { data: session } = useSession();
  const returnHref = useTraegerReturnHref();

  const settingsItems = traegerSettingsNavItemsFor(session ?? null);
  const settingsEntry = settingsItems[0];
  const items: readonly BottomNavItem[] = isTraegerSettingsPath(pathname)
    ? [
        { ...TRAEGER_BACK_NAV, href: returnHref, direct: true },
        ...settingsItems,
      ]
    : [
        // Die Leiste folgt der Seitenleiste (#5685); was nicht passt,
        // steht unter „Mehr“.
        ...traegerNavGroupsFor(session ?? null).flatMap((group) => group.items),
        HELP_ITEM,
        ...(settingsEntry
          ? [{ ...TRAEGER_SETTINGS_NAV, href: settingsEntry.href }]
          : []),
      ];
  const navItems: readonly PortalOverflowNavItem[] = items.map((item) => ({
    key: item.key,
    href: item.direct ? item.href : traegerPath(item.href),
    label: item.label,
    shortLabel: "shortLabel" in item ? item.shortLabel : undefined,
    concept: item.concept,
    active: item.direct !== true && isTraegerNavActive(item.href, pathname),
    newTab: item.help === true,
    // Die Hilfe ist kein Arbeitsbereich: eine Linie trennt sie ab, wie unten
    // in der Seitenleiste.
    separated: item.help === true,
  }));

  return (
    <PortalOverflowBottomNav
      items={navItems}
      navItemAttribute="data-traeger-nav-item"
      activeInMore
    />
  );
}
