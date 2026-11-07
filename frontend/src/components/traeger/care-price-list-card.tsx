"use client";

// Die Preisliste einer Schule (#4684): je Modul eine Zeile, je Wochentag der
// Monatspreis, der im gewählten Monat gilt. Ein gebuchter Wochentag ohne
// Preis ist markiert, denn das Kind kommt sonst nicht in den Monatslauf.

import { ResponsiveActionList } from "~/components/ui/responsive-action-list";
import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { SectionCard } from "~/components/ui/section-card";
import type { CareTariffs, CareWeekday } from "~/lib/traeger-care-billing-api";
import {
  type PriceRow,
  WEEKDAY_LONG,
  WEEKDAY_SHORT,
  contractCountLabel,
  endsAtLabel,
  formatMonth,
  missingPrices,
  priceRowsAt,
  priceStands,
  shownWeekdays,
  weekdayList,
} from "./care-billing-format";
import { formatEuro } from "./contribution-format";

function priceCell(row: PriceRow, weekday: CareWeekday) {
  const cents = row.prices[weekday];
  if (cents !== undefined) {
    return (
      <span className="text-gray-900 tabular-nums">{formatEuro(cents)}</span>
    );
  }
  if (row.bookedWeekdays.includes(weekday)) {
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

function columns(
  weekdays: readonly CareWeekday[],
): DataTableColumn<PriceRow>[] {
  return [
    {
      key: "module",
      header: "Modul",
      stacked: "title",
      render: (row) => (
        <div className="min-w-0">
          <p className="font-medium text-gray-900">{row.moduleName}</p>
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
    },
    ...weekdays.map((weekday): DataTableColumn<PriceRow> => ({
      key: weekday,
      header: WEEKDAY_SHORT[weekday],
      align: "right",
      stacked: "field",
      stackedRender: (row) => (
        <span>
          <span className="text-gray-600">{WEEKDAY_LONG[weekday]}: </span>
          {priceCell(row, weekday)}
        </span>
      ),
      render: (row) => priceCell(row, weekday),
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
  const rows = priceRowsAt(data, school.id, shownMonth || currentMonth);
  const weekdays = shownWeekdays(rows);
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
    billedUntil
      ? `Abgerechnet bis ${formatMonth(billedUntil)}. Preise, mit denen schon abgerechnet ist, bleiben. Fehlende Preise können Sie ergänzen.`
      : null,
  ]
    .filter(Boolean)
    .join(" ");

  return (
    <SectionCard
      title={school.name}
      description={description || "Monatspreis je gebuchtem Wochentag."}
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
      <div className="space-y-4">
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
          <DataTable
            columns={columns(weekdays)}
            rows={rows}
            getRowKey={(row) => row.careOfferingId}
            stackedOnMobile
          />
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
