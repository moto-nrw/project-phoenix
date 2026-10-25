"use client";

// Ein Ferienzeitraum des Trägers (#4659): ändern und veröffentlichen.

import { useParams, useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import {
  formatDateRange,
  holidayStatusLabel,
} from "~/components/traeger/holiday-period-format";
import { HolidayPeriodForm } from "~/components/traeger/holiday-period-form";
import { HolidayPeriodPublishCard } from "~/components/traeger/holiday-period-publish-card";
import { useTraegerSchools } from "~/components/traeger/use-traeger-schools";
import { BackLink } from "~/components/ui/back-link";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { Loading } from "~/components/ui/loading";
import { StatusBadge } from "~/components/ui/status-badge";
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
    <div className="w-full space-y-6">
      <div className="space-y-2">
        <BackLink href={listHref} label="Zurück zu den Ferien" />
        {period ? (
          <>
            <div className="flex flex-wrap items-center gap-3">
              <h1 className="text-xl font-semibold text-gray-900 sm:text-2xl">
                {period.name}
              </h1>
              <StatusBadge
                label={holidayStatusLabel(period)}
                tone={period.status === "published" ? "green" : "gray"}
              />
            </div>
            <p className="text-sm text-gray-500">
              Anmeldung{" "}
              {formatDateRange(
                period.registrationOpensOn,
                period.registrationClosesOn,
              )}
            </p>
          </>
        ) : null}
      </div>

      {loadError ? (
        <LoadErrorAlert error={loadError} />
      ) : period && schools ? (
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
      ) : (
        <Loading fullPage={false} />
      )}
    </div>
  );
}
