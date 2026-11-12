"use client";

// Die Preisliste einer Schule (#4684, #6196). Jedes Modul hat eine Preisform:
// „Preis je Wochentag“ (der Monatsbeitrag ist die Summe der gebuchten Tage)
// oder „Staffel nach Tagen je Woche“ (der Monatsbeitrag hängt nur von der
// Zahl der gebuchten Tage ab). Je Preis steht darunter der Preis für jedes
// weitere Kind, wenn das Büro einen eingetragen hat. Ein gebuchter Tag ohne
// Preis ist markiert, denn das Kind kommt sonst nicht in den Monatslauf.

import { ResponsiveActionList } from "~/components/ui/responsive-action-list";
import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { SectionCard } from "~/components/ui/section-card";
import { SectionLabel } from "~/components/ui/section-label";
import { StatusBadge } from "~/components/ui/status-badge";
import type { CareTariffs, CareWeekday } from "~/lib/traeger-care-billing-api";
import {
  type PriceRow,
  PRICE_FORM_LABELS,
  SCALE_STEPS,
  WEEKDAY_LONG,
  WEEKDAY_SHORT,
  contractCountLabel,
  daysLabel,
  endsAtLabel,
  formatMonth,
  freeMonthsAt,
  freeMonthsLabel,
  missingPrices,
  priceRowsAt,
  priceStands,
  shownWeekdays,
  weekdayList,
} from "./care-billing-format";
import { formatEuro } from "./contribution-format";
import { schoolsConcept } from "~/lib/concepts/schools";

/** Whether any row of the list names a sibling price. */
function hasSiblingPrices(rows: readonly PriceRow[]): boolean {
  return rows.some(
    (row) =>
      Object.keys(row.siblingPrices).length > 0 ||
      Object.keys(row.siblingSteps).length > 0,
  );
}

/**
 * A price with the price for every further child under it. A stored sibling
 * price that no rule bills stays visible, muted, so the office can check what
 * is saved; the line above the table says once that it does not apply. From
 * md on the row's label column names the two lines once („1. Kind“ /
 * „ab 2. Kind“); a phone row names the second line in the cell.
 */
function PriceCell({
  cents,
  sibling,
  siblingApplies,
  booked,
}: Readonly<{
  cents?: number;
  sibling?: number;
  siblingApplies: boolean;
  booked: boolean;
}>) {
  if (cents !== undefined) {
    return (
      <span className="inline-flex flex-col items-end leading-tight">
        <span className="text-gray-900 tabular-nums">{formatEuro(cents)}</span>
        {sibling !== undefined && sibling !== cents ? (
          <span
            className={`text-sm tabular-nums ${siblingApplies ? "text-gray-600" : "text-gray-500"}`}
          >
            <span className="md:sr-only">ab 2. Kind </span>
            {formatEuro(sibling)}
            {siblingApplies ? null : (
              <span className="sr-only"> (gilt zurzeit nicht)</span>
            )}
          </span>
        ) : null}
      </span>
    );
  }
  if (booked) {
    return (
      <span className="text-moto-orange-strong font-medium">Preis fehlt</span>
    );
  }
  return (
    <span className="text-gray-400" aria-label="nicht gebucht">
      –
    </span>
  );
}

/** Whether a cell of the row shows a second line for the 2nd child on. */
function rowHasSiblingLine(row: PriceRow): boolean {
  const differs = (
    prices: Partial<Record<string, number>>,
    siblings: Partial<Record<string, number>>,
  ) =>
    Object.entries(siblings).some(
      ([key, cents]) =>
        prices[key] !== undefined &&
        cents !== undefined &&
        cents !== prices[key],
    );
  return (
    differs(row.prices, row.siblingPrices) ||
    differs(row.steps, row.siblingSteps)
  );
}

/**
 * Names the two lines of the price cells once per row, from the tablet on:
 * the cells then carry only amounts and the module keeps its width.
 */
function siblingLabelColumn(): DataTableColumn<PriceRow> {
  return {
    key: "price-for",
    header: "Preis für",
    headerSrOnly: true,
    align: "right",
    stacked: "hidden",
    className: "align-top",
    render: (row) =>
      rowHasSiblingLine(row) ? (
        <span className="inline-flex flex-col items-end leading-tight whitespace-nowrap text-gray-500">
          <span>1. Kind</span>
          <span className="text-sm">ab 2. Kind</span>
        </span>
      ) : null,
  };
}

