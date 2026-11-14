"use client";

// Dateiablage (#2596): folders on the left, the selected folder's files on
// the right. Who sees which folder is decided in the backend (visibility per
// folder); who may upload or delete arrives as can_upload / can_delete and is
// rendered as-is. Follows the calm Anmeldungen/Planung surface language: one
// content section, gray-50 stat blocks, no colored chips.

import { SectionLabel } from "~/components/ui/section-label";
import { FileText, FolderOpen, Lock, Users } from "lucide-react";
import Link from "~/components/ui/navigation-link";
import { useSession } from "next-auth/react";
import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { CustomSelect } from "~/components/ui/custom-select";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { Dropzone } from "~/components/ui/dropzone";
import { EmptyState } from "~/components/ui/empty-state";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { SectionCard } from "~/components/ui/section-card";
import { TenantPage } from "~/components/ui/tenant-page";
import { FileTypeIcon } from "~/components/ui/file-type-icon";
import { DataField } from "~/components/ui/data-field";
import {
  OverflowMenu,
  type OverflowMenuEntry,
} from "~/components/ui/page-header/OverflowMenu";
import { formatDate } from "~/lib/date-helpers";
import { hasPermission } from "~/lib/auth-utils";
import {
  filesService,
  FOLDER_VISIBILITY_LABELS,
  type FileFolder,
  type FolderFiles,
  type FolderOverview,
  type StoredFile,
} from "~/lib/files-api";
import { formatBytes, isViewableInBrowser } from "~/lib/file-format";
import { useApiErrorDisplay, useApiFormError } from "~/contexts/ToastContext";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { createLogger } from "~/lib/logger";
import { useSWRAuth } from "~/lib/swr";
import { cn } from "~/lib/utils";
import { FolderModal } from "./folder-modal";
import {
  TraegerDocumentsCard,
  useSchoolTraegerDocuments,
} from "./traeger-documents-card";

const logger = createLogger({ component: "FilesPage" });

/** Statuszeile unter dem Titel: gezählte Dateien und Ordner der Ablage. */
function filesStatusLine(folders: readonly FileFolder[]): string {
  const fileCount = folders.reduce((sum, folder) => sum + folder.fileCount, 0);
  return `${fileCount} ${fileCount === 1 ? "Datei" : "Dateien"} · ${
    folders.length
  } Ordner`;
}

const ACCEPTED_FILE_TYPES = ".pdf,.docx,.xlsx,.pptx,.png,.jpg,.jpeg";
const MAX_FILE_SIZE_BYTES = 25 * 1024 * 1024;

function visibilityIcon(visibility: FileFolder["visibility"]) {
  const className = "h-3.5 w-3.5 text-gray-400";
  if (visibility === "admins") {
    return <Lock className={className} aria-hidden="true" />;
  }
  if (visibility === "selected") {
    return <Users className={className} aria-hidden="true" />;
  }
  return null;
}

