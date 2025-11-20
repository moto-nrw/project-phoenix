"use client";

import { useState, useCallback } from "react";
import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import { ResponsiveLayout } from "~/components/dashboard";
import Link from "next/link";
import { Loading } from "~/components/ui/loading";

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

      const response = await fetch("/api/import/students/template", {
        headers: {
          Authorization: `Bearer ${token}`,
        },
      });

      if (!response.ok) {
        throw new Error("Fehler beim Herunterladen der Vorlage");
      }

      const blob = await response.blob();
      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.download = "schueler-import-vorlage.csv";
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

        const result = await response.json();

        if (!response.ok) {
          throw new Error(result.message || "Fehler bei der Vorschau");
        }

        // Transform backend response to display format
        const importData = result.data as ImportResult;
        const displayData: DisplayStudent[] = [];

        // Process errors (rows with issues)
        if (importData.Errors) {
          for (const row of importData.Errors) {
            const hasErrors = row.Errors.some((e) => e.severity === "error");
            const hasWarnings = row.Errors.some((e) => e.severity === "warning");
            const isExisting = row.Errors.some((e) => e.code === "duplicate");

            displayData.push({
              row: row.RowNumber,
              status: hasErrors
                ? "error"
                : isExisting
                  ? "existing"
                  : hasWarnings
                    ? "warning"
                    : "new",
              errors: row.Errors.map((e) => e.message),
              first_name: row.Data.first_name,
              last_name: row.Data.last_name,
              school_class: row.Data.school_class,
              group_name: row.Data.group_name || "",
              guardian_info:
                row.Data.guardians && row.Data.guardians.length > 0
                  ? `${row.Data.guardians[0]?.first_name || ""} ${row.Data.guardians[0]?.last_name || ""} (${row.Data.guardians[0]?.relationship_type || ""})`
                  : "",
              health_info: row.Data.health_info || "",
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

      const result = await response.json();

      if (!response.ok) {
        throw new Error(result.message || "Fehler beim Import");
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
      if (file && (file.type === "text/csv" || file.name.endsWith(".csv"))) {
        void handleFileUpload(file);
      } else {
        setError("Bitte nur CSV-Dateien hochladen");
      }
    }
  };

  // Get status badge
  const getStatusBadge = (rowStatus: RowStatus) => {
    switch (rowStatus) {
      case "new":
        return (
          <span className="inline-flex items-center rounded-full bg-green-100 px-2 py-1 text-xs font-medium text-green-700">
            Neu
          </span>
        );
      case "existing":
        return (
          <span className="inline-flex items-center rounded-full bg-blue-100 px-2 py-1 text-xs font-medium text-blue-700">
            Vorhanden
          </span>
        );
      case "error":
        return (
          <span className="inline-flex items-center rounded-full bg-red-100 px-2 py-1 text-xs font-medium text-red-700">
            Fehler
          </span>
        );
      case "warning":
        return (
          <span className="inline-flex items-center rounded-full bg-amber-100 px-2 py-1 text-xs font-medium text-amber-700">
            Warnung
          </span>
        );
    }
  };

  // Stats
  const stats = {
    total: importResult?.TotalRows || 0,
    new:
      (importResult?.TotalRows || 0) -
      (importResult?.ErrorCount || 0) -
      (previewData.filter((r) => r.status === "existing").length || 0),
    existing: previewData.filter((r) => r.status === "existing").length,
    errors: importResult?.ErrorCount || 0,
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
                CSV-Import Anleitung
              </h3>
              <ul className="list-inside list-disc space-y-1 text-sm text-gray-600">
                <li>
                  Laden Sie die{" "}
                  <button
                    onClick={() => void handleDownloadTemplate()}
                    className="font-medium text-blue-600 underline hover:text-blue-800"
                  >
                    Muster-CSV
                  </button>{" "}
                  herunter
                </li>
                <li>
                  Füllen Sie die Datei mit Ihren Schülerdaten aus (Excel oder
                  Texteditor)
                </li>
                <li>Speichern Sie die Datei als CSV (UTF-8, Komma-getrennt)</li>
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
          <button
            onClick={() => void handleDownloadTemplate()}
            className="flex items-center gap-3 rounded-xl bg-gradient-to-br from-purple-500 to-purple-600 px-6 py-3 text-white shadow-lg transition-all duration-300 hover:scale-105 hover:shadow-xl active:scale-95"
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
            <span className="font-semibold">Muster-CSV herunterladen</span>
          </button>
        </div>

        {/* Upload Section */}
        <div className="rounded-xl border border-gray-100 bg-white p-6">
          <h3 className="mb-4 flex items-center gap-2 text-sm font-semibold text-gray-900">
            <svg
              className="h-5 w-5 text-green-600"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12"
              />
            </svg>
            Schritt 2: CSV-Datei hochladen
          </h3>

          {/* Drag & Drop Area */}
          <div
            onDragEnter={handleDragEnter}
            onDragLeave={handleDragLeave}
            onDragOver={handleDragOver}
            onDrop={handleDrop}
            className={`relative rounded-xl border-2 border-dashed p-12 text-center transition-all duration-300 ${
              isDragging
                ? "border-green-500 bg-green-50"
                : "border-gray-300 bg-gray-50 hover:border-gray-400"
            }`}
          >
            {isLoading ? (
              <div className="flex flex-col items-center gap-4">
                <div className="h-12 w-12 animate-spin rounded-full border-4 border-gray-300 border-t-green-600"></div>
                <p className="text-sm text-gray-600">Datei wird analysiert...</p>
              </div>
            ) : (
              <div className="flex flex-col items-center gap-4">
                <svg
                  className={`h-16 w-16 transition-colors ${isDragging ? "text-green-500" : "text-gray-400"}`}
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={1.5}
                    d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12"
                  />
                </svg>
                <div>
                  <p className="mb-1 text-lg font-medium text-gray-900">
                    {isDragging
                      ? "Datei hier ablegen..."
                      : "Datei hierher ziehen"}
                  </p>
                  <p className="text-sm text-gray-500">oder</p>
                </div>
                <label className="cursor-pointer">
                  <span className="inline-flex items-center gap-2 rounded-lg bg-gray-900 px-6 py-3 text-white transition-colors hover:bg-gray-700">
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
                        d="M15.172 7l-6.586 6.586a2 2 0 102.828 2.828l6.414-6.586a4 4 0 00-5.656-5.656l-6.415 6.585a6 6 0 108.486 8.486L20.5 13"
                      />
                    </svg>
                    Datei auswählen
                  </span>
                  <input
                    type="file"
                    accept=".csv"
                    onChange={(e) => {
                      const file = e.target.files?.[0];
                      if (file) void handleFileUpload(file);
                    }}
                    className="hidden"
                  />
                </label>
                {uploadedFile && (
                  <div className="flex items-center gap-2 rounded-lg border border-gray-200 bg-white px-4 py-2 text-sm text-gray-600">
                    <svg
                      className="h-4 w-4 text-green-600"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                    >
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        strokeWidth={2}
                        d="M5 13l4 4L19 7"
                      />
                    </svg>
                    {uploadedFile.name}
                  </div>
                )}
              </div>
            )}
          </div>
        </div>

        {/* Preview Section */}
        {previewData.length > 0 && !importComplete && (
          <>
            {/* Statistics */}
            <div className="grid grid-cols-2 gap-4 md:grid-cols-4">
              <div className="rounded-xl border border-gray-100 bg-white p-4">
                <p className="text-2xl font-bold text-gray-900">{stats.total}</p>
                <p className="text-xs text-gray-600">Gesamt</p>
              </div>
              <div className="rounded-xl border border-green-100 bg-green-50 p-4">
                <p className="text-2xl font-bold text-green-700">{stats.new}</p>
                <p className="text-xs text-green-600">Neu</p>
              </div>
              <div className="rounded-xl border border-blue-100 bg-blue-50 p-4">
                <p className="text-2xl font-bold text-blue-700">
                  {stats.existing}
                </p>
                <p className="text-xs text-blue-600">Vorhanden</p>
              </div>
              <div className="rounded-xl border border-red-100 bg-red-50 p-4">
                <p className="text-2xl font-bold text-red-700">{stats.errors}</p>
                <p className="text-xs text-red-600">Fehler</p>
              </div>
            </div>

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
                {previewData.map((student, index) => (
                  <div
                    key={index}
                    className="rounded-xl border border-gray-100 bg-white p-3"
                  >
                    <div className="flex items-center gap-3">
                      <div className="flex h-9 w-9 items-center justify-center rounded-full bg-gray-100 text-xs font-semibold text-gray-600">
                        {student.row || index + 1}
                      </div>
                      <div className="min-w-0 flex-1">
                        <div className="flex items-center gap-2">
                          <h4 className="text-sm font-semibold text-gray-900">
                            {student.first_name} {student.last_name}
                          </h4>
                          {getStatusBadge(student.status)}
                        </div>
                        <div className="mt-0.5 flex flex-wrap items-center gap-2 text-xs text-gray-500">
                          {student.school_class && (
                            <span>{student.school_class}</span>
                          )}
                          {student.group_name && (
                            <>
                              <span>•</span>
                              <span>{student.group_name}</span>
                            </>
                          )}
                          {student.guardian_info && (
                            <>
                              <span>•</span>
                              <span>{student.guardian_info}</span>
                            </>
                          )}
                        </div>
                        {student.errors.length > 0 && (
                          <p className="mt-1 text-xs text-red-600">
                            {student.errors.join(", ")}
                          </p>
                        )}
                      </div>
                    </div>
                  </div>
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
