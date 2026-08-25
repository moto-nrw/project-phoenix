"use client";

import {
  formatCents,
  invoiceStatusLabel,
  invoiceStatusTone,
  type ContractOverview,
  type Invoice,
} from "~/lib/contract-api";
import { formatDate } from "~/lib/date-helpers";
import { InfoCard, InfoItem } from "~/components/ui/info-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { EmptyState } from "~/components/ui/empty-state";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";

// Vertrag (#1459 Demo) — reine Anzeigebausteine.
//
// Verständlichkeit: Auf dieser Seite kann niemand etwas ändern. Deshalb hat
// hier nichts eine Schaltflächen-Optik, keinen Pfeil und keinen Hover-Effekt,
// und jeder Block sagt in einem Satz, wofür er da ist. Wer etwas ändern will,
// findet oben den Weg dorthin (Kontakt zum moto-Team).

const NOT_SET = "Noch nicht hinterlegt";

/** Leere Werte werden als Satz ausgegeben, nie als leere Zeile. */
function orNotSet(value: string): string {
  return value.trim() === "" ? NOT_SET : value;
}

function orNotSetDate(value: string | null): string {
  return value ? formatDate(value) : NOT_SET;
}

function DocumentIcon() {
  return (
    <svg
      className="h-5 w-5 text-gray-600"
      fill="none"
      stroke="currentColor"
      strokeWidth={2}
      viewBox="0 0 24 24"
      aria-hidden="true"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        d="M9 12h6m-6 4h6M9 8h6M5 21h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v14a2 2 0 002 2z"
      />
    </svg>
  );
}

function ChildrenIcon() {
  return (
    <svg
      className="h-5 w-5 text-gray-600"
      fill="none"
      stroke="currentColor"
      strokeWidth={2}
      viewBox="0 0 24 24"
      aria-hidden="true"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z"
      />
    </svg>
  );
}

function MailIcon() {
  return (
    <svg
      className="h-5 w-5 text-gray-600"
      fill="none"
      stroke="currentColor"
      strokeWidth={2}
      viewBox="0 0 24 24"
      aria-hidden="true"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
      />
    </svg>
  );
}

/** „Ihr Tarif": was gebucht ist, zu welchem Preis und wie lange. */
export function ContractFactsCard({
  overview,
}: Readonly<{ overview: ContractOverview }>) {
  const price =
    overview.pricePerChildCents > 0
      ? `${formatCents(overview.pricePerChildCents)} pro Kind und Monat`
      : NOT_SET;

  const term =
    overview.termStart || overview.termEnd
      ? `${orNotSetDate(overview.termStart)} bis ${overview.termEnd ? formatDate(overview.termEnd) : "unbefristet"}`
      : NOT_SET;

  return (
    <InfoCard title="Ihr Tarif" icon={<DocumentIcon />}>
      <p className="text-sm text-gray-600">
        Das hat Ihre Schule mit moto vereinbart.
      </p>
      <InfoItem label="Tarif" value={overview.tierLabel} />
      <InfoItem label="Preis" value={price} />
      <InfoItem label="Zahlungsrhythmus" value={overview.billingCycleLabel} />
      <InfoItem label="Laufzeit" value={term} />
    </InfoCard>
  );
}

/**
 * „Kinder im Vertrag": gebuchte Zahl neben der heute tatsächlich aktiven.
 *
 * Wichtig für die Verständlichkeit: Eine Überschreitung sperrt nichts. Wer das
 * nicht dazuschreibt, liest die rote Zahl als Warnung vor einer Abschaltung.
 */
export function ChildQuotaCard({
  overview,
}: Readonly<{ overview: ContractOverview }>) {
  const booked = overview.bookedChildren;
  const active = overview.activeChildren;
  const exceeded = booked > 0 && active > booked;

  return (
    <InfoCard title="Kinder im Vertrag" icon={<ChildrenIcon />}>
      <p className="text-sm text-gray-600">
        So viele Kinder deckt Ihr Vertrag ab, verglichen mit heute.
      </p>
      <InfoItem
        label="Gebucht"
        value={booked > 0 ? `${booked} Kinder` : NOT_SET}
      />
      <InfoItem
        label={`Aktiv am ${formatDate(overview.referenceDate)}`}
        value={`${active} Kinder mit Status „aktiv“`}
      />
      {exceeded ? (
        <p className="text-sm text-gray-600">
          Sie betreuen mehr Kinder als gebucht. moto bleibt vollständig nutzbar.
          Das moto-Team meldet sich wegen der Anpassung bei Ihnen.
        </p>
      ) : null}
    </InfoCard>
  );
}

/** „Rechnung und Kontakt": wohin die Rechnung geht und wer Fragen beantwortet. */
export function BillingContactCard({
  overview,
}: Readonly<{ overview: ContractOverview }>) {
  return (
    <InfoCard title="Rechnung und Kontakt" icon={<MailIcon />}>
      <p className="text-sm text-gray-600">
        Dorthin schickt moto die Rechnungen. Dort beantwortet moto Ihre Fragen.
      </p>
      <InfoItem
        label="Rechnung geht an"
        value={orNotSet(overview.invoiceRecipient)}
      />
      <InfoItem
        label="Kundennummer"
        value={orNotSet(overview.customerNumber)}
      />
      <InfoItem
        label="Fragen zur Rechnung"
        value={
          overview.supportEmail.trim() === "" ? (
            NOT_SET
          ) : (
            <a
              className="text-moto-blue underline"
              href={`mailto:${overview.supportEmail}`}
            >
              {overview.supportEmail}
            </a>
          )
        }
      />
    </InfoCard>
  );
}

/**
 * Zahlungsplan. Reine Anzeige: keine Zeile ist anklickbar, weil dahinter
 * nichts passiert. Den Status setzt das moto-Team.
 */
export function InvoiceTable({
  invoices,
}: Readonly<{ invoices: readonly Invoice[] }>) {
  const columns: DataTableColumn<Invoice>[] = [
    {
      key: "period",
      header: "Zeitraum",
      render: (row) => (
        <span className="font-medium text-gray-900">{row.periodLabel}</span>
      ),
      sortValue: (row) => row.dueDate,
    },
    {
      key: "due",
      header: "Fällig am",
      render: (row) => formatDate(row.dueDate),
      sortValue: (row) => row.dueDate,
    },
    {
      key: "amount",
      header: "Betrag",
      align: "right",
      render: (row) => formatCents(row.amountCents),
      sortValue: (row) => row.amountCents,
    },
    {
      key: "status",
      header: "Status",
      render: (row) => (
        <div className="flex flex-col items-start gap-1">
          <StatusBadge
            label={invoiceStatusLabel(row)}
            tone={invoiceStatusTone(row)}
          />
          {row.paidOn ? (
            <span className="text-xs text-gray-500">
              Bezahlt am {formatDate(row.paidOn)}
            </span>
          ) : null}
        </div>
      ),
      sortValue: (row) => invoiceStatusLabel(row),
    },
    {
      key: "number",
      header: "Rechnungsnummer",
      render: (row) =>
        row.invoiceNumber === "" ? (
          <span className="text-gray-500">Noch keine</span>
        ) : (
          row.invoiceNumber
        ),
      sortValue: (row) => row.invoiceNumber,
    },
  ];

  return (
    <DataTable
      columns={columns}
      rows={[...invoices]}
      getRowKey={(row) => row.id}
      defaultSortKey="due"
      defaultSortDirection="desc"
      emptyState={
        <EmptyState
          title="Noch keine Rechnungen"
          description="Sobald moto eine Rechnung stellt, steht sie hier."
        />
      }
    />
  );
}
