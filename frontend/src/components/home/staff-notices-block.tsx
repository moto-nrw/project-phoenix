"use client";

import Link from "~/components/ui/navigation-link";
import { ChevronRight } from "lucide-react";

import { TodayNoticeList } from "~/components/staff-notices/today-notice-list";
import { Alert } from "~/components/ui/alert";
import { EmptyState } from "~/components/ui/empty-state";
import { Loading } from "~/components/ui/loading";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { SectionCard } from "~/components/ui/section-card";
import { fetchTodaysNotices } from "~/lib/staff-notices-api";
import type { StaffNotice } from "~/lib/staff-notices-api";
import { useSWRAuth } from "~/lib/swr";
import { useTenantAwarePath } from "~/lib/tenant-path";

/**
 * Baustein „Tagesinformationen" der Startseite (#2180): die Hinweise der
 * Leitung, die heute gelten, an der Stelle, an der die Person morgens ohnehin
 * landet. Dieselbe Liste wie auf /tagesinformationen, damit eine Kenntnisnahme
 * hier genauso zählt wie dort.
 */
export function StaffNoticesBlock() {
  const tenantPath = useTenantAwarePath();
  const {
    data: notices,
    error,
    isLoading,
    mutate,
  } = useSWRAuth<StaffNotice[]>("staff-notices-today", fetchTodaysNotices, {
    revalidateOnFocus: false,
  });

  return (
    <SectionCard
      title="Tagesinformationen"
      leading={<MotoConceptIcon concept="announcements" size={20} />}
      actions={
        <Link
          href={tenantPath("/tagesinformationen")}
          aria-label="Tagesinformationen: alle ansehen"
          className="flex items-center gap-1 text-sm font-medium text-gray-600 transition-colors hover:text-gray-900"
        >
          Alle ansehen
          <ChevronRight className="h-4 w-4" aria-hidden="true" />
        </Link>
      }
    >
      {(() => {
        if (error) {
          return (
            <Alert
              type="error"
              message="Die Tagesinformationen konnten nicht geladen werden. Bitte die Seite neu laden."
            />
          );
        }
        if (isLoading && notices === undefined) {
          return <Loading fullPage={false} message="Hinweise werden geladen" />;
        }
        if (!notices || notices.length === 0) {
          return (
            <EmptyState
              className="py-8"
              title="Heute gibt es keine Hinweise"
              description="Neue Hinweise der Leitung erscheinen hier."
            />
          );
        }
        return <TodayNoticeList notices={notices} onChanged={mutate} />;
      })()}
    </SectionCard>
  );
}
