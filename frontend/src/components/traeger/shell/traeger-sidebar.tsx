"use client";

import { usePathname } from "next/navigation";
import { useSession } from "next-auth/react";
import {
  PortalSidebar,
  PortalSidebarGroup,
  PortalSidebarLink,
} from "~/components/ui/portal-sidebar";
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
} from "./traeger-nav-items";
import { useTraegerReturnHref } from "./traeger-return-path";

/**
 * Die dauerhafte Seitennavigation des Träger-Portals ab 1024 px (#5145):
 * die geteilte `PortalSidebar` mit den Zielen, die die Rechte der Person
 * öffnen. Dieselbe Leiste zeigt je nach Adresse die Hauptnavigation oder die
 * Einstellungen mit dem Rückweg zur App (#5685). Abmelden sitzt im Profil-Menü
 * der geteilten Kopfzeile, die auf jeder Breite sichtbar ist.
 */
export function TraegerSidebar() {
  const pathname = usePathname() ?? "";
  const { data: session } = useSession();
  const settingsItems = traegerSettingsNavItemsFor(session ?? null);
  const returnHref = useTraegerReturnHref();

  if (isTraegerSettingsPath(pathname)) {
    return (
      <PortalSidebar navLabel={TRAEGER_SETTINGS_NAV.label}>
        <PortalSidebarLink
          href={returnHref}
          label={TRAEGER_BACK_NAV.label}
          concept={TRAEGER_BACK_NAV.concept}
          active={false}
          data-traeger-nav-item={TRAEGER_BACK_NAV.key}
        />
        <PortalSidebarGroup label={TRAEGER_SETTINGS_NAV.label}>
          {settingsItems.map((item) => (
            <PortalSidebarLink
              key={item.key}
              href={traegerPath(item.href)}
              label={item.label}
              concept={item.concept}
              active={isTraegerNavActive(item.href, pathname)}
              data-traeger-nav-item={item.key}
            />
          ))}
        </PortalSidebarGroup>
      </PortalSidebar>
    );
  }

  const settingsEntry = settingsItems[0];
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
          {settingsEntry ? (
            <PortalSidebarLink
              href={traegerPath(settingsEntry.href)}
              label={TRAEGER_SETTINGS_NAV.label}
              concept={TRAEGER_SETTINGS_NAV.concept}
              active={false}
              data-traeger-nav-item={TRAEGER_SETTINGS_NAV.key}
            />
          ) : null}
        </>
      }
    >
      {traegerNavGroupsFor(session ?? null).map((group) => (
        <PortalSidebarGroup key={group.key} label={group.label}>
          {group.items.map((item) => (
            <PortalSidebarLink
              key={item.key}
              href={traegerPath(item.href)}
              label={item.label}
              concept={item.concept}
              active={isTraegerNavActive(item.href, pathname)}
              data-traeger-nav-item={item.key}
            />
          ))}
        </PortalSidebarGroup>
      ))}
    </PortalSidebar>
  );
}
