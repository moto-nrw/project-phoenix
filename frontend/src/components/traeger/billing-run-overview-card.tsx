"use client";

// Die Rechnungen eines Abrechnungslaufs (#4668, #4672, #5146): EINE Tabelle,
// je Familie eine Zeile. Vor der Freigabe Entwürfe ohne Nummer, die sich je
// Kind korrigieren lassen; nach der Freigabe die Übersicht für die Kämmerei
// mit allen Positionen und Summen als PDF, Excel oder CSV im Menü der Karte
// unter „Exportieren“, wie im Schul-Portal (#5240). Die Aufschlüsselung steht
// im Seitenpanel, Korrektur und Schreiben im Menü der Zeile. Nach der
// Freigabe stehen dort auch Storno und Gutschrift (#4690); beide sind eigene
// Zeilen mit eigener Nummer, Minusbetrag und Bezug auf die Rechnung.

import { useState } from "react";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import {
  type ExportMenuOption,
  exportFormatOptions,
  exportMenuItems,
} from "~/components/ui/export-menu";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { useApiErrorDisplay } from "~/contexts/ToastContext";
import { downloadBlob } from "~/lib/file-download";
import { createLogger } from "~/lib/logger";
import {
  type BillingChild,
  type BillingDocument,
  type BillingRunDetail,
  type BillingRunExportFormat,
  downloadBillingRunOverview,
} from "~/lib/traeger-billing-api";
import {
  BillingDocumentBreakdown,
  recipientName,
} from "./billing-document-card";
import {
  BILLING_DOCUMENT_KIND_LABELS,
  billingReferenceText,
  isCancelled,
  isCredited,
} from "./billing-format";
import { formatEuro } from "./contribution-format";

const logger = createLogger({ component: "BillingRunOverviewCard" });

const FORMATS: readonly ExportMenuOption[] = exportFormatOptions([
  "pdf",
  "xlsx",
  "csv",
]);

/** A draft has no ID yet; its registration names it. */
function rowKey(document: BillingDocument): string {
  return document.id || `draft-${document.registrationId}`;
}

/**
 * What an issued invoice still asks for one child: the child's amount
 * minus the Gutschriften of the run for that child (#4690).
 */
function leftForChild(
  run: BillingRunDetail,
  invoice: BillingDocument,
  child: BillingChild,
): number {
  let left = child.totalCents;
  for (const document of run.documents) {
    if (document.kind !== "credit") continue;
    if (document.referenceNumber !== invoice.number) continue;
    for (const credited of document.children) {
      if (credited.childId === child.childId) left += credited.totalCents;
    }
  }
  return left;
}

