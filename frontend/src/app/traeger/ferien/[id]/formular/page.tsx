"use client";

// Anmeldeformular eines Ferienzeitraums (#5198): die Fragen, die Eltern bei
// der Ferienanmeldung zusätzlich zu den festen Angaben beantworten. Jede
// gespeicherte Änderung ist eine neue Version; eine abgeschickte Anmeldung
// behält ihre Version.

import { useParams } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import {
  HolidayPeriodStatusLine,
  holidayPeriodTabs,
} from "~/components/traeger/holiday-period-head";
import { HolidayFormEditor } from "~/components/traeger/holiday-form-editor";
import { TenantPage } from "~/components/ui/tenant-page";
import { useApiLoadError } from "~/contexts/ToastContext";
import { formatBerlinDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import { useTraegerHolidayPeriod } from "~/components/traeger/use-traeger-resources";
import {
  fetchHolidayForm,
  type HolidayForm,
} from "~/lib/traeger-holiday-form-api";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "TraegerHolidayFormPage" });

export default function TraegerHolidayFormPage() {
  const params = useParams<{ id: string }>();
  const id = params?.id ?? "";
  const {
    data: period,
    preview,
    loadError: periodError,
  } = useTraegerHolidayPeriod(id);
  const [form, setForm] = useState<HolidayForm | null>(null);
  const formLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = formLoad;

  const load = useCallback(async () => {
    try {
      const loadedForm = await fetchHolidayForm(id);
      setForm(loadedForm);
      clearLoadError();
    } catch (err) {
      logger.error("traeger_holiday_form_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "das Anmeldeformular",
        retry: () => void load(),
      });
    }
  }, [clearLoadError, id, showLoadError]);

  useEffect(() => {
    void load();
  }, [load]);

  const loadError = formLoad.error ?? periodError;

  return (
    <TenantPage
      title={period?.name ?? preview?.name ?? "Ferienzeitraum"}
      back
      backHref={traegerPath("/traeger/ferien")}
      backLabel="Zurück zu den Ferien"
      info={
        <p>
          Hier legen Sie fest, was Eltern bei der Ferienanmeldung zusätzlich
          angeben. Name, Geburtsdatum, Schule, Wochen, Gesundheit, Kontakte und
          Zustimmungen fragt die Anmeldung immer ab. Jedes Speichern legt eine
          neue Version an. Schon abgeschickte Anmeldungen behalten ihre Version.
        </p>
      }
      stats={
        period && form ? (
          <HolidayPeriodStatusLine period={period}>
            <span>
              · Formular Version {form.version}, gespeichert am{" "}
              {formatBerlinDate(form.createdAt)}
            </span>
          </HolidayPeriodStatusLine>
        ) : null
      }
      statsLoading={!(period && form) && !loadError}
      tabs={holidayPeriodTabs(id, "formular", period?.registrationCount)}
      error={loadError}
      loading={!loadError && !(period && form)}
    >
      {period && form ? (
        <HolidayFormEditor
          key={form.version}
          periodId={id}
          form={form}
          onSaved={setForm}
        />
      ) : null}
    </TenantPage>
  );
}
