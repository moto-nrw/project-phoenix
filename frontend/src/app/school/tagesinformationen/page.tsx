"use client";

// Tagesinformationen in moto schule (#2208): die Hinweise der OGS-Leitung, die
// heute für Lehrkräfte gelten. Dieselbe Liste wie im OGS-Portal, gebunden an
// die school-Session; das Backend liefert nur, was "alle" oder "nur
// Lehrkräfte" erreicht. Verwaltet wird im OGS-Portal — hier wird gelesen und
// zur Kenntnis genommen.

import { useEffect } from "react";
import { useSession } from "next-auth/react";

import { TodayNoticeList } from "~/components/staff-notices/today-notice-list";
import { SectionCard } from "~/components/ui/section-card";
import { TenantPage } from "~/components/ui/tenant-page";
import { useApiLoadError } from "~/contexts/ToastContext";
import {
  SCHOOL_NOTICES_TODAY_KEY,
  schoolStaffNoticesApi,
} from "~/lib/school-staff-notices-api";
import type { StaffNotice } from "~/lib/staff-notices-api";
import { useSWRAuth } from "~/lib/swr";

export default function SchoolNoticesPage() {
  const { data: session } = useSession();
  const { data, error, isLoading, mutate } = useSWRAuth<StaffNotice[]>(
    session ? SCHOOL_NOTICES_TODAY_KEY : null,
    schoolStaffNoticesApi.fetchTodaysNotices,
    { revalidateOnFocus: false },
  );
  const notices = data ?? [];
  // Ein Ladefehler darf nicht wie "keine Hinweise" aussehen: er steht über
  // den gemeinsamen Fehlerweg dort, wo die Liste fehlt.
  const load = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = load;
  useEffect(() => {
    if (!error) {
      clearLoadError();
      return;
    }
    void showLoadError(error, {
      object: "die Hinweise der Leitung",
      retry: () => void mutate(),
    });
  }, [error, mutate, showLoadError, clearLoadError]);

  return (
    <TenantPage
      title="Hinweise der Leitung"
      loading={isLoading && notices.length === 0}
      loadingLabel="Hinweise der Leitung werden geladen…"
      error={error ? load.error : null}
      empty={
        !error && notices.length === 0
          ? {
              title: "Heute keine Hinweise",
              description: "Für heute liegen keine Hinweise der OGS vor.",
            }
          : null
      }
    >
      <SectionCard
        title="Heute"
        description="Hinweise der OGS-Leitung an die Lehrkräfte. Sie werden im OGS-Büro angelegt; hier lesen Sie, was heute gilt."
      >
        <TodayNoticeList
          notices={notices}
          onChanged={mutate}
          acknowledge={schoolStaffNoticesApi.acknowledgeStaffNotice}
          layout="grid"
        />
      </SectionCard>
    </TenantPage>
  );
}
