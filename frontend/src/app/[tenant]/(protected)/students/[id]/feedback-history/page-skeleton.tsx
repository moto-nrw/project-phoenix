"use client";

import { SectionCard } from "~/components/ui/section-card";
import { Skeleton } from "~/components/ui/skeleton";
import { StudentHistoryLoadingBack } from "../student-history-loading-back";
import { TenantPageHeaderSkeleton } from "~/components/ui/page-skeletons";

// Mirrors the header block, den Zeitraum-Umschalter und die Karte mit
// Anteilsleiste, Legende und Diagramm, so it swaps in without layout shift.
export function FeedbackHistorySkeleton() {
  return (
    <div
      role="status"
      aria-busy="true"
      aria-label="Feedbackhistorie wird geladen"
      data-testid="feedback-history-skeleton"
      className="w-full"
    >
      {/* Spiegelt die Kopfkarte des geladenen Zustands. */}
      <TenantPageHeaderSkeleton leading back={<StudentHistoryLoadingBack />} />

      <div className="mb-6">
        <Skeleton shape="round" className="h-9 w-full max-w-md" />
      </div>

      <SectionCard padding="none">
        <div className="p-4 sm:p-6 md:p-8">
          <Skeleton className="mb-4 h-5 w-40" />

          <Skeleton shape="round" className="mt-4 h-3 w-full" />

          <div className="mt-3 flex flex-wrap gap-x-5 gap-y-1">
            {Array.from({ length: 3 }, (_, i) => (
              <div key={i} className="flex items-center gap-1.5">
                <Skeleton shape="round" className="h-2.5 w-2.5" />
                <Skeleton className="h-4 w-16" />
              </div>
            ))}
          </div>

          <Skeleton
            shape="control"
            className="mt-6 h-[180px] w-full sm:h-[220px]"
          />
        </div>
      </SectionCard>
    </div>
  );
}
