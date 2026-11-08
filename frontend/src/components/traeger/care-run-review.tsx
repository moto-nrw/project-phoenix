"use client";

// Die Prüfphase eines Monatslaufs vor der Freigabe (#5916, SJR-Abgleich E1
// A): das Menü „Ausschließen“ je Rechnung, die Karte der ausgeschlossenen
// Rechnungen mit dem Weg zurück und die Prüfliste als Excel oder CSV. Eine
// ausgeschlossene Rechnung bekommt bei der Freigabe keine Nummer; ihr Kind
// kommt im nächsten Lauf desselben Monats wieder. Die Dialoge lädt die
// Seite erst, wenn jemand sie öffnet.

import { useState } from "react";
import { Button } from "~/components/ui/button";
import type { DataTableColumn } from "~/components/ui/data-table";
import {
  type ExportMenuOption,
  ExportMenu,
  exportFormatOptions,
} from "~/components/ui/export-menu";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { ResponsiveActionList } from "~/components/ui/responsive-action-list";
import { SectionCard } from "~/components/ui/section-card";
import { useApiErrorDisplay } from "~/contexts/ToastContext";
import { formatDate } from "~/lib/date-helpers";
import { downloadBlob } from "~/lib/file-download";
import { createLogger } from "~/lib/logger";
import type { CareInvoice } from "~/lib/traeger-care-billing-api";
import {
  type CareReviewFormat,
  downloadCareRunReview,
} from "~/lib/traeger-care-review-api";
import { invoiceCountLabel } from "./care-billing-format";
import type { CareInvoiceRow } from "./care-run-invoices";
import { formatEuro } from "./contribution-format";

const logger = createLogger({ component: "CareRunReview" });

const REVIEW_FORMATS: readonly ExportMenuOption[] = exportFormatOptions([
  "xlsx",
  "csv",
]);

function childName(invoice: CareInvoice): string {
  return `${invoice.childFirstName} ${invoice.childLastName}`.trim();
}

/** The row menu of a preview invoice: leave it out of the release. */
export function exclusionColumn(
  onExclude: (invoice: CareInvoice) => void,
): DataTableColumn<CareInvoiceRow> {
  return {
    key: "actions",
    header: "",
    align: "right",
    stacked: "meta",
    stackedLabel: false,
    className: "w-12 align-top",
    render: ({ invoice }) => (
      <OverflowMenu
        ariaLabel={`Aktionen für die Rechnung von ${childName(invoice)}`}
        triggerSize="sm"
        items={[
          {
            label: "Ausschließen",
            description: "Bekommt bei der Freigabe keine Nummer",
            onClick: () => onExclude(invoice),
          },
        ]}
      />
    ),
  };
}

/** „Prüfliste“: the preview as Excel or CSV, without invoice numbers. */
export function CareReviewExport({ runId }: Readonly<{ runId: string }>) {
  const [busy, setBusy] = useState<string | null>(null);
  const errors = useApiErrorDisplay();
  const download = async (format: CareReviewFormat) => {
    setBusy(format);
    try {
      const file = await downloadCareRunReview(runId, format);
      downloadBlob(file.blob, file.filename);
    } catch (err) {
      logger.error("traeger_care_review_export_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void errors.show(err, { object: "die Prüfliste" });
    } finally {
      setBusy(null);
    }
  };
  return (
    <ExportMenu
      label="Prüfliste"
      ariaLabel="Prüfliste herunterladen"
      options={REVIEW_FORMATS}
      busyKey={busy}
      onSelect={(option) => {
        if (option.format === "xlsx" || option.format === "csv") {
          void download(option.format);
        }
      }}
    />
  );
}

/**
 * The invoices left out of the release, each with its reason and the way
 * back into the run.
 */
export function CareExcludedCard({
  invoices,
  returning,
  onInclude,
}: Readonly<{
  invoices: readonly CareInvoice[];
  /** The contract whose invoice is on its way back. */
  returning: string | null;
  onInclude: (invoice: CareInvoice) => void;
}>) {
  if (invoices.length === 0) return null;
  const total = invoices.reduce((sum, invoice) => sum + invoice.totalCents, 0);
  return (
    <SectionCard
      title="Ausgeschlossen"
      titleBadge={
        <span className="text-sm font-normal text-gray-600 tabular-nums">
          {invoiceCountLabel(invoices.length)} · {formatEuro(total)}
        </span>
      }
      description="Diese Rechnungen bekommen bei der Freigabe keine Nummer. Das Kind kommt im nächsten Lauf dieses Monats wieder."
    >
      <ResponsiveActionList>
        {invoices.map((invoice) => (
          <li
            key={invoice.contractId}
            className="flex flex-col gap-2 py-3 first:pt-0 last:pb-0 sm:col-span-2 sm:grid sm:grid-cols-subgrid sm:items-center"
          >
            <div className="min-w-0">
              <p className="flex flex-wrap items-baseline gap-x-2 text-sm">
                <span className="font-semibold text-gray-900">
                  {childName(invoice)}
                </span>
                <span className="text-gray-600">{invoice.schoolName}</span>
                <span className="text-gray-500 tabular-nums">
                  {formatEuro(invoice.totalCents)}
                </span>
              </p>
              <p className="mt-0.5 text-sm wrap-anywhere text-gray-700">
                Grund: {invoice.exclusion?.reason}
              </p>
              {invoice.exclusion ? (
                <p className="mt-0.5 text-xs text-gray-500">
                  Ausgeschlossen am {formatDate(invoice.exclusion.excludedAt)}
                </p>
              ) : null}
            </div>
            <div className="flex w-full flex-col gap-2 min-[480px]:w-auto min-[480px]:flex-row sm:justify-end">
              <Button
                type="button"
                variant="surface"
                size="compact"
                isLoading={returning === invoice.contractId}
                loadingText="Wird aufgenommen …"
                disabled={returning !== null}
                aria-label={`Wieder aufnehmen: Rechnung von ${childName(invoice)}`}
                onClick={() => onInclude(invoice)}
              >
                Wieder aufnehmen
              </Button>
            </div>
          </li>
        ))}
      </ResponsiveActionList>
    </SectionCard>
  );
}
