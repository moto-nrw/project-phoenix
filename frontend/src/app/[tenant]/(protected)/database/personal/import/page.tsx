"use client";

import { useState, useCallback, useEffect, useMemo } from "react";
import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import { Loading } from "~/components/ui/loading";
import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import { Alert } from "~/components/ui/alert";
import { UploadSection } from "~/components/import/upload-section";
import { StatsCards } from "~/components/import/stats-cards";
import { useToast } from "~/contexts/ToastContext";
import { createCrudService } from "~/lib/database/service-factory";
import { rolesConfig } from "~/components/database/configs/roles.config";
import { getRoleDisplayName, type Role } from "~/lib/auth-helpers";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "StaffImportPage" });

// Types matching the backend API response
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
    email: string;
    role_name: string;
    position?: string;
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

interface DisplayStaff {
  row: number;
  status: RowStatus;
  errors: string[];
  first_name: string;
  last_name: string;
  email: string;
  role_name: string;
  position: string;
}

function statusBadge(status: RowStatus) {
  switch (status) {
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
}

function rowStatusFor(errors: ImportError[]): RowStatus {
  const isExisting = errors.some((e) => e.code === "already_exists");
  if (isExisting) return "existing";
  if (errors.some((e) => e.severity === "error")) return "error";
  if (errors.some((e) => e.severity === "warning")) return "warning";
  return "new";
}

function toDisplayStaff(row: ImportRowResult): DisplayStaff {
  return {
    row: row.RowNumber,
    status: rowStatusFor(row.Errors),
    errors: row.Errors.map((e) => e.message),
    first_name: row.Data.first_name,
    last_name: row.Data.last_name,
    email: row.Data.email,
    role_name: row.Data.role_name,
    position: row.Data.position ?? "",
  };
}

export default function StaffImportPage() {
  const [uploadedFile, setUploadedFile] = useState<File | null>(null);
  const [previewData, setPreviewData] = useState<DisplayStaff[]>([]);
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

  const toast = useToast();

  // Load the tenant's role names so the user knows what to put in the
  // "Rolle" column (the import matches role names exactly, case-insensitive).
  const rolesService = useMemo(() => createCrudService(rolesConfig), []);
  const [availableRoles, setAvailableRoles] = useState<string[]>([]);

  useEffect(() => {
    rolesService
      .getList({ page: 1, pageSize: 500 })
      .then((data) => {
        const list: Role[] = Array.isArray(data.data) ? data.data : [];
        setAvailableRoles(
          list
            .map((r) => getRoleDisplayName(r.name))
            .filter((name) => name !== ""),
        );
      })
      .catch((err: unknown) => {
        logger.warn("roles_load_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
      });
  }, [rolesService]);

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
        `/api/import/teachers/template?format=${templateFormat}`,
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
          ? "mitarbeiter-import-vorlage.xlsx"
          : "mitarbeiter-import-vorlage.csv";
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

        const response = await fetch("/api/import/teachers/preview", {
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

        const importData = result.data as ImportResult;
        const displayData: DisplayStaff[] = (importData.Errors ?? []).map(
          toDisplayStaff,
        );

        // Rows without issues are not in the Errors array — summarise them.
        const newCount = importData.TotalRows - displayData.length;
        if (newCount > 0 && displayData.length === 0) {
          displayData.push({
            row: 0,
            status: "new",
            errors: [],
            first_name: `${importData.TotalRows} Mitarbeiter`,
            last_name: "bereit zum Import",
            email: "",
            role_name: "",
            position: "",
          });
        }

        setPreviewData(displayData);
        setImportResult(importData);
      } catch (err) {
        logger.error("staff_preview_failed", {
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

      const response = await fetch("/api/import/teachers/import", {
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

      if (importData.ErrorCount > 0) {
        // Partial success: keep preview visible so the user sees which rows failed.
        setPreviewData(importData.Errors.map(toDisplayStaff));
        toast.warning(
          `${importData.CreatedCount} eingeladen, ${importData.ErrorCount} übersprungen`,
        );
      } else {
        setImportComplete(true);
        toast.success(`${importData.CreatedCount} Mitarbeiter eingeladen`);
        resetForm();
      }
    } catch (err) {
      logger.error("staff_import_failed", {
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

  const stats = {
    total: importResult?.TotalRows ?? 0,
    new: importResult?.CreatedCount ?? 0,
    existing: importResult?.UpdatedCount ?? 0,
    errors: importResult?.ErrorCount ?? 0,
  };

  if (status === "loading") {
    return <Loading fullPage={false} />;
  }

  return (
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
              Import-Anleitung
            </h3>
            <ul className="list-inside list-disc space-y-1 text-sm text-gray-600">
              <li>Laden Sie die Vorlage herunter (siehe unten)</li>
              <li>Füllen Sie die Datei mit Ihren Mitarbeiterdaten aus</li>
              <li>
                Die Spalte „Rolle" muss exakt einer vorhandenen Rolle
                entsprechen
                {availableRoles.length > 0 && (
                  <>
                    :{" "}
                    {availableRoles.map((role, i) => (
                      <span key={role}>
                        {i > 0 && ", "}
                        <span className="font-medium text-gray-900">
                          {role}
                        </span>
                      </span>
                    ))}
                  </>
                )}
              </li>
              <li>
                Der Import verschickt Einladungen — Mitarbeitende setzen ihr
                Passwort selbst über den Link in der E-Mail
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
        <div className="relative">
          <Alert type="error" message={error} />
          <button
            type="button"
            onClick={() => setError(null)}
            className="absolute top-1/2 right-4 -translate-y-1/2 text-red-600 hover:text-red-800"
            aria-label="Fehler schließen"
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
      )}

      {/* Download Template */}
      <div className="rounded-xl border border-gray-100 bg-white p-6">
        <h3 className="mb-4 flex items-center gap-2 text-sm font-semibold text-gray-900">
          <svg
            className="h-5 w-5 text-gray-600"
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
              <span className="font-medium">Email</span>,{" "}
              <span className="font-medium">Rolle</span>,{" "}
              <span className="font-medium">Position</span> (optional)
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
              Vorlage herunterladen
            </Button>
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
          <StatsCards
            total={stats.total}
            newCount={stats.new}
            existing={stats.existing}
            errors={stats.errors}
          />

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
              {previewData.map((staff, idx) => (
                <div
                  key={staff.row}
                  className="rounded-xl border border-gray-100 bg-white p-3"
                >
                  <div className="flex items-center gap-3">
                    <div className="flex h-9 w-9 items-center justify-center rounded-full bg-gray-100 text-xs font-semibold text-gray-600">
                      {staff.row || idx + 1}
                    </div>
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-2">
                        <h4 className="text-sm font-semibold text-gray-900">
                          {staff.first_name} {staff.last_name}
                        </h4>
                        {statusBadge(staff.status)}
                      </div>
                      <div className="mt-0.5 flex flex-wrap items-center gap-2 text-xs text-gray-500">
                        {staff.email && <span>{staff.email}</span>}
                        {staff.role_name && (
                          <>
                            <span>•</span>
                            <span>{staff.role_name}</span>
                          </>
                        )}
                        {staff.position && (
                          <>
                            <span>•</span>
                            <span>{staff.position}</span>
                          </>
                        )}
                      </div>
                      {staff.errors.length > 0 && (
                        <p className="mt-1 text-xs text-red-600">
                          {staff.errors.join(", ")}
                        </p>
                      )}
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </div>

          {/* Spacer for sticky action bar */}
          <div className="h-20" />

          {/* Action Buttons */}
          <div className="sticky bottom-4 z-10 flex flex-col gap-2 rounded-xl border border-gray-200 bg-white/95 px-4 py-3 shadow-lg backdrop-blur-sm sm:flex-row sm:gap-3">
            <button
              type="button"
              onClick={resetForm}
              className="flex-1 rounded-lg bg-gray-200 px-3 py-2 text-xs font-medium text-gray-800 transition-all duration-200 hover:bg-gray-300 hover:shadow-md md:px-4 md:text-sm"
            >
              Abbrechen
            </button>
            <button
              type="button"
              onClick={() => void handleImport()}
              disabled={stats.errors > 0 || isImporting}
              className="bg-moto-green hover:bg-moto-green-hover flex-1 rounded-lg px-3 py-2 text-xs font-medium text-gray-950 transition-all duration-200 hover:shadow-lg disabled:cursor-not-allowed disabled:opacity-50 md:px-4 md:text-sm"
            >
              {isImporting
                ? "Importiere..."
                : `${stats.new} Mitarbeiter einladen`}
            </button>
          </div>
        </>
      )}
    </div>
  );
}
