"use client";

import { useCallback, useMemo, useState } from "react";

import { Button } from "~/components/ui/button";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { StatusBadge } from "~/components/ui/status-badge";
import { useApiErrorDisplay } from "~/contexts/ToastContext";
import { schoolClassLabel } from "~/lib/school-class-label";
import {
  confirmCatererOrder,
  type CatererOrderChild,
  type CatererOrderDay,
  type CatererOrders,
  type CatererSchool,
} from "~/lib/caterer-api";
import {
  berlinClockFromISO,
  berlinTodayISO,
  parseISODate,
} from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "CatererSchoolOrders" });

function shortDay(date: string): string {
  return parseISODate(date).toLocaleDateString("de-DE", {
    weekday: "short",
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

/** Die Frist eines Tages; am selben Tag genügt die Uhrzeit. */
function deadlineLabel(day: CatererOrderDay): string {
  return berlinTodayISO(new Date(day.deadline)) === day.date
    ? berlinClockFromISO(day.deadline)
    : dateTime(day.deadline);
}

function classLabel(schoolClass: string): string {
  return schoolClassLabel(schoolClass) || "Ohne Klasse";
}

function childLabel(child: CatererOrderChild): string {
  const name = `${child.firstName} ${child.lastName}`.trim();
  if (!name) return child.schoolClass;
  return child.schoolClass ? `${name} (${child.schoolClass})` : name;
}

/** Ein Tag, an dem nichts geliefert wird, zeigt keine Zahl. */
function delivers(day: CatererOrderDay | undefined): day is CatererOrderDay {
  return Boolean(day && !day.closed && day.status !== "unordered");
}

/** Die Kinder, die nach der Frist dazukamen oder abgemeldet wurden. */
function changeCounts(
  day: CatererOrderDay,
  schoolClass?: string,
): { added: number; removed: number } {
  let added = 0;
  let removed = 0;
  for (const change of day.changes) {
    if (schoolClass !== undefined && change.child.schoolClass !== schoolClass)
      continue;
    if (change.delta > 0) added += change.delta;
    else removed -= change.delta;
  }
  return { added, removed };
}

/** Eine Zahl mit den Nachmeldungen daneben, wie in der Übersicht. */
function CountWithChanges({
  count,
  added,
  removed,
  strong = false,
}: Readonly<{
  count: number;
  added: number;
  removed: number;
  strong?: boolean;
}>) {
  return (
    <span className="inline-flex items-center justify-end gap-1 tabular-nums">
      <span className={strong ? "font-semibold text-gray-900" : undefined}>
        {count}
      </span>
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
}

/** Where one day stands, as one badge; an open day names its deadline. */
function DayState({ day }: Readonly<{ day: CatererOrderDay }>) {
  if (day.closed) return <StatusBadge label="Schule geschlossen" tone="gray" />;
  if (day.status === "unordered")
    return <StatusBadge label="Nicht bestellt" tone="gray" />;
  if (day.status === "open")
    return (
      <span className="inline-flex flex-col items-end gap-1">
        <StatusBadge label="Offen" tone="gray" />
        <span className="text-xs whitespace-nowrap text-gray-600">
          bis {deadlineLabel(day)}
        </span>
      </span>
    );
  if (day.status === "confirmed")
    return <StatusBadge label="Bestätigt" tone="green" />;
  return <StatusBadge label="Bestellt" tone="blue" />;
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

/** Klassen in Schulreihenfolge: 1a, 1b, 2a … 10a. */
function classesOf(days: readonly CatererOrderDay[]): string[] {
  const found = new Set<string>();
  for (const day of days) {
    for (const count of day.classes) found.add(count.schoolClass);
    for (const change of day.changes) found.add(change.child.schoolClass);
  }
  return [...found].sort((a, b) => a.localeCompare(b, "de", { numeric: true }));
}

type OrderRow =
  | { key: string; kind: "state" }
  | { key: string; kind: "class"; schoolClass: string }
  | { key: string; kind: "total" }
  | { key: string; kind: "names" };

/**
 * One school's orders of the chosen days, as its caterer reads them (ADR
 * 0074), in the grid of the overview above: a column per day, a row per
 * class when the school orders by class, the sum below. Changes after the
 * deadline are counted per class as „+2“/„−1“ beside the number instead of
 * listed child by child. The first row says where each day stands; a fixed
 * order is confirmed there once. Names appear only while the school shows
 * them and someone opens them for a day.
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
  // Die Namen eines Tages sind zugeklappt: ausgeschrieben machen sie die
  // Spalte so hoch wie die Kinderliste und verdrängen die Zahlen.
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
  const named = days.some(
    (day) =>
      day.children.length > 0 ||
      day.changes.some((change) => change.child.firstName !== ""),
  );
  const firstDays = useMemo(() => firstDaysOfOrders(days), [days]);
  const classes = useMemo(
    () => (byClass ? classesOf(days) : []),
    [byClass, days],
  );

  const rows = useMemo<OrderRow[]>(
    () => [
      { key: "state", kind: "state" },
      ...classes.map((schoolClass): OrderRow => ({
        key: `class-${schoolClass}`,
        kind: "class",
        schoolClass,
      })),
      { key: "total", kind: "total" },
      ...(named ? [{ key: "names", kind: "names" } as const] : []),
    ],
    [classes, named],
  );

  const columns = useMemo<DataTableColumn<OrderRow>[]>(() => {
    const label = (row: OrderRow) => {
      if (row.kind === "state") return "Stand";
      if (row.kind === "class") return classLabel(row.schoolClass);
      if (row.kind === "names") return "Namen";
      return byClass ? "Essen gesamt" : "Essen";
    };
    const cell = (row: OrderRow, day: CatererOrderDay) => {
      if (row.kind === "state") {
        return day.status === "ordered" &&
          day.orderId &&
          firstDays.has(day.date) ? (
          <Button
            type="button"
            variant="outline"
            size="md"
            className="whitespace-nowrap"
            isLoading={confirming === day.orderId}
            loadingText="Wird bestätigt …"
            disabled={confirming !== null}
            onClick={() => void confirm(day.orderId!)}
          >
            Erhalten bestätigen
          </Button>
        ) : (
          <DayState day={day} />
        );
      }
      if (!delivers(day)) return "–";
      if (row.kind === "class") {
        const count =
          day.classes.find((entry) => entry.schoolClass === row.schoolClass)
            ?.count ?? 0;
        return (
          <CountWithChanges
            count={count}
            {...changeCounts(day, row.schoolClass)}
          />
        );
      }
      if (row.kind === "total") {
        return (
          <CountWithChanges count={day.total} {...changeCounts(day)} strong />
        );
      }
      return (
        <DayNames
          day={day}
          shown={namesShown.has(day.date)}
          onToggle={() => toggleNames(day.date)}
        />
      );
    };
    const sum = (row: OrderRow) => {
      if (row.kind === "state" || row.kind === "names") return null;
      const total = days
        .filter(delivers)
        .reduce(
          (result, day) =>
            result +
            (row.kind === "total"
              ? day.total
              : (day.classes.find(
                  (entry) => entry.schoolClass === row.schoolClass,
                )?.count ?? 0)),
          0,
        );
      return (
        <span
          className={`tabular-nums ${row.kind === "total" ? "font-semibold text-gray-900" : ""}`}
        >
          {total}
        </span>
      );
    };
    return [
      {
        key: "row",
        header: "",
        stacked: "title",
        render: (row) => (
          <span
            className={
              row.kind === "total"
                ? "font-semibold text-gray-900"
                : "font-medium text-gray-900"
            }
          >
            {label(row)}
          </span>
        ),
      },
      ...days.map((day): DataTableColumn<OrderRow> => ({
        key: day.date,
        header: shortDay(day.date),
        align: "right",
        render: (row) => cell(row, day),
      })),
      ...(days.length > 1
        ? [
            {
              key: "sum",
              header: "Summe",
              align: "right",
              render: sum,
            } satisfies DataTableColumn<OrderRow>,
          ]
        : []),
    ];
  }, [byClass, confirm, confirming, days, firstDays, namesShown, toggleNames]);

  return (
    <DataTable
      columns={columns}
      rows={rows}
      getRowKey={(row) => row.key}
      rowClassName={(row) => (row.kind === "total" ? "bg-gray-50" : "")}
      stackedOnMobile
      caption={`Bestellung von ${school.name} an den gewählten Tagen`}
    />
  );
}

/**
 * Die Namen eines Tages, zugeklappt hinter einem Knopf: die bestellten
 * Kinder, darunter die Nachmeldungen mit +1/−1.
 */
function DayNames({
  day,
  shown,
  onToggle,
}: Readonly<{ day: CatererOrderDay; shown: boolean; onToggle: () => void }>) {
  const changes = day.changes.filter((change) => change.child.firstName !== "");
  const count = day.children.length + changes.length;
  if (count === 0) return "–";
  return (
    <div className="flex flex-col items-end gap-1 text-right">
      <Button
        type="button"
        variant="ghost"
        size="compact"
        aria-expanded={shown}
        onClick={onToggle}
      >
        {shown ? "Namen ausblenden" : `${count} Namen anzeigen`}
      </Button>
      {shown ? (
        <ul className="space-y-0.5 text-sm text-gray-700">
          {day.children.map((child) => (
            <li
              key={`${child.lastName}-${child.firstName}-${child.schoolClass}`}
            >
              {childLabel(child)}
            </li>
          ))}
          {changes.map((change) => (
            <li key={change.sequence} className="text-moto-orange-strong">
              {`${change.delta > 0 ? "+1" : "−1"} ${childLabel(change.child)}`}
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  );
}
