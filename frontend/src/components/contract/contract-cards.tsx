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
import { DetailIcons } from "~/components/ui/detail-modal-components";

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
    <InfoCard title="Ihr Tarif" icon={DetailIcons.document}>
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
    <InfoCard title="Kinder im Vertrag" icon={DetailIcons.group}>
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
    <InfoCard title="Rechnung und Kontakt" icon={DetailIcons.document}>
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
