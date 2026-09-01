"use client";

import type { ReactNode } from "react";
import { UnreadBadge } from "~/components/messaging/unread-badge";
import { Button } from "~/components/ui/button";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import {
  SIDEBAR_ICON_CLASSES,
  sidebarLabelClasses,
  sidebarRowClasses,
} from "~/components/dashboard/sidebar-geometry";
import { MOTO_CONCEPTS, type MotoConceptKey } from "~/lib/moto-concepts";

interface SidebarAccordionSectionProps {
  readonly icon: string;
  readonly concept?: MotoConceptKey;
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
  // Eingeklappte Leiste (#2825/#2923): dieselbe Zeile, nur ohne Text. Der
  // Bereichsinhalt passt nicht in den Streifen, deshalb bleibt er dort
  // geschlossen; der Klick klappt die Leiste auf (die Elternkomponente
  // entscheidet das in onToggle).
  readonly collapsed?: boolean;
  // Während der Breitenänderung bleiben Text und Chevron stehen und blenden
  // aus, statt zu blitzen. Siehe useSidebarCollapseTransition.
  readonly labelsMounted?: boolean;
  readonly labelsVisible?: boolean;
}

export function SidebarAccordionSection({
  icon,
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
  collapsed = false,
  labelsMounted = true,
  labelsVisible = true,
}: SidebarAccordionSectionProps) {
  const iconColorClass =
    (isIconActive ?? isActive) && activeColor ? activeColor : "";
  const conceptDefinition = concept ? MOTO_CONCEPTS[concept] : null;
  const ConceptIcon = conceptDefinition?.icon;
  const showActiveIcon = isIconActive ?? isActive;
  // Im Streifen ist der Bereichsinhalt nie offen.
  const bodyExpanded = isExpanded && !collapsed;

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
        onKeyDown={(e) => {
          if (e.key === "Enter" || e.key === " ") {
            e.preventDefault();
            onToggle();
          }
        }}
        className={`${sidebarRowClasses({ isActive })} cursor-pointer`}
        aria-expanded={bodyExpanded}
        {...(collapsed ? { title: label, "aria-label": label } : {})}
      >
        {conceptDefinition && ConceptIcon ? (
          showActiveIcon ? (
            <MotoDuotoneIcon
              icon={conceptDefinition.icon}
              tone={conceptDefinition.tone}
              size={20}
              className={SIDEBAR_ICON_CLASSES}
            />
          ) : (
            <ConceptIcon
              size={20}
              weight="regular"
              className={SIDEBAR_ICON_CLASSES}
              aria-hidden="true"
            />
          )
        ) : (
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
        )}
        {labelsMounted && (
          <>
            <span className={sidebarLabelClasses(labelsVisible)}>{label}</span>
            {/* Sammelzähler auf der geschlossenen Kopfzeile; sobald der
                Bereich offen ist, übernehmen die Zähler der Unterpunkte. */}
            {!bodyExpanded && (
              <span
                className={`ml-2 shrink-0 motion-safe:transition-opacity motion-safe:duration-150 ${labelsVisible ? "opacity-100" : "opacity-0"}`}
              >
                <UnreadBadge count={badgeCount} />
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
            className={`absolute top-1 right-1 motion-safe:transition-opacity motion-safe:duration-150 ${labelsVisible ? "opacity-0" : "opacity-100"}`}
          >
            <UnreadBadge count={badgeCount} />
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
        <div className="overflow-hidden" inert={!bodyExpanded}>
          <div className="py-0.5">
            {isLoading && (
              /* Skeleton shimmer */
              <div className="space-y-1 pr-3 pl-11">
                <div className="h-6 w-3/4 animate-pulse rounded bg-gray-100" />
                <div className="h-6 w-2/3 animate-pulse rounded bg-gray-100" />
                <div className="h-6 w-1/2 animate-pulse rounded bg-gray-100" />
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
