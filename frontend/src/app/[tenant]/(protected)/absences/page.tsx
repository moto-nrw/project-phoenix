"use client";

// Abwesenheits-Übersicht (#2288): eine zeitraumfilterbare Liste aller
// eingetragenen Abwesenheitstage (Krank / Entschuldigt / Klassenfahrt) über
// alle Kinder der sichtbaren Gruppen. Bewusst nur nach vorn gerichtet (heute
// bis Enddatum): Gruppenzuordnungen sind nicht datiert, vergangene Einträge
// ließen sich nicht sicher der damaligen Gruppe zuordnen, dieselbe
// Einschränkung wie bei der Tagesauswertung.

import { useDeferredValue, useEffect, useMemo, useState } from "react";
import type { DateRange } from "react-day-picker";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { DateRangePicker } from "~/components/ui/date-range-picker";
import { EmptyState } from "~/components/ui/empty-state";
import { Input } from "~/components/ui/input";
import { PageIntro } from "~/components/ui/page-intro";
import { SectionCard } from "~/components/ui/section-card";
import { StatusDotBadge } from "~/components/ui/status-dot-badge";
import {
  berlinTodayISO,
  formatDate,
  parseISODate,
  toISODate,
} from "~/lib/date-helpers";
import { dayLogSourceLabel } from "~/lib/day-log-api";
import { LOCATION_COLORS } from "~/lib/location-helper";
import { createLogger } from "~/lib/logger";
import {
  fetchStatusDayOverview,
  StatusDayOverviewForbiddenError,
  type StatusDayOverview,
  type StatusDayOverviewEntry,
  type StudentStatusKind,
} from "~/lib/student-status-days-api";
import { useSWRAuth } from "~/lib/swr";
import { useTenantRouter } from "~/lib/tenant-router";

const logger = createLogger({ component: "AbsencesPage" });

const STATUS_COLORS: Record<StudentStatusKind, string> = {
  sick: LOCATION_COLORS.SICK,
  excused: LOCATION_COLORS.EXCUSED,
  class_trip: LOCATION_COLORS.CLASS_TRIP,
};

const STATUS_FILTER_OPTIONS = [
  { value: "all", label: "Alle Status" },
  { value: "sick", label: "Krank" },
  { value: "excused", label: "Entschuldigt" },
  { value: "class_trip", label: "Klassenfahrt" },
] as const;

function defaultRange(todayIso: string): DateRange {
  const from = parseISODate(todayIso);
  // Zwei Monate voraus, derselbe Zeitraum wie der Betreuungsplan auf der
  // Kind-Detailseite.
  const to = new Date(from.getFullYear(), from.getMonth() + 2, from.getDate());
  return { from, to };
}

function rangePresets(todayIso: string) {
  const today = parseISODate(todayIso);
  const plusDays = (days: number) =>
    new Date(today.getFullYear(), today.getMonth(), today.getDate() + days);
  return [
    {
      label: "Nächste 7 Tage",
      range: () => ({ from: today, to: plusDays(7) }),
    },
    {
      label: "Nächste 30 Tage",
      range: () => ({ from: today, to: plusDays(30) }),
    },
    { label: "Nächste 2 Monate", range: () => defaultRange(todayIso) },
  ];
}

function reportedLine(entry: StatusDayOverviewEntry): string {
  const reported = formatDate(entry.reported_at);
  const source = dayLogSourceLabel(entry.source);
  return source ? `${reported} · ${source}` : reported;
}

