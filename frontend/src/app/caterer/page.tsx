"use client";

// Overview of the caterer portal (ADR 0077, #3873): one table of every
// school the caterer supplies, the days of the chosen period as columns,
// with a sum per school and per day. Each school opens to its days with
// classes, names (if the school shows them) and the confirmation. Each
// school is read on its own; a school that ends the supply drops out.

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Download, FileSpreadsheet, Printer } from "lucide-react";
import { CatererSchoolOrders } from "~/components/caterer/caterer-school-orders";
import { Button } from "~/components/ui/button";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { Loading } from "~/components/ui/loading";
import { SectionCard } from "~/components/ui/section-card";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { StatusBadge } from "~/components/ui/status-badge";
import { useApiErrorDisplay, useApiLoadError } from "~/contexts/ToastContext";
import { apiErrorFromResponse } from "~/lib/api-error";
import {
  downloadCatererDeliveries,
  fetchCatererOrders,
  fetchCatererSchools,
  type CatererOrderDay,
  type CatererOrders,
  type CatererSchool,
  type CatererSchools,
  type DeliveryExportFormat,
} from "~/lib/caterer-api";
import {
  defaultPeriod,
  periodDays,
  PERIODS,
  type Period,
} from "~/lib/caterer-period";
import { berlinTodayISO, parseISODate } from "~/lib/date-helpers";
import { openBlobForPrint } from "~/lib/file-download";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "CatererOverview" });

function shortDay(iso: string): string {
  return parseISODate(iso).toLocaleDateString("de-DE", {
    weekday: "short",
    day: "2-digit",
    month: "2-digit",
  });
}

function berlinDateTime(instant: string): string {
  return new Date(instant).toLocaleString("de-DE", {
    weekday: "short",
    day: "2-digit",
    month: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    timeZone: "Europe/Berlin",
  });
}

/** The children added and removed after the deadline, each counted. */
function changeCounts(day: CatererOrderDay): {
  added: number;
  removed: number;
} {
  let added = 0;
  let removed = 0;
  for (const change of day.changes) {
    if (change.delta > 0) added += change.delta;
    else removed -= change.delta;
  }
  return { added, removed };
}

function counts(day: CatererOrderDay | undefined): number {
  if (!day || day.closed || day.status === "unordered") return 0;
  return day.total;
}

/** Where a school's orders of the period stand, in one badge. */
function SchoolState({ orders }: Readonly<{ orders: CatererOrders }>) {
  const live = orders.days.filter(
    (day) => !day.closed && day.status !== "unordered",
  );
  const open = live.filter((day) => day.status === "open");
  if (open.length > 0) {
    const first = open.reduce((a, b) => (a.deadline < b.deadline ? a : b));
    return (
      <StatusBadge
        label={`Offen bis ${berlinDateTime(first.deadline)}`}
        tone="gray"
        compact
      />
    );
  }
  if (live.length === 0) return null;
  return live.every((day) => day.status === "confirmed") ? (
    <StatusBadge label="Bestätigt" tone="green" compact />
  ) : (
    <StatusBadge label="Bestellt" tone="blue" compact />
  );
}

interface OverviewRow {
  key: string;
  school: CatererSchool | null;
  orders: CatererOrders | null;
  failed: boolean;
}

