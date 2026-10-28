"use client";

import { usePathname } from "next/navigation";
import { useSession } from "next-auth/react";
import {
  PortalSidebar,
  PortalSidebarLink,
  PortalSidebarSection,
} from "~/components/ui/portal-sidebar";
import { traegerPath } from "~/lib/traeger-url";
import { isTraegerNavActive } from "./traeger-nav-active";
import {
  TRAEGER_HELP_NAV,
  TRAEGER_SETTINGS_NAV,
  traegerNavItemsFor,
  traegerSettingsNavItemsFor,
} from "./traeger-nav-items";

/**
 * Die dauerhafte Seitennavigation des Träger-Portals ab 1024 px (#5145):
 * die geteilte `PortalSidebar` des Schul-Portals mit den Zielen, die die
 * Rechte der Person öffnen. Unten stehen wie im Schul-Portal die Hilfe und
 * die einklappbaren Einstellungen (#5242). Abmelden sitzt im Profil-Menü der
 * geteilten Kopfzeile, die auf jeder Breite sichtbar ist.
 */
export function TraegerSidebar() {
  const pathname = usePathname() ?? "";
  const { data: session } = useSession();

  return (
    <PortalSidebar
      secondaryLabel="Hilfe und Einstellungen"
      secondary={
        <>
          <PortalSidebarLink
            href={TRAEGER_HELP_NAV.href}
            label={TRAEGER_HELP_NAV.label}
            concept={TRAEGER_HELP_NAV.concept}
            active={false}
            newTab
            data-traeger-nav-item={TRAEGER_HELP_NAV.key}
          />
          <PortalSidebarSection
            label={TRAEGER_SETTINGS_NAV.label}
            concept={TRAEGER_SETTINGS_NAV.concept}
            data-portal-nav-section={TRAEGER_SETTINGS_NAV.key}
            items={traegerSettingsNavItemsFor(session ?? null).map((item) => ({
              key: item.key,
              href: traegerPath(item.href),
              label: item.label,
              active: isTraegerNavActive(item.href, pathname),
            }))}
          />
        </>
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
