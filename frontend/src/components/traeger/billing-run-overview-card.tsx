"use client";

// Die Rechnungen eines Abrechnungslaufs (#4668, #4672, #5146): EINE Tabelle,
// je Familie eine Zeile. Vor der Freigabe Entwürfe ohne Nummer, die sich je
// Kind korrigieren lassen; nach der Freigabe die Übersicht für die Kämmerei
// mit allen Positionen und Summen als PDF, Excel oder CSV im Menü der Karte
// unter „Exportieren“, wie im Schul-Portal (#5240). Die Aufschlüsselung steht
// im Seitenpanel, Korrektur und Schreiben im Menü der Zeile. Nach der
// Freigabe stehen dort auch Storno und Gutschrift (#4690); beide sind eigene
// Zeilen mit eigener Nummer, Minusbetrag und Bezug auf die Rechnung.
// Sobald die Schreiben erstellt sind, zeigt ein Klick auf eine Zeile ihr
// Schreiben als PDF (#5692): ab xl rechts neben der Liste, darunter an ihrer
// Stelle mit dem Weg zurück zur Liste.

import { FileText } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { ContributionBreakdown } from "~/components/holiday-contribution/contribution-breakdown";
import { Button } from "~/components/ui/button";
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
import { BillingLetterPreview } from "./billing-letter-preview";
import {
  BILLING_DOCUMENT_KIND_LABELS,
  billingPositionLabel,
  billingReferenceText,
  isCancelled,
  isCredited,
} from "./billing-format";
import { formatEuro } from "./contribution-format";
import { recipientKindLabel } from "./billing-payment-format";
import { InvoicePaymentLine } from "./invoice-payment-line";

const logger = createLogger({ component: "BillingRunOverviewCard" });

const FORMATS: readonly ExportMenuOption[] = exportFormatOptions([
  "pdf",
  "xlsx",
  "csv",
]);

function rowMenuLabel(document: BillingDocument): string {
  return `Aktionen für die Rechnung an ${recipientName(document)}`;
}

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

function invoicesDescription(released: boolean, canPreview: boolean) {
  if (!released) {
    return "Je Familie eine Rechnung.";
  }
  const overview =
    "Die Übersicht für die Kämmerei enthält jede Position, die Summe je Rechnung und die Gesamtsumme. Storno und Gutschrift stehen als eigene Zeilen mit Minusbetrag.";
  return canPreview
    ? `${overview} Wählen Sie eine Zeile, um ihr Schreiben als PDF zu sehen.`
    : overview;
}

/** The positions of a row's children, opened in place under the row. */
function InlineBreakdown({
  document,
}: Readonly<{ document: BillingDocument }>) {
  return (
    <div className="max-w-xl py-1">
      <ContributionBreakdown
        items={document.children.map((child) => ({
          key: child.childId,
          name: `${child.firstName} ${child.lastName}`,
          lines: child.positions.map((position) => ({
            label: billingPositionLabel(position),
            amount: formatEuro(position.amountCents),
          })),
          total: formatEuro(child.totalCents),
        }))}
        childTotalLabel="Summe"
        totalLabel="Rechnungsbetrag"
        total={formatEuro(document.totalCents)}
      />
    </div>
  );
}

