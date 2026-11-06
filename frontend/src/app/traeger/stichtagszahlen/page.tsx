"use client";

// Stichtagszahlen des Träger-Portals (#4673): die Zahlen, nach denen moto jede
// Schule im Monat berechnet, mit derselben Zählweise und aus denselben Zeilen
// wie der Bericht des Betreibers. Oben der gewählte Monat für alle Schulen,
// darunter jede Schule, unten alle erfassten Monate. Nur Zahlen, keine Namen.

import { type MouseEvent, useMemo, useState } from "react";
import { CalendarCheck, School } from "lucide-react";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { CustomSelect } from "~/components/ui/custom-select";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import NavigationLink from "~/components/ui/navigation-link";
import { PlanningContextBar } from "~/components/ui/planning-context-bar";
import { StatCard } from "~/components/ui/stat-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { TenantPage, TenantPageStats } from "~/components/ui/tenant-page";
import {
  capturedLate,
  formatKeyDateChange,
  formatKeyDateMonth,
  keyDateChange,
  type KeyDateSchoolRow,
  selectKeyDateMonth,
} from "~/components/traeger/key-date-counts-model";
import { useTraegerKeyDateCounts } from "~/components/traeger/use-traeger-key-date-counts";
import { childrenConcept } from "~/lib/concepts/children";
import { devicesConcept } from "~/lib/concepts/devices";
import { schoolsConcept } from "~/lib/concepts/schools";
import {
  formatBerlinDate,
  formatBerlinTime,
  formatDate,
} from "~/lib/date-helpers";
import type { TraegerKeyDateMonth } from "~/lib/traeger-api";
import { traegerPath } from "~/lib/traeger-url";
import { foldForSearch } from "~/lib/search-fold";

const EMPTY_VALUE = "–";

function schoolPath(id: string): string {
  return traegerPath(`/traeger/schulen/${id}`);
}

function matchesSearch(row: KeyDateSchoolRow, search: string): boolean {
  const needle = foldForSearch(search.trim());
  if (needle === "") return true;
  return foldForSearch(`${row.school.name} ${row.school.city}`).includes(
    needle,
  );
}

/** „12 mehr als im September 2026", ohne Vormonat kein Vergleich. */
function changeHint(
  change: number | null,
  previous: TraegerKeyDateMonth | null,
): string | undefined {
  if (change === null || !previous) return undefined;
  const month = formatKeyDateMonth(previous.period);
  if (change > 0) return `${change} mehr als im ${month}.`;
  if (change < 0) return `${Math.abs(change)} weniger als im ${month}.`;
  return `Genauso viele wie im ${month}.`;
}

/** Zahl einer Schule mit der Veränderung zum Vormonat daneben. */
function FigureWithChange({
  value,
  change,
  previousMonth,
}: Readonly<{
  value: number;
  change: number | null;
  previousMonth: string | null;
}>) {
  return (
    <span className="inline-flex items-baseline gap-2 text-sm tabular-nums">
      <span className="font-semibold text-gray-900">{value}</span>
      {change !== null && previousMonth ? (
        <span className="text-xs text-gray-500">
          {formatKeyDateChange(change)}
          <span className="sr-only"> gegenüber {previousMonth}</span>
        </span>
      ) : null}
    </span>
  );
}

function MissingFigure() {
  return (
    <span className="text-sm text-gray-500">
      {EMPTY_VALUE}
      <span className="sr-only">, keine Zahlen zu diesem Stichtag</span>
    </span>
  );
}

