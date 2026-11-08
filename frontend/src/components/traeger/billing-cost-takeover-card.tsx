"use client";

// Kostenübernahme in einem Abrechnungslauf (#5627): je Kostenträger, wie
// viele Rechnungen er übernimmt und welchen Betrag. Nach der Freigabe lädt
// die Abrechnung je Kostenträger die Liste für die Erstattung als PDF, Excel
// oder CSV herunter. Die Karte erscheint nur, wenn ein Kostenträger im Lauf
// etwas übernimmt.

import { useApiErrorDisplay } from "~/contexts/ToastContext";
import { downloadBlob } from "~/lib/file-download";
import { createLogger } from "~/lib/logger";
import {
  type BillingRunDetail,
  type BillingRunExportFormat,
  downloadCostTakeoverList,
} from "~/lib/traeger-billing-api";
import {
  type CostBearerTotal,
  CostBearerTotalsCard,
} from "./cost-bearer-totals-card";

const logger = createLogger({ component: "BillingCostTakeoverCard" });

/**
 * What each cost bearer pays in the run, in the order it first appears. A
 * Storno negates its invoice, the takeover too (#4690): it reverses the
 * amount and takes the invoice out of the count; a cost bearer with nothing
 * left drops out.
 */
export function runCostTakeovers(run: BillingRunDetail): CostBearerTotal[] {
  const takeovers = new Map<string, CostBearerTotal>();
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
  const { show: showActionError } = useApiErrorDisplay();
  const released = run.status === "released";

  const download = async (
    takeover: CostBearerTotal,
    format: BillingRunExportFormat,
  ) => {
    try {
      const file = await downloadCostTakeoverList(run.id, takeover.id, format);
      downloadBlob(file.blob, file.filename);
    } catch (err) {
      logger.error("traeger_cost_takeover_export_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showActionError(err, { object: "die Liste für die Erstattung" });
    }
  };

  return (
    <CostBearerTotalsCard
      title="Kostenübernahme"
      description={
        released
          ? "Diese Beträge bezahlen Kostenträger statt der Familien. Die Liste für die Erstattung steht im Menü der Zeile."
          : "Diese Beträge bezahlen Kostenträger statt der Familien. Die Liste für die Erstattung gibt es nach der Freigabe."
      }
      totals={runCostTakeovers(run)}
      list={
        released
          ? {
              header: "Liste für die Erstattung",
              ariaLabel: (takeover) =>
                `Liste für die Erstattung durch ${takeover.name} herunterladen`,
              download,
            }
          : undefined
      }
    />
  );
}
