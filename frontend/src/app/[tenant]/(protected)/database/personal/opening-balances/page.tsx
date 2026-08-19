"use client";

import { useCallback, useMemo, useRef, useState } from "react";
import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";

import { Alert } from "~/components/ui/alert";
import { BackButton } from "~/components/ui/back-button";
import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import { ISODatePicker } from "~/components/ui/date-picker";
import { EmptyState } from "~/components/ui/empty-state";
import { ForbiddenPage } from "~/components/ui/forbidden-page";
import { Input } from "~/components/ui/input";
import {
  FormSkeleton,
  SkeletonRegion,
  TableSkeleton,
} from "~/components/ui/page-skeletons";
import { StatusBadge } from "~/components/ui/status-badge";
import { UploadSection } from "~/components/import/upload-section";
import { useToast } from "~/contexts/ToastContext";
import { hasPermission } from "~/lib/auth-utils";
import { formatDate, parseISODate, toISODate } from "~/lib/date-helpers";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "OpeningBalanceImportPage" });

// Wire-Format des generischen Imports (PascalCase, siehe
// services/import/import_service.go). Zeilen ohne Befund tauchen NICHT in
// `Errors` auf — nur fehlerhafte und mit Hinweisen versehene Zeilen.
interface ImportValidationError {
  field: string;
  message: string;
  code: string;
  severity: "error" | "warning";
}

interface OpeningBalanceRowData {
  personnel_number?: string;
  first_name?: string;
  last_name?: string;
  hours_balance?: string;
  vacation_entitled?: string;
  vacation_carryover?: string;
  vacation_remaining?: string;
}

interface ImportRowResult {
  RowNumber: number;
  Data: OpeningBalanceRowData;
  Errors: ImportValidationError[];
  Timestamp: string;
}

interface ImportResult {
  TotalRows: number;
  CreatedCount: number;
  UpdatedCount: number;
  ErrorCount: number;
  WarningCount: number;
  Errors: ImportRowResult[];
  DryRun: boolean;
}

type RowStatus = "ok" | "warning" | "error";

interface DisplayRow {
  key: string;
  rowNumber: number;
  status: RowStatus;
  name: string;
  personnelNumber: string;
  values: readonly { label: string; value: string }[];
  messages: readonly string[];
  isSummary: boolean;
}

const STATUS_TONE = {
  ok: { tone: "green", label: "Übernehmbar" },
  warning: { tone: "orange", label: "Hinweis" },
  error: { tone: "red", label: "Fehler" },
} as const;

function rowStatusFor(errors: readonly ImportValidationError[]): RowStatus {
  if (errors.some((e) => e.severity === "error")) return "error";
  if (errors.length > 0) return "warning";
  return "ok";
}

/** Leere Zellen bedeuten „diese Seite überspringen", nicht „Null". */
function cellValue(raw: string | undefined): string {
  const trimmed = raw?.trim() ?? "";
  return trimmed === "" ? "–" : trimmed;
}

function toDisplayRow(row: ImportRowResult): DisplayRow {
  const data = row.Data;
  const name = `${data.first_name ?? ""} ${data.last_name ?? ""}`.trim();
  return {
    key: `row-${row.RowNumber}`,
    rowNumber: row.RowNumber,
    status: rowStatusFor(row.Errors),
    name: name === "" ? "Ohne Namen" : name,
    personnelNumber: data.personnel_number?.trim() ?? "",
    values: [
      { label: "Stundensaldo", value: cellValue(data.hours_balance) },
      { label: "Jahresanspruch", value: cellValue(data.vacation_entitled) },
      { label: "Übertrag", value: cellValue(data.vacation_carryover) },
      { label: "Resturlaub", value: cellValue(data.vacation_remaining) },
    ],
    messages: row.Errors.map((e) => e.message),
    isSummary: false,
  };
}

/**
 * Sammelzeile für die Zeilen, die der Import gar nicht erst zurückmeldet:
 * das Framework listet nur auffällige Zeilen, sonst sähe eine saubere Datei
 * in der Vorschau leer aus.
 */
