"use client";

// Ein Dokument des Trägers (#4692): für wen es freigegeben ist, eine neue
// Freigabe und das Zurücknehmen einer Freigabe. Zurückgenommen sehen Eltern
// und Schulen das Dokument sofort nicht mehr. Das Menü im Kopf löscht das
// Dokument mit allen Freigaben (#5993).

import {
  DownloadSimpleIcon,
  ShareNetworkIcon,
  TrashIcon,
} from "@phosphor-icons/react";
import { useSession } from "next-auth/react";
import { useParams, useRouter } from "next/navigation";
import { useMemo, useState } from "react";
import {
  documentFileFacts,
  releaseAudienceLabel,
  revokeConsequence,
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
import { FramedList } from "~/components/ui/framed-list";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
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
  deleteTraegerDocument,
  downloadTraegerDocument,
  revokeTraegerDocumentRelease,
  type TraegerDocumentRelease,
} from "~/lib/traeger-documents-api";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "TraegerDocumentPage" });

/** Das Recht, das Freigeben, Zurücknehmen und Löschen öffnet (Rolle Dokumente, #5992). */
const MANAGE_RIGHT = "traeger_documents:manage";

export default function TraegerDocumentPage() {
  const params = useParams<{ id: string }>();
  const router = useRouter();
  const id = params?.id ?? "";
  const { data: session } = useSession();
  const canManage = hasPermission(session ?? null, MANAGE_RIGHT);
  const { data, isLoading, loadError, mutate } = useTraegerDocuments();
  // Without the school names two releases to different schools read alike;
  // the page waits for them and shows their failed load with a retry, so no
  // release is revoked by mistake.
  const { schools, error: schoolsError } = useTraegerSchools();
  const schoolsPending = schools === null && !schoolsError;
  const audiences = useTraegerDocumentAudiences(canManage);
  const { success } = useToast();
  const downloadErrors = useApiErrorDisplay();
  const revokeErrors = useApiFormError();
  const deleteErrors = useApiFormError();
  const [releasing, setReleasing] = useState(false);
  const [pending, setPending] = useState<TraegerDocumentRelease | null>(null);
  const [revoking, setRevoking] = useState(false);
  const [downloading, setDownloading] = useState(false);
  const [confirmingDelete, setConfirmingDelete] = useState(false);
  const [deleting, setDeleting] = useState(false);
  // After a deletion the page leaves for the list; until it is gone it
  // shows loading, never "Dieses Dokument gibt es nicht".
  const [deleted, setDeleted] = useState(false);

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

  const remove = async () => {
    if (!document) return;
    setDeleting(true);
    deleteErrors.clear();
    try {
      await deleteTraegerDocument(document.id);
      success(`„${document.title}“ gelöscht.`);
      setDeleted(true);
      setConfirmingDelete(false);
      router.replace(traegerPath("/traeger/dokumente"));
      void mutate();
    } catch (err) {
      logger.error("traeger_document_delete_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void deleteErrors.show(err, { object: "das Dokument" });
    } finally {
      setDeleting(false);
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
      error={loadError ?? schoolsError}
      loading={(isLoading && !document) || schoolsPending || deleted}
      empty={
        missing && !deleted
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
              className="gap-2"
              tooltip="Lädt das Dokument auf Ihr Gerät."
              onClick={() => void download()}
              isLoading={downloading}
              loadingText="Wird geladen …"
            >
              <DownloadSimpleIcon className="h-4 w-4" aria-hidden="true" />
              Herunterladen
            </Button>
            {canManage && (
              <Button
                type="button"
                variant="primary"
                size="md"
                className="gap-2"
                tooltip="Gibt das Dokument für Eltern oder Schulen frei, Eltern bekommen eine Nachricht."
                onClick={() => setReleasing(true)}
              >
                <ShareNetworkIcon className="h-4 w-4" aria-hidden="true" />
                Freigeben
              </Button>
            )}
            {/* Without the school names the dialog could not name the
                schools that lose the document; the page shows their failed
                load with a retry instead. */}
            {canManage && schools !== null && (
              <OverflowMenu
                ariaLabel="Weitere Aktionen zum Dokument"
                items={[
                  {
                    label: "Dokument löschen",
                    description:
                      "moto fragt vorher nach, dann enden alle Freigaben und die Datei ist weg.",
                    icon: <TrashIcon className="size-4" aria-hidden="true" />,
                    destructive: true,
                    onClick: () => {
                      deleteErrors.clear();
                      setConfirmingDelete(true);
                    },
                  },
                ]}
              />
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
              schoolsLoadError={audiences.loadError}
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
                  {pending
                    ? revokeConsequence(
                        pending,
                        document.releases,
                        names,
                        document.title,
                      )
                    : ""}{" "}
                  Das Dokument selbst bleibt hier gespeichert. Ganz löschen
                  können Sie es über das Menü oben.
                </>
              }
              confirmLabel="Zurücknehmen"
              loadingLabel="Wird zurückgenommen …"
              gate={{ mode: "twoStep", firstStepLabel: "Ja, zurücknehmen" }}
              loading={revoking}
              error={revokeErrors.error}
            />
            <ConfirmDeleteModal
              isOpen={confirmingDelete}
              onClose={() => {
                if (deleting) return;
                deleteErrors.clear();
                setConfirmingDelete(false);
              }}
              onConfirm={() => void remove()}
              title="Dokument löschen?"
              description={
                <div className="space-y-3">
                  <p>
                    <span className="font-medium text-gray-900">
                      „{document.title}“
                    </span>{" "}
                    wird gelöscht. Danach ist die Datei weg, auch hier im
                    Träger-Büro.
                  </p>
                  {document.releases.length > 0 ? (
                    <div className="space-y-2">
                      <FramedList
                        maxHeight="sm"
                        header={
                          <p className="text-sm font-medium text-gray-900">
                            Diese Freigaben enden sofort
                          </p>
                        }
                      >
                        {document.releases.map((release) => (
                          <li
                            key={release.id}
                            className="flex items-center gap-3 px-4 py-2.5"
                          >
                            <MotoConceptIcon
                              concept={
                                release.audience === "parents"
                                  ? parentsConcept
                                  : schoolsConcept
                              }
                              size={18}
                              className="shrink-0"
                            />
                            <span className="min-w-0 text-sm text-gray-900">
                              {releaseAudienceLabel(release, names)}
                            </span>
                          </li>
                        ))}
                      </FramedList>
                      <p>
                        Eltern und Schulen finden das Dokument dann nicht mehr.
                        Auch ein gespeicherter Link öffnet es nicht mehr.
                      </p>
                    </div>
                  ) : (
                    <p>Das Dokument ist für niemanden freigegeben.</p>
                  )}
                  <p>
                    Im Nachweis bleibt, wer es wann freigegeben, geöffnet und
                    gelöscht hat.
                  </p>
                </div>
              }
              gate={{
                mode: "textConfirm",
                expected: "LÖSCHEN",
                inputId: "confirm-delete-traeger-document",
                label: "Tippen Sie zur Bestätigung LÖSCHEN ein:",
                placeholder: "LÖSCHEN",
              }}
              confirmLabel="Endgültig löschen"
              loadingLabel="Wird gelöscht …"
              loading={deleting}
              error={deleteErrors.error}
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
              : `Diese Menschen finden das Dokument: Eltern in der Eltern-App unter „Dokumente“, die OGS-Leitung in moto unter „Dateien“.${canManage ? "" : " Ändern kann, wer die Rolle „Dokumente“ hat."}`
          }
        >
          {document.releases.length === 0 ? (
            <EmptyState
              title="Noch nicht freigegeben."
              description={
                canManage
                  ? "Noch sieht niemand dieses Dokument. Geben Sie es oben frei: für Eltern oder für Schulen."
                  : "Noch sieht niemand dieses Dokument. Freigeben kann, wer die Rolle „Dokumente“ hat."
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
                        tooltip="moto fragt vorher nach, dann sehen diese Personen das Dokument nicht mehr."
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
