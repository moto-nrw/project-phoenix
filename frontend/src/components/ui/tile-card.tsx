"use client";

import type { ReactNode } from "react";
import Link from "~/components/ui/navigation-link";
import { cn } from "~/lib/utils";
import { cardSurfaceClasses } from "./card-surface";

/**
 * Die anklickbare Kachel des Tenant-Portals: Kind, Mitarbeiter, Raum,
 * Aktivität, Nachricht, Bereich der Datenverwaltung.
 *
 * Vorher hatte jede dieser Flächen ihre eigene Klassenkette — dieselbe
 * Kachel in fünf Schreibweisen, mit drei verschiedenen Schatten. Radius,
 * Fläche, Rahmen, Hover-Anhebung und der Fokusring kommen aus
 * `card-surface.ts`, der Aufrufer bringt nur seinen Inhalt mit.
 *
 * Für Flächen OHNE Klick ist `SectionCard` die richtige Karte, nicht diese.
 */

/**
 * Die Fläche der Kachel als Klassenkette. Für Kacheln, die eigene
 * Zustands-Attribute oder ein anderes Element mitbringen und deshalb nicht
 * durch `TileCard` gehen: die Kind-Kachel mit ihren Check-in-Zuständen und
 * der Auslöser des Statusmenüs in der Kinderakte.
 */
export function tileCardSurface({
  padding = "md",
}: Readonly<{ padding?: "none" | "md" }> = {}): string {
  return cn(
    "group relative block w-full overflow-hidden text-left",
    cardSurfaceClasses({
      hover: true,
      focusRing: true,
      padded: padding === "md",
    }),
  );
}

const NEW_TAB_HINT = "(öffnet in neuem Tab)";

export function TileCard({
  href,
  nativeAnchor = false,
  newTab = false,
  onClick,
  disabled = false,
  padding = "md",
  ariaLabel,
  ariaHasPopup,
  className,
  children,
}: Readonly<{
  /** Ziel, wenn die Kachel navigiert. Ohne `href` wird sie zur Schaltfläche. */
  href?: string;
  /**
   * Navigiert per Dokumentwechsel (`<a>`) statt per Client-Navigation. Nur,
   * wenn die Zielseite eigene Kopfzeilen braucht, die eine Client-Navigation
   * nicht lädt: das Anmeldeformular setzt die Turnstile-CSP, die strengere
   * Richtlinie der Auswahlseite bliebe sonst stehen (#5594).
   */
  nativeAnchor?: boolean;
  /**
   * Öffnet das Ziel in einem neuen Tab (`<a target="_blank">`), etwa eine
   * Vorschau neben dem Editor. Der Screenreader hört „öffnet in neuem Tab“
   * mit, auch wenn die Kachel ein `ariaLabel` trägt.
   */
  newTab?: boolean;
  onClick?: () => void;
  disabled?: boolean;
  /** `none`, wenn der Inhalt seine Ränder selbst setzt (Kopfleiste, Bild). */
  padding?: "none" | "md";
  ariaLabel?: string;
  /** Für Kacheln, die ein Detail-Panel öffnen statt zu navigieren. */
  ariaHasPopup?: "dialog";
  className?: string;
  children: ReactNode;
}>) {
  const shared = cn(
    tileCardSurface({ padding }),
    disabled ? "cursor-not-allowed opacity-70" : "cursor-pointer",
    className,
  );

  if (href && !disabled && newTab) {
    return (
      <a
        href={href}
        target="_blank"
        rel="noopener noreferrer"
        aria-label={ariaLabel ? `${ariaLabel} ${NEW_TAB_HINT}` : undefined}
        className={shared}
      >
        {children}
        <span className="sr-only"> {NEW_TAB_HINT}</span>
      </a>
    );
  }

  if (href && !disabled) {
    const Anchor = nativeAnchor ? "a" : Link;
    return (
      <Anchor href={href} aria-label={ariaLabel} className={shared}>
        {children}
      </Anchor>
    );
  }

  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      aria-label={ariaLabel}
      aria-haspopup={ariaHasPopup}
      className={shared}
    >
      {children}
    </button>
  );
}
