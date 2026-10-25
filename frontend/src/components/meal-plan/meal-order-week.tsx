"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { CalendarRange, FileSpreadsheet, FileText } from "lucide-react";

import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import {
  EXPORT_FORMAT_LABELS,
  ExportDialog,
} from "~/components/ui/export-dialog";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { SectionCard } from "~/components/ui/section-card";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { StatusBadge } from "~/components/ui/status-badge";
import {
  OverflowMenu,
  type OverflowMenuEntry,
} from "~/components/ui/page-header/OverflowMenu";
import { useApiErrorDisplay, useApiLoadError } from "~/contexts/ToastContext";
import { useExportRun } from "~/hooks/use-export-run";
import { parseISODate, toISODate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  downloadMealDeliveries,
  downloadMealMonth,
  getMealOrders,
  type MealOrderDay,
} from "~/lib/meal-plan-api";

const logger = createLogger({ component: "MealOrderWeek" });

type DeliveryFormat = "pdf" | "xlsx" | "csv";
type MonthFormat = "pdf" | "xlsx";

const DELIVERY_FORMATS: readonly DeliveryFormat[] = ["pdf", "xlsx", "csv"];
const MONTH_FORMATS: readonly MonthFormat[] = ["pdf", "xlsx"];

/**
 * Die Bestellung der Woche (#3873): je Tag, ob die Frist noch läuft oder
 * bestellt ist, die Zahl der Essen, je Klasse und die Nachmeldungen danach.
 * Namen stehen in der Tagesliste darunter.
 */
