"use client";

// Dateiablage (#2596): folders on the left, the selected folder's files on
// the right. Who sees which folder is decided in the backend (visibility per
// folder); who may upload or delete arrives as can_upload / can_delete and is
// rendered as-is. Follows the calm Anmeldungen/Planung surface language: one
// content section, gray-50 stat blocks, no colored chips.

import {
  FileImage,
  FileSpreadsheet,
  FileText,
  File as FileIcon,
  FolderOpen,
  Lock,
  Presentation,
  Upload,
  Users,
} from "lucide-react";
import { useSession } from "next-auth/react";
import { useEffect, useRef, useState } from "react";
import { Alert } from "~/components/ui/alert";
import { Button, ButtonLink } from "~/components/ui/button";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { CustomSelect } from "~/components/ui/custom-select";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import { InfoItem } from "~/components/ui/info-card";
import {
  OverflowMenu,
  type OverflowMenuEntry,
} from "~/components/ui/page-header/OverflowMenu";
import { Skeleton } from "~/components/ui/skeleton";
import { formatDate } from "~/lib/date-helpers";
import { hasPermission } from "~/lib/auth-utils";
import { GROUP_ROOM_SHADES, LOCATION_COLORS } from "~/lib/location-helper";
import {
  filesService,
  FOLDER_VISIBILITY_LABELS,
  formatBytes,
  isViewableInBrowser,
  type FileFolder,
  type FolderFiles,
  type FolderOverview,
  type StoredFile,
} from "~/lib/files-api";
import { createLogger } from "~/lib/logger";
import { useSWRAuth } from "~/lib/swr";
import { cn } from "~/lib/utils";
import { FolderModal } from "./folder-modal";

const logger = createLogger({ component: "FilesPage" });

const ACCEPTED_FILE_TYPES = ".pdf,.docx,.xlsx,.pptx,.png,.jpg,.jpeg";
const MAX_FILE_SIZE_BYTES = 25 * 1024 * 1024;

