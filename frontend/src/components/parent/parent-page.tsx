"use client";

import type React from "react";
import Link from "next/link";
import { ArrowLeft } from "lucide-react";
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
  variant = "surface",
}: Readonly<{
  kicker?: string;
  title: string;
  description?: string;
  actions?: React.ReactNode;
  backHref?: string;
  backLabel?: string;
  /** Optional leading element, e.g. a child's initials avatar. */
  media?: React.ReactNode;
  /** A quiet page introduction for dashboards that already sit inside app chrome. */
  variant?: "surface" | "plain";
}>) {
  return (
    <header
      className={
        variant === "plain"
          ? "space-y-2 px-1 py-1"
          : "moto-content-surface space-y-3 rounded-2xl border p-5 shadow-sm backdrop-blur-md"
      }
    >
      {backHref && backLabel && (
        <ParentBackLink href={backHref} label={backLabel} />
      )}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div className="flex min-w-0 gap-3">
          {media}
          <div className="min-w-0">
            {kicker && (
              <p className="text-[15px] font-semibold text-[#5080D8]">
                {kicker}
              </p>
            )}
            <h1
              className={`text-xl font-semibold text-balance break-words text-gray-900 sm:text-2xl ${kicker ? "mt-1" : ""}`}
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
        {actions && (
          <div className="flex shrink-0 flex-wrap items-center gap-2">
            {actions}
          </div>
        )}
      </div>
    </header>
  );
}

/** Text back link. Quiet by design — navigation, not an action. */
function ParentBackLink({
  href,
  label,
}: Readonly<{ href: string; label: string }>) {
  return (
    <Link
      href={href}
      className="-ml-2 inline-flex h-8 items-center gap-1.5 rounded-lg px-2 text-sm font-medium text-gray-600 transition-colors hover:bg-gray-100 hover:text-gray-900 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
    >
      <ArrowLeft className="h-4 w-4" aria-hidden="true" />
      {label}
    </Link>
  );
}

/**
 * Shared page skeleton. `rows` controls how many section placeholders follow
 * the header so a page can approximate its own shape without redefining the
 * whole block.
 */
export function ParentPageSkeleton({ rows = 2 }: Readonly<{ rows?: number }>) {
  return (
    <ParentPage>
      <Skeleton className="h-28 w-full rounded-2xl" />
      {Array.from({ length: rows }, (_, index) => (
        <Skeleton key={index} className="h-40 w-full rounded-2xl" />
      ))}
    </ParentPage>
  );
}
