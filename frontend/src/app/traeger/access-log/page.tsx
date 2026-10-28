"use client";

// Zugriffsprotokoll des Träger-Portals (#4655): wer wann an einer Schule des
// Trägers Daten gelesen oder geändert hat. Nur zum Lesen, mit Filtern und
// CSV-Export. Jedes Öffnen steht selbst im Protokoll der gezeigten Schulen.

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { ScrollText } from "lucide-react";
import type { DateRange } from "react-day-picker";
import { Button } from "~/components/ui/button";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { DateRangePicker } from "~/components/ui/date-range-picker";
import { EmptyState } from "~/components/ui/empty-state";
import {
  ExportMenu,
  type ExportMenuOption,
  exportFormatOptions,
} from "~/components/ui/export-menu";
import type {
  ActiveFilter,
  FilterConfig,
} from "~/components/ui/page-header/types";
import { StatusBadge } from "~/components/ui/status-badge";
import { TenantPage } from "~/components/ui/tenant-page";
import { useApiErrorDisplay, useApiLoadError } from "~/contexts/ToastContext";
import {
  formatBerlinDate,
  parseISODate,
  todayISO,
  toISODate,
} from "~/lib/date-helpers";
import { downloadBlob } from "~/lib/file-download";
import { createLogger } from "~/lib/logger";
import {
  downloadTraegerAccessLog,
  fetchTraegerAccessLog,
  fetchTraegerAccessLogPeople,
  fetchTraegerSchools,
  type TraegerAccessKind,
  type TraegerAccessLogEntry,
  type TraegerAccessLogFilter,
  type TraegerAccessLogPerson,
  type TraegerSchool,
} from "~/lib/traeger-api";

const logger = createLogger({ component: "TraegerAccessLogPage" });

type KindChoice = TraegerAccessKind | "all";

const EXPORT_FORMATS: readonly ExportMenuOption[] = exportFormatOptions([
  "csv",
]);

const KIND_ITEMS: readonly { value: KindChoice; label: string }[] = [
  { value: "all", label: "Alles" },
  { value: "read", label: "Gelesen" },
  { value: "change", label: "Geändert" },
];

const KIND_LABELS: Readonly<Record<TraegerAccessKind, string>> = {
  read: "Gelesen",
  change: "Geändert",
};

const BERLIN_TIME = new Intl.DateTimeFormat("de-DE", {
  timeZone: "Europe/Berlin",
  hour: "2-digit",
  minute: "2-digit",
});

const COLUMNS: DataTableColumn<TraegerAccessLogEntry>[] = [
  {
    key: "occurredAt",
    header: "Zeitpunkt",
    render: (entry) => (
      <span className="whitespace-nowrap text-gray-900">
        {formatBerlinDate(entry.occurredAt)},{" "}
        {BERLIN_TIME.format(new Date(entry.occurredAt))} Uhr
      </span>
    ),
    stacked: "meta",
  },
  {
    key: "person",
    header: "Person",
    render: (entry) => (
      // A Träger-Büro address can be long; it wraps on a phone instead of
      // running into the time beside it.
      <span className="font-medium wrap-anywhere text-gray-900">
        {entry.person}
      </span>
    ),
    stacked: "title",
  },
  {
    key: "school",
    header: "Schule",
    render: (entry) => entry.schoolName || "–",
    stacked: "field",
  },
  {
    key: "kind",
    header: "Art",
    render: (entry) => (
      <StatusBadge
        tone={entry.kind === "change" ? "orange" : "blue"}
        label={KIND_LABELS[entry.kind]}
      />
    ),
    stacked: "field",
  },
  {
    key: "data",
    header: "Daten",
    render: (entry) => entry.dataLabel,
    stacked: "field",
  },
];

/** The last 30 days up to today, as the page opens. */
function lastThirtyDays(): DateRange {
  const today = parseISODate(todayISO());
  return {
    from: new Date(today.getFullYear(), today.getMonth(), today.getDate() - 29),
    to: today,
  };
}