function schoolColumns(
  previous: TraegerKeyDateMonth | null,
): DataTableColumn<KeyDateSchoolRow>[] {
  const previousMonth = previous ? formatKeyDateMonth(previous.period) : null;
  return [
    {
      key: "name",
      header: "Schule",
      render: (row) => (
        <div className="min-w-0">
          <NavigationLink
            href={schoolPath(row.school.id)}
            className="font-medium text-gray-900 hover:underline"
            onClick={(event: MouseEvent<HTMLAnchorElement>) =>
              event.stopPropagation()
            }
          >
            {row.school.name}
          </NavigationLink>
          {row.school.city ? (
            <span className="block text-xs text-gray-500 max-md:hidden">
              {row.school.city}
            </span>
          ) : null}
        </div>
      ),
      sortValue: (row) => row.school.name,
      stacked: "title",
    },
    {
      key: "students",
      header: "Aktive Kinder",
      align: "right",
      render: (row) =>
        row.count ? (
          <FigureWithChange
            value={row.count.activeStudents}
            change={keyDateChange(
              row.count.activeStudents,
              row.previous?.activeStudents,
            )}
            previousMonth={previousMonth}
          />
        ) : (
          <MissingFigure />
        ),
      sortValue: (row) => row.count?.activeStudents ?? null,
    },
    {
      key: "terminals",
      header: "Aktive Terminals",
      align: "right",
      render: (row) =>
        row.count ? (
          <FigureWithChange
            value={row.count.activeTerminals}
            change={keyDateChange(
              row.count.activeTerminals,
              row.previous?.activeTerminals,
            )}
            previousMonth={previousMonth}
          />
        ) : (
          <MissingFigure />
        ),
      sortValue: (row) => row.count?.activeTerminals ?? null,
    },
    {
      key: "recorded",
      header: "Festgehalten",
      align: "right",
      render: (row) =>
        row.count ? (
          <span className="inline-flex flex-wrap items-center justify-end gap-2 text-sm text-gray-700 tabular-nums">
            {`${formatBerlinDate(row.count.recordedAt)}, ${formatBerlinTime(row.count.recordedAt)} Uhr`}
            {capturedLate(row.count) ? (
              <StatusBadge
                tone="gray"
                size="xs"
                label="Nachgeholt"
                title="moto hat die Zahlen erst nach dem Stichtag festgehalten. Sie gelten trotzdem für den Stichtag."
              />
            ) : null}
          </span>
        ) : (
          <span className="text-sm text-gray-500">
            Keine Zahlen zu diesem Stichtag
          </span>
        ),
      sortValue: (row) => row.count?.recordedAt ?? null,
    },
  ];
}

const MONTH_COLUMNS: DataTableColumn<TraegerKeyDateMonth>[] = [
  {
    key: "period",
    header: "Monat",
    render: (month) => (
      <span className="font-medium text-gray-900">
        {formatKeyDateMonth(month.period)}
      </span>
    ),
    sortValue: (month) => month.period,
    stacked: "title",
  },
  {
    key: "keyDate",
    header: "Stichtag",
    render: (month) => (
      <span className="text-sm text-gray-700 tabular-nums">
        {formatDate(month.keyDate)}
      </span>
    ),
    sortValue: (month) => month.keyDate,
    stacked: "meta",
  },
  {
    key: "students",
    header: "Aktive Kinder",
    align: "right",
    render: (month) => (
      <span className="text-sm font-semibold text-gray-900 tabular-nums">
        {month.activeStudents}
      </span>
    ),
    sortValue: (month) => month.activeStudents,
  },
  {
    key: "terminals",
    header: "Aktive Terminals",
    align: "right",
    render: (month) => (
      <span className="text-sm font-semibold text-gray-900 tabular-nums">
        {month.activeTerminals}
      </span>
    ),
    sortValue: (month) => month.activeTerminals,
  },
  {
    key: "schools",
    header: "Schulen",
    align: "right",
    render: (month) => (
      <span className="text-sm text-gray-700 tabular-nums">
        {month.schools}
      </span>
    ),
    sortValue: (month) => month.schools,
  },
];

function MonthFigures({
  month,
  previous,
  schoolCount,
}: Readonly<{
  month: TraegerKeyDateMonth;
  previous: TraegerKeyDateMonth | null;
  schoolCount: number;
}>) {
  const missing = schoolCount - month.schools;
  return (
    <div className="grid grid-cols-1 gap-4 max-sm:gap-3 sm:grid-cols-3">
      <StatCard
        label="Aktive Kinder"
        value={month.activeStudents}
        hint={
          changeHint(
            keyDateChange(month.activeStudents, previous?.activeStudents),
            previous,
          ) ?? "Kinder, die moto am Stichtag betreut hat."
        }
        icon={<MotoConceptIcon concept={childrenConcept} />}
        tone="neutral"
      />
      <StatCard
        label="Aktive Terminals"
        value={month.activeTerminals}
        hint={
          changeHint(
            keyDateChange(month.activeTerminals, previous?.activeTerminals),
            previous,
          ) ?? "Geräte zum Ein- und Auschecken am Stichtag."
        }
        icon={<MotoConceptIcon concept={devicesConcept} />}
        tone="neutral"
      />
      <StatCard
        label="Schulen mit Zahlen"
        value={`${month.schools} von ${schoolCount}`}
        hint={
          missing > 0
            ? `${missing === 1 ? "Eine Schule hat" : `${missing} Schulen haben`} zu diesem Stichtag keine Zahlen.`
            : "Für jede Schule gibt es Zahlen zu diesem Stichtag."
        }
        icon={<MotoConceptIcon concept={schoolsConcept} />}
        tone="neutral"
      />
    </div>
  );
}

