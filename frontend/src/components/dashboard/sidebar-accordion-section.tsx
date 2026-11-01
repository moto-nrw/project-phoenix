"use client";

import { useId, type ReactNode } from "react";
import {
  UnreadBadge,
  type UnreadBadgeTone,
} from "~/components/messaging/unread-badge";
import { Button } from "~/components/ui/button";
import { Skeleton } from "~/components/ui/skeleton";
import { MotoNavIcon } from "~/components/ui/moto-nav-icon";
import { NavSceneFallback } from "~/components/ui/nav-scene-icon/nav-scene-icon";
import {
  PORTAL_ICON_CLASSES,
  portalSidebarRowClasses,
  SIDEBAR_ICON_CLASSES,
  sidebarLabelClasses,
  sidebarRowClasses,
} from "~/components/dashboard/sidebar-geometry";
import type { MotoConcept } from "~/lib/moto-concepts";

interface SidebarAccordionSectionProps {
  readonly icon?: string;
  /** Keep dashboard defaults; portals share their responsive link geometry. */
  readonly variant?: "dashboard" | "portal";
  readonly "data-portal-nav-section"?: string;
  readonly concept?: MotoConcept;
  readonly label: string;
  readonly activeColor?: string;
  readonly isExpanded: boolean;
  readonly onToggle: () => void;
  readonly isActive: boolean;
  readonly isIconActive?: boolean;
  readonly isLoading?: boolean;
  readonly emptyText?: string;
  readonly children?: ReactNode;
  readonly hasChildren: boolean;
  // Aggregate section badge for unread messages and pending requests. The
  // count stays visible while the section is collapsed.
  readonly badgeCount?: number;
  readonly badgeTone?: UnreadBadgeTone;
  // Was gezählt wird, für Vorleseprogramme (Standard: Nachrichten).
  readonly badgeNoun?: string;
  // Eingeklappte Leiste (#2825/#2923): dieselbe Zeile, nur ohne Text. Der
  // Bereichsinhalt passt nicht in den Streifen, deshalb bleibt er dort
  // geschlossen; der Klick klappt die Leiste auf (die Elternkomponente
  // entscheidet das in onToggle).
  readonly collapsed?: boolean;
  // Während der Breitenänderung bleiben Text und Chevron stehen und blenden
  // aus, statt zu blitzen. Siehe useSidebarCollapseTransition.
  readonly labelsMounted?: boolean;
  readonly labelsVisible?: boolean;
  // Ziel der geführten Tour der ersten Schritte (#2832).
  readonly tourId?: string;
}

