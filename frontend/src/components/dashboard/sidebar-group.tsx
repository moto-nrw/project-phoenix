"use client";

import type { ReactNode } from "react";
import type { Icon as PhosphorIcon } from "@phosphor-icons/react";

import { UnreadBadge } from "~/components/messaging/unread-badge";
import { Button } from "~/components/ui/button";
import {
  SIDEBAR_GROUP_HEADING_CLASSES,
  SIDEBAR_ICON_CLASSES,
  SIDEBAR_NAV_GAP,
  sidebarLabelClasses,
} from "~/components/dashboard/sidebar-geometry";

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
   * Summe der Zähler in der Gruppe. Sichtbar nur, solange die Gruppe zu ist;
   * offen tragen die Zeilen ihre Zähler selbst.
   */
  readonly badgeCount?: number;
  /** Farbe des Zählers: Eltern-Zähler blau, Team-Zähler orange, wie an den Zeilen. */
  readonly badgeTone?: "parents" | "staff";
  readonly collapsed?: boolean;
  readonly labelsMounted?: boolean;
  readonly labelsVisible?: boolean;
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
  badgeCount = 0,
  badgeTone = "staff",
  collapsed = false,
  labelsMounted = true,
  labelsVisible = true,
  children,
}: SidebarGroupProps) {
  const iconColor =
    containsActive && !isOpen ? "text-gray-900" : "text-gray-400";
  const showBadge = badgeCount > 0 && !isOpen;

  return (
    <div>
      <Button
        type="button"
        variant="ghost"
        size="md"
        onClick={onToggle}
        className={`${SIDEBAR_GROUP_HEADING_CLASSES} cursor-pointer`}
        aria-expanded={isOpen}
        {...(collapsed ? { title: label, "aria-label": label } : {})}
      >
        <Icon
          size={20}
          weight="regular"
          className={`${SIDEBAR_ICON_CLASSES} ${iconColor}`}
          aria-hidden="true"
        />
        {labelsMounted && (
          <>
            <span
              className={`${sidebarLabelClasses(labelsVisible)} text-[11px] font-semibold tracking-wider uppercase`}
            >
              {label}
            </span>
            {showBadge && (
              <span
                aria-hidden={!labelsVisible}
                className={`ml-2 shrink-0 motion-safe:transition-opacity motion-safe:duration-150 ${labelsVisible ? "opacity-100" : "opacity-0"}`}
              >
                <UnreadBadge count={badgeCount} tone={badgeTone} />
              </span>
            )}
            <svg
              className={`ml-2 h-4 w-4 shrink-0 text-gray-400 motion-safe:transition-[transform,opacity] motion-safe:duration-200 ${isOpen ? "rotate-180" : ""} ${labelsVisible ? "opacity-100" : "opacity-0"}`}
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
        {collapsed && showBadge && (
          <span
            aria-hidden={labelsVisible}
            className={`absolute top-0 right-1 motion-safe:transition-opacity motion-safe:duration-150 ${labelsVisible ? "opacity-0" : "opacity-100"}`}
          >
            <UnreadBadge count={badgeCount} tone={badgeTone} />
          </span>
        )}
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
          <div className={`${SIDEBAR_NAV_GAP} pt-1`}>{children}</div>
        </div>
      </div>
    </div>
  );
}
