"use client";

// Ein Ferienzeitraum des Trägers (#4659): ändern und veröffentlichen. Der
// Reiter „Anmeldungen“ führt zur Prüfung der Anmeldungen (#4663).

import { useParams, useRouter } from "next/navigation";
import { useCallback, useState } from "react";
import {
  HolidayPeriodStatusLine,
  holidayPeriodTabs,
} from "~/components/traeger/holiday-period-head";
import { HolidayPeriodForm } from "~/components/traeger/holiday-period-form";
import { HolidayPeriodPublishCard } from "~/components/traeger/holiday-period-publish-card";
import { useTraegerSchools } from "~/components/traeger/use-traeger-schools";
import { TenantPage } from "~/components/ui/tenant-page";
import type { HolidayPeriod } from "~/lib/traeger-api";
import { useTraegerHolidayPeriod } from "~/components/traeger/use-traeger-resources";
import { useTenantMutateMatching } from "~/lib/swr";
import { traegerKeys } from "~/lib/swr/portal-keys";
import { traegerPath } from "~/lib/traeger-url";

export default function TraegerHolidayPeriodPage() {
  const router = useRouter();
  const params = useParams<{ id: string }>();
  const id = params?.id ?? "";
  const {
    data: period,
    preview,
    loadError: periodError,
    mutate,
  } = useTraegerHolidayPeriod(id);
  const invalidatePeriods = useTenantMutateMatching([
    traegerKeys.periodsFamily,
  ]);
  const [dirty, setDirty] = useState(false);
  const { schools, error: schoolsError } = useTraegerSchools();
  const listHref = traegerPath("/traeger/ferien");

  const saved = useCallback(
    (next: HolidayPeriod) => {
      void mutate(next, { revalidate: false });
      void invalidatePeriods({ except: [traegerKeys.period(id)] });
    },
    [mutate, invalidatePeriods, id],
  );

  const loadError = periodError ?? schoolsError;

  return (
    <TenantPage
      title={period?.name ?? preview?.name ?? "Ferienzeitraum"}
      back
      backHref={listHref}
      backLabel="Zurück zu den Ferien"
      stats={period ? <HolidayPeriodStatusLine period={period} /> : null}
      statsLoading={!period && !loadError}
      tabs={holidayPeriodTabs(id, "planung", period?.registrationCount)}
      error={loadError}
      loading={!loadError && !(period && schools)}
    >
      {period && schools ? (
        <>
          <HolidayPeriodPublishCard
            period={period}
            unsavedChanges={dirty}
            onPublished={saved}
          />
          <HolidayPeriodForm
            key={period.id}
            period={period}
            schools={schools}
            onDirtyChange={setDirty}
            onCancel={() => router.push(listHref)}
            onSaved={saved}
          />
        </>
      ) : null}
    </TenantPage>
  );
}
