"use client";

// Zugriffsprotokoll des Träger-Portals (#4655): wer wann an einer Schule des
// Trägers Daten gelesen oder geändert hat. Nur zum Lesen, mit Filtern und
// CSV-Export. Jedes Öffnen steht selbst im Protokoll der gezeigten Schulen.

import { useCallback, useEffect, useMemo, useState } from "react";
import { ScrollText } from "lucide-react";
import type { DateRange } from "react-day-picker";
import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { DateRangePicker } from "~/components/ui/date-range-picker";
import { EmptyState } from "~/components/ui/empty-state";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import {
  SegmentedControl,
  type SegmentedControlItem,
} from "~/components/ui/segmented-control";
import { StatusBadge } from "~/components/ui/status-badge";
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

const KIND_ITEMS: readonly SegmentedControlItem<KindChoice>[] = [
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
      <span className="font-medium text-gray-900">{entry.person}</span>
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
    fetchTraegerAccessLogPeople(from, to)
      .then(setPeople)
      .catch((err: unknown) => {
        // The person filter then offers only "Alle Personen"; the list
        // itself still loads and shows its own error.
        logger.warn("traeger_access_log_people_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        setPeople([]);
      });
  }, [from, to]);

  const load = useCallback(async () => {
    if (!filter) return;
    setLoading(true);
    try {
      const result = await fetchTraegerAccessLog(filter, 1);
      setEntries(result.entries);
      setHasMore(result.hasMore);
      setPage(1);
      clearLoadError();
    } catch (err) {
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
      setLoading(false);
    }
  }, [filter, clearLoadError, showLoadError]);

  useEffect(() => {
    void load();
  }, [load]);

  const loadMore = async () => {
    if (!filter) return;
    setLoadingMore(true);
    try {
      const result = await fetchTraegerAccessLog(filter, page + 1);
      setEntries((current) => [...current, ...result.entries]);
      setHasMore(result.hasMore);
      setPage(page + 1);
    } catch (err) {
      logger.error("traeger_access_log_load_more_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showActionError(err, { object: "die weiteren Einträge" });
    } finally {
      setLoadingMore(false);
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

  return (
    <div className="w-full space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold text-gray-900 sm:text-2xl">
            Zugriffsprotokoll
          </h1>
          <p className="mt-1 text-sm text-gray-500">
            Wer hat an Ihren Schulen Daten gelesen oder geändert?
          </p>
        </div>
        <Button
          type="button"
          variant="outline"
          size="md"
          isLoading={exporting}
          loadingText="Export läuft …"
          disabled={!filter || loading}
          onClick={() => void exportCsv()}
        >
          Als CSV herunterladen
        </Button>
      </div>

      <div className="flex flex-wrap items-center gap-3">
        <DateRangePicker
          value={range}
          onChange={setRange}
          presets={rangePresets()}
          toMax={parseISODate(todayISO())}
          className="w-full sm:w-auto"
          triggerClassName="w-full sm:w-auto"
        />
        <CustomSelect
          value={schoolId}
          options={schoolOptions}
          onChange={setSchoolId}
          ariaLabel="Schule"
          className="w-full sm:w-56"
        />
        <CustomSelect
          value={accountId}
          options={personOptions}
          onChange={setAccountId}
          ariaLabel="Person"
          className="w-full sm:w-56"
        />
        <SegmentedControl
          items={KIND_ITEMS}
          value={kind}
          onChange={setKind}
          ariaLabel="Art des Zugriffs"
        />
      </div>

      {logLoad.error ? (
        <LoadErrorAlert error={logLoad.error} />
      ) : (
        <>
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
        </>
      )}
    </div>
  );
}
