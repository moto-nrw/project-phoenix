"use client";

import { useId, type ReactNode } from "react";
import { Button } from "~/components/ui/button";
import { Skeleton } from "~/components/ui/skeleton";
import { MotoNavIcon } from "~/components/ui/moto-nav-icon";
import {
  SIDEBAR_ICON_CLASSES,
  sidebarRowClasses,
} from "~/components/ui/sidebar-geometry";
import {
  SidebarRowContent,
  type SidebarRowBadge,
} from "~/components/ui/sidebar-row";
import type { MotoConcept } from "~/lib/moto-concepts";

interface SidebarAccordionSectionProps {
  readonly concept: MotoConcept;
  readonly label: string;
  readonly isExpanded: boolean;
  readonly onToggle: () => void;
  readonly isActive: boolean;
  readonly isIconActive?: boolean;
  readonly isLoading?: boolean;
  readonly emptyText?: string;
  readonly children?: ReactNode;
  readonly hasChildren: boolean;
  /**
   * Sammelzähler des Bereichs (ungelesene Anmeldungen). Sichtbar, solange der
   * Bereich zu ist; offen tragen die Unterpunkte ihre Zähler selbst.
   */
  readonly badge?: SidebarRowBadge | null;
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
  concept,
  label,
  isExpanded,
  onToggle,
  isActive,
  isIconActive,
  isLoading = false,
  emptyText = "Keine Einträge",
  children,
  hasChildren,
  badge,
  collapsed = false,
  labelsMounted = true,
  labelsVisible = true,
  tourId,
}: SidebarAccordionSectionProps) {
  const bodyId = useId();
  const showActiveIcon = isIconActive ?? isActive;
  // Im Streifen ist der Bereichsinhalt nie offen.
  const bodyExpanded = isExpanded && !collapsed;
  // Ausgeklappt bleibt die Kopfzeile unmarkiert, sobald ein Unterpunkt
  // ausgewählt ist — dort trägt der Unterpunkt die Markierung. Im Streifen
  // sind die Unterpunkte nicht sichtbar; ohne diese Zeile stünde der Bereich
  // dort ganz ohne Markierung da, obwohl man in ihm ist — etwa auf
  // /database/rooms oder /ogs-groups?group=… (#2923).
  const rowActive = collapsed ? showActiveIcon : isActive;
  // Sammelzähler auf der geschlossenen Kopfzeile; sobald der Bereich offen
  // ist, übernehmen die Zähler der Unterpunkte.
  const shownBadge =
    badge != null && badge.count > 0 && !bodyExpanded ? badge : null;
  const collapsedLabel = shownBadge
    ? `${label}, ${shownBadge.ariaLabel}`
    : label;

  return (
    <div>
      {/* Kopfzeile des Bereichs — ein Kasten mit Icon, Bezeichnung, Chevron.
          Kit-Button (variant="ghost", size="md") mit dem Zeilenraster der
          Seitenleiste: Grundverhalten, Tastatur, Hover und Fokusring kommen
          aus dem Kit, Höhe, Abstände und Aktiv-Zustand aus sidebar-geometry —
          derselben Quelle wie bei den Navigationslinks daneben. */}
      <Button
        type="button"
        variant="ghost"
        size="md"
        onClick={onToggle}
        data-setup-tour={tourId}
        className={`${sidebarRowClasses({ isActive: rowActive })} cursor-pointer`}
        data-active={rowActive ? "true" : "false"}
        aria-controls={bodyId}
        aria-expanded={bodyExpanded}
        {...(collapsed ? { title: label, "aria-label": collapsedLabel } : {})}
      >
        <SidebarRowContent
          icon={
            <MotoNavIcon
              scene
              concept={concept}
              active={showActiveIcon}
              className={SIDEBAR_ICON_CLASSES}
            />
          }
          label={label}
          badge={shownBadge}
          chevronOpen={bodyExpanded}
          collapsed={collapsed}
          labelsMounted={labelsMounted}
          labelsVisible={labelsVisible}
        />
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
                <Skeleton className="h-6 w-3/4" />
                <Skeleton className="h-6 w-2/3" />
                <Skeleton className="h-6 w-1/2" />
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
