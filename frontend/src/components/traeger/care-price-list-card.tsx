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

/** A price with the price for every further child under it. */
function PriceCell({
  cents,
  sibling,
  booked,
}: Readonly<{ cents?: number; sibling?: number; booked: boolean }>) {
  if (cents !== undefined) {
    return (
      <span className="inline-flex flex-col items-end leading-tight">
        <span className="text-gray-900 tabular-nums">{formatEuro(cents)}</span>
        {sibling !== undefined && sibling !== cents ? (
          <span className="text-xs text-gray-600 tabular-nums">
            ab 2. Kind {formatEuro(sibling)}
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

function moduleColumn(): DataTableColumn<PriceRow> {
  return {
    key: "module",
    header: "Modul",
    stacked: "title",
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
        <p className="mt-0.5 text-xs text-gray-600">
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
): DataTableColumn<PriceRow>[] {
  const cell = (row: PriceRow, weekday: CareWeekday) => (
    <PriceCell
      cents={row.prices[weekday]}
      sibling={row.siblingPrices[weekday]}
      booked={row.bookedWeekdays.includes(weekday)}
    />
  );
  return [
    moduleColumn(),
    ...weekdays.map((weekday): DataTableColumn<PriceRow> => ({
      key: weekday,
      header: WEEKDAY_SHORT[weekday],
      align: "right",
      stacked: "field",
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

function scaleColumns(): DataTableColumn<PriceRow>[] {
  const cell = (row: PriceRow, days: number) => (
    <PriceCell
      cents={row.steps[days]}
      sibling={row.siblingSteps[days]}
      booked={row.bookedDaysPerWeek.includes(days)}
    />
  );
  return [
    moduleColumn(),
    ...SCALE_STEPS.map((days): DataTableColumn<PriceRow> => ({
      key: `days-${days}`,
      header: daysLabel(days),
      align: "right",
      stacked: "field",
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
}: Readonly<{
  data: CareTariffs;
  school: { id: string; name: string };
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
      ? `Abgerechnet bis ${formatMonth(billedUntil)}. Preise, mit denen schon abgerechnet ist, bleiben. Fehlende Preise können Sie ergänzen.`
      : null,
  ]
    .filter(Boolean)
    .join(" ");

  const table = (
    groupRows: PriceRow[],
    columns: DataTableColumn<PriceRow>[],
    label: string,
  ) =>
    groupRows.length === 0 ? null : (
      <div className="space-y-2">
        {weekdayRows.length > 0 && scaleRows.length > 0 ? (
          <SectionLabel as="h3">{label}</SectionLabel>
        ) : null}
        <DataTable
          columns={columns}
          rows={groupRows}
          getRowKey={(row) => row.careOfferingId}
          stackedOnMobile
        />
      </div>
    );

  return (
    <SectionCard
      title={school.name}
      description={
        description ||
        (scaleRows.length > 0 && weekdayRows.length === 0
          ? "Monatspreis nach Zahl der gebuchten Tage je Woche."
          : "Monatspreis je gebuchtem Wochentag.")
      }
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
              weekdayColumns(weekdays),
              PRICE_FORM_LABELS.weekday,
            )}
            {table(scaleRows, scaleColumns(), PRICE_FORM_LABELS.days_per_week)}
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
