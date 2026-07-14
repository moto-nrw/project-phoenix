"use client";

import { Skeleton } from "~/components/ui/skeleton";

// Mirrors the real Dienstplan layout: the "Schichtarten verwalten" toolbar
// button, the card's 3-col sub-header (helper text, week-nav, "Diese Woche"),
// and the week grid (staff column + 5 weekday columns), so the loaded page
// swaps in without layout shift.
export function DienstplanPageSkeleton() {
  return (
    <div
      role="status"
      aria-busy="true"
      aria-label="Dienstplan wird geladen"
      data-testid="dienstplan-skeleton"
      className="space-y-4"
    >
      <div className="flex justify-end">
        <Skeleton className="h-9 w-56 rounded-lg" />
      </div>
      <div className="moto-content-surface rounded-2xl border p-4 shadow-sm sm:p-6">
        <div className="mb-4 flex flex-col gap-3 sm:grid sm:grid-cols-3 sm:items-center">
          <Skeleton className="hidden h-3 w-64 rounded sm:block" />
          <div className="flex min-w-0 items-center justify-center gap-2">
            <Skeleton className="h-7 w-7 flex-shrink-0 rounded-full" />
            <Skeleton className="h-4 w-40 rounded sm:w-56" />
            <Skeleton className="h-7 w-7 flex-shrink-0 rounded-full" />
          </div>
          <div className="flex justify-center sm:justify-end">
            <Skeleton className="h-6 w-24 rounded-full" />
          </div>
        </div>
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
          </table>
        </div>
      </div>
    </div>
  );
}
