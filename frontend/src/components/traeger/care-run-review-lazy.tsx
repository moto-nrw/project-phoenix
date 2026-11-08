"use client";

// Die Bausteine der Prüfphase (#5916) lädt die Laufseite erst, wenn ein Lauf
// in der Prüfphase ist. Ein freigegebener Lauf und die Liste laden ihren
// Code nicht.

import dynamic from "next/dynamic";
import type { DataTableColumn } from "~/components/ui/data-table";
import type { CareInvoice } from "~/lib/traeger-care-billing-api";
import type { CareInvoiceRow } from "./care-run-invoices";

export const CareReviewExport = dynamic(
  () => import("./care-run-review").then((module) => module.CareReviewExport),
  { ssr: false },
);

export const CareExcludedCard = dynamic(
  () => import("./care-run-review").then((module) => module.CareExcludedCard),
  { ssr: false },
);

export const CareDebitHintsCard = dynamic(
  () => import("./care-run-review").then((module) => module.CareDebitHintsCard),
  { ssr: false },
);

const ExclusionMenu = dynamic(
  () => import("./care-run-review").then((module) => module.ExclusionMenu),
  { ssr: false },
);

/** The row menu column of a preview: leave the invoice out of the release,
 * or open the recipient and payment of its family (#4689). */
export function exclusionColumn(
  onExclude: (invoice: CareInvoice) => void,
  onFamilyBilling?: (invoice: CareInvoice) => void,
): DataTableColumn<CareInvoiceRow> {
  return {
    key: "actions",
    header: "",
    align: "right",
    stacked: "meta",
    stackedLabel: false,
    className: "w-12 align-top",
    render: ({ invoice }) => (
      <ExclusionMenu
        invoice={invoice}
        onExclude={onExclude}
        onFamilyBilling={onFamilyBilling}
      />
    ),
  };
}