/** The empty letter frame beside the list before a row is chosen. */
function LetterPreviewPlaceholder() {
  return (
    <EmptyState
      variant="dashed"
      icon={<FileText className="h-10 w-10" aria-hidden="true" />}
      title="Noch kein Schreiben gewählt"
      description="Wählen Sie links eine Rechnung. Hier sehen Sie dann ihr Schreiben mit Nummer, Beträgen und Gestaltung."
      className="aspect-[210/297] w-full justify-center p-8"
    />
  );
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
  const [breakdownShown, setBreakdownShown] = useState(false);
  // The issued document whose letter shows beside the list (#5692).
  const [previewId, setPreviewId] = useState<string | null>(null);
  const previewRef = useRef<HTMLDivElement>(null);
  const [exporting, setExporting] = useState<BillingRunExportFormat | null>(
    null,
  );
  const { show: showActionError } = useApiErrorDisplay();
  const released = run.status === "released";
  const shown =
    run.documents.find((document) => rowKey(document) === shownKey) ?? null;
  // Every issued document has its letter once the letters of the run exist.
  const canPreview = released && run.letters !== null;
  const previewed =
    (canPreview &&
      run.documents.find(
        (document) => document.id !== "" && document.id === previewId,
      )) ||
    null;

  // Below xl the letter takes the place of the list; a long list may have
  // scrolled the card's top out of view, so the letter starts in view.
  const previewedId = previewed?.id ?? null;
  useEffect(() => {
    if (!previewedId) return;
    const pane = previewRef.current;
    if (pane && pane.getBoundingClientRect().top < 0) {
      pane.scrollIntoView({ block: "start" });
    }
  }, [previewedId]);

  const openPreview = (document: BillingDocument) => {
    if (document.id) setPreviewId(document.id);
  };
  // Back from the letter, the reader continues at the menu of its row, which
  // shows again with the list on the next frame.
  const closePreview = () => {
    const documentId = previewed?.id;
    const area = previewRef.current?.parentElement;
    setPreviewId(null);
    if (!documentId || !area) return;
    requestAnimationFrame(() => {
      const triggers = [
        ...area.querySelectorAll<HTMLElement>(
          `[data-billing-document-id="${CSS.escape(documentId)}"] button[aria-haspopup="menu"]`,
        ),
      ];
      const trigger =
        triggers.find((element) => element.getClientRects().length > 0) ??
        triggers[0];
      trigger?.focus({ preventScroll: true });
    });
  };

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
      render: (document) => {
        const kind = recipientKindLabel(document.recipientKind);
        const second =
          document.recipientKind === "both"
            ? document.recipientSecondName?.trim()
            : "";
        return (
          <span className="block min-w-0">
            <span className="block font-medium text-gray-900">
              {`${document.recipientLastName}, ${document.recipientFirstName}`}
            </span>
            {second ? (
              <span className="block text-sm text-gray-600">und {second}</span>
            ) : null}
            {kind ? (
              <span className="mt-1 block">
                <StatusBadge label={kind} tone="blue" />
              </span>
            ) : null}
            {document.kind === "invoice" && document.payment ? (
              document.totalCents <= 0 ? (
                <span className="mt-0.5 block text-sm text-gray-600">
                  Nichts zu zahlen
                </span>
              ) : (
                <InvoicePaymentLine payment={document.payment} />
              )
            ) : null}
          </span>
        );
      },
      sortValue: (document) => document.recipientLastName,
      stacked: "title",
    },
    {
      key: "number",
      header: "Nummer",
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
      // Beside the letter the names stand in the letter itself; the list
      // keeps its room for recipient, number and amount.
      ...(canPreview
        ? { className: "xl:hidden", headerClassName: "xl:hidden" }
        : {}),
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
      stackedTone: "figure",
    },
    {
      key: "actions",
      header: "",
      align: "right",
      stacked: "meta",
      stackedLabel: false,
      render: (document) => (
        // React also bubbles clicks from the portaled menu through this wrapper.
        // oxlint-disable-next-line jsx-a11y/click-events-have-key-events, jsx-a11y/no-static-element-interactions -- keine eigene Aktion; hält Menüklicks von der Zeilenaktion getrennt
        <div
          data-billing-document-id={document.id || undefined}
          onClick={(event) => event.stopPropagation()}
        >
          <OverflowMenu
            ariaLabel={rowMenuLabel(document)}
            items={[
              {
                label: "Aufschlüsselung zeigen",
                onClick: () => setShownKey(rowKey(document)),
              },
              ...(canPreview && document.id
                ? [
                    {
                      label: "Schreiben ansehen",
                      onClick: () => openPreview(document),
                    },
                  ]
                : []),
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
        </div>
      ),
    },
  ];

  const table = (
    <DataTable
      columns={columns}
      rows={[...run.documents]}
      getRowKey={rowKey}
      onRowClick={canPreview ? openPreview : undefined}
      // Before the release one switch shows every row's breakdown in place,
      // so checking the run needs no menu per row; afterwards a row opens
      // its letter.
      renderSubRows={
        breakdownShown && !canPreview
          ? (document) =>
              document.children.length > 0
                ? [
                    {
                      key: `${rowKey(document)}-breakdown`,
                      content: <InlineBreakdown document={document} />,
                    },
                  ]
                : []
          : undefined
      }
      activeRowKey={previewed ? rowKey(previewed) : null}
      rowHasInteractiveControls
      stackedOnMobile
      emptyState={
        <EmptyState
          title="Keine Rechnung in diesem Lauf."
          description="Alle bestätigten Anmeldungen haben schon eine Rechnung, oder es ist noch keine bestätigt."
        />
      }
    />
  );

  return (
    <SectionCard
      title="Rechnungen"
      overflow="visible"
      description={invoicesDescription(released, canPreview)}
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
        ) : !canPreview && run.documents.length > 0 ? (
          // Table only: the phone rows keep the breakdown in their menu.
          // The label names the next step; a second state attribute would
          // make screen readers say it twice.
          <Button
            type="button"
            variant="outline"
            size="md"
            className="max-md:hidden"
            onClick={() => setBreakdownShown((shown) => !shown)}
          >
            {breakdownShown
              ? "Aufschlüsselung ausblenden"
              : "Aufschlüsselung zeigen"}
          </Button>
        ) : null
      }
    >
      {canPreview && run.documents.length > 0 ? (
        <div className="xl:grid xl:grid-cols-[minmax(0,7fr)_minmax(0,5fr)] xl:items-start xl:gap-6">
          <div className={previewed ? "max-xl:hidden" : undefined}>{table}</div>
          <div
            ref={previewRef}
            className={`scroll-mt-24 xl:sticky xl:top-20 ${previewed ? "" : "max-xl:hidden"}`}
          >
            {previewed ? (
              <BillingLetterPreview
                runId={run.id}
                document={previewed}
                onClose={closePreview}
              />
            ) : (
              <LetterPreviewPlaceholder />
            )}
          </div>
        </div>
      ) : (
        table
      )}
      <BillingDocumentBreakdown
        document={shown}
        runId={run.id}
        onClose={() => setShownKey(null)}
        onReissue={onReissue}
      />
    </SectionCard>
  );
}