function moduleColumn(): DataTableColumn<PriceRow> {
  return {
    key: "module",
    header: "Modul",
    stacked: "title",
    // Top-aligned like the price cells, whose second line may be empty.
    className: "align-top",
    render: (row) => (
      <div className="min-w-0">
        <p className="flex flex-wrap items-center gap-x-2 gap-y-1 font-medium text-gray-900">
          {row.moduleName}
          {row.mealExtra ? (
            <StatusBadge
              tone="blue"
              size="xs"
              label="Essen extra"
              title="Der Preis enthält das Essen nicht. Das Essen rechnet die Mensa ab."
            />
          ) : null}
        </p>
        <p className="mt-0.5 text-sm text-gray-600">
          {[
            endsAtLabel(row.endsAt),
            row.contractCount > 0
              ? `${contractCountLabel(row.contractCount)} · ${weekdayList(row.bookedWeekdays)}`
              : "Noch nicht gebucht",
          ]
            .filter(Boolean)
            .join(" · ")}
        </p>
      </div>
    ),
  };
}

function weekdayColumns(
  weekdays: readonly CareWeekday[],
  siblings: boolean,
  labels: boolean,
): DataTableColumn<PriceRow>[] {
  const cell = (row: PriceRow, weekday: CareWeekday) => (
    <PriceCell
      cents={row.prices[weekday]}
      sibling={row.siblingPrices[weekday]}
      siblingApplies={siblings}
      booked={row.bookedWeekdays.includes(weekday)}
    />
  );
  return [
    moduleColumn(),
    ...(labels ? [siblingLabelColumn()] : []),
    ...weekdays.map((weekday): DataTableColumn<PriceRow> => ({
      key: weekday,
      header: WEEKDAY_SHORT[weekday],
      align: "right",
      stacked: "field",
      className: "align-top",
      stackedRender: (row) => (
        <span>
          <span className="text-gray-600">{WEEKDAY_LONG[weekday]}: </span>
          {cell(row, weekday)}
        </span>
      ),
      render: (row) => cell(row, weekday),
    })),
  ];
}

function scaleColumns(
  siblings: boolean,
  labels: boolean,
): DataTableColumn<PriceRow>[] {
  const cell = (row: PriceRow, days: number) => (
    <PriceCell
      cents={row.steps[days]}
      sibling={row.siblingSteps[days]}
      siblingApplies={siblings}
      booked={row.bookedDaysPerWeek.includes(days)}
    />
  );
  return [
    moduleColumn(),
    ...(labels ? [siblingLabelColumn()] : []),
    ...SCALE_STEPS.map((days): DataTableColumn<PriceRow> => ({
      key: `days-${days}`,
      header: daysLabel(days),
      align: "right",
      stacked: "field",
      className: "align-top",
      stackedRender: (row) => (
        <span>
          <span className="text-gray-600">{daysLabel(days)} je Woche: </span>
          {cell(row, days)}
        </span>
      ),
      render: (row) => cell(row, days),
    })),
  ];
}

