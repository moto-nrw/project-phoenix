"use client";

import { usePathname } from "next/navigation";
import { useSession } from "next-auth/react";
import { MotoNavIcon } from "~/components/ui/moto-nav-icon";
import { NavLink } from "~/components/ui/nav-link";
import { traegerPath } from "~/lib/traeger-url";
import { isTraegerNavActive } from "./traeger-nav-active";
import {
  TRAEGER_HELP_NAV,
  traegerNavItemsFor,
  type TraegerNavItem,
} from "./traeger-nav-items";

const ROW =
  "flex items-center rounded-lg px-3 py-2.5 text-sm transition-colors focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none lg:px-4 lg:py-3 lg:text-base xl:px-3 xl:py-2.5 xl:text-sm";
const ROW_ACTIVE = "bg-gray-100 font-semibold text-gray-900";
const ROW_IDLE =
  "font-medium text-gray-600 hover:bg-gray-50 hover:text-gray-900";
const ICON =
  "mr-3 h-5 w-5 shrink-0 lg:mr-3.5 lg:h-[22px] lg:w-[22px] xl:mr-3 xl:h-5 xl:w-5";

/**
 * Die dauerhafte Seitennavigation des Träger-Portals ab 1024 px (#5145),
 * gebaut wie die des Schul-Portals.
 *
 * Unter 1024 px übernimmt die mobile Leiste; CSS blendet die Spalte dort
 * bereits beim ersten Paint aus. Abmelden sitzt im Profil-Menü der geteilten
 * Kopfzeile, die auf jeder Breite sichtbar ist.
 */
export function TraegerSidebar() {
  const pathname = usePathname() ?? "";
  const { data: session } = useSession();
  const items = traegerNavItemsFor(session ?? null);

  const renderItem = (item: Omit<TraegerNavItem, "right">, help: boolean) => {
    const active = !help && isTraegerNavActive(item.href, pathname);
    return (
      <li key={item.key}>
        <NavLink
          href={help ? item.href : traegerPath(item.href)}
          data-traeger-nav-item={item.key}
          data-active={active ? "true" : "false"}
          aria-current={active ? "page" : undefined}
          {...(help ? { target: "_blank", rel: "noopener noreferrer" } : {})}
          className={`${ROW} ${active ? ROW_ACTIVE : ROW_IDLE}`}
        >
          <MotoNavIcon
            concept={item.concept}
            active={active}
            className={`${ICON} ${active ? "" : "text-gray-400"}`}
          />
          <span className="flex-1">{item.label}</span>
        </NavLink>
      </li>
    );
  };

  return (
    <aside className="hidden min-h-screen w-64 shrink-0 border-r border-gray-200/70 bg-white/95 lg:block">
      <div className="sticky top-[73px] flex h-[calc(100vh-73px)] flex-col">
        <nav
          aria-label="Hauptnavigation"
          className="flex-1 overflow-y-auto p-3 lg:p-4 xl:p-3"
        >
          <ul>{items.map((item) => renderItem(item, false))}</ul>
        </nav>

        <nav
          aria-label="Hilfe"
          className="border-t border-gray-200 p-3 lg:p-4 xl:p-3"
        >
          <ul>{renderItem(TRAEGER_HELP_NAV, true)}</ul>
        </nav>
      </div>
    </aside>
  );
}
