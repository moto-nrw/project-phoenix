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
}

interface ListGroupHeaderProps {
  /** Name der Gruppe, etwa der Tag einer Agenda. */
  readonly title: ReactNode;
  /** Kleiner Zusatz direkt nach dem Namen: ein Hinweis-Chip, ein Zähler. */
  readonly addon?: ReactNode;
  /** Kennzahl oder Steuerung am rechten Rand, etwa „3 Einträge“. */
  readonly meta?: ReactNode;
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
  addon,
  meta,
  as = "h3",
  collapsible,
  className,
}: ListGroupHeaderProps) {
  const addonId = useId();
  return (
    <div
      className={cn(
        "flex items-center justify-between gap-2 bg-gray-50 px-4 py-2",
        className,
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
              <span className={cn("truncate", SECTION_LABEL_CLASS)}>
                {title}
              </span>
            </Button>
          ) : (
            title
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
}
