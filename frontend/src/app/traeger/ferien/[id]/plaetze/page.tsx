"use client";

// Plätze eines Ferienzeitraums (#4699): je Woche, wie viele Plätze es gibt,
// wie viele vergeben sind und wer auf der Warteliste steht. Das Büro legt
// hier die Plätze je Woche fest. Mehr Plätze geben die Warteliste sofort
// frei, in der Reihenfolge der Anmeldung.

import { useParams } from "next/navigation";
import { useCallback, useEffect, useMemo, useState } from "react";
import {
  HolidayHeadFacts,
  holidayPeriodTabs,
} from "~/components/traeger/holiday-period-head";
import { schoolNames } from "~/components/traeger/holiday-registration-format";
import {
  HolidayWeekPlacesCard,
  WaitlistFact,
} from "~/components/traeger/holiday-week-places";
import { useTraegerHolidayPeriod } from "~/components/traeger/use-traeger-resources";
import { useTraegerSchools } from "~/components/traeger/use-traeger-schools";
import { TenantPage } from "~/components/ui/tenant-page";
import { useApiLoadError } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  fetchHolidayWeekPlaces,
  type HolidayWeekPlaces,
} from "~/lib/traeger-holiday-registrations-api";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "TraegerHolidayPlacesPage" });

export default function TraegerHolidayPlacesPage() {
  const params = useParams<{ id: string }>();
  const id = params?.id ?? "";
  const {
    data: period,
    preview,
    loadError: periodError,
  } = useTraegerHolidayPeriod(id);
  const { schools } = useTraegerSchools();
  const names = useMemo(() => schoolNames(schools), [schools]);
  const [weeks, setWeeks] = useState<readonly HolidayWeekPlaces[] | null>(null);
  const [editing, setEditing] = useState<string | null>(null);
  const placesLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = placesLoad;

  const load = useCallback(async () => {
    try {
      setWeeks(await fetchHolidayWeekPlaces(id));
      clearLoadError();
    } catch (err) {
      logger.error("traeger_holiday_places_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "die Plätze",
        retry: () => void load(),
      });
    }
  }, [clearLoadError, id, showLoadError]);

  useEffect(() => {
    void load();
  }, [load]);

  const loadError = placesLoad.error ?? periodError;
  const totals = useMemo(() => {
    const all = weeks ?? [];
    return {
      placed: all.reduce((sum, week) => sum + week.placed, 0),
      pending: all.reduce((sum, week) => sum + (week.pending ?? 0), 0),
      waiting: new Set(
        all.flatMap((week) => week.waitlist.map((entry) => entry.childId)),
      ).size,
      limited: all.filter((week) => week.capacity !== null).length,
    };
  }, [weeks]);

  return (
    <TenantPage
      title={period?.name ?? preview?.name ?? "Ferienzeitraum"}
      back
      backHref={traegerPath("/traeger/ferien")}
      backLabel="Zurück zu den Ferien"
      info={
        period?.allocationMode === "criteria" ? (
          <p>
            Hier legen Sie fest, wie viele Kinder in einer Woche mitmachen
            können. Die Plätze vergibt Ihr Büro im Reiter „Vergabe“. moto
            sortiert die Anmeldungen dafür nach Ihren Kriterien vor. Wird später
            ein Platz frei, rückt das nächste Kind der Warteliste automatisch
            nach. Seine Eltern bekommen eine E-Mail. Kinder auf der Warteliste
            oder mit offenem Platz zahlen nichts und stehen nicht auf der Liste
            der OGS.
          </p>
        ) : (
          <p>
            Hier legen Sie fest, wie viele Kinder in einer Woche mitmachen
            können. Plätze gehen in der Reihenfolge, in der die Anmeldungen
            eingehen. Ist eine Woche voll, kommt das Kind auf die Warteliste.
            Wird ein Platz frei, rückt das nächste Kind automatisch nach. Seine
            Eltern bekommen eine E-Mail. Kinder auf der Warteliste zahlen nichts
            und stehen nicht auf der Liste der OGS.
          </p>
        )
      }
      stats={
        weeks ? (
          <HolidayHeadFacts>
            <span>
              {weeks.length === 1 ? "1 Woche" : `${weeks.length} Wochen`},{" "}
              {totals.limited === 0
                ? "ohne Grenze"
                : `${totals.limited} mit Grenze`}
            </span>
            <span>{totals.placed} Plätze vergeben</span>
            {totals.pending > 0 ? <span>{totals.pending} offen</span> : null}
            <WaitlistFact count={totals.waiting} />
          </HolidayHeadFacts>
        ) : null
      }
      statsLoading={!weeks && !loadError}
      tabs={holidayPeriodTabs(
        id,
        "plaetze",
        period?.registrationCount,
        period?.allocationMode,
      )}
      error={loadError}
      loading={!loadError && !weeks}
      empty={
        weeks?.length === 0
          ? {
              title: "Noch keine Wochen",
              description:
                "Legen Sie zuerst im Reiter „Planung“ die Wochen der Ferien an.",
            }
          : undefined
      }
    >
      {weeks && weeks.length > 0 ? (
        <div className="flex flex-col gap-6">
          {weeks.map((week) => (
            <HolidayWeekPlacesCard
              key={week.weekId}
              periodId={id}
              week={week}
              hostSchoolName={names.get(week.hostSchoolId) ?? "Schule"}
              editing={editing === week.weekId}
              onEdit={() => setEditing(week.weekId)}
              onCancel={() => setEditing(null)}
              onSaved={() => {
                setEditing(null);
                void load();
              }}
            />
          ))}
        </div>
      ) : null}
    </TenantPage>
  );
}
