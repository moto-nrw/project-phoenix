"use client";

import { DataTableSkeleton } from "~/components/ui/data-table";
import { SectionCard } from "~/components/ui/section-card";
import { Skeleton } from "~/components/ui/skeleton";
import { StudentHistoryLoadingBack } from "../student-history-loading-back";
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
      {/* Spiegelt die Kopfkarte des geladenen Zustands. */}
      <TenantPageHeaderSkeleton leading back={<StudentHistoryLoadingBack />} />

      <div className="mb-4 grid grid-cols-1 gap-4 md:mb-6 md:grid-cols-2 md:gap-6">
        {Array.from({ length: 2 }, (_, i) => (
          <SectionCard key={i} padding="none">
            <div className="p-4 sm:p-6">
              <div className="mb-3 space-y-1.5">
                <Skeleton className="h-5 w-32" />
                <Skeleton className="h-3 w-40" />
              </div>
              <Skeleton
                shape="inner"
                className="h-[180px] w-full sm:h-[200px]"
              />
            </div>
          </SectionCard>
        ))}
      </div>

      <SectionCard padding="none">
        <div className="border-b border-gray-100 px-4 py-3 sm:px-6 sm:py-4">
          <Skeleton className="h-5 w-48" />
          <Skeleton className="mt-1.5 h-3 w-56" />
        </div>

        {/* Desktop table */}
        <div className="hidden p-6 md:block">
          <DataTableSkeleton rows={4} columns={5} />
        </div>

        {/* Mobile day-card list */}
        <div className="md:hidden">
          {Array.from({ length: 4 }, (_, i) => (
            <div
              key={i}
              className="flex items-center justify-between border-b border-gray-100 px-4 py-3 last:border-b-0"
            >
              <div className="flex items-center gap-3">
                <Skeleton shape="round" className="h-9 w-9 flex-shrink-0" />
                <div className="space-y-1.5">
                  <Skeleton className="h-4 w-16" />
                  <Skeleton className="h-3 w-24" />
                </div>
              </div>
              <Skeleton shape="round" className="h-5 w-16" />
            </div>
          ))}
        </div>
      </SectionCard>
    </div>
  );
}