function summaryRow(count: number): DisplayRow {
  return {
    key: "row-summary",
    rowNumber: 0,
    status: "ok",
    name: `${count} weitere ${count === 1 ? "Zeile" : "Zeilen"} ohne Befund`,
    personnelNumber: "",
    values: [],
    messages: [],
    isSummary: true,
  };
}

function buildDisplayRows(result: ImportResult): DisplayRow[] {
  const listed = (result.Errors ?? []).map(toDisplayRow);
  const untouched = result.TotalRows - listed.length;
  return untouched > 0 ? [...listed, summaryRow(untouched)] : listed;
}

function isSpreadsheet(file: File): boolean {
  return (
    file.type === "text/csv" ||
    file.type ===
      "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" ||
    file.name.endsWith(".csv") ||
    file.name.endsWith(".xlsx")
  );
}

function PreviewRowCard({ row }: { readonly row: DisplayRow }) {
  const status = STATUS_TONE[row.status];
  return (
    <div className="rounded-xl border border-gray-200 bg-white p-3">
      <div className="flex items-start gap-3">
        <div className="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-full bg-gray-100 text-xs font-semibold text-gray-600">
          {row.isSummary ? "∑" : row.rowNumber}
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <h4 className="text-sm font-semibold text-gray-900">{row.name}</h4>
            {!row.isSummary && (
              <StatusBadge label={status.label} tone={status.tone} />
            )}
            {row.personnelNumber !== "" && (
              <span className="text-xs text-gray-500">
                Personalnummer {row.personnelNumber}
              </span>
            )}
          </div>
          {row.values.length > 0 && (
            <dl className="mt-2 grid grid-cols-2 gap-x-4 gap-y-1 sm:grid-cols-4">
              {row.values.map((value) => (
                <div key={value.label}>
                  <dt className="text-[11px] font-medium tracking-wide text-gray-500 uppercase">
                    {value.label}
                  </dt>
                  <dd className="text-sm text-gray-900 tabular-nums">
                    {value.value}
                  </dd>
                </div>
              ))}
            </dl>
          )}
          {row.messages.length > 0 && (
            <ul className="mt-2 space-y-0.5">
              {row.messages.map((message) => (
                <li
                  key={message}
                  className={`text-xs ${row.status === "error" ? "text-[#9F1F1E]" : "text-[#8A5600]"}`}
                >
                  {message}
                </li>
              ))}
            </ul>
          )}
        </div>
      </div>
    </div>
  );
}

function StatTile({
  label,
  value,
}: {
  readonly label: string;
  readonly value: number;
}) {
  return (
    <div className="rounded-xl bg-gray-50 px-3 py-2.5">
      <p className="text-[11px] font-medium tracking-wide text-gray-500 uppercase">
        {label}
      </p>
      <p className="mt-0.5 text-xl font-semibold text-gray-900 tabular-nums">
        {value}
      </p>
    </div>
  );
}

