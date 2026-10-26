"use client";

// Übersicht eines freigegebenen Laufs für die Kämmerei (#4672): je Familie
// Rechnungsnummer, Empfänger, Kinder und Betrag, und der Download als PDF,
// Excel oder CSV mit allen Positionen und Summen.

import { useState } from "react";
import { Button } from "~/components/ui/button";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
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

const FORMATS: readonly {
  readonly format: BillingRunExportFormat;
  readonly label: string;
}[] = [
  { format: "pdf", label: "Als PDF herunterladen" },
  { format: "xlsx", label: "Als Excel herunterladen" },
  { format: "csv", label: "Als CSV herunterladen" },
];

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
      downloadBlob(
        file.blob,
        file.filename ?? `abrechnungsanordnung.${format}`,
      );
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
        <div className="flex flex-wrap gap-2">
          {FORMATS.map(({ format, label }) => (
            <Button
              key={format}
              type="button"
              variant="outline"
              size="sm"
              isLoading={exporting === format}
              loadingText="Wird erstellt …"
              disabled={exporting !== null}
              onClick={() => void exportFile(format)}
            >
              {label}
            </Button>
          ))}
        </div>
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
