"use client";

// Übersicht eines freigegebenen Laufs für die Kämmerei (#4672): je Familie
// Rechnungsnummer, Empfänger, Kinder und Betrag, und der Download als PDF,
// Excel oder CSV mit allen Positionen und Summen.

import { useState } from "react";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import {
  ExportMenu,
  type ExportMenuOption,
  exportFormatOptions,
} from "~/components/ui/export-menu";
import { SectionCard } from "~/components/ui/section-card";
import { useApiErrorDisplay } from "~/contexts/ToastContext";
import { downloadBlob } from "~/lib/file-download";
import { createLogger } from "~/lib/logger";
import {
  type BillingDocument,
  type BillingRunDetail,
  type BillingRunExportFormat,
  downloadBillingRunOverview,
} from "~/lib/traeger-billing-api";
import { formatEuro } from "./contribution-format";

const logger = createLogger({ component: "BillingRunOverviewCard" });

const FORMATS: readonly ExportMenuOption[] = exportFormatOptions([
  "pdf",
  "xlsx",
  "csv",
]);

const COLUMNS: DataTableColumn<BillingDocument>[] = [
  {
    key: "number",
    header: "Rechnung",
    render: (document) => (
      <span className="font-mono text-gray-900">{document.number}</span>
    ),
    stacked: "title",
  },
  {
    key: "recipient",
    header: "Empfänger",
    render: (document) =>
      `${document.recipientLastName}, ${document.recipientFirstName}`,
  },
  {
    key: "children",
    header: "Kinder",
    render: (document) =>
      document.children.map((child) => child.firstName).join(", "),
  },
  {
    key: "total",
    header: "Betrag",
    align: "right",
    render: (document) => (
      <span className="tabular-nums">{formatEuro(document.totalCents)}</span>
    ),
    stacked: "meta",
  },
];

export function BillingRunOverviewCard({
  run,
}: Readonly<{ run: BillingRunDetail }>) {
  const [exporting, setExporting] = useState<BillingRunExportFormat | null>(
    null,
  );
  const { show: showActionError } = useApiErrorDisplay();

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

  return (
    <SectionCard
      title="Übersicht für die Kämmerei"
      description="Alle Rechnungen dieses Laufs. Die Datei enthält jede Position, die Summe je Rechnung und die Gesamtsumme."
      actions={
        <ExportMenu
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
      }
    >
      <DataTable
        columns={COLUMNS}
        rows={[...run.documents]}
        getRowKey={(document) => document.number}
        stackedOnMobile
      />
    </SectionCard>
  );
}
