"use client";

import { useEffect, useMemo, useState } from "react";
import type { ReactNode } from "react";
import { createPortal } from "react-dom";
import {
  Check,
  Download,
  FileSpreadsheet,
  FileText,
  Info,
  X,
} from "lucide-react";
import {
  STUDENT_EXPORT_COLUMNS,
  STUDENT_EXPORT_PRESETS,
  exportStudents,
  type StudentExportColumn,
  type StudentExportColumnOption,
  type StudentExportFilters,
  type StudentExportFormat,
  type StudentExportPreset,
} from "~/lib/student-export-api";
import { useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "StudentExportModal" });

interface StudentExportModalProps {
  readonly isOpen: boolean;
  readonly filters: StudentExportFilters;
  readonly resultCount: number;
  readonly onClose: () => void;
}

const FORMAT_OPTIONS: Array<{
  value: StudentExportFormat;
  label: string;
  icon: ReactNode;
}> = [
  { value: "pdf", label: "PDF", icon: <FileText className="h-4 w-4" /> },
  { value: "docx", label: "DOCX", icon: <FileText className="h-4 w-4" /> },
  {
    value: "xlsx",
    label: "XLSX",
    icon: <FileSpreadsheet className="h-4 w-4" />,
  },
];

export function StudentExportModal({
  isOpen,
  filters,
  resultCount,
  onClose,
}: StudentExportModalProps) {
  const [mounted, setMounted] = useState(false);
  const [preset, setPreset] = useState<StudentExportPreset>("ogs_weekly");
  const [format, setFormat] = useState<StudentExportFormat>("pdf");
  const [title, setTitle] = useState("OGS Wochenliste");
  const [columns, setColumns] = useState<StudentExportColumn[]>(
    STUDENT_EXPORT_PRESETS[0]?.columns ?? [],
  );
  const [exporting, setExporting] = useState(false);
  const toast = useToast();

  useEffect(() => {
    setMounted(true);
  }, []);

  useEffect(() => {
    if (!isOpen) return;
    const presetConfig = STUDENT_EXPORT_PRESETS.find(
      (item) => item.id === preset,
    );
    if (!presetConfig) return;
    setTitle(presetConfig.label);
    setColumns(presetConfig.columns);
  }, [isOpen, preset]);

  useEffect(() => {
    if (!isOpen) return;
    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") onClose();
    }
    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [isOpen, onClose]);

  const activePreset = useMemo(
    () => STUDENT_EXPORT_PRESETS.find((item) => item.id === preset),
    [preset],
  );

  if (!isOpen || !mounted) return null;

  const toggleColumn = (column: StudentExportColumn) => {
    setColumns((current) =>
      current.includes(column)
        ? current.filter((item) => item !== column)
        : sortColumns([...current, column]),
    );
  };

  const handleExport = async () => {
    if (columns.length === 0) {
      toast.error("Bitte wähle mindestens eine Spalte aus.");
      return;
    }
    setExporting(true);
    try {
      await exportStudents({ format, preset, title, filters, columns });
      toast.success("Export wurde erstellt.");
      onClose();
    } catch (error) {
      const message =
        error instanceof Error ? error.message : "Export fehlgeschlagen";
      logger.error("student_export_failed", { error: message });
      toast.error(message);
    } finally {
      setExporting(false);
    }
  };

  return createPortal(
    <div
      className="fixed inset-0 z-[1000] flex items-center justify-center bg-gray-950/35 p-4 backdrop-blur-sm"
      role="dialog"
      aria-modal="true"
      aria-labelledby="student-export-title"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) onClose();
      }}
    >
      <div className="moto-content-surface flex max-h-[calc(100vh-2rem)] w-full max-w-2xl flex-col overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-xl">
        <div className="flex items-start justify-between gap-4 border-b border-gray-100 px-5 py-4">
          <div>
            <h2
              id="student-export-title"
              className="text-base font-semibold text-gray-950"
            >
              Kindersuche exportieren
            </h2>
            <p className="mt-1 text-sm text-gray-500">
              {resultCount} Kinder aus der aktuellen Filterung.
            </p>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="inline-flex h-8 w-8 items-center justify-center rounded-lg border border-gray-200 bg-white text-gray-500 shadow-sm transition-colors hover:bg-gray-50 hover:text-gray-800 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
            aria-label="Export schließen"
          >
            <X className="h-4 w-4" aria-hidden />
          </button>
        </div>

        <div className="min-h-0 flex-1 space-y-5 overflow-y-auto px-5 py-5">
          <section>
            <label
              htmlFor="student-export-title-input"
              className="text-sm font-medium text-gray-900"
            >
              Titel
            </label>
            <input
              id="student-export-title-input"
              value={title}
              onChange={(event) => setTitle(event.target.value)}
              className="mt-2 h-10 w-full rounded-lg border border-gray-200 bg-white px-3 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
            />
          </section>

          <section>
            <p className="text-sm font-medium text-gray-900">Vorlage</p>
            <div className="mt-2 grid gap-2 sm:grid-cols-2">
              {STUDENT_EXPORT_PRESETS.map((item) => (
                <button
                  key={item.id}
                  type="button"
                  onClick={() => setPreset(item.id)}
                  className={`rounded-lg border px-3 py-3 text-left shadow-sm transition-colors focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none ${
                    preset === item.id
                      ? "border-gray-900 bg-gray-950 text-white"
                      : "border-gray-200 bg-white text-gray-800 hover:bg-gray-50"
                  }`}
                >
                  <span className="block text-sm font-semibold">
                    {item.label}
                  </span>
                  <span
                    className={`mt-1 block text-xs ${
                      preset === item.id ? "text-gray-200" : "text-gray-500"
                    }`}
                  >
                    {item.description}
                  </span>
                </button>
              ))}
            </div>
          </section>

          <section>
            <p className="text-sm font-medium text-gray-900">Format</p>
            <div className="mt-2 flex flex-wrap gap-2">
              {FORMAT_OPTIONS.map((item) => (
                <button
                  key={item.value}
                  type="button"
                  onClick={() => setFormat(item.value)}
                  className={`inline-flex h-9 items-center gap-2 rounded-lg border px-3 text-sm font-medium shadow-sm transition-colors focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none ${
                    format === item.value
                      ? "border-gray-900 bg-gray-950 text-white"
                      : "border-gray-200 bg-white text-gray-700 hover:bg-gray-50"
                  }`}
                >
                  {item.icon}
                  {item.label}
                </button>
              ))}
            </div>
          </section>

          <section>
            <div className="flex items-center justify-between gap-3">
              <div>
                <p className="text-sm font-medium text-gray-900">Spalten</p>
                <p className="text-xs text-gray-500">
                  {activePreset?.label ?? "Vorlage"} kann angepasst werden.
                </p>
              </div>
              <span className="rounded-full bg-gray-100 px-2 py-1 text-xs font-medium text-gray-600">
                {columns.length} aktiv
              </span>
            </div>
            <div className="mt-3 grid gap-2 sm:grid-cols-2">
              {STUDENT_EXPORT_COLUMNS.map((column) => {
                const checked = columns.includes(column.id);
                return (
                  <ExportColumnCheckbox
                    key={column.id}
                    checked={checked}
                    column={column}
                    onChange={() => toggleColumn(column.id)}
                  />
                );
              })}
            </div>
          </section>
        </div>

        <div className="flex shrink-0 flex-col-reverse gap-2 rounded-b-2xl border-t border-gray-100 bg-white px-5 pt-4 pb-6 sm:flex-row sm:justify-end">
          <button
            type="button"
            onClick={onClose}
            disabled={exporting}
            className="inline-flex h-9 items-center justify-center rounded-lg border border-gray-200 bg-white px-3 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
          >
            Abbrechen
          </button>
          <button
            type="button"
            onClick={() => void handleExport()}
            disabled={exporting}
            className="inline-flex h-9 items-center justify-center gap-2 rounded-lg bg-gray-900 px-3 text-sm font-medium text-white shadow-sm transition-colors hover:bg-gray-700 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
          >
            <Download className="h-4 w-4" aria-hidden />
            {exporting ? "Exportiert..." : "Exportieren"}
          </button>
        </div>
      </div>
    </div>,
    document.body,
  );
}

