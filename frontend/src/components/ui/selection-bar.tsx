"use client";

import { CheckSquare } from "lucide-react";
import { forwardRef, type HTMLAttributes, type ReactNode } from "react";

import { Button } from "~/components/ui/button";
import { cn } from "~/lib/utils";
import { STICKY_ABOVE_BOTTOM_NAV, Z_LAYER } from "./overlay-styles";

/**
 * Wo die Leiste steht:
 * - `top` klebt beim Scrollen oben und bleibt immer eine Fläche.
 * - `bottom` klebt erst, wenn etwas ausgewählt ist. Vorher ist sie eine leise
 *   Zeile im Fluss der Seite und liegt über nichts.
 * - `inline` klebt nicht, etwa in einer vom Aufrufer fixierten Hülle.
 */
type SelectionBarPlacement = "top" | "bottom" | "inline";

function placementClass(placement: SelectionBarPlacement, count: number) {
  if (placement === "top") {
    return `moto-content-surface sticky top-2 ${Z_LAYER.nav} mb-3`;
  }
  if (placement === "inline") return "moto-content-surface";
  return count > 0
    ? `${STICKY_ABOVE_BOTTOM_NAV} ${Z_LAYER.sticky} border-gray-200 bg-white/95 shadow-sm backdrop-blur`
    : "border-transparent bg-gray-50/80 shadow-none";
}

type SelectionBarProps = Omit<HTMLAttributes<HTMLDivElement>, "aria-busy"> & {
  /** Anzahl der markierten Einträge. */
  readonly count: number;
  /** Ersetzt den Text „{count} ausgewählt“. */
  readonly label?: ReactNode;
  /** Eine leise Zeile unter dem Text. */
  readonly description?: ReactNode;
  /** Steht in derselben Zeile direkt hinter dem Text (Umschalter, Hinweise). */
  readonly extra?: ReactNode;
  /** Aktionen rechts, hinter „Aufheben“. */
  readonly actions?: ReactNode;
  /** Zeigt „Aufheben“. Der Knopf ist aus, solange nichts ausgewählt ist. */
  readonly onClear?: () => void;
  /** Eine Aktion läuft: „Aufheben“ ist gesperrt. */
  readonly busy?: boolean;
  readonly placement?: SelectionBarPlacement;
};

/**
 * Die Leiste für eine Mehrfachauswahl: was ausgewählt ist, links, und was sich
 * damit tun lässt, rechts. `aria-label` benennt die Region, `children` sind
 * weitere Zeilen unter der ersten (Zielauswahl, zweite Knopfreihe).
 */
export const SelectionBar = forwardRef<HTMLDivElement, SelectionBarProps>(
  function SelectionBar(
    {
      count,
      label,
      description,
      extra,
      actions,
      onClear,
      busy = false,
      placement = "top",
      className,
      children,
      ...props
    },
    ref,
  ) {
    return (
      <div
        ref={ref}
        role="region"
        aria-busy={busy}
        className={cn(
          "rounded-xl border px-3 py-2 transition-shadow",
          placementClass(placement, count),
          className,
        )}
        {...props}
      >
        <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
          <div
            className={cn(
              "flex min-w-0 items-start gap-1.5",
              // Ein langer Hinweis bricht um, statt die Aktionen in eine eigene
              // Zeile zu schieben.
              description || placement === "inline"
                ? "flex-[1_1_8rem]"
                : "shrink-0",
            )}
          >
            <CheckSquare
              className="text-moto-green mt-0.5 size-4 shrink-0"
              aria-hidden
            />
            <div className="min-w-0">
              <p
                className="text-sm font-semibold text-gray-900 tabular-nums"
                aria-live="polite"
              >
                {label ?? `${count} ausgewählt`}
              </p>
              {description ? (
                <p className="text-sm text-gray-600">{description}</p>
              ) : null}
            </div>
          </div>
          {extra}
          {onClear || actions ? (
            <div className="ml-auto flex flex-wrap items-center gap-1.5">
              {onClear ? (
                <Button
                  type="button"
                  variant="ghost"
                  size="compact"
                  onClick={onClear}
                  disabled={count === 0 || busy}
                >
                  Aufheben
                </Button>
              ) : null}
              {actions}
            </div>
          ) : null}
        </div>
        {children ? <div className="mt-2">{children}</div> : null}
      </div>
    );
  },
);
