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
import {
  PAYMENT_METHOD_LABELS,
  paymentMethodsText,
} from "~/components/traeger/billing-payment-format";
import { BILLING_DOCUMENT_KIND_LABELS } from "~/components/traeger/billing-format";
import { formatEuro } from "~/components/traeger/contribution-format";
import { formatDateRange } from "~/components/traeger/holiday-period-format";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { EmptyState } from "~/components/ui/empty-state";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { TenantPage, TenantPageStats } from "~/components/ui/tenant-page";
import { useApiLoadError } from "~/contexts/ToastContext";
import { parentsConcept } from "~/lib/concepts/parents";
import { billingConcept } from "~/lib/concepts/billing";
import { documentsConcept } from "~/lib/concepts/documents";
import { formatDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  type BillingFamilyDetail,
  type BillingFamilyDocument,
  fetchBillingFamily,
} from "~/lib/traeger-billing-families-api";
import { traegerPath } from "~/lib/traeger-url";
import { ConceptIconTile } from "~/components/ui/concept-icon-tile";

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
          {/* The state sits beside the number: a column of its own no
              longer fit beside the amounts on a tablet. */}
          <span className="flex flex-wrap items-center gap-x-2 gap-y-1 font-medium text-gray-900">
            <span>
              {BILLING_DOCUMENT_KIND_LABELS[document.kind]}{" "}
              <span className="font-mono whitespace-nowrap">
                {document.number}
              </span>
            </span>
            {document.status ? (
              <StatusBadge
                label={INVOICE_STATUS_LABELS[document.status]}
                tone={INVOICE_STATUS_TONES[document.status]}
              />
            ) : null}
          </span>
          {reference ? (
            <span className="block text-sm text-gray-500">{reference}</span>
          ) : null}
          {/* Payment and due date stay as the invoice set them, also after
              a Storno or a credit: the history of the invoice. */}
          {document.kind === "invoice" && document.paymentMethod ? (
            <span className="block text-sm text-gray-500">
              {PAYMENT_METHOD_LABELS[document.paymentMethod]}
              {document.dueOn
                ? ` · fällig am ${formatDate(document.dueOn)}`
                : ""}
            </span>
          ) : null}
          {document.payment && document.payment.kind !== "family" ? (
            <span className="mt-1 flex max-w-80 min-w-0">
              {/* The cost bearer's full name: it wraps, a tooltip would
                  stay out of reach by finger. */}
              <StatusBadge
                label={paymentLabel(document.payment)}
                tone={PAYMENT_TONES[document.payment.kind]}
                wrap
              />
            </span>
          ) : null}
        </span>
      );
    },
    stacked: "title",
  },
  {
    // Period and children share one column: seven columns no longer fit
    // beside each other on a laptop.
    key: "period",
    header: "Zeitraum und Kinder",
    render: (document) => (
      <span className="block min-w-0">
        {/* text-balance: a wrapping name splits into two even lines and
            keeps its year with the word before it („Herbstferien 2026“). */}
        <span className="block text-balance wrap-break-word text-gray-900">
          {document.periodName}
        </span>
        <span className="block text-sm text-gray-500 md:whitespace-nowrap">
          {formatDateRange(document.serviceFrom, document.serviceUntil)}
        </span>
        {document.childNames.length > 0 ? (
          <span className="block text-sm wrap-break-word text-gray-600">
            <span className="text-gray-500">
              {document.childNames.length === 1 ? "Kind" : "Kinder"}:{" "}
            </span>
            <span>{document.childNames.join(", ")}</span>
          </span>
        ) : null}
      </span>
    ),
    // On a phone the period, its dates and the labelled children name
    // themselves across the card's width; a label beside them would wrap.
    stackedLabel: false,
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
    key: "open",
    header: "Berechnet",
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
              : "text-gray-500 tabular-nums"
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

  // An invoice stays valid until a Storno cancels it; a credit only lowers it.
  const validInvoices = (family?.documents ?? []).filter(
    (document) =>
      document.kind === "invoice" && document.status !== "cancelled",
  ).length;

  return (
    <TenantPage
      title={family ? familyName(family) : "Familie"}
      leading={<ConceptIconTile concept={parentsConcept} size="page" />}
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
              // The backend's figure: invoices that still ask for money
              // after Storno and credit (as the families list counts).
              {
                value: family.openInvoiceCount,
                label: "mit berechnetem Betrag",
              },
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
            leading={<ConceptIconTile concept={billingConcept} />}
            title="Empfänger und Zahlung"
            description="An diese Adresse gehen die Schreiben. Wer zahlt und der Zahlungsweg stehen auf jeder Rechnung."
          >
            <DataGrid columns={2}>
              <DataField label="E-Mail-Adresse">
                <span className="wrap-anywhere">{family.recipientEmail}</span>
              </DataField>
              <DataField label="Debitorennummer">
                {family.debtorNumbers.length > 0 ? (
                  // Each number stays whole; only the list wraps.
                  <span className="font-mono">
                    {family.debtorNumbers.map((number, index) => (
                      <span key={number} className="whitespace-nowrap">
                        {number}
                        {index < family.debtorNumbers.length - 1 ? ", " : ""}
                      </span>
                    ))}
                  </span>
                ) : (
                  "Keine eingetragen"
                )}
              </DataField>
              <DataField label="Zahlungsweg">
                {paymentMethodsText(family.paymentMethods) ??
                  (validInvoices > 0
                    ? "Auf den Rechnungen nicht genannt"
                    : "Steht auf neuen Rechnungen")}
              </DataField>
              <DataField label="Wer zahlt" fullWidth>
                {family.payments.length > 0 ? (
                  <span className="flex max-w-full min-w-0 flex-wrap gap-1.5">
                    {family.payments.map((payment) => (
                      <StatusBadge
                        key={`${payment.kind}-${payment.costBearerName}`}
                        label={paymentLabel(payment)}
                        tone={PAYMENT_TONES[payment.kind]}
                        wrap
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
            leading={<ConceptIconTile concept={documentsConcept} />}
            title="Dokumente"
            description="Neueste Dokumente zuerst. Jede Zeile öffnet den Abrechnungslauf."
          >
            <DataTable
              columns={COLUMNS}
              rows={[...family.documents]}
              getRowKey={(document) =>
                `${document.service ?? "holiday"}-${document.id}`
              }
              stackedOnMobile
              onRowClick={(document) =>
                router.push(
                  traegerPath(
                    document.service === "care"
                      ? `/traeger/abrechnung/monatsbeitraege/${encodeURIComponent(document.runId)}`
                      : `/traeger/abrechnung/${encodeURIComponent(document.runId)}`,
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
