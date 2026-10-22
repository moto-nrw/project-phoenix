"use client";

import { useCallback, useMemo, useState } from "react";

import { Button } from "~/components/ui/button";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { StatusBadge } from "~/components/ui/status-badge";
import { useApiErrorDisplay } from "~/contexts/ToastContext";
import {
  confirmCatererOrder,
  type CatererOrderChild,
  type CatererOrderDay,
  type CatererOrders,
  type CatererSchool,
} from "~/lib/caterer-api";
import { parseISODate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "CatererSchoolOrders" });

function dayLabel(date: string): string {
  return parseISODate(date).toLocaleDateString("de-DE", {
    weekday: "long",
    day: "2-digit",
    month: "2-digit",
  });
}

function dateTime(instant: string): string {
  return new Date(instant).toLocaleString("de-DE", {
    weekday: "short",
    day: "2-digit",
    month: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    timeZone: "Europe/Berlin",
  });
}

function childLabel(child: CatererOrderChild): string {
  const name = `${child.firstName} ${child.lastName}`.trim();
  if (!name) return child.schoolClass;
  return child.schoolClass ? `${name} (${child.schoolClass})` : name;
}

function changesLabel(day: CatererOrderDay): string {
  if (day.changes.length === 0) return "–";
  return day.changes
    .map((change) => {
      const who = childLabel(change.child);
      const sign = change.delta > 0 ? "+1" : "−1";
      return who ? `${sign} ${who}` : sign;
    })
    .join(" · ");
}

/** The first day of each order in the window carries its confirmation. */
function firstDaysOfOrders(days: readonly CatererOrderDay[]): Set<string> {
  const seen = new Set<string>();
  const first = new Set<string>();
  for (const day of days) {
    if (day.orderId && !seen.has(day.orderId)) {
      seen.add(day.orderId);
      first.add(day.date);
    }
  }
  return first;
}

/**
 * One school's orders of the chosen days, as its caterer reads them (ADR
 * 0074): per day the state, the number of meals, by class when the school
 * orders by class, the names while the school shows them, and the changes
 * after the deadline. A fixed order can be confirmed once; the overview
 * loads the orders and reloads them after a confirmation.
 */
export function CatererSchoolOrders({
  school,
  orders,
  onConfirmed,
}: Readonly<{
  school: CatererSchool;
  orders: CatererOrders;
  onConfirmed: () => Promise<void>;
}>) {
  const [confirming, setConfirming] = useState<string | null>(null);
  // Die Namen eines Tages sind zugeklappt: ausgeschrieben machen sie jede
  // Zeile so hoch wie die Kinderliste und verdrängen die Zahlen.
  const [namesShown, setNamesShown] = useState<ReadonlySet<string>>(
    () => new Set(),
  );
  const { show: showActionError } = useApiErrorDisplay();
  const toggleNames = useCallback((date: string) => {
    setNamesShown((current) => {
      const next = new Set(current);
      if (next.has(date)) next.delete(date);
      else next.add(date);
      return next;
    });
  }, []);

  const confirm = useCallback(
    async (orderId: string) => {
      setConfirming(orderId);
      try {
        await confirmCatererOrder(school.id, orderId);
        await onConfirmed();
      } catch (cause) {
        logger.error("caterer_order_confirm_failed", {
          error: cause instanceof Error ? cause.message : String(cause),
        });
        void showActionError(cause, { object: "die Bestätigung" });
      } finally {
        setConfirming(null);
      }
    },
    [onConfirmed, school.id, showActionError],
  );

  const days = useMemo(() => orders?.days ?? [], [orders]);
  const byClass = orders?.detail === "by_class";
  const named = days.some((day) => day.children.length > 0);
  const firstDays = useMemo(() => firstDaysOfOrders(days), [days]);

  const columns = useMemo<DataTableColumn<CatererOrderDay>[]>(() => {
    const result: DataTableColumn<CatererOrderDay>[] = [
      {
        key: "date",
        header: "Tag",
        render: (day) => dayLabel(day.date),
        stacked: "title",
      },
      {
        key: "status",
        header: "Stand",
        render: (day) =>
          day.closed ? (
            <StatusBadge label="Schule geschlossen" tone="gray" />
          ) : day.status === "unordered" ? (
            <StatusBadge label="Nicht bestellt" tone="gray" />
          ) : day.status === "open" ? (
            <StatusBadge label="Offen" tone="gray" />
          ) : day.status === "confirmed" ? (
            <StatusBadge label="Bestätigt" tone="green" />
          ) : (
            <StatusBadge label="Bestellt" tone="blue" />
          ),
        stacked: "meta",
      },
      {
        key: "deadline",
        header: "Frist",
        className: "whitespace-nowrap",
        render: (day) =>
          day.closed || day.status === "unordered"
            ? "–"
            : dateTime(day.deadline),
      },
      {
        key: "total",
        header: "Essen",
        render: (day) =>
          day.closed || day.status === "unordered" ? "–" : String(day.total),
        align: "right",
      },
    ];
    if (byClass) {
      result.push({
        key: "classes",
        header: "Je Klasse",
        render: (day) =>
          day.classes.length === 0
            ? "–"
            : day.classes
                .map((count) => `${count.schoolClass || "–"}: ${count.count}`)
                .join(" · "),
      });
    }
    if (named) {
      result.push({
        key: "children",
        header: "Kinder",
        render: (day) => {
          if (day.children.length === 0) return "–";
          const shown = namesShown.has(day.date);
          return (
            <div className="flex flex-col items-start gap-1">
              {shown ? (
                <span>{day.children.map(childLabel).join(", ")}</span>
              ) : null}
              <Button
                type="button"
                variant="ghost"
                size="compact"
                aria-expanded={shown}
                onClick={() => toggleNames(day.date)}
              >
                {shown
                  ? "Namen ausblenden"
                  : `${day.children.length} Namen anzeigen`}
              </Button>
            </div>
          );
        },
      });
    }
    result.push(
      {
        key: "changes",
        header: "Änderungen nach der Frist",
        render: changesLabel,
      },
      {
        key: "confirm",
        header: "Bestätigung",
        render: (day) =>
          day.status === "ordered" && day.orderId && firstDays.has(day.date) ? (
            <Button
              type="button"
              variant="outline"
              size="compact"
              isLoading={confirming === day.orderId}
              loadingText="Wird bestätigt …"
              disabled={confirming !== null}
              onClick={() => void confirm(day.orderId!)}
            >
              Erhalten bestätigen
            </Button>
          ) : (
            "–"
          ),
      },
    );
    return result;
  }, [byClass, confirm, confirming, firstDays, named, namesShown, toggleNames]);

  return (
    <DataTable
      columns={columns}
      rows={days}
      getRowKey={(day) => day.date}
      stackedOnMobile
      caption={`Bestellung von ${school.name} an den gewählten Tagen`}
    />
  );
}
