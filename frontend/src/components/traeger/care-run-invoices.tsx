"use client";

// Die Rechnungen eines Monatslaufs (#4684), je Schule eine Karte. Jede
// Rechnung zeigt das Kind, den Empfänger und je gebuchtem Modul eine
// Position mit den Wochentagen und ihrem Monatspreis. Es gibt keine Zeile je
// Kalendertag. Mit den Schreiben des Laufs steht ein Storno als eigene Zeile
// mit Minusbetrag direkt unter seiner Rechnung. Die Spalten für Versand und
// Menü kommen nur auf der Seite „Schreiben“ dazu
// (care-run-letter-invoices.tsx), damit der Lauf ihren Code nicht lädt.

import { ResponsiveActionList } from "~/components/ui/responsive-action-list";
import { ButtonLink } from "~/components/ui/button";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { formatDate } from "~/lib/date-helpers";
import type {
  CareDuplicate,
  CareInvoice,
  CareLetterMailing,
  CarePosition,
  CareReduction,
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
import { reductionExplanation } from "./care-reduction-format";

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

/** A reduction of the chain (#4685): rule, explanation, negative amount. */
function ReductionLine({ reduction }: Readonly<{ reduction: CareReduction }>) {
  return (
    <li className="flex items-baseline justify-between gap-4 py-1.5">
      <div className="min-w-0">
        <p className="text-sm text-gray-900">{reduction.name}</p>
        <p className="mt-0.5 text-xs text-gray-600">
          {reductionExplanation(reduction)}
        </p>
      </div>
      <span className="text-moto-green-strong shrink-0 text-sm tabular-nums">
        {formatEuro(reduction.amountCents)}
      </span>
    </li>
  );
}

/** An invoice of the table, or the Storno of the invoice above it. */
export interface CareInvoiceRow {
  readonly key: string;
  readonly invoice: CareInvoice;
  /** The letter of this row once the run has its letters. */
  readonly letter: CareLetterMailing | null;
  readonly storno: boolean;
}

export function careLetterKey(letter: CareLetterMailing): string {
  return `${letter.kind}-${letter.invoiceId}`;
}

/** The rows of a school: each invoice, its Storno directly below it. */
function invoiceRows(
  invoices: readonly CareInvoice[],
  letters: readonly CareLetterMailing[] | undefined,
): CareInvoiceRow[] {
  const byKey = new Map(
    (letters ?? []).map((letter) => [careLetterKey(letter), letter]),
  );
  return invoices.flatMap((invoice) => {
    const letter = byKey.get(`invoice-${invoice.id}`) ?? null;
    const storno = byKey.get(`cancellation-${invoice.id}`) ?? null;
    const rows: CareInvoiceRow[] = [
      { key: invoice.contractId, invoice, letter, storno: false },
    ];
    if (storno) {
      rows.push({
        key: `storno-${invoice.id}`,
        invoice,
        letter: storno,
        storno: true,
      });
    }
    return rows;
  });
}

function invoiceColumns(month: string): DataTableColumn<CareInvoiceRow>[] {
  return [
    {
      key: "child",
      header: "Kind",
      stacked: "title",
      className: "w-56 align-top",
      render: ({ invoice, letter, storno }) => {
        const recipient =
          `${invoice.recipient.firstName} ${invoice.recipient.lastName}`.trim();
        const number = storno ? letter?.number : invoice.number;
        const issuedOn = storno ? letter?.issuedOn : invoice.issuedOn;
        return (
          <div className="min-w-0">
            <p className="font-semibold text-gray-900">
              {invoice.childFirstName} {invoice.childLastName}
            </p>
            <p className="mt-0.5 truncate text-sm text-gray-600">
              {recipient ? `An ${recipient}` : "Ohne Empfänger"}
            </p>
            {number ? (
              <p className="mt-1 flex flex-wrap items-center gap-1.5">
                <span className="font-mono text-xs text-gray-500">
                  {number}
                </span>
                {storno ? <StatusBadge label="Storno" tone="blue" /> : null}
                {!storno && letter?.cancelled ? (
                  <StatusBadge label="Storniert" tone="red" />
                ) : null}
              </p>
            ) : null}
            {issuedOn ? (
              <p className="mt-0.5 text-xs text-gray-500">
                Ausgestellt am {formatDate(issuedOn)}
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
      render: ({ invoice, letter, storno }) =>
        storno ? (
          <div className="min-w-0 space-y-0.5 py-1.5 text-sm">
            <p className="text-gray-900">
              Storno zu Rechnung{" "}
              <span className="font-mono">{invoice.number}</span>
            </p>
            {letter?.reason ? (
              <p className="wrap-anywhere text-gray-600">
                Grund: {letter.reason}
              </p>
            ) : null}
          </div>
        ) : (
          <ul className="min-w-0 divide-y divide-gray-100">
            {invoice.positions.map((position) => (
              <PositionLine
                key={`${position.careOfferingId}-${position.serviceFrom}`}
                position={position}
                month={month}
                showAmount={
                  invoice.positions.length > 1 || invoice.reductions.length > 0
                }
              />
            ))}
            {invoice.reductions.map((reduction, index) => (
              <ReductionLine
                key={`${reduction.kind}-${index}`}
                reduction={reduction}
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
      render: ({ invoice, letter, storno }) => (
        <span
          className={`font-semibold tabular-nums ${!storno && letter?.cancelled ? "text-gray-500 line-through" : "text-gray-900"}`}
        >
          {formatEuro(
            storno && letter ? letter.totalCents : invoice.totalCents,
          )}
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
  letters,
  letterColumns,
}: Readonly<{
  invoices: readonly CareInvoice[];
  month: string;
  released: boolean;
  /** The letters of a released run, as `GET …/letters` lists them: they add the Storno rows. */
  letters?: readonly CareLetterMailing[];
  /** „An die Familie“ and the row menu, only on the page „Schreiben“. */
  letterColumns?: readonly DataTableColumn<CareInvoiceRow>[];
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
          overflow={letterColumns ? "visible" : undefined}
          description={
            !released
              ? "Vorschau. Jede Rechnung bekommt ihre Nummer erst bei der Freigabe."
              : letterColumns
                ? "Ausgestellte Rechnungen. Sie ändern sich nicht mehr. „An die Familie“ zeigt, ob das Schreiben schon gesendet ist."
                : "Ausgestellte Rechnungen. Sie ändern sich nicht mehr."
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
            columns={[...invoiceColumns(month), ...(letterColumns ?? [])]}
            rows={invoiceRows(group.invoices, letters)}
            getRowKey={(row) => row.key}
            rowHasInteractiveControls={letterColumns !== undefined}
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

/** "Kurzbetreuung: Montag, Dienstag · Frühbetreuung: Freitag". */
function bookedTwice(entries: readonly CareDuplicate[]): string {
  const days = new Map<string, string[]>();
  for (const entry of entries) {
    if (!entry.moduleName) continue;
    const weekday = WEEKDAY_LONG[entry.weekday];
    const known = days.get(entry.moduleName) ?? [];
    if (!known.includes(weekday)) {
      days.set(entry.moduleName, [...known, weekday]);
    }
  }
  return [...days.entries()]
    .map(([module, weekdays]) => `${module}: ${weekdays.join(", ")}`)
    .join(" · ");
}

type DuplicateContract = CareDuplicate["contracts"][number];

interface DuplicateChild {
  readonly key: string;
  readonly entries: CareDuplicate[];
  readonly contracts: DuplicateContract[];
}

/**
 * The entries by child: contracts that meet in an entry, directly or
 * through a third one, are one child, with each contract once.
 */
function byChild(duplicates: readonly CareDuplicate[]): DuplicateChild[] {
  const parent = new Map<string, string>();
  const root = (id: string): string => {
    let current = id;
    while (parent.get(current) !== current) {
      current = parent.get(current) ?? current;
    }
    return current;
  };
  for (const entry of duplicates) {
    for (const contract of entry.contracts) {
      if (!parent.has(contract.contractId)) {
        parent.set(contract.contractId, contract.contractId);
      }
    }
    const [first, ...rest] = entry.contracts;
    if (!first) continue;
    for (const contract of rest) {
      parent.set(root(contract.contractId), root(first.contractId));
    }
  }
  const children = new Map<string, DuplicateChild>();
  for (const entry of duplicates) {
    const first = entry.contracts[0];
    if (!first) continue;
    const key = root(first.contractId);
    const child = children.get(key) ?? { key, entries: [], contracts: [] };
    child.entries.push(entry);
    for (const contract of entry.contracts) {
      if (!child.contracts.some((c) => c.contractId === contract.contractId)) {
        child.contracts.push(contract);
      }
    }
    children.set(key, child);
  }
  return [...children.values()];
}

/**
 * The children the run holds back for a duplicate contract, each with all
 * its contracts (ADR 0158, Nachtrag Kind-Monat). A preview names the module
 * and weekdays billed twice; a released run shows the children its release
 * held back, as stored then. Each contract opens from here.
 */
export function CareDuplicatesCard({
  duplicates,
  released = false,
}: Readonly<{ duplicates: readonly CareDuplicate[]; released?: boolean }>) {
  if (duplicates.length === 0) return null;
  const children = byChild(duplicates);
  return (
    <SectionCard
      title={
        released
          ? "Nicht abgerechnet – Doppelter Vertrag"
          : "Doppelter Vertrag – bitte prüfen"
      }
      titleBadge={
        <span className="text-moto-orange-strong text-sm font-medium">
          {children.length === 1 ? "1 Kind" : `${children.length} Kinder`}
        </span>
      }
      description={
        released
          ? "Diese Kinder haben zwei gültige Verträge für dasselbe Modul am selben Tag. Sie sind in diesem Lauf nicht abgerechnet. Beenden oder ändern Sie einen der Verträge. Danach rechnet ein neuer Monatslauf das Kind für diesen Monat ab."
          : "Diese Kinder haben zwei gültige Verträge für dasselbe Modul am selben Tag. Die Freigabe rechnet sie nicht ab, alle anderen Kinder schon. Beenden oder ändern Sie einen der Verträge. Danach rechnet ein neuer Monatslauf das Kind ab."
      }
    >
      <ResponsiveActionList>
        {children.map(({ key, entries, contracts }) => {
          const first = entries[0];
          if (!first) return null;
          const child = `${first.childFirstName} ${first.childLastName}`;
          const booked = bookedTwice(entries);
          return (
            <li
              key={key}
              className="flex flex-col gap-2 py-3 first:pt-0 last:pb-0 sm:col-span-2 sm:grid sm:grid-cols-subgrid sm:items-center"
            >
              <div className="min-w-0">
                <p className="text-sm font-semibold text-gray-900">{child}</p>
                <p className="mt-0.5 text-sm text-gray-600">
                  {booked || "In diesem Lauf nicht abgerechnet."}
                </p>
              </div>
              <div className="flex w-full flex-col gap-2 min-[480px]:w-auto min-[480px]:flex-row sm:justify-end">
                {contracts.map((contract, index) => {
                  const label =
                    contracts.length === 1 ? "Vertrag" : `Vertrag ${index + 1}`;
                  return (
                    <ButtonLink
                      key={contract.contractId}
                      href={traegerPath(
                        `/traeger/vertraege/${contract.contractId}`,
                      )}
                      variant="surface"
                      size="compact"
                      aria-label={`${label} von ${child} öffnen: ${contract.schoolName}`}
                    >
                      {label} · {contract.schoolName}
                    </ButtonLink>
                  );
                })}
              </div>
            </li>
          );
        })}
      </ResponsiveActionList>
    </SectionCard>
  );
}
