"use client";

import { useState } from "react";
import { SidebarAccordionSection } from "~/components/dashboard/sidebar-accordion-section";
import { PORTAL_SUB_ITEM_CLASSES } from "~/components/dashboard/sidebar-geometry";
import { NavLink } from "~/components/ui/nav-link";
import { usePathname } from "next/navigation";
import { useSession } from "next-auth/react";
import {
  PortalSidebar,
  PortalSidebarLink,
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
  const settingsItems = traegerSettingsNavItemsFor(session ?? null);
  const containsActive = settingsItems.some((item) =>
    isTraegerNavActive(item.href, pathname),
  );
  const [settingsOpen, setSettingsOpen] = useState(containsActive);
  // Open on entry into settings, but still allow manually collapsing them.
  const [wasActive, setWasActive] = useState(containsActive);
  if (containsActive !== wasActive) {
    setWasActive(containsActive);
    if (containsActive) setSettingsOpen(true);
  }

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
          {settingsItems.length > 0 && (
            <li>
              <SidebarAccordionSection
                variant="portal"
                label={TRAEGER_SETTINGS_NAV.label}
                concept={TRAEGER_SETTINGS_NAV.concept}
                data-portal-nav-section={TRAEGER_SETTINGS_NAV.key}
                isExpanded={settingsOpen}
                onToggle={() => setSettingsOpen((value) => !value)}
                isActive={containsActive && !settingsOpen}
                isIconActive={containsActive}
                hasChildren
              >
                <ul className="space-y-0.5">
                  {settingsItems.map((item) => {
                    const active = isTraegerNavActive(item.href, pathname);
                    return (
                      <li key={item.key}>
                        <NavLink
                          href={traegerPath(item.href)}
                          data-portal-nav-item={item.key}
                          data-active={active ? "true" : "false"}
                          aria-current={active ? "page" : undefined}
                          className={`${PORTAL_SUB_ITEM_CLASSES} ${active ? "bg-gray-100 font-semibold text-gray-900" : "font-medium text-gray-500 hover:bg-gray-50 hover:text-gray-700"}`}
                        >
                          {item.label}
                        </NavLink>
                      </li>
                    );
                  })}
                </ul>
              </SidebarAccordionSection>
            </li>
          )}
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
