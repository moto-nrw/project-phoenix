"use client";

// Sammelimport für Klassenlisteneinträge (#2382): CSV/Excel mit genau drei
// Spalten (Vorname, Nachname, Klasse). Bereits vorhandene Einträge und
// bereits regulär angelegte Kinder werden übersprungen, sie stehen schon
// auf der Klassenliste.

import { useState, useCallback } from "react";
import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import { Download, Info, ListChecks, X } from "lucide-react";
import { SkeletonRegion, FormSkeleton } from "~/components/ui/page-skeletons";
import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import { Alert } from "~/components/ui/alert";
import { BackButton } from "~/components/ui/back-button";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { SectionCard } from "~/components/ui/section-card";
import { UploadSection } from "~/components/import/upload-section";
import { StatsCards } from "~/components/import/stats-cards";
import { StudentRowCard } from "~/components/import/student-row-card";
import { useToast } from "~/contexts/ToastContext";
import { useRequirePermission } from "~/lib/hooks/use-require-permission";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "ClassListImportPage" });

interface ImportError {
  field: string;
  message: string;
  code: string;
  severity: "error" | "warning" | "info";
}

interface ImportRowResult {
  RowNumber: number;
  Data: {
    first_name: string;
    last_name: string;
    school_class: string;
  };
  Errors: ImportError[];
  Timestamp: string;
}

interface ImportResult {
  StartedAt: string;
  CompletedAt: string;
  TotalRows: number;
  CreatedCount: number;
  UpdatedCount: number;
  SkippedCount: number;
  ErrorCount: number;
  WarningCount: number;
  Errors: ImportRowResult[];
  BulkActions: string[];
  DryRun: boolean;
}

type RowStatus = "new" | "existing" | "error" | "warning";

interface DisplayEntry {
  row: number;
  status: RowStatus;
  errors: string[];
  first_name: string;
  last_name: string;
  school_class: string;
}

function rowStatusFor(errors: ImportError[]): RowStatus {
  const isExisting = errors.some((e) => e.code === "already_exists");
  if (isExisting) return "existing";
  if (errors.some((e) => e.severity === "error")) return "error";
  if (errors.some((e) => e.severity === "warning")) return "warning";
  return "new";
}

// The generic import engine records rows that are already on the class list
// as already_exists row errors and never fills SkippedCount — the skipped
// share has to be derived from the row results.
function countAlreadyExists(rows: ImportRowResult[]): number {
  return rows.filter((r) => r.Errors.some((e) => e.code === "already_exists"))
    .length;
}

function toDisplayEntry(row: ImportRowResult): DisplayEntry {
  return {
    row: row.RowNumber,
    status: rowStatusFor(row.Errors),
    errors: row.Errors.map((e) => e.message),
    first_name: row.Data.first_name,
    last_name: row.Data.last_name,
    school_class: row.Data.school_class,
  };
}

