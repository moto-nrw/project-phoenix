"use client";

// Tagesinformationen in moto schule (#2208): die Hinweise der OGS-Leitung, die
// heute für Lehrkräfte gelten. Dieselbe Liste wie im OGS-Portal, gebunden an
// die school-Session; das Backend liefert nur, was "alle" oder "nur
// Lehrkräfte" erreicht. Verwaltet wird im OGS-Portal — hier wird gelesen und
// zur Kenntnis genommen.

import { useSession } from "next-auth/react";

import { TodayNoticeList } from "~/components/staff-notices/today-notice-list";
import { Alert } from "~/components/ui/alert";
import { Loading } from "~/components/ui/loading";
import { SectionCard } from "~/components/ui/section-card";
import { getApiErrorMessage } from "~/lib/api-error-message";
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

  return (
    <div className="-mt-1.5 w-full space-y-6">
      <div>
        <h1 className="text-xl font-semibold text-gray-900 sm:text-2xl">
          Tagesinformationen
        </h1>
        <p className="mt-1 text-sm text-gray-500">
          Hinweise der OGS-Leitung an die Lehrkräfte. Sie werden im OGS-Büro
          angelegt; hier lesen Sie, was heute gilt.
        </p>
      </div>

      <SectionCard title="Heute">
        {isLoading && notices.length === 0 ? (
          <Loading fullPage={false} />
        ) : error ? (
          // Ein Ladefehler darf nicht wie "keine Hinweise" aussehen.
          <Alert
            type="error"
            message={getApiErrorMessage(
              error,
              "laden",
              "die Tagesinformationen",
              "Die Tagesinformationen konnten nicht geladen werden.",
            )}
          />
        ) : notices.length === 0 ? (
          <p className="text-sm leading-6 text-gray-600">
            Für heute liegen keine Hinweise der OGS vor.
          </p>
        ) : (
          <TodayNoticeList
            notices={notices}
            onChanged={mutate}
            acknowledge={schoolStaffNoticesApi.acknowledgeStaffNotice}
          />
        )}
      </SectionCard>
    </div>
  );
}
