"use client";

// Eine Familie in der Abrechnung (#4691): ihre Summen ausgestellt,
// storniert, gutgeschrieben und offen, wer ihre Rechnungen zahlt, und jedes
// Dokument mit Ferienzeitraum, Kindern, Betrag und Stand, das neueste
// zuerst. Ein Klick auf ein Dokument öffnet seinen Abrechnungslauf, wo
// Storno, Gutschrift und Schreiben liegen. Nur zum Lesen.

import { useParams, useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import { BillingAmountCards } from "~/components/traeger/billing-amount-cards";
import {
  INVOICE_STATUS_LABELS,
  INVOICE_STATUS_TONES,
  PAYMENT_TONES,
  familyName,
  paymentLabel,
} from "~/components/traeger/billing-family-format";
import { BILLING_DOCUMENT_KIND_LABELS } from "~/components/traeger/billing-format";
import { formatEuro } from "~/components/traeger/contribution-format";
import { formatDateRange } from "~/components/traeger/holiday-period-format";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { EmptyState } from "~/components/ui/empty-state";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { TenantPage, TenantPageStats } from "~/components/ui/tenant-page";
import { useApiLoadError } from "~/contexts/ToastContext";
import { parentsConcept } from "~/lib/concepts/parents";
import { formatDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  type BillingFamilyDetail,
  type BillingFamilyDocument,
  fetchBillingFamily,
} from "~/lib/traeger-billing-families-api";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "TraegerBillingFamilyPage" });

/** The line under a Storno or Gutschrift: its invoice and its reason. */
function referenceText(document: BillingFamilyDocument): string {
  if (!document.referenceNumber) return "";
  const reference =
    document.kind === "invoice"
      ? `Ersetzt ${document.referenceNumber}`
      : `Zu ${document.referenceNumber}`;
  return document.reason ? `${reference} · ${document.reason}` : reference;
}

const COLUMNS: DataTableColumn<BillingFamilyDocument>[] = [
  {
    key: "issued",
    header: "Datum",
    sortValue: (document) => document.issuedOn,
    render: (document) => (
      <span className="whitespace-nowrap">{formatDate(document.issuedOn)}</span>
    ),
    stacked: "meta",
  },
  {
    key: "document",
    header: "Dokument",
    render: (document) => {
      const reference = referenceText(document);
      return (
        <span className="block min-w-0">
          <span className="block font-medium text-gray-900">
            {BILLING_DOCUMENT_KIND_LABELS[document.kind]}{" "}
            <span className="font-mono whitespace-nowrap">
              {document.number}
            </span>
          </span>
          {reference ? (
            <span className="block text-sm text-gray-500">{reference}</span>
          ) : null}
          {document.payment && document.payment.kind !== "family" ? (
            <span className="mt-1 flex max-w-80 min-w-0">
              <StatusBadge
                label={paymentLabel(document.payment)}
                tone={PAYMENT_TONES[document.payment.kind]}
                truncate
              />
            </span>
          ) : null}
        </span>
      );
    },
    stacked: "title",
  },
  {
    key: "period",
    header: "Ferienzeitraum",
    render: (document) => (
      <span className="block min-w-0">
        <span className="block text-gray-900">{document.periodName}</span>
        <span className="block text-sm text-gray-500 md:whitespace-nowrap">
          {formatDateRange(document.serviceFrom, document.serviceUntil)}
        </span>
      </span>
    ),
  },
  {
    key: "children",
    header: "Kinder",
    render: (document) =>
      document.childNames.length > 0 ? document.childNames.join(", ") : "–",
  },
  {
    key: "amount",
    header: "Betrag",
    align: "right",
    sortValue: (document) => document.totalCents,
    render: (document) => (
      <span
        className={`tabular-nums ${document.totalCents < 0 ? "text-moto-red-strong" : "text-gray-900"}`}
      >
        {formatEuro(document.totalCents)}
      </span>
    ),
  },
  {
    key: "status",
    header: "Stand",
    render: (document) =>
      document.status ? (
        <StatusBadge
          label={INVOICE_STATUS_LABELS[document.status]}
          tone={INVOICE_STATUS_TONES[document.status]}
        />
      ) : (
        "–"
      ),
  },
  {
    key: "open",
    header: "Offen",
    align: "right",
    sortValue: (document) => document.openCents,
    render: (document) =>
      document.kind !== "invoice" ? (
        "–"
      ) : (
        <span
          className={
            document.openCents > 0
              ? "font-semibold text-gray-900 tabular-nums"
              : "text-gray-400 tabular-nums"
          }
        >
          {formatEuro(document.openCents)}
        </span>
      ),
  },
];