export function BillingRunOverviewCard({
  run,
  onCorrect,
  onDownloadLetter,
  onCancel,
  onCredit,
  onReissue,
}: Readonly<{
  run: BillingRunDetail;
  /** Set while the run is a preview; corrects one child of a document. */
  onCorrect?: (document: BillingDocument, child: BillingChild) => void;
  /** Set once the letters of the released run exist (#4669). */
  onDownloadLetter?: (document: BillingDocument) => void;
  /** Set once the run is released: Storno of an invoice (#4690). */
  onCancel?: (document: BillingDocument) => void;
  /** Set once the run is released: Gutschrift for one child (#4690). */
  onCredit?: (
    document: BillingDocument,
    child: BillingChild,
    leftCents: number,
  ) => void;
  /** Opens the run that issues a cancelled invoice again (#4690). */
  onReissue?: (document: BillingDocument) => void;
}>) {
  // Gemerkt wird die Zeile, nicht die Rechnung: nach einer Korrektur zeigt das
  // Panel die neuen Beträge.
  const [shownKey, setShownKey] = useState<string | null>(null);
  const [exporting, setExporting] = useState<BillingRunExportFormat | null>(
    null,
  );
  const { show: showActionError } = useApiErrorDisplay();
  const released = run.status === "released";
  const shown =
    run.documents.find((document) => rowKey(document) === shownKey) ?? null;

  const exportFile = async (format: BillingRunExportFormat) => {
    setExporting(format);
    try {
      const file = await downloadBillingRunOverview(run.id, format);
      downloadBlob(file.blob, file.filename);
    } catch (err) {
      logger.error("traeger_billing_run_export_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showActionError(err, { object: "die Übersicht" });
    } finally {
      setExporting(null);
    }
  };

  // Storno and Gutschrift only for an issued invoice that is not cancelled;
  // a Storno only while it has no Gutschrift, a Gutschrift only while the
  // child has something left.
  const correctionItems = (document: BillingDocument) => {
    if (!released || document.kind !== "invoice" || isCancelled(document)) {
      return [];
    }
    const items: { label: string; onClick: () => void }[] = [];
    if (onCancel && !isCredited(document)) {
      items.push({
        label: "Rechnung stornieren",
        onClick: () => onCancel(document),
      });
    }
    if (onCredit) {
      for (const child of document.children) {
        const left = leftForChild(run, document, child);
        if (left <= 0) continue;
        items.push({
          label: `Gutschrift für ${child.firstName}`,
          onClick: () => onCredit(document, child, left),
        });
      }
    }
    return items;
  };

  const columns: DataTableColumn<BillingDocument>[] = [
    {
      key: "recipient",
      header: "Empfänger",
      // Nachname zuerst, wie in der Übersicht für die Kämmerei.
      render: (document) => (
        <span className="font-medium text-gray-900">
          {`${document.recipientLastName}, ${document.recipientFirstName}`}
        </span>
      ),
      sortValue: (document) => document.recipientLastName,
      stacked: "title",
    },
    {
      key: "number",
      header: "Rechnung",
      render: (document) => {
        const reference = billingReferenceText(document);
        return (
          <span className="flex flex-col items-start gap-1">
            <span className="flex flex-wrap items-center gap-2">
              {document.number ? (
                <span className="font-mono text-gray-900">
                  {document.number}
                </span>
              ) : (
                "Entwurf ohne Nummer"
              )}
              {document.kind !== "invoice" ? (
                <StatusBadge
                  label={BILLING_DOCUMENT_KIND_LABELS[document.kind]}
                  tone="blue"
                />
              ) : null}
              {isCancelled(document) ? (
                <StatusBadge label="Storniert" tone="red" />
              ) : null}
            </span>
            {reference ? (
              <span className="text-sm wrap-anywhere text-gray-600">
                {reference}
              </span>
            ) : null}
          </span>
        );
      },
      stacked: "field",
    },
    {
      key: "children",
      header: "Kinder",
      render: (document) =>
        document.children.map((child) => child.firstName).join(", "),
      stacked: "field",
    },
    {
      key: "total",
      header: "Betrag",
      align: "right",
      render: (document) => (
        <span className="tabular-nums">{formatEuro(document.totalCents)}</span>
      ),
      sortValue: (document) => document.totalCents,
      stacked: "meta",
    },
    {
      key: "actions",
      header: "",
      align: "right",
      stacked: "meta",
      stackedLabel: false,
      render: (document) => (
        <OverflowMenu
          ariaLabel={`Aktionen für die Rechnung an ${recipientName(document)}`}
          triggerSize="sm"
          items={[
            {
              label: "Aufschlüsselung ansehen",
              onClick: () => setShownKey(rowKey(document)),
            },
            ...(onCorrect
              ? document.children.map((child) => ({
                  label: `Betrag für ${child.firstName} korrigieren`,
                  onClick: () => onCorrect(document, child),
                }))
              : []),
            ...correctionItems(document),
            ...(onDownloadLetter
              ? [
                  {
                    label: "Schreiben herunterladen",
                    onClick: () => onDownloadLetter(document),
                  },
                ]
              : []),
          ]}
        />
      ),
    },
  ];

  return (
    <SectionCard
      title="Rechnungen"
      description={
        released
          ? "Die Übersicht für die Kämmerei enthält jede Position, die Summe je Rechnung und die Gesamtsumme. Storno und Gutschrift stehen als eigene Zeilen mit Minusbetrag."
          : "Je Familie eine Rechnung. Die Aufschlüsselung je Kind steht im Menü der Zeile."
      }
      actions={
        released && run.documents.length > 0 ? (
          <OverflowMenu
            ariaLabel="Übersicht für die Kämmerei exportieren"
            items={exportMenuItems({
              header: "Exportieren",
              options: FORMATS,
              busyKey: exporting,
              onSelect: (option) => {
                if (
                  option.format === "pdf" ||
                  option.format === "xlsx" ||
                  option.format === "csv"
                ) {
                  void exportFile(option.format);
                }
              },
            })}
          />
        ) : null
      }
    >
      <DataTable
        columns={columns}
        rows={[...run.documents]}
        getRowKey={rowKey}
        rowHasInteractiveControls
        stackedOnMobile
        emptyState={
          <EmptyState
            title="Keine Rechnung in diesem Lauf."
            description="Alle bestätigten Anmeldungen haben schon eine Rechnung, oder es ist noch keine bestätigt."
          />
        }
      />
      <BillingDocumentBreakdown
        document={shown}
        runId={run.id}
        onClose={() => setShownKey(null)}
        onReissue={onReissue}
      />
    </SectionCard>
  );
}
