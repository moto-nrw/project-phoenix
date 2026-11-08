"use client";

// Mensa-Freigabe (#4686): Am Monatsende prüft die OGS-Leitung, wer in der
// Mensa gegessen hat, und gibt den Monat frei. Ein Essen zählt, wenn das
// Kind an einem Schultag angemeldet und da war. Abgemeldete Tage, Ferien,
// Feiertage und Schließtage zählen nicht. Erst ein freigegebener Monat geht
// in die Abrechnung des Trägers; danach ändert er sich nicht mehr.

import { useCallback, useEffect, useState } from "react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import { ConfirmationModal } from "~/components/ui/modal";
import { SectionCard } from "~/components/ui/section-card";
import { StatCard } from "~/components/ui/stat-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { TenantPage } from "~/components/ui/tenant-page";
import {
  useApiErrorDisplay,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
import { formatDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  type MealBillingChild,
  type MealBillingMonth,
  type MealBillingStatus,
  type MealBillingSummary,
  fetchMealBillingMonth,
  fetchMealBillingMonths,
  releaseMealBillingMonth,
} from "~/lib/meal-billing-api";
import { schoolClassLabel } from "~/lib/school-class-label";

const logger = createLogger({ component: "MealBillingReleasePage" });

const MONTH_NAMES = [
  "Januar",
  "Februar",
  "März",
  "April",
  "Mai",
  "Juni",
  "Juli",
  "August",
  "September",
  "Oktober",
  "November",
  "Dezember",
];

function monthLabel(iso: string): string {
  const [year, month] = iso.split("-");
  return `${MONTH_NAMES[Number(month) - 1] ?? month} ${year}`;
}

function nextMonthStart(iso: string): string {
  const [year, month] = iso.split("-").map(Number);
  const next = new Date(Date.UTC(year ?? 0, month ?? 1, 1));
  return `1. ${MONTH_NAMES[next.getUTCMonth()]} ${next.getUTCFullYear()}`;
}

const STATUS_LABELS: Record<MealBillingStatus, string> = {
  running: "Läuft noch",
  open: "Freigabe fehlt",
  released: "Freigegeben",
};

const STATUS_TONES: Record<MealBillingStatus, "gray" | "orange" | "green"> = {
  running: "gray",
  open: "orange",
  released: "green",
};

function absentDays(child: MealBillingChild): string {
  return child.absentDates
    .map((date) => formatDate(date).slice(0, 6))
    .join(", ");
}

function columns(released: boolean): DataTableColumn<MealBillingChild>[] {
  return [
    {
      key: "child",
      header: "Kind",
      stacked: "title",
      render: (child) => (
        <div className="min-w-0">
          <p className="font-medium text-gray-900">
            {child.lastName}, {child.firstName}
          </p>
          {child.schoolClass ? (
            <p className="mt-0.5 text-xs text-gray-600">
              {schoolClassLabel(child.schoolClass)}
            </p>
          ) : null}
        </div>
      ),
    },
    {
      key: "registered",
      header: "Angemeldet",
      align: "right",
      stacked: "field",
      render: (child) => (
        <span className="text-gray-700 tabular-nums">
          {child.registeredDays} {child.registeredDays === 1 ? "Tag" : "Tage"}
        </span>
      ),
    },
    {
      key: "absent",
      header: "Nicht da",
      stacked: "field",
      render: (child) => {
        const count = child.registeredDays - child.mealDays;
        if (count === 0) {
          return <span className="text-gray-400">–</span>;
        }
        return (
          <span className="text-moto-orange-strong">
            <span className="font-medium tabular-nums">
              {count} {count === 1 ? "Tag" : "Tage"}
            </span>
            {released ? null : (
              <span className="text-gray-600"> · {absentDays(child)}</span>
            )}
          </span>
        );
      },
    },
    {
      key: "meals",
      header: "Essen",
      align: "right",
      stacked: "meta",
      render: (child) => (
        <span className="font-semibold text-gray-900 tabular-nums">
          {child.mealDays}
        </span>
      ),
    },
  ];
}

export default function MealBillingReleasePage() {
  const [months, setMonths] = useState<MealBillingSummary[] | null>(null);
  const [selected, setSelected] = useState("");
  const [month, setMonth] = useState<MealBillingMonth | null>(null);
  const [confirming, setConfirming] = useState(false);
  const [releasing, setReleasing] = useState(false);
  const pageLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = pageLoad;
  const errors = useApiErrorDisplay();
  const { success } = useToast();

  const loadMonths = useCallback(async () => {
    try {
      const list = await fetchMealBillingMonths();
      setMonths(list);
      // Start with the month that ended last; that is the one the
      // OGS-Leitung releases at the end of a month.
      const start =
        list.find((entry) => entry.status !== "running")?.month ??
        list[0]?.month ??
        "";
      setSelected((current) => current || start);
      clearLoadError();
    } catch (err) {
      logger.error("meal_billing_months_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "die Mensa-Monate",
        retry: () => void loadMonths(),
      });
    }
  }, [clearLoadError, showLoadError]);

  const loadMonth = useCallback(
    async (value: string) => {
      setMonth(null);
      try {
        setMonth(await fetchMealBillingMonth(value));
        clearLoadError();
      } catch (err) {
        logger.error("meal_billing_month_load_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        await showLoadError(err, {
          object: "den Mensa-Monat",
          retry: () => void loadMonth(value),
        });
      }
    },
    [clearLoadError, showLoadError],
  );

  useEffect(() => {
    void loadMonths();
  }, [loadMonths]);

  useEffect(() => {
    if (selected) void loadMonth(selected);
  }, [selected, loadMonth]);

  const release = async () => {
    if (!month) return;
    setReleasing(true);
    try {
      const released = await releaseMealBillingMonth(month);
      setMonth(released);
      setMonths(
        (current) =>
          current?.map((entry) =>
            entry.month === released.month
              ? {
                  ...entry,
                  status: "released",
                  releasedAt: released.releasedAt,
                  childCount: released.childCount,
                  mealCount: released.mealCount,
                }
              : entry,
          ) ?? null,
      );
      success(`${monthLabel(released.month)} ist freigegeben.`);
      setConfirming(false);
    } catch (err) {
      logger.error("meal_billing_month_release_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setConfirming(false);
      void errors.show(err, { object: "die Freigabe" });
      void loadMonth(month.month);
    } finally {
      setReleasing(false);
    }
  };

  const released = month?.status === "released";

  return (
    <TenantPage
      title="Mensa-Freigabe"
      back
      backHref="/meal-plan"
      backLabel="Zurück zum Speiseplan"
      info="Prüfen Sie am Monatsende, wer gegessen hat. Ein Essen zählt, wenn das Kind angemeldet und da war. Abgemeldete Tage, Ferien und Feiertage zählen nicht. Mit der Freigabe rechnet der Träger die Mensa ab."
      searchSlot={
        months ? (
          <div className="w-full sm:max-w-xs">
            <CustomSelect
              id="meal-billing-month"
              ariaLabel="Monat"
              value={selected}
              options={months.map((entry) => ({
                value: entry.month,
                label: `${monthLabel(entry.month)} · ${STATUS_LABELS[entry.status]}`,
              }))}
              onChange={setSelected}
            />
          </div>
        ) : undefined
      }
      actions={
        month?.status === "open" ? (
          <Button
            type="button"
            variant="success"
            size="md"
            onClick={() => setConfirming(true)}
          >
            Monat freigeben
          </Button>
        ) : undefined
      }
      error={pageLoad.error}
      loading={!pageLoad.error && !month}
      overlays={
        <ConfirmationModal
          isOpen={confirming}
          onClose={() => setConfirming(false)}
          onConfirm={() => void release()}
          title={month ? `${monthLabel(month.month)} freigeben?` : "Freigeben?"}
          confirmText="Freigeben"
          cancelText="Abbrechen"
          confirmVariant="success"
          isConfirmLoading={releasing}
        >
          {month ? (
            <p className="text-sm text-gray-600">
              {month.childCount} {month.childCount === 1 ? "Kind" : "Kinder"}{" "}
              mit {month.mealCount} Essen. Danach rechnet der Träger den Monat
              ab. Die Freigabe lässt sich nicht mehr ändern.
            </p>
          ) : null}
        </ConfirmationModal>
      }
    >
      {month ? (
        <>
          {month.status === "running" ? (
            <Alert
              type="info"
              message={`Der Monat läuft noch. Sie können ihn ab dem ${nextMonthStart(month.month)} freigeben.`}
            />
          ) : null}
          {month.status === "open" ? (
            <Alert
              type="warning"
              message="Bitte prüfen Sie die Tage, an denen Kinder nicht da waren. Fehlt eine Anwesenheit, tragen Sie sie zuerst nach. Danach geben Sie den Monat frei."
            />
          ) : null}
          {released && month.releasedAt ? (
            <Alert
              type="success"
              message={`Freigegeben am ${formatDate(month.releasedAt)}. Der Träger rechnet diesen Monat ab. Die Zahlen bleiben so.`}
            />
          ) : null}
          <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
            <StatCard
              label="Schultage"
              value={released ? "–" : month.schoolDays}
              hint={
                released
                  ? "Steht nur vor der Freigabe."
                  : "Ohne Ferien, Feiertage und Schließtage."
              }
            />
            <StatCard
              label="Kinder"
              value={month.childCount}
              hint="Mindestens einen Tag angemeldet."
            />
            <StatCard
              label="Essen"
              tone="green"
              value={month.mealCount}
              hint="Angemeldet und da."
            />
            <StatCard
              label="Nicht da"
              tone={month.absenceCount > 0 ? "orange" : "neutral"}
              value={month.absenceCount}
              hint="Angemeldet, aber nicht da. Zählt nicht."
            />
          </div>
          <SectionCard
            title="Kinder"
            titleBadge={
              <StatusBadge
                tone={STATUS_TONES[month.status]}
                label={STATUS_LABELS[month.status]}
              />
            }
            description={`Angemeldete Tage und Essen im ${monthLabel(month.month)}.`}
          >
            {month.children.length === 0 ? (
              <EmptyState
                variant="compact"
                title="Kein Kind angemeldet."
                description="In diesem Monat war kein Kind an einem Schultag zum Mittagessen angemeldet."
              />
            ) : (
              <DataTable
                columns={columns(released)}
                rows={[...month.children]}
                getRowKey={(child) => child.studentId}
                stackedOnMobile
              />
            )}
          </SectionCard>
        </>
      ) : null}
    </TenantPage>
  );
}
