"use client";

import { Skeleton } from "~/components/ui/skeleton";

// Mirrors the real Dienstplan layout so the loaded page swaps in without layout
// shift: the PlanningContextBar header (title, week nav, view switcher,
// "Schichtarten verwalten" action), the week grid (staff column + 5 weekday
// columns, 6 rows), and the CapacityStrip footer row (label + 5 cells). Used
// for both loading states — session/permission and data (docs/05 Abschnitt 5).
export function DienstplanPageSkeleton() {
  return (
    <div
      role="status"
      aria-busy="true"
      aria-label="Dienstplan wird geladen"
      data-testid="dienstplan-skeleton"
      className="space-y-4"
    >
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
    </div>
  );
}