function sortColumns(columns: StudentExportColumn[]): StudentExportColumn[] {
  const order = new Map(
    STUDENT_EXPORT_COLUMNS.map((column, index) => [column.id, index]),
  );
  return [...columns].sort((a, b) => (order.get(a) ?? 0) - (order.get(b) ?? 0));
}

function ExportColumnCheckbox({
  checked,
  column,
  onChange,
}: Readonly<{
  checked: boolean;
  column: StudentExportColumnOption;
  onChange: () => void;
}>) {
  return (
    <label
      title={column.description}
      className={`flex min-h-11 cursor-pointer items-center gap-3 rounded-2xl border px-3 py-2.5 text-sm font-medium text-gray-700 transition-colors focus-within:ring-2 focus-within:ring-gray-300 ${
        checked
          ? "border-gray-300 bg-gray-50"
          : "border-gray-100 bg-white hover:border-gray-200 hover:bg-gray-50"
      }`}
    >
      <input
        type="checkbox"
        checked={checked}
        onChange={onChange}
        className="sr-only"
      />
      <span
        className={`flex h-5 w-5 shrink-0 items-center justify-center rounded-md border shadow-sm transition-all ${
          checked ? "border-gray-900 bg-gray-900" : "border-gray-300 bg-white"
        }`}
        aria-hidden="true"
      >
        <Check
          className={`h-3.5 w-3.5 text-white transition-opacity ${
            checked ? "opacity-100" : "opacity-0"
          }`}
        />
      </span>
      <span className="min-w-0 flex-1 leading-snug">
        <span className="flex items-center gap-1.5">
          <span>{column.label}</span>
          <Info className="h-3.5 w-3.5 text-gray-400" aria-hidden />
        </span>
        <span className="mt-0.5 block text-xs leading-snug font-normal text-gray-500">
          {column.description}
        </span>
      </span>
    </label>
  );
}
