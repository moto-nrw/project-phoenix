"use client";

// Ein Dokument des Trägers (#4692): für wen es freigegeben ist, eine neue
// Freigabe und das Zurücknehmen einer Freigabe. Zurückgenommen sehen Eltern
// und Schulen das Dokument sofort nicht mehr.

import { DownloadSimpleIcon, ShareNetworkIcon } from "@phosphor-icons/react";
import { useSession } from "next-auth/react";
import { useParams } from "next/navigation";
import { useMemo, useState } from "react";
import {
  documentFileFacts,
  releaseAudienceLabel,
  schoolNamesOf,
} from "~/components/traeger/traeger-document-format";
import { TraegerDocumentReleaseModal } from "~/components/traeger/traeger-document-release-modal";
import {
  useTraegerDocumentAudiences,
  useTraegerDocuments,
} from "~/components/traeger/use-traeger-documents";
import { useTraegerSchools } from "~/components/traeger/use-traeger-schools";
import { Button } from "~/components/ui/button";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { EmptyState } from "~/components/ui/empty-state";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { ResponsiveActionList } from "~/components/ui/responsive-action-list";
import { SectionCard } from "~/components/ui/section-card";
import { TenantPage } from "~/components/ui/tenant-page";
import {
  useApiErrorDisplay,
  useApiFormError,
  useToast,
} from "~/contexts/ToastContext";
import { hasPermission } from "~/lib/auth-utils";
import { documentsConcept } from "~/lib/concepts/documents";
import { parentsConcept } from "~/lib/concepts/parents";
import { schoolsConcept } from "~/lib/concepts/schools";
import { formatBerlinDate } from "~/lib/date-helpers";
import { downloadBlob } from "~/lib/file-download";
import { createLogger } from "~/lib/logger";
import {
  downloadTraegerDocument,
  revokeTraegerDocumentRelease,
  type TraegerDocumentRelease,
} from "~/lib/traeger-documents-api";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "TraegerDocumentPage" });

/** Das Recht, das Freigeben und Zurücknehmen öffnet (Verwaltung). */
const MANAGE_RIGHT = "traeger_enrollment:manage";

