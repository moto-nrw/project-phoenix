"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { MotoNavIcon } from "~/components/ui/moto-nav-icon";
import { schoolPath } from "~/lib/school-url";
import { isSchoolNavActive } from "./school-nav-active";
import {
  SCHOOL_PRIMARY_NAV,
  SCHOOL_SECONDARY_NAV,
} from "./school-nav-items";

const ITEM =
  "relative z-10 flex min-h-[44px] items-center justify-center gap-2.5 rounded-full px-4 py-2.5 transition-colors duration-200";
const ITEM_ACTIVE = "bg-gray-100 text-gray-900";
const ITEM_IDLE = "text-gray-500 hover:text-gray-700";

const SCHOOL_MOBILE_NAV = [...SCHOOL_PRIMARY_NAV, ...SCHOOL_SECONDARY_NAV];

/**
 * Die mobile Hauptnavigation des Schul-Portals.
 *
 * Beschriftet ist nur das aktive Ziel, die übrigen tragen ihr Icon und ihren
 * Namen als aria-label — dasselbe Muster wie die Eltern-App. Solange es zwei
 * Ziele waren, standen alle Beschriftungen dauerhaft da; mit dem dritten
 * (#2527) passte das auf einem 390 px breiten Gerät nicht mehr in die Pille:
 * die Zeile brauchte 447 px bei 324 px Platz, "Hilfe" lag komplett ausserhalb
 * des Bildschirms und "Meine Aufsichten" ragte über die Pille hinaus.
 *
 * Kein "Mehr"-Menü und kein gleitender Indikator — dafür sind es zu wenige
 * Ziele. Ab 1024 px übernimmt die Seitennavigation; CSS blendet diese Leiste
 * dort schon beim ersten Paint aus.
 */
export function SchoolBottomNav() {
  const pathname = usePathname();

  return (
    <nav
      aria-label="Hauptnavigation"
      className="fixed inset-x-0 bottom-0 z-30 lg:hidden"
    >
      {/* Die Leiste schwebt, ihr Rand ist also durchsichtig — ohne diesen
          Verlauf laeuft der Inhalt daneben und darunter sichtbar weiter und
          endet mitten in einer Zeile. Der Verlauf geht auf die Seitenfarbe
          (gray-50) und nicht auf Weiss, damit er auf dem gepunkteten
          Hintergrund keine Kante zieht. */}
      <div
        aria-hidden="true"
        className="pointer-events-none absolute inset-x-0 bottom-0 h-28 bg-gradient-to-t from-gray-50 via-gray-50/90 to-transparent"
      />
      <div className="relative px-4 pb-4">
        <div className="rounded-full border border-gray-200/50 bg-white/95 px-3 py-2 shadow-[0_-2px_20px_rgba(0,0,0,0.08)] backdrop-blur-md">
          <ul className="flex items-center justify-around gap-1">
            {SCHOOL_MOBILE_NAV.map((item) => {
              const active = isSchoolNavActive(item.href, pathname);
              return (
                <li key={item.key}>
                  <Link
                    href={item.portalPath ? schoolPath(item.href) : item.href}
                    data-school-nav-item={item.key}
                    data-active={active ? "true" : "false"}
                    aria-current={active ? "page" : undefined}
                    {...(item.newTab
                      ? { target: "_blank", rel: "noopener noreferrer" }
                      : {})}
                    aria-label={item.label}
                    className={`${ITEM} ${active ? ITEM_ACTIVE : ITEM_IDLE}`}
                  >
                    <MotoNavIcon
                      concept={item.concept}
                      active={active}
                      className="h-5 w-5 shrink-0"
                    />
                    {active ? (
                      <span className="text-sm font-semibold whitespace-nowrap">
                        {item.label}
                      </span>
                    ) : null}
                  </Link>
                </li>
              );
            })}
          </ul>
        </div>
      </div>
      <div className="h-safe-area-inset-bottom bg-transparent" />
    </nav>
  );
}
