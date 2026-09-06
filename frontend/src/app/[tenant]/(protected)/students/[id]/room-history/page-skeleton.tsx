"use client";

import { Skeleton } from "~/components/ui/skeleton";
import { TenantPageHeaderSkeleton } from "~/components/ui/page-skeletons";

// Mirrors the loaded page: back button, student header (name + plain
// two-span meta line), a two-column chart grid, then the history table
// (desktop table + mobile day-card list), so there's no layout shift.
export function RoomHistorySkeleton() {
  return (
    <div
      role="status"
      aria-busy="true"
      aria-label="Anwesenheitsprotokoll wird geladen"
      data-testid="room-history-skeleton"
      className="w-full"
    >
      <Skeleton className="mb-4 h-9 w-24 rounded-lg" />

      {/* Spiegelt die Kopfkarte des geladenen Zustands. */}
      <TenantPageHeaderSkeleton leading />

      <div className="mb-4 grid grid-cols-1 gap-4 md:mb-6 md:grid-cols-2 md:gap-6">
        {Array.from({ length: 2 }, (_, i) => (
          <div
            key={i}
            className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm"
          >
            <div className="p-4 sm:p-6">
              <div className="mb-3 space-y-1.5">
                <Skeleton className="h-5 w-32 rounded" />
                <Skeleton className="h-3 w-40 rounded" />
              </div>
              <Skeleton className="h-[180px] w-full rounded-xl sm:h-[200px]" />
            </div>
          </div>
        ))}
      </div>

      <div className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm">
        <div className="border-b border-gray-100 px-4 py-3 sm:px-6 sm:py-4">
          <Skeleton className="h-5 w-48 rounded" />
          <Skeleton className="mt-1.5 h-3 w-56 rounded" />
        </div>

        {/* Desktop table */}
        <div className="hidden md:block">
          <table className="w-full">
            <thead>
              <tr className="border-b border-gray-100">
                {["Tag", "Ankunft", "Abmeldung", "Dauer", "Räume"].map(
                  (label) => (
                    <th key={label} className="px-6 py-3 text-left">
                      <Skeleton className="h-3 w-14 rounded" />
                    </th>
                  ),
                )}
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-50">
              {Array.from({ length: 4 }, (_, row) => (
                <tr key={row}>
                  {Array.from({ length: 5 }, (_, cell) => (
                    <td key={cell} className="px-6 py-3">
                      <Skeleton className="h-4 w-16 rounded" />
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        {/* Mobile day-card list */}
        <div className="md:hidden">
          {Array.from({ length: 4 }, (_, i) => (
            <div
              key={i}
              className="flex items-center justify-between border-b border-gray-100 px-4 py-3 last:border-b-0"
            >
              <div className="flex items-center gap-3">
                <Skeleton className="h-9 w-9 flex-shrink-0 rounded-full" />
                <div className="space-y-1.5">
                  <Skeleton className="h-4 w-16 rounded" />
                  <Skeleton className="h-3 w-24 rounded" />
                </div>
              </div>
              <Skeleton className="h-5 w-16 rounded-full" />
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
