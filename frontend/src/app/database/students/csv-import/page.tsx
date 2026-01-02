"use client";

import { useState, useCallback } from "react";
import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import { ResponsiveLayout } from "~/components/dashboard";
import Link from "next/link";
import { Loading } from "~/components/ui/loading";
import { UploadSection, StatsCards, StudentRowCard } from "~/components/import";

// Types matching backend API response
interface ImportError {
  field: string;
  message: string;
  code: string;
  severity: "error" | "warning";
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
  first_name: string;
  last_name: string;
  school_class: string;
  group_name: string;
  guardian_info: string;
  health_info: string;
}

export default function StudentCSVImportPage() {
  const [uploadedFile, setUploadedFile] = useState<File | null>(null);
  const [previewData, setPreviewData] = useState<DisplayStudent[]>([]);
  const [isDragging, setIsDragging] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [isImporting, setIsImporting] = useState(false);
  const [importComplete, setImportComplete] = useState(false);
  const [importResult, setImportResult] = useState<ImportResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [templateFormat, setTemplateFormat] = useState<"csv" | "xlsx">("csv");

  const { data: session, status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
    },
  });

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
      document.body.removeChild(link);
      URL.revokeObjectURL(url);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unbekannter Fehler");
    }
  };

  // Handle file upload and preview via backend API
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
            const isExisting = row.Errors.some((e) => e.code === "duplicate");

            // Determine row status based on error conditions
            const getRowStatus = ():
              | "error"
              | "existing"
              | "warning"
              | "new" => {
              if (hasErrors) return "error";
              if (isExisting) return "existing";
              if (hasWarnings) return "warning";
              return "new";
            };

            displayData.push({
              row: row.RowNumber,
              status: getRowStatus(),
              errors: row.Errors.map((e) => e.message),
              first_name: row.Data.first_name,
              last_name: row.Data.last_name,
              school_class: row.Data.school_class,
              group_name: row.Data.group_name ?? "",
              guardian_info:
                row.Data.guardians && row.Data.guardians.length > 0
                  ? `${row.Data.guardians[0]?.first_name ?? ""} ${row.Data.guardians[0]?.last_name ?? ""} (${row.Data.guardians[0]?.relationship_type ?? ""})`
                  : "",
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
            first_name: `${importData.TotalRows} Schüler`,
            last_name: "bereit zum Import",
            school_class: "",
            group_name: "",
            guardian_info: "",
            health_info: "",
          });
        }

        setPreviewData(displayData);
        setImportResult(importData);
      } catch (err) {
        setError(err instanceof Error ? err.message : "Unbekannter Fehler");
        setPreviewData([]);
      } finally {
        setIsLoading(false);
      }
    },
    [session],
  );

  // Handle actual import
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

      setImportResult(result.data as ImportResult);
      setImportComplete(true);
    } catch (err) {
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
        setError("Bitte nur CSV- oder Excel-Dateien hochladen");
      }
    }
  };

  // Stats
  const stats = {
    total: importResult?.TotalRows ?? 0,
    new:
      (importResult?.TotalRows ?? 0) -
      (importResult?.ErrorCount ?? 0) -
      (previewData.filter((r) => r.status === "existing").length ?? 0),
    existing: previewData.filter((r) => r.status === "existing").length,
    errors: importResult?.ErrorCount ?? 0,
  };

  if (status === "loading") {
    return (
      <ResponsiveLayout>
        <Loading fullPage={false} />
      </ResponsiveLayout>
    );
  }

  return (
    <ResponsiveLayout>
      <div className="w-full space-y-6">
        {/* Info Section */}
        <div className="rounded-xl border border-blue-100 bg-blue-50/30 p-6">
          <div className="flex items-start gap-4">
            <div className="flex-shrink-0">
              <svg
                className="h-6 w-6 text-blue-600"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                />
              </svg>
            </div>
            <div className="flex-1">
              <h3 className="mb-2 text-sm font-semibold text-gray-900">
                CSV/Excel-Import Anleitung
              </h3>
              <ul className="list-inside list-disc space-y-1 text-sm text-gray-600">
                <li>
                  Laden Sie die Vorlage herunter (CSV oder Excel - siehe unten)
                </li>
                <li>Füllen Sie die Datei mit Ihren Schülerdaten aus</li>
                <li>
                  Speichern Sie die Datei (CSV behält das Format, Excel wird als
                  .xlsx gespeichert)
                </li>
                <li>
                  Laden Sie die Datei hier hoch und überprüfen Sie die Vorschau
                </li>
                <li>Bestätigen Sie den Import</li>
              </ul>
            </div>
          </div>
        </div>

        {/* Error Display */}
        {error && (
          <div className="rounded-xl border border-red-200 bg-red-50 p-4">
            <div className="flex items-center gap-3">
              <svg
                className="h-5 w-5 text-red-600"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                />
              </svg>
              <p className="text-sm text-red-800">{error}</p>
              <button
                onClick={() => setError(null)}
                className="ml-auto text-red-600 hover:text-red-800"
              >
                <svg
                  className="h-4 w-4"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M6 18L18 6M6 6l12 12"
                  />
                </svg>
              </button>
            </div>
          </div>
        )}

        {/* Import Complete Message */}
        {importComplete && importResult && (
          <div className="rounded-xl border border-green-200 bg-green-50 p-6">
            <div className="flex items-start gap-4">
              <svg
                className="h-6 w-6 text-green-600"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"
                />
              </svg>
              <div>
                <h3 className="mb-2 text-sm font-semibold text-green-900">
                  Import abgeschlossen
                </h3>
                <p className="text-sm text-green-800">
                  {importResult.CreatedCount} Schüler erstellt,{" "}
                  {importResult.UpdatedCount} aktualisiert,{" "}
                  {importResult.ErrorCount} Fehler
                </p>
                <Link
                  href="/database/students"
                  className="mt-3 inline-block text-sm font-medium text-green-700 underline hover:text-green-900"
                >
                  Zur Schülerliste →
                </Link>
              </div>
            </div>
          </div>
        )}

        {/* Download Template Button */}
        <div className="rounded-xl border border-gray-100 bg-white p-6">
          <h3 className="mb-4 flex items-center gap-2 text-sm font-semibold text-gray-900">
            <svg
              className="h-5 w-5 text-purple-600"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4"
              />
            </svg>
            Schritt 1: Vorlage herunterladen
          </h3>
          <div className="flex flex-col gap-4 sm:flex-row sm:items-center">
            <div className="flex-1">
              <label
                htmlFor="format-select"
                className="mb-2 block text-sm font-medium text-gray-700"
              >
                Format wählen
              </label>
              <select
                id="format-select"
                value={templateFormat}
                onChange={(e) =>
                  setTemplateFormat(e.target.value as "csv" | "xlsx")
                }
                className="w-full rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm text-gray-900 focus:border-purple-500 focus:ring-2 focus:ring-purple-500/20 focus:outline-none"
              >
                <option value="csv">CSV (Komma-getrennt)</option>
                <option value="xlsx">Excel (.xlsx)</option>
              </select>
            </div>
            <div className="flex-1 sm:pt-6">
              <button
                onClick={() => handleDownloadTemplate().catch(() => undefined)}
                className="flex w-full items-center justify-center gap-3 rounded-xl bg-gradient-to-br from-purple-500 to-purple-600 px-6 py-3 text-white shadow-lg transition-all duration-300 hover:scale-105 hover:shadow-xl active:scale-95"
              >
                <svg
                  className="h-5 w-5"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M12 10v6m0 0l-3-3m3 3l3-3m2 8H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"
                  />
                </svg>
                <span className="font-semibold">Herunterladen</span>
              </button>
            </div>
          </div>
        </div>

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
            {/* Statistics */}
            <StatsCards
              total={stats.total}
              newCount={stats.new}
              existing={stats.existing}
              errors={stats.errors}
            />

            {/* Data List */}
            <div className="overflow-hidden rounded-xl border border-gray-100 bg-white">
              <div className="border-b border-gray-100 p-4">
                <h3 className="flex items-center gap-2 text-sm font-semibold text-gray-900">
                  <svg
                    className="h-5 w-5 text-blue-600"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2}
                      d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2"
                    />
                  </svg>
                  Schritt 3: Datenvorschau
                </h3>
              </div>

              <div className="space-y-2 p-3">
                {previewData.map((student) => (
                  <StudentRowCard
                    key={student.row}
                    student={student}
                    index={student.row}
                  />
                ))}
              </div>
            </div>

            {/* Action Buttons */}
            <div className="sticky bottom-6 flex flex-col gap-4 rounded-xl border border-gray-200 bg-white/95 p-4 shadow-lg backdrop-blur-sm sm:flex-row">
              <Link
                href="/database/students"
                className="flex-1 rounded-lg border border-gray-300 px-6 py-3 text-center text-sm font-medium text-gray-700 transition-colors hover:border-gray-400 hover:bg-gray-50"
              >
                Abbrechen
              </Link>
              <button
                onClick={() => void handleImport()}
                disabled={stats.errors > 0 || isImporting}
                className={`flex flex-1 items-center justify-center gap-2 rounded-lg px-6 py-3 text-sm font-medium text-white shadow-lg transition-all duration-300 ${
                  stats.errors > 0 || isImporting
                    ? "cursor-not-allowed bg-gray-400"
                    : "bg-gradient-to-br from-green-500 to-green-600 hover:scale-105 hover:shadow-xl active:scale-95"
                }`}
              >
                {isImporting ? (
                  <>
                    <div className="h-4 w-4 animate-spin rounded-full border-2 border-white border-t-transparent"></div>
                    Importiere...
                  </>
                ) : (
                  `${stats.new} Schüler importieren`
                )}
              </button>
            </div>
          </>
        )}
      </div>
    </ResponsiveLayout>
  );
}
