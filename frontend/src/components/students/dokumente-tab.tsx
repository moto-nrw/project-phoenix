"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { FileText, FileImage, File as FileIcon, Upload } from "lucide-react";
import { AccidentReportCreateModal } from "~/components/students/accident-report-create-modal";
import { Button } from "~/components/ui/button";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { CustomSelect } from "~/components/ui/custom-select";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import NavigationLink from "~/components/ui/navigation-link";
import {
  OverflowMenu,
  type OverflowMenuEntry,
} from "~/components/ui/page-header/OverflowMenu";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import {
  useApiErrorDisplay,
  useApiFormError,
  useApiLoadError,
} from "~/contexts/ToastContext";
import {
  berlinClockFromISO,
  formatBerlinDate,
  formatDate,
} from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import { formatFileSize } from "~/lib/staff-documents-api";
import {
  accidentReportPath,
  studentAccidentReportsService,
} from "~/lib/student-accident-reports-api";
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
// Accident reports (#4771) are recorded with „Unfall erfassen“ and carry
// their paper copy on their own page, so the plain upload leaves them out.
const ACCIDENT_CATEGORY = "unfallanzeige";
const MAX_FILE_SIZE_BYTES = 10 * 1024 * 1024;

function accidentReportTitle(doc: StudentDocument): string {
  const occurredAt = doc.accidentReport?.occurredAt;
  if (!occurredAt) return doc.filename;
  const title = `Unfall am ${formatBerlinDate(occurredAt)}, ${berlinClockFromISO(occurredAt)} Uhr`;
  const version = doc.accidentReport?.version ?? 0;
  return version > 1 ? `${title} · Fassung ${version}` : title;
}

function AccidentReportStatus({ doc }: Readonly<{ doc: StudentDocument }>) {
  const report = doc.accidentReport;
  if (!report) return null;
  if (report.supersededById) {
    return <StatusBadge tone="gray" label="Korrigiert" />;
  }
  return report.status === "signed" ? (
    <StatusBadge tone="green" label="Unterschrieben" />
  ) : (
    <StatusBadge tone="orange" label="Offen" />
  );
}

function documentIcon(contentType: string) {
  if (contentType.startsWith("image/")) {
    return (
      <FileImage
        className="h-4 w-4 shrink-0 text-gray-400"
        aria-hidden="true"
      />
    );
  }
  if (contentType === "application/pdf") {
    return (
      <FileText className="h-4 w-4 shrink-0 text-gray-400" aria-hidden="true" />
    );
  }
  return (
    <FileIcon className="h-4 w-4 shrink-0 text-gray-400" aria-hidden="true" />
  );
}