export default function ClassListImportPage() {
  const [uploadedFile, setUploadedFile] = useState<File | null>(null);
  const [previewData, setPreviewData] = useState<DisplayEntry[]>([]);
  const [isDragging, setIsDragging] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [isImporting, setIsImporting] = useState(false);
  const [importComplete, setImportComplete] = useState(false);
  const [importResult, setImportResult] = useState<ImportResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [templateFormat, setTemplateFormat] = useState<"csv" | "xlsx">("xlsx");

  const { data: session, status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
    },
  });
  // Der Import legt Einträge an, dieselbe Hürde wie das Backend
  // (/api/import/class-list-entries verlangt users:create). Wer sie nicht
  // nimmt, wird zum Dashboard umgeleitet statt in einen 403 zu laufen.
  const { isReady } = useRequirePermission("users:create");

  const toast = useToast();

  const resetForm = useCallback(() => {
    setUploadedFile(null);
    setPreviewData([]);
    setIsDragging(false);
    setIsLoading(false);
    setIsImporting(false);
    setImportComplete(false);
    setImportResult(null);
    setError(null);
  }, []);

  const handleDownloadTemplate = async () => {
    try {
      const token = session?.user?.token;
      if (!token) {
        setError("Keine Authentifizierung");
        return;
      }

      const response = await fetch(
        `/api/import/class-list-entries/template?format=${templateFormat}`,
        {
          headers: {
            Authorization: `Bearer ${token}`,
          },
        },
      );

      if (!response.ok) {
        throw new Error("Fehler beim Herunterladen der Vorlage");
      }

      const blob = await response.blob();
      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.download =
        templateFormat === "xlsx"
          ? "klassenliste-import-vorlage.xlsx"
          : "klassenliste-import-vorlage.csv";
      document.body.appendChild(link);
      link.click();
      link.remove();
      URL.revokeObjectURL(url);
    } catch (err) {
      logger.error("template_download_failed", {
        error: err instanceof Error ? err.message : String(err),
        format: templateFormat,
      });
      setError(err instanceof Error ? err.message : "Unbekannter Fehler");
    }
  };

  const handleFileUpload = useCallback(
    async (file: File) => {
      setUploadedFile(file);
      setError(null);
      setIsLoading(true);
      setImportComplete(false);
      setImportResult(null);

      try {
        const token = session?.user?.token;
        if (!token) {
          throw new Error("Keine Authentifizierung");
        }

        const formData = new FormData();
        formData.append("file", file);

        const response = await fetch("/api/import/class-list-entries/preview", {
          method: "POST",
          headers: {
            Authorization: `Bearer ${token}`,
          },
          body: formData,
        });

        const result = (await response.json()) as Record<string, unknown>;

        if (!response.ok) {
          // Failure responses carry the actionable text in `error`
          // (api/common ErrResponse); `message` exists only on successes.
          throw new Error(
            (result.error as string | undefined) ??
              (result.message as string | undefined) ??
              "Fehler bei der Vorschau",
          );
        }

        const importData = result.data as ImportResult;
        const displayData: DisplayEntry[] = (importData.Errors ?? []).map(
          toDisplayEntry,
        );

        // Rows without issues are not in the Errors array — summarise them.
        const newCount = importData.TotalRows - displayData.length;
        if (newCount > 0 && displayData.length === 0) {
          displayData.push({
            row: 0,
            status: "new",
            errors: [],
            first_name: `${importData.TotalRows} Einträge`,
            last_name: "bereit zum Import",
            school_class: "",
          });
        }

        setPreviewData(displayData);
        setImportResult(importData);
      } catch (err) {
        logger.error("class_list_preview_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        setError(err instanceof Error ? err.message : "Unbekannter Fehler");
        setPreviewData([]);
      } finally {
        setIsLoading(false);
      }
    },
    [session],
  );

  const handleImport = async () => {
    if (!uploadedFile) return;

    setIsImporting(true);
    setError(null);

    try {
      const token = session?.user?.token;
      if (!token) {
        throw new Error("Keine Authentifizierung");
      }

      const formData = new FormData();
      formData.append("file", uploadedFile);

      const response = await fetch("/api/import/class-list-entries/import", {
        method: "POST",
        headers: {
          Authorization: `Bearer ${token}`,
        },
        body: formData,
      });

      const result = (await response.json()) as Record<string, unknown>;

      if (!response.ok) {
        throw new Error(
          (result.error as string | undefined) ??
            (result.message as string | undefined) ??
            "Fehler beim Import",
        );
      }

      const importData = result.data as ImportResult;
      setImportResult(importData);

      if (importData.ErrorCount > 0) {
        // Partial success: keep preview visible so the user sees which rows
        // were skipped (already on the class list) or failed.
        setPreviewData(importData.Errors.map(toDisplayEntry));
        const skipped = countAlreadyExists(importData.Errors);
        const failed = importData.ErrorCount - skipped;
        toast.warning(
          `${importData.CreatedCount} angelegt, ${skipped} übersprungen` +
            (failed > 0 ? `, ${failed} fehlgeschlagen` : ""),
        );
      } else {
        setImportComplete(true);
        toast.success(`${importData.CreatedCount} Einträge angelegt`);
        resetForm();
      }
    } catch (err) {
      logger.error("class_list_import_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setError(err instanceof Error ? err.message : "Unbekannter Fehler");
    } finally {
      setIsImporting(false);
    }
  };

  const handleDragEnter = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setIsDragging(true);
  };

  const handleDragLeave = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setIsDragging(false);
  };

  const handleDragOver = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
  };

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setIsDragging(false);

    const files = e.dataTransfer.files;
    if (files.length > 0) {
      const file = files[0];
      if (
        file &&
        (file.type === "text/csv" ||
          file.type ===
            "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" ||
          file.name.endsWith(".csv") ||
          file.name.endsWith(".xlsx"))
      ) {
        handleFileUpload(file).catch(() => undefined);
      } else {
        setError("Bitte nur CSV- oder Excel-Dateien (.csv, .xlsx) hochladen");
      }
    }
  };

  const existingCount = countAlreadyExists(importResult?.Errors ?? []);
  const stats = {
    total: importResult?.TotalRows ?? 0,
    new: importResult?.CreatedCount ?? 0,
    existing: existingCount,
    errors: (importResult?.ErrorCount ?? 0) - existingCount,
  };

  if (status === "loading" || !isReady) {
    return (
      <div className="-mt-1.5 w-full space-y-6">
        <BackButton referrer="/database/students/class-list" />

        <PageHeaderWithSearch title="Klassenliste importieren" />

        <SkeletonRegion label="Klassenlisten-Import wird geladen…">
          <FormSkeleton fields={2} />
        </SkeletonRegion>
      </div>
    );
  }

  return (
    <div className="-mt-1.5 w-full space-y-6">
      <BackButton referrer="/database/students/class-list" />

      <PageHeaderWithSearch title="Klassenliste importieren" />

      {/* Info Section */}
      <SectionCard
        kicker="Klassenliste"
        title="Sammelimport für Klassenlisteneinträge"
        icon={Info}
      >
        <ul className="list-inside list-disc space-y-1 text-sm text-gray-600">
          <li>Laden Sie die Vorlage herunter (siehe unten)</li>
          <li>
            Tragen Sie ab Zeile 2 pro Kind nur Vorname, Nachname und Klasse ein.
            Mehr braucht ein Klassenlisteneintrag nicht
          </li>
          <li>
            Kinder, die bereits in moto angelegt sind, werden übersprungen, sie
            stehen schon auf der Klassenliste
          </li>
          <li>Laden Sie die Datei hier hoch und überprüfen Sie die Vorschau</li>
          <li>Bestätigen Sie den Import</li>
        </ul>
      </SectionCard>

      {/* Error Display */}
      {error && (
        <div className="relative">
          <Alert type="error" message={error} />
          <Button
            type="button"
            variant="ghost"
            size="icon"
            onClick={() => setError(null)}
            className="text-moto-red hover:text-moto-red-strong absolute top-1/2 right-2 -translate-y-1/2"
            aria-label="Fehler schließen"
          >
            <X className="h-4 w-4" aria-hidden="true" />
          </Button>
        </div>
      )}

      {/* Download Template */}
      <SectionCard
        kicker="Schritt 1"
        title="Vorlage herunterladen"
        icon={Download}
      >
        <div className="flex flex-col gap-4 sm:flex-row sm:items-start">
          <div className="flex-1">
            <label
              id="format-select-label"
              htmlFor="format-select"
              className="mb-2 block text-sm font-medium text-gray-700"
            >
              Format wählen
            </label>
            <CustomSelect
              id="format-select"
              ariaLabelledBy="format-select-label"
              value={templateFormat}
              options={[
                { value: "csv", label: "CSV (Komma-getrennt)" },
                { value: "xlsx", label: "Excel (.xlsx)" },
              ]}
              onChange={(next) => setTemplateFormat(next as "csv" | "xlsx")}
            />
            <p className="mt-2 text-sm text-gray-500">
              Spalten: <span className="font-medium">Vorname</span>,{" "}
              <span className="font-medium">Nachname</span>,{" "}
              <span className="font-medium">Klasse</span>
            </p>
          </div>
          <div className="flex-1">
            {/* Spacer matches the format-select label height so the button
                aligns with the dropdown on the sm+ row layout. */}
            <span
              aria-hidden="true"
              className="mb-2 hidden text-sm font-medium text-gray-700 sm:block"
            >
              {"\u00A0"}
            </span>
            <Button
              type="button"
              variant="primary"
              size="sm"
              onClick={() => handleDownloadTemplate().catch(() => undefined)}
              className="h-10 w-full gap-2"
            >
              <Download className="h-5 w-5" aria-hidden="true" />
              Vorlage herunterladen
            </Button>
          </div>
        </div>
      </SectionCard>

      {/* Upload Section */}
      <UploadSection
        isDragging={isDragging}
        isLoading={isLoading}
        uploadedFile={uploadedFile}
        onDragEnter={handleDragEnter}
        onDragLeave={handleDragLeave}
        onDragOver={handleDragOver}
        onDrop={handleDrop}
        onFileSelect={(file) => handleFileUpload(file).catch(() => undefined)}
      />

      {/* Preview Section */}
      {previewData.length > 0 && !importComplete && (
        <>
          <StatsCards
            total={stats.total}
            newCount={stats.new}
            existing={stats.existing}
            errors={stats.errors}
          />

          <SectionCard
            kicker="Schritt 3"
            title="Datenvorschau"
            icon={ListChecks}
          >
            <div className="space-y-2">
              {previewData.map((entry, idx) => (
                <StudentRowCard
                  key={entry.row}
                  student={{
                    row: entry.row,
                    status: entry.status,
                    errors: entry.errors,
                    first_name: entry.first_name,
                    last_name: entry.last_name,
                    meta: [entry.school_class],
                  }}
                  index={idx}
                />
              ))}
            </div>
          </SectionCard>

          {/* Spacer for sticky action bar */}
          <div className="h-20" />

          {/* Action Buttons */}
          <div className="sticky bottom-4 z-10 flex flex-col gap-2 rounded-xl border border-gray-200 bg-white/95 px-4 py-3 shadow-lg backdrop-blur-sm sm:flex-row sm:gap-3">
            <Button
              type="button"
              variant="outline"
              size="md"
              className="flex-1"
              onClick={resetForm}
            >
              Abbrechen
            </Button>
            <Button
              type="button"
              variant="primary"
              size="md"
              className="flex-1"
              onClick={() => void handleImport()}
              disabled={stats.new === 0 || isImporting}
            >
              {isImporting
                ? "Importiere..."
                : `${stats.new} Einträge importieren`}
            </Button>
          </div>
        </>
      )}
    </div>
  );
}
