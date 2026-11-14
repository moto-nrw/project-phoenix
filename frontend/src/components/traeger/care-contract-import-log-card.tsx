"use client";

// Die bisherigen Vertragsimporte (#4723): welche Datei, wann, von wem und wie
// viele Verträge. Nur zur Information; die Datei selbst bewahrt moto nicht auf.

import { useEffect, useState } from "react";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { SectionCard } from "~/components/ui/section-card";
import { formatBerlinDate, formatBerlinTime } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import { fetchTraegerBuero } from "~/lib/traeger-api";
import type { ContractImportLogEntry } from "~/lib/traeger-care-contract-import-api";

const logger = createLogger({ component: "CareContractImportLogCard" });

export function CareContractImportLogCard({
  entries,
}: Readonly<{ entries: readonly ContractImportLogEntry[] }>) {
  const [emails, setEmails] = useState<ReadonlyMap<string, string>>(new Map());

  useEffect(() => {
    if (entries.length === 0) return;
    let current = true;
    // Die Personen des Träger-Büros nennen die Konten; ohne Leserecht
    // bleibt die Kontonummer.
    fetchTraegerBuero()
      .then((buero) => {
        if (current) {
          setEmails(new Map(buero.members.map((m) => [m.accountId, m.email])));
        }
      })
      .catch((err: unknown) => {
        logger.warn("contract_import_log_people_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
      });
    return () => {
      current = false;
    };
  }, [entries.length]);

  const columns: DataTableColumn<ContractImportLogEntry>[] = [
    {
      key: "file",
      header: "Datei",
      render: (entry) => (
        <span className="font-medium wrap-anywhere text-gray-900">
          {entry.file}
        </span>
      ),
      stacked: "title",
    },
    {
      key: "at",
      header: "Übernommen am",
      render: (entry) =>
        `${formatBerlinDate(entry.importedAt)}, ${formatBerlinTime(entry.importedAt)} Uhr`,
      sortValue: (entry) => entry.importedAt,
    },
    {
      key: "by",
      header: "Von",
      render: (entry) => (
        <span className="wrap-anywhere">
          {entry.accountId === null
            ? "Gelöschtes Konto"
            : (emails.get(entry.accountId) ?? `Konto ${entry.accountId}`)}
        </span>
      ),
    },
    {
      key: "count",
      header: "Verträge",
      align: "right",
      render: (entry) => (
        <span className="tabular-nums">{entry.contracts}</span>
      ),
      stacked: "meta",
    },
  ];

  return (
    <SectionCard
      title="Bisherige Importe"
      description="Nur zur Information. Jeder übernommene Vertrag merkt sich die Datei und die Zeile."
    >
      {entries.length === 0 ? (
        <p className="text-sm text-gray-600">Noch keine Verträge importiert.</p>
      ) : (
        <DataTable
          columns={columns}
          rows={[...entries]}
          getRowKey={(entry) =>
            `${entry.file}|${entry.importedAt}|${entry.accountId ?? ""}`
          }
          stackedOnMobile
          stackedLayout="lines"
          dense
        />
      )}
    </SectionCard>
  );
}
