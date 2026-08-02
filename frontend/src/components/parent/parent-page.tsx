"use client";

import type React from "react";
import Link from "next/link";
import { ArrowLeft } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { Alert } from "~/components/ui/alert";
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
 * Page container: one column, one rhythm, one max width.
 *
 * `wide` is for the two pages that render a five-column week (calendar, meal
 * plan) and genuinely need the room; everything else stays at the reading width
 * so a two-line paragraph doesn't stretch across a 27-inch monitor.
 */
export function ParentPage({
  children,
  width = "default",
  className = "",
}: Readonly<{
  children: React.ReactNode;
  width?: "default" | "wide";
  className?: string;
}>) {
  const max = width === "wide" ? "max-w-7xl" : "max-w-5xl";
  return (
    <div className={`mx-auto w-full space-y-5 ${max} ${className}`}>
      {children}
    </div>
  );
}

/**
 * The page's identity block, on the same white surface as the sections below.
 * The calm comes from the typography (blue kicker, `text-xl` title, no oversized
 * display type) and from dropping the old split hero with its dotted panel —
 * not from stripping the card, which left the title floating on the dotted page
 * background.
 */
export function ParentPageHeader({
  kicker,
  title,
  description,
  actions,
  backHref,
  backLabel,
  media,
}: Readonly<{
  kicker?: string;
  title: string;
  description?: string;
  actions?: React.ReactNode;
  backHref?: string;
  backLabel?: string;
  /** Optional leading element, e.g. a child's initials avatar. */
  media?: React.ReactNode;
}>) {
  return (
    <header className="moto-content-surface space-y-3 rounded-2xl border p-5 shadow-sm backdrop-blur-md">
      {backHref && backLabel && (
        <ParentBackLink href={backHref} label={backLabel} />
      )}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div className="flex min-w-0 gap-3">
          {media}
          <div className="min-w-0">
            {kicker && (
              <p className="text-xs font-semibold tracking-wide text-[#5080D8] uppercase">
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
export function ParentBackLink({
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
 * Primary page action rendered as a link. Matches `Button` `variant="primary"`
 * `size="md"` so a link and a button can sit next to each other without a
 * height or weight mismatch.
 */
export function ParentLinkAction({
  href,
  children,
  variant = "primary",
  className = "",
}: Readonly<{
  href: string;
  children: React.ReactNode;
  variant?: "primary" | "secondary";
  className?: string;
}>) {
  const styles =
    variant === "primary"
      ? "bg-gray-900 text-white hover:bg-gray-700"
      : "border border-gray-200 bg-white text-gray-700 hover:bg-gray-50";
  return (
    <Link
      href={href}
      className={`inline-flex h-9 items-center justify-center gap-2 rounded-lg px-3 text-sm font-medium shadow-sm transition-colors focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none ${styles} ${className}`}
    >
      {children}
    </Link>
  );
}

/**
 * A labelled fact inside a section. Replaces the three different `InfoRow` /
 * `CompactInfoRow` implementations the portal used to carry, and reads the same
 * at every breakpoint (label above value, no desktop-only two-column split).
 */
export function ParentField({
  label,
  children,
}: Readonly<{ label: string; children: React.ReactNode }>) {
  return (
    <div className="min-w-0">
      <dt className="text-[11px] font-medium tracking-wide text-gray-500 uppercase">
        {label}
      </dt>
      <dd className="mt-0.5 text-sm font-medium break-words text-gray-900">
        {children}
      </dd>
    </div>
  );
}

/** Grid of `ParentField`s. */
export function ParentFieldGrid({
  children,
  className = "",
}: Readonly<{ children: React.ReactNode; className?: string }>) {
  return (
    <dl className={`grid gap-4 sm:grid-cols-2 ${className}`}>{children}</dl>
  );
}

/**
 * Compact status line inside a section: neutral icon tile, label, value.
 * Used for the child page's "Heute" facts, where a colored badge per row would
 * turn three quiet statements into an alarm.
 */
export function ParentStatusRow({
  icon: Icon,
  label,
  children,
}: Readonly<{
  icon: LucideIcon;
  label: string;
  children: React.ReactNode;
}>) {
  return (
    <div className="flex items-start gap-3 rounded-xl bg-gray-50 px-3 py-2.5">
      <span className="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-white text-gray-600 shadow-sm">
        <Icon className="h-4 w-4" aria-hidden="true" />
      </span>
      <div className="min-w-0 flex-1">
        <p className="text-[11px] font-medium tracking-wide text-gray-500 uppercase">
          {label}
        </p>
        <div className="mt-0.5 text-sm font-medium break-words text-gray-900">
          {children}
        </div>
      </div>
    </div>
  );
}

/** Shared load-failure block. */
export function ParentLoadError({ message }: Readonly<{ message: string }>) {
  return (
    <ParentPage>
      <Alert type="error" message={message} />
    </ParentPage>
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
