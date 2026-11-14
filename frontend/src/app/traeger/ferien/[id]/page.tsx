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
import { useHolidayEditorCalendar } from "~/components/traeger/use-week-no-care-days";
import { HolidayPeriodPublishCard } from "~/components/traeger/holiday-period-publish-card";
import { useTraegerSchools } from "~/components/traeger/use-traeger-schools";
import { useTraegerHolidayCriteria } from "~/components/traeger/use-traeger-resources";
import { metadataLoadError } from "~/components/traeger/metadata-load-error";
import {
  HOLIDAY_BILLING_HINT,
  useHolidayPeriodBillingHref,
} from "~/components/traeger/use-holiday-period-billing-href";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { billingConcept } from "~/lib/concepts/billing";
import { ButtonLink } from "~/components/ui/button";
import { TenantPage } from "~/components/ui/tenant-page";
import type { HolidayPeriod } from "~/lib/traeger-api";
import { useTraegerHolidayPeriod } from "~/components/traeger/use-traeger-resources";
import { useTenantMutateMatching } from "~/lib/swr";
import { traegerKeys } from "~/lib/swr/portal-keys";
import { traegerPath } from "~/lib/traeger-url";

export default function TraegerHolidayPeriodPage() {
  const holidayCalendar = useHolidayEditorCalendar();
  const router = useRouter();
  const params = useParams<{ id: string }>();
  const id = params?.id ?? "";
  const {
    data: period,
    preview,
    loadError: periodError,
    error: periodFailure,
    mutate,
  } = useTraegerHolidayPeriod(id);
  const billingHref = useHolidayPeriodBillingHref(period);
  const invalidatePeriods = useTenantMutateMatching([
    traegerKeys.periodsFamily,
  ]);
  const [dirty, setDirty] = useState(false);
  const {
    schools,
    error: schoolsError,
    fetchError: schoolsFailure,
  } = useTraegerSchools();
  const listHref = traegerPath("/traeger/ferien");
  const { data: criteria } = useTraegerHolidayCriteria();

  const saved = useCallback(
    (next: HolidayPeriod) => {
      void mutate(next, { revalidate: false });
      void invalidatePeriods({ except: [traegerKeys.period(id)] });
    },
    [mutate, invalidatePeriods, id],
  );

  const loadError = periodError ?? schoolsError;
  const shownError = metadataLoadError(
    loadError,
    Boolean(period && schools),
    periodFailure,
    schoolsFailure,
  );

  return (
    <TenantPage
      title={period?.name ?? preview?.name ?? "Ferienzeitraum"}
      back
      backHref={listHref}
      backLabel="Zurück zu den Ferien"
      stats={period ? <HolidayPeriodStatusLine period={period} /> : null}
      statsLoading={!period && !loadError}
      tabs={holidayPeriodTabs(
        id,
        "planung",
        period?.registrationCount,
        period?.allocationMode,
      )}
      actions={
        billingHref ? (
          <ButtonLink
            href={billingHref}
            variant="outline"
            size="md"
            tooltip={HOLIDAY_BILLING_HINT}
          >
            <MotoConceptIcon
              concept={billingConcept}
              size={18}
              className="mr-2"
            />
            Zur Abrechnung
          </ButtonLink>
        ) : null
      }
      error={shownError}
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
            calendar={holidayCalendar}
            key={period.id}
            period={period}
            schools={schools}
            criteria={criteria}
            onDirtyChange={setDirty}
            onCancel={() => router.push(listHref)}
            onSaved={saved}
          />
        </>
      ) : null}
    </TenantPage>
  );
}