export default function TraegerBillingFamilyPage() {
  const params = useParams<{ id: string }>();
  const familyId = params?.id ?? "";
  const router = useRouter();
  const [family, setFamily] = useState<BillingFamilyDetail | null>(null);
  const pageLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = pageLoad;

  const load = useCallback(async () => {
    try {
      setFamily(await fetchBillingFamily(familyId));
      clearLoadError();
    } catch (err) {
      logger.error("traeger_billing_family_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "die Familie",
        retry: () => void load(),
      });
    }
  }, [familyId, clearLoadError, showLoadError]);

  useEffect(() => {
    void load();
  }, [load]);

  return (
    <TenantPage
      title={family ? familyName(family) : "Familie"}
      leading={<MotoConceptIcon concept={parentsConcept} size={28} />}
      back
      backHref={traegerPath("/traeger/abrechnung/familien")}
      backLabel="Zurück zu den Familien"
      stats={
        family ? (
          <TenantPageStats
            items={[
              {
                value: family.invoiceCount,
                label: family.invoiceCount === 1 ? "Rechnung" : "Rechnungen",
              },
              { value: family.openInvoiceCount, label: "offen" },
              {
                value: family.documentCount - family.invoiceCount,
                label:
                  family.documentCount - family.invoiceCount === 1
                    ? "Storno oder Gutschrift"
                    : "Stornos und Gutschriften",
              },
            ]}
          />
        ) : null
      }
      statsLoading={!family && !pageLoad.error}
      error={pageLoad.error}
      loading={!pageLoad.error && !family}
    >
      {family ? (
        <>
          <BillingAmountCards amounts={family} />
          <SectionCard
            title="Empfänger und Zahlungsart"
            description="An diese Adresse gehen die Schreiben. Die Zahlungsart steht auf jeder Rechnung."
          >
            <DataGrid columns={2}>
              <DataField label="E-Mail-Adresse">
                <span className="wrap-anywhere">{family.recipientEmail}</span>
              </DataField>
              <DataField label="Debitorennummer">
                {family.debtorNumbers.length > 0 ? (
                  <span className="font-mono">
                    {family.debtorNumbers.join(", ")}
                  </span>
                ) : (
                  "Keine eingetragen"
                )}
              </DataField>
              <DataField label="Zahlungsart" fullWidth>
                {family.payments.length > 0 ? (
                  <span className="flex max-w-full min-w-0 flex-wrap gap-1.5">
                    {family.payments.map((payment) => (
                      <StatusBadge
                        key={`${payment.kind}-${payment.costBearerName}`}
                        label={paymentLabel(payment)}
                        tone={PAYMENT_TONES[payment.kind]}
                        truncate
                      />
                    ))}
                  </span>
                ) : (
                  "Keine gültige Rechnung"
                )}
              </DataField>
            </DataGrid>
          </SectionCard>
          <SectionCard
            title="Dokumente"
            description="Rechnungen, Stornos und Gutschriften, das neueste zuerst. Ein Klick öffnet den Abrechnungslauf."
          >
            <DataTable
              columns={COLUMNS}
              rows={[...family.documents]}
              getRowKey={(document) => document.id}
              stackedOnMobile
              onRowClick={(document) =>
                router.push(
                  traegerPath(
                    `/traeger/abrechnung/${encodeURIComponent(document.runId)}`,
                  ),
                )
              }
              emptyState={<EmptyState title="Kein Dokument." />}
            />
          </SectionCard>
        </>
      ) : null}
    </TenantPage>
  );
}
