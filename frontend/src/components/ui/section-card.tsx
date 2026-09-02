"use client";

import type { ReactNode } from "react";
import { useId, useState } from "react";
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
 * parents portal's — leave it off. The open state is uncontrolled by default
 * (`defaultCollapsed`); pass `collapsed` when a parent owns it, e.g. to open
 * the section that holds a deep-linked field or to expand every section at
 * once (settings page, #2830).
 */
export function SectionCard({
  kicker,
  title,
  titleBadge,
  description,
  icon: Icon,
  leading,
  action,
  actions,
  bare = false,
  testId,
  collapsible = false,
  defaultCollapsed = false,
  collapsed: collapsedProp,
  onCollapsedChange,
  headingLevel = 2,
  titleClassName,
  bodyClassName,
  className = "",
  /** `visible` lässt Dropdowns aus der Kopfzeile über den Kartenrand ragen. */
  overflow = "hidden",
  children,
  id,
}: Readonly<{
  kicker?: string;
  /**
   * Ohne Titel rendert die Karte keinen Kopf und ist die reine Inhaltsfläche
   * einer Seite. Genau dafür haben Seiten sich vorher `<section
   * className="moto-content-surface …">` selbst gebaut.
   */
  title?: string;
  /** Small badge next to the title, e.g. a `StatusBadge` with a count. */
  titleBadge?: ReactNode;
  description?: ReactNode;
  icon?: LucideIcon;
  /** Existing icon tile or other leading visual for non-Lucide icon systems. */
  leading?: ReactNode;
  /** Single header action. `actions` is the multi-element form. */
  action?: ReactNode;
  actions?: ReactNode;
  /**
   * Ohne eigene Kartenfläche, wenn der Inhalt selbst schon aus Karten besteht
   * (eine Reihe `StatCard`, eine Liste `TileCard`). Sonst steht eine weiße
   * Karte auf einer weißen Karte und beide Ränder werden schwach. Dieselbe
   * Entscheidung trifft das Eltern-Portal mit `ParentSection bare`.
   */
  bare?: boolean;
  /** Testanker; erspart eine zusätzliche Hülle nur für den Selektor. */
  testId?: string;
  collapsible?: boolean;
  defaultCollapsed?: boolean;
  /** Controlled open state; `onCollapsedChange` then reports the requested one. */
  collapsed?: boolean;
  onCollapsedChange?: (collapsed: boolean) => void;
  /** 1 for a page's primary section, 2 (default) for the rest. */
  headingLevel?: 1 | 2 | 3;
  titleClassName?: string;
  /** Overrides the default `mt-4` spacing above the body. */
  bodyClassName?: string;
  className?: string;
  overflow?: "hidden" | "visible";
  children?: ReactNode;
  id?: string;
}>) {
  const [collapsedState, setCollapsedState] = useState(defaultCollapsed);
  const collapsed = collapsedProp ?? collapsedState;
  const bodyId = useId();
  const Heading = `h${headingLevel}` as "h1" | "h2" | "h3";
  const headerActions = actions ?? action;
  const hasBody = children != null && children !== false && children !== "";
  const showBody = hasBody && !(collapsible && collapsed);
  const hasHeader =
    title != null ||
    kicker != null ||
    description != null ||
    leading != null ||
    Icon != null ||
    headerActions != null ||
    collapsible;

  return (
    <section
      id={id}
      data-testid={testId}
      className={
        bare
          ? cn("space-y-4", className)
          : `moto-content-surface ${overflow === "hidden" ? "overflow-hidden" : "overflow-visible"} rounded-2xl border p-5 shadow-sm backdrop-blur-md max-sm:p-4 ${className}`
      }
    >
      {hasHeader && (
        // `flex-wrap` + `flex-1` am Titelblock: der Einklapp-Pfeil bleibt auf
        // dem Telefon in der Titelzeile, nur die Aktionen brechen als volle
        // Zeile um (dev-Fix "keep SectionCard collapse chevron on the title
        // row on mobile").
        <div className="flex flex-wrap items-start gap-3">
          <div className="flex min-w-0 flex-1 gap-3">
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
              {(title != null || titleBadge != null) && (
                <div
                  className={cn(
                    "flex flex-wrap items-center gap-2",
                    kicker && "mt-1",
                  )}
                >
                  {title != null && (
                    <Heading
                      className={cn(
                        "text-base font-semibold text-balance text-gray-900",
                        titleClassName,
                      )}
                    >
                      {title}
                    </Heading>
                  )}
                  {titleBadge}
                </div>
              )}
              {description && (
                <p className="mt-1 max-w-2xl text-sm leading-6 text-gray-600">
                  {description}
                </p>
              )}
            </div>
          </div>
          {headerActions != null && (
            <div className="order-last flex w-full flex-wrap items-center gap-2 sm:order-none sm:w-auto sm:shrink-0">
              {headerActions}
            </div>
          )}
          {collapsible && (
            <Button
              type="button"
              variant="ghost"
              size="icon"
              className="shrink-0"
              aria-label={
                collapsed
                  ? `${title ?? "Abschnitt"} ausklappen`
                  : `${title ?? "Abschnitt"} einklappen`
              }
              aria-expanded={!collapsed}
              aria-controls={showBody ? bodyId : undefined}
              onClick={() => {
                const next = !collapsed;
                if (collapsedProp === undefined) setCollapsedState(next);
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
      {showBody && (
        <div
          id={collapsible ? bodyId : undefined}
          className={
            hasHeader && !bare ? (bodyClassName ?? "mt-4") : bodyClassName
          }
        >
          {children}
        </div>
      )}
    </section>
  );
}