export default function TraegerDocumentPage() {
  const params = useParams<{ id: string }>();
  const id = params?.id ?? "";
  const { data: session } = useSession();
  const canManage = hasPermission(session ?? null, MANAGE_RIGHT);
  const { data, isLoading, loadError, mutate } = useTraegerDocuments();
  const { schools } = useTraegerSchools();
  const audiences = useTraegerDocumentAudiences(canManage);
  const { success } = useToast();
  const downloadErrors = useApiErrorDisplay();
  const revokeErrors = useApiFormError();
  const [releasing, setReleasing] = useState(false);
  const [pending, setPending] = useState<TraegerDocumentRelease | null>(null);
  const [revoking, setRevoking] = useState(false);
  const [downloading, setDownloading] = useState(false);

  const document = data?.find((candidate) => candidate.id === id) ?? null;
  const names = useMemo(() => schoolNamesOf(schools ?? []), [schools]);
  const missing = !isLoading && !loadError && data !== undefined && !document;

  const download = async () => {
    if (!document) return;
    setDownloading(true);
    try {
      const { blob, filename } = await downloadTraegerDocument(document.id);
      downloadBlob(blob, filename ?? document.fileName);
    } catch (err) {
      logger.error("traeger_document_download_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void downloadErrors.show(err, { object: "das Dokument" });
    } finally {
      setDownloading(false);
    }
  };

  const revoke = async () => {
    if (!document || !pending) return;
    setRevoking(true);
    revokeErrors.clear();
    try {
      await revokeTraegerDocumentRelease(document.id, pending.id);
      success(
        `Freigabe für ${releaseAudienceLabel(pending, names)} zurückgenommen.`,
      );
      setPending(null);
      await mutate();
    } catch (err) {
      logger.error("traeger_document_revoke_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void revokeErrors.show(err, { object: "das Zurücknehmen" });
    } finally {
      setRevoking(false);
    }
  };

  return (
    <TenantPage
      title={document?.title ?? "Dokument"}
      back
      backHref={traegerPath("/traeger/dokumente")}
      backLabel="Zurück zu den Dokumenten"
      stats={
        document ? (
          <span className="flex flex-wrap items-center gap-x-2 gap-y-1">
            <span className="truncate">{document.fileName}</span>
            <span aria-hidden="true">·</span>
            <span>{documentFileFacts(document)}</span>
            <span aria-hidden="true">·</span>
            <span>hochgeladen am {formatBerlinDate(document.createdAt)}</span>
            <span aria-hidden="true">·</span>
            <span>
              {document.reads === 0
                ? "noch nicht geöffnet"
                : `${document.reads}-mal geöffnet`}
            </span>
          </span>
        ) : null
      }
      statsLoading={isLoading}
      error={loadError}
      loading={isLoading && !document}
      empty={
        missing
          ? {
              title: "Dieses Dokument gibt es nicht.",
              description:
                "Vielleicht gehört es zu einem anderen Träger. Gehen Sie zurück zu den Dokumenten.",
            }
          : null
      }
      actions={
        document ? (
          <>
            <Button
              type="button"
              variant="outline"
              size="md"
              onClick={() => void download()}
              disabled={downloading}
            >
              <DownloadSimpleIcon className="h-4 w-4" aria-hidden="true" />
              {downloading ? "Wird geladen …" : "Herunterladen"}
            </Button>
            {canManage && (
              <Button
                type="button"
                variant="primary"
                size="md"
                onClick={() => setReleasing(true)}
              >
                <ShareNetworkIcon className="h-4 w-4" aria-hidden="true" />
                Freigeben
              </Button>
            )}
          </>
        ) : null
      }
      overlays={
        document ? (
          <>
            <TraegerDocumentReleaseModal
              isOpen={releasing}
              documentId={document.id}
              documentTitle={document.title}
              schools={audiences.data ?? []}
              schoolsLoading={audiences.isLoading}
              onClose={() => setReleasing(false)}
              onReleased={(release) => {
                setReleasing(false);
                success(
                  `Freigegeben für ${releaseAudienceLabel(release, names)}.`,
                );
                void mutate();
              }}
            />
            <ConfirmDeleteModal
              isOpen={pending !== null}
              onClose={() => {
                if (revoking) return;
                revokeErrors.clear();
                setPending(null);
              }}
              onConfirm={() => void revoke()}
              title="Freigabe zurücknehmen?"
              description={
                <>
                  <span className="font-medium">
                    {pending ? releaseAudienceLabel(pending, names) : ""}
                  </span>{" "}
                  sehen „{document.title}“ danach nicht mehr. Das Dokument
                  selbst bleibt hier gespeichert.
                </>
              }
              confirmLabel="Zurücknehmen"
              loadingLabel="Wird zurückgenommen …"
              gate={{ mode: "twoStep", firstStepLabel: "Ja, zurücknehmen" }}
              loading={revoking}
              error={revokeErrors.error}
            />
          </>
        ) : null
      }
    >
      {document ? (
        <SectionCard
          title="Freigegeben für"
          concept={documentsConcept}
          description={
            document.releases.length === 0
              ? undefined
              : "Diese Menschen finden das Dokument: Eltern in der Eltern-App unter „Dokumente“, die OGS-Leitung in moto unter „Dateien“."
          }
        >
          {document.releases.length === 0 ? (
            <EmptyState
              title="Noch nicht freigegeben."
              description={
                canManage
                  ? "Noch sieht niemand dieses Dokument. Geben Sie es oben frei: für Eltern oder für Schulen."
                  : "Noch sieht niemand dieses Dokument. Freigeben kann die Verwaltung."
              }
            />
          ) : (
            <ResponsiveActionList>
              {document.releases.map((release) => (
                <li
                  key={release.id}
                  className="flex flex-col gap-2 py-3 first:pt-0 last:pb-0 sm:col-span-2 sm:grid sm:grid-cols-subgrid sm:items-center"
                >
                  <div className="flex min-w-0 items-center gap-3">
                    <MotoConceptIcon
                      concept={
                        release.audience === "parents"
                          ? parentsConcept
                          : schoolsConcept
                      }
                      size={20}
                      className="shrink-0"
                    />
                    <div className="min-w-0">
                      <p className="text-sm font-semibold text-gray-900">
                        {releaseAudienceLabel(release, names)}
                      </p>
                      <p className="mt-0.5 text-sm text-gray-600">
                        Freigegeben am {formatBerlinDate(release.releasedAt)}
                      </p>
                    </div>
                  </div>
                  {canManage ? (
                    <div className="flex w-full flex-col gap-2 min-[480px]:w-auto min-[480px]:flex-row sm:justify-end">
                      <Button
                        type="button"
                        variant="outline_danger"
                        size="compact"
                        aria-label={`Freigabe zurücknehmen: ${releaseAudienceLabel(release, names)}`}
                        onClick={() => {
                          revokeErrors.clear();
                          setPending(release);
                        }}
                      >
                        Zurücknehmen
                      </Button>
                    </div>
                  ) : null}
                </li>
              ))}
            </ResponsiveActionList>
          )}
        </SectionCard>
      ) : null}
    </TenantPage>
  );
}
