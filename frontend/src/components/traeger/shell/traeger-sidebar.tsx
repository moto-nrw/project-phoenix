"use client";

import { usePathname } from "next/navigation";
import { useSession } from "next-auth/react";
import {
  PortalSidebar,
  PortalSidebarLink,
} from "~/components/ui/portal-sidebar";
import { traegerPath } from "~/lib/traeger-url";
import { isTraegerNavActive } from "./traeger-nav-active";
import { TRAEGER_HELP_NAV, traegerNavItemsFor } from "./traeger-nav-items";

/**
 * Die dauerhafte Seitennavigation des Träger-Portals ab 1024 px (#5145):
 * die geteilte `PortalSidebar` des Schul-Portals mit den Zielen, die die
 * Rechte der Person öffnen. Abmelden sitzt im Profil-Menü der geteilten
 * Kopfzeile, die auf jeder Breite sichtbar ist.
 */
export function TraegerSidebar() {
  const pathname = usePathname() ?? "";
  const { data: session } = useSession();

  return (
    <PortalSidebar
      secondary={
        <PortalSidebarLink
          href={TRAEGER_HELP_NAV.href}
          label={TRAEGER_HELP_NAV.label}
          concept={TRAEGER_HELP_NAV.concept}
          active={false}
          newTab
          data-traeger-nav-item={TRAEGER_HELP_NAV.key}
        />
      }
    >
      {traegerNavItemsFor(session ?? null).map((item) => (
        <PortalSidebarLink
          key={item.key}
          href={traegerPath(item.href)}
          label={item.label}
          concept={item.concept}
          active={isTraegerNavActive(item.href, pathname)}
          data-traeger-nav-item={item.key}
        />
      ))}
    </PortalSidebar>
  );
}
