"use client";

// Abwesenheits-Übersicht (#2288): eine zeitraumfilterbare Liste aller
// eingetragenen Abwesenheitstage (Krank / Entschuldigt / Klassenfahrt) über
// alle Kinder der sichtbaren Gruppen. Bewusst nur nach vorn gerichtet (heute
// bis Enddatum): Gruppenzuordnungen sind nicht datiert, vergangene Einträge
// ließen sich nicht sicher der damaligen Gruppe zuordnen — dieselbe
// Einschränkung wie bei der Tagesauswertung.

import { useEffect, useMemo, useState } from "react";
import type { DateRange } from "react-day-picker";
import { Alert } from "~/components/ui/alert";
import { CustomSelect } from "~/components/ui/custom-select";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { DateRangePicker } from "~/components/ui/date-range-picker";
import { EmptyState } from "~/components/ui/empty-state";
import { Input } from "~/components/ui/input";
import { Skeleton } from "~/components/ui/skeleton";
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
  type StatusDayOverviewEntry,
  type StudentStatusKind,
} from "~/lib/student-status-days-api";
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
  // Zwei Monate voraus — derselbe Zeitraum wie der Betreuungsplan auf der
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
    sortValue: (row) => row.date,
  },
  {
    key: "child",
    header: "Kind",
    render: (row) => (
      <span className="font-medium text-gray-900">
        {row.last_name}, {row.first_name}
      </span>
    ),
    sortValue: (row) => `${row.last_name} ${row.first_name}`,
  },
  {
    key: "school_class",
    header: "Klasse",
    render: (row) => row.school_class,
    sortValue: (row) => row.school_class,
  },
  {
    key: "group",
    header: "Gruppe",
    render: (row) => row.group_name,
    sortValue: (row) => row.group_name,
  },
  {
    key: "status",
    header: "Status",
    render: (row) => (
      <StatusDotBadge label={row.label} color={STATUS_COLORS[row.status]} />
    ),
    sortValue: (row) => row.label,
  },
  {
    key: "reported",
    header: "Gemeldet",
    render: (row) => <span className="text-gray-600">{reportedLine(row)}</span>,
  },
  {
    key: "note",
    header: "Notiz",
    render: (row) => <span className="text-gray-600">{row.note ?? ""}</span>,
  },
];

export default function AbsencesPage() {
  const router = useTenantRouter();
  const todayIso = berlinTodayISO();
  const [range, setRange] = useState<DateRange | undefined>(() =>
    defaultRange(berlinTodayISO()),
  );
  const [entries, setEntries] = useState<StatusDayOverviewEntry[] | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [query, setQuery] = useState("");
  const [statusFilter, setStatusFilter] = useState("all");
  const [groupFilter, setGroupFilter] = useState("all");

  // Nur mit vollständigem Zeitraum laden; das Backend akzeptiert kein
  // Startdatum in der Vergangenheit, deshalb wird `from` auf heute geklemmt
  // (relevant für eine über Mitternacht offene Seite).
  const fromIso = range?.from ? toISODate(range.from) : null;
  const toIso = range?.to ? toISODate(range.to) : null;
  const effectiveFromIso = fromIso && fromIso < todayIso ? todayIso : fromIso;

  useEffect(() => {
    if (!effectiveFromIso || !toIso) return;
    let cancelled = false;
    setLoading(true);
    setError(null);
    fetchStatusDayOverview(effectiveFromIso, toIso)
      .then((overview) => {
        if (!cancelled) setEntries(overview.entries);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setEntries(null);
        setError(
          err instanceof StatusDayOverviewForbiddenError
            ? err.message
            : "Abwesenheiten konnten nicht geladen werden.",
        );
        logger.error("absences_fetch_failed", {
          from: effectiveFromIso,
          to: toIso,
          error: err instanceof Error ? err.message : String(err),
        });
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [effectiveFromIso, toIso]);

  const groupOptions = useMemo(() => {
    const byId = new Map<string, string>();
    for (const entry of entries ?? []) {
      byId.set(entry.group_id, entry.group_name);
    }
    const options = [...byId.entries()]
      .sort((a, b) => a[1].localeCompare(b[1], "de"))
      .map(([value, label]) => ({ value, label }));
    return [{ value: "all", label: "Alle Gruppen" }, ...options];
  }, [entries]);

  const filtered = useMemo(() => {
    const needle = query.trim().toLowerCase();
    return (entries ?? []).filter(
      (entry) =>
        (statusFilter === "all" || entry.status === statusFilter) &&
        (groupFilter === "all" || entry.group_id === groupFilter) &&
        (needle === "" ||
          `${entry.first_name} ${entry.last_name} ${entry.school_class}`
            .toLowerCase()
            .includes(needle)),
    );
  }, [entries, query, statusFilter, groupFilter]);

  const minDate = useMemo(() => parseISODate(todayIso), [todayIso]);

  return (
    <div className="w-full">
      <section className="moto-content-surface rounded-2xl border p-5 shadow-sm backdrop-blur-md">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
          <div>
            <p className="text-moto-blue text-xs font-semibold tracking-wide uppercase">
              Abwesenheiten
            </p>
            <h2 className="mt-1 text-base font-semibold text-gray-900">
              Eingetragene Abwesenheitstage
            </h2>
            <p className="mt-1 max-w-2xl text-sm leading-6 text-gray-600">
              Alle gemeldeten Krank-, Entschuldigt- und Klassenfahrt-Tage von
              heute an – zum Nachschlagen, ob für ein Kind schon etwas
              eingetragen ist.
            </p>
          </div>
          <DateRangePicker
            value={range}
            onChange={setRange}
            presets={rangePresets(todayIso)}
            fromMin={minDate}
            className="w-full sm:w-auto"
          />
        </div>

        <div className="mt-4 flex flex-col gap-2 sm:flex-row sm:flex-wrap sm:items-center">
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
            value={groupFilter}
            options={groupOptions}
            onChange={setGroupFilter}
            ariaLabel="Nach Gruppe filtern"
            className="w-full sm:w-52"
          />
        </div>

        {error !== null && (
          <div className="mt-4">
            <Alert type="error" message={error} />
          </div>
        )}

        {loading && entries === null && (
          <div className="mt-4 space-y-3">
            <Skeleton className="h-10 w-full" />
            <Skeleton className="h-64 w-full" />
          </div>
        )}

        {error === null && entries !== null && (
          <div className="mt-4">
            <DataTable
              columns={COLUMNS}
              rows={filtered}
              getRowKey={(row) => row.id}
              onRowClick={(row) =>
                router.push(`/students/${row.student_id}?from=/absences`)
              }
              defaultSortKey="date"
              defaultSortDirection="asc"
              pageSize={50}
              caption={`${filtered.length} ${
                filtered.length === 1 ? "Eintrag" : "Einträge"
              } im gewählten Zeitraum`}
              emptyState={
                <EmptyState
                  title="Keine Abwesenheiten eingetragen"
                  description={
                    entries.length === 0
                      ? "Im gewählten Zeitraum ist für kein Kind eine Abwesenheit eingetragen."
                      : "Kein Eintrag passt zu den gewählten Filtern."
                  }
                />
              }
            />
          </div>
        )}
      </section>
    </div>
  );
}
