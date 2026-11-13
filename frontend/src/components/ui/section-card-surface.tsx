import type { HTMLAttributes, ReactNode } from "react";
import { cn } from "~/lib/utils";
import { CARD_FRAME, CARD_HOVER } from "./card-surface";

interface SectionCardSurfaceProps extends Omit<
  HTMLAttributes<HTMLElement>,
  "className"
> {
  readonly className?: string;
  readonly overflow?: "hidden" | "visible";
  readonly bare?: boolean;
  /**
   * `article` für eine Karte, die für sich allein steht (eine Serie unter
   * vielen); sonst bleibt es beim Abschnitt.
   */
  readonly as?: "section" | "article";
  /** Hebt die Fläche beim Hovern an (`moto-hover-elevated`), ohne sie klickbar zu machen. */
  readonly hoverElevated?: boolean;
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
  as: Element = "section",
  hoverElevated = false,
  children,
  ...rest
}: Readonly<SectionCardSurfaceProps>) {
  return (
    <Element
      {...rest}
      className={
        bare
          ? cn("space-y-4", className)
          : cn(
              CARD_FRAME,
              hoverElevated && CARD_HOVER,
              overflow === "hidden" ? "overflow-hidden" : "overflow-visible",
              className,
            )
      }
    >
      {children}
    </Element>
  );
}
