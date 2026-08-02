"use client";

import type { ReactNode } from "react";
import { useState } from "react";
import { ChevronDown } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { Button } from "~/components/ui/button";

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
  action,
  actions,
  collapsible = false,
  headingLevel = 2,
  bodyClassName,
  className = "",
  children,
  id,
}: Readonly<{
  kicker?: string;
  title: string;
  description?: string;
  icon?: LucideIcon;
  /** Single header action. `actions` is the multi-element form. */
  action?: ReactNode;
  actions?: ReactNode;
  collapsible?: boolean;
  /** 1 for a page's primary section, 2 (default) for the rest. */
  headingLevel?: 1 | 2 | 3;
  /** Overrides the default `mt-4` spacing above the body. */
  bodyClassName?: string;
  className?: string;
  children?: ReactNode;
  id?: string;
}>) {
  const [collapsed, setCollapsed] = useState(false);
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
          {Icon && (
            <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-gray-50 text-gray-600 shadow-sm">
              <Icon className="h-5 w-5" aria-hidden="true" />
            </span>
          )}
          <div className="min-w-0">
            {kicker && (
              <p className="text-xs font-semibold tracking-wide text-[#5080D8] uppercase">
                {kicker}
              </p>
            )}
            <Heading
              className={`text-base font-semibold text-balance text-gray-900 ${kicker ? "mt-1" : ""}`}
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
                onClick={() => setCollapsed((prev) => !prev)}
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

/**
 * A quiet numeric/textual fact inside a SectionCard. Deliberately understated:
 * no colored dot, no oversized number. Colour is reserved for the single value
 * that genuinely needs attention, passed via `tone="critical"`.
 */
export function StatTile({
  label,
  value,
  tone = "neutral",
}: Readonly<{
  label: string;
  value: ReactNode;
  tone?: "neutral" | "critical";
}>) {
  return (
    <div className="rounded-xl bg-gray-50 px-3 py-2">
      <span
        className={`block text-sm font-semibold ${tone === "critical" ? "text-[#FF3130]" : "text-gray-900"}`}
      >
        {value}
      </span>
      <span className="block text-[11px] font-medium text-gray-500">
        {label}
      </span>
    </div>
  );
}