export function FilesPage() {
  const { data: session } = useSession();
  const {
    data: overview,
    isLoading,
    error,
    mutate: mutateFolders,
  } = useSWRAuth<FolderOverview>("files-folders", () =>
    filesService.listFolders(),
  );

  const loadError = useSwrLoadError(
    error,
    "die Dateiablage",
    () => void mutateFolders(),
  );
  const folders = overview?.folders ?? [];
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const selected =
    folders.find((folder) => folder.id === selectedId) ?? folders[0] ?? null;

  // Keep the selection pointing at an existing folder after deletes.
  useEffect(() => {
    if (selected && selected.id !== selectedId) {
      setSelectedId(selected.id);
    }
  }, [selected, selectedId]);

  const [folderModal, setFolderModal] = useState<{
    open: boolean;
    folder: FileFolder | null;
  }>({ open: false, folder: null });
  const [deleteFolderTarget, setDeleteFolderTarget] =
    useState<FileFolder | null>(null);
  const [deletingFolder, setDeletingFolder] = useState(false);
  // Fehler beim Löschen bleiben im offenen Bestätigungsdialog.
  const deleteFolderErrors = useApiFormError();
  const latestDeleteFolderRef = useRef<() => void>(() => undefined);

  const handleDeleteFolder = async () => {
    if (!deleteFolderTarget) return;
    setDeletingFolder(true);
    deleteFolderErrors.clear();
    try {
      await filesService.deleteFolder(deleteFolderTarget.id);
      setDeleteFolderTarget(null);
      await mutateFolders();
    } catch (err) {
      logger.error("folder_delete_failed", {
        folder_id: deleteFolderTarget.id,
        error: err instanceof Error ? err.message : String(err),
      });
      await deleteFolderErrors.show(err, {
        object: "das Löschen des Ordners",
        retry: () => latestDeleteFolderRef.current(),
      });
    } finally {
      setDeletingFolder(false);
    }
  };
  // „Wiederholen“ löscht den Ordner, der dann im Dialog steht.
  useLayoutEffect(() => {
    latestDeleteFolderRef.current = () => void handleDeleteFolder();
  });

  const canManage = overview?.canManage ?? false;
  // Dokumente des Trägers (#4692) sieht, wer die Dateien der Schule verwaltet.
  const traegerDocuments = useSchoolTraegerDocuments(
    hasPermission(session, "files:manage"),
  );
  const showsTraegerCard = traegerDocuments.shown;
  const canUpload = overview?.canUpload ?? false;
  const canChangeUploadPermission =
    hasPermission(session, "config:read") &&
    hasPermission(session, "config:update");

  const folderNav = (
    <nav aria-label="Ordner" className="space-y-1">
      {folders.map((folder) => {
        const active = folder.id === selected?.id;
        return (
          <button
            key={folder.id}
            type="button"
            onClick={() => setSelectedId(folder.id)}
            aria-current={active ? "true" : undefined}
            className={cn(
              "flex w-full items-center gap-2 rounded-lg px-3 py-2 text-left transition-colors",
              active ? "bg-gray-900 text-white" : "hover:bg-gray-50",
            )}
          >
            <FolderOpen
              className={cn(
                "h-4 w-4 shrink-0",
                active ? "text-white" : "text-gray-400",
              )}
              aria-hidden="true"
            />
            <span className="min-w-0 flex-1">
              <span className="block truncate text-sm font-medium">
                {folder.name}
              </span>
              <span
                className={cn(
                  "flex items-center gap-1 text-xs",
                  active ? "text-gray-300" : "text-gray-500",
                )}
              >
                {!active && visibilityIcon(folder.visibility)}
                {FOLDER_VISIBILITY_LABELS[folder.visibility]}
              </span>
            </span>
            <span
              className={cn(
                "shrink-0 text-xs tabular-nums",
                active ? "text-gray-300" : "text-gray-500",
              )}
            >
              {folder.fileCount}
            </span>
          </button>
        );
      })}
    </nav>
  );

  return (
    <TenantPage
      title="Dateien"
      // Ohne geladene Ablage keine "0 Ordner" neben dem Ladefehler (#2517).
      stats={overview === undefined ? null : filesStatusLine(folders)}
      statsLoading={isLoading}
      // Bis der Katalogtext des Ladefehlers da ist, bleibt das Skelett
      // stehen, nie der Leerzustand.
      loading={isLoading || (!!error && !loadError)}
      error={loadError}
      empty={
        !isLoading && !error && folders.length === 0 && !showsTraegerCard
          ? {
              icon: <FolderOpen className="h-12 w-12" aria-hidden="true" />,
              title: "Noch keine Ordner",
              description: canManage
                ? "Legen Sie den ersten Ordner an und wählen Sie, wer ihn sehen darf."
                : "Sobald die Leitung einen Ordner für Sie freigibt, erscheint er hier.",
            }
          : null
      }
      actions={
        canManage ? (
          <Button
            type="button"
            variant="primary"
            size="md"
            onClick={() => setFolderModal({ open: true, folder: null })}
            tooltip="Öffnet ein Formular für einen neuen Ordner."
          >
            Neuer Ordner
          </Button>
        ) : undefined
      }
      overlays={
        <>
          <FolderModal
            isOpen={folderModal.open}
            initial={folderModal.folder}
            onClose={() => setFolderModal({ open: false, folder: null })}
            onSaved={() => void mutateFolders()}
          />

          <ConfirmDeleteModal
            isOpen={deleteFolderTarget !== null}
            title="Ordner löschen"
            description={
              <>
                Der Ordner{" "}
                <span className="font-medium">{deleteFolderTarget?.name}</span>{" "}
                und alle {deleteFolderTarget?.fileCount ?? 0} Dateien darin
                werden endgültig gelöscht.
              </>
            }
            gate={{ mode: "twoStep" }}
            onConfirm={handleDeleteFolder}
            onClose={() => {
              if (deletingFolder) return;
              deleteFolderErrors.clear();
              setDeleteFolderTarget(null);
            }}
            loading={deletingFolder}
            error={deleteFolderErrors.error}
          />
        </>
      }
    >
      <div className="flex min-h-[28rem] flex-col gap-4">
        <TraegerDocumentsCard {...traegerDocuments} />
        {folders.length > 0 && (
          <div className="grid min-h-0 flex-1 gap-4 lg:grid-cols-[280px_minmax(0,1fr)]">
            {/* Folder list: a panel on desktop, a select on small screens */}
            <SectionCard className="lg:hidden">
              <CustomSelect
                id="dateien-ordner"
                label="Ordner"
                value={selected?.id ?? ""}
                options={folders.map((folder) => ({
                  value: folder.id,
                  label: `${folder.name} (${folder.fileCount})`,
                }))}
                onChange={(value) => setSelectedId(value)}
                controlSize="dense"
              />
            </SectionCard>
            <SectionCard
              className="hidden h-full min-h-0 flex-col lg:flex"
              bodyClassName="flex min-h-0 flex-1 flex-col"
            >
              <aside className="flex min-h-0 flex-1 flex-col">
                <SectionLabel as="p" className="px-3 pt-1 pb-2">
                  Ordner
                </SectionLabel>
                <div className="min-h-0 flex-1 overflow-y-auto">
                  {folderNav}
                </div>
                {canManage && overview && overview.maxBytes > 0 && (
                  <dl className="mt-3 space-y-3 border-t border-gray-100 px-3 pt-3">
                    <DataField label="Speicherplatz">
                      <span className="tabular-nums">
                        {formatBytes(overview.usedBytes)} von{" "}
                        {formatBytes(overview.maxBytes)} belegt
                      </span>
                    </DataField>
                    <DataField label="Dateien hochladen">
                      {overview.staffUploadEnabled
                        ? "Leitung und Team"
                        : "Nur Leitung"}
                      {canChangeUploadPermission && (
                        <span className="mt-1 block">
                          <Link
                            href="/settings?tab=operations&highlight=files.staff_upload_enabled"
                            className="inline-flex min-h-6 items-center text-xs font-medium text-gray-500 underline decoration-gray-300 underline-offset-2 transition-colors hover:text-gray-800 hover:decoration-gray-500 focus-visible:rounded-sm focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:ring-offset-1 focus-visible:outline-none"
                          >
                            Berechtigung ändern
                          </Link>
                        </span>
                      )}
                    </DataField>
                  </dl>
                )}
              </aside>
            </SectionCard>

            {selected && (
              <SectionCard className="flex min-h-0 flex-col">
                <FolderFilesPanel
                  key={selected.id}
                  folder={selected}
                  canManage={canManage}
                  canUpload={canUpload}
                  onEdit={() =>
                    setFolderModal({ open: true, folder: selected })
                  }
                  onDelete={() => {
                    deleteFolderErrors.clear();
                    setDeleteFolderTarget(selected);
                  }}
                  onFilesChanged={() => void mutateFolders()}
                />
              </SectionCard>
            )}
          </div>
        )}
      </div>
    </TenantPage>
  );
}