export function SidebarAccordionSection({
  icon,
  variant = "dashboard",
  "data-portal-nav-section": dataSection,
  concept,
  label,
  activeColor,
  isExpanded,
  onToggle,
  isActive,
  isIconActive,
  isLoading = false,
  emptyText = "Keine Einträge",
  children,
  hasChildren,
  badgeCount = 0,
  badgeTone,
  badgeNoun,
  collapsed = false,
  labelsMounted = true,
  labelsVisible = true,
  tourId,
}: SidebarAccordionSectionProps) {
  const bodyId = useId();
  const iconColorClass =
    (isIconActive ?? isActive) && activeColor ? activeColor : "";
  const conceptDefinition = concept ?? null;
  const showActiveIcon = isIconActive ?? isActive;
  // Im Streifen ist der Bereichsinhalt nie offen.
  const bodyExpanded = isExpanded && !collapsed;
  // Ausgeklappt bleibt die Kopfzeile unmarkiert, sobald ein Unterpunkt
  // ausgewählt ist — dort trägt der Unterpunkt die Markierung. Im Streifen
  // sind die Unterpunkte nicht sichtbar; ohne diese Zeile stünde der Bereich
  // dort ganz ohne Markierung da, obwohl man in ihm ist — etwa auf
  // /database/rooms oder /ogs-groups?group=… (#2923).
  const rowActive = collapsed ? showActiveIcon : isActive;

  return (
    <div>
      {/* Kopfzeile des Bereichs — ein Kasten mit Icon, Bezeichnung, Chevron.
          Kit-Button (variant="ghost", size="md") mit dem Zeilenraster der
          Seitenleiste: Grundverhalten, Hover und Fokusring kommen aus dem Kit,
          Höhe, Abstände und Aktiv-Zustand aus sidebar-geometry — derselben
          Quelle wie bei den Navigationslinks daneben. */}
      <Button
        type="button"
        variant="ghost"
        size="md"
        onClick={onToggle}
        data-setup-tour={tourId}
        onKeyDown={(e) => {
          if (e.key === "Enter" || e.key === " ") {
            e.preventDefault();
            onToggle();
          }
        }}
        className={`${variant === "portal" ? `${portalSidebarRowClasses(rowActive)} w-full justify-start text-left` : sidebarRowClasses({ isActive: rowActive })} cursor-pointer`}
        data-portal-nav-section={dataSection}
        data-active={rowActive ? "true" : "false"}
        aria-controls={bodyId}
        aria-expanded={bodyExpanded}
        {...(collapsed ? { title: label, "aria-label": label } : {})}
      >
        {variant === "portal" && concept ? (
          <MotoNavIcon
            scene
            concept={concept}
            active={showActiveIcon}
            className={`${PORTAL_ICON_CLASSES} ${showActiveIcon ? "" : "text-gray-400"}`}
          />
        ) : conceptDefinition ? (
          <MotoNavIcon
            scene
            concept={conceptDefinition}
            active={showActiveIcon}
            className={SIDEBAR_ICON_CLASSES}
          />
        ) : (
          <NavSceneFallback>
            <svg
              className={`${SIDEBAR_ICON_CLASSES} ${iconColorClass}`}
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              aria-hidden="true"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d={icon}
              />
            </svg>
          </NavSceneFallback>
        )}
        {labelsMounted && (
          <>
            <span
              className={
                variant === "portal"
                  ? "flex-1"
                  : sidebarLabelClasses(labelsVisible)
              }
            >
              {label}
            </span>
            {/* Sammelzähler auf der geschlossenen Kopfzeile; sobald der
                Bereich offen ist, übernehmen die Zähler der Unterpunkte.
                Für die Dauer der Blende steht er neben dem Zähler im
                Streifen; der ausblendende ist `aria-hidden`, sonst läse ein
                Vorleseprogramm die Zahl zweimal (#2923). */}
            {!bodyExpanded && (
              <span
                aria-hidden={!labelsVisible}
                className={`ml-2 shrink-0 motion-safe:transition-opacity motion-safe:duration-150 ${labelsVisible ? "opacity-100" : "opacity-0"}`}
              >
                <UnreadBadge
                  count={badgeCount}
                  tone={badgeTone}
                  noun={badgeNoun}
                />
              </span>
            )}
            <svg
              className={`ml-2 h-4 w-4 shrink-0 text-gray-400 motion-safe:transition-[transform,opacity] motion-safe:duration-200 ${bodyExpanded ? "rotate-180" : ""} ${labelsVisible ? "opacity-100" : "opacity-0"}`}
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              strokeWidth={2}
              aria-hidden="true"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                d="M19 9l-7 7-7-7"
              />
            </svg>
          </>
        )}
        {/* Im Streifen wandert der Zähler als Punkt auf die Ecke des Icons —
            eingeblendet, während die Bezeichnung ausblendet, damit sich
            nichts überlagert. */}
        {collapsed && badgeCount > 0 && (
          <span
            aria-hidden={labelsVisible}
            className={`absolute top-1 right-1 motion-safe:transition-opacity motion-safe:duration-150 ${labelsVisible ? "opacity-0" : "opacity-100"}`}
          >
            <UnreadBadge count={badgeCount} tone={badgeTone} noun={badgeNoun} />
          </span>
        )}
      </Button>

      {/* Bereichsinhalt — Höhenwechsel über CSS-Grid */}
      <div
        className={`grid motion-safe:transition-[grid-template-rows] motion-safe:duration-200 motion-safe:ease-in-out ${
          bodyExpanded ? "grid-rows-[1fr]" : "grid-rows-[0fr]"
        }`}
      >
        {/* inert: geschlossene Bereiche bleiben im Baum, dürfen aber keinen
            Tastaturfokus fangen — sonst springt der Fokus in unsichtbare
            Unterpunkte (#2923). */}
        <div id={bodyId} className="overflow-hidden" inert={!bodyExpanded}>
          <div className="py-0.5">
            {isLoading && (
              /* Skeleton shimmer */
              <div className="space-y-1 pr-3 pl-11">
                <Skeleton className="h-6 w-3/4 rounded" />
                <Skeleton className="h-6 w-2/3 rounded" />
                <Skeleton className="h-6 w-1/2 rounded" />
              </div>
            )}
            {!isLoading && hasChildren && children}
            {!isLoading && !hasChildren && (
              /* Empty state */
              <div className="py-2 pr-3 pl-11 text-xs text-gray-400">
                {emptyText}
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
