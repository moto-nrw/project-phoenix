"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { Download, FileSpreadsheet } from "lucide-react";

import { Button } from "~/components/ui/button";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { SectionCard } from "~/components/ui/section-card";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { StatusBadge } from "~/components/ui/status-badge";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { useApiErrorDisplay, useApiLoadError } from "~/contexts/ToastContext";
import { parseISODate, toISODate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  downloadMealDeliveries,
  downloadMealMonth,
  getMealOrders,
  type MealOrderDay,
} from "~/lib/meal-plan-api";

const logger = createLogger({ component: "MealOrderWeek" });

/**
 * Die Bestellung der Woche (#3873): je Tag, ob die Frist noch läuft oder
 * bestellt ist, die Zahl der Essen, je Klasse und die Nachmeldungen danach.
 * Namen stehen in der Tagesliste darunter.
 */
export function MealOrderWeek({
  mondayISO,
  canExportMonth,
}: Readonly<{ mondayISO: string; canExportMonth: boolean }>) {
  const t = useTranslations("mealOrders");
  const locale = useLocale();
  const [days, setDays] = useState<MealOrderDay[]>([]);
  const [loading, setLoading] = useState(true);
  const [exporting, setExporting] = useState<"pdf" | "xlsx" | null>(null);
  const { error, show, clear } = useApiLoadError();
  const { show: showActionError } = useApiErrorDisplay();
  const reloadRef = useRef<() => void>(() => undefined);

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
    (value: string) =>
      parseISODate(`${value}-01`).toLocaleDateString(locale, {
        month: "long",
      }),
    [locale],
  );

  const load = useCallback(async () => {
    setLoading(true);
    clear();
    try {
      const orders = await getMealOrders(mondayISO, friday);
      setDays(orders.days);
    } catch (cause) {
      logger.error("meal_orders_load_failed", {
        error: cause instanceof Error ? cause.message : String(cause),
      });
      setDays([]);
      void show(cause, {
        object: t("errorObject"),
        retry: () => reloadRef.current(),
      });
    } finally {
      setLoading(false);
    }
  }, [clear, friday, mondayISO, show, t]);
  useEffect(() => {
    reloadRef.current = () => void load();
  });
  useEffect(() => {
    void load();
  }, [load]);

  async function downloadDeliveries(format: "pdf" | "xlsx" | "csv") {
    try {
      await downloadMealDeliveries(mondayISO, friday, format);
    } catch (cause) {
      logger.error("meal_delivery_export_failed", {
        error: cause instanceof Error ? cause.message : String(cause),
      });
      void showActionError(cause, { object: t("errorObjectDelivery") });
    }
  }

  async function download(format: "pdf" | "xlsx") {
    setExporting(format);
    try {
      await downloadMealMonth(month, format);
    } catch (cause) {
      logger.error("meal_month_export_failed", {
        error: cause instanceof Error ? cause.message : String(cause),
      });
      void showActionError(cause, { object: t("errorObjectMonth") });
    } finally {
      setExporting(null);
    }
  }

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
        <div className="flex flex-wrap items-center gap-2">
          {canExportMonth ? (
            <div
              role="group"
              aria-label={t("monthGroup")}
              className="flex flex-wrap items-center gap-2"
            >
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
              ) : null}
              <Button
                type="button"
                variant="outline"
                size="md"
                className="gap-2 bg-white"
                onClick={() => void download("pdf")}
                isLoading={exporting === "pdf"}
                loadingText={t("downloadLoading")}
                disabled={exporting !== null}
              >
                <Download className="h-4 w-4" aria-hidden="true" />
                {t("monthPdf", { month: monthName(month) })}
              </Button>
              <Button
                type="button"
                variant="outline"
                size="md"
                className="gap-2 bg-white"
                onClick={() => void download("xlsx")}
                isLoading={exporting === "xlsx"}
                loadingText={t("downloadLoading")}
                disabled={exporting !== null}
              >
                <FileSpreadsheet className="h-4 w-4" aria-hidden="true" />
                {t("monthExcel", { month: monthName(month) })}
              </Button>
            </div>
          ) : null}
          {/* Die Lieferliste ist dieselbe Vorlage, die der Caterer bekommt:
              ohne Namen, darum schon mit meals:read (#3873). */}
          <OverflowMenu
            ariaLabel={t("deliveryMenu")}
            items={[
              {
                label: t("deliveryPdf"),
                onClick: () => void downloadDeliveries("pdf"),
              },
              {
                label: t("deliveryExcel"),
                onClick: () => void downloadDeliveries("xlsx"),
              },
              {
                label: t("deliveryCsv"),
                onClick: () => void downloadDeliveries("csv"),
              },
            ]}
          />
        </div>
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
    </SectionCard>
  );
}
