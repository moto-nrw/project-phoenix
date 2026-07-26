"use client";

import { Skeleton } from "~/components/ui/skeleton";

// The PlanningContextBar header placeholder (title, week nav, view switcher,
// "Schichtarten verwalten" action). Only used by the full-page skeleton — once
// the session is ready the real PlanningContextBar renders and only the grid
// area falls back to a skeleton on cold data (docs/05 Abschnitt 5).
function HeaderSkeleton() {
  return (
    <div className="moto-content-surface rounded-2xl border p-3 sm:p-4">
      <div className="flex flex-wrap items-center gap-2">
        <Skeleton className="h-6 w-28 rounded" />
        <div className="flex items-center gap-1">
          <Skeleton className="h-8 w-8 rounded-md" />
          <Skeleton className="h-4 w-44 rounded" />
          <Skeleton className="h-8 w-8 rounded-md" />
        </div>
        <Skeleton className="h-9 w-40 rounded-lg" />
        <Skeleton className="ml-auto h-9 w-48 rounded-lg" />
      </div>
    </div>
  );
}

// The grid surface card: staff column + 5 weekday columns, 6 rows, plus the
// CapacityStrip footer row (label + 5 cells). Shared by both the full-page
// skeleton and the standalone grid skeleton — no status role here so it can be
// composed under either wrapper without nesting two live regions.
function GridSkeletonBody() {
  return (
    <div className="moto-content-surface rounded-2xl border p-4 shadow-sm sm:p-6">
      <div className="max-w-full overflow-x-auto rounded-2xl border border-gray-100">
        <table className="w-full min-w-[960px] border-collapse text-sm">
          <tbody className="divide-y divide-gray-100">
            {Array.from({ length: 6 }, (_, row) => (
              <tr key={row} className="bg-white">
                <td className="px-4 py-2">
                  <Skeleton className="h-4 w-32 rounded" />
                </td>
                {Array.from({ length: 5 }, (_, col) => (
                  <td key={col} className="px-3 py-2">
                    <Skeleton className="h-9 w-full rounded-md" />
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
          <tfoot>
            <tr className="border-t border-gray-200 bg-gray-50">
              <td className="px-4 py-2">
                <Skeleton className="h-4 w-28 rounded" />
              </td>
              {Array.from({ length: 5 }, (_, col) => (
                <td key={col} className="px-3 py-2">
                  <Skeleton className="h-4 w-8 rounded" />
                </td>
              ))}
            </tr>
          </tfoot>
        </table>
      </div>
    </div>
  );
}

// Grid-only skeleton for the content area while the schedule data loads on a
// cold SWR key. The PlanningContextBar stays mounted above it, so week/view
// navigation never disappears mid-click (docs/05 Abschnitt 5).
export function DienstplanGridSkeleton() {
  return (
    <div
      role="status"
      aria-busy="true"
      aria-label="Dienstplan wird geladen"
      data-testid="dienstplan-grid-skeleton"
    >
      <GridSkeletonBody />
    </div>
  );
}

// Full-page skeleton (header + grid) for the session/permission loading state,
// before the real PlanningContextBar can render.
export function DienstplanPageSkeleton() {
  return (
    <div
      role="status"
      aria-busy="true"
      aria-label="Dienstplan wird geladen"
      data-testid="dienstplan-skeleton"
      className="space-y-4"
    >
      <HeaderSkeleton />
      <GridSkeletonBody />
    </div>
  );
}