export function StudentDokumenteTab({
  studentId,
  canRecordAccidents = false,
}: {
  readonly studentId: string;
  /** student_accidents:manage: shows „Unfall erfassen“ (#4771). */
  readonly canRecordAccidents?: boolean;
}) {
  const [accidentOpen, setAccidentOpen] = useState(false);
  const { data, isLoading, error, mutate } = useSWRAuth<StudentDocumentList>(
    `student-documents-${studentId}`,
    () => studentDocumentsService.list(studentId),
  );

  const [filter, setFilter] = useState<string>("alle");
  const [uploadCategory, setUploadCategory] = useState<string | null>(null);
  const [uploading, setUploading] = useState(false);
  const [dragActive, setDragActive] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<StudentDocument | null>(
    null,
  );
  const [deleting, setDeleting] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const uploadErrors = useApiErrorDisplay();
  const deleteErrors = useApiFormError();
  const load = useApiLoadError();
  const showLoadError = load.show;
  const clearLoadError = load.clear;
  useEffect(() => {
    if (error) {
      void showLoadError(error, {
        object: "die Liste der Dokumente",
        retry: () => void mutate(),
      });
    } else {
      clearLoadError();
    }
  }, [error, mutate, showLoadError, clearLoadError]);

  const visibleCategories = useMemo(
    () => data?.visibleCategories ?? [],
    [data],
  );
  const uploadCategories = useMemo(
    () =>
      visibleCategories.filter(
        (category) => category.value !== ACCIDENT_CATEGORY,
      ),
    [visibleCategories],
  );
  const canUploadDocuments = uploadCategories.length > 0;
  const documents = data?.documents ?? [];
  const effectiveUploadCategory =
    uploadCategory ?? uploadCategories[0]?.value ?? null;

  const filterItems: readonly { value: string; label: string }[] = [
    { value: "alle", label: "Alle Kategorien" },
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
      uploadErrors.showMessage(
        "Die Datei ist größer als 10 MB. Bitte wählen Sie eine kleinere Datei.",
      );
      return;
    }
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
      // Kein Wiederholen: die Datei muss neu gewählt werden.
      await uploadErrors.show(err, { object: "das Dokument" });
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
    deleteErrors.clear();
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
      await deleteErrors.show(err, {
        object: "das Dokument",
        retry: () => void handleDelete(),
      });
    } finally {
      setDeleting(false);
    }
  };

  const columns: DataTableColumn<StudentDocument>[] = [
    {
      key: "filename",
      header: "Dokument",
      // Die Spalte nimmt den freien Platz und kürzt einen langen Namen,
      // statt die Tabelle über die Karte hinaus zu verbreitern (#4975).
      // min-w-40 hält auf dem Handy Platz für den Anfang des Namens.
      className: "w-full max-w-0 min-w-40",
      render: (doc) =>
        doc.accidentReport ? (
          <span className="flex min-w-0 flex-wrap items-center gap-2">
            {documentIcon(doc.contentType)}
            <NavigationLink
              href={accidentReportPath(studentId, doc.id)}
              className="min-w-0 text-sm font-medium break-words text-gray-900 underline-offset-2 hover:underline"
            >
              {accidentReportTitle(doc)}
            </NavigationLink>
            <AccidentReportStatus doc={doc} />
          </span>
        ) : (
          <span className="flex min-w-0 items-center gap-2">
            {documentIcon(doc.contentType)}
            <span
              title={doc.filename}
              className="truncate text-sm font-medium text-gray-900"
            >
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
      className: "whitespace-nowrap",
      render: (doc) => (
        <span className="text-xs text-gray-500">
          {/* A report without paper copy has no file, only its PDF. */}
          {doc.hasFile ? formatFileSize(doc.sizeBytes) : "–"}
        </span>
      ),
      sortValue: (doc) => doc.sizeBytes,
    },
    {
      key: "uploaded",
      header: "Hochgeladen",
      className: "whitespace-nowrap",
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
        const items = documentMenu(doc);
        return (
          <OverflowMenu
            items={items}
            ariaLabel={`Aktionen für ${doc.filename}`}
          />
        );
      },
    },
  ];

  function documentMenu(doc: StudentDocument): readonly OverflowMenuEntry[] {
    const remove: readonly OverflowMenuEntry[] = [
      { kind: "separator" },
      {
        label: "Löschen",
        destructive: true,
        onClick: () => {
          deleteErrors.clear();
          setDeleteTarget(doc);
        },
      },
    ];
    if (!doc.accidentReport) {
      return [
        {
          label: "Herunterladen",
          href: studentDocumentsService.downloadUrl(studentId, doc.id),
          external: true,
          onClick: () => undefined,
        },
        ...remove,
      ];
    }
    // A signed report stays as a record; only an open one can be deleted.
    return [
      {
        label: "Öffnen",
        href: accidentReportPath(studentId, doc.id),
        onClick: () => undefined,
      },
      {
        label: "PDF herunterladen",
        href: studentAccidentReportsService.pdfUrl(studentId, doc.id),
        external: true,
        onClick: () => undefined,
      },
      ...(doc.accidentReport.status === "open" && canRecordAccidents
        ? remove
        : []),
    ];
  }

  if (error) {
    return <LoadErrorAlert error={load.error} />;
  }

  return (
    <div className="space-y-6">
      <SectionCard
        title="Dokumente"
        description="Dateien zum Kind. Uploads und Löschungen werden im Änderungsprotokoll festgehalten. Beim Löschen wird die Datei sofort entfernt, der Protokolleintrag bleibt."
        action={
          canRecordAccidents ? (
            <Button
              type="button"
              variant="outline"
              size="md"
              onClick={() => setAccidentOpen(true)}
            >
              Unfall erfassen
            </Button>
          ) : undefined
        }
      >
        {canUploadDocuments && (
          /* Upload row: category choice + drop zone */
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
                options={uploadCategories.map((category) => ({
                  value: category.value,
                  label: category.label,
                }))}
                onChange={(value) => setUploadCategory(value)}
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
                  ? "border-moto-green bg-moto-green/5"
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
                  className="text-moto-blue"
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
        )}

        {/* Category filter */}
        {documents.length > 0 && filterItems.length > 2 && (
          // Eine Auswahlliste statt Pillen: alle Kategorien nebeneinander
          // passten auf Handy und Tablet nicht in die Karte, die Pillen
          // überlagerten sich und die letzten waren nicht erreichbar (#4975).
          <div className="mb-4 flex flex-wrap items-center gap-3">
            <label
              htmlFor="kind-dokument-filter"
              className="text-xs font-medium text-gray-600"
            >
              Anzeigen
            </label>
            <CustomSelect
              id="kind-dokument-filter"
              value={filter}
              options={filterItems}
              onChange={setFilter}
              triggerClassName="moto-content-surface h-9 w-56 hover:border-gray-300"
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
                  ? canUploadDocuments
                    ? "Noch keine Dokumente"
                    : "Noch keine Unfallanzeigen"
                  : "Keine Dokumente in dieser Kategorie"
              }
              description={
                canUploadDocuments
                  ? "Lade die erste Datei über den Bereich oben hoch."
                  : undefined
              }
            />
          }
        />
      </SectionCard>

      {canRecordAccidents && (
        <AccidentReportCreateModal
          isOpen={accidentOpen}
          onClose={() => setAccidentOpen(false)}
          studentId={studentId}
          onCreated={() => {
            setAccidentOpen(false);
            void mutate();
          }}
          openCreated
        />
      )}

      <ConfirmDeleteModal
        isOpen={deleteTarget !== null}
        title="Dokument löschen"
        description={
          <>
            <span className="font-medium">
              {deleteTarget ? accidentReportTitle(deleteTarget) : ""}
            </span>{" "}
            wird endgültig gelöscht. Der Vorgang wird im Änderungsprotokoll
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
        error={deleteErrors.error}
      />
    </div>
  );
}