export default function CatererOverviewPage() {
  const today = useMemo(() => berlinTodayISO(), []);
  const [period, setPeriod] = useState<Period>(() => defaultPeriod(today));
  const days = useMemo(() => periodDays(period, today), [period, today]);
  const from = days[0]!;
  const to = days[days.length - 1]!;
  const [schools, setSchools] = useState<CatererSchools | null>(null);
  const [orders, setOrders] = useState<Record<string, CatererOrders | null>>(
    {},
  );
  const [loading, setLoading] = useState(true);
  const [exporting, setExporting] = useState<string | null>(null);
  const { error, show, clear } = useApiLoadError();
  const { show: showActionError } = useApiErrorDisplay();
  const reloadRef = useRef<() => void>(() => undefined);

  const generationRef = useRef(0);
  const requestRef = useRef(0);
  const loadOrders = useCallback(
    async (
      list: readonly CatererSchool[],
      generation = generationRef.current,
    ) => {
      if (generation !== generationRef.current) return;
      // Only the newest request lands: a slow answer for the previous
      // period never overwrites the one now chosen.
      const request = ++requestRef.current;
      const results = await Promise.allSettled(
        list.map((school) => fetchCatererOrders(school.id, from, to)),
      );
      if (
        generation !== generationRef.current ||
        request !== requestRef.current
      )
        return;
      const next: Record<string, CatererOrders | null> = {};
      results.forEach((result, index) => {
        const school = list[index]!;
        if (result.status === "fulfilled") {
          next[school.id] = result.value;
        } else {
          next[school.id] = null;
          logger.warn("caterer_orders_load_failed", {
            school_id: school.id,
            error:
              result.reason instanceof Error
                ? result.reason.message
                : String(result.reason),
          });
        }
      });
      setOrders(next);
    },
    [from, to],
  );

  const load = useCallback(async () => {
    // Cover the school lookup and its order requests with one generation.
    const generation = ++generationRef.current;
    setLoading(true);
    clear();
    try {
      const loaded = await fetchCatererSchools();
      if (generation !== generationRef.current) return;
      setSchools(loaded);
      await loadOrders(loaded.schools, generation);
    } catch (cause) {
      // Superseded loads must not show errors for the current period.
      if (generation !== generationRef.current) return;
      logger.error("caterer_schools_load_failed", {
        error: cause instanceof Error ? cause.message : String(cause),
      });
      void show(cause, {
        object: "Ihre Schulen",
        retry: () => reloadRef.current(),
      });
    } finally {
      if (generation === generationRef.current) setLoading(false);
    }
  }, [clear, loadOrders, show]);
  useEffect(() => {
    reloadRef.current = () => void load();
  });
  useEffect(() => {
    void load();
  }, [load]);

  async function download(format: DeliveryExportFormat) {
    setExporting(format);
    try {
      await downloadCatererDeliveries(from, to, format);
    } catch (cause) {
      void showActionError(cause, { object: "die Lieferliste" });
    } finally {
      setExporting(null);
    }
  }

  async function print() {
    // Opened inside the click: a tab opened after the download is blocked.
    const target = window.open("", "_blank");
    setExporting("print");
    try {
      const params = new URLSearchParams({ from, to, format: "pdf" });
      const response = await fetch(
        `/api/caterer/orders/export?${params.toString()}`,
      );
      if (!response.ok) {
        throw await apiErrorFromResponse(response, "caterer print failed");
      }
      openBlobForPrint(await response.blob(), target);
    } catch (cause) {
      target?.close();
      void showActionError(cause, { object: "die Druckansicht" });
    } finally {
      setExporting(null);
    }
  }

  const list = schools?.schools ?? [];
  const rows: OverviewRow[] = [
    ...list.map((school) => ({
      key: school.id,
      school,
      orders: orders[school.id] ?? null,
      failed: orders[school.id] === null,
    })),
    ...(list.length > 1
      ? [{ key: "sum", school: null, orders: null, failed: false }]
      : []),
  ];
  const dayOf = (row: OverviewRow, date: string) =>
    row.orders?.days.find((day) => day.date === date);
  const daySum = (date: string) =>
    list.reduce(
      (sum, school) =>
        sum + counts(orders[school.id]?.days.find((day) => day.date === date)),
      0,
    );

  const columns: DataTableColumn<OverviewRow>[] = [
    {
      key: "school",
      header: "Schule",
      stacked: "title",
      render: (row) =>
        row.school ? (
          <div className="flex flex-col items-start gap-1">
            <span className="font-medium text-gray-900">{row.school.name}</span>
            {row.orders ? <SchoolState orders={row.orders} /> : null}
            {row.failed ? (
              <span className="text-sm text-gray-600">
                Bestellung nicht geladen
              </span>
            ) : null}
          </div>
        ) : (
          <span className="font-semibold text-gray-900">Summe</span>
        ),
    },
    ...days.map((date) => ({
      key: date,
      header: shortDay(date),
      align: "right" as const,
      render: (row: OverviewRow) => {
        if (!row.school)
          return <span className="font-semibold">{daySum(date)}</span>;
        const day = dayOf(row, date);
        if (!day || day.closed || day.status === "unordered") return "–";
        const { added, removed } = changeCounts(day);
        return (
          <span className="inline-flex items-center justify-end gap-1">
            {day.total}
            {added > 0 ? (
              <StatusBadge
                label={`+${added}`}
                tone="orange"
                compact
                title="Nach der Frist dazugekommen"
              />
            ) : null}
            {removed > 0 ? (
              <StatusBadge
                label={`−${removed}`}
                tone="orange"
                compact
                title="Nach der Frist abgemeldet"
              />
            ) : null}
          </span>
        );
      },
    })),
    {
      key: "sum",
      header: "Summe",
      align: "right",
      render: (row) => (
        <span className="font-semibold">
          {row.school
            ? days.reduce((sum, date) => sum + counts(dayOf(row, date)), 0)
            : days.reduce((sum, date) => sum + daySum(date), 0)}
        </span>
      ),
    },
  ];

  return (
    <div className="space-y-6">
      {loading && !schools ? (
        <Loading message="Wird geladen …" />
      ) : error ? (
        <LoadErrorAlert error={error} />
      ) : list.length === 0 ? (
        <EmptyState
          title="Sie beliefern zurzeit keine Schule"
          description="Sobald eine Schule Sie einlädt, sehen Sie hier ihre Bestellungen."
        />
      ) : (
        <>
          <SectionCard
            title={
              schools?.catererName
                ? `Bestellungen für ${schools.catererName}`
                : "Bestellungen"
            }
            description="Eine Zahl wie +2 ist eine Änderung nach der Frist."
            actions={
              <div className="flex flex-wrap items-center gap-2">
                <SegmentedControl<Period>
                  items={PERIODS}
                  value={period}
                  onChange={setPeriod}
                  ariaLabel="Zeitraum wählen"
                />
                <Button
                  type="button"
                  variant="outline"
                  size="md"
                  className="gap-2 bg-white"
                  isLoading={exporting === "pdf"}
                  loadingText="Wird erstellt …"
                  disabled={exporting !== null}
                  onClick={() => void download("pdf")}
                >
                  <Download className="h-4 w-4" aria-hidden="true" />
                  PDF
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  size="md"
                  className="gap-2 bg-white"
                  isLoading={exporting === "xlsx"}
                  loadingText="Wird erstellt …"
                  disabled={exporting !== null}
                  onClick={() => void download("xlsx")}
                >
                  <FileSpreadsheet className="h-4 w-4" aria-hidden="true" />
                  Excel
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  size="md"
                  className="gap-2 bg-white"
                  isLoading={exporting === "csv"}
                  loadingText="Wird erstellt …"
                  disabled={exporting !== null}
                  onClick={() => void download("csv")}
                >
                  <FileSpreadsheet className="h-4 w-4" aria-hidden="true" />
                  CSV
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  size="md"
                  className="gap-2 bg-white"
                  isLoading={exporting === "print"}
                  loadingText="Wird erstellt …"
                  disabled={exporting !== null}
                  onClick={() => void print()}
                >
                  <Printer className="h-4 w-4" aria-hidden="true" />
                  Drucken
                </Button>
              </div>
            }
          >
            <DataTable
              columns={columns}
              rows={rows}
              getRowKey={(row) => row.key}
              isLoading={loading}
              stackedOnMobile
              caption="Essen je Schule und Tag im gewählten Zeitraum"
            />
          </SectionCard>

          {list.map((school) => {
            const schoolOrders = orders[school.id];
            return (
              <SectionCard
                key={school.id}
                title={school.name}
                description={
                  schoolOrders?.deliveryAddress
                    ? `Lieferadresse: ${schoolOrders.deliveryAddress}`
                    : undefined
                }
                collapsible
                defaultCollapsed
              >
                {schoolOrders ? (
                  <CatererSchoolOrders
                    school={school}
                    orders={schoolOrders}
                    onConfirmed={() => loadOrders(list)}
                  />
                ) : (
                  <p className="text-sm text-gray-600">
                    Die Bestellung dieser Schule ließ sich nicht laden. Bitte
                    laden Sie die Seite neu.
                  </p>
                )}
              </SectionCard>
            );
          })}
        </>
      )}
    </div>
  );
}
