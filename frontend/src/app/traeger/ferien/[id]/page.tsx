"use client";

// Ein Ferienzeitraum des Trägers (#4659): ändern und veröffentlichen. Der
// Reiter „Anmeldungen“ führt zur Prüfung der Anmeldungen (#4663).

import { useParams, useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import {
  HolidayPeriodStatusLine,
  holidayPeriodTabs,
} from "~/components/traeger/holiday-period-head";
import { HolidayPeriodForm } from "~/components/traeger/holiday-period-form";
import { HolidayPeriodPublishCard } from "~/components/traeger/holiday-period-publish-card";
import { useTraegerSchools } from "~/components/traeger/use-traeger-schools";
import { TenantPage } from "~/components/ui/tenant-page";
import { useApiLoadError } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import { fetchHolidayPeriod, type HolidayPeriod } from "~/lib/traeger-api";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "TraegerHolidayPeriodPage" });

export default function TraegerHolidayPeriodPage() {
  const router = useRouter();
  const params = useParams<{ id: string }>();
  const id = params?.id ?? "";
  const [period, setPeriod] = useState<HolidayPeriod | null>(null);
  const [dirty, setDirty] = useState(false);
  const { schools, error: schoolsError } = useTraegerSchools();
  const periodLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = periodLoad;
  const listHref = traegerPath("/traeger/ferien");

  const load = useCallback(async () => {
    try {
      setPeriod(await fetchHolidayPeriod(id));
      clearLoadError();
    } catch (err) {
      logger.error("traeger_holiday_period_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "den Ferienzeitraum",
        retry: () => void load(),
      });
    }
  }, [clearLoadError, id, showLoadError]);

  useEffect(() => {
    void load();
  }, [load]);

  const loadError = periodLoad.error ?? schoolsError;

  return (
    <TenantPage
      title={period?.name ?? "Ferienzeitraum"}
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
            onPublished={setPeriod}
          />
          <HolidayPeriodForm
            key={period.id}
            period={period}
            schools={schools}
            onDirtyChange={setDirty}
            onCancel={() => router.push(listHref)}
            onSaved={setPeriod}
          />
        </>
      ) : null}
    </TenantPage>
  );
}
