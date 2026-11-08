"use client";

// Mensa-Abrechnung des Trägers (#4686): je Schule, ob der Monatslauf die
// Mensa je Essen oder als Pauschale je Kind und Monat abrechnet, ab welchem
// Monat und zu welchem Preis. Gezählt wird nur ein Monat, den die
// OGS-Leitung freigegeben hat.

import { ForkKnifeIcon } from "@phosphor-icons/react";
import { useCallback, useEffect, useState } from "react";
import { billingTabs } from "~/components/traeger/billing-family-format";
import { formatMonth, monthOf } from "~/components/traeger/care-billing-format";
import { CareMealPriceModal } from "~/components/traeger/care-meal-price-modal";
import { CareMealSchoolCard } from "~/components/traeger/care-meal-school-card";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { EmptyState } from "~/components/ui/empty-state";
import { TenantPage, TenantPageStats } from "~/components/ui/tenant-page";
import {
  useApiFormError,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
import { berlinTodayISO } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  type CareMealTariff,
  type CareMealTariffs,
  deleteCareMealTariff,
  fetchCareMealTariffs,
  mealTariffOn,
} from "~/lib/traeger-care-meal-api";

const logger = createLogger({ component: "TraegerMensaPage" });

interface SchoolRef {
  readonly id: string;
  readonly name: string;
}

export default function TraegerMensaPage() {
  const currentMonth = monthOf(berlinTodayISO());
  const [data, setData] = useState<CareMealTariffs | null>(null);
  const [editing, setEditing] = useState<SchoolRef | null>(null);
  const [deleting, setDeleting] = useState<
    (SchoolRef & { tariff: CareMealTariff }) | null
  >(null);
  const [busy, setBusy] = useState(false);
  const pageLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = pageLoad;
  const deleteErrors = useApiFormError();
  const { success } = useToast();

  const load = useCallback(async () => {
    try {
      setData(await fetchCareMealTariffs());
      clearLoadError();
    } catch (err) {
      logger.error("traeger_care_meal_tariffs_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "die Mensa-Preise",
        retry: () => void load(),
      });
    }
  }, [clearLoadError, showLoadError]);

  useEffect(() => {
    void load();
  }, [load]);

  const remove = async () => {
    if (!deleting) return;
    setBusy(true);
    deleteErrors.clear();
    try {
      setData(
        await deleteCareMealTariff(deleting.id, deleting.tariff.validFrom),
      );
      success(
        `Der Mensa-Preis ab ${formatMonth(deleting.tariff.validFrom)} ist gelöscht.`,
      );
      setDeleting(null);
    } catch (err) {
      logger.error("care_meal_tariff_delete_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void deleteErrors.show(err, { object: "den Mensa-Preis" });
    } finally {
      setBusy(false);
    }
  };

  const schools = data?.schools ?? [];
  const priced = data
    ? schools.filter((school) =>
        mealTariffOn(data.tariffs, school.id, currentMonth),
      ).length
    : 0;

  return (
    <TenantPage
      title="Abrechnung"
      info="Hier legen Sie fest, wie die Mensa jeder Schule abgerechnet wird: je Essen oder als feste Pauschale je Kind und Monat. Gezählt wird nur ein Monat, den die OGS-Leitung freigegeben hat. Der Monatslauf setzt die Mensa auf die Rechnung des Kindes."
      stats={
        data && schools.length > 0 ? (
          <TenantPageStats
            items={[
              {
                value: schools.length,
                label: schools.length === 1 ? "Schule" : "Schulen",
              },
              { value: priced, label: "mit Mensa-Preis" },
            ]}
          />
        ) : null
      }
      statsLoading={!data && !pageLoad.error}
      tabs={billingTabs("mensa")}
      error={pageLoad.error}
      loading={!pageLoad.error && !data}
      overlays={
        data ? (
          <>
            <CareMealPriceModal
              data={data}
              school={editing}
              currentMonth={currentMonth}
              onClose={() => setEditing(null)}
              onSaved={(next) => {
                setData(next);
                setEditing(null);
              }}
            />
            <ConfirmDeleteModal
              isOpen={deleting !== null}
              onClose={() => {
                setDeleting(null);
                deleteErrors.clear();
              }}
              onConfirm={() => void remove()}
              title="Mensa-Preis löschen?"
              description={
                deleting
                  ? `Der Mensa-Preis von ${deleting.name} ab ${formatMonth(deleting.tariff.validFrom)} fällt weg. Dann gilt wieder der Preis davor.`
                  : ""
              }
              confirmLabel="Löschen"
              loadingLabel="Wird gelöscht …"
              gate={{ mode: "twoStep", firstStepLabel: "Ja, löschen" }}
              loading={busy}
              error={deleteErrors.error}
            />
          </>
        ) : null
      }
    >
      {data && schools.length === 0 ? (
        <EmptyState
          icon={<ForkKnifeIcon className="h-12 w-12" aria-hidden="true" />}
          title="Noch keine Schule."
          description="Mensa-Preise gibt es für die Schulen Ihres Trägers."
        />
      ) : null}
      {data
        ? schools.map((school) => (
            <CareMealSchoolCard
              key={school.id}
              data={data}
              school={school}
              currentMonth={currentMonth}
              onEdit={() => setEditing(school)}
              onDelete={(tariff) => setDeleting({ ...school, tariff })}
            />
          ))
        : null}
    </TenantPage>
  );
}
