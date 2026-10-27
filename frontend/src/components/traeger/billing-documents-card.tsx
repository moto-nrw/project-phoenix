"use client";

// Die Rechnungen eines Abrechnungslaufs (#4668, #5146): EINE Tabelle, je
// Familie eine Zeile. Vor der Freigabe Entwürfe ohne Nummer, die sich je Kind
// korrigieren lassen; danach die ausgestellten Rechnungen mit der Übersicht
// für die Kämmerei (#4672) als Download. Die Aufschlüsselung je Kind steht im
// Seitenpanel, Korrektur und Schreiben im Menü der Zeile.

import { useState } from "react";
import { ContributionBreakdown } from "~/components/holiday-contribution/contribution-breakdown";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import {
  ExportMenu,
  type ExportMenuOption,
  exportFormatOptions,
} from "~/components/ui/export-menu";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { SectionCard } from "~/components/ui/section-card";
import {
  SlideOver,
  SlideOverBody,
  SlideOverCloseButton,
  SlideOverContent,
  SlideOverDescription,
  SlideOverHeader,
  SlideOverTitle,
} from "~/components/ui/slide-over";
import { useApiErrorDisplay } from "~/contexts/ToastContext";
import { formatBerlinDate, formatDate } from "~/lib/date-helpers";
import { downloadBlob } from "~/lib/file-download";
import { createLogger } from "~/lib/logger";
import {
  type BillingChild,
  type BillingDocument,
  type BillingRunDetail,
  type BillingRunExportFormat,
  downloadBillingRunOverview,
} from "~/lib/traeger-billing-api";
import { billingPositionLabel } from "./billing-format";
import { formatEuro } from "./contribution-format";
import { formatDateRange } from "./holiday-period-format";

const logger = createLogger({ component: "BillingDocumentsCard" });

const FORMATS: readonly ExportMenuOption[] = exportFormatOptions([
  "pdf",
  "xlsx",
  "csv",
]);

function recipientName(document: BillingDocument): string {
  return `${document.recipientFirstName} ${document.recipientLastName}`;
}

function issuedText(document: BillingDocument): string {
  return document.number
    ? `Rechnung ${document.number}${document.issuedOn ? ` vom ${formatDate(document.issuedOn)}` : ""}`
    : "Entwurf ohne Nummer";
}

function CorrectionHistory({ child }: Readonly<{ child: BillingChild }>) {
  if (child.corrections.length === 0) return null;
  return (
    <div className="space-y-1">
      <p className="text-sm font-medium text-gray-700">
        Korrekturen für {child.firstName}
      </p>
      <ul className="space-y-1 text-sm text-gray-700">
        {child.corrections.map((correction) => (
          <li key={correction.id}>
            {formatBerlinDate(correction.createdAt)}:{" "}
            <span className="tabular-nums">
              {formatEuro(correction.totalBeforeCents)} →{" "}
              {formatEuro(correction.totalAfterCents)}
            </span>
            . {correction.reason}
          </li>
        ))}
      </ul>
    </div>
  );
}

/** Die Aufschlüsselung einer Rechnung: Positionen je Kind und Korrekturen. */
function BillingDocumentBreakdown({
  document,
  onClose,
}: Readonly<{ document: BillingDocument | null; onClose: () => void }>) {
  return (
    <SlideOver
      open={document !== null}
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      {document ? (
        <SlideOverContent>
          <SlideOverHeader className="flex-row items-start justify-between gap-3">
            <div className="min-w-0 space-y-1">
              <SlideOverTitle>
                Rechnung an {recipientName(document)}
              </SlideOverTitle>
              <SlideOverDescription>
                {issuedText(document)} · {document.recipientEmail} · Betreuung{" "}
                {formatDateRange(document.serviceFrom, document.serviceUntil)}
              </SlideOverDescription>
            </div>
            <SlideOverCloseButton />
          </SlideOverHeader>
          <SlideOverBody className="space-y-4">
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
            {document.children.map((child) => (
              <CorrectionHistory key={child.childId} child={child} />
            ))}
          </SlideOverBody>
        </SlideOverContent>
      ) : null}
    </SlideOver>
  );
}

export function BillingDocumentsCard({
  run,
  onCorrect,
  onDownloadLetter,
}: Readonly<{
  run: BillingRunDetail;
  /** Set while the run is a preview; corrects one child of a document. */
  onCorrect?: (document: BillingDocument, child: BillingChild) => void;
  /** Set once the letters of the released run exist (#4669). */
  onDownloadLetter?: (document: BillingDocument) => void;
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
    run.documents.find((document) => document.registrationId === shownKey) ??
    null;

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
      render: (document) =>
        document.number ? (
          <span className="font-mono text-gray-900">{document.number}</span>
        ) : (
          "Entwurf ohne Nummer"
        ),
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
              onClick: () => setShownKey(document.registrationId),
            },
            ...(onCorrect
              ? document.children.map((child) => ({
                  label: `Betrag für ${child.firstName} korrigieren`,
                  onClick: () => onCorrect(document, child),
                }))
              : []),
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
          ? "Die Übersicht für die Kämmerei enthält jede Position, die Summe je Rechnung und die Gesamtsumme."
          : "Je Familie eine Rechnung. Die Aufschlüsselung je Kind steht im Menü der Zeile."
      }
      actions={
        released && run.documents.length > 0 ? (
          <ExportMenu
            label="Übersicht für die Kämmerei"
            ariaLabel="Übersicht für die Kämmerei herunterladen"
            options={FORMATS}
            busyKey={exporting}
            onSelect={(option) => {
              if (
                option.format === "pdf" ||
                option.format === "xlsx" ||
                option.format === "csv"
              ) {
                void exportFile(option.format);
              }
            }}
          />
        ) : null
      }
    >
      <DataTable
        columns={columns}
        rows={[...run.documents]}
        getRowKey={(document) => document.registrationId}
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
        onClose={() => setShownKey(null)}
      />
    </SectionCard>
  );
}
