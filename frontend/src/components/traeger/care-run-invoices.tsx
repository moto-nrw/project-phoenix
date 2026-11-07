"use client";

// Die Rechnungen eines Monatslaufs (#4684), je Schule eine Karte. Jede
// Rechnung zeigt das Kind, den Empfänger und je gebuchtem Modul eine
// Position mit den Wochentagen und ihrem Monatspreis. Es gibt keine Zeile je
// Kalendertag.

import { ResponsiveActionList } from "~/components/ui/responsive-action-list";
import { ButtonLink } from "~/components/ui/button";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { SectionCard } from "~/components/ui/section-card";
import { formatDate } from "~/lib/date-helpers";
import type {
  CareInvoice,
  CarePosition,
  CareUnpriced,
} from "~/lib/traeger-care-billing-api";
import { traegerPath } from "~/lib/traeger-url";
import {
  WEEKDAY_SHORT,
  WEEKDAY_LONG,
  formatMonth,
  invoiceCountLabel,
} from "./care-billing-format";
import { formatEuro } from "./contribution-format";

interface SchoolGroup {
  readonly schoolId: string;
  readonly schoolName: string;
  readonly invoices: CareInvoice[];
  readonly totalCents: number;
}

/** The invoices by school, in the order the run lists them. */
function bySchool(invoices: readonly CareInvoice[]): SchoolGroup[] {
  const groups = new Map<string, SchoolGroup>();
  for (const invoice of invoices) {
    const group = groups.get(invoice.schoolId) ?? {
      schoolId: invoice.schoolId,
      schoolName: invoice.schoolName,
      invoices: [],
      totalCents: 0,
    };
    groups.set(invoice.schoolId, {
      ...group,
      invoices: [...group.invoices, invoice],
      totalCents: group.totalCents + invoice.totalCents,
    });
  }
  return [...groups.values()];
}

/** The days a position bills when they are not the whole month. */
function positionDays(position: CarePosition, month: string): string | null {
  const whole =
    position.serviceFrom === month &&
    position.serviceUntil.slice(0, 7) === month.slice(0, 7) &&
    position.weekdays.every((day) => day.amountCents === day.monthlyCents);
  if (whole) return null;
  return `${formatDate(position.serviceFrom)} bis ${formatDate(position.serviceUntil)}`;
}

function PositionLine({
  position,
  month,
  showAmount,
}: Readonly<{ position: CarePosition; month: string; showAmount: boolean }>) {
  const days = positionDays(position, month);
  return (
    <li className="flex items-baseline justify-between gap-4 py-1.5">
      <div className="min-w-0">
        <p className="text-sm text-gray-900">{position.moduleName}</p>
        <p className="mt-0.5 flex flex-wrap gap-x-3 gap-y-0.5 text-xs text-gray-600">
          {position.weekdays.map((day) => (
            <span key={day.weekday} title={WEEKDAY_LONG[day.weekday]}>
              <span className="font-medium text-gray-700">
                {WEEKDAY_SHORT[day.weekday]}
              </span>{" "}
              <span className="tabular-nums">
                {formatEuro(day.amountCents)}
              </span>
              {day.amountCents === day.monthlyCents ? null : (
                <span className="text-gray-500">
                  {" "}
                  (von {formatEuro(day.monthlyCents)})
                </span>
              )}
            </span>
          ))}
          {days ? <span className="text-gray-500">{days}</span> : null}
        </p>
      </div>
      {showAmount ? (
        <span className="shrink-0 text-sm text-gray-700 tabular-nums">
          {formatEuro(position.amountCents)}
        </span>
      ) : null}
    </li>
  );
}

