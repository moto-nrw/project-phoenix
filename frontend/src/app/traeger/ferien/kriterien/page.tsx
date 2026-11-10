"use client";

// Kriterien der Platzvergabe des Trägers (#4700). Sie gelten für jeden
// Ferienzeitraum, der seine Plätze nach Kriterien vergibt. moto sortiert die
// Anmeldungen damit nur vor; das Büro entscheidet.

import { HolidayCriteriaEditor } from "~/components/traeger/holiday-criteria-editor";
import { useTraegerHolidayCriteria } from "~/components/traeger/use-traeger-resources";
import { HolidayHeadFacts } from "~/components/traeger/holiday-period-head";
import { TenantPage } from "~/components/ui/tenant-page";
import { traegerPath } from "~/lib/traeger-url";

export default function TraegerHolidayCriteriaPage() {
  const { data: criteria, loadError, mutate } = useTraegerHolidayCriteria();
  const active = criteria?.filter((criterion) => criterion.active).length ?? 0;

  return (
    <TenantPage
      title="Kriterien der Platzvergabe"
      back
      backHref={traegerPath("/traeger/ferien")}
      backLabel="Zurück zu den Ferien"
      info={
        <p>
          Die Kriterien gelten für alle Ferien, die ihre Plätze nach Kriterien
          vergeben. Eltern beantworten jede Frage mit Ja oder Nein. moto
          sortiert die Anmeldungen damit vor. Das ist nur ein Vorschlag. Ihr
          Büro entscheidet über jeden Platz. Eltern sehen nie die Angaben
          anderer Familien.
        </p>
      }
      stats={
        criteria ? (
          <HolidayHeadFacts>
            <span>
              {criteria.length === 1
                ? "1 Kriterium"
                : `${criteria.length} Kriterien`}
            </span>
            <span>{active} eingeschaltet</span>
          </HolidayHeadFacts>
        ) : null
      }
      statsLoading={!criteria && !loadError}
      error={loadError}
      loading={!loadError && !criteria}
    >
      {criteria ? (
        <HolidayCriteriaEditor
          criteria={criteria}
          onSaved={(saved) => void mutate(saved, { revalidate: false })}
        />
      ) : null}
    </TenantPage>
  );
}