const COLUMNS: DataTableColumn<StatusDayOverviewEntry>[] = [
  {
    key: "date",
    header: "Datum",
    render: (row) => formatDate(row.date, true),
  },
  {
    key: "child",
    header: "Kind",
    render: (row) => (
      <span className="font-medium text-gray-900">
        {row.last_name}, {row.first_name}
      </span>
    ),
  },
  {
    key: "status",
    header: "Status",
    render: (row) => (
      <StatusDotBadge label={row.label} color={STATUS_COLORS[row.status]} />
    ),
  },
  {
    key: "school_class",
    header: "Klasse",
    render: (row) => row.school_class,
  },
  // Gruppe und Meldedatum sind sekundär; auf Phone-Breite ausgeblendet,
  // damit Datum, Kind und Status ohne horizontales Scrollen sichtbar sind.
  {
    key: "group",
    header: "Gruppe",
    render: (row) => row.group_name,
    className: "hidden sm:table-cell",
    headerClassName: "hidden sm:table-cell",
  },
  {
    key: "reported",
    header: "Gemeldet",
    render: (row) => <span className="text-gray-600">{reportedLine(row)}</span>,
    className: "hidden sm:table-cell",
    headerClassName: "hidden sm:table-cell",
  },
];

export default function AbsencesPage() {
  const router = useTenantRouter();
  const todayIso = berlinTodayISO();
  const [range, setRange] = useState<DateRange | undefined>(() =>
    defaultRange(berlinTodayISO()),
  );
  const [query, setQuery] = useState("");
  const [statusFilter, setStatusFilter] = useState("all");
  const [groupFilter, setGroupFilter] = useState("all");
  const [page, setPage] = useState(1);
  const deferredQuery = useDeferredValue(query);

  // Nur mit vollständigem Zeitraum laden; das Backend akzeptiert kein
  // Startdatum in der Vergangenheit, deshalb wird `from` auf heute geklemmt
  // (relevant für eine über Mitternacht offene Seite).
  const fromIso = range?.from ? toISODate(range.from) : null;
  const toIso = range?.to ? toISODate(range.to) : null;
  const effectiveFromIso = fromIso && fromIso < todayIso ? todayIso : fromIso;
  const effectiveToIso =
    toIso && effectiveFromIso && toIso < effectiveFromIso
      ? effectiveFromIso
      : toIso;

  useEffect(() => {
    if (!range?.from || fromIso === effectiveFromIso) return;
    const from = parseISODate(effectiveFromIso!);
    setRange({
      from,
      to: effectiveToIso ? parseISODate(effectiveToIso) : from,
    });
  }, [effectiveFromIso, effectiveToIso, fromIso, range?.from]);

  const {
    data: overview,
    isLoading,
    isValidating,
    error: swrError,
  } = useSWRAuth<StatusDayOverview>(
    effectiveFromIso && effectiveToIso
      ? `student-status-days-overview-${effectiveFromIso}-${effectiveToIso}-${page}-${deferredQuery}-${statusFilter}-${groupFilter}`
      : null,
    () =>
      fetchStatusDayOverview(effectiveFromIso!, effectiveToIso!, {
        page,
        query: deferredQuery,
        status: statusFilter,
        groupId: groupFilter,
      }),
    { keepPreviousData: true },
  );
  const entries = overview?.entries ?? null;
  const displayedPage = overview?.page ?? page;
  const hasMore = overview?.has_more ?? false;
  const error = swrError
    ? swrError instanceof StatusDayOverviewForbiddenError
      ? swrError.message
      : "Abwesenheiten konnten nicht geladen werden."
    : null;

  useEffect(() => {
    if (!swrError) return;
    logger.error("absences_fetch_failed", {
      from: effectiveFromIso,
      to: effectiveToIso,
      error: swrError instanceof Error ? swrError.message : String(swrError),
    });
  }, [effectiveFromIso, effectiveToIso, swrError]);

  const groupOptions = useMemo(() => {
    const options = [...(overview?.groups ?? [])]
      .sort((a, b) => a.name.localeCompare(b.name, "de"))
      .map((group) => ({ value: group.id, label: group.name }));
    return [{ value: "all", label: "Alle Gruppen" }, ...options];
  }, [overview?.groups]);

  const effectiveGroupFilter = groupOptions.some(
    (option) => option.value === groupFilter,
  )
    ? groupFilter
    : "all";
  const hasActiveFilters =
    deferredQuery.trim() !== "" ||
    statusFilter !== "all" ||
    effectiveGroupFilter !== "all";

  useEffect(() => {
    if (groupFilter !== effectiveGroupFilter) {
      setGroupFilter(effectiveGroupFilter);
    }
  }, [effectiveGroupFilter, groupFilter]);

  useEffect(() => {
    setPage(1);
  }, [
    effectiveFromIso,
    effectiveToIso,
    deferredQuery,
    statusFilter,
    effectiveGroupFilter,
  ]);

  const minDate = useMemo(() => parseISODate(todayIso), [todayIso]);
  const maxDate = useMemo(() => {
    const today = parseISODate(todayIso);
    return new Date(
      today.getFullYear(),
      today.getMonth(),
      today.getDate() + 365,
    );
  }, [todayIso]);

  return (
    <div className="w-full space-y-6">
      {/* Kopfkarte auf allen Breakpoints, wie in der Eltern-App: Zeitraum
          rechts im Kopf, Such- und Filterzeile darunter in derselben Karte. */}
      <PageIntro
        title="Abwesenheiten"
        description="Alle gemeldeten Krank-, Entschuldigt- und Klassenfahrt-Tage von heute an, zum Nachschlagen, ob für ein Kind schon etwas eingetragen ist."
        actions={
          <DateRangePicker
            value={range}
            onChange={setRange}
            presets={rangePresets(todayIso)}
            fromMin={minDate}
            toMax={maxDate}
            className="w-full sm:w-auto"
          />
        }
      >
        <div className="flex flex-col gap-2 sm:flex-row sm:flex-wrap sm:items-center">
          <Input
            type="search"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Nach Kind oder Klasse suchen…"
            aria-label="Nach Kind oder Klasse suchen"
            className="w-full sm:w-64"
          />
          <CustomSelect
            value={statusFilter}
            options={STATUS_FILTER_OPTIONS}
            onChange={setStatusFilter}
            ariaLabel="Nach Status filtern"
            className="w-full sm:w-44"
          />
          <CustomSelect
            value={effectiveGroupFilter}
            options={groupOptions}
            onChange={setGroupFilter}
            ariaLabel="Nach Gruppe filtern"
            className="w-full sm:w-52"
          />
          {entries && (
            <span className="text-sm text-gray-500 sm:ml-auto">
              {entries.length} {entries.length === 1 ? "Eintrag" : "Einträge"}{" "}
              auf Seite {displayedPage}
            </span>
          )}
        </div>
      </PageIntro>

      <SectionCard title="Eingetragene Abwesenheitstage" bodyClassName="">
        {error !== null && (
          <div
            className={`mt-4 transition-opacity ${isValidating ? "opacity-60" : ""}`}
            aria-busy={isValidating}
          >
            <Alert type="error" message={error} />
          </div>
        )}

        {error === null && (
          <div className="mt-4">
            <DataTable
              columns={COLUMNS}
              rows={entries ?? []}
              isLoading={isLoading && entries === null}
              getRowKey={(row) => row.id}
              onRowClick={(row) =>
                router.push(`/students/${row.student_id}?from=/absences`)
              }
              emptyState={
                <EmptyState
                  title="Keine Abwesenheiten eingetragen"
                  description={
                    hasActiveFilters
                      ? "Kein Eintrag passt zu den gewählten Filtern."
                      : "Im gewählten Zeitraum ist für kein Kind eine Abwesenheit eingetragen."
                  }
                />
              }
            />
            {entries && (displayedPage > 1 || hasMore) && (
              <div className="mt-3 flex justify-end gap-2">
                <Button
                  type="button"
                  variant="outline"
                  size="md"
                  disabled={displayedPage === 1 || isValidating}
                  onClick={() => setPage(Math.max(1, displayedPage - 1))}
                >
                  Zurück
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  size="md"
                  disabled={!hasMore || isValidating}
                  onClick={() => setPage(displayedPage + 1)}
                >
                  Weiter
                </Button>
              </div>
            )}
          </div>
        )}
      </SectionCard>
    </div>
  );
}
