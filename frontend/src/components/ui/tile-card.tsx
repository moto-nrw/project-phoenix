"use client";

import type { KeyboardEvent, ReactNode } from "react";
import Link from "~/components/ui/navigation-link";
import { cn } from "~/lib/utils";

/**
 * Die anklickbare Kachel des Tenant-Portals: Kind, Mitarbeiter, Raum,
 * Aktivität, Nachricht, Bereich der Datenverwaltung.
 *
 * Vorher hatte jede dieser Flächen ihre eigene Klassenkette — dieselbe
 * Kachel in fünf Schreibweisen, mit drei verschiedenen Schatten. Radius,
 * Fläche, Rahmen, Hover-Anhebung und der Fokusring liegen jetzt hier, der
 * Aufrufer bringt nur seinen Inhalt mit.
 *
 * Für Flächen OHNE Klick ist `SectionCard` die richtige Karte, nicht diese.
 */
/**
 * Die Fläche der Kachel als Klassenkette. Nur für Kacheln, die eigene
 * Zustands-Attribute mitbringen und deshalb nicht durch `TileCard` gehen
 * (die Kind-Kachel mit ihren Check-in-Zuständen). Eine Definition, zwei
 * Verwendungen — keine zweite Schreibweise.
 */
export const TILE_CARD_SURFACE =
  "group moto-content-surface moto-hover-elevated relative block w-full overflow-hidden rounded-2xl border text-left focus-visible:outline-2 focus-visible:outline-gray-300 focus-visible:outline-offset-2";

export function TileCard({
  href,
  nativeAnchor = false,
  onClick,
  disabled = false,
  padding = "md",
  containsControls = false,
  ariaLabel,
  ariaHasPopup,
  ariaControls,
  ariaExpanded,
  elementRef,
  className,
  children,
  testId,
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
  onClick?: () => void;
  disabled?: boolean;
  /** `none`, wenn der Inhalt seine Ränder selbst setzt (Kopfleiste, Bild). */
  padding?: "none" | "md";
  /**
   * Die Kachel trägt eigene Bedienelemente (Menü, Schalter). Dann ist sie
   * keine Schaltfläche — verschachtelte Schaltflächen sind ungültiges HTML —,
   * sondern eine Fläche mit `role="button"` und Tastaturbedienung.
   */
  containsControls?: boolean;
  ariaLabel?: string;
  /** Für Kacheln, die ein Detail-Panel öffnen statt zu navigieren. */
  ariaHasPopup?: "dialog";
  ariaControls?: string;
  ariaExpanded?: boolean;
  /**
   * Zugriff auf das Element, etwa um den Fokus zurückzugeben, nachdem ein
   * Detail-Panel geschlossen wurde.
   */
  elementRef?: (node: HTMLButtonElement | null) => void;
  className?: string;
  children: ReactNode;
  testId?: string;
}>) {
  const shared = cn(
    TILE_CARD_SURFACE,
    padding === "md" && "p-5 max-sm:p-4 compact:p-4",
    disabled ? "cursor-not-allowed opacity-70" : "cursor-pointer",
    className,
  );

  if (href && !disabled) {
    const Anchor = nativeAnchor ? "a" : Link;
    return (
      <Anchor
        href={href}
        aria-label={ariaLabel}
        className={shared}
        data-testid={testId}
      >
        {children}
      </Anchor>
    );
  }

  if (containsControls) {
    const interactive = Boolean(onClick) && !disabled;
    return (
      <div
        {...(interactive
          ? {
              role: "button" as const,
              tabIndex: 0,
              onClick,
              onKeyDown: (event: KeyboardEvent<HTMLDivElement>) => {
                if (event.key === "Enter" || event.key === " ") {
                  event.preventDefault();
                  onClick?.();
                }
              },
            }
          : {})}
        aria-label={ariaLabel}
        className={cn(shared, !interactive && "cursor-default")}
        data-testid={testId}
      >
        {children}
      </div>
    );
  }

  return (
    <button
      type="button"
      ref={elementRef}
      onClick={onClick}
      disabled={disabled}
      aria-label={ariaLabel}
      aria-haspopup={ariaHasPopup}
      aria-controls={ariaControls}
      aria-expanded={ariaExpanded}
      className={shared}
      data-testid={testId}
    >
      {children}
    </button>
  );
}
