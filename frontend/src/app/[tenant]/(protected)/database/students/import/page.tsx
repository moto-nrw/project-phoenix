"use client";

import { useState, useCallback, useRef } from "react";
import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import { Download, Info, ListChecks, RefreshCw, X } from "lucide-react";
import { SkeletonRegion, FormSkeleton } from "~/components/ui/page-skeletons";
import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import { Alert } from "~/components/ui/alert";
import { BackButton } from "~/components/ui/back-button";
import { PageIntro } from "~/components/ui/page-intro";
import { SectionCard } from "~/components/ui/section-card";
import { UploadSection } from "~/components/import/upload-section";
import { StatsCards } from "~/components/import/stats-cards";
import { StudentRowCard } from "~/components/import/student-row-card";
import { SegmentedControl } from "~/components/ui/segmented-control";
import {
  IMPORT_MODE_HINTS,
  IMPORT_MODE_ITEMS,
  type ImportMode,
} from "~/lib/import-mode";
import { useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "StudentImportPage" });

// Types matching backend API response
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
    group_name: string;
    birthday: string;
    guardians: Array<{
      first_name: string;
      last_name: string;
      email: string;
      phone: string;
      relationship_type: string;
      is_primary: boolean;
    }>;
    health_info?: string;
    supervisor_notes?: string;
    extra_info?: string;
    privacy_accepted: boolean;
    data_retention_days: number;
    bus_permission: boolean;
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

// Status types for display
type RowStatus = "new" | "existing" | "error" | "warning";

interface DisplayStudent {
  row: number;
  status: RowStatus;
  errors: string[];
  notes: string[];
  first_name: string;
  last_name: string;
  school_class: string;
  group_name: string;
  guardian_info: string;
  health_info: string;
}

function childCountLabel(count: number): string {
  return count === 1 ? "1 Kind" : `${count} Kinder`;
}

/** Blocking and warning messages in red; plain hints stay gray. */
function splitMessages(errors: ImportError[]): {
  errors: string[];
  notes: string[];
} {
  return {
    errors: errors.filter((e) => e.severity !== "info").map((e) => e.message),
    notes: errors.filter((e) => e.severity === "info").map((e) => e.message),
  };
}

/** "Maria Muster (Mutter)" from whatever parts the row carries; empty when none. */
function guardianLabel(
  guardians: ImportRowResult["Data"]["guardians"] | undefined,
): string {
  const first = guardians?.[0];
  if (!first) return "";
  const name = [first.first_name, first.last_name]
    .filter((part) => part && part.trim() !== "")
    .join(" ");
  const relation = first.relationship_type?.trim() ?? "";
  if (name && relation) return `${name} (${relation})`;
  return name || relation || first.email || "";
}

