"use client";

import { useState, useCallback, useEffect, useMemo, useRef } from "react";
import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import { Download, Info, ListChecks, RefreshCw, X } from "lucide-react";
import { SkeletonRegion, FormSkeleton } from "~/components/ui/page-skeletons";
import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import { Alert } from "~/components/ui/alert";
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
import { hasPermission } from "~/lib/auth-utils";
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
  severity: "error" | "warning" | "info";
}

interface ImportRowResult {
  RowNumber: number;
  Data: {
    first_name: string;
    last_name: string;
    email?: string;
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
  notes: string[];
  first_name: string;
  last_name: string;
  email: string;
  role_name: string;
  position: string;
}

function rowStatusFor(errors: ImportError[]): RowStatus {
  const isExisting = errors.some(
    (e) => e.code === "already_exists" || e.code === "will_update",
  );
  if (isExisting) return "existing";
  if (errors.some((e) => e.severity === "error")) return "error";
  if (errors.some((e) => e.severity === "warning")) return "warning";
  return "new";
}

function toDisplayStaff(row: ImportRowResult): DisplayStaff {
  return {
    row: row.RowNumber,
    status: rowStatusFor(row.Errors),
    errors: row.Errors.filter((e) => e.severity !== "info").map(
      (e) => e.message,
    ),
    notes: row.Errors.filter((e) => e.severity === "info").map(
      (e) => e.message,
    ),
    first_name: row.Data.first_name,
    last_name: row.Data.last_name,
    email: row.Data.email ?? "",
    role_name: row.Data.role_name,
    position: row.Data.position ?? "",
  };
}

export default function StaffImportPage() {
  const previewGeneration = useRef(0);
  const [uploadedFile, setUploadedFile] = useState<File | null>(null);
  const [previewData, setPreviewData] = useState<DisplayStaff[]>([]);
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

  // Update and upsert mode write the same fields as the personnel screens,
  // so the backend refuses them without both personnel permissions (#2906).
  // Without them the page stays in create mode and says so.
  const canChangeExisting =
    hasPermission(session, "staff:manage") &&
    hasPermission(session, "staff:stammdaten");

  // Load the tenant's role names so the user knows what to put in the
  // "Rolle" column (the import matches role names exactly, case-insensitive).
  // The list endpoint accepts users:create, the permission this page opens
  // on, so the hint is complete for every user who may import (#2906).
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
            notes: [],
            first_name: `${importData.TotalRows} Mitarbeiter`,
            last_name: "bereit zum Import",
            email: "",
            role_name: "",
            position: "",
          });
        }

        if (generation !== previewGeneration.current) return;
        setPreviewData(displayData);
        setImportResult(importData);
      } catch (err) {
        logger.error("staff_preview_failed", {
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
          `${importData.CreatedCount} angelegt, ${importData.UpdatedCount} aktualisiert, ${importData.ErrorCount} übersprungen`,
        );
      } else {
        setImportComplete(true);
        toast.success(
          `${importData.CreatedCount} angelegt, ${importData.UpdatedCount} aktualisiert`,
        );
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
  const importLabel =
    mode === "create"
      ? `${stats.new} Mitarbeiter anlegen`
      : mode === "update"
        ? `${stats.existing} Mitarbeiter aktualisieren`
        : `${stats.new + stats.existing} Mitarbeiter übernehmen`;

  if (status === "loading") {
    return (
      <SkeletonRegion label="Mitarbeiter-Import wird geladen…">
        <FormSkeleton fields={2} />
      </SkeletonRegion>
    );
  }

  return (
    <div className="w-full space-y-6">
      {/* Info Section */}
      <div className="border-moto-blue/20 bg-moto-blue-soft rounded-xl border p-6">
        <div className="flex items-start gap-4">
          <div className="flex-shrink-0">
            <Info
              className="text-moto-blue-strong h-6 w-6"
              aria-hidden="true"
            />
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
                Jede Zeile wird sofort in der Personalliste angelegt, mit
                Stammdaten wie Personalnummer, Adresse und Vertragsdaten
              </li>
              <li>
                Steht eine E-Mail in der Zeile, bekommt die Person zusätzlich
                eine Einladung und setzt ihr Passwort selbst. Ohne E-Mail gibt
                es keinen Zugang
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
            className="text-moto-red hover:text-moto-red-strong absolute top-1/2 right-4 -translate-y-1/2"
            aria-label="Fehler schließen"
          >
            <X className="h-4 w-4" aria-hidden="true" />
          </button>
        </div>
      )}

      {/* Download Template */}
      <div className="moto-content-surface rounded-xl border p-6 shadow-sm">
        <h3 className="mb-4 flex items-center gap-2 text-sm font-semibold text-gray-900">
          <Download className="h-5 w-5 text-gray-600" aria-hidden="true" />
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
              Pflicht: <span className="font-medium">Vorname</span>,{" "}
              <span className="font-medium">Nachname</span>,{" "}
              <span className="font-medium">Rolle</span>. Alle weiteren Spalten
              (E-Mail, Personalnummer, Adresse, Vertrag, Qualifikationen) sind
              optional und im Blatt „Hinweise" erklärt
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
      </div>

      <div className="moto-content-surface rounded-xl border p-6 shadow-sm">
        <h3 className="mb-4 flex items-center gap-2 text-sm font-semibold text-gray-900">
          <RefreshCw className="h-5 w-5 text-gray-600" aria-hidden="true" />
          Schritt 2: Was soll der Import tun?
        </h3>
        {canChangeExisting ? (
          <>
            <SegmentedControl
              items={IMPORT_MODE_ITEMS}
              value={mode}
              onChange={handleModeChange}
              fullWidth
              ariaLabel="Import-Modus"
            />
            <p className="mt-3 text-sm text-gray-600">
              {IMPORT_MODE_HINTS[mode]}
            </p>
            <p className="mt-1 text-sm text-gray-500">
              Bekannt ist eine Zeile über Personalnummer, sonst E-Mail, sonst
              Vor- und Nachname.
            </p>
          </>
        ) : (
          <p className="text-sm text-gray-600">
            Mit Ihren Berechtigungen legt der Import nur neue Mitarbeiter an.
            Zeilen, die es schon gibt, werden als Fehler gemeldet. Bestehende
            Datensätze ändert die Leitung.
          </p>
        )}
      </div>

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
          <StatsCards
            total={stats.total}
            newCount={stats.new}
            existing={stats.existing}
            existingTitle={
              mode === "create" ? "Vorhanden" : "Wird aktualisiert"
            }
            errors={stats.errors}
          />

          <div className="moto-content-surface overflow-hidden rounded-xl border shadow-sm">
            <div className="border-b border-gray-100 p-4">
              <h3 className="flex items-center gap-2 text-sm font-semibold text-gray-900">
                <ListChecks
                  className="h-5 w-5 text-gray-600"
                  aria-hidden="true"
                />
                Schritt 4: Datenvorschau
              </h3>
            </div>

            <div className="space-y-2 p-3">
              {previewData.map((staff, idx) => (
                <StudentRowCard
                  key={staff.row}
                  student={{
                    row: staff.row,
                    status: staff.status,
                    errors: staff.errors,
                    notes: staff.notes,
                    first_name: staff.first_name,
                    last_name: staff.last_name,
                    meta: [staff.email, staff.role_name, staff.position],
                  }}
                  index={idx}
                />
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
              disabled={stats.errors > 0 || isImporting || isLoading}
              className="bg-moto-green hover:bg-moto-green-hover flex-1 rounded-lg px-3 py-2 text-xs font-medium text-gray-950 transition-all duration-200 hover:shadow-lg disabled:cursor-not-allowed disabled:opacity-50 md:px-4 md:text-sm"
            >
              {isImporting ? "Importiere..." : importLabel}
            </button>
          </div>
        </>
      )}
    </div>
  );
}
