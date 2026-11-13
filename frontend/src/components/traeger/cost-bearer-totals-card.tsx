"use client";

// Was jeder Kostenträger in einem Lauf übernimmt (#5627 Ferien, #4687
// Monatsbeitrag): eine Zeile je Kostenträger mit Zahl der Rechnungen und
// Betrag, im Menü der Zeile die Liste als PDF, Excel oder CSV. Ferien- und
// Monatslauf teilen diese Karte; jeder Lauf liefert seine Zeilen, Texte und
// den Download.

import type { LucideIcon } from "lucide-react";
import { type ReactNode, useState } from "react";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import {
  exportFormatOptions,
  exportMenuItems,
} from "~/components/ui/export-menu";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { SectionCard } from "~/components/ui/section-card";
import { formatEuro } from "./contribution-format";

const FORMATS = exportFormatOptions(["pdf", "xlsx", "csv"]);

export type CostBearerListFormat = "pdf" | "xlsx" | "csv";

export interface CostBearerTotal {
  readonly id: string;
  readonly name: string;
  readonly documents: number;
  readonly cents: number;
}

export function CostBearerTotalsCard({
  title,
  description,
  icon,
  titleBadge,
  amountHeader = "Betrag",
  totals,
  list,
}: Readonly<{
  title: string;
  description: ReactNode;
  icon?: LucideIcon;
  titleBadge?: ReactNode;
  amountHeader?: string;
  totals: readonly CostBearerTotal[];
  /** The download per row; without it the rows carry no menu. */
  list?: {
    readonly header: string;
    readonly ariaLabel: (total: CostBearerTotal) => string;
    readonly download: (
      total: CostBearerTotal,
      format: CostBearerListFormat,
    ) => Promise<void>;
  };
}>) {
  const [exporting, setExporting] = useState<string | null>(null);
  if (totals.length === 0) return null;

  const columns: DataTableColumn<CostBearerTotal>[] = [
    {
      key: "name",
      header: "Kostenträger",
      render: (total) => (
        <span className="font-medium wrap-anywhere text-gray-900">
          {total.name}
        </span>
      ),
      stacked: "title",
    },
    {
      key: "documents",
      header: "Rechnungen",
      render: (total) => String(total.documents),
      stacked: "field",
    },
    {
      key: "cents",
      header: amountHeader,
      align: "right",
      render: (total) => (
        <span className="tabular-nums">{formatEuro(total.cents)}</span>
      ),
      stacked: "meta",
      stackedTone: "figure",
    },
  ];
  if (list) {
    columns.push({
      key: "actions",
      header: "",
      align: "right",
      stacked: "meta",
      stackedLabel: false,
      render: (total) => (
        <OverflowMenu
          ariaLabel={list.ariaLabel(total)}
          items={exportMenuItems({
            header: list.header,
            options: FORMATS,
            busyKey: exporting?.startsWith(`${total.id}:`)
              ? exporting.slice(total.id.length + 1)
              : null,
            onSelect: (option) => {
              const format = option.format;
              if (format === "pdf" || format === "xlsx" || format === "csv") {
                setExporting(`${total.id}:${format}`);
                void list
                  .download(total, format)
                  .finally(() => setExporting(null));
              }
            },
          })}
        />
      ),
    });
  }

  return (
    <SectionCard
      icon={icon}
      title={title}
      titleBadge={titleBadge}
      description={description}
    >
      <DataTable
        columns={columns}
        rows={[...totals]}
        getRowKey={(total) => total.id}
        rowHasInteractiveControls={list !== undefined}
        stackedOnMobile
      />
    </SectionCard>
  );
}
