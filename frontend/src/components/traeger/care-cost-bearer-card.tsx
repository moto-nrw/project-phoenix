"use client";

// Kostenträger im Monatslauf (#4687): je Kostenträger, für wie viele
// Rechnungen er einen Anteil übernimmt und welchen Betrag. Die Liste je
// Kostenträger lädt die Abrechnung als PDF, Excel oder CSV herunter, um den
// Anteil getrennt abzurechnen. Vor der Freigabe ist sie eine Vorschau ohne
// Rechnungsnummern. Die Seite lädt die Karte erst, wenn ein Kostenträger im
// Lauf etwas übernimmt.

import { HandCoins } from "lucide-react";
import { useState } from "react";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import {
  exportFormatOptions,
  exportMenuItems,
} from "~/components/ui/export-menu";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { useApiErrorDisplay } from "~/contexts/ToastContext";
import { downloadBlob } from "~/lib/file-download";
import { createLogger } from "~/lib/logger";
import type { CareCostBearerTotal } from "~/lib/traeger-care-billing-api";
import { downloadCareCostShareList } from "~/lib/traeger-care-cost-shares-api";
import { formatEuro } from "./contribution-format";

const logger = createLogger({ component: "CareCostBearerCard" });

const FORMATS = exportFormatOptions(["pdf", "xlsx", "csv"]);

export function CareCostBearerCard({
  runId,
  released,
  totals,
}: Readonly<{
  runId: string;
  released: boolean;
  totals: readonly CareCostBearerTotal[];
}>) {
  const [exporting, setExporting] = useState<string | null>(null);
  const { show: showActionError } = useApiErrorDisplay();
  if (totals.length === 0) return null;
  const sum = totals.reduce((total, bearer) => total + bearer.amountCents, 0);

  const exportList = async (
    bearer: CareCostBearerTotal,
    format: "pdf" | "xlsx" | "csv",
  ) => {
    setExporting(`${bearer.costBearerId}:${format}`);
    try {
      const file = await downloadCareCostShareList(
        runId,
        bearer.costBearerId,
        format,
      );
      downloadBlob(file.blob, file.filename);
    } catch (err) {
      logger.error("traeger_care_cost_share_export_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showActionError(err, {
        object: `die Liste für ${bearer.costBearerName}`,
      });
    } finally {
      setExporting(null);
    }
  };

  const columns: DataTableColumn<CareCostBearerTotal>[] = [
    {
      key: "name",
      header: "Kostenträger",
      stacked: "title",
      render: (bearer) => (
        <span className="font-medium wrap-anywhere text-gray-900">
          {bearer.costBearerName}
        </span>
      ),
    },
    {
      key: "invoices",
      header: "Rechnungen",
      stacked: "field",
      render: (bearer) => String(bearer.invoiceCount),
    },
    {
      key: "amount",
      header: "Übernimmt",
      align: "right",
      stacked: "meta",
      render: (bearer) => (
        <span className="font-semibold text-gray-900 tabular-nums">
          {formatEuro(bearer.amountCents)}
        </span>
      ),
    },
    {
      key: "actions",
      header: "",
      align: "right",
      stacked: "meta",
      stackedLabel: false,
      render: (bearer) => (
        <OverflowMenu
          ariaLabel={`Liste für ${bearer.costBearerName} herunterladen`}
          triggerSize="sm"
          items={exportMenuItems({
            header: released ? "Liste für den Kostenträger" : "Vorschau",
            options: FORMATS,
            busyKey: exporting?.startsWith(`${bearer.costBearerId}:`)
              ? exporting.slice(bearer.costBearerId.length + 1)
              : null,
            onSelect: (option) => {
              if (
                option.format === "pdf" ||
                option.format === "xlsx" ||
                option.format === "csv"
              ) {
                void exportList(bearer, option.format);
              }
            },
          })}
        />
      ),
    },
  ];

  return (
    <SectionCard
      icon={HandCoins}
      title="Kostenträger"
      titleBadge={
        <StatusBadge tone="blue" label={`${formatEuro(sum)} übernommen`} />
      }
      description={
        released
          ? "Diesen Teil der Beiträge bezahlen Kostenträger, nicht die Familien. Die Liste für jeden Kostenträger steht im Menü der Zeile."
          : "Diesen Teil der Beiträge bezahlen Kostenträger, nicht die Familien. Vor der Freigabe ist die Liste eine Vorschau ohne Rechnungsnummern."
      }
    >
      <DataTable
        columns={columns}
        rows={[...totals]}
        getRowKey={(bearer) => bearer.costBearerId}
        rowHasInteractiveControls
        stackedOnMobile
      />
    </SectionCard>
  );
}