/** One school's price list at a month, with its earlier and later lists. */
export function CarePriceListCard({
  data,
  school,
  shownMonth,
  currentMonth,
  onShownMonthChange,
  onEdit,
  onDelete,
  siblingPricesApply = true,
}: Readonly<{
  data: CareTariffs;
  school: { id: string; name: string };
  /**
   * Whether a sibling rule bills the sibling prices of the list
   * („Preis laut Preisliste“). Without one they are never charged: the cells
   * show them muted and the line above each table says so once.
   */
  siblingPricesApply?: boolean;
  /** The price list the card shows, "YYYY-MM-01"; empty without one. */
  shownMonth: string;
  currentMonth: string;
  onShownMonthChange: (month: string) => void;
  onEdit: () => void;
  onDelete: (validFrom: string) => void;
}>) {
  const stands = priceStands(data, school.id);
  const month = shownMonth || currentMonth;
  const rows = priceRowsAt(data, school.id, month);
  const weekdayRows = rows.filter((row) => row.form === "weekday");
  const scaleRows = rows.filter((row) => row.form === "days_per_week");
  const weekdays = shownWeekdays(weekdayRows);
  const freeMonths = freeMonthsAt(data, school.id, month);
  const billedUntil = data.billedUntil[school.id];
  const gaps = rows.filter((row) => missingPrices(row).length > 0);
  const deletable = shownMonth !== "";

  const standLabel = (stand: string) => {
    const index = stands.indexOf(stand);
    const next = stands[index + 1];
    const holds = stand <= currentMonth && (!next || next > currentMonth);
    const status = holds
      ? "gilt jetzt"
      : stand > currentMonth
        ? "geplant"
        : "früher";
    return `ab ${formatMonth(stand)} · ${status}`;
  };

  const description = [
    stands.length === 0 ? "Noch keine Preise." : null,
    freeMonths.length > 0
      ? `Im ${freeMonthsLabel(freeMonths)} zahlen Familien keinen Beitrag.`
      : null,
    billedUntil
      ? `Abgerechnet bis ${formatMonth(billedUntil)}. Abgerechnete Preise bleiben, fehlende können Sie ergänzen.`
      : null,
  ]
    .filter(Boolean)
    .join(" ");

  const table = (
    groupRows: PriceRow[],
    columns: DataTableColumn<PriceRow>[],
    label: string,
    basis: string,
  ) =>
    groupRows.length === 0 ? null : (
      <div className="space-y-2">
        {weekdayRows.length > 0 && scaleRows.length > 0 ? (
          <SectionLabel as="h3">{label}</SectionLabel>
        ) : null}
        {/* What the amounts are stays right above them, whatever the card
            says about free months or billing; so does whether the prices
            from the 2nd child on apply. */}
        <p className="text-sm text-gray-600">
          {basis}
          {!siblingPricesApply && hasSiblingPrices(groupRows)
            ? " Die Preise ab dem 2. Kind gelten zurzeit nicht: Für Geschwister gilt die Ermäßigung unter „Ermäßigungen“."
            : null}
        </p>
        <DataTable
          columns={columns}
          rows={groupRows}
          getRowKey={(row) => row.careOfferingId}
          // Five or more price columns: narrower cells keep the price from
          // the 2nd child on one line from the laptop on.
          dense
          stackedOnMobile
        />
      </div>
    );

  return (
    <SectionCard
      concept={schoolsConcept}
      title={school.name}
      description={description || undefined}
      actions={
        <div className="flex items-center gap-2">
          <Button type="button" variant="outline" size="md" onClick={onEdit}>
            Neue Preise
          </Button>
          {stands.length > 0 ? (
            <OverflowMenu
              ariaLabel={`Weitere Aktionen: ${school.name}`}
              items={[
                {
                  label: shownMonth
                    ? `Preise ab ${formatMonth(shownMonth)} löschen`
                    : "Preise löschen",
                  destructive: true,
                  disabled: !deletable,
                  onClick: () => onDelete(shownMonth),
                },
              ]}
            />
          ) : null}
        </div>
      }
    >
      <div className="space-y-5">
        {stands.length > 1 ? (
          <div className="max-w-xs">
            <CustomSelect
              id={`care-price-stand-${school.id}`}
              label="Preise anzeigen"
              value={shownMonth}
              options={[...stands]
                .reverse()
                .map((stand) => ({ value: stand, label: standLabel(stand) }))}
              onChange={onShownMonthChange}
            />
          </div>
        ) : stands.length === 1 && shownMonth ? (
          <p className="text-sm text-gray-600">{standLabel(shownMonth)}</p>
        ) : null}
        {rows.length === 0 ? (
          <p className="text-sm text-gray-600">
            Noch keine Module. Module kommen aus den gültigen Verträgen der
            Schule.
          </p>
        ) : (
          <>
            {table(
              weekdayRows,
              weekdayColumns(
                weekdays,
                siblingPricesApply,
                weekdayRows.some(rowHasSiblingLine),
              ),
              PRICE_FORM_LABELS.weekday,
              "Jeder Betrag ist der Monatspreis für einen gebuchten Wochentag.",
            )}
            {table(
              scaleRows,
              scaleColumns(
                siblingPricesApply,
                scaleRows.some(rowHasSiblingLine),
              ),
              PRICE_FORM_LABELS.days_per_week,
              "Jeder Betrag ist der Monatspreis bei so vielen gebuchten Tagen je Woche.",
            )}
          </>
        )}
        {gaps.length > 0 ? (
          <ResponsiveActionList>
            <li className="bg-moto-orange/10 flex flex-col gap-2 rounded-xl px-4 py-3 sm:col-span-2 sm:grid sm:grid-cols-subgrid sm:items-center">
              <p className="text-sm text-gray-900">
                {gaps.length === 1
                  ? `„${gaps[0]?.moduleName}“ hat gebuchte Tage ohne Preis.`
                  : `${gaps.length} Module haben gebuchte Tage ohne Preis.`}{" "}
                Diese Kinder kommen erst in den Monatslauf, wenn jeder Tag einen
                Preis hat.
              </p>
              <div className="flex sm:justify-end">
                <Button
                  type="button"
                  variant="surface"
                  size="compact"
                  onClick={onEdit}
                  aria-label={`Preise ergänzen: ${school.name}`}
                >
                  Preise ergänzen
                </Button>
              </div>
            </li>
          </ResponsiveActionList>
        ) : null}
      </div>
    </SectionCard>
  );
}
