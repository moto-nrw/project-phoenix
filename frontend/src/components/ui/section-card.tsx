"use client";

import type { ReactNode } from "react";
import { useState } from "react";
import { ChevronDown } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { Button } from "~/components/ui/button";
import { cn } from "~/lib/utils";

/**
 * The canonical content section of the calm design language the app converged
 * on (Anmeldungen, Planung, Personal-Stammdaten, Elternportal): a
 * `moto-content-surface` card with a blue uppercase kicker, a `text-base`
 * title, a muted description and an action slot on the right.
 *
 * Use this instead of hand-rolling `<section className="rounded-2xl border …">`
 * with a local header component. `InfoCard` stays the right choice for the
 * denser icon + title stat cards; this one is for page sections that carry a
 * heading, an explanation and actions.
 *
 * `collapsible` opts into the disclosure toggle the staff Stammdaten tab uses
 * (many sections on one long page). Sections that are already short — the
 * parents portal's — leave it off.
 */
export function SectionCard({
  kicker,
  title,
  description,
  icon: Icon,
  leading,
  action,
  actions,
  collapsible = false,
  defaultCollapsed = false,
  onCollapsedChange,
  headingLevel = 2,
  titleClassName,
  bodyClassName,
  className = "",
  children,
  id,
}: Readonly<{
  kicker?: string;
  title: string;
  description?: string;
  icon?: LucideIcon;
  /** Existing icon tile or other leading visual for non-Lucide icon systems. */
  leading?: ReactNode;
  /** Single header action. `actions` is the multi-element form. */
  action?: ReactNode;
  actions?: ReactNode;
  collapsible?: boolean;
  defaultCollapsed?: boolean;
  onCollapsedChange?: (collapsed: boolean) => void;
  /** 1 for a page's primary section, 2 (default) for the rest. */
  headingLevel?: 1 | 2 | 3;
  titleClassName?: string;
  /** Overrides the default `mt-4` spacing above the body. */
  bodyClassName?: string;
  className?: string;
  children?: ReactNode;
  id?: string;
}>) {
  const [collapsed, setCollapsed] = useState(defaultCollapsed);
  const Heading = `h${headingLevel}` as "h1" | "h2" | "h3";
  const headerActions = actions ?? action;
  const showBody = children != null && !(collapsible && collapsed);

  return (
    <section
      id={id}
      className={`moto-content-surface overflow-hidden rounded-2xl border p-5 shadow-sm backdrop-blur-md ${className}`}
    >
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div className="flex min-w-0 gap-3">
          {leading ??
            (Icon && (
              <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-gray-50 text-gray-600 shadow-sm">
                <Icon className="h-5 w-5" aria-hidden="true" />
              </span>
            ))}
          <div className="min-w-0">
            {kicker && (
              <p className="text-moto-blue text-xs font-semibold tracking-wide uppercase">
                {kicker}
              </p>
            )}
            <Heading
              className={cn(
                "text-base font-semibold text-balance text-gray-900",
                kicker && "mt-1",
                titleClassName,
              )}
            >
              {title}
            </Heading>
            {description && (
              <p className="mt-1 max-w-2xl text-sm leading-6 text-gray-600">
                {description}
              </p>
            )}
          </div>
        </div>
        {(headerActions != null || collapsible) && (
          <div className="flex shrink-0 flex-wrap items-center gap-2">
            {headerActions}
            {collapsible && (
              <Button
                type="button"
                variant="ghost"
                size="icon"
                aria-label={
                  collapsed ? `${title} ausklappen` : `${title} einklappen`
                }
                aria-expanded={!collapsed}
                onClick={() => {
                  const next = !collapsed;
                  setCollapsed(next);
                  onCollapsedChange?.(next);
                }}
              >
                <ChevronDown
                  className={`h-4 w-4 transition-transform ${collapsed ? "-rotate-90" : ""}`}
                  aria-hidden="true"
                />
              </Button>
            )}
          </div>
        )}
      </div>
      {showBody && <div className={bodyClassName ?? "mt-4"}>{children}</div>}
    </section>
  );
}