export default function TraegerKeyDateCountsPage() {
  const { counts, error } = useTraegerKeyDateCounts();
  const [period, setPeriod] = useState<string | null>(null);
  const [search, setSearch] = useState("");
  const loading = counts === null && !error;
  const schools = counts?.schools ?? [];
  const months = counts?.months ?? [];
  const selection = useMemo(
    () =>
      counts ? selectKeyDateMonth(counts.schools, counts.months, period) : null,
    [counts, period],
  );
  const rows = useMemo(
    () => (selection?.rows ?? []).filter((row) => matchesSearch(row, search)),
    [selection, search],
  );
  const columns = useMemo(
    () => schoolColumns(selection?.previous ?? null),
    [selection],
  );

  const selectedIndex = selection
    ? months.findIndex((month) => month.period === selection.month.period)
    : -1;
  const older = months[selectedIndex + 1];
  const newer = selectedIndex > 0 ? months[selectedIndex - 1] : undefined;

  return (
    <TenantPage
      title="Stichtagszahlen"
      info={
        <>
          Nach diesen Zahlen berechnet moto jede Schule im Monat. moto hält sie
          am Stichtag einmal fest. Danach ändern sie sich nicht mehr. Das
          moto-Team sieht in seinem Bericht genau dieselben Zahlen. Ein Klick
          auf einen Monat unten zeigt seine Zahlen.
        </>
      }
      stats={
        counts ? (
          <TenantPageStats
            items={[
              {
                value: schools.length,
                label: schools.length === 1 ? "Schule" : "Schulen",
              },
              {
                value: months.length,
                label: months.length === 1 ? "Monat erfasst" : "Monate erfasst",
              },
            ]}
          />
        ) : null
      }
      statsLoading={loading}
      error={error}
      empty={
        counts && months.length === 0
          ? {
              title: "Noch keine Stichtagszahlen",
              description:
                "moto hält die Zahlen am Stichtag jedes Monats fest. Danach stehen sie hier.",
              icon: <CalendarCheck className="h-12 w-12" aria-hidden="true" />,
            }
          : null
      }
      search={
        selection && schools.length > 1
          ? {
              value: search,
              onChange: setSearch,
              placeholder: "Schule oder Ort suchen…",
            }
          : undefined
      }
      searchSlot={
        selection ? (
          <PlanningContextBar
            withoutContextRow
            navigationInGroup
            navigationSlot={
              <CustomSelect
                ariaLabel="Monat"
                value={selection.month.period}
                onChange={setPeriod}
                options={months.map((month) => ({
                  value: month.period,
                  label: formatKeyDateMonth(month.period),
                }))}
                className="min-w-0"
              />
            }
            onPrevious={older ? () => setPeriod(older.period) : undefined}
            onNext={newer ? () => setPeriod(newer.period) : undefined}
            previousLabel="Monat davor"
            nextLabel="Monat danach"
            onToday={newer ? () => setPeriod(null) : undefined}
            todayLabel="Neuester Monat"
          />
        ) : undefined
      }
    >
      {selection ? (
        <>
          <MonthFigures
            month={selection.month}
            previous={selection.previous}
            schoolCount={schools.length}
          />
          <DataTable
            heading={{
              title: `Zahlen je Schule am ${formatDate(selection.month.keyDate)}`,
              count: rows.length,
            }}
            columns={columns}
            rows={[...rows]}
            getRowKey={(row) => row.school.id}
            defaultSortKey="name"
            stackedOnMobile
            emptyState={
              <EmptyState
                icon={<School className="h-12 w-12" aria-hidden="true" />}
                title="Keine Schule passt zu Ihrer Suche."
                description="Prüfen Sie den Namen oder suchen Sie nach dem Ort."
              />
            }
          />
          <DataTable
            heading={{ title: "Alle Monate", count: months.length }}
            columns={MONTH_COLUMNS}
            rows={[...months]}
            getRowKey={(month) => month.period}
            activeRowKey={selection.month.period}
            onRowClick={(month) => setPeriod(month.period)}
            defaultSortKey="period"
            defaultSortDirection="desc"
            stackedOnMobile
          />
        </>
      ) : (
        <DataTable
          columns={MONTH_COLUMNS}
          rows={[]}
          getRowKey={(month) => month.period}
          isLoading={loading}
          loadingRowCount={3}
        />
      )}
    </TenantPage>
  );
}
