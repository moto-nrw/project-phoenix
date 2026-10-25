"use client";

// Overview of the caterer portal (ADR 0077, #3873): one table of every
// school the caterer supplies, the days of the chosen period as columns,
// with a sum per school and per day. Each school opens to its days with
// classes, names (if the school shows them) and the confirmation. Each
// school is read on its own; a school that ends the supply drops out.

import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { Download, FileSpreadsheet, FileText, Printer } from "lucide-react";
import { CatererSchoolOrders } from "~/components/caterer/caterer-school-orders";
import { Button } from "~/components/ui/button";
import {
  DataTable,
  DataTableSkeleton,
  type DataTableColumn,
} from "~/components/ui/data-table";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import {
  OverflowMenu,
  type OverflowMenuEntry,
} from "~/components/ui/page-header/OverflowMenu";
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
      />
    );
  }
  if (live.length === 0) return null;
  return live.every((day) => day.status === "confirmed") ? (
    <StatusBadge label="Bestätigt" tone="green" />
  ) : (
    <StatusBadge label="Bestellt" tone="blue" />
  );
}

/**
 * A school whose orders did not load, where its orders would stand: the
 * shared load-error text with „Wiederholen“ for this one school.
 */
function SchoolLoadError({
  school,
  cause,
  onRetry,
}: Readonly<{
  school: CatererSchool;
  cause: unknown;
  onRetry: () => Promise<void>;
}>) {
  const { error, show } = useApiLoadError();
  const retryRef = useRef(onRetry);
  useLayoutEffect(() => {
    retryRef.current = onRetry;
  });
  useEffect(() => {
    void show(cause, {
      object: `die Bestellung von ${school.name}`,
      retry: () => void retryRef.current(),
    });
  }, [cause, school.name, show]);
  return <LoadErrorAlert error={error} />;
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
  // Why a school's orders did not load, shown in that school's card.
  const [failures, setFailures] = useState<Record<string, unknown>>({});
  const [loading, setLoading] = useState(true);
  const [exporting, setExporting] = useState<string | null>(null);
  const { error, show, clear } = useApiLoadError();
  const { show: showActionError } = useApiErrorDisplay();
  const reloadRef = useRef<() => Promise<void>>(async () => undefined);

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
      const failed: Record<string, unknown> = {};
      results.forEach((result, index) => {
        const school = list[index]!;
        if (result.status === "fulfilled") {
          next[school.id] = result.value;
        } else {
          next[school.id] = null;
          failed[school.id] = result.reason;
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
      setFailures(failed);
    },
    [from, to],
  );

  // „Wiederholen“ in the card of one school asks for that school only. A
  // newer load of the whole page wins over a retry still on its way.
  const retrySchool = useCallback(
    async (school: CatererSchool) => {
      const generation = generationRef.current;
      const request = requestRef.current;
      const current = () =>
        generation === generationRef.current && request === requestRef.current;
      try {
        const loaded = await fetchCatererOrders(school.id, from, to);
        if (!current()) return;
        setOrders((previous) => ({ ...previous, [school.id]: loaded }));
        setFailures((previous) => {
          const next = { ...previous };
          delete next[school.id];
          return next;
        });
      } catch (cause) {
        if (!current()) return;
        logger.warn("caterer_orders_load_failed", {
          school_id: school.id,
          error: cause instanceof Error ? cause.message : String(cause),
        });
        setFailures((previous) => ({ ...previous, [school.id]: cause }));
      }
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
    reloadRef.current = load;
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
                title="Nach der Frist dazugekommen"
              />
            ) : null}
            {removed > 0 ? (
              <StatusBadge
                label={`−${removed}`}
                tone="orange"
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

  // Eine sichtbare Aktion neben dem Zeitraum, die Formate im Menü (wie
  // Tagesauswertung und Statistik): Drucken ist der tägliche Griff, die
  // Dateien sind der seltene. Vorher standen vier Knöpfe in der Kopfzeile.
  const exportMenuItems: OverflowMenuEntry[] = [
    { kind: "header", label: "Herunterladen" },
    ...(
      [
        { format: "pdf", label: "PDF", Icon: Download },
        { format: "xlsx", label: "Excel", Icon: FileSpreadsheet },
        { format: "csv", label: "CSV", Icon: FileText },
      ] as {
        format: DeliveryExportFormat;
        label: string;
        Icon: typeof Download;
      }[]
    ).map(({ format, label, Icon }) => ({
      label: exporting === format ? "Wird exportiert…" : label,
      icon: <Icon className="h-4 w-4" aria-hidden />,
      onClick: () => void download(format),
      disabled: !schools || exporting !== null,
    })),
  ];

  // Laden, Fehler und Leer stehen in derselben Karte wie die Tabelle, die
  // sie ersetzen; nichts steht frei auf dem Hintergrund.
  const unavailable = Boolean(error) || (schools !== null && list.length === 0);

  return (
    <div className="space-y-6">
      <SectionCard
        title={
          schools?.catererName
            ? `Bestellungen für ${schools.catererName}`
            : "Bestellungen"
        }
        description={
          unavailable
            ? undefined
            : "Eine Zahl wie +2 ist eine Änderung nach der Frist."
        }
        actions={
          unavailable ? undefined : (
            <div className="flex flex-wrap items-center gap-2">
              <SegmentedControl<Period>
                items={PERIODS}
                value={period}
                onChange={setPeriod}
                ariaLabel="Zeitraum wählen"
              />
              <Button
                type="button"
                variant="primary"
                size="md"
                className="gap-2"
                disabled={!schools || exporting !== null}
                onClick={() => void print()}
              >
                <Printer className="h-4 w-4" aria-hidden />
                {exporting === "print" ? "Wird geöffnet…" : "Drucken"}
              </Button>
              <OverflowMenu
                items={exportMenuItems}
                ariaLabel="Weitere Aktionen"
              />
            </div>
          )
        }
      >
        {error ? (
          <LoadErrorAlert error={error} />
        ) : schools !== null && list.length === 0 ? (
          <p className="text-sm leading-6 text-gray-600">
            Sie beliefern zurzeit keine Schule. Sobald eine Schule Sie einlädt,
            sehen Sie hier ihre Bestellungen.
          </p>
        ) : (
          <DataTable
            columns={columns}
            rows={rows}
            getRowKey={(row) => row.key}
            isLoading={loading}
            loadingRowCount={3}
            stackedOnMobile
            caption="Essen je Schule und Tag im gewählten Zeitraum"
          />
        )}
      </SectionCard>

      {error || !schools
        ? null
        : list.map((school) => {
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
                {schoolOrders === undefined ? (
                  <DataTableSkeleton rows={3} columns={5} />
                ) : schoolOrders ? (
                  <CatererSchoolOrders
                    school={school}
                    orders={schoolOrders}
                    onConfirmed={() => reloadRef.current()}
                  />
                ) : (
                  <SchoolLoadError
                    school={school}
                    cause={failures[school.id]}
                    onRetry={() => retrySchool(school)}
                  />
                )}
              </SectionCard>
            );
          })}
    </div>
  );
}