export default function OpeningBalanceImportPage() {
  const { data: session, status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
    },
  });
  const toast = useToast();
  const berlinToday = useBerlinToday();

  const [templateFormat, setTemplateFormat] = useState<"csv" | "xlsx">("xlsx");
  const [effectiveDate, setEffectiveDate] = useState("");
  const [note, setNote] = useState("");
  const [uploadedFile, setUploadedFile] = useState<File | null>(null);
  const [previewRows, setPreviewRows] = useState<DisplayRow[]>([]);
  const [previewResult, setPreviewResult] = useState<ImportResult | null>(null);
  const [previewStale, setPreviewStale] = useState(false);
  const [importResult, setImportResult] = useState<ImportResult | null>(null);
  const [isDragging, setIsDragging] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [isImporting, setIsImporting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const previewRequestVersion = useRef(0);

  // Ein Eröffnungssaldo beschreibt einen abgeschlossenen Stand; der heutige
  // Tag läuft noch, deshalb ist gestern der späteste sinnvolle Stichtag.
  const latestEffectiveDate = useMemo(() => {
    const latest = parseISODate(berlinToday);
    latest.setDate(latest.getDate() - 1);
    return toISODate(latest);
  }, [berlinToday]);

  const paramsComplete = effectiveDate !== "" && note.trim() !== "";

  const resetPreview = useCallback(() => {
    setPreviewRows([]);
    setPreviewResult(null);
    setImportResult(null);
    setPreviewStale(false);
  }, []);

  const resetAll = useCallback(() => {
    setUploadedFile(null);
    resetPreview();
    setIsDragging(false);
    setError(null);
  }, [resetPreview]);

  const buildFormData = useCallback(
    (file: File) => {
      const formData = new FormData();
      formData.append("file", file);
      formData.append("effective_date", effectiveDate);
      formData.append("note", note.trim());
      return formData;
    },
    [effectiveDate, note],
  );

  const handleDownloadTemplate = useCallback(async () => {
    try {
      const token = session?.user?.token;
      if (!token) throw new Error("Keine Authentifizierung");

      const response = await fetch(
        `/api/import/opening-balances/template?format=${templateFormat}`,
        { headers: { Authorization: `Bearer ${token}` } },
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
          ? "eroeffnungssalden-import-vorlage.xlsx"
          : "eroeffnungssalden-import-vorlage.csv";
      document.body.appendChild(link);
      link.click();
      link.remove();
      URL.revokeObjectURL(url);
    } catch (err) {
      logger.error("opening_balance_template_download_failed", {
        error: err instanceof Error ? err.message : String(err),
        format: templateFormat,
      });
      setError(err instanceof Error ? err.message : "Unbekannter Fehler");
    }
  }, [session, templateFormat]);

  const runPreview = useCallback(
    async (file: File) => {
      const requestVersion = ++previewRequestVersion.current;
      setError(null);
      setIsLoading(true);
      setImportResult(null);
      setPreviewStale(false);

      try {
        const token = session?.user?.token;
        if (!token) throw new Error("Keine Authentifizierung");

        const response = await fetch("/api/import/opening-balances/preview", {
          method: "POST",
          headers: { Authorization: `Bearer ${token}` },
          body: buildFormData(file),
        });
        const payload = (await response.json()) as Record<string, unknown>;
        if (!response.ok) {
          throw new Error(
            (payload.error as string | undefined) ??
              (payload.message as string | undefined) ??
              "Fehler bei der Vorschau",
          );
        }

        const result = payload.data as ImportResult;
        if (requestVersion !== previewRequestVersion.current) return;
        setPreviewResult(result);
        setPreviewRows(buildDisplayRows(result));
      } catch (err) {
        logger.error("opening_balance_preview_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        if (requestVersion === previewRequestVersion.current) {
          setError(err instanceof Error ? err.message : "Unbekannter Fehler");
          setPreviewRows([]);
          setPreviewResult(null);
        }
      } finally {
        if (requestVersion === previewRequestVersion.current)
          setIsLoading(false);
      }
    },
    [buildFormData, session],
  );

  const handleFileSelect = useCallback(
    (file: File) => {
      previewRequestVersion.current++;
      setUploadedFile(file);
      resetPreview();
      if (!paramsComplete) {
        setError(
          "Bitte zuerst Stichtag und Begründung angeben, dann wird die Vorschau erstellt.",
        );
        return;
      }
      void runPreview(file);
    },
    [paramsComplete, resetPreview, runPreview],
  );

  // Stichtag und Begründung gehen in jede Buchung ein — eine Vorschau, die
  // mit anderen Werten gerechnet wurde, darf nicht bestätigt werden.
  const invalidatePreview = useCallback(() => {
    previewRequestVersion.current++;
    setPreviewRows([]);
    setPreviewResult(null);
    setImportResult(null);
    setPreviewStale(uploadedFile !== null);
  }, [uploadedFile]);

  const handleImport = useCallback(async () => {
    if (!uploadedFile) return;
    setIsImporting(true);
    setError(null);

    try {
      const token = session?.user?.token;
      if (!token) throw new Error("Keine Authentifizierung");

      const response = await fetch("/api/import/opening-balances/import", {
        method: "POST",
        headers: { Authorization: `Bearer ${token}` },
        body: buildFormData(uploadedFile),
      });
      const payload = (await response.json()) as Record<string, unknown>;
      if (!response.ok) {
        throw new Error(
          (payload.error as string | undefined) ??
            (payload.message as string | undefined) ??
            "Fehler beim Import",
        );
      }

      const result = payload.data as ImportResult;
      setImportResult(result);
      setPreviewResult(null);
      setPreviewRows((result.Errors ?? []).map(toDisplayRow));

      if (result.ErrorCount > 0) {
        toast.warning(
          `${result.CreatedCount} übernommen, ${result.ErrorCount} übersprungen`,
        );
      } else {
        toast.success(
          `${result.CreatedCount} ${result.CreatedCount === 1 ? "Übernahme" : "Übernahmen"} gebucht`,
        );
      }
    } catch (err) {
      logger.error("opening_balance_import_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setError(err instanceof Error ? err.message : "Unbekannter Fehler");
    } finally {
      setIsImporting(false);
    }
  }, [buildFormData, session, toast, uploadedFile]);

  const handleDrop = useCallback(
    (e: React.DragEvent) => {
      e.preventDefault();
      e.stopPropagation();
      setIsDragging(false);
      const file = e.dataTransfer.files[0];
      if (!file) return;
      if (!isSpreadsheet(file)) {
        setError("Bitte nur CSV- oder Excel-Dateien (.csv, .xlsx) hochladen");
        return;
      }
      handleFileSelect(file);
    },
    [handleFileSelect],
  );

  if (status === "loading") {
    return (
      <SkeletonRegion
        label="Eröffnungssalden-Import wird geladen"
        className="mx-auto max-w-5xl space-y-5 px-4 py-6 sm:px-6 lg:px-8"
      >
        <FormSkeleton fields={4} />
        <TableSkeleton rows={5} columns={4} />
      </SkeletonRegion>
    );
  }

  // Die Datenverwaltung ist bereits adminOnly (database/layout.tsx); die
  // Buchungen selbst hängen am Stundenkonto-Recht.
  if (!hasPermission(session, "time_tracking:manage")) {
    return (
      <ForbiddenPage message="Für den Import von Eröffnungssalden wird die Berechtigung zur Verwaltung der Zeiterfassung benötigt." />
    );
  }

  const importable = previewResult?.CreatedCount ?? 0;
  const previewErrors = previewResult?.ErrorCount ?? 0;
  const showPreview = previewResult !== null && importResult === null;
  const importButtonLabel = isImporting
    ? "Wird übernommen…"
    : `${importable} ${importable === 1 ? "Übernahme" : "Übernahmen"} buchen`;

  return (
    <main className="mx-auto w-full max-w-5xl space-y-5 px-4 py-6 sm:px-6 lg:px-8">
      <BackButton referrer="/database/personal" />

      <header>
        <p className="text-xs font-semibold tracking-wide text-[#5080D8] uppercase">
          Datenverwaltung
        </p>
        <h1 className="mt-1 text-2xl font-bold text-gray-900 sm:text-3xl">
          Eröffnungssalden importieren
        </h1>
        <p className="mt-2 max-w-3xl text-sm leading-relaxed text-gray-600">
          Übernimmt die Stände aus dem Altsystem: Stundenkonto-Saldo,
          Urlaubsanspruch samt Vorjahresübertrag und den Resturlaub zum
          Stichtag. Aus dem Resturlaub errechnet moto die vor der Einführung
          genommenen Urlaubstage. Pro Person ist nur eine Übernahme möglich.
        </p>
      </header>

      {error && (
        <div className="relative">
          <Alert type="error" message={error} />
          <button
            type="button"
            onClick={() => setError(null)}
            className="absolute top-1/2 right-4 -translate-y-1/2 text-gray-500 hover:text-gray-800"
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

      {/* Schritt 1 — Vorlage */}
      <section className="moto-content-surface rounded-2xl border border-gray-200 bg-white p-4 shadow-sm sm:p-5">
        <h2 className="text-sm font-semibold text-gray-900">
          Schritt 1: Vorlage herunterladen
        </h2>
        <p className="mt-1 text-sm text-gray-600">
          Spalten: <span className="font-medium">Personalnummer</span>{" "}
          (optional), <span className="font-medium">Vorname</span>,{" "}
          <span className="font-medium">Nachname</span>,{" "}
          <span className="font-medium">Stundensaldo</span>,{" "}
          <span className="font-medium">Jahresanspruch</span>,{" "}
          <span className="font-medium">Vorjahresübertrag</span>,{" "}
          <span className="font-medium">Resturlaub</span>. Eine leere Zelle
          lässt die jeweilige Seite unangetastet.
        </p>
        <div className="mt-3 flex flex-col gap-3 sm:flex-row sm:items-end">
          <div className="sm:w-64">
            <label
              id="template-format-label"
              htmlFor="template-format"
              className="mb-1.5 block text-sm font-medium text-gray-700"
            >
              Format wählen
            </label>
            <CustomSelect
              id="template-format"
              ariaLabelledBy="template-format-label"
              value={templateFormat}
              options={[
                { value: "csv", label: "CSV (Komma-getrennt)" },
                { value: "xlsx", label: "Excel (.xlsx)" },
              ]}
              onChange={(next) => setTemplateFormat(next as "csv" | "xlsx")}
            />
          </div>
          <Button
            type="button"
            variant="outline"
            size="md"
            className="h-9"
            onClick={() => void handleDownloadTemplate()}
          >
            Vorlage herunterladen
          </Button>
        </div>
      </section>

      {/* Schritt 2 — Stichtag und Begründung */}
      <section className="moto-content-surface rounded-2xl border border-gray-200 bg-white p-4 shadow-sm sm:p-5">
        <h2 className="text-sm font-semibold text-gray-900">
          Schritt 2: Stichtag und Begründung
        </h2>
        <p className="mt-1 text-sm text-gray-600">
          Beides gilt für die ganze Datei und wird an jeder Buchung
          protokolliert.
        </p>
        <div className="mt-3 grid gap-3 sm:grid-cols-2">
          <ISODatePicker
            id="opening-balance-effective-date"
            label="Stichtag"
            value={effectiveDate}
            min={`${berlinToday.slice(0, 4)}-01-01`}
            max={latestEffectiveDate}
            onChange={(next) => {
              setEffectiveDate(next);
              invalidatePreview();
            }}
            placeholder="Datum wählen"
          />
          <Input
            id="opening-balance-note"
            name="opening-balance-note"
            label="Begründung"
            value={note}
            maxLength={200}
            placeholder={`Übernahme aus Altsystem zum ${formatDate(latestEffectiveDate)}`}
            onChange={(e) => {
              setNote(e.target.value);
              invalidatePreview();
            }}
          />
        </div>
      </section>

      {/* Schritt 3 — Datei */}
      <UploadSection
        title="Schritt 3: Datei hochladen"
        isDragging={isDragging}
        isLoading={isLoading}
        uploadedFile={uploadedFile}
        onDragEnter={(e) => {
          e.preventDefault();
          e.stopPropagation();
          setIsDragging(true);
        }}
        onDragLeave={(e) => {
          e.preventDefault();
          e.stopPropagation();
          setIsDragging(false);
        }}
        onDragOver={(e) => {
          e.preventDefault();
          e.stopPropagation();
        }}
        onDrop={handleDrop}
        onFileSelect={handleFileSelect}
      />

      {previewStale && uploadedFile !== null && (
        <section className="moto-content-surface flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-gray-200 bg-white p-4 shadow-sm">
          <p className="text-sm text-gray-600">
            Stichtag oder Begründung wurden geändert. Die Vorschau muss mit den
            neuen Angaben neu erstellt werden.
          </p>
          <Button
            type="button"
            variant="outline"
            size="md"
            className="h-9"
            disabled={!paramsComplete || isLoading}
            onClick={() => void runPreview(uploadedFile)}
          >
            Vorschau erneut erstellen
          </Button>
        </section>
      )}

      {/* Schritt 4 — Vorschau */}
      {showPreview && previewResult && (
        <section className="moto-content-surface rounded-2xl border border-gray-200 bg-white p-4 shadow-sm sm:p-5">
          <h2 className="text-sm font-semibold text-gray-900">
            Schritt 4: Vorschau prüfen
          </h2>
          <div className="mt-3 grid grid-cols-2 gap-2.5 sm:grid-cols-4">
            <StatTile label="Zeilen" value={previewResult.TotalRows} />
            <StatTile label="Übernehmbar" value={importable} />
            <StatTile label="Hinweise" value={previewResult.WarningCount} />
            <StatTile label="Fehler" value={previewErrors} />
          </div>
          {previewErrors > 0 && importable > 0 && (
            <p className="mt-3 text-sm text-gray-600">
              Fehlerhafte Zeilen werden beim Import übersprungen; die übrigen{" "}
              {importable} werden gebucht.
            </p>
          )}
          <div className="mt-3 space-y-2">
            {previewRows.length === 0 ? (
              <EmptyState
                title="Keine Zeilen gefunden"
                description="Die Datei enthält keine auswertbaren Datenzeilen."
              />
            ) : (
              previewRows.map((row) => (
                <PreviewRowCard key={row.key} row={row} />
              ))
            )}
          </div>
        </section>
      )}

      {/* Ergebnis */}
      {importResult && (
        <section className="moto-content-surface rounded-2xl border border-gray-200 bg-white p-4 shadow-sm sm:p-5">
          <h2 className="text-sm font-semibold text-gray-900">
            Import abgeschlossen
          </h2>
          <div className="mt-3 grid grid-cols-2 gap-2.5 sm:grid-cols-4">
            <StatTile label="Zeilen" value={importResult.TotalRows} />
            <StatTile label="Übernommen" value={importResult.CreatedCount} />
            <StatTile label="Hinweise" value={importResult.WarningCount} />
            <StatTile label="Fehler" value={importResult.ErrorCount} />
          </div>
          {previewRows.length > 0 && (
            <>
              <p className="mt-3 text-sm text-gray-600">
                Diese Zeilen wurden übersprungen oder tragen Hinweise:
              </p>
              <div className="mt-2 space-y-2">
                {previewRows.map((row) => (
                  <PreviewRowCard key={row.key} row={row} />
                ))}
              </div>
            </>
          )}
          <div className="mt-4">
            <Button
              type="button"
              variant="outline"
              size="md"
              className="h-9"
              onClick={resetAll}
            >
              Weitere Datei importieren
            </Button>
          </div>
        </section>
      )}

      {showPreview && previewRows.length > 0 && (
        <div className="sticky bottom-4 z-10 flex flex-col gap-2 rounded-2xl border border-gray-200 bg-white/95 px-4 py-3 shadow-lg backdrop-blur-sm sm:flex-row">
          <Button
            type="button"
            variant="outline"
            size="md"
            className="h-9 flex-1"
            onClick={resetAll}
          >
            Abbrechen
          </Button>
          <Button
            type="button"
            variant="success"
            size="md"
            className="h-9 flex-1"
            disabled={importable === 0 || isImporting}
            onClick={() => void handleImport()}
          >
            {importButtonLabel}
          </Button>
        </div>
      )}
    </main>
  );
}
