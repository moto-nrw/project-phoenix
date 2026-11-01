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
import { FileSpreadsheet, FileText, Printer } from "lucide-react";
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
  confirmCatererOrder,
  fetchCatererOrders,
  fetchCatererSchools,
  type CatererOrderDay,
  type CatererOrders,
  type CatererSchool,
  type DeliveryExportFormat,
} from "~/lib/caterer-api";
import {
  defaultPeriod,
  periodDays,
  PERIODS,
  type Period,
} from "~/lib/caterer-period";
import { parseISODate } from "~/lib/date-helpers";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";
import { openBlobForPrint } from "~/lib/file-download";
import { createLogger } from "~/lib/logger";
import { useSession } from "next-auth/react";
import { useSWRAuth, useInvalidateMatching, useSWRCacheKey } from "~/lib/swr";
import { useDiscardRefusedEntity } from "~/lib/swr/hooks";
import { catererKeys } from "~/lib/swr/portal-keys";
import { useOptimisticAction } from "~/lib/swr/optimistic";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";

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
  const today = useBerlinToday();
  const [period, setPeriod] = useState<Period>(() => defaultPeriod(today));
  const days = useMemo(() => periodDays(period, today), [period, today]);
  const from = days[0]!;
  const to = days[days.length - 1]!;
  const { data: session } = useSession();
  const enabled = session?.user.scope === "caterer";
  const schoolResponse = useSWRAuth(
    enabled ? catererKeys.schools() : null,
    fetchCatererSchools,
    {
      keepPreviousData: false,
      revalidateOnFocus: true,
      focusThrottleInterval: 60 * 1000,
    },
  );
  const schools = schoolResponse.data ?? null;
  const schoolIds = schools?.schools.map((school) => school.id) ?? [];
  const ordersKey =
    enabled && schools ? catererKeys.orders(schoolIds, from, to) : null;
  const response = useSWRAuth(
    ordersKey,
    async () => {
      const results = await Promise.allSettled(
        schoolIds.map((schoolId) => fetchCatererOrders(schoolId, from, to)),
      );
      const orders: Record<string, CatererOrders | null> = {};
      const failures: Record<string, unknown> = {};
      results.forEach((result, index) => {
        const schoolId = schoolIds[index]!;
        if (result.status === "fulfilled") orders[schoolId] = result.value;
        else {
          orders[schoolId] = null;
          failures[schoolId] = result.reason;
          logger.warn("caterer_orders_load_failed", {
            school_id: schoolId,
            error:
              result.reason instanceof Error
                ? result.reason.message
                : String(result.reason),
          });
        }
      });
      return { orders, failures };
    },
    { keepPreviousData: false },
  );
  const discardRefused = useDiscardRefusedEntity();
  useEffect(() => {
    for (const cause of Object.values(response.data?.failures ?? {})) {
      // The current window already contains the per-school error, never old orders.
      // Other cached windows must not reopen data whose access was just refused.
      discardRefused(cause, {
        codes: ["meals.not_supplied", "meals.order_not_found"],
        matches: (key) =>
          key.startsWith("caterer-orders-") && key !== ordersKey,
      });
    }
  }, [response.data, ordersKey, discardRefused]);
  const scopedKey = useSWRCacheKey()(ordersKey ?? "");
  const currentWindow = useRef({
    scopedKey,
    key: ordersKey,
    mutate: response.mutate,
  });
  useLayoutEffect(() => {
    currentWindow.current = {
      scopedKey,
      key: ordersKey,
      mutate: response.mutate,
    };
  });
  const action = useOptimisticAction(ordersKey, response.data);
  const orders = action.data?.orders ?? {};
  const failures = action.data?.failures ?? {};
  const loading = schoolResponse.isLoading || response.isLoading;
  const error = useSwrLoadError(
    schoolResponse.error ?? response.error,
    schoolResponse.error ? "Ihre Schulen" : "die Bestellungen",
    schoolResponse.error ? schoolResponse.mutate : response.mutate,
    schoolResponse.error ? schoolResponse.data : response.data,
  );
  const mutateOrders = response.mutate;
  const [exporting, setExporting] = useState<string | null>(null);
  const { show: showActionError } = useApiErrorDisplay();
  const invalidate = useInvalidateMatching();

  const runAction = action.run;
  const retries = useRef(new Map<string, number>());
  const retrySchool = useCallback(
    async (school: CatererSchool) => {
      if (currentWindow.current.scopedKey !== scopedKey) return;
      const ticket = (retries.current.get(school.id) ?? 0) + 1;
      retries.current.set(school.id, ticket);
      const outcome = await runAction({
        // A refresh has no provisional data, but shares the action's generation guard.
        optimistic: (current) => current,
        request: () => fetchCatererOrders(school.id, from, to),
        refusal: {
          codes: ["meals.not_supplied", "meals.order_not_found"],
          matches: (key) => key.startsWith("caterer-orders-"),
        },
        commit: (loaded) => {
          if (retries.current.get(school.id) !== ticket) return;
          // Read the latest window only after the GET, so other schools' writes survive.
          void mutateOrders(
            (current) =>
              current
                ? {
                    orders: { ...current.orders, [school.id]: loaded },
                    failures: { ...current.failures, [school.id]: undefined },
                  }
                : current,
            { revalidate: false },
          );
        },
      });
      if (outcome.status === "discarded") return;
      if (outcome.status === "failed" && outcome.refused) {
        // The action cleared every cached window; its error is not a reader error.
        void showActionError(outcome.error, {
          object: "die Bestellungen",
          retry: () => void invalidate((key) => key === ordersKey),
        });
        return;
      }
      if (
        outcome.status === "failed" &&
        retries.current.get(school.id) === ticket
      ) {
        // Keep a successfully confirmed snapshot and show the refresh error beside it.
        await mutateOrders(
          (current) =>
            current
              ? {
                  ...current,
                  failures: { ...current.failures, [school.id]: outcome.error },
                }
              : current,
          { revalidate: false },
        );
      }
    },
    [
      from,
      to,
      mutateOrders,
      runAction,
      scopedKey,
      showActionError,
      invalidate,
      ordersKey,
    ],
  );

  async function confirm(school: CatererSchool, orderId: string) {
    const markConfirmed = (current: NonNullable<typeof response.data>) => {
      const previous = current.orders[school.id];
      if (!previous) return current;
      return {
        ...current,
        orders: {
          ...current.orders,
          [school.id]: {
            ...previous,
            days: previous.days.map((day) =>
              day.orderId === orderId
                ? { ...day, status: "confirmed" as const }
                : day,
            ),
          },
        },
      };
    };
    const outcome = await action.run({
      optimistic: markConfirmed,
      request: () => confirmCatererOrder(school.id, orderId),
      commit: () => {
        // The successful POST confirms the status even if the subsequent GET fails.
        void mutateOrders(
          (current) => (current ? markConfirmed(current) : current),
          { revalidate: false },
        );
        const visible = currentWindow.current;
        if (visible.key !== null && visible.key !== ordersKey) {
          // A period selected while the POST ran stays visible and gets the acknowledged status.
          void visible.mutate(
            (current) => (current ? markConfirmed(current) : current),
            { revalidate: true },
          );
        }
        // Inactive windows have no reader to refresh them: never leave stale action data there.
        void invalidate(
          (key) =>
            key.startsWith("caterer-orders-") &&
            key !== ordersKey &&
            key !== visible.key,
          { clear: true },
        );
      },
      refusal: {
        codes: ["meals.not_supplied", "meals.order_not_found"],
        matches: (key) => key.startsWith("caterer-orders-"),
      },
    });
    if (outcome.status === "failed") throw outcome.error;
    if (outcome.status === "discarded") {
      // The session/window no longer owns this action: no toast or reload.
      return;
    }
    if (currentWindow.current.scopedKey === scopedKey)
      await retrySchool(school);
  }

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
  const totalsComplete =
    !loading && list.every((school) => orders[school.id] != null);
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
          return (
            <span className="font-semibold">
              {totalsComplete ? daySum(date) : "Unvollständig"}
            </span>
          );
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
          {row.school && !row.orders
            ? "Nicht geladen"
            : !row.school && !totalsComplete
              ? "Unvollständig"
              : row.school
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
        { format: "pdf", label: "PDF", Icon: FileText },
        { format: "xlsx", label: "Excel", Icon: FileSpreadsheet },
        { format: "csv", label: "CSV", Icon: FileSpreadsheet },
      ] as {
        format: DeliveryExportFormat;
        label: string;
        Icon: typeof FileText;
      }[]
    ).map(({ format, label, Icon }) => ({
      label: exporting === format ? "Wird erstellt…" : label,
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
                  <>
                    {failures[school.id] != null && (
                      <SchoolLoadError
                        school={school}
                        cause={failures[school.id]}
                        onRetry={() => retrySchool(school)}
                      />
                    )}
                    <CatererSchoolOrders
                      school={school}
                      orders={schoolOrders}
                      onConfirm={(orderId) => confirm(school, orderId)}
                    />
                  </>
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