function fileIcon(contentType: string) {
  const className = "h-4 w-4 text-gray-400";
  if (contentType.startsWith("image/")) {
    return <FileImage className={className} aria-hidden="true" />;
  }
  if (contentType === "application/pdf") {
    return <FileText className={className} aria-hidden="true" />;
  }
  if (contentType.includes("spreadsheetml")) {
    return <FileSpreadsheet className={className} aria-hidden="true" />;
  }
  if (contentType.includes("presentationml")) {
    return <Presentation className={className} aria-hidden="true" />;
  }
  return <FileIcon className={className} aria-hidden="true" />;
}

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
  const [deleteFolderError, setDeleteFolderError] = useState("");

  const handleDeleteFolder = async () => {
    if (!deleteFolderTarget) return;
    setDeletingFolder(true);
    setDeleteFolderError("");
    try {
      await filesService.deleteFolder(deleteFolderTarget.id);
      setDeleteFolderTarget(null);
      await mutateFolders();
    } catch (err) {
      logger.error("folder_delete_failed", {
        folder_id: deleteFolderTarget.id,
        error: err instanceof Error ? err.message : String(err),
      });
      setDeleteFolderError(
        err instanceof Error
          ? err.message
          : "Ordner konnte nicht gelöscht werden.",
      );
    } finally {
      setDeletingFolder(false);
    }
  };

  if (error) {
    return (
      <Alert
        type="error"
        message="Die Dateiablage konnte nicht geladen werden."
      />
    );
  }

  const canManage = overview?.canManage ?? false;
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
                  "flex items-center gap-1 text-[11px]",
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
    <div className="flex min-h-[calc(100vh-7rem)] flex-col gap-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <p
            className="text-xs font-semibold tracking-wide uppercase"
            style={{ color: LOCATION_COLORS.OTHER_ROOM }}
          >
            Dateiablage
          </p>
          <h1 className="text-base font-semibold text-gray-900">Dateien</h1>
          <p className="max-w-3xl text-sm leading-6 text-gray-600">
            Hier liegen gemeinsame Dateien der OGS, zum Beispiel Formulare und
            Notfallpläne. Die Leitung entscheidet für jeden Ordner, wer ihn
            sehen darf. Unterlagen zu Kindern und Mitarbeitenden bleiben bei der
            jeweiligen Person.
          </p>
        </div>
        {canManage && (
          <Button
            type="button"
            variant="primary"
            size="md"
            onClick={() => setFolderModal({ open: true, folder: null })}
          >
            Neuer Ordner
          </Button>
        )}
      </div>

      {isLoading ? (
        <div className="moto-content-surface flex-1 rounded-2xl border p-5 shadow-sm">
          <div className="space-y-3">
            <Skeleton className="h-10 w-full" />
            <Skeleton className="h-10 w-full" />
            <Skeleton className="h-10 w-2/3" />
          </div>
        </div>
      ) : folders.length === 0 ? (
        <div className="moto-content-surface flex flex-1 items-center justify-center rounded-2xl border p-5 shadow-sm">
          <EmptyState
            icon={<FolderOpen className="h-12 w-12" aria-hidden="true" />}
            title="Noch keine Ordner"
            description={
              canManage
                ? "Legen Sie den ersten Ordner an und wählen Sie, wer ihn sehen darf."
                : "Sobald die Leitung einen Ordner für Sie freigibt, erscheint er hier."
            }
          />
        </div>
      ) : (
        <div className="grid min-h-0 flex-1 gap-4 lg:grid-cols-[280px_minmax(0,1fr)]">
          {/* Folder list: a panel on desktop, a select on small screens */}
          <div className="moto-content-surface rounded-2xl border p-3 shadow-sm lg:hidden">
            <label
              htmlFor="dateien-ordner"
              className="mb-1.5 block text-xs font-medium text-gray-600"
            >
              Ordner
            </label>
            <CustomSelect
              id="dateien-ordner"
              value={selected?.id ?? ""}
              options={folders.map((folder) => ({
                value: folder.id,
                label: `${folder.name} (${folder.fileCount})`,
              }))}
              onChange={(value) => setSelectedId(value)}
              triggerClassName="moto-content-surface h-9 w-full hover:border-gray-300"
            />
          </div>
          <aside className="moto-content-surface hidden flex-col rounded-2xl border p-3 shadow-sm lg:flex">
            <p className="px-3 pt-1 pb-2 text-[11px] font-semibold tracking-wide text-gray-500 uppercase">
              Ordner
            </p>
            <div className="min-h-0 flex-1 overflow-y-auto">{folderNav}</div>
            {canManage && overview && overview.maxBytes > 0 && (
              <div className="mt-3 space-y-3 border-t border-gray-100 px-3 pt-3">
                <InfoItem
                  label="Speicherplatz"
                  value={
                    <span className="tabular-nums">
                      {formatBytes(overview.usedBytes)} von{" "}
                      {formatBytes(overview.maxBytes)} belegt
                    </span>
                  }
                />
                <InfoItem
                  label="Dateien hochladen"
                  value={
                    overview.staffUploadEnabled
                      ? "Leitung und Team"
                      : "Nur Leitung"
                  }
                />
                {canChangeUploadPermission && (
                  <ButtonLink
                    href="/settings?tab=operations&highlight=files.staff_upload_enabled"
                    variant="surface"
                    size="compact"
                  >
                    Berechtigung ändern
                  </ButtonLink>
                )}
              </div>
            )}
          </aside>

          {selected && (
            <section className="moto-content-surface flex min-h-0 flex-col rounded-2xl border p-5 shadow-sm">
              <FolderFilesPanel
                key={selected.id}
                folder={selected}
                canManage={canManage}
                canUpload={canUpload}
                onEdit={() => setFolderModal({ open: true, folder: selected })}
                onDelete={() => {
                  setDeleteFolderError("");
                  setDeleteFolderTarget(selected);
                }}
                onFilesChanged={() => void mutateFolders()}
              />
            </section>
          )}
        </div>
      )}

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
            <span className="font-medium">{deleteFolderTarget?.name}</span> und
            alle {deleteFolderTarget?.fileCount ?? 0} Dateien darin werden
            endgültig gelöscht.
          </>
        }
        gate={{ mode: "twoStep" }}
        onConfirm={handleDeleteFolder}
        onClose={() => {
          if (!deletingFolder) setDeleteFolderTarget(null);
        }}
        loading={deletingFolder}
        error={deleteFolderError}
      />
    </div>
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

  const [uploading, setUploading] = useState(false);
  const [actionError, setActionError] = useState("");
  const [dragActive, setDragActive] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<StoredFile | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState("");
  const fileInputRef = useRef<HTMLInputElement>(null);

  const handleFiles = async (list: FileList | File[] | null) => {
    if (!list || uploading || !canUpload) return;
    const picked = Array.from(list);
    if (picked.length === 0) return;
    const tooLarge = picked.find((file) => file.size > MAX_FILE_SIZE_BYTES);
    if (tooLarge) {
      setActionError(`„${tooLarge.name}“ ist größer als 25 MB.`);
      return;
    }
    setActionError("");
    setUploading(true);
    try {
      for (const file of picked) {
        await filesService.upload(folder.id, file);
      }
      await mutate();
      onFilesChanged();
    } catch (err) {
      logger.error("file_upload_failed", {
        folder_id: folder.id,
        error: err instanceof Error ? err.message : String(err),
      });
      setActionError(
        err instanceof Error
          ? err.message
          : "Datei konnte nicht hochgeladen werden.",
      );
      await mutate();
      onFilesChanged();
    } finally {
      setUploading(false);
      if (fileInputRef.current) fileInputRef.current.value = "";
    }
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;
    setDeleting(true);
    setDeleteError("");
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
      setDeleteError(
        err instanceof Error
          ? err.message
          : "Datei konnte nicht gelöscht werden.",
      );
    } finally {
      setDeleting(false);
    }
  };

  const columns: DataTableColumn<StoredFile>[] = [
    {
      key: "filename",
      header: "Datei",
      render: (file) => (
        <span className="flex min-w-0 items-center gap-2">
          {fileIcon(file.contentType)}
          {isViewableInBrowser(file.contentType) ? (
            <a
              href={filesService.viewUrl(folder.id, file.id)}
              target="_blank"
              rel="noopener"
              title="Im Browser öffnen"
              className="truncate text-sm font-medium text-gray-900 hover:underline"
            >
              {file.filename}
            </a>
          ) : (
            <a
              href={filesService.downloadUrl(folder.id, file.id)}
              title="Herunterladen"
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
      render: (file) => (
        <span className="text-xs text-gray-500">
          {formatDate(file.uploadedAt)}
        </span>
      ),
      sortValue: (file) => file.uploadedAt,
    },
    {
      key: "actions",
      header: "",
      align: "right",
      render: (file) => {
        const items: OverflowMenuEntry[] = [];
        if (isViewableInBrowser(file.contentType)) {
          items.push({
            label: "Öffnen",
            href: filesService.viewUrl(folder.id, file.id),
            external: true,
            onClick: () => undefined,
          });
        }
        items.push({
          label: "Herunterladen",
          href: filesService.downloadUrl(folder.id, file.id),
          external: true,
          onClick: () => undefined,
        });
        if (file.canDelete) {
          items.push(
            { kind: "separator" },
            {
              label: "Löschen",
              destructive: true,
              onClick: () => {
                setDeleteError("");
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
    { label: "Ordner bearbeiten", onClick: onEdit },
    { kind: "separator" },
    { label: "Ordner löschen", destructive: true, onClick: onDelete },
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

      {error && (
        <Alert type="error" message="Dateien konnten nicht geladen werden." />
      )}
      {actionError !== "" && <Alert type="error" message={actionError} />}

      {canUpload ? (
        <div
          className={cn(
            "flex flex-col items-center justify-center gap-2 rounded-lg border-2 border-dashed px-4 py-5 text-center transition-colors",
            dragActive ? "" : "border-gray-200 bg-gray-50/50",
          )}
          style={
            dragActive
              ? {
                  borderColor: LOCATION_COLORS.GROUP_ROOM,
                  backgroundColor: GROUP_ROOM_SHADES.bgHover,
                }
              : undefined
          }
          onDragOver={(e) => {
            e.preventDefault();
            setDragActive(true);
          }}
          onDragLeave={() => setDragActive(false)}
          onDrop={(e) => {
            e.preventDefault();
            setDragActive(false);
            void handleFiles(e.dataTransfer.files);
          }}
        >
          <Upload className="h-5 w-5 text-gray-400" aria-hidden="true" />
          <p className="text-sm text-gray-600">
            Dateien hierher ziehen oder{" "}
            <Button
              type="button"
              variant="ghost"
              size="compact"
              style={{ color: LOCATION_COLORS.OTHER_ROOM }}
              disabled={uploading}
              onClick={() => fileInputRef.current?.click()}
            >
              {uploading ? "Wird hochgeladen…" : "Dateien auswählen"}
            </Button>
          </p>
          <p className="text-xs text-gray-400">
            PDF, Word, Excel, PowerPoint, PNG oder JPG · höchstens 25 MB
          </p>
          <input
            ref={fileInputRef}
            type="file"
            multiple
            accept={ACCEPTED_FILE_TYPES}
            className="hidden"
            aria-label="Dateien auswählen"
            onChange={(e) => void handleFiles(e.target.files)}
          />
        </div>
      ) : (
        <p className="rounded-lg bg-gray-50 px-3 py-2 text-xs leading-5 text-gray-600">
          Nur zum Ansehen und Herunterladen. Dateien lädt die Leitung hoch.
        </p>
      )}

      <DataTable
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
          if (!deleting) setDeleteTarget(null);
        }}
        loading={deleting}
        error={deleteError}
      />
    </div>
  );
}
