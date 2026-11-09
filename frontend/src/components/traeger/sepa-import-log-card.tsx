"use client";

// Die bisherigen Mandatsimporte (#6198): welche Datei, wann, von wem und wie
// viele Mandate. Nur zur Information; die Datei selbst bewahrt moto nicht auf.

import { useEffect, useState } from "react";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { SectionCard } from "~/components/ui/section-card";
import { formatBerlinDate, formatBerlinTime } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import { fetchTraegerBuero } from "~/lib/traeger-api";
import type { ImportLogEntry } from "~/lib/traeger-sepa-import-api";

const logger = createLogger({ component: "SepaImportLogCard" });

export function SepaImportLogCard({
  entries,
}: Readonly<{ entries: readonly ImportLogEntry[] }>) {
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
        logger.warn("sepa_import_log_people_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
      });
    return () => {
      current = false;
    };
  }, [entries.length]);

  const columns: DataTableColumn<ImportLogEntry>[] = [
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
      header: "Mandate",
      align: "right",
      render: (entry) => <span className="tabular-nums">{entry.mandates}</span>,
      stacked: "meta",
    },
  ];

  return (
    <SectionCard
      title="Bisherige Importe"
      description="Nur zur Information. Jedes übernommene Mandat zeigt in seinem Verlauf die Datei und die Zeile."
    >
      {entries.length === 0 ? (
        <p className="text-sm text-gray-600">Noch keine Mandate importiert.</p>
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
