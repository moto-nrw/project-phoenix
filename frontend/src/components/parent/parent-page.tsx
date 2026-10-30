"use client";

import type React from "react";
import { useTranslations } from "next-intl";
import { MobilePageHelpTarget } from "~/components/help/mobile-page-help-target";
import { BackLink } from "~/components/ui/back-link";
import { Skeleton } from "~/components/ui/skeleton";

/**
 * Layout primitives shared by every parents-portal page.
 *
 * Before this module each page hand-rolled its own hero panel, panel header,
 * back bar and skeleton — six variants of the same three elements, in three
 * different type scales. Everything here follows the calm design language of
 * the Anmeldungen / Planung screens: blue uppercase kicker, `text-xl` page
 * title, `text-base` section titles, gray-900 primary actions.
 */

/**
 * Page container: one column, one rhythm, full width.
 *
 * No `max-w-*` cap — the parents portal fills the shell's content area like
 * every tenant page does (`AppShell` already supplies the page padding). Long
 * prose stays readable because the text blocks inside carry their own
 * `max-w-2xl`, not because the whole page is narrowed.
 */
export function ParentPage({
  children,
  className = "",
}: Readonly<{
  children: React.ReactNode;
  className?: string;
}>) {
  return <div className={`w-full space-y-5 ${className}`}>{children}</div>;
}

/**
 * The page identity block. Detail and list pages use the default white surface.
 * Dashboards may use the plain variant when the app chrome already provides
 * enough context and daily content should stay above the fold.
 */
export function ParentPageHeader({
  kicker,
  title,
  description,
  actions,
  backHref,
  backLabel,
  media,
  prominent = false,
}: Readonly<{
  kicker?: string;
  title: string;
  description?: string;
  actions?: React.ReactNode;
  backHref?: string;
  backLabel?: string;
  /** Optional leading element, e.g. a child's initials avatar. */
  media?: React.ReactNode;
  /** Gives dashboard greetings one clear step above section headings. */
  prominent?: boolean;
}>) {
  return (
    <header className="moto-content-surface space-y-3 rounded-2xl border p-5 shadow-sm backdrop-blur-md">
      {backHref && backLabel && <BackLink href={backHref} label={backLabel} />}
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="flex min-w-0 flex-1 basis-0 gap-3">
          {media}
          <div className="min-w-0">
            {kicker && (
              <p className="text-moto-blue text-xs font-semibold tracking-wide uppercase">
                {kicker}
              </p>
            )}
            <h1
              className={`${prominent ? "text-2xl sm:text-[28px]" : "text-xl sm:text-2xl"} leading-tight font-semibold tracking-tight text-balance break-words text-gray-900 ${kicker ? "mt-1" : ""}`}
            >
              {title}
            </h1>
            {description && (
              <p className="mt-1 max-w-2xl text-sm leading-6 text-gray-600">
                {description}
              </p>
            )}
          </div>
        </div>
        {actions ? (
          <div className="contents sm:flex sm:shrink-0 sm:items-center sm:gap-2">
            <div className="flex shrink-0 flex-wrap items-center gap-2 max-sm:order-last max-sm:basis-full">
              {actions}
            </div>
            <MobilePageHelpTarget />
          </div>
        ) : (
          <MobilePageHelpTarget />
        )}
      </div>
    </header>
  );
}

/**
 * Shared page skeleton. `rows` controls how many section placeholders follow
 * the header so a page can approximate its own shape without redefining the
 * whole block.
 */
export function ParentPageSkeleton({ rows = 2 }: Readonly<{ rows?: number }>) {
  const t = useTranslations("parentNav");
  return (
    <ParentPage>
      <div
        data-testid="parent-page-header-skeleton"
        className="moto-content-surface rounded-2xl border p-5 shadow-sm backdrop-blur-md"
        role="status"
      >
        <span className="sr-only">{t("sectionLoading")}</span>
        <Skeleton aria-hidden="true" className="h-3 w-24" />
        <Skeleton aria-hidden="true" className="mt-2 h-7 w-56 max-w-3/4" />
        <Skeleton aria-hidden="true" className="mt-2 h-4 w-full max-w-xl" />
        <Skeleton aria-hidden="true" className="mt-2 h-4 w-2/3 max-w-md" />
      </div>
      {Array.from({ length: rows }, (_, index) => (
        <ParentSectionSkeleton key={index} announce={false} />
      ))}
    </ParentPage>
  );
}

export function ParentSectionSkeleton({
  rows = 3,
  showHeader = true,
  className = "",
  announce = true,
}: Readonly<{
  rows?: number;
  showHeader?: boolean;
  className?: string;
  /** False only when another status covers the same loading phase. */
  announce?: boolean;
}>) {
  const t = useTranslations("parentNav");
  // Die Fläche selbst ist nur Grafik. Der Status sagt Screenreadern, dass
  // hier noch Inhalt kommt, etwa beim ersten Öffnen eines Reiters (#4994).
  return (
    <div
      data-testid="parent-page-section-skeleton"
      className={`moto-content-surface overflow-hidden rounded-2xl border shadow-sm ${className}`}
      role={announce ? "status" : undefined}
      aria-hidden={announce ? undefined : true}
    >
      {announce ? <span className="sr-only">{t("sectionLoading")}</span> : null}
      <div aria-hidden="true">
        {showHeader ? (
          <div className="flex items-center gap-3 border-b border-gray-100 p-4 sm:px-5">
            <Skeleton className="size-10 shrink-0 rounded-xl" />
            <div className="min-w-0 flex-1 space-y-2">
              <Skeleton className="h-5 w-40 max-w-2/3" />
              <Skeleton className="h-3 w-56 max-w-full" />
            </div>
          </div>
        ) : null}
        <div className="divide-y divide-gray-100 px-4 sm:px-5">
          {Array.from({ length: rows }, (_, rowIndex) => (
            <div
              key={rowIndex}
              data-testid="parent-page-section-row-skeleton"
              className="flex min-h-16 items-center gap-3 py-3"
            >
              <div className="min-w-0 flex-1 space-y-2">
                <Skeleton className="h-4 w-2/3" />
                <Skeleton className="h-3 w-1/2" />
              </div>
              <Skeleton className="h-8 w-12 shrink-0 rounded-lg" />
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