function FolderFilesPanel({
  folder,
  canManage,
  canUpload,
  onEdit,
  onDelete,
  onFilesChanged,
}: {
  readonly folder: FileFolder;
  readonly canManage: boolean;
  readonly canUpload: boolean;
  readonly onEdit: () => void;
  readonly onDelete: () => void;
  readonly onFilesChanged: () => void;
}) {
  const { data, isLoading, error, mutate } = useSWRAuth<FolderFiles>(
    `files-folder-${folder.id}`,
    () => filesService.listFiles(folder.id),
  );
  const files = data?.files ?? [];
  const loadError = useSwrLoadError(
    error,
    "die Liste der Dateien",
    () => void mutate(),
  );

  const [uploading, setUploading] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<StoredFile | null>(null);
  const [deleting, setDeleting] = useState(false);
  // Fehler beim Löschen bleiben im offenen Bestätigungsdialog.
  const deleteErrors = useApiFormError();
  const { show: showError, showMessage } = useApiErrorDisplay();
  const latestDeleteRef = useRef<() => void>(() => undefined);

  const handleFiles = async (picked: File[]) => {
    if (uploading || !canUpload) return;
    const tooLarge = picked.find((file) => file.size > MAX_FILE_SIZE_BYTES);
    if (tooLarge) {
      showMessage(
        `Die Datei „${tooLarge.name}“ ist größer als 25 MB. Bitte wählen Sie eine kleinere Datei.`,
      );
      return;
    }
    setUploading(true);
    try {
      for (const file of picked) {
        await filesService.upload(folder.id, file);
      }
    } catch (err) {
      logger.error("file_upload_failed", {
        folder_id: folder.id,
        error: err instanceof Error ? err.message : String(err),
      });
      // Kein Wiederholen: die Auswahl ist danach weg, und schon hochgeladene
      // Dateien kämen doppelt.
      void showError(err, { object: "das Hochladen der Datei" });
    } finally {
      setUploading(false);
    }
    // Auch nach einem Fehler: Dateien vor dem Fehlschlag sind gespeichert.
    await mutate();
    onFilesChanged();
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;
    setDeleting(true);
    deleteErrors.clear();
    try {
      await filesService.deleteFile(folder.id, deleteTarget.id);
      setDeleteTarget(null);
      await mutate();
      onFilesChanged();
    } catch (err) {
      logger.error("file_delete_failed", {
        folder_id: folder.id,
        file_id: deleteTarget.id,
        error: err instanceof Error ? err.message : String(err),
      });
      await deleteErrors.show(err, {
        object: "das Löschen der Datei",
        retry: () => latestDeleteRef.current(),
      });
    } finally {
      setDeleting(false);
    }
  };
  // „Wiederholen“ löscht die Datei, die dann im Dialog steht.
  useLayoutEffect(() => {
    latestDeleteRef.current = () => void handleDelete();
  });

  const columns: DataTableColumn<StoredFile>[] = [
    {
      key: "filename",
      header: "Datei",
      stacked: "title",
      // Die Spalte nimmt den freien Platz und kürzt einen langen Namen,
      // statt die Tabelle über die Karte hinaus zu verbreitern (#4975).
      // min-w-40 hält auf dem Handy Platz für den Anfang des Namens.
      className: "w-full max-w-0 min-w-40",
      render: (file) => (
        <span className="flex min-w-0 items-center gap-2">
          <FileTypeIcon contentType={file.contentType} />
          {isViewableInBrowser(file.contentType) ? (
            <a
              href={filesService.viewUrl(folder.id, file.id)}
              target="_blank"
              rel="noopener"
              title={`${file.filename} (im Browser öffnen)`}
              className="truncate text-sm font-medium text-gray-900 hover:underline"
            >
              {file.filename}
            </a>
          ) : (
            <a
              href={filesService.downloadUrl(folder.id, file.id)}
              title={`${file.filename} (herunterladen)`}
              className="truncate text-sm font-medium text-gray-900 hover:underline"
            >
              {file.filename}
            </a>
          )}
        </span>
      ),
      sortValue: (file) => file.filename.toLowerCase(),
    },
    {
      key: "size",
      header: "Größe",
      className: "whitespace-nowrap",
      stacked: "hidden",
      render: (file) => (
        <span className="text-xs text-gray-500">
          {formatBytes(file.sizeBytes)}
        </span>
      ),
      sortValue: (file) => file.sizeBytes,
    },
    {
      key: "uploaded",
      header: "Hochgeladen",
      className: "whitespace-nowrap",
      render: (file) => (
        <span className="text-xs text-gray-500">
          {formatDate(file.uploadedAt)}
        </span>
      ),
      // Auf dem Handy steht die Größe mit in dieser Zeile unter dem Namen.
      stackedLabel: false,
      stackedRender: (file) =>
        `${formatBytes(file.sizeBytes)} · ${formatDate(file.uploadedAt)}`,
      sortValue: (file) => file.uploadedAt,
    },
    {
      key: "actions",
      header: "",
      align: "right",
      stacked: "meta",
      render: (file) => {
        const items: OverflowMenuEntry[] = [];
        if (isViewableInBrowser(file.contentType)) {
          items.push({
            label: "Öffnen",
            description: "Zeigt die Datei in einem neuen Tab",
            href: filesService.viewUrl(folder.id, file.id),
            external: true,
            onClick: () => undefined,
          });
        }
        items.push({
          label: "Herunterladen",
          description: "Speichert die Datei auf Ihrem Gerät",
          href: filesService.downloadUrl(folder.id, file.id),
          external: true,
          onClick: () => undefined,
        });
        if (file.canDelete) {
          items.push(
            { kind: "separator" },
            {
              label: "Löschen",
              description: "Löscht die Datei endgültig. moto fragt vorher nach",
              destructive: true,
              onClick: () => {
                deleteErrors.clear();
                setDeleteTarget(file);
              },
            },
          );
        }
        return (
          <OverflowMenu
            items={items}
            ariaLabel={`Aktionen für ${file.filename}`}
          />
        );
      },
    },
  ];

  const folderMenu: OverflowMenuEntry[] = [
    {
      label: "Ordner bearbeiten",
      description: "Ändert Name und wer den Ordner sieht",
      onClick: onEdit,
    },
    { kind: "separator" },
    {
      label: "Ordner löschen",
      description:
        "Löscht den Ordner mit allen Dateien. moto fragt vorher nach",
      destructive: true,
      onClick: onDelete,
    },
  ];

  return (
    <div className="min-w-0 space-y-4">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <h3 className="truncate text-base font-semibold text-gray-900">
            {folder.name}
          </h3>
          <p className="flex items-center gap-1 text-xs text-gray-500">
            {visibilityIcon(folder.visibility)}
            Sichtbar für: {FOLDER_VISIBILITY_LABELS[folder.visibility]}
          </p>
        </div>
        {canManage && (
          <OverflowMenu
            items={folderMenu}
            ariaLabel={`Aktionen für Ordner ${folder.name}`}
          />
        )}
      </div>

      <LoadErrorAlert error={loadError} />

      {canUpload ? (
        <Dropzone
          accept={ACCEPTED_FILE_TYPES}
          legend="Bereich zum Hochladen von Dateien"
          label="Dateien hierher ziehen"
          dragLabel="Dateien hier ablegen"
          buttonLabel="Dateien auswählen"
          hint="PDF, Word, Excel, PowerPoint, PNG oder JPG · höchstens 25 MB"
          loading={uploading}
          loadingLabel="Wird hochgeladen…"
          multiple
          onFiles={(files) => void handleFiles(files)}
        />
      ) : (
        <Alert
          type="info"
          message="Nur zum Ansehen und Herunterladen. Dateien lädt die Leitung hoch."
        />
      )}

      {/* Ohne geladene Liste kein „Noch keine Dateien“: dort steht der
          Ladefehler. */}
      {error && !data ? null : (
        <DataTable
          // Auf dem Handy eine Zeile je Datei, das Menü rechts daneben: in der
          // Tabelle lag es außerhalb des Bildes (#4975).
          stackedOnMobile
          stackedLayout="row"
          stackedDetailKey="uploaded"
          columns={columns}
          rows={files}
          getRowKey={(file) => file.id}
          isLoading={isLoading}
          defaultSortKey="uploaded"
          defaultSortDirection="desc"
          emptyState={
            <EmptyState
              icon={<FileText className="h-12 w-12" aria-hidden="true" />}
              title="Noch keine Dateien in diesem Ordner"
              description={
                canUpload
                  ? "Laden Sie oben die erste Datei hoch."
                  : "Sobald die Leitung Dateien ablegt, erscheinen sie hier."
              }
            />
          }
        />
      )}

      <ConfirmDeleteModal
        isOpen={deleteTarget !== null}
        title="Datei löschen"
        description={
          <>
            <span className="font-medium">{deleteTarget?.filename}</span> wird
            endgültig gelöscht.
          </>
        }
        gate={{ mode: "twoStep" }}
        onConfirm={handleDelete}
        onClose={() => {
          if (deleting) return;
          deleteErrors.clear();
          setDeleteTarget(null);
        }}
        loading={deleting}
        error={deleteErrors.error}
      />
    </div>
  );
}
