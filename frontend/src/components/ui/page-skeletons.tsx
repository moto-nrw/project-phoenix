"use client";

// Shared realistic loading skeletons. Each primitive mirrors the markup of
// its real kit counterpart (DataTable, InfoCard, PageHeaderWithSearch,
// detail-modal-components) so the pulse state has the same silhouette as the
// loaded screen and the swap causes no layout reshuffle.
//
// Composition rule: exactly one wrapper per loading region announces itself
// (`<output aria-label=…>`); the primitives inside are plain presentational
// divs. Use `SkeletonRegion` (or a page-level `<output>`) as that wrapper.

import type React from "react";

import { DataTableSkeleton } from "~/components/ui/data-table";
import { PageHeaderSkeleton } from "~/components/ui/page-header/PageHeaderSkeleton";
import { Skeleton } from "~/components/ui/skeleton";

export { PageHeaderSkeleton } from "~/components/ui/page-header/PageHeaderSkeleton";

// Deterministic width cycle — varied line lengths read as real text without
// Math.random() (which would break SSR hydration and test snapshots).
const LINE_WIDTHS = ["w-3/4", "w-1/2", "w-2/3", "w-5/6", "w-2/5", "w-3/5"];

function lineWidth(i: number): string {
  return LINE_WIDTHS[i % LINE_WIDTHS.length] ?? "w-2/3";
}

/**
 * Announcing wrapper: one per loading region. The visual loading state mounts
 * immediately so every skeleton follows the same timing contract and no
 * invisible placeholder phase can replace already-visible content.
 */
export function SkeletonRegion({
  label,
  className,
  children,
  testId,
}: Readonly<{
  label: string;
  className?: string;
  children: React.ReactNode;
  testId?: string;
}>) {
  return (
    <output
      aria-label={label}
      aria-live="polite"
      aria-busy="true"
      data-testid={testId}
      className={`block w-full ${className ?? ""}`}
    >
      <div>{children}</div>
      <span className="sr-only">{label}</span>
    </output>
  );
}

/**
 * Loading table for call sites without column definitions. Thin alias of
 * the colocated `DataTableSkeleton` in data-table.tsx, which shares its
 * class constants with the real DataTable — zero drift by construction.
 * Prefer `<DataTable isLoading>` whenever the columns exist: that renders
 * the real header labels.
 */
export { DataTableSkeleton as TableSkeleton } from "~/components/ui/data-table";

/** One InfoCard-shaped card: icon tile, title, then label/value rows. */
export function CardSkeleton({ rows = 3 }: Readonly<{ rows?: number }>) {
  return (
    <div className="moto-content-surface flex h-full flex-col rounded-2xl border p-4 shadow-sm backdrop-blur sm:p-6">
      <div className="mb-4 flex items-center gap-3">
        <Skeleton className="h-9 w-9 flex-shrink-0 rounded-xl sm:h-10 sm:w-10" />
        <Skeleton className="h-5 w-36 rounded" />
      </div>
      <div className="flex flex-1 flex-col space-y-3">
        {Array.from({ length: rows }, (_, i) => (
          <div key={i} className="space-y-1">
            <Skeleton className="h-3 w-20 rounded" />
            <Skeleton className={`h-4 rounded ${lineWidth(i)}`} />
          </div>
        ))}
      </div>
    </div>
  );
}

/** Responsive grid of InfoCard-shaped cards (same breakpoints as the list pages). */
export function CardGridSkeleton({
  cards = 6,
  rowsPerCard = 3,
  className = "grid grid-cols-1 gap-6 md:grid-cols-2 xl:grid-cols-3",
}: Readonly<{ cards?: number; rowsPerCard?: number; className?: string }>) {
  return (
    <div className={className}>
      {Array.from({ length: cards }, (_, i) => (
        <CardSkeleton key={i} rows={rowsPerCard} />
      ))}
    </div>
  );
}

/**
 * Mirrors InfoSection + DataGrid from detail-modal-components: section
 * heading, then a two-column grid of label/value field pairs.
 */
export function DetailSectionSkeleton({
  fields = 4,
}: Readonly<{ fields?: number }>) {
  return (
    <div className="moto-content-surface rounded-2xl border p-4 shadow-sm sm:p-6">
      <Skeleton className="mb-4 h-5 w-40 rounded" />
      <div className="grid grid-cols-1 gap-x-6 gap-y-4 sm:grid-cols-2">
        {Array.from({ length: fields }, (_, i) => (
          <div key={i} className="space-y-1.5">
            <Skeleton className="h-3 w-24 rounded" />
            <Skeleton className={`h-4 rounded ${lineWidth(i)}`} />
          </div>
        ))}
      </div>
    </div>
  );
}

/** Stacked detail sections — a whole detail page body. */
export function DetailSkeleton({
  sections = 2,
  fieldsPerSection = 4,
}: Readonly<{ sections?: number; fieldsPerSection?: number }>) {
  return (
    <div className="space-y-4 sm:space-y-6">
      {Array.from({ length: sections }, (_, i) => (
        <DetailSectionSkeleton key={i} fields={fieldsPerSection} />
      ))}
    </div>
  );
}

/** Mirrors a kit form: label + input pairs, footer action buttons. */
export function FormSkeleton({ fields = 5 }: Readonly<{ fields?: number }>) {
  return (
    <div className="moto-content-surface rounded-2xl border p-4 shadow-sm sm:p-6">
      <div className="space-y-5">
        {Array.from({ length: fields }, (_, i) => (
          <div key={i} className="space-y-1.5">
            <Skeleton className="h-4 w-28 rounded" />
            <Skeleton className="h-11 w-full rounded-md" />
          </div>
        ))}
      </div>
      <div className="mt-6 flex justify-end gap-2">
        <Skeleton className="h-9 w-24 rounded-md" />
        <Skeleton className="h-9 w-32 rounded-md" />
      </div>
    </div>
  );
}

/**
 * Stacked list rows (avatar + two text lines + trailing badge) inside the
 * canonical surface — for inboxes, request queues, and tab content.
 */
export function ListSkeleton({
  rows = 6,
  avatar = true,
}: Readonly<{ rows?: number; avatar?: boolean }>) {
  return (
    <div className="moto-content-surface divide-y divide-gray-100 overflow-hidden rounded-2xl border shadow-sm">
      {Array.from({ length: rows }, (_, i) => (
        <div key={i} className="flex items-center gap-3 px-4 py-3.5 sm:px-5">
          {avatar && (
            <Skeleton className="h-9 w-9 flex-shrink-0 rounded-full" />
          )}
          <div className="min-w-0 flex-1 space-y-1.5">
            <Skeleton className={`h-4 rounded ${lineWidth(i)}`} />
            <Skeleton className={`h-3 rounded ${lineWidth(i + 3)}`} />
          </div>
          <Skeleton className="h-6 w-16 flex-shrink-0 rounded-full" />
        </div>
      ))}
    </div>
  );
}

/**
 * Full generic list page: header + table. The right default when a page's
 * loaded state is a PageHeaderWithSearch over a DataTable.
 */
export function ListPageSkeleton({
  label,
  chips = 0,
  rows = 8,
  columns = 5,
  testId,
}: Readonly<{
  label: string;
  chips?: number;
  rows?: number;
  columns?: number;
  testId?: string;
}>) {
  return (
    <SkeletonRegion label={label} testId={testId}>
      <PageHeaderSkeleton chips={chips} />
      <DataTableSkeleton rows={rows} columns={columns} />
    </SkeletonRegion>
  );
}
