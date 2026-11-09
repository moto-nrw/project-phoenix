"use client";

// Vorlagen des Trägers (#4677, ADR 0152): für jede Dokumentart Betreff,
// Anrede, Brieftext und Grußformel mit Platzhaltern. Die Liste zeigt, ob eine
// Art die Standardvorlage oder eine eigene Version nutzt. Lesen darf, wer
// „Übersicht lesen“ hat, ändern nur, wer „Einstellungen“ hat.

import { FileText } from "lucide-react";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import { StatusBadge } from "~/components/ui/status-badge";
import { TenantPage, TenantPageStats } from "~/components/ui/tenant-page";
import { useApiLoadError } from "~/contexts/ToastContext";
import { formatDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  DOCUMENT_TEMPLATE_KIND_LABELS,
  DOCUMENT_TEMPLATE_KIND_USE,
  type DocumentTemplateSummary,
  fetchDocumentTemplates,
} from "~/lib/traeger-document-templates";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "TraegerVorlagenPage" });

const COLUMNS: DataTableColumn<DocumentTemplateSummary>[] = [
  {
    key: "kind",
    header: "Dokument",
    render: (template) => (
      <div className="min-w-0">
        <p className="font-medium text-gray-900">
          {DOCUMENT_TEMPLATE_KIND_LABELS[template.kind]}
        </p>
        <p className="text-sm text-gray-600">
          {DOCUMENT_TEMPLATE_KIND_USE[template.kind]}
        </p>
      </div>
    ),
    stacked: "title",
  },
  {
    key: "state",
    header: "Vorlage",
    render: (template) =>
      template.standard ? (
        <StatusBadge label="Standardvorlage" tone="gray" />
      ) : (
        <StatusBadge
          label={`Eigene, Version ${template.version}`}
          tone="green"
        />
      ),
    stacked: "meta",
  },
  {
    key: "updated",
    header: "Gespeichert am",
    render: (template) =>
      template.updatedAt ? formatDate(template.updatedAt) : "–",
    stacked: "meta",
  },
];

export default function TraegerVorlagenPage() {
  const router = useRouter();
  const [templates, setTemplates] = useState<DocumentTemplateSummary[] | null>(
    null,
  );
  const pageLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = pageLoad;

  const load = useCallback(async () => {
    try {
      setTemplates(await fetchDocumentTemplates());
      clearLoadError();
    } catch (err) {
      logger.error("traeger_document_templates_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "die Liste der Vorlagen",
        retry: () => void load(),
      });
    }
  }, [clearLoadError, showLoadError]);

  useEffect(() => {
    void load();
  }, [load]);

  const own = templates?.filter((template) => !template.standard).length ?? 0;

  return (
    <TenantPage
      title="Vorlagen"
      back
      backHref={traegerPath("/traeger/einstellungen/briefkopf")}
      backLabel="Zurück zu Briefkopf und Vorlagen"
      info="Hier legen Sie fest, was in den Briefen Ihres Trägers steht. Platzhalter wie {{familienname}} füllt moto für jede Familie aus. Logo, Absender und Fußzeile kommen aus dem Briefkopf."
      stats={
        templates ? (
          <TenantPageStats
            items={[
              { value: templates.length, label: "Dokumente" },
              {
                value: own,
                label: own === 1 ? "eigene Vorlage" : "eigene Vorlagen",
              },
            ]}
          />
        ) : null
      }
      statsLoading={!templates && !pageLoad.error}
      error={pageLoad.error}
      loading={!pageLoad.error && !templates}
    >
      {templates ? (
        <DataTable
          columns={COLUMNS}
          rows={templates}
          getRowKey={(template) => template.kind}
          stackedOnMobile
          stackedLayout="row"
          onRowClick={(template) =>
            router.push(
              traegerPath(
                `/traeger/einstellungen/briefkopf/vorlagen/${template.kind}`,
              ),
            )
          }
          emptyState={
            <EmptyState
              icon={<FileText className="h-12 w-12" aria-hidden="true" />}
              title="Keine Vorlagen."
            />
          }
        />
      ) : null}
    </TenantPage>
  );
}
