import type { HTMLAttributes, ReactNode } from "react";
import { cn } from "~/lib/utils";

// Das eine Kartenpolster (ADR 0149, Entscheidung 9): 20 px, auf dem Telefon
// und in der kompakten Ansicht 16 px.
export const SECTION_CARD_PADDING = "compact:p-4 p-5 max-sm:p-4";

interface SectionCardSurfaceProps extends Omit<
  HTMLAttributes<HTMLElement>,
  "className"
> {
  readonly className?: string;
  readonly overflow?: "hidden" | "visible";
  readonly bare?: boolean;
  readonly children?: ReactNode;
}

/**
 * Die serverfähige Fläche einer SectionCard. Statische Seiten verwenden sie,
 * wenn der Client-Code für einklappbare Karten und Kopfaktionen nicht nötig
 * ist. Für jede Karte mit Kopf, Zustand oder Aktionen bleibt `SectionCard`
 * der Standard.
 */
export function SectionCardSurface({
  className,
  overflow = "hidden",
  bare = false,
  children,
  ...rest
}: Readonly<SectionCardSurfaceProps>) {
  return (
    <section
      {...rest}
      className={
        bare
          ? cn("space-y-4", className)
          : cn(
              "moto-content-surface rounded-2xl border",
              overflow === "hidden" ? "overflow-hidden" : "overflow-visible",
              className,
            )
      }
    >
      {children}
    </section>
  );
}
