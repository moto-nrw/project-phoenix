"use client";

// Kostenübernahme in einem Abrechnungslauf (#5627): je Kostenträger, wie
// viele Rechnungen er übernimmt und welchen Betrag. Nach der Freigabe lädt
// die Abrechnung je Kostenträger die Liste für die Erstattung als PDF, Excel
// oder CSV herunter. Die Karte erscheint nur, wenn ein Kostenträger im Lauf
// etwas übernimmt.

import { useState } from "react";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import {
  exportFormatOptions,
  exportMenuItems,
} from "~/components/ui/export-menu";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { SectionCard } from "~/components/ui/section-card";
import { useApiErrorDisplay } from "~/contexts/ToastContext";
import { downloadBlob } from "~/lib/file-download";
import { createLogger } from "~/lib/logger";
import {
  type BillingRunDetail,
  type BillingRunExportFormat,
  downloadCostTakeoverList,
} from "~/lib/traeger-billing-api";
import { formatEuro } from "./contribution-format";

const logger = createLogger({ component: "BillingCostTakeoverCard" });

const FORMATS = exportFormatOptions(["pdf", "xlsx", "csv"]);

interface CostTakeover {
  readonly id: string;
  readonly name: string;
  readonly documents: number;
  readonly cents: number;
}

/**
 * What each cost bearer pays in the run, in the order it first appears. A
 * Storno negates its invoice, the takeover too (#4690): it reverses the
 * amount and takes the invoice out of the count; a cost bearer with nothing
 * left drops out.
 */
export function runCostTakeovers(run: BillingRunDetail): CostTakeover[] {
  const takeovers = new Map<string, CostTakeover>();
  for (const document of run.documents) {
    const counted = new Set<string>();
    const count = document.kind === "cancellation" ? -1 : 1;
    for (const child of document.children) {
      for (const position of child.positions) {
        if (position.kind !== "cost_takeover" || !position.costBearerId) {
          continue;
        }
        const id = position.costBearerId;
        const current = takeovers.get(id) ?? {
          id,
          name: position.costBearerName ?? "",
          documents: 0,
          cents: 0,
        };
        takeovers.set(id, {
          ...current,
          documents: current.documents + (counted.has(id) ? 0 : count),
          cents: current.cents - position.amountCents,
        });
        counted.add(id);
      }
    }
  }
  return [...takeovers.values()].filter(
    (takeover) => takeover.documents > 0 || takeover.cents !== 0,
  );
}

export function BillingCostTakeoverCard({
  run,
}: Readonly<{ run: BillingRunDetail }>) {
  const [exporting, setExporting] = useState<string | null>(null);
  const { show: showActionError } = useApiErrorDisplay();
  const takeovers = runCostTakeovers(run);
  if (takeovers.length === 0) return null;
  const released = run.status === "released";

  const exportList = async (
    takeover: CostTakeover,
    format: BillingRunExportFormat,
  ) => {
    setExporting(`${takeover.id}:${format}`);
    try {
      const file = await downloadCostTakeoverList(run.id, takeover.id, format);
      downloadBlob(file.blob, file.filename);
    } catch (err) {
      logger.error("traeger_cost_takeover_export_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showActionError(err, { object: "die Liste für die Erstattung" });
    } finally {
      setExporting(null);
    }
  };

  const columns: DataTableColumn<CostTakeover>[] = [
    {
      key: "name",
      header: "Kostenträger",
      render: (takeover) => (
        <span className="font-medium wrap-anywhere text-gray-900">
          {takeover.name}
        </span>
      ),
      stacked: "title",
    },
    {
      key: "documents",
      header: "Rechnungen",
      render: (takeover) => String(takeover.documents),
      stacked: "field",
    },
    {
      key: "cents",
      header: "Betrag",
      align: "right",
      render: (takeover) => (
        <span className="tabular-nums">{formatEuro(takeover.cents)}</span>
      ),
      stacked: "meta",
    },
  ];
  if (released) {
    columns.push({
      key: "actions",
      header: "",
      align: "right",
      stacked: "meta",
      stackedLabel: false,
      render: (takeover) => (
        <OverflowMenu
          ariaLabel={`Liste für die Erstattung durch ${takeover.name} herunterladen`}
          triggerSize="sm"
          items={exportMenuItems({
            header: "Liste für die Erstattung",
            options: FORMATS,
            busyKey: exporting?.startsWith(`${takeover.id}:`)
              ? exporting.slice(takeover.id.length + 1)
              : null,
            onSelect: (option) => {
              if (
                option.format === "pdf" ||
                option.format === "xlsx" ||
                option.format === "csv"
              ) {
                void exportList(takeover, option.format);
              }
            },
          })}
        />
      ),
    });
  }

  return (
    <SectionCard
      title="Kostenübernahme"
      description={
        released
          ? "Diese Beträge bezahlen Kostenträger statt der Familien. Die Liste für die Erstattung steht im Menü der Zeile."
          : "Diese Beträge bezahlen Kostenträger statt der Familien. Die Liste für die Erstattung gibt es nach der Freigabe."
      }
    >
      <DataTable
        columns={columns}
        rows={takeovers}
        getRowKey={(takeover) => takeover.id}
        rowHasInteractiveControls={released}
        stackedOnMobile
      />
    </SectionCard>
  );
}