function rangePresets() {
  const today = parseISODate(todayISO());
  const daysBack = (days: number) =>
    new Date(today.getFullYear(), today.getMonth(), today.getDate() - days);
  return [
    { label: "Heute", range: () => ({ from: today, to: today }) },
    { label: "Letzte 7 Tage", range: () => ({ from: daysBack(6), to: today }) },
    { label: "Letzte 30 Tage", range: lastThirtyDays },
    {
      label: "Letztes Jahr",
      range: () => ({ from: daysBack(365), to: today }),
    },
  ];
}

export default function TraegerAccessLogPage() {
  const [range, setRange] = useState<DateRange | undefined>(lastThirtyDays);
  const [schoolId, setSchoolId] = useState("");
  const [accountId, setAccountId] = useState("");
  const [kind, setKind] = useState<KindChoice>("all");
  const [schools, setSchools] = useState<TraegerSchool[]>([]);
  const [people, setPeople] = useState<TraegerAccessLogPerson[]>([]);
  const [entries, setEntries] = useState<TraegerAccessLogEntry[]>([]);
  const [page, setPage] = useState(1);
  const [asOf, setAsOf] = useState("");
  // Each new filter starts a new list. An answer for an older list (a slow
  // first page, a pending "Weitere Einträge laden") is dropped.
  const listGeneration = useRef(0);
  const [hasMore, setHasMore] = useState(false);
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [exporting, setExporting] = useState(false);
  const logLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = logLoad;
  const { show: showActionError } = useApiErrorDisplay();

  const from = range?.from ? toISODate(range.from) : "";
  const to = range?.to ? toISODate(range.to) : from;

  const filter = useMemo<TraegerAccessLogFilter | null>(
    () =>
      from
        ? { from, to, schoolId, accountId, kind: kind === "all" ? "" : kind }
        : null,
    [from, to, schoolId, accountId, kind],
  );

  useEffect(() => {
    fetchTraegerSchools()
      .then(setSchools)
      .catch((err: unknown) => {
        // The school filter then offers only "Alle Schulen"; the list
        // itself still loads and shows its own error.
        logger.warn("traeger_access_log_schools_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
      });
  }, []);

  useEffect(() => {
    if (!from) return;
    let current = true;
    fetchTraegerAccessLogPeople(from, to)
      .then((loaded) => {
        if (!current) return;
        setPeople(loaded);
        // A person who does not act in the new window can no longer be
        // chosen; the filter would stay active without a visible choice.
        setAccountId((chosen) =>
          loaded.some((person) => person.accountId === chosen) ? chosen : "",
        );
      })
      .catch((err: unknown) => {
        // The person filter then offers only "Alle Personen"; the list
        // itself still loads and shows its own error.
        logger.warn("traeger_access_log_people_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        if (!current) return;
        setPeople([]);
        setAccountId("");
      });
    return () => {
      current = false;
    };
  }, [from, to]);

  const load = useCallback(async () => {
    if (!filter) return;
    const generation = ++listGeneration.current;
    setLoading(true);
    setLoadingMore(false);
    try {
      const result = await fetchTraegerAccessLog(filter, 1);
      if (generation !== listGeneration.current) return;
      setEntries(result.entries);
      setHasMore(result.hasMore);
      setAsOf(result.asOf);
      setPage(1);
      clearLoadError();
    } catch (err) {
      if (generation !== listGeneration.current) return;
      logger.error("traeger_access_log_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setEntries([]);
      setHasMore(false);
      await showLoadError(err, {
        object: "das Zugriffsprotokoll",
        retry: () => void load(),
      });
    } finally {
      if (generation === listGeneration.current) setLoading(false);
    }
  }, [filter, clearLoadError, showLoadError]);

  useEffect(() => {
    void load();
  }, [load]);

  const loadMore = async () => {
    if (!filter) return;
    const generation = listGeneration.current;
    setLoadingMore(true);
    try {
      const result = await fetchTraegerAccessLog(filter, page + 1, asOf);
      if (generation !== listGeneration.current) return;
      setEntries((current) => [...current, ...result.entries]);
      setHasMore(result.hasMore);
      setPage(page + 1);
    } catch (err) {
      if (generation !== listGeneration.current) return;
      logger.error("traeger_access_log_load_more_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showActionError(err, { object: "die weiteren Einträge" });
    } finally {
      if (generation === listGeneration.current) setLoadingMore(false);
    }
  };

  const exportCsv = async () => {
    if (!filter) return;
    setExporting(true);
    try {
      const file = await downloadTraegerAccessLog(filter);
      downloadBlob(
        file.blob,
        file.filename ?? `zugriffsprotokoll-${from}-bis-${to}.csv`,
      );
    } catch (err) {
      logger.error("traeger_access_log_export_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showActionError(err, { object: "die Exportdatei" });
    } finally {
      setExporting(false);
    }
  };

  const schoolOptions = [
    { value: "", label: "Alle Schulen" },
    ...schools.map((school) => ({ value: school.id, label: school.name })),
  ];
  const personOptions = [
    { value: "", label: "Alle Personen" },
    ...people.map((person) => ({
      value: person.accountId,
      label: person.name,
    })),
  ];
  const filtered = Boolean(schoolId || accountId || kind !== "all");

  const filters: FilterConfig[] = [
    {
      id: "school",
      label: "Schule",
      type: "dropdown",
      value: schoolId,
      onChange: (value) => setSchoolId(value as string),
      options: schoolOptions,
    },
    {
      id: "person",
      label: "Person",
      type: "dropdown",
      value: accountId,
      onChange: (value) => setAccountId(value as string),
      options: personOptions,
    },
    {
      id: "kind",
      label: "Art des Zugriffs",
      type: "buttons",
      value: kind,
      onChange: (value) => setKind(value as KindChoice),
      options: [...KIND_ITEMS],
    },
  ];
  const activeFilters: ActiveFilter[] = [
    ...(schoolId
      ? [
          {
            id: "school",
            label:
              schoolOptions.find((option) => option.value === schoolId)
                ?.label ?? "Schule",
            onRemove: () => setSchoolId(""),
          },
        ]
      : []),
    ...(accountId
      ? [
          {
            id: "person",
            label:
              personOptions.find((option) => option.value === accountId)
                ?.label ?? "Person",
            onRemove: () => setAccountId(""),
          },
        ]
      : []),
    ...(kind !== "all"
      ? [
          {
            id: "kind",
            label: KIND_LABELS[kind],
            onRemove: () => setKind("all"),
          },
        ]
      : []),
  ];

  return (
    <TenantPage
      title="Zugriffsprotokoll"
      info="Wer hat an Ihren Schulen Daten gelesen oder geändert? Hier steht jeder Zugriff mit Zeitpunkt, Person und Schule."
      actions={
        <>
          <DateRangePicker
            value={range}
            onChange={setRange}
            presets={rangePresets()}
            toMax={parseISODate(todayISO())}
            className="w-full sm:w-auto"
            triggerClassName="w-full sm:w-auto"
          />
          <ExportMenu
            ariaLabel="Zugriffsprotokoll herunterladen"
            options={EXPORT_FORMATS}
            busyKey={exporting ? "csv" : null}
            unavailable={!filter || loading}
            onSelect={() => void exportCsv()}
          />
        </>
      }
      filters={filters}
      activeFilters={activeFilters}
      onClearAllFilters={() => {
        setSchoolId("");
        setAccountId("");
        setKind("all");
      }}
      error={logLoad.error}
    >
      <DataTable
        columns={COLUMNS}
        rows={entries}
        getRowKey={(entry) => entry.id}
        isLoading={loading}
        loadingRowCount={5}
        stackedOnMobile
        emptyState={
          <EmptyState
            icon={<ScrollText className="h-12 w-12" aria-hidden="true" />}
            title={
              filtered
                ? "Zu diesen Filtern gibt es keinen Eintrag."
                : "In diesem Zeitraum gibt es keinen Eintrag."
            }
            description="Wählen Sie einen anderen Zeitraum oder andere Filter."
          />
        }
      />
      {hasMore ? (
        <div className="flex justify-center">
          <Button
            type="button"
            variant="outline"
            size="md"
            isLoading={loadingMore}
            loadingText="Lädt …"
            onClick={() => void loadMore()}
          >
            Weitere Einträge laden
          </Button>
        </div>
      ) : null}
    </TenantPage>
  );
}
