"use client";

// Ferienzeiträume des Trägers (#4659): Übersicht aller Zeiträume, von hier
// aus anlegen und öffnen.

import { CalendarDays, Plus } from "lucide-react";
import { useRouter } from "next/navigation";
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
import type { HolidayPeriod } from "~/lib/traeger-api";
import { useTraegerHolidayPeriods } from "~/components/traeger/use-traeger-resources";
import { traegerPath } from "~/lib/traeger-url";

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
  const { data, isLoading: loading, loadError } = useTraegerHolidayPeriods();
  const periods = data ?? [];

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
      error={loadError}
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