function invoiceColumns(month: string): DataTableColumn<CareInvoice>[] {
  return [
    {
      key: "child",
      header: "Kind",
      stacked: "title",
      className: "w-56 align-top",
      render: (invoice) => {
        const recipient =
          `${invoice.recipient.firstName} ${invoice.recipient.lastName}`.trim();
        return (
          <div className="min-w-0">
            <p className="font-semibold text-gray-900">
              {invoice.childFirstName} {invoice.childLastName}
            </p>
            <p className="mt-0.5 truncate text-sm text-gray-600">
              {recipient ? `An ${recipient}` : "Ohne Empfänger"}
            </p>
            {invoice.number ? (
              <p className="mt-0.5 font-mono text-xs text-gray-500">
                {invoice.number}
              </p>
            ) : null}
          </div>
        );
      },
    },
    {
      key: "positions",
      header: "Positionen",
      stacked: "field",
      stackedLabel: false,
      className: "align-top",
      render: (invoice) => (
        <ul className="min-w-0 divide-y divide-gray-100">
          {invoice.positions.map((position) => (
            <PositionLine
              key={`${position.careOfferingId}-${position.serviceFrom}`}
              position={position}
              month={month}
              showAmount={invoice.positions.length > 1}
            />
          ))}
        </ul>
      ),
    },
    {
      key: "total",
      header: "Betrag",
      align: "right",
      stacked: "meta",
      className: "align-top",
      render: (invoice) => (
        <span className="font-semibold text-gray-900 tabular-nums">
          {formatEuro(invoice.totalCents)}
        </span>
      ),
    },
  ];
}

/** One card per school with its invoices and its sum. */
export function CareRunInvoices({
  invoices,
  month,
  released,
}: Readonly<{
  invoices: readonly CareInvoice[];
  month: string;
  released: boolean;
}>) {
  if (invoices.length === 0) {
    return (
      <SectionCard
        title="Rechnungen"
        description={`Für ${formatMonth(month)} gibt es nichts abzurechnen. Es zählen Verträge, die in diesem Monat gelten und deren Wochentage einen Preis haben.`}
      />
    );
  }
  return (
    <>
      {bySchool(invoices).map((group) => (
        <SectionCard
          key={group.schoolId}
          title={group.schoolName}
          description={
            released
              ? "Ausgestellte Rechnungen. Sie ändern sich nicht mehr."
              : "Vorschau. Jede Rechnung bekommt ihre Nummer erst bei der Freigabe."
          }
          titleBadge={
            <span className="text-sm font-normal text-gray-600 tabular-nums">
              {invoiceCountLabel(group.invoices.length)} ·{" "}
              {formatEuro(group.totalCents)}
            </span>
          }
          collapsible={invoices.length > 8}
        >
          <DataTable
            columns={invoiceColumns(month)}
            rows={group.invoices}
            getRowKey={(invoice) => invoice.contractId}
            stackedOnMobile
          />
        </SectionCard>
      ))}
    </>
  );
}

/**
 * The children the run leaves out because a booked weekday has no price,
 * with the way to the price list of their school.
 */
export function CareUnpricedCard({
  unpriced,
}: Readonly<{ unpriced: readonly CareUnpriced[] }>) {
  if (unpriced.length === 0) return null;
  const children = new Map<string, CareUnpriced[]>();
  for (const entry of unpriced) {
    children.set(entry.contractId, [
      ...(children.get(entry.contractId) ?? []),
      entry,
    ]);
  }
  return (
    <SectionCard
      title="Ohne Preis"
      titleBadge={
        <span className="text-moto-orange-strong text-sm font-medium">
          {children.size === 1 ? "1 Kind" : `${children.size} Kinder`}
        </span>
      }
      description="Diese Kinder kommen erst in den Lauf, wenn jeder gebuchte Wochentag einen Preis hat."
    >
      <ResponsiveActionList>
        {[...children.values()].map((entries) => {
          const first = entries[0];
          if (!first) return null;
          return (
            <li
              key={first.contractId}
              className="flex flex-col gap-2 py-3 first:pt-0 last:pb-0 sm:col-span-2 sm:grid sm:grid-cols-subgrid sm:items-center"
            >
              <div className="min-w-0">
                <p className="text-sm font-semibold text-gray-900">
                  {first.childFirstName} {first.childLastName}
                </p>
                <p className="mt-0.5 text-sm text-gray-600">
                  {first.schoolName} ·{" "}
                  {entries
                    .map(
                      (entry) =>
                        `${entry.moduleName} am ${WEEKDAY_LONG[entry.weekday]}`,
                    )
                    .join(", ")}
                </p>
              </div>
              <div className="flex w-full flex-col gap-2 min-[480px]:w-auto min-[480px]:flex-row sm:justify-end">
                <ButtonLink
                  href={traegerPath(
                    `/traeger/abrechnung/preise?schule=${encodeURIComponent(first.schoolId)}`,
                  )}
                  variant="surface"
                  size="compact"
                  aria-label={`Preis eintragen: ${first.schoolName}`}
                >
                  Preis eintragen
                </ButtonLink>
              </div>
            </li>
          );
        })}
      </ResponsiveActionList>
    </SectionCard>
  );
}
