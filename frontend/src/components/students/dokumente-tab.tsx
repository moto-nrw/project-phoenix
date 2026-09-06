"use client";

import { useMemo, useRef, useState } from "react";
import { FileText, FileImage, File as FileIcon, Upload } from "lucide-react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { CustomSelect } from "~/components/ui/custom-select";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import {
  OverflowMenu,
  type OverflowMenuEntry,
} from "~/components/ui/page-header/OverflowMenu";
import { SectionCard } from "~/components/ui/section-card";
import {
  SegmentedControl,
  type SegmentedControlItem,
} from "~/components/ui/segmented-control";
import { StatusBadge } from "~/components/ui/status-badge";
import { formatDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import { formatFileSize } from "~/lib/staff-documents-api";
import {
  studentDocumentsService,
  type StudentDocument,
  type StudentDocumentList,
} from "~/lib/student-documents-api";
import { useSWRAuth } from "~/lib/swr";

// Dokumente tab (#777): flat file list per child with upload, download,
// audited delete and category filter. The backend already filters per
// category permission (Attest/Impfnachweis/Medikamentenplan →
// student_documents:health, Sorgerecht → student_documents:legal, rest →
// users:update) and sends the caller's visible categories — the tab renders
// exactly that, no client-side authority.

const logger = createLogger({ component: "StudentDokumenteTab" });

const ACCEPTED_FILE_TYPES = ".pdf,.docx,.png,.jpg,.jpeg";
const MAX_FILE_SIZE_BYTES = 10 * 1024 * 1024;

function documentIcon(contentType: string) {
  if (contentType.startsWith("image/")) {
    return <FileImage className="h-4 w-4 text-gray-400" aria-hidden="true" />;
  }
  if (contentType === "application/pdf") {
    return <FileText className="h-4 w-4 text-gray-400" aria-hidden="true" />;
  }
  return <FileIcon className="h-4 w-4 text-gray-400" aria-hidden="true" />;
}

export function StudentDokumenteTab({
  studentId,
}: {
  readonly studentId: string;
}) {
  const { data, isLoading, error, mutate } = useSWRAuth<StudentDocumentList>(
    `student-documents-${studentId}`,
    () => studentDocumentsService.list(studentId),
  );

  const [filter, setFilter] = useState<string>("alle");
  const [uploadCategory, setUploadCategory] = useState<string | null>(null);
  const [uploading, setUploading] = useState(false);
  const [actionError, setActionError] = useState("");
  const [dragActive, setDragActive] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<StudentDocument | null>(
    null,
  );
  const [deleting, setDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState("");
  const fileInputRef = useRef<HTMLInputElement>(null);

  const visibleCategories = useMemo(
    () => data?.visibleCategories ?? [],
    [data],
  );
  const documents = data?.documents ?? [];
  const effectiveUploadCategory =
    uploadCategory ?? visibleCategories[0]?.value ?? null;

  const filterItems: readonly SegmentedControlItem<string>[] = [
    { value: "alle", label: "Alle" },
    ...visibleCategories.map((category) => ({
      value: category.value,
      label: category.label,
    })),
  ];

  const filtered =
    filter === "alle"
      ? documents
      : documents.filter((doc) => doc.category === filter);

  const handleFiles = async (files: FileList | File[] | null) => {
    const file = files?.[0];
    if (!file || uploading) {
      return;
    }
    if (!effectiveUploadCategory) {
      return;
    }
    if (file.size > MAX_FILE_SIZE_BYTES) {
      setActionError("Die Datei ist größer als 10 MB.");
      return;
    }
    setActionError("");
    setUploading(true);
    try {
      await studentDocumentsService.upload(
        studentId,
        file,
        effectiveUploadCategory,
      );
      await mutate();
    } catch (err) {
      logger.error("student_document_upload_failed", {
        student_id: studentId,
        error: err instanceof Error ? err.message : String(err),
      });
      setActionError(
        err instanceof Error
          ? err.message
          : "Dokument konnte nicht hochgeladen werden.",
      );
    } finally {
      setUploading(false);
      if (fileInputRef.current) {
        fileInputRef.current.value = "";
      }
    }
  };

  const handleDelete = async () => {
    if (!deleteTarget) {
      return;
    }
    setDeleting(true);
    setDeleteError("");
    try {
      await studentDocumentsService.delete(studentId, deleteTarget.id);
      setDeleteTarget(null);
      await mutate();
    } catch (err) {
      logger.error("student_document_delete_failed", {
        student_id: studentId,
        document_id: deleteTarget.id,
        error: err instanceof Error ? err.message : String(err),
      });
      setDeleteError(
        err instanceof Error
          ? err.message
          : "Dokument konnte nicht gelöscht werden.",
      );
    } finally {
      setDeleting(false);
    }
  };

  const columns: DataTableColumn<StudentDocument>[] = [
    {
      key: "filename",
      header: "Dokument",
      render: (doc) => (
        <span className="flex min-w-0 items-center gap-2">
          {documentIcon(doc.contentType)}
          <span className="truncate text-sm font-medium text-gray-900">
            {doc.filename}
          </span>
        </span>
      ),
      sortValue: (doc) => doc.filename.toLowerCase(),
    },
    {
      key: "category",
      header: "Kategorie",
      render: (doc) => (
        <StatusBadge
          tone={doc.sensitive ? "blue" : "gray"}
          label={doc.categoryLabel}
          title={
            doc.sensitive
              ? "Besonders schützenswert — jeder Download wird protokolliert."
              : undefined
          }
        />
      ),
      sortValue: (doc) => doc.categoryLabel,
    },
    {
      key: "size",
      header: "Größe",
      render: (doc) => (
        <span className="text-xs text-gray-500">
          {formatFileSize(doc.sizeBytes)}
        </span>
      ),
      sortValue: (doc) => doc.sizeBytes,
    },
    {
      key: "uploaded",
      header: "Hochgeladen",
      render: (doc) => (
        <span className="text-xs text-gray-500">
          {formatDate(doc.uploadedAt)}
        </span>
      ),
      sortValue: (doc) => doc.uploadedAt,
    },
    {
      key: "actions",
      header: "",
      align: "right",
      render: (doc) => {
        const items: readonly OverflowMenuEntry[] = [
          {
            label: "Herunterladen",
            href: studentDocumentsService.downloadUrl(studentId, doc.id),
            external: true,
            onClick: () => undefined,
          },
          { kind: "separator" },
          {
            label: "Löschen",
            destructive: true,
            onClick: () => {
              setDeleteError("");
              setDeleteTarget(doc);
            },
          },
        ];
        return (
          <OverflowMenu
            items={items}
            ariaLabel={`Aktionen für ${doc.filename}`}
          />
        );
      },
    },
  ];

  if (error) {
    return (
      <Alert type="error" message="Dokumente konnten nicht geladen werden." />
    );
  }

  return (
    <div className="space-y-6">
      <SectionCard
        title="Dokumente"
        description="Dateien zum Kind. Uploads und Löschungen werden im Änderungsprotokoll festgehalten. Beim Löschen wird die Datei sofort entfernt, der Protokolleintrag bleibt."
      >
        {actionError !== "" && (
          <div className="mb-4">
            <Alert type="error" message={actionError} />
          </div>
        )}

        {/* Upload row: category choice + drop zone */}
        <div className="mb-5 space-y-3">
          <div className="flex flex-wrap items-center gap-3">
            <label
              htmlFor="kind-dokument-kategorie"
              className="text-xs font-medium text-gray-600"
            >
              Kategorie
            </label>
            <CustomSelect
              id="kind-dokument-kategorie"
              value={effectiveUploadCategory ?? ""}
              options={visibleCategories.map((category) => ({
                value: category.value,
                label: category.label,
              }))}
              onChange={(value) => setUploadCategory(value)}
              disabled={visibleCategories.length === 0}
              // triggerClassName REPLACES the default trigger classes, and the
              // base class carries a colorless `border` — without
              // moto-content-surface the Tailwind-4 default (currentColor)
              // paints it black.
              triggerClassName="moto-content-surface h-9 w-56 hover:border-gray-300"
            />
          </div>

          <div
            className={`flex flex-col items-center justify-center gap-2 rounded-lg border-2 border-dashed px-4 py-6 text-center transition-colors ${
              dragActive
                ? "border-[#83CD2D] bg-[#83CD2D]/5"
                : "border-gray-200 bg-gray-50/50"
            }`}
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
              Datei hierher ziehen oder{" "}
              <Button
                type="button"
                variant="ghost"
                size="compact"
                className="text-[#5080D8]"
                disabled={uploading || !effectiveUploadCategory}
                onClick={() => fileInputRef.current?.click()}
              >
                {uploading ? "Wird hochgeladen…" : "Datei auswählen"}
              </Button>
            </p>
            <p className="text-xs text-gray-400">
              PDF, DOCX, PNG oder JPG · max. 10 MB
            </p>
            <input
              ref={fileInputRef}
              type="file"
              accept={ACCEPTED_FILE_TYPES}
              className="hidden"
              aria-label="Dokument auswählen"
              onChange={(e) => void handleFiles(e.target.files)}
            />
          </div>
        </div>

        {/* Category filter */}
        {documents.length > 0 && filterItems.length > 2 && (
          <div className="mb-4">
            <SegmentedControl
              variant="pills"
              items={filterItems}
              value={filter}
              onChange={setFilter}
              ariaLabel="Nach Kategorie filtern"
            />
          </div>
        )}

        <DataTable
          columns={columns}
          rows={filtered}
          getRowKey={(doc) => doc.id}
          isLoading={isLoading}
          defaultSortKey="uploaded"
          defaultSortDirection="desc"
          paginationResetKey={filter}
          emptyState={
            <EmptyState
              icon={<FileText className="h-12 w-12" aria-hidden="true" />}
              title={
                filter === "alle"
                  ? "Noch keine Dokumente"
                  : "Keine Dokumente in dieser Kategorie"
              }
              description="Lade die erste Datei über den Bereich oben hoch."
            />
          }
        />
      </SectionCard>

      <ConfirmDeleteModal
        isOpen={deleteTarget !== null}
        title="Dokument löschen"
        description={
          <>
            <span className="font-medium">{deleteTarget?.filename}</span> wird
            endgültig gelöscht. Der Vorgang wird im Änderungsprotokoll
            festgehalten.
          </>
        }
        gate={{ mode: "twoStep" }}
        onConfirm={handleDelete}
        onClose={() => {
          if (!deleting) {
            setDeleteTarget(null);
          }
        }}
        loading={deleting}
        error={deleteError}
      />
    </div>
  );
}
