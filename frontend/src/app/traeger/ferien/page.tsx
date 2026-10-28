"use client";

// Ferienzeiträume des Trägers (#4659): Übersicht aller Zeiträume, von hier
// aus anlegen und öffnen.

import { CalendarDays, Plus } from "lucide-react";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import {
  formatDateRange,
  formatWeeks,
  holidayStatusLabel,
} from "~/components/traeger/holiday-period-format";
import { ButtonLink } from "~/components/ui/button";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import { StatusBadge } from "~/components/ui/status-badge";
import { TenantPage, TenantPageStats } from "~/components/ui/tenant-page";
import { useApiLoadError } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import { fetchHolidayPeriods, type HolidayPeriod } from "~/lib/traeger-api";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "TraegerHolidayPeriodsPage" });

const COLUMNS: DataTableColumn<HolidayPeriod>[] = [
  {
    key: "name",
    header: "Ferienzeitraum",
    render: (period) => (
      <span className="font-medium text-gray-900">{period.name}</span>
    ),
    sortValue: (period) => period.name,
    stacked: "title",
  },
  {
    key: "registration",
    header: "Anmeldung",
    render: (period) =>
      formatDateRange(period.registrationOpensOn, period.registrationClosesOn),
    sortValue: (period) => period.registrationOpensOn,
    stacked: "meta",
  },
  {
    key: "weeks",
    header: "Wochen",
    render: (period) => formatWeeks(period),
    sortValue: (period) => period.weeks[0]?.startsOn ?? null,
    stacked: "meta",
  },
  {
    key: "status",
    header: "Status",
    render: (period) => (
      <StatusBadge
        label={holidayStatusLabel(period)}
        tone={period.status === "published" ? "green" : "gray"}
      />
    ),
    sortValue: (period) => period.status,
    stacked: "meta",
  },
];

export default function TraegerHolidayPeriodsPage() {
  const router = useRouter();
  const [periods, setPeriods] = useState<HolidayPeriod[]>([]);
  const [loading, setLoading] = useState(true);
  const periodsLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = periodsLoad;

  const load = useCallback(async () => {
    setLoading(true);
    try {
      setPeriods(await fetchHolidayPeriods());
      clearLoadError();
    } catch (err) {
      logger.error("traeger_holiday_periods_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setPeriods([]);
      await showLoadError(err, {
        object: "die Liste der Ferienzeiträume",
        retry: () => void load(),
      });
    } finally {
      setLoading(false);
    }
  }, [clearLoadError, showLoadError]);

  useEffect(() => {
    void load();
  }, [load]);

  const newHref = traegerPath("/traeger/ferien/neu");

  const published = periods.filter(
    (period) => period.status === "published",
  ).length;

  return (
    <TenantPage
      title="Ferien"
      info="Hier planen Sie die Ferienbetreuung für die Schulen Ihres Trägers. Ein neuer Ferienzeitraum ist zuerst ein Entwurf. Eltern melden ihre Kinder erst an, wenn Sie ihn veröffentlicht haben."
      stats={
        periods.length > 0 ? (
          <TenantPageStats
            items={[
              {
                value: periods.length,
                label:
                  periods.length === 1 ? "Ferienzeitraum" : "Ferienzeiträume",
              },
              { value: published, label: "veröffentlicht" },
            ]}
          />
        ) : null
      }
      statsLoading={loading}
      error={periodsLoad.error}
      actions={
        <ButtonLink href={newHref} variant="primary" size="md">
          <Plus className="h-4 w-4" aria-hidden="true" />
          Ferienzeitraum anlegen
        </ButtonLink>
      }
    >
      <DataTable
        columns={COLUMNS}
        rows={periods}
        getRowKey={(period) => period.id}
        isLoading={loading}
        loadingRowCount={3}
        stackedOnMobile
        stackedLayout="row"
        onRowClick={(period) =>
          router.push(traegerPath(`/traeger/ferien/${period.id}`))
        }
        emptyState={
          <EmptyState
            icon={<CalendarDays className="h-12 w-12" aria-hidden="true" />}
            title="Noch kein Ferienzeitraum."
            description="Legen Sie oben einen Ferienzeitraum an. Danach planen Sie die Wochen."
          />
        }
      />
    </TenantPage>
  );
}