export function MealOrderWeek({
  mondayISO,
  canExportMonth,
  refreshVersion = 0,
}: Readonly<{
  mondayISO: string;
  canExportMonth: boolean;
  refreshVersion?: number;
}>) {
  const t = useTranslations("mealOrders");
  const locale = useLocale();
  const [days, setDays] = useState<MealOrderDay[]>([]);
  const [loading, setLoading] = useState(true);
  const [deliveryBusy, setDeliveryBusy] = useState<DeliveryFormat | null>(null);
  const [monthExportOpen, setMonthExportOpen] = useState(false);
  const [monthBusy, setMonthBusy] = useState<MonthFormat | null>(null);
  const { error, show, clear } = useApiLoadError();
  const { show: showActionError } = useApiErrorDisplay();
  const reloadRef = useRef<() => void>(() => undefined);
  const requestGeneration = useRef(0);

  const friday = useMemo(() => {
    const date = parseISODate(mondayISO);
    date.setDate(date.getDate() + 4);
    return toISODate(date);
  }, [mondayISO]);
  // Eine Woche über zwei Monate bietet beide Monate zum Herunterladen an.
  const months = useMemo(
    () => [...new Set([mondayISO.slice(0, 7), friday.slice(0, 7)])],
    [friday, mondayISO],
  );
  const [chosenMonth, setChosenMonth] = useState<string | null>(null);
  const month =
    chosenMonth && months.includes(chosenMonth) ? chosenMonth : months[0]!;
  const monthName = useCallback(
    (value: string, withYear = false) =>
      parseISODate(`${value}-01`).toLocaleDateString(locale, {
        month: "long",
        ...(withYear ? { year: "numeric" } : {}),
      }),
    [locale],
  );

  const load = useCallback(async () => {
    const generation = ++requestGeneration.current;
    setLoading(true);
    clear();
    try {
      const orders = await getMealOrders(mondayISO, friday);
      if (generation !== requestGeneration.current) return;
      setDays(orders.days);
    } catch (cause) {
      // A superseded request must not replace the current week's view.
      if (generation !== requestGeneration.current) return;
      logger.error("meal_orders_load_failed", {
        error: cause instanceof Error ? cause.message : String(cause),
      });
      setDays([]);
      void show(cause, {
        object: t("errorObject"),
        retry: () => reloadRef.current(),
      });
    } finally {
      if (generation === requestGeneration.current) setLoading(false);
    }
  }, [clear, friday, mondayISO, show, t]);
  const invalidateRequests = useCallback(() => {
    requestGeneration.current++;
  }, []);
  useEffect(() => {
    reloadRef.current = () => void load();
  });
  useEffect(() => {
    void load();
    return invalidateRequests;
  }, [invalidateRequests, load, refreshVersion]);

  async function downloadDeliveries(format: DeliveryFormat) {
    setDeliveryBusy(format);
    try {
      await downloadMealDeliveries(mondayISO, friday, format);
    } catch (cause) {
      logger.error("meal_delivery_export_failed", {
        error: cause instanceof Error ? cause.message : String(cause),
      });
      void showActionError(cause, { object: t("errorObjectDelivery") });
    } finally {
      setDeliveryBusy(null);
    }
  }

  // Liegt die Woche in einem Monat, hat die Monatsübersicht keine Option:
  // ein Eintrag je Format wie bei der Lieferliste. Nur eine Woche über zwei
  // Monate fragt den Monat im gemeinsamen Export-Dialog ab
  // (docs/reference/exporte.md, Regel 2); Fehler bleiben dort im Dialog.
  async function downloadMonth(format: MonthFormat) {
    setMonthBusy(format);
    try {
      await downloadMealMonth(month, format);
    } catch (cause) {
      logger.error("meal_month_export_failed", {
        error: cause instanceof Error ? cause.message : String(cause),
      });
      void showActionError(cause, { object: t("errorObjectMonth") });
    } finally {
      setMonthBusy(null);
    }
  }
  const monthRun = useExportRun<MonthFormat>({
    isOpen: monthExportOpen,
    onDownload: (format) => downloadMealMonth(month, format),
    errorObject: t("errorObjectMonth"),
    onDone: () => setMonthExportOpen(false),
    logName: "meal_month",
  });

  // EIN Export-Einstieg im Kartenkopf: die Lieferliste ohne Optionen mit
  // einem Eintrag je Format, die Monatsübersicht mit Optionen als Dialog.
  const exportItems: OverflowMenuEntry[] = [
    { kind: "header", label: t("deliveryHeader") },
    ...DELIVERY_FORMATS.map((format): OverflowMenuEntry => ({
      label:
        deliveryBusy === format
          ? t("downloadLoading")
          : EXPORT_FORMAT_LABELS[format],
      icon:
        format === "pdf" ? (
          <FileText className="h-4 w-4" aria-hidden />
        ) : (
          <FileSpreadsheet className="h-4 w-4" aria-hidden />
        ),
      disabled: deliveryBusy !== null,
      onClick: () => void downloadDeliveries(format),
    })),
    { kind: "separator" },
    ...(months.length > 1
      ? ([
          { kind: "header", label: t("monthHeader") },
          {
            label: t("monthExport"),
            icon: <CalendarRange className="h-4 w-4" aria-hidden />,
            onClick: () => setMonthExportOpen(true),
          },
        ] satisfies OverflowMenuEntry[])
      : [
          {
            kind: "header" as const,
            label: `${t("monthHeader")} ${monthName(month, true)}`,
          },
          ...MONTH_FORMATS.map((format): OverflowMenuEntry => ({
            label:
              monthBusy === format
                ? t("downloadLoading")
                : EXPORT_FORMAT_LABELS[format],
            icon:
              format === "pdf" ? (
                <FileText className="h-4 w-4" aria-hidden />
              ) : (
                <FileSpreadsheet className="h-4 w-4" aria-hidden />
              ),
            disabled: monthBusy !== null,
            onClick: () => void downloadMonth(format),
          })),
        ]),
  ];

  const dateTime = useCallback(
    (instant: string) =>
      new Date(instant).toLocaleString(locale, {
        weekday: "short",
        day: "2-digit",
        month: "2-digit",
        hour: "2-digit",
        minute: "2-digit",
        timeZone: "Europe/Berlin",
      }),
    [locale],
  );

  const columns = useMemo<DataTableColumn<MealOrderDay>[]>(
    () => [
      {
        key: "date",
        header: t("day"),
        render: (day) =>
          parseISODate(day.date).toLocaleDateString(locale, {
            weekday: "long",
            day: "2-digit",
            month: "2-digit",
          }),
        stacked: "title",
      },
      {
        key: "status",
        header: t("status"),
        render: (day) =>
          day.closed ? (
            <StatusBadge label={t("closed")} tone="gray" />
          ) : day.status === "unordered" ? (
            <StatusBadge label={t("unordered")} tone="gray" />
          ) : day.status === "open" ? (
            <StatusBadge label={t("open")} tone="gray" />
          ) : day.status === "confirmed" ? (
            <StatusBadge label={t("confirmed")} tone="green" />
          ) : (
            <StatusBadge label={t("ordered")} tone="blue" />
          ),
        stacked: "meta",
      },
      {
        key: "deadline",
        header: t("deadline"),
        className: "whitespace-nowrap",
        render: (day) =>
          day.closed || day.status === "unordered"
            ? "–"
            : dateTime(day.deadline),
      },
      {
        key: "total",
        header: t("meals"),
        render: (day) =>
          day.closed || day.status === "unordered" ? "–" : String(day.total),
        align: "right",
      },
      {
        key: "classes",
        header: t("byClass"),
        render: (day) =>
          day.classes.length === 0
            ? "–"
            : day.classes
                .map((count) => `${count.schoolClass || "–"}: ${count.count}`)
                .join(" · "),
      },
      {
        key: "changes",
        header: t("changes"),
        render: (day) => {
          const added = day.changes.filter((change) => change.delta > 0);
          const removed = day.changes.filter((change) => change.delta < 0);
          if (added.length === 0 && removed.length === 0) {
            return (
              <>
                <span aria-hidden="true">–</span>
                <span className="sr-only">{t("noChanges")}</span>
              </>
            );
          }
          const label = [
            added.length > 0
              ? t("changesAdded", { count: added.length })
              : null,
            removed.length > 0
              ? t("changesRemoved", { count: removed.length })
              : null,
          ]
            .filter(Boolean)
            .join(", ");
          return (
            <>
              <span aria-hidden="true" title={label}>
                {[
                  added.length > 0 ? `+${added.length}` : null,
                  removed.length > 0 ? `−${removed.length}` : null,
                ]
                  .filter(Boolean)
                  .join(" · ")}
              </span>
              <span className="sr-only">{label}</span>
            </>
          );
        },
      },
    ],
    [dateTime, locale, t],
  );

  return (
    <SectionCard
      title={t("title")}
      description={t("description")}
      actions={
        canExportMonth ? (
          <OverflowMenu ariaLabel={t("exportMenu")} items={exportItems} />
        ) : undefined
      }
    >
      {error ? (
        <LoadErrorAlert error={error} />
      ) : (
        <DataTable
          columns={columns}
          rows={days}
          getRowKey={(day) => day.date}
          isLoading={loading}
          stackedOnMobile
          caption={t("caption")}
        />
      )}
      {canExportMonth ? (
        <ExportDialog<MonthFormat>
          isOpen={monthExportOpen}
          onClose={() => setMonthExportOpen(false)}
          title={t("monthDialogTitle")}
          description={t("monthDialogDescription")}
          formats={MONTH_FORMATS}
          run={monthRun}
        >
          <section>
            <p className="text-sm font-medium text-gray-900">
              {t("monthLabel")}
            </p>
            <div className="mt-2">
              {/* Eine Woche über zwei Monate bietet beide Monate an. */}
              {months.length > 1 ? (
                <SegmentedControl<string>
                  items={months.map((value) => ({
                    value,
                    label: monthName(value),
                  }))}
                  value={month}
                  onChange={setChosenMonth}
                  ariaLabel={t("monthChoice")}
                />
              ) : (
                <p className="text-sm text-gray-700">
                  {monthName(month, true)}
                </p>
              )}
            </div>
          </section>
        </ExportDialog>
      ) : null}
    </SectionCard>
  );
}
