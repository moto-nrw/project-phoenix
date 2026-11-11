"use client";

import type { ReactNode } from "react";
import type { Icon as PhosphorIcon } from "@phosphor-icons/react";

import {
  SidebarRowContent,
  type SidebarRowBadge,
} from "~/components/ui/sidebar-row";
import { Button } from "~/components/ui/button";
import { NavSceneIcon } from "~/components/ui/nav-scene-icon/nav-scene-icon";
import { SECTION_LABEL_CLASS } from "~/components/ui/section-label";
import {
  SIDEBAR_GROUP_HEADING_CLASSES,
  SIDEBAR_GROUP_HEADING_RAIL_BADGE_CLASSES,
  sidebarGroupBodyClasses,
  sidebarGroupIconClasses,
} from "~/components/ui/sidebar-geometry";

interface SidebarGroupProps {
  readonly label: string;
  readonly icon: PhosphorIcon;
  readonly isOpen: boolean;
  readonly onToggle: () => void;
  /**
   * Steht die aktuelle Seite in dieser Gruppe? Eine zugeklappte Gruppe zeigt
   * das am Icon, damit man im Streifen und in der Leiste sieht, wo man ist.
   */
  readonly containsActive: boolean;
  /**
   * Sammelzähler der Gruppe (`combineSidebarRowBadges`). Sichtbar nur,
   * solange die Gruppe zu ist; offen tragen die Zeilen ihre Zähler selbst.
   * `ariaLabel` nennt, was gezählt wird (#6566).
   */
  readonly badge?: SidebarRowBadge | null;
  readonly collapsed?: boolean;
  readonly labelsMounted?: boolean;
  readonly labelsVisible?: boolean;
  // Ziel der geführten Tour der ersten Schritte (#2832).
  readonly tourId?: string;
  readonly children: ReactNode;
}

/**
 * Eine klappbare Gruppe der Seitenleiste (#2826): Kopfzeile mit Icon,
 * Bezeichnung in Kapitälchen und Chevron, darunter die Zeilen der Gruppe.
 *
 * Anders als ein Akkordeon-Bereich führt die Kopfzeile nirgendwohin: sie
 * klappt nur. Und anders als dort bleibt der Inhalt auch im eingeklappten
 * Streifen offen — die Zeilen einer Gruppe haben Icons und passen in 64px,
 * die Unterpunkte eines Akkordeons (nur Text) nicht.
 */
export function SidebarGroup({
  label,
  icon: Icon,
  isOpen,
  onToggle,
  containsActive,
  badge,
  collapsed = false,
  labelsMounted = true,
  labelsVisible = true,
  tourId,
  children,
}: SidebarGroupProps) {
  const shownBadge = badge != null && badge.count > 0 && !isOpen ? badge : null;
  const collapsedLabel = shownBadge
    ? `${label}, ${shownBadge.ariaLabel}`
    : label;

  return (
    <div>
      <Button
        type="button"
        variant="ghost"
        size="md"
        onClick={onToggle}
        data-setup-tour={tourId}
        className={`${SIDEBAR_GROUP_HEADING_CLASSES} cursor-pointer`}
        aria-expanded={isOpen}
        {...(collapsed ? { title: label, "aria-label": collapsedLabel } : {})}
      >
        <SidebarRowContent
          icon={
            <NavSceneIcon
              icon={Icon}
              className={sidebarGroupIconClasses(containsActive && !isOpen)}
            />
          }
          label={label}
          labelClassName={SECTION_LABEL_CLASS}
          badge={shownBadge}
          chevronOpen={isOpen}
          railBadgeClassName={SIDEBAR_GROUP_HEADING_RAIL_BADGE_CLASSES}
          collapsed={collapsed}
          labelsMounted={labelsMounted}
          labelsVisible={labelsVisible}
        />
      </Button>

      {/* Inhalt der Gruppe — Höhenwechsel über CSS-Grid, wie beim Akkordeon. */}
      <div
        className={`grid motion-safe:transition-[grid-template-rows] motion-safe:duration-200 motion-safe:ease-in-out ${
          isOpen ? "grid-rows-[1fr]" : "grid-rows-[0fr]"
        }`}
      >
        {/* inert: eine zugeklappte Gruppe bleibt im Baum, darf aber keinen
            Tastaturfokus fangen. */}
        <div className="overflow-hidden" inert={!isOpen}>
          {/* Eingerückt hinter einer Linie, damit sich die Gruppe als Baum
              liest, nicht als flache Liste mit Zwischenüberschrift. */}
          <div className={sidebarGroupBodyClasses(collapsed)}>{children}</div>
        </div>
      </div>
    </div>
  );
}
