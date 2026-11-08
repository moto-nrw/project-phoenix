"use client";

// Kostenträger im Monatslauf (#4687): je Kostenträger, für wie viele
// Rechnungen er einen Anteil übernimmt und welchen Betrag. Die Liste je
// Kostenträger lädt die Abrechnung als PDF, Excel oder CSV herunter, um den
// Anteil getrennt abzurechnen. Vor der Freigabe ist sie eine Vorschau ohne
// Rechnungsnummern. Die Seite lädt die Karte erst, wenn ein Kostenträger im
// Lauf etwas übernimmt.

import { HandCoins } from "lucide-react";
import { StatusBadge } from "~/components/ui/status-badge";
import { useApiErrorDisplay } from "~/contexts/ToastContext";
import { downloadBlob } from "~/lib/file-download";
import { createLogger } from "~/lib/logger";
import type { CareCostBearerTotal } from "~/lib/traeger-care-billing-api";
import { downloadCareCostShareList } from "~/lib/traeger-care-cost-shares-api";
import { formatEuro } from "./contribution-format";
import {
  type CostBearerListFormat,
  type CostBearerTotal,
  CostBearerTotalsCard,
} from "./cost-bearer-totals-card";

const logger = createLogger({ component: "CareCostBearerCard" });

export function CareCostBearerCard({
  runId,
  released,
  totals,
}: Readonly<{
  runId: string;
  released: boolean;
  totals: readonly CareCostBearerTotal[];
}>) {
  const { show: showActionError } = useApiErrorDisplay();
  const rows: CostBearerTotal[] = totals.map((total) => ({
    id: total.costBearerId,
    name: total.costBearerName,
    documents: total.invoiceCount,
    cents: total.amountCents,
  }));
  const sum = rows.reduce((total, row) => total + row.cents, 0);

  const download = async (
    bearer: CostBearerTotal,
    format: CostBearerListFormat,
  ) => {
    try {
      const file = await downloadCareCostShareList(runId, bearer.id, format);
      downloadBlob(file.blob, file.filename);
    } catch (err) {
      logger.error("traeger_care_cost_share_export_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showActionError(err, { object: `die Liste für ${bearer.name}` });
    }
  };

  return (
    <CostBearerTotalsCard
      icon={HandCoins}
      title="Kostenträger"
      titleBadge={
        <StatusBadge tone="blue" label={`${formatEuro(sum)} übernommen`} />
      }
      amountHeader="Übernimmt"
      description={
        released
          ? "Diesen Teil der Beiträge bezahlen Kostenträger, nicht die Familien. Die Liste für jeden Kostenträger steht im Menü der Zeile."
          : "Diesen Teil der Beiträge bezahlen Kostenträger, nicht die Familien. Vor der Freigabe ist die Liste eine Vorschau ohne Rechnungsnummern."
      }
      totals={rows}
      list={{
        header: released ? "Liste für den Kostenträger" : "Vorschau",
        ariaLabel: (bearer) => `Liste für ${bearer.name} herunterladen`,
        download,
      }}
    />
  );
}
