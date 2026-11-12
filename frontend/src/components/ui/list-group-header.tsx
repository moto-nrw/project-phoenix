"use client";

import { ChevronRight } from "lucide-react";
import { useId, type ReactNode } from "react";

import { Button } from "~/components/ui/button";
import {
  SECTION_LABEL_CLASS,
  SectionLabel,
} from "~/components/ui/section-label";
import { cn } from "~/lib/utils";

interface ListGroupHeaderCollapsible {
  readonly expanded: boolean;
  readonly onToggle: () => void;
  /** What a click does in each state (#6744); a translated list passes its own. */
  readonly tooltip?: { readonly expand: string; readonly collapse: string };
}

const DEFAULT_GROUP_TOOLTIP = {
  expand: "Zeigt die Einträge dieser Gruppe.",
  collapse: "Blendet die Einträge dieser Gruppe aus.",
};

interface ListGroupHeaderProps {
  /** Name der Gruppe, etwa der Tag einer Agenda. */
  readonly title: ReactNode;
  /** Kleiner Zusatz im Klapp-Knopf, der nicht mit dem Namen gekürzt wird. */
  readonly titleAddon?: ReactNode;
  /** Kleiner Zusatz direkt nach dem Namen: ein Hinweis-Chip, ein Zähler. */
  readonly addon?: ReactNode;
  /** Kennzahl oder Steuerung am rechten Rand, etwa „3 Einträge“. */
  readonly meta?: ReactNode;
  /**
   * Ein kurzer Satz unter dem Namen, der für alle Zeilen der Gruppe gilt,
   * etwa was zu tun ist. Er steht einmal, nicht in jeder Zeile.
   */
  readonly description?: ReactNode;
  /** Ebene der Überschrift in der Seite. */
  readonly as?: "h2" | "h3" | "h4";
  /** Der Name klappt die Gruppe auf und zu. */
  readonly collapsible?: ListGroupHeaderCollapsible;
  readonly className?: string;
}

/**
 * Der Kopf einer Gruppe in einer Liste (#5590): grauer Streifen über den
 * Zeilen, links der Name als `SectionLabel`, rechts eine Zahl oder Steuerung.
 * Mit `collapsible` wird der Name zum Knopf, der die Zeilen ein- und
 * ausklappt; die Zeilen rendert der Aufrufer nur, wenn `expanded` gilt.
 */
export function ListGroupHeader({
  title,
  titleAddon,
  addon,
  meta,
  description,
  as = "h3",
  collapsible,
  className,
}: ListGroupHeaderProps) {
  const addonId = useId();
  // Ohne Zusatz bleibt der Name, wie er ist, und bricht bei Bedarf um.
  const staticTitle =
    titleAddon === undefined ? (
      title
    ) : (
      <span className="flex min-w-0 items-center gap-1.5">
        <span className="min-w-0 truncate">{title}</span>
        <span className="shrink-0">{titleAddon}</span>
      </span>
    );
  const row = (
    <div
      className={cn(
        "flex items-center justify-between gap-2 bg-gray-50 px-4 py-2",
        description === undefined ? className : "pb-1",
      )}
    >
      <div className="flex min-w-0 flex-1 flex-wrap items-center gap-2">
        <SectionLabel as={as} className="min-w-0 flex-1">
          {collapsible ? (
            <Button
              type="button"
              variant="ghost"
              size="compact"
              className="-ml-2 w-full min-w-0 justify-start"
              aria-expanded={collapsible.expanded}
              tooltip={
                collapsible.expanded
                  ? (collapsible.tooltip ?? DEFAULT_GROUP_TOOLTIP).collapse
                  : (collapsible.tooltip ?? DEFAULT_GROUP_TOOLTIP).expand
              }
              aria-describedby={addon === undefined ? undefined : addonId}
              onClick={collapsible.onToggle}
            >
              <ChevronRight
                aria-hidden
                className={cn(
                  "h-3.5 w-3.5 shrink-0 text-gray-400 transition-transform",
                  collapsible.expanded && "rotate-90",
                )}
              />
              {/* Ohne `flex-1`: der Knopf ist so breit wie die Zeile, der
                  Zusatz soll aber direkt nach dem Namen stehen, nicht am
                  rechten Rand; `text-left` hebt die Mitte des Knopfs auf. */}
              <span
                className={cn(
                  "min-w-0 truncate text-left",
                  SECTION_LABEL_CLASS,
                )}
              >
                {title}
              </span>
              {titleAddon === undefined ? null : (
                <span className="shrink-0">{titleAddon}</span>
              )}
            </Button>
          ) : (
            staticTitle
          )}
        </SectionLabel>
        {addon === undefined ? null : <span id={addonId}>{addon}</span>}
      </div>
      {meta === undefined ? null : (
        <div className="shrink-0 text-xs text-gray-500 tabular-nums">
          {meta}
        </div>
      )}
    </div>
  );
  if (description === undefined) return row;
  return (
    <div className={cn("bg-gray-50", className)}>
      {row}
      <p className="px-4 pb-2 text-xs text-gray-600">{description}</p>
    </div>
  );
}