export default function StudentImportPage() {
  const previewGeneration = useRef(0);
  const [uploadedFile, setUploadedFile] = useState<File | null>(null);
  const [previewData, setPreviewData] = useState<DisplayStudent[]>([]);
  const [isDragging, setIsDragging] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [isImporting, setIsImporting] = useState(false);
  const [importComplete, setImportComplete] = useState(false);
  const [importResult, setImportResult] = useState<ImportResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [templateFormat, setTemplateFormat] = useState<"csv" | "xlsx">("xlsx");
  const [mode, setMode] = useState<ImportMode>("create");

  const { data: session, status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
    },
  });

  const toast = useToast();

  // Reset form to initial state
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

  // Handle template download from backend
  const handleDownloadTemplate = async () => {
    try {
      const token = session?.user?.token;
      if (!token) {
        setError("Keine Authentifizierung");
        return;
      }

      const response = await fetch(
        `/api/import/students/template?format=${templateFormat}`,
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
          ? "schueler-import-vorlage.xlsx"
          : "schueler-import-vorlage.csv";
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

  // Handle file upload and preview via backend API
  const handleFileUpload = useCallback(
    async (file: File, importMode: ImportMode = mode) => {
      const generation = ++previewGeneration.current;
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
        formData.append("mode", importMode);

        const response = await fetch("/api/import/students/preview", {
          method: "POST",
          headers: {
            Authorization: `Bearer ${token}`,
          },
          body: formData,
        });

        const result = (await response.json()) as Record<string, unknown>;

        if (!response.ok) {
          throw new Error(
            (result.message as string | undefined) ?? "Fehler bei der Vorschau",
          );
        }

        // Transform backend response to display format
        const importData = result.data as ImportResult;
        const displayData: DisplayStudent[] = [];

        // Process errors (rows with issues)
        if (importData.Errors) {
          for (const row of importData.Errors) {
            const hasErrors = row.Errors.some((e) => e.severity === "error");
            const hasWarnings = row.Errors.some(
              (e) => e.severity === "warning",
            );
            const isExisting = row.Errors.some(
              (e) => e.code === "already_exists" || e.code === "will_update",
            );

            // Determine row status based on error conditions
            // Check isExisting first because already_exists has severity "error"
            const getRowStatus = ():
              "error" | "existing" | "warning" | "new" => {
              if (isExisting) return "existing";
              if (hasErrors) return "error";
              if (hasWarnings) return "warning";
              return "new";
            };

            displayData.push({
              row: row.RowNumber,
              status: getRowStatus(),
              ...splitMessages(row.Errors),
              first_name: row.Data.first_name,
              last_name: row.Data.last_name,
              school_class: row.Data.school_class,
              group_name: row.Data.group_name ?? "",
              guardian_info: guardianLabel(row.Data.guardians),
              health_info: row.Data.health_info ?? "",
            });
          }
        }

        // Calculate how many are new (total - errors)
        const newCount = importData.TotalRows - displayData.length;

        // Add placeholder entries for successful rows (they're not in Errors array)
        // Note: In a real implementation, we'd want the backend to return all rows
        if (newCount > 0 && displayData.length === 0) {
          // If no errors, create a summary
          displayData.push({
            row: 0,
            status: "new",
            errors: [],
            notes: [],
            first_name: `${importData.TotalRows} Kinder`,
            last_name: "bereit zum Import",
            school_class: "",
            group_name: "",
            guardian_info: "",
            health_info: "",
          });
        }

        if (generation !== previewGeneration.current) return;
        setPreviewData(displayData);
        setImportResult(importData);
      } catch (err) {
        logger.error("student_preview_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        if (generation !== previewGeneration.current) return;
        setError(err instanceof Error ? err.message : "Unbekannter Fehler");
        setPreviewData([]);
      } finally {
        if (generation === previewGeneration.current) setIsLoading(false);
      }
    },
    [session, mode],
  );

  // A new mode changes what the preview means, so the uploaded file is
  // checked again right away.
  const handleModeChange = (next: ImportMode) => {
    setMode(next);
    if (uploadedFile) {
      setPreviewData([]);
      handleFileUpload(uploadedFile, next).catch(() => undefined);
    }
  };

  // Handle actual import
  const handleImport = async () => {
    if (!uploadedFile || isLoading) return;

    setIsImporting(true);
    setError(null);

    try {
      const token = session?.user?.token;
      if (!token) {
        throw new Error("Keine Authentifizierung");
      }

      const formData = new FormData();
      formData.append("file", uploadedFile);
      formData.append("mode", mode);

      const response = await fetch("/api/import/students/import", {
        method: "POST",
        headers: {
          Authorization: `Bearer ${token}`,
        },
        body: formData,
      });

      const result = (await response.json()) as Record<string, unknown>;

      if (!response.ok) {
        throw new Error(
          (result.message as string | undefined) ?? "Fehler beim Import",
        );
      }

      const importData = result.data as ImportResult;
      setImportResult(importData);

      // Handle partial failures vs full success
      if (importData.ErrorCount > 0) {
        // Partial success: Show warning and keep form to display error details
        // Don't set importComplete - keep preview visible so user sees which rows failed
        // Update previewData with error details from import result
        const errorDisplayData: DisplayStudent[] = importData.Errors.map(
          (row) => ({
            row: row.RowNumber,
            status: row.Errors.some(
              (error) =>
                error.code === "already_exists" || error.code === "will_update",
            )
              ? "existing"
              : row.Errors.some((error) => error.severity === "error")
                ? "error"
                : row.Errors.some((error) => error.severity === "warning")
                  ? "warning"
                  : "new",
            ...splitMessages(row.Errors),
            first_name: row.Data.first_name,
            last_name: row.Data.last_name,
            school_class: row.Data.school_class,
            group_name: row.Data.group_name ?? "",
            guardian_info: guardianLabel(row.Data.guardians),
            health_info: row.Data.health_info ?? "",
          }),
        );
        setPreviewData(errorDisplayData);
        toast.warning(
          `${childCountLabel(importData.CreatedCount)} importiert, ${importData.UpdatedCount} aktualisiert, ${importData.ErrorCount} übersprungen`,
        );
      } else {
        // Full success: Mark complete, show success toast and reset form for next import
        setImportComplete(true);
        toast.success(
          `${childCountLabel(importData.CreatedCount)} importiert, ${importData.UpdatedCount} aktualisiert`,
        );
        resetForm();
      }
    } catch (err) {
      logger.error("student_import_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setError(err instanceof Error ? err.message : "Unbekannter Fehler");
    } finally {
      setIsImporting(false);
    }
  };

  // Drag and drop handlers
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

  // Stats - use backend counts directly
  const stats = {
    total: importResult?.TotalRows ?? 0,
    new: importResult?.CreatedCount ?? 0,
    existing: importResult?.UpdatedCount ?? 0,
    errors: importResult?.ErrorCount ?? 0,
  };
  const importLabel =
    mode === "create"
      ? `${childCountLabel(stats.new)} importieren`
      : mode === "update"
        ? `${childCountLabel(stats.existing)} aktualisieren`
        : `${childCountLabel(stats.new + stats.existing)} übernehmen`;

  // Statuszeile des Seitenkopfs: der Stand des Imports, nicht ein Erklärsatz.
  const statusLine = uploadedFile
    ? [
        uploadedFile.name,
        importComplete
          ? "Import abgeschlossen"
          : `${stats.total} ${stats.total === 1 ? "Zeile" : "Zeilen"}`,
        !importComplete && stats.errors > 0
          ? `${stats.errors} ${stats.errors === 1 ? "Fehler" : "Fehler"}`
          : null,
      ]
        .filter(Boolean)
        .join(" · ")
    : "Noch keine Datei gewählt";

  if (status === "loading") {
    return (
      <div className="w-full space-y-6">
        <BackButton referrer="/database/students" />

        <PageIntro
          kicker="Datenverwaltung"
          title="Kinder importieren"
          description={statusLine}
        />

        <SkeletonRegion label="Kinder-Import wird geladen…">
          <FormSkeleton fields={2} />
        </SkeletonRegion>
      </div>
    );
  }

  return (
    <div className="w-full space-y-6">
      <BackButton referrer="/database/students" />

      <PageIntro
        kicker="Datenverwaltung"
        title="Kinder importieren"
        description={statusLine}
      />

      {/* Info Section */}
      <section className="border-moto-blue/20 bg-moto-blue-soft rounded-2xl border p-5">
        <div className="flex items-center gap-3">
          <span className="text-moto-blue flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-white">
            <Info className="h-5 w-5" aria-hidden="true" />
          </span>
          <h2 className="text-base font-semibold text-gray-900">
            Import-Anleitung
          </h2>
        </div>
        <div className="mt-4">
          <ul className="list-inside list-disc space-y-1 text-sm text-gray-600">
            <li>Laden Sie die Vorlage herunter (siehe unten)</li>
            <li>Füllen Sie die Datei mit Ihren Kinderdaten aus</li>
            <li>
              Für Geburtstage sind diese Formate erlaubt: JJJJ-MM-TT, TT.MM.JJJJ
              oder TT.MM.JJ
            </li>
            <li>
              Die Vorlage enthält auch Adresse, RFID-Karte und bis zu vier
              Erziehungsberechtigte. Das Blatt „Hinweise“ erklärt jede Spalte
            </li>
            <li>Speichern Sie die ausgefüllte Datei</li>
            <li>
              Laden Sie die Datei hier hoch und überprüfen Sie die Vorschau
            </li>
            <li>Bestätigen Sie den Import</li>
          </ul>
        </div>
      </section>

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

      {/* Download Template Button */}
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
              Beispiel Geburtstag:{" "}
              <span className="font-medium">2015-08-15</span>,{" "}
              <span className="font-medium">15.08.2015</span> oder{" "}
              <span className="font-medium">15.08.15</span>
            </p>
          </div>
          <div className="flex-1">
            {/* Spacer matches the format-select label height so the button
                aligns with the dropdown on the sm+ row layout. */}
            {/* Non-breaking space keeps a reliable label-height line box
                (a normal space would collapse) without duplicating the
                "Format wählen" label text. */}
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

      <SectionCard
        kicker="Schritt 2"
        title="Was soll der Import tun?"
        icon={RefreshCw}
      >
        <SegmentedControl
          items={IMPORT_MODE_ITEMS}
          value={mode}
          onChange={handleModeChange}
          fullWidth
          ariaLabel="Import-Modus"
        />
        <p className="mt-3 text-sm text-gray-600">{IMPORT_MODE_HINTS[mode]}</p>
        <p className="mt-1 text-sm text-gray-500">
          Bekannt ist eine Zeile über Vorname, Nachname und Klasse. Bei
          Klassenwechsel über die RFID-Karte oder den Geburtstag.
        </p>
      </SectionCard>

      {/* Upload Section */}
      <UploadSection
        title="Schritt 3: Datei hochladen"
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
          {/* Statistics */}
          <StatsCards
            total={stats.total}
            newCount={stats.new}
            existing={stats.existing}
            existingTitle={
              mode === "create" ? "Vorhanden" : "Wird aktualisiert"
            }
            errors={stats.errors}
          />

          {/* Data List */}
          <SectionCard
            kicker="Schritt 4"
            title="Datenvorschau"
            icon={ListChecks}
          >
            <div className="space-y-2">
              {previewData.map((student, idx) => (
                <StudentRowCard
                  key={student.row}
                  student={{
                    row: student.row,
                    status: student.status,
                    errors: student.errors,
                    notes: student.notes,
                    first_name: student.first_name,
                    last_name: student.last_name,
                    meta: [
                      student.school_class,
                      student.group_name,
                      student.guardian_info,
                    ],
                  }}
                  index={idx}
                />
              ))}
            </div>
          </SectionCard>

          {/* Spacer for sticky action bar */}
          <div className="h-20" />

          {/* Action Buttons */}
          <div className="sticky bottom-4 z-10 flex flex-col gap-2 rounded-2xl border border-gray-200 bg-white/95 px-4 py-3 shadow-lg backdrop-blur-sm sm:flex-row sm:gap-3">
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
              variant="success"
              size="md"
              className="flex-1"
              disabled={stats.errors > 0 || isImporting || isLoading}
              onClick={() => void handleImport()}
            >
              {isImporting ? "Wird importiert…" : importLabel}
            </Button>
          </div>
        </>
      )}
    </div>
  );
}
