"use client";

// Dokumente, die der Träger für die Schule oder die Eltern an ihr freigegeben
// hat (#4692, Spec #4611 Story 99). Die OGS-Leitung liest und lädt sie
// herunter; ändern kann sie nur der Träger. Zieht der Träger eine Freigabe
// zurück, verschwindet das Dokument hier.

import { DownloadSimpleIcon } from "@phosphor-icons/react";
import { useState } from "react";
import {
  documentFileFacts,
  schoolReleaseAudienceLabel,
} from "~/components/traeger/traeger-document-format";
import { Button } from "~/components/ui/button";
import { FileTypeIcon } from "~/components/ui/file-type-icon";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { ResponsiveActionList } from "~/components/ui/responsive-action-list";
import { SectionCard } from "~/components/ui/section-card";
import { useApiErrorDisplay } from "~/contexts/ToastContext";
import { documentsConcept } from "~/lib/concepts/documents";
import { formatBerlinDate } from "~/lib/date-helpers";
import { downloadBlob } from "~/lib/file-download";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { createLogger } from "~/lib/logger";
import { useSWRAuth } from "~/lib/swr";
import { schoolTraegerDocumentKeys } from "~/lib/swr/portal-keys";
import {
  downloadSchoolTraegerDocument,
  fetchSchoolTraegerDocuments,
  type SharedTraegerDocument,
} from "~/lib/traeger-documents-api";

const logger = createLogger({ component: "TraegerDocumentsCard" });

/**
 * Die Dokumente des Trägers für die eigene Schule. Nur wer die Dateien der
 * Schule verwaltet (OGS-Leitung), lädt sie; `enabled` ist sonst falsch.
 */
export function useSchoolTraegerDocuments(enabled: boolean) {
  const { data, error, mutate } = useSWRAuth<SharedTraegerDocument[]>(
    enabled ? schoolTraegerDocumentKeys.list() : null,
    fetchSchoolTraegerDocuments,
  );
  const loadError = useSwrLoadError(error, "die Dokumente des Trägers", mutate);
  return { documents: data ?? [], loadError, mutate };
}

/** Die Dokumente des Trägers auf „Dateien“; ohne Dokument fehlt die Karte. */
export function TraegerDocumentsCard({
  documents,
  loadError,
  mutate,
}: Readonly<ReturnType<typeof useSchoolTraegerDocuments>>) {
  const downloadErrors = useApiErrorDisplay();
  const [busyId, setBusyId] = useState<string | null>(null);

  if (loadError === null && documents.length === 0) return null;

  const download = async (document: SharedTraegerDocument) => {
    setBusyId(document.id);
    try {
      const { blob, filename } = await downloadSchoolTraegerDocument(
        document.id,
      );
      downloadBlob(blob, filename ?? document.fileName);
    } catch (err) {
      logger.error("traeger_document_download_failed", {
        document_id: document.id,
        error: err instanceof Error ? err.message : String(err),
      });
      void downloadErrors.show(err, { object: "das Dokument" });
      // Ein zurückgenommenes Dokument verschwindet beim nächsten Laden.
      void mutate();
    } finally {
      setBusyId(null);
    }
  };

  return (
    <SectionCard
      title="Vom Träger"
      concept={documentsConcept}
      description="Diese Dokumente hat Ihr Träger für Ihre Schule oder für die Eltern Ihrer Schule freigegeben. Ändern kann sie nur der Träger."
      titleBadge={
        documents.length > 0 ? (
          <span className="text-sm text-gray-500 tabular-nums">
            {documents.length}
          </span>
        ) : undefined
      }
    >
      {loadError ? (
        <LoadErrorAlert error={loadError} actionLayout="responsive" />
      ) : (
        <ResponsiveActionList>
          {documents.map((document) => (
            <li
              key={document.id}
              className="flex flex-col gap-2 py-3 first:pt-0 last:pb-0 sm:col-span-2 sm:grid sm:grid-cols-subgrid sm:items-center"
            >
              <div className="flex min-w-0 items-start gap-3">
                <FileTypeIcon
                  contentType={document.contentType}
                  className="mt-0.5 h-5 w-5 shrink-0 text-gray-400"
                />
                <div className="min-w-0">
                  <p className="truncate text-sm font-semibold text-gray-900">
                    {document.title}
                  </p>
                  <p className="mt-0.5 text-sm text-gray-600">
                    Für{" "}
                    {document.releases
                      .map(schoolReleaseAudienceLabel)
                      .join(", ")}{" "}
                    · seit {formatBerlinDate(document.releasedAt)}
                  </p>
                  <p className="text-xs text-gray-500">
                    {document.fileName} · {documentFileFacts(document)}
                  </p>
                </div>
              </div>
              <div className="flex w-full flex-col gap-2 min-[480px]:w-auto min-[480px]:flex-row sm:justify-end">
                <Button
                  type="button"
                  variant="surface"
                  size="compact"
                  aria-label={`Herunterladen: ${document.title}`}
                  disabled={busyId === document.id}
                  onClick={() => void download(document)}
                >
                  <DownloadSimpleIcon className="h-4 w-4" aria-hidden="true" />
                  Herunterladen
                </Button>
              </div>
            </li>
          ))}
        </ResponsiveActionList>
      )}
    </SectionCard>
  );
}
