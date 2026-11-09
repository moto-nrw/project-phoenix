"use client";

// Preisliste der Monatsbeiträge (#4684): je Schule ein Monatspreis für jeden
// gebuchten Wochentag eines Moduls, gültig ab dem 1. eines Monats bis zum
// nächsten Preis. Der Monatslauf rechnet damit ab. Ein Monat, den schon eine
// Rechnung abrechnet, behält seine Preise. Der Filter `?schule=` öffnet die
// Preise einer Schule, zum Beispiel aus „Ohne Preis“ im Monatslauf.

import { Tags } from "lucide-react";
import { useSearchParams } from "next/navigation";
import { Suspense, useCallback, useEffect, useState } from "react";
import {
  billingAreaTitle,
  billingTabs,
} from "~/components/traeger/billing-family-format";
import {
  careSchools,
  formatMonth,
  missingPrices,
  monthOf,
  priceRowsAt,
  priceStands,
  standAt,
} from "~/components/traeger/care-billing-format";
import { CarePriceListCard } from "~/components/traeger/care-price-list-card";
import { CarePriceListModal } from "~/components/traeger/care-price-list-modal";
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
  type CareTariffs,
  deleteCarePriceList,
  fetchCareTariffs,
} from "~/lib/traeger-care-billing-api";

const logger = createLogger({ component: "TraegerPreislistePage" });

const SCHOOL_PARAM = "schule";

interface SchoolRef {
  readonly id: string;
  readonly name: string;
}

export default function TraegerPreislistePage() {
  // `useSearchParams` braucht eine Suspense-Grenze; bis dahin lädt die Seite.
  return (
    <Suspense
      fallback={<TenantPage title={billingAreaTitle("preise")} loading />}
    >
      <PriceListPage />
    </Suspense>
  );
}

function PriceListPage() {
  const searchParams = useSearchParams();
  const focusSchool = searchParams?.get(SCHOOL_PARAM) ?? "";
  const currentMonth = monthOf(berlinTodayISO());
  const [data, setData] = useState<CareTariffs | null>(null);
  const [shown, setShown] = useState<Record<string, string>>({});
  const [editing, setEditing] = useState<SchoolRef | null>(null);
  const [deleting, setDeleting] = useState<
    (SchoolRef & { validFrom: string }) | null
  >(null);
  const [busy, setBusy] = useState(false);
  const pageLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = pageLoad;
  const deleteErrors = useApiFormError();
  const { success } = useToast();

  const load = useCallback(async () => {
    try {
      setData(await fetchCareTariffs());
      clearLoadError();
    } catch (err) {
      logger.error("traeger_care_tariffs_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "die Preisliste",
        retry: () => void load(),
      });
    }
  }, [clearLoadError, showLoadError]);

  useEffect(() => {
    void load();
  }, [load]);

  // Open a school asked for in the address once its data is there.
  useEffect(() => {
    if (!data || !focusSchool) return;
    const school = data.schools.find((entry) => entry.id === focusSchool);
    if (!school) return;
    globalThis.document
      ?.getElementById(`care-prices-${school.id}`)
      ?.scrollIntoView({ block: "start" });
  }, [data, focusSchool]);

  const shownMonth = (schoolId: string) =>
    data
      ? (shown[schoolId] ?? standAt(priceStands(data, schoolId), currentMonth))
      : "";

  const remove = async () => {
    if (!deleting) return;
    setBusy(true);
    deleteErrors.clear();
    try {
      setData(await deleteCarePriceList(deleting.id, deleting.validFrom));
      setShown(({ [deleting.id]: _removed, ...rest }) => rest);
      success(
        `Die Preise ab ${formatMonth(deleting.validFrom)} sind gelöscht.`,
      );
      setDeleting(null);
    } catch (err) {
      logger.error("care_price_list_delete_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void deleteErrors.show(err, { object: "die Preisliste" });
    } finally {
      setBusy(false);
    }
  };

  const schools = data ? careSchools(data) : [];
  const gaps = data
    ? schools.reduce(
        (count, school) =>
          count +
          priceRowsAt(data, school.id, currentMonth).filter(
            (row) => missingPrices(row).length > 0,
          ).length,
        0,
      )
    : 0;
  const modules = data?.modules.length ?? 0;

  return (
    <TenantPage
      title={billingAreaTitle("preise")}
      info="Hier legen Sie die Monatspreise fest. Jede Schule hat eigene Preise, je Modul und gebuchtem Wochentag. Neue Preise gelten ab dem 1. eines Monats. Ferienpreise legen Sie im Ferienzeitraum fest."
      stats={
        data && modules > 0 ? (
          <TenantPageStats
            items={[
              {
                value: schools.length,
                label: schools.length === 1 ? "Schule" : "Schulen",
              },
              { value: modules, label: modules === 1 ? "Modul" : "Module" },
              ...(gaps > 0
                ? [
                    {
                      value: gaps,
                      label:
                        gaps === 1
                          ? "Modul ohne vollen Preis"
                          : "Module ohne vollen Preis",
                    },
                  ]
                : []),
            ]}
          />
        ) : null
      }
      statsLoading={!data && !pageLoad.error}
      tabs={billingTabs("preise")}
      error={pageLoad.error}
      loading={!pageLoad.error && !data}
      overlays={
        data ? (
          <>
            <CarePriceListModal
              data={data}
              school={editing}
              currentMonth={currentMonth}
              initialMonth={editing ? shown[editing.id] : undefined}
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
              title="Preise löschen?"
              description={
                deleting
                  ? `Die Preise von ${deleting.name} ab ${formatMonth(deleting.validFrom)} fallen weg. Dann gelten wieder die Preise davor.`
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
          icon={<Tags className="h-12 w-12" aria-hidden="true" />}
          title="Noch keine Schule."
          description="Preise gibt es für die Schulen Ihres Trägers."
        />
      ) : null}
      {data
        ? schools.map((school) => (
            <div
              key={school.id}
              id={`care-prices-${school.id}`}
              className="scroll-mt-24"
            >
              <CarePriceListCard
                data={data}
                school={school}
                shownMonth={shownMonth(school.id)}
                currentMonth={currentMonth}
                onShownMonthChange={(month) =>
                  setShown((current) => ({ ...current, [school.id]: month }))
                }
                onEdit={() => setEditing(school)}
                onDelete={(validFrom) => setDeleting({ ...school, validFrom })}
              />
            </div>
          ))
        : null}
    </TenantPage>
  );
}
